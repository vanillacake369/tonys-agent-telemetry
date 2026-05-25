// Package vllm implements a telemetry.ProviderIngestor for the vLLM
// inference server. Detection is a HEAD on /metrics; ingestion polls the
// Prometheus endpoint, tracks per-model counter deltas, and synthesises a
// telemetry.Span whenever a scrape reveals new tokens.
//
// vLLM has no per-request log; the scraper emits one aggregate Span per
// scrape interval per model. Each Span's TraceID is synthetic: "vllm-<model>".
// This is intentionally coarse — finer-grained spans require vLLM's OTel
// SDK integration which is opt-in and not always enabled.
package vllm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// DefaultEndpoint is vLLM's default Prometheus port.
const DefaultEndpoint = "http://127.0.0.1:8000/metrics"

// Ingestor scrapes vLLM's /metrics endpoint and emits Spans on counter
// deltas.
type Ingestor struct {
	Endpoint     string        // override via env or constructor
	PollInterval time.Duration // default 10s

	httpClient *http.Client
}

// New returns an Ingestor with default endpoint and a sane HTTP client.
func New() *Ingestor {
	return &Ingestor{
		Endpoint:     DefaultEndpoint,
		PollInterval: 10 * time.Second,
		httpClient:   &http.Client{Timeout: 2 * time.Second},
	}
}

// ProviderID returns "vllm".
func (i *Ingestor) ProviderID() string { return "vllm" }

// Detect performs a fast HEAD on the metrics endpoint. A 2xx response that
// includes "vllm:" prefixed metrics confirms vLLM. We compromise: HEAD is
// cheaper than GET but the body check would miss collisions with other
// Prom-exposing services on port 8000 (e.g. llama-server). Mitigation: a
// GET with a short read window.
func (i *Ingestor) Detect(ctx context.Context) bool {
	rctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, i.Endpoint, nil)
	if err != nil {
		return false
	}
	client := i.httpClient
	if client == nil {
		client = &http.Client{Timeout: 200 * time.Millisecond}
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}
	// Read at most 8KB looking for any "vllm:" prefixed metric line.
	buf := make([]byte, 8192)
	n, _ := io.ReadFull(resp.Body, buf)
	return strings.Contains(string(buf[:n]), "vllm:")
}

// Ingest polls the metrics endpoint, computes deltas, and emits Spans.
// Stops when ctx is cancelled. Transient HTTP errors are ignored.
func (i *Ingestor) Ingest(ctx context.Context, out chan<- telemetry.Span) error {
	ticker := time.NewTicker(i.PollInterval)
	defer ticker.Stop()

	type counterKey struct {
		metric, model string
	}
	previous := make(map[counterKey]float64)
	first := true

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		metrics, err := i.scrape(ctx)
		if err != nil {
			continue
		}

		// Group by model.
		modelTotals := make(map[string]struct {
			promptDelta, genDelta float64
		})
		for _, m := range metrics {
			if !strings.HasPrefix(m.name, "vllm:") {
				continue
			}
			model := m.labels["model_name"]
			if model == "" {
				model = "unknown"
			}
			key := counterKey{metric: m.name, model: model}
			prev, seen := previous[key]
			previous[key] = m.value
			if !seen || first {
				continue
			}
			delta := m.value - prev
			if delta <= 0 {
				continue
			}
			entry := modelTotals[model]
			switch m.name {
			case "vllm:prompt_tokens_total":
				entry.promptDelta += delta
			case "vllm:generation_tokens_total":
				entry.genDelta += delta
			}
			modelTotals[model] = entry
		}
		first = false

		now := time.Now()
		for model, totals := range modelTotals {
			if totals.promptDelta == 0 && totals.genDelta == 0 {
				continue
			}
			sp := telemetry.Span{
				TraceID:      "vllm-" + model,
				SpanID:       fmt.Sprintf("vllm-%s-%d", model, now.UnixNano()),
				System:       "vllm",
				Model:        model,
				InputTokens:  int(totals.promptDelta),
				OutputTokens: int(totals.genDelta),
				StartTime:    now.Add(-i.PollInterval),
				EndTime:      now,
				Status:       "done",
				Attrs: map[string]string{
					"gen_ai.operation.name": "chat",
				},
			}
			select {
			case out <- sp:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

// metric is one parsed Prometheus sample.
type metric struct {
	name   string
	labels map[string]string
	value  float64
}

// scrape fetches and parses Prometheus text format.
func (i *Ingestor) scrape(ctx context.Context) ([]metric, error) {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, i.Endpoint, nil)
	if err != nil {
		return nil, err
	}
	client := i.httpClient
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return parsePrometheus(resp.Body), nil
}

// parsePrometheus parses Prometheus text format into samples. Tolerates
// HELP/TYPE comments, blank lines, and malformed lines (skipped).
func parsePrometheus(r io.Reader) []metric {
	var out []metric
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Format: name{label="v",...} value [timestamp]
		// Also allowed: name value
		var name, labelStr, valueStr string
		if idx := strings.Index(line, "{"); idx >= 0 {
			name = line[:idx]
			rest := line[idx+1:]
			end := strings.Index(rest, "}")
			if end < 0 {
				continue
			}
			labelStr = rest[:end]
			valueStr = strings.TrimSpace(rest[end+1:])
		} else {
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			name = parts[0]
			valueStr = parts[1]
		}
		// Trim potential trailing timestamp.
		if sp := strings.Index(valueStr, " "); sp > 0 {
			valueStr = valueStr[:sp]
		}
		val, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}
		out = append(out, metric{
			name:   name,
			labels: parseLabels(labelStr),
			value:  val,
		})
	}
	return out
}

// parseLabels parses label="v",label2="v2" into a map.
func parseLabels(s string) map[string]string {
	m := map[string]string{}
	if s == "" {
		return m
	}
	// Walk respecting quoted commas. For our purposes a naive split on `",`
	// works because Prom escapes embedded quotes as `\"`.
	for _, pair := range splitLabels(s) {
		eq := strings.Index(pair, "=")
		if eq < 0 {
			continue
		}
		k := strings.TrimSpace(pair[:eq])
		v := strings.TrimSpace(pair[eq+1:])
		v = strings.TrimPrefix(v, `"`)
		v = strings.TrimSuffix(v, `"`)
		m[k] = v
	}
	return m
}

func splitLabels(s string) []string {
	var parts []string
	var buf strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s):
			buf.WriteByte(c)
			buf.WriteByte(s[i+1])
			i++
		case c == '"':
			inQuote = !inQuote
			buf.WriteByte(c)
		case c == ',' && !inQuote:
			parts = append(parts, buf.String())
			buf.Reset()
		default:
			buf.WriteByte(c)
		}
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	return parts
}

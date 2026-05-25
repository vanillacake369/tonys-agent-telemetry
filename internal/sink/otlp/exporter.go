// Package otlp implements a telemetry-sink exporter that forwards
// telemetry.Span values to a remote OTLP/HTTP endpoint as JSON. Pair with
// internal/provider/otlp (the receiver) to chain local detection → local
// TUI rendering → remote collector (Tempo, Honeycomb, Langfuse, etc.).
//
// MVP scope: best-effort POST, batched by count + timeout; no retries, no
// gRPC, no compression. Failures are silent (debug log only) — this sink
// must never block or crash the producing pipeline.
package otlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// Exporter forwards Spans to a remote OTLP/HTTP collector. Constructed with
// the target URL; call Run to start consuming a channel.
type Exporter struct {
	URL          string
	BatchSize    int           // flush when this many spans queued (default 32)
	BatchTimeout time.Duration // flush after this idle period (default 2s)
	HTTPTimeout  time.Duration // per-request timeout (default 5s)

	httpClient *http.Client
	mu         sync.Mutex
	batch      []telemetry.Span
}

// New returns an Exporter targeting url with sane defaults.
func New(url string) *Exporter {
	return &Exporter{
		URL:          url,
		BatchSize:    32,
		BatchTimeout: 2 * time.Second,
		HTTPTimeout:  5 * time.Second,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
	}
}

// Run consumes spans from in until ctx is cancelled or the channel closes.
// Each batch is POSTed to URL as OTLP/JSON.
func (e *Exporter) Run(ctx context.Context, in <-chan telemetry.Span) error {
	if e.BatchSize <= 0 {
		e.BatchSize = 32
	}
	if e.BatchTimeout <= 0 {
		e.BatchTimeout = 2 * time.Second
	}
	timer := time.NewTimer(e.BatchTimeout)
	defer timer.Stop()
	defer e.flush(ctx) // drain on exit

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case sp, ok := <-in:
			if !ok {
				return nil
			}
			e.mu.Lock()
			e.batch = append(e.batch, sp)
			full := len(e.batch) >= e.BatchSize
			e.mu.Unlock()
			if full {
				e.flush(ctx)
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(e.BatchTimeout)
			}
		case <-timer.C:
			e.flush(ctx)
			timer.Reset(e.BatchTimeout)
		}
	}
}

// flush sends the current batch and clears it. Errors are swallowed.
func (e *Exporter) flush(ctx context.Context) {
	e.mu.Lock()
	if len(e.batch) == 0 {
		e.mu.Unlock()
		return
	}
	batch := e.batch
	e.batch = nil
	e.mu.Unlock()

	payload := encodeTracesJSON(batch)
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	rctx, cancel := context.WithTimeout(ctx, e.HTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, e.URL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := e.httpClient
	if client == nil {
		client = &http.Client{Timeout: e.HTTPTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// EncodeTracesJSON is exposed for tests; converts a batch of Spans to the
// OTLP ExportTraceServiceRequest JSON envelope.
func EncodeTracesJSON(batch []telemetry.Span) any { return encodeTracesJSON(batch) }

func encodeTracesJSON(batch []telemetry.Span) any {
	// Group by System (gen_ai.system) — each becomes a resource_spans entry.
	groups := map[string][]telemetry.Span{}
	for _, s := range batch {
		groups[s.System] = append(groups[s.System], s)
	}

	type kv struct {
		Key   string `json:"key"`
		Value struct {
			StringValue string `json:"stringValue,omitempty"`
			IntValue    string `json:"intValue,omitempty"`
		} `json:"value"`
	}
	type otlpSpan struct {
		TraceID           string `json:"traceId"`
		SpanID            string `json:"spanId"`
		ParentSpanID      string `json:"parentSpanId,omitempty"`
		Name              string `json:"name"`
		StartTimeUnixNano string `json:"startTimeUnixNano,omitempty"`
		EndTimeUnixNano   string `json:"endTimeUnixNano,omitempty"`
		Attributes        []kv   `json:"attributes,omitempty"`
		Status            struct {
			Code int `json:"code"`
		} `json:"status"`
	}
	type scopeSpans struct {
		Spans []otlpSpan `json:"spans"`
	}
	type resource struct {
		Attributes []kv `json:"attributes,omitempty"`
	}
	type resourceSpans struct {
		Resource   resource     `json:"resource"`
		ScopeSpans []scopeSpans `json:"scopeSpans"`
	}
	type envelope struct {
		ResourceSpans []resourceSpans `json:"resourceSpans"`
	}

	stringAttr := func(k, v string) kv {
		x := kv{Key: k}
		x.Value.StringValue = v
		return x
	}
	intAttr := func(k string, v int) kv {
		x := kv{Key: k}
		x.Value.IntValue = fmt.Sprintf("%d", v)
		return x
	}

	env := envelope{}
	for sys, spans := range groups {
		rs := resourceSpans{
			Resource: resource{
				Attributes: []kv{stringAttr("service.name", "tonys-agent-telemetry"),
					stringAttr("gen_ai.system", sys)},
			},
		}
		ss := scopeSpans{}
		for _, sp := range spans {
			o := otlpSpan{
				TraceID:      sp.TraceID,
				SpanID:       sp.SpanID,
				ParentSpanID: sp.ParentSpanID,
				Name:         sp.Attrs["gen_ai.operation.name"],
			}
			if !sp.StartTime.IsZero() {
				o.StartTimeUnixNano = fmt.Sprintf("%d", sp.StartTime.UnixNano())
			}
			if !sp.EndTime.IsZero() {
				o.EndTimeUnixNano = fmt.Sprintf("%d", sp.EndTime.UnixNano())
			}
			if sp.Model != "" {
				o.Attributes = append(o.Attributes, stringAttr("gen_ai.request.model", sp.Model))
			}
			if sp.InputTokens > 0 {
				o.Attributes = append(o.Attributes, intAttr("gen_ai.usage.input_tokens", sp.InputTokens))
			}
			if sp.OutputTokens > 0 {
				o.Attributes = append(o.Attributes, intAttr("gen_ai.usage.output_tokens", sp.OutputTokens))
			}
			for k, v := range sp.Attrs {
				if k == "gen_ai.operation.name" {
					continue
				}
				o.Attributes = append(o.Attributes, stringAttr(k, v))
			}
			if sp.Status == "error" {
				o.Status.Code = 2
			} else {
				o.Status.Code = 1
			}
			ss.Spans = append(ss.Spans, o)
		}
		rs.ScopeSpans = append(rs.ScopeSpans, ss)
		env.ResourceSpans = append(env.ResourceSpans, rs)
	}
	return env
}

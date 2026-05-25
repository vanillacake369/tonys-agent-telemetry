// Package ollama implements a telemetry.ProviderIngestor for the Ollama
// local model runtime. Ollama exposes no per-request counters; this
// adapter polls /api/ps to track which models are currently loaded and
// emits one Span when a model first appears in the running set.
//
// Coarse-grained by necessity. Per-call token counts are available only in
// individual /api/chat response bodies, which Ollama doesn't broadcast.
// Users who need finer telemetry can run OpenLLMetry-style instrumentation
// inside their client code; the OTLP receiver picks those spans up.
package ollama

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// DefaultBaseURL is Ollama's default listen address.
const DefaultBaseURL = "http://127.0.0.1:11434"

// Ingestor polls Ollama for running-model snapshots.
type Ingestor struct {
	BaseURL      string
	PollInterval time.Duration

	httpClient *http.Client
}

// New returns an Ingestor with the default endpoint and a sane HTTP client.
func New() *Ingestor {
	return &Ingestor{
		BaseURL:      DefaultBaseURL,
		PollInterval: 5 * time.Second,
		httpClient:   &http.Client{Timeout: 2 * time.Second},
	}
}

// ProviderID returns "ollama".
func (i *Ingestor) ProviderID() string { return "ollama" }

// Detect issues a brief GET /api/tags; a 200 response with a JSON body
// containing the expected "models" key confirms Ollama.
func (i *Ingestor) Detect(ctx context.Context) bool {
	rctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, i.BaseURL+"/api/tags", nil)
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
	if resp.StatusCode != 200 {
		return false
	}
	// Use a pointer so we can distinguish "missing key" (nil) from "empty
	// array" (non-nil with len 0). Non-Ollama JSON servers that return 200
	// won't have a "models" field at all.
	var probe struct {
		Models *[]json.RawMessage `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&probe); err != nil {
		return false
	}
	return probe.Models != nil
}

// Ingest polls /api/ps until ctx is cancelled. Emits one Span the first
// time a model appears as loaded.
func (i *Ingestor) Ingest(ctx context.Context, out chan<- telemetry.Span) error {
	ticker := time.NewTicker(i.PollInterval)
	defer ticker.Stop()

	seen := make(map[string]time.Time)
	var mu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		ps, err := i.fetchPS(ctx)
		if err != nil {
			continue
		}
		now := time.Now()
		for _, m := range ps.Models {
			if m.Model == "" {
				continue
			}
			mu.Lock()
			_, already := seen[m.Model]
			if !already {
				seen[m.Model] = now
			}
			mu.Unlock()
			if already {
				continue
			}
			sp := telemetry.Span{
				TraceID:   "ollama-" + m.Model,
				SpanID:    fmt.Sprintf("ollama-%s-%d", m.Model, now.UnixNano()),
				System:    "ollama",
				Model:     m.Model,
				StartTime: now,
				Status:    "running",
				Attrs: map[string]string{
					"gen_ai.operation.name": "chat",
					"ollama.size_bytes":     fmt.Sprintf("%d", m.SizeBytes),
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

func (i *Ingestor) fetchPS(ctx context.Context) (*psResponse, error) {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, i.BaseURL+"/api/ps", nil)
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
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var out psResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

type psResponse struct {
	Models []psModel `json:"models"`
}

type psModel struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	SizeBytes int64  `json:"size"`
}

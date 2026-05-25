package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

func TestIngestor_ProviderID(t *testing.T) {
	if got := New().ProviderID(); got != "ollama" {
		t.Errorf("ProviderID = %q, want ollama", got)
	}
}

func TestIngestor_DetectOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/tags") {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:latest","model":"llama3:latest"}]}`))
	}))
	defer srv.Close()

	ing := New()
	ing.BaseURL = srv.URL
	if !ing.Detect(context.Background()) {
		t.Error("Detect = false, want true")
	}
}

func TestIngestor_DetectNonOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"something":"else"}`))
	}))
	defer srv.Close()

	ing := New()
	ing.BaseURL = srv.URL
	if ing.Detect(context.Background()) {
		t.Error("Detect = true, want false for non-Ollama JSON")
	}
}

func TestIngestor_DetectUnreachable(t *testing.T) {
	ing := New()
	ing.BaseURL = "http://127.0.0.1:1"
	if ing.Detect(context.Background()) {
		t.Error("Detect = true for unreachable endpoint")
	}
}

func TestIngest_EmitsSpanForNewModel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3","model":"llama3","size":4700000000}]}`))
	}))
	defer srv.Close()

	ing := New()
	ing.BaseURL = srv.URL
	ing.PollInterval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan telemetry.Span, 4)
	done := make(chan struct{})
	go func() { _ = ing.Ingest(ctx, out); close(done) }()

	var got telemetry.Span
	select {
	case got = <-out:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no span emitted")
	}
	cancel()
	<-done

	if got.System != "ollama" || got.Model != "llama3" {
		t.Errorf("span = %+v, want ollama/llama3", got)
	}
	if got.TraceID != "ollama-llama3" {
		t.Errorf("TraceID = %q, want ollama-llama3", got.TraceID)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.Attrs["ollama.size_bytes"] != "4700000000" {
		t.Errorf("size_bytes attr = %q", got.Attrs["ollama.size_bytes"])
	}
}

func TestIngest_DedupesSameModelAcrossPolls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"x","model":"x"}]}`))
	}))
	defer srv.Close()

	ing := New()
	ing.BaseURL = srv.URL
	ing.PollInterval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan telemetry.Span, 8)
	done := make(chan struct{})
	go func() { _ = ing.Ingest(ctx, out); close(done) }()

	// Wait long enough for several polls.
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	// Drain channel and count.
	count := 0
	for {
		select {
		case <-out:
			count++
		default:
			if count != 1 {
				t.Errorf("got %d spans, want 1 (dedup across polls)", count)
			}
			return
		}
	}
}

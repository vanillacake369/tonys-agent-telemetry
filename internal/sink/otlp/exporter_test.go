package otlp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

func TestEncodeTracesJSON_GroupsBySystem(t *testing.T) {
	batch := []telemetry.Span{
		{TraceID: "t1", SpanID: "a", System: "anthropic", Model: "claude-sonnet-4-6", InputTokens: 100, OutputTokens: 50},
		{TraceID: "t2", SpanID: "b", System: "openai", Model: "gpt-4o"},
		{TraceID: "t3", SpanID: "c", System: "anthropic"},
	}
	out := EncodeTracesJSON(batch)
	body, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(body)

	// Each system becomes its own resource_spans entry.
	if !strings.Contains(s, `"anthropic"`) || !strings.Contains(s, `"openai"`) {
		t.Errorf("missing system: %s", s)
	}
	if !strings.Contains(s, `"gen_ai.request.model"`) {
		t.Errorf("model attribute not encoded: %s", s)
	}
	if !strings.Contains(s, `"intValue":"100"`) {
		t.Errorf("input_tokens not encoded: %s", s)
	}
}

func TestExporter_PostsBatch(t *testing.T) {
	var received atomic.Int32
	var mu sync.Mutex
	var lastBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		mu.Lock()
		lastBody = b
		mu.Unlock()
		received.Add(1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	e := New(srv.URL)
	e.BatchSize = 2
	e.BatchTimeout = 1 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	in := make(chan telemetry.Span, 4)
	done := make(chan struct{})
	go func() { _ = e.Run(ctx, in); close(done) }()

	in <- telemetry.Span{TraceID: "t1", SpanID: "a", System: "anthropic"}
	in <- telemetry.Span{TraceID: "t1", SpanID: "b", System: "anthropic"}
	// BatchSize=2 → flush should fire.

	deadline := time.After(500 * time.Millisecond)
	for received.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("no POST received within 500ms")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	<-done

	mu.Lock()
	body := lastBody
	mu.Unlock()
	if !strings.Contains(string(body), `"traceId":"t1"`) {
		t.Errorf("body missing trace data: %s", body)
	}
}

func TestExporter_FlushesOnTimeout(t *testing.T) {
	var received atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
		received.Add(1)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	e := New(srv.URL)
	e.BatchSize = 100
	e.BatchTimeout = 40 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := make(chan telemetry.Span, 4)
	done := make(chan struct{})
	go func() { _ = e.Run(ctx, in); close(done) }()

	in <- telemetry.Span{TraceID: "t", SpanID: "a", System: "sys"}
	// Wait > BatchTimeout — should auto-flush.
	time.Sleep(120 * time.Millisecond)

	if received.Load() == 0 {
		t.Error("expected at least one POST from timeout flush")
	}
	cancel()
	<-done
}

func TestExporter_FailureIsSilent(t *testing.T) {
	// Unreachable URL — exporter must not panic or block.
	e := New("http://127.0.0.1:1/v1/traces")
	e.BatchSize = 1
	e.BatchTimeout = 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	in := make(chan telemetry.Span, 1)
	done := make(chan struct{})
	go func() { _ = e.Run(ctx, in); close(done) }()
	in <- telemetry.Span{TraceID: "t", SpanID: "a"}

	<-done // must exit on ctx, not hang
}

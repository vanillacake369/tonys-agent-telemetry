package vllm

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
	if got := New().ProviderID(); got != "vllm" {
		t.Errorf("ProviderID = %q, want vllm", got)
	}
}

func TestParsePrometheus_BasicLine(t *testing.T) {
	body := strings.NewReader(`# HELP vllm:prompt_tokens_total Total prompt tokens
# TYPE vllm:prompt_tokens_total counter
vllm:prompt_tokens_total{model_name="llama-3"} 1234
vllm:generation_tokens_total{model_name="llama-3"} 567

other:metric 42
`)
	out := parsePrometheus(body)
	if len(out) != 3 {
		t.Fatalf("got %d metrics, want 3", len(out))
	}
	if out[0].name != "vllm:prompt_tokens_total" || out[0].labels["model_name"] != "llama-3" || out[0].value != 1234 {
		t.Errorf("first metric mis-parsed: %+v", out[0])
	}
	if out[2].name != "other:metric" || out[2].value != 42 {
		t.Errorf("third metric mis-parsed: %+v", out[2])
	}
}

func TestParsePrometheus_MalformedSkipped(t *testing.T) {
	body := strings.NewReader(`not-a-metric
vllm:ok{m="x"} 1
also nonsense
`)
	out := parsePrometheus(body)
	// "not-a-metric" has one field only — skipped.
	// "vllm:ok ..." parsed.
	// "also nonsense" — "also" + "nonsense", and "nonsense" is not a number → skipped.
	if len(out) != 1 {
		t.Fatalf("got %d metrics, want 1: %+v", len(out), out)
	}
	if out[0].name != "vllm:ok" {
		t.Errorf("name = %q", out[0].name)
	}
}

func TestIngestor_DetectVLLMServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("# HELP vllm:prompt_tokens_total\nvllm:prompt_tokens_total 0\n"))
	}))
	defer srv.Close()
	ing := New()
	ing.Endpoint = srv.URL
	if !ing.Detect(context.Background()) {
		t.Error("Detect = false, want true")
	}
}

func TestIngestor_DetectNonVLLM(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("# Some other metrics server\nother:counter 0\n"))
	}))
	defer srv.Close()
	ing := New()
	ing.Endpoint = srv.URL
	if ing.Detect(context.Background()) {
		t.Error("Detect = true for non-vLLM server")
	}
}

func TestIngestor_DetectUnreachable(t *testing.T) {
	ing := New()
	ing.Endpoint = "http://127.0.0.1:1" // closed port
	if ing.Detect(context.Background()) {
		t.Error("Detect = true for unreachable endpoint")
	}
}

func TestIngest_EmitsSpanOnDelta(t *testing.T) {
	// Server returns increasing counters on each request.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		prompt := calls * 100
		gen := calls * 50
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("# vllm metrics\n" +
			"vllm:prompt_tokens_total{model_name=\"m1\"} " + itoa(prompt) + "\n" +
			"vllm:generation_tokens_total{model_name=\"m1\"} " + itoa(gen) + "\n"))
	}))
	defer srv.Close()

	ing := New()
	ing.Endpoint = srv.URL
	ing.PollInterval = 30 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan telemetry.Span, 4)
	done := make(chan struct{})
	go func() { _ = ing.Ingest(ctx, out); close(done) }()

	var got telemetry.Span
	select {
	case got = <-out:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no span emitted within 500ms")
	}
	cancel()
	<-done

	if got.System != "vllm" || got.Model != "m1" {
		t.Errorf("span = %+v, want System=vllm Model=m1", got)
	}
	if got.InputTokens <= 0 || got.OutputTokens <= 0 {
		t.Errorf("delta tokens = %d/%d, want > 0", got.InputTokens, got.OutputTokens)
	}
	if got.TraceID != "vllm-m1" {
		t.Errorf("TraceID = %q, want vllm-m1", got.TraceID)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

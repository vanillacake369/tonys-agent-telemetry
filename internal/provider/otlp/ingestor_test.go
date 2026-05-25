package otlp

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestResolveBindAddr_Default asserts that when TONYS_OTLP_BIND is unset,
// the resolved bind address defaults to localhost-only (127.0.0.1:4318),
// never 0.0.0.0. See PIVOT_PLAN.md security gate (QA finding #3, P0).
func TestResolveBindAddr_Default(t *testing.T) {
	t.Setenv("TONYS_OTLP_BIND", "")
	got := resolveBindAddr()
	if got != "127.0.0.1:4318" {
		t.Errorf("resolveBindAddr() = %q, want 127.0.0.1:4318", got)
	}
}

// TestResolveBindAddr_EnvOverride asserts that TONYS_OTLP_BIND, when set,
// overrides the default — allowing operators to opt in to a wider bind scope.
func TestResolveBindAddr_EnvOverride(t *testing.T) {
	t.Setenv("TONYS_OTLP_BIND", "0.0.0.0:14318")
	got := resolveBindAddr()
	if got != "0.0.0.0:14318" {
		t.Errorf("resolveBindAddr() = %q, want 0.0.0.0:14318", got)
	}
}

func TestIngestor_ProviderID(t *testing.T) {
	if got := New().ProviderID(); got != "otlp-http" {
		t.Errorf("ProviderID = %q, want otlp-http", got)
	}
}

func TestIngestor_DetectAvailable(t *testing.T) {
	ing := &Ingestor{Addr: ":0"} // OS-assigned free port
	if !ing.Detect(context.Background()) {
		t.Error("Detect = false, expected true for free port")
	}
}

func TestIngestor_DetectUnavailable(t *testing.T) {
	// Bind a port and keep it occupied.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ing := &Ingestor{Addr: ln.Addr().String()}
	if ing.Detect(context.Background()) {
		t.Error("Detect = true, expected false for bound port")
	}
}

func TestParseTracesJSON_GenAISpan(t *testing.T) {
	payload := []byte(`{
		"resourceSpans": [{
			"resource": {"attributes": [
				{"key":"service.name","value":{"stringValue":"langgraph-agent"}}
			]},
			"scopeSpans": [{
				"spans": [{
					"traceId":"5b8aa5a2d2c872e8321cf37308d69df2",
					"spanId":"051581bf3cb55c13",
					"parentSpanId":"",
					"name":"chat",
					"startTimeUnixNano":"1700000000000000000",
					"endTimeUnixNano":"1700000001500000000",
					"attributes":[
						{"key":"gen_ai.system","value":{"stringValue":"openai"}},
						{"key":"gen_ai.request.model","value":{"stringValue":"gpt-4o-mini"}},
						{"key":"gen_ai.usage.input_tokens","value":{"intValue":"150"}},
						{"key":"gen_ai.usage.output_tokens","value":{"intValue":"45"}}
					],
					"status":{"code":1}
				}]
			}]
		}]
	}`)

	spans, err := ParseTracesJSON(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	s := spans[0]
	if s.TraceID != "5b8aa5a2d2c872e8321cf37308d69df2" || s.SpanID != "051581bf3cb55c13" {
		t.Errorf("ids: %q / %q", s.TraceID, s.SpanID)
	}
	if s.System != "openai" || s.Model != "gpt-4o-mini" {
		t.Errorf("system/model: %q / %q", s.System, s.Model)
	}
	if s.InputTokens != 150 || s.OutputTokens != 45 {
		t.Errorf("tokens: %d / %d, want 150 / 45", s.InputTokens, s.OutputTokens)
	}
	if s.Status != "done" {
		t.Errorf("Status = %q, want done", s.Status)
	}
	if s.Attrs["service.name"] != "langgraph-agent" {
		t.Errorf("resource attr lost: %v", s.Attrs)
	}
	if s.Attrs["gen_ai.operation.name"] != "chat" {
		t.Errorf("operation.name = %q, want chat", s.Attrs["gen_ai.operation.name"])
	}
	wantStart := time.Unix(0, 1700000000000000000)
	if !s.StartTime.Equal(wantStart) {
		t.Errorf("StartTime = %v, want %v", s.StartTime, wantStart)
	}
}

func TestParseTracesJSON_ErrorStatus(t *testing.T) {
	payload := []byte(`{
		"resourceSpans":[{"scopeSpans":[{"spans":[{
			"traceId":"a","spanId":"b","status":{"code":2}
		}]}]}]
	}`)
	spans, err := ParseTracesJSON(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spans) != 1 || spans[0].Status != "error" {
		t.Errorf("got %+v, want Status=error", spans)
	}
}

func TestParseTracesJSON_Empty(t *testing.T) {
	spans, err := ParseTracesJSON([]byte(`{"resourceSpans":[]}`))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(spans) != 0 {
		t.Errorf("got %d spans, want 0", len(spans))
	}
}

func TestParseTracesJSON_Malformed(t *testing.T) {
	if _, err := ParseTracesJSON([]byte(`{not json`)); err == nil {
		t.Error("want error on malformed JSON")
	}
}

func TestIngest_AcceptsValidExport(t *testing.T) {
	// Pick a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	ing := &Ingestor{Addr: addr}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan telemetry.Span, 4)
	done := make(chan struct{})
	go func() { _ = ing.Ingest(ctx, out); close(done) }()

	// Wait for server to start (best-effort retry).
	deadline := time.Now().Add(500 * time.Millisecond)
	var resp *http.Response
	payload := []byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[{
		"traceId":"t1","spanId":"s1","name":"op",
		"attributes":[{"key":"gen_ai.system","value":{"stringValue":"vllm"}}]
	}]}]}]}`)
	for time.Now().Before(deadline) {
		var perr error
		resp, perr = http.Post("http://"+addr+"/v1/traces", "application/json", bytes.NewReader(payload))
		if perr == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if resp == nil {
		t.Fatal("server never became reachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	select {
	case sp := <-out:
		if sp.TraceID != "t1" || sp.SpanID != "s1" || sp.System != "vllm" {
			t.Errorf("unexpected span: %+v", sp)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("no span emitted")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down on cancel")
	}
}

// TestIngest_DoubleStartReturnsError verifies that starting two Ingestor.Ingest
// goroutines concurrently on the same bind address results in exactly one
// returning a non-nil error within 200ms (TOCTOU / double-bind protection).
//
// TONYS_OTLP_BIND is used to pick a deterministic free port so the test does
// not interfere with a real OTLP receiver that might be using 4318.
func TestIngest_DoubleStartReturnsError(t *testing.T) {
	// Grab a free ephemeral port and immediately release it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	t.Setenv("TONYS_OTLP_BIND", addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out1 := make(chan telemetry.Span, 1)
	out2 := make(chan telemetry.Span, 1)
	errCh := make(chan error, 2)

	go func() {
		ing := &Ingestor{Addr: addr}
		errCh <- ing.Ingest(ctx, out1)
	}()
	go func() {
		ing := &Ingestor{Addr: addr}
		errCh <- ing.Ingest(ctx, out2)
	}()

	// At least one goroutine must return a non-nil (non-context) error within
	// 200ms because both try to bind the same port.
	deadline := time.NewTimer(200 * time.Millisecond)
	defer deadline.Stop()

	var gotErr error
	select {
	case e := <-errCh:
		gotErr = e
	case <-deadline.C:
	}

	// Cancel so the surviving goroutine also returns.
	cancel()

	if gotErr == nil {
		// The first return might have been from ctx cancellation rather than a
		// bind error. Drain the second result with a brief timeout.
		select {
		case e := <-errCh:
			gotErr = e
		case <-time.After(2 * time.Second):
		}
	}

	if gotErr == nil {
		t.Fatal("expected at least one Ingest to return a non-nil error for double-bind; got nil from both")
	}
	// The error must NOT be a context cancellation — it must be an address-in-use error.
	if gotErr == context.Canceled || gotErr == context.DeadlineExceeded {
		t.Errorf("expected bind error, got context error: %v", gotErr)
	}
}

func TestIngest_RejectsNonPost(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()

	ing := &Ingestor{Addr: addr}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan telemetry.Span, 1)
	go func() { _ = ing.Ingest(ctx, out) }()
	time.Sleep(60 * time.Millisecond)

	resp, err := http.Get("http://" + addr + "/v1/traces")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

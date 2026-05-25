// Package telemetry_test — Beta Quality Gate: multi-provider canonical Span normalization.
//
// This file proves that all four provider ingestors emit telemetry.Span values
// that share a canonical schema sufficient for Swarm DAG reconstruction via
// BuildForests. Providers that lack inherent parent linkage (vllm, ollama) are
// asserted as "leaf-only" and explicitly documented rather than silently skipped.
//
// The test lives in the external test package (telemetry_test) to avoid an
// import cycle: providers import telemetry, so placing provider imports inside
// package telemetry itself would be circular.
package telemetry_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	claudecode "github.com/vanillacake369/tonys-agent-telemetry/internal/provider/claudecode"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider/ollama"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider/otlp"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider/vllm"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// ---------------------------------------------------------------------------
// Provider: claudecode
// ---------------------------------------------------------------------------

// TestNormalization_ClaudeCode_CanonicalFields verifies that ConvertHookPayload
// populates all fields required for DAG reconstruction: TraceID, SpanID,
// ParentSpanID, System, Model, tokens, operation name, and status.
func TestNormalization_ClaudeCode_CanonicalFields(t *testing.T) {
	raw := []byte(`{
		"sessionId":  "session-cc-001",
		"uuid":       "span-cc-001",
		"parentUuid": "span-cc-000",
		"type":       "assistant",
		"cwd":        "/workspace/myproject",
		"gitBranch":  "main",
		"timestamp":  "2026-05-25T10:00:00Z",
		"message": {
			"model": "claude-sonnet-4-6",
			"usage": {"input_tokens": 512, "output_tokens": 128}
		}
	}`)

	sp, err := claudecode.ConvertHookPayload("PostToolUse", raw)
	if err != nil {
		t.Fatalf("claudecode: ConvertHookPayload error: %v", err)
	}

	// Identity fields required for DAG reconstruction.
	if sp.TraceID == "" {
		t.Error("claudecode: TraceID is empty")
	}
	if sp.SpanID == "" {
		t.Error("claudecode: SpanID is empty")
	}
	if sp.ParentSpanID == "" {
		t.Error("claudecode: ParentSpanID is empty (expected non-empty when parentUuid set)")
	}
	if sp.TraceID != "session-cc-001" {
		t.Errorf("claudecode: TraceID = %q, want session-cc-001", sp.TraceID)
	}
	if sp.SpanID != "span-cc-001" {
		t.Errorf("claudecode: SpanID = %q, want span-cc-001", sp.SpanID)
	}
	if sp.ParentSpanID != "span-cc-000" {
		t.Errorf("claudecode: ParentSpanID = %q, want span-cc-000", sp.ParentSpanID)
	}

	// System is hardcoded to "anthropic" by the converter.
	if sp.System != "anthropic" {
		t.Errorf("claudecode: System = %q, want anthropic", sp.System)
	}

	// Model populated from message.model.
	if sp.Model == "" {
		t.Error("claudecode: Model is empty")
	}
	if sp.Model != "claude-sonnet-4-6" {
		t.Errorf("claudecode: Model = %q, want claude-sonnet-4-6", sp.Model)
	}

	// Token counts.
	if sp.InputTokens == 0 {
		t.Error("claudecode: InputTokens is 0; expected non-zero from fixture")
	}
	if sp.OutputTokens == 0 {
		t.Error("claudecode: OutputTokens is 0; expected non-zero from fixture")
	}

	// Status set.
	if sp.Status == "" {
		t.Error("claudecode: Status is empty")
	}
	if sp.Status != "done" {
		t.Errorf("claudecode: Status = %q, want done", sp.Status)
	}

	// Operation name lands in Attrs under the OTel semconv key.
	if sp.Attrs["gen_ai.operation.name"] == "" {
		t.Error("claudecode: Attrs[gen_ai.operation.name] is empty")
	}

	// Tool name flows to gen_ai.tool.name when present — tested via ToolName field.
	rawTool := []byte(`{
		"sessionId": "s", "uuid": "u", "parentUuid": "",
		"type": "queue-operation", "tool_name": "Bash"
	}`)
	spTool, _ := claudecode.ConvertHookPayload("PreToolUse", rawTool)
	if spTool.Attrs["gen_ai.tool.name"] != "Bash" {
		t.Errorf("claudecode: tool.name attr = %q, want Bash", spTool.Attrs["gen_ai.tool.name"])
	}
}

// TestNormalization_ClaudeCode_RootSpanNoParent confirms that a root span
// (parentUuid = "") correctly leaves ParentSpanID empty.
func TestNormalization_ClaudeCode_RootSpanNoParent(t *testing.T) {
	raw := []byte(`{"sessionId":"s","uuid":"root","parentUuid":"","type":"assistant"}`)
	sp, err := claudecode.ConvertHookPayload("SessionStart", raw)
	if err != nil {
		t.Fatalf("claudecode: ConvertHookPayload error: %v", err)
	}
	if sp.ParentSpanID != "" {
		t.Errorf("claudecode: root span has ParentSpanID = %q, want empty", sp.ParentSpanID)
	}
	if sp.TraceID == "" || sp.SpanID == "" {
		t.Errorf("claudecode: root span missing ids: TraceID=%q SpanID=%q", sp.TraceID, sp.SpanID)
	}
}

// ---------------------------------------------------------------------------
// Provider: otlp
// ---------------------------------------------------------------------------

// TestNormalization_OTLP_CanonicalFields verifies that ParseTracesJSON maps
// OTLP wire fields onto the canonical Span schema including explicit parent.
func TestNormalization_OTLP_CanonicalFields(t *testing.T) {
	payload := []byte(`{
		"resourceSpans": [{
			"resource": {"attributes": [
				{"key":"service.name","value":{"stringValue":"test-agent"}}
			]},
			"scopeSpans": [{
				"spans": [{
					"traceId":       "aabbccddeeff00112233445566778899",
					"spanId":        "0011223344556677",
					"parentSpanId":  "ffeeddccbbaa9988",
					"name":          "llm.chat",
					"startTimeUnixNano": "1700000000000000000",
					"endTimeUnixNano":   "1700000002000000000",
					"attributes": [
						{"key":"gen_ai.system",        "value":{"stringValue":"openai"}},
						{"key":"gen_ai.request.model", "value":{"stringValue":"gpt-4o"}},
						{"key":"gen_ai.usage.input_tokens",  "value":{"intValue":"300"}},
						{"key":"gen_ai.usage.output_tokens", "value":{"intValue":"80"}}
					],
					"status": {"code": 1}
				}]
			}]
		}]
	}`)

	spans, err := otlp.ParseTracesJSON(payload)
	if err != nil {
		t.Fatalf("otlp: ParseTracesJSON error: %v", err)
	}
	if len(spans) != 1 {
		t.Fatalf("otlp: got %d spans, want 1", len(spans))
	}
	sp := spans[0]

	if sp.TraceID == "" {
		t.Error("otlp: TraceID is empty")
	}
	if sp.SpanID == "" {
		t.Error("otlp: SpanID is empty")
	}
	if sp.ParentSpanID == "" {
		t.Error("otlp: ParentSpanID is empty (expected non-empty from fixture)")
	}
	if sp.TraceID != "aabbccddeeff00112233445566778899" {
		t.Errorf("otlp: TraceID = %q", sp.TraceID)
	}
	if sp.SpanID != "0011223344556677" {
		t.Errorf("otlp: SpanID = %q", sp.SpanID)
	}
	if sp.ParentSpanID != "ffeeddccbbaa9988" {
		t.Errorf("otlp: ParentSpanID = %q, want ffeeddccbbaa9988", sp.ParentSpanID)
	}

	// System promoted from gen_ai.system.
	if sp.System != "openai" {
		t.Errorf("otlp: System = %q, want openai", sp.System)
	}
	// Model promoted from gen_ai.request.model.
	if sp.Model != "gpt-4o" {
		t.Errorf("otlp: Model = %q, want gpt-4o", sp.Model)
	}
	// Token counts promoted.
	if sp.InputTokens != 300 {
		t.Errorf("otlp: InputTokens = %d, want 300", sp.InputTokens)
	}
	if sp.OutputTokens != 80 {
		t.Errorf("otlp: OutputTokens = %d, want 80", sp.OutputTokens)
	}
	// Status.
	if sp.Status == "" {
		t.Error("otlp: Status is empty")
	}
	// Operation name from span.Name fallback.
	if sp.Attrs["gen_ai.operation.name"] == "" {
		t.Error("otlp: Attrs[gen_ai.operation.name] is empty; expected span.Name fallback")
	}
	// Timing set.
	if sp.StartTime.IsZero() {
		t.Error("otlp: StartTime is zero")
	}
	if sp.EndTime.IsZero() {
		t.Error("otlp: EndTime is zero")
	}
}

// TestNormalization_OTLP_ErrorStatus confirms status code 2 maps to "error".
func TestNormalization_OTLP_ErrorStatus(t *testing.T) {
	payload := []byte(`{"resourceSpans":[{"scopeSpans":[{"spans":[{
		"traceId":"t","spanId":"s","status":{"code":2}
	}]}]}]}`)
	spans, _ := otlp.ParseTracesJSON(payload)
	if len(spans) != 1 || spans[0].Status != "error" {
		t.Errorf("otlp: error status: got %+v, want Status=error", spans)
	}
}

// ---------------------------------------------------------------------------
// Provider: vllm  (leaf-only — no parent linkage)
// ---------------------------------------------------------------------------

// TestNormalization_VLLM_CanonicalFields drives the vllm ingestor with a mock
// Prometheus endpoint and asserts canonical fields. Parent linkage is absent
// by design; this is acknowledged and documented.
func TestNormalization_VLLM_CanonicalFields(t *testing.T) {
	// First call: baseline counters. Second call: incremented counters that
	// produce a non-zero delta and trigger span emission.
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		tokens := calls * 200
		gen := calls * 100
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w,
			"# HELP vllm:prompt_tokens_total Total prompt tokens\n"+
				"# TYPE vllm:prompt_tokens_total counter\n"+
				"vllm:prompt_tokens_total{model_name=\"mistral-7b\"} %d\n"+
				"vllm:generation_tokens_total{model_name=\"mistral-7b\"} %d\n",
			tokens, gen)
	}))
	defer srv.Close()

	ing := vllm.New()
	ing.Endpoint = srv.URL
	ing.PollInterval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan telemetry.Span, 4)
	done := make(chan struct{})
	go func() { _ = ing.Ingest(ctx, out); close(done) }()

	var sp telemetry.Span
	select {
	case sp = <-out:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("vllm: no span emitted within 500ms")
	}
	cancel()
	<-done

	// Identity fields required by canonical schema.
	if sp.TraceID == "" {
		t.Error("vllm: TraceID is empty")
	}
	if sp.SpanID == "" {
		t.Error("vllm: SpanID is empty")
	}
	if sp.System != "vllm" {
		t.Errorf("vllm: System = %q, want vllm", sp.System)
	}
	if sp.Model == "" {
		t.Error("vllm: Model is empty")
	}
	if sp.InputTokens <= 0 {
		t.Errorf("vllm: InputTokens = %d, want > 0", sp.InputTokens)
	}
	if sp.OutputTokens <= 0 {
		t.Errorf("vllm: OutputTokens = %d, want > 0", sp.OutputTokens)
	}
	if sp.Status != "done" {
		t.Errorf("vllm: Status = %q, want done", sp.Status)
	}
	if sp.Attrs["gen_ai.operation.name"] == "" {
		t.Error("vllm: Attrs[gen_ai.operation.name] is empty")
	}

	// ACKNOWLEDGED GAP: vllm Prometheus scraping is aggregate per model; there
	// is no per-request trace structure. ParentSpanID is always empty and
	// TraceID is synthetic ("vllm-<model>"). These spans are leaf-only in a
	// multi-provider DAG — they do not link into the parent hierarchy.
	t.Logf("vllm: leaf-only, no parent linkage — ParentSpanID=%q TraceID=%q (synthetic). "+
		"Per-request traces require vLLM's opt-in OTel SDK integration "+
		"(then the otlp provider receives them instead).", sp.ParentSpanID, sp.TraceID)

	if sp.ParentSpanID != "" {
		t.Errorf("vllm: expected empty ParentSpanID for aggregate scrape span, got %q", sp.ParentSpanID)
	}
}

// ---------------------------------------------------------------------------
// Provider: ollama  (leaf-only — no parent linkage)
// ---------------------------------------------------------------------------

// TestNormalization_Ollama_CanonicalFields drives the ollama ingestor with a
// mock /api/ps endpoint and asserts canonical fields. Parent linkage is absent
// by design; this is acknowledged and documented.
func TestNormalization_Ollama_CanonicalFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/api/ps") {
			fmt.Fprint(w, `{"models":[{"name":"llama3:latest","model":"llama3:latest","size":4700000000}]}`)
			return
		}
		// /api/tags for Detect
		fmt.Fprint(w, `{"models":[{"name":"llama3:latest"}]}`)
	}))
	defer srv.Close()

	ing := ollama.New()
	ing.BaseURL = srv.URL
	ing.PollInterval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan telemetry.Span, 4)
	done := make(chan struct{})
	go func() { _ = ing.Ingest(ctx, out); close(done) }()

	var sp telemetry.Span
	select {
	case sp = <-out:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ollama: no span emitted within 500ms")
	}
	cancel()
	<-done

	// Identity fields.
	if sp.TraceID == "" {
		t.Error("ollama: TraceID is empty")
	}
	if sp.SpanID == "" {
		t.Error("ollama: SpanID is empty")
	}
	if sp.System != "ollama" {
		t.Errorf("ollama: System = %q, want ollama", sp.System)
	}
	if sp.Model == "" {
		t.Error("ollama: Model is empty")
	}
	if sp.Status != "running" {
		t.Errorf("ollama: Status = %q, want running", sp.Status)
	}
	if sp.Attrs["gen_ai.operation.name"] == "" {
		t.Error("ollama: Attrs[gen_ai.operation.name] is empty")
	}

	// ACKNOWLEDGED GAP (P1): ollama /api/ps reports model-load events only;
	// there is no per-request tracing or token count. TraceID is synthetic
	// ("ollama-<model>") and ParentSpanID is always empty. Token counts
	// (InputTokens, OutputTokens) remain zero — confirmed intentional per the
	// package comment ("per-call token counts are available only in individual
	// /api/chat response bodies"). These spans are leaf-only in a multi-provider
	// DAG and cannot be linked to a parent call hierarchy without client-side
	// instrumentation forwarded via the otlp provider.
	t.Logf("ollama: leaf-only, no parent linkage — ParentSpanID=%q TraceID=%q (synthetic). "+
		"P1 gap: InputTokens=%d OutputTokens=%d (both zero by design — poll-based).",
		sp.ParentSpanID, sp.TraceID, sp.InputTokens, sp.OutputTokens)

	if sp.ParentSpanID != "" {
		t.Errorf("ollama: expected empty ParentSpanID for model-load span, got %q", sp.ParentSpanID)
	}
	// Token counts are expected to be zero (poll-based, no per-request data).
	if sp.InputTokens != 0 || sp.OutputTokens != 0 {
		t.Errorf("ollama: unexpected tokens %d/%d; expected 0/0 for /api/ps poll span",
			sp.InputTokens, sp.OutputTokens)
	}
}

// ---------------------------------------------------------------------------
// Beta Quality Gate: mixed-provider single-trace topology
// ---------------------------------------------------------------------------

// TestNormalization_MixedProvider_SingleTraceTopology is THE key beta gate.
//
// It constructs a single shared traceID where:
//   - claudecode contributes the root span (parentUuid = "")
//   - otlp contributes two children that reference the claudecode root SpanID
//
// After BuildForests the resulting forest must have:
//   - exactly 1 root for the shared trace (the claudecode root)
//   - exactly 2 children on that root (the otlp child spans)
//   - zero orphans within the shared trace
//
// This proves canonical schema compatibility: ClaudeCode's SpanID and OTLP's
// parentSpanId round-trip correctly through BuildForests.
func TestNormalization_MixedProvider_SingleTraceTopology(t *testing.T) {
	const sharedTraceID = "trace-mixed-provider-001"
	const rootSpanID = "span-root-from-claudecode"

	// --- Root span from claudecode ---
	ccRaw := []byte(`{
		"sessionId":  "` + sharedTraceID + `",
		"uuid":       "` + rootSpanID + `",
		"parentUuid": "",
		"type":       "assistant",
		"timestamp":  "2026-05-25T10:00:00Z",
		"message": {"model": "claude-sonnet-4-6", "usage": {"input_tokens": 800, "output_tokens": 200}}
	}`)
	rootSpan, err := claudecode.ConvertHookPayload("SessionStart", ccRaw)
	if err != nil {
		t.Fatalf("mixed: claudecode root span error: %v", err)
	}

	// Sanity-check the root span before building the forest.
	if rootSpan.TraceID != sharedTraceID {
		t.Fatalf("mixed: root TraceID = %q, want %q", rootSpan.TraceID, sharedTraceID)
	}
	if rootSpan.SpanID != rootSpanID {
		t.Fatalf("mixed: root SpanID = %q, want %q", rootSpan.SpanID, rootSpanID)
	}
	if rootSpan.ParentSpanID != "" {
		t.Fatalf("mixed: root ParentSpanID = %q, want empty", rootSpan.ParentSpanID)
	}

	// --- Child spans from otlp that reference the claudecode root ---
	otlpPayload := []byte(`{
		"resourceSpans": [{
			"scopeSpans": [{
				"spans": [
					{
						"traceId":      "` + sharedTraceID + `",
						"spanId":       "span-otlp-child-001",
						"parentSpanId": "` + rootSpanID + `",
						"name":         "tool.bash",
						"startTimeUnixNano": "1700000001000000000",
						"endTimeUnixNano":   "1700000002000000000",
						"attributes": [
							{"key":"gen_ai.system","value":{"stringValue":"anthropic"}},
							{"key":"gen_ai.usage.input_tokens","value":{"intValue":"50"}},
							{"key":"gen_ai.usage.output_tokens","value":{"intValue":"10"}}
						]
					},
					{
						"traceId":      "` + sharedTraceID + `",
						"spanId":       "span-otlp-child-002",
						"parentSpanId": "` + rootSpanID + `",
						"name":         "tool.read_file",
						"startTimeUnixNano": "1700000003000000000",
						"endTimeUnixNano":   "1700000004000000000",
						"attributes": [
							{"key":"gen_ai.system","value":{"stringValue":"anthropic"}}
						]
					}
				]
			}]
		}]
	}`)
	otlpSpans, err := otlp.ParseTracesJSON(otlpPayload)
	if err != nil {
		t.Fatalf("mixed: otlp parse error: %v", err)
	}
	if len(otlpSpans) != 2 {
		t.Fatalf("mixed: got %d otlp spans, want 2", len(otlpSpans))
	}

	// Verify child spans carry the correct parentSpanID before topology check.
	for i, sp := range otlpSpans {
		if sp.TraceID != sharedTraceID {
			t.Errorf("mixed: otlp span[%d] TraceID = %q, want %q", i, sp.TraceID, sharedTraceID)
		}
		if sp.ParentSpanID != rootSpanID {
			t.Errorf("mixed: otlp span[%d] ParentSpanID = %q, want %q", i, sp.ParentSpanID, rootSpanID)
		}
	}

	// --- Build the mixed-provider forest ---
	allSpans := append([]telemetry.Span{rootSpan}, otlpSpans...)
	forests := telemetry.BuildForests(allSpans)

	roots, ok := forests[sharedTraceID]
	if !ok {
		t.Fatalf("mixed: no forest entry for traceID %q", sharedTraceID)
	}

	// There must be exactly 1 root (the claudecode span). If BuildForests
	// promoted any child to root it means the parent reference did not
	// resolve — a canonical schema incompatibility.
	if len(roots) != 1 {
		t.Fatalf("mixed: got %d roots, want 1 — extra roots indicate orphaned spans "+
			"(parent-ID schema mismatch between providers): %v",
			len(roots), rootIDs(roots))
	}

	root := roots[0]
	if root.Span.SpanID != rootSpanID {
		t.Errorf("mixed: forest root SpanID = %q, want %q", root.Span.SpanID, rootSpanID)
	}
	if root.Span.System != "anthropic" {
		t.Errorf("mixed: forest root System = %q, want anthropic", root.Span.System)
	}

	// Root must have exactly 2 children (the two OTLP spans).
	if len(root.Children) != 2 {
		t.Fatalf("mixed: root has %d children, want 2", len(root.Children))
	}

	// Confirm child SpanIDs are the expected OTLP spans.
	childIDs := map[string]bool{}
	for _, child := range root.Children {
		childIDs[child.Span.SpanID] = true
	}
	for _, want := range []string{"span-otlp-child-001", "span-otlp-child-002"} {
		if !childIDs[want] {
			t.Errorf("mixed: expected child span %q not found in forest; child ids: %v", want, childIDs)
		}
	}

	// No orphans: every span in allSpans must appear either as the root or
	// as a descendant under the root — not as an additional root entry.
	totalNodesInForest := 1 + len(root.Children) // root + 2 direct children
	if totalNodesInForest != len(allSpans) {
		t.Errorf("mixed: forest contains %d nodes, input had %d spans — topology mismatch",
			totalNodesInForest, len(allSpans))
	}

	t.Logf("mixed-provider: PASS — 1 claudecode root + %d otlp children under traceID %q; 0 orphans",
		len(root.Children), sharedTraceID)
}

// rootIDs returns a slice of SpanIDs from a []*telemetry.SpanNode slice, for error messages.
func rootIDs(nodes []*telemetry.SpanNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.Span.SpanID
	}
	return ids
}

package signal_test

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// T is a base time used across failed_handoff tests.
var handoffBase = time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)

// handoffForest builds:
//
//	root
//	  ├─ spanA: tool="bash", status=errStatus, EndTime=T+5s
//	  ├─ spanB: tool="bash", status=retryStatus, StartTime=T+retryStartOffset
//	  └─ spanC: tool="read_file", status="done"
func handoffForest(errStatus, retryStatus string, retryStartOffset time.Duration, errorType string) signal.Forest {
	T := handoffBase
	root := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-ho",
			SpanID:    "root",
			StartTime: T,
			EndTime:   T.Add(10 * time.Second),
			Status:    "done",
		},
	}
	attrs := map[string]string{"gen_ai.tool.name": "bash"}
	if errorType != "" {
		attrs["error.type"] = errorType
	}
	spanA := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-ho",
			SpanID:       "span-a",
			ParentSpanID: "root",
			StartTime:    T.Add(1 * time.Second),
			EndTime:      T.Add(5 * time.Second), // ends at T+5s
			Status:       errStatus,
			Attrs:        attrs,
		},
	}
	spanB := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-ho",
			SpanID:       "span-b",
			ParentSpanID: "root",
			StartTime:    T.Add(retryStartOffset), // caller controls when retry starts
			EndTime:      T.Add(10 * time.Second),
			Status:       retryStatus,
			Attrs:        map[string]string{"gen_ai.tool.name": "bash"},
		},
	}
	spanC := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-ho",
			SpanID:       "span-c",
			ParentSpanID: "root",
			StartTime:    T.Add(2 * time.Second),
			EndTime:      T.Add(4 * time.Second),
			Status:       "done",
			Attrs:        map[string]string{"gen_ai.tool.name": "read_file"},
		},
	}
	root.Children = []*telemetry.SpanNode{spanA, spanB, spanC}
	return signal.Forest{"trace-ho": {root}}
}

// TestFailedHandoff_Positive is SIGNALS_SPEC T6: spanA errors, spanB retries
// (StartTime after spanA EndTime). Expect one signal citing span-a and span-b.
func TestFailedHandoff_Positive(t *testing.T) {
	// spanA ends at T+5s; spanB starts at T+6s → after
	forest := handoffForest("error", "done", 6*time.Second, "")
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)

	var ho []signal.Signal
	for _, s := range signals {
		if s.Type == signal.SignalFailedHandoff {
			ho = append(ho, s)
		}
	}
	if len(ho) != 1 {
		t.Fatalf("expected 1 failed_handoff signal, got %d", len(ho))
	}
	h := ho[0]
	if len(h.SpanIDs) != 2 {
		t.Fatalf("SpanIDs = %v, want 2 entries", h.SpanIDs)
	}
	// Verify error_span_id and retry_span_id in evidence.
	if h.Evidence["error_span_id"] != "span-a" {
		t.Errorf("error_span_id = %v, want span-a", h.Evidence["error_span_id"])
	}
	if h.Evidence["retry_span_id"] != "span-b" {
		t.Errorf("retry_span_id = %v, want span-b", h.Evidence["retry_span_id"])
	}
	if h.Evidence["tool_name"] != "bash" {
		t.Errorf("tool_name = %v, want bash", h.Evidence["tool_name"])
	}
	if h.Evidence["retry_also_errored"] != false {
		t.Errorf("retry_also_errored = %v, want false", h.Evidence["retry_also_errored"])
	}
	// Confidence: base=0.7 (no error_type boost, no retry_also_errored boost)
	if !floatNear(h.Confidence, 0.7, 0.001) {
		t.Errorf("Confidence = %.4f, want 0.7", h.Confidence)
	}
}

// TestFailedHandoff_Negative_RetryBefore: T6 negative — spanB starts at T+3s
// (before spanA EndTime T+5s). No signal.
func TestFailedHandoff_Negative_RetryBefore(t *testing.T) {
	forest := handoffForest("error", "done", 3*time.Second, "")
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalFailedHandoff {
			t.Errorf("retry before error end must not emit signal, got %+v", s)
		}
	}
}

// TestFailedHandoff_BothError is SIGNALS_SPEC T7: span-b also errors.
// retry_also_errored=true → confidence boost to 0.8 (base 0.7 + 0.1).
func TestFailedHandoff_BothError(t *testing.T) {
	forest := handoffForest("error", "error", 6*time.Second, "")
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)

	var ho []signal.Signal
	for _, s := range signals {
		if s.Type == signal.SignalFailedHandoff {
			ho = append(ho, s)
		}
	}
	if len(ho) != 1 {
		t.Fatalf("expected 1 failed_handoff signal, got %d", len(ho))
	}
	h := ho[0]
	if h.Evidence["retry_also_errored"] != true {
		t.Errorf("retry_also_errored = %v, want true", h.Evidence["retry_also_errored"])
	}
	// Confidence: 0.7 + 0.1 = 0.8
	if !floatNear(h.Confidence, 0.8, 0.001) {
		t.Errorf("Confidence = %.4f, want ~0.8", h.Confidence)
	}
}

// TestFailedHandoff_WithErrorType: error_type non-empty → +0.2 confidence.
func TestFailedHandoff_WithErrorType(t *testing.T) {
	forest := handoffForest("error", "done", 6*time.Second, "timeout")
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	var ho []signal.Signal
	for _, s := range signals {
		if s.Type == signal.SignalFailedHandoff {
			ho = append(ho, s)
		}
	}
	if len(ho) != 1 {
		t.Fatalf("expected 1 failed_handoff signal, got %d", len(ho))
	}
	// Confidence: 0.7 + 0.2 = 0.9
	if !floatNear(ho[0].Confidence, 0.9, 0.001) {
		t.Errorf("Confidence = %.4f, want ~0.9", ho[0].Confidence)
	}
}

// TestFailedHandoff_E3_ErrorSpanZeroEndTime: E3 — error span without EndTime skipped.
func TestFailedHandoff_E3_ErrorSpanZeroEndTime(t *testing.T) {
	T := handoffBase
	root := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-e3ho",
			SpanID:    "root",
			StartTime: T,
			EndTime:   T.Add(10 * time.Second),
			Status:    "done",
		},
	}
	spanA := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-e3ho",
			SpanID:       "span-a",
			ParentSpanID: "root",
			StartTime:    T.Add(1 * time.Second),
			// EndTime zero = still running; must skip
			Status: "error",
			Attrs:  map[string]string{"gen_ai.tool.name": "bash"},
		},
	}
	spanB := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-e3ho",
			SpanID:       "span-b",
			ParentSpanID: "root",
			StartTime:    T.Add(6 * time.Second),
			EndTime:      T.Add(9 * time.Second),
			Status:       "done",
			Attrs:        map[string]string{"gen_ai.tool.name": "bash"},
		},
	}
	root.Children = []*telemetry.SpanNode{spanA, spanB}
	forest := signal.Forest{"trace-e3ho": {root}}
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalFailedHandoff {
			t.Errorf("zero-EndTime error span must be skipped; got %+v", s)
		}
	}
}

// TestFailedHandoff_E4_SingleSpan: E4 — single-span trace; no siblings; no signal.
func TestFailedHandoff_E4_SingleSpan(t *testing.T) {
	T := handoffBase
	root := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-e4ho",
			SpanID:    "only",
			StartTime: T,
			EndTime:   T.Add(5 * time.Second),
			Status:    "error",
			Attrs:     map[string]string{"gen_ai.tool.name": "bash"},
		},
	}
	forest := signal.Forest{"trace-e4ho": {root}}
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalFailedHandoff {
			t.Errorf("single-span trace must not emit failed_handoff; got %+v", s)
		}
	}
}

// TestFailedHandoff_Evidence_Fields verifies all required evidence keys.
func TestFailedHandoff_Evidence_Fields(t *testing.T) {
	forest := handoffForest("error", "done", 6*time.Second, "timeout")
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	var h *signal.Signal
	for i := range signals {
		if signals[i].Type == signal.SignalFailedHandoff {
			h = &signals[i]
			break
		}
	}
	if h == nil {
		t.Fatal("no failed_handoff signal found")
	}
	for _, key := range []string{"tool_name", "error_span_id", "retry_span_id", "parent_span_id", "error_type", "gap_ms", "retry_also_errored"} {
		if _, ok := h.Evidence[key]; !ok {
			t.Errorf("evidence missing key %q", key)
		}
	}
}

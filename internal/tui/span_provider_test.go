package tui

import (
	"testing"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestSpanProvider_DAGTabSatisfies is a compile-time + runtime proof that
// *DAGTab satisfies SpanProvider. The compile-time assertion lives in
// span_provider.go; this test validates the runtime contract too.
func TestSpanProvider_DAGTabSatisfies(t *testing.T) {
	// Runtime interface check: confirm *DAGTab implements SpanProvider.
	var provider SpanProvider = NewDAGTab()

	// Spans() on a fresh DAGTab returns the zero-value span slice.
	// The contract only requires the method to exist and not panic.
	spans := provider.Spans()
	// len(nil) == 0 in Go, so this assertion is valid for both nil and empty.
	if len(spans) != 0 {
		t.Errorf("fresh DAGTab.Spans() = %d spans, want 0", len(spans))
	}
}

// TestSpanProvider_SpansReturnedThroughInterface verifies that spans added to
// a *DAGTab are visible when accessed through the SpanProvider interface.
func TestSpanProvider_SpansReturnedThroughInterface(t *testing.T) {
	dag := NewDAGTab()
	dag.spans = []telemetry.Span{
		{TraceID: "t1", SpanID: "s1", System: "anthropic"},
		{TraceID: "t1", SpanID: "s2", System: "anthropic"},
	}

	var provider SpanProvider = dag
	got := provider.Spans()
	if len(got) != 2 {
		t.Errorf("Spans() through interface returned %d spans, want 2", len(got))
	}
}

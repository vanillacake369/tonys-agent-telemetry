package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// ---------------------------------------------------------------------------
// resolveSpanCap
// ---------------------------------------------------------------------------

func TestResolveSpanCap_DefaultIs50k(t *testing.T) {
	t.Setenv(spanCapEnvVar, "")
	got := resolveSpanCap()
	if got != 50_000 {
		t.Errorf("resolveSpanCap() = %d, want 50000", got)
	}
}

func TestResolveSpanCap_EnvOverride(t *testing.T) {
	t.Setenv(spanCapEnvVar, "12345")
	got := resolveSpanCap()
	if got != 12345 {
		t.Errorf("resolveSpanCap() = %d, want 12345", got)
	}
}

func TestResolveSpanCap_InvalidEnvFallsBackToDefault(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"negative", "-1"},
		{"zero", "0"},
		{"non-integer", "not-a-number"},
		{"empty", ""},
		{"float", "1.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(spanCapEnvVar, tc.value)
			got := resolveSpanCap()
			if got != defaultSpanCap {
				t.Errorf("resolveSpanCap() with %q = %d, want %d (default)", tc.value, got, defaultSpanCap)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// capSpans
// ---------------------------------------------------------------------------

func TestCapSpans_RetainsMostRecentByEndTime(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	spans := make([]telemetry.Span, 10)
	for i := range spans {
		spans[i] = telemetry.Span{
			SpanID:  fmt.Sprintf("s%d", i),
			TraceID: "t1",
			EndTime: base.Add(time.Duration(i) * time.Second),
		}
	}

	// Keep only the last 5; the input is time-ordered so the tail is newest.
	got := capSpans(spans, 5)
	if len(got) != 5 {
		t.Fatalf("capSpans returned %d spans, want 5", len(got))
	}
	// The retained spans should be spans[5]…spans[9].
	for i, s := range got {
		want := fmt.Sprintf("s%d", i+5)
		if s.SpanID != want {
			t.Errorf("got[%d].SpanID = %q, want %q", i, s.SpanID, want)
		}
	}
}

func TestCapSpans_KeepsInProgressSpans(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Build 8 completed spans…
	spans := make([]telemetry.Span, 0, 10)
	for i := 0; i < 8; i++ {
		spans = append(spans, telemetry.Span{
			SpanID:  fmt.Sprintf("done-%d", i),
			TraceID: "t1",
			EndTime: base.Add(time.Duration(i) * time.Second),
		})
	}
	// …then 2 in-progress (zero EndTime) spans inserted at position 0 and 1
	// to simulate out-of-order backfill. Re-insert them at the front.
	inProgress := []telemetry.Span{
		{SpanID: "live-a", TraceID: "t1"},
		{SpanID: "live-b", TraceID: "t1"},
	}
	all := append(inProgress, spans...) // in-progress at head, completed at tail

	// Cap to 8: the tail-slice would drop live-a and live-b (they're in the
	// evicted head), so the rescue pass must bring them back.
	got := capSpans(all, 8)

	inProgressFound := map[string]bool{"live-a": false, "live-b": false}
	for _, s := range got {
		if _, ok := inProgressFound[s.SpanID]; ok {
			inProgressFound[s.SpanID] = true
		}
	}
	for id, found := range inProgressFound {
		if !found {
			t.Errorf("in-progress span %q was dropped by capSpans", id)
		}
	}
}

func TestCapSpans_NoCapWhenBelowLimit(t *testing.T) {
	spans := []telemetry.Span{
		{SpanID: "a", TraceID: "t"},
		{SpanID: "b", TraceID: "t"},
	}
	got := capSpans(spans, 100)
	if len(got) != 2 {
		t.Errorf("capSpans below cap returned %d, want 2", len(got))
	}
}

func TestCapSpans_ExactlyAtLimit(t *testing.T) {
	spans := make([]telemetry.Span, 5)
	for i := range spans {
		spans[i] = telemetry.Span{SpanID: fmt.Sprintf("s%d", i), TraceID: "t"}
	}
	got := capSpans(spans, 5)
	if len(got) != 5 {
		t.Errorf("capSpans at exact limit returned %d, want 5", len(got))
	}
}

// ---------------------------------------------------------------------------
// DAGTab integration
// ---------------------------------------------------------------------------

// TestDAGTab_LargeBatchRespectsCap pushes 60k synthetic spans via App.Update
// and asserts the final DAGTab state respects the resolved cap.
func TestDAGTab_LargeBatchRespectsCap(t *testing.T) {
	const batchSize = 60_000

	// Force a known cap via env so the test isn't sensitive to the default.
	const cap = 50_000
	t.Setenv(spanCapEnvVar, fmt.Sprintf("%d", cap))

	a := NewApp()
	a.width, a.height = 80, 24
	a = a.propagateSize()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	batch := make([]telemetry.Span, batchSize)
	for i := range batch {
		batch[i] = telemetry.Span{
			SpanID:  fmt.Sprintf("s%d", i),
			TraceID: fmt.Sprintf("trace-%d", i%1000), // 1000 distinct traces
			EndTime: base.Add(time.Duration(i) * time.Millisecond),
		}
	}

	updated, _ := a.Update(SpanBatchMsg{Spans: batch})
	got := updated.(App)

	d := got.tabs[TabDAG].(*DAGTab)
	if len(d.spans) > cap {
		t.Errorf("DAGTab has %d spans after 60k batch, want ≤ %d (cap)", len(d.spans), cap)
	}
	if len(d.spans) == 0 {
		t.Error("DAGTab has 0 spans after 60k batch — cap evicted everything")
	}
}

package trends

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// anchor is a fixed wall-clock reference for live snapshot tests.
var anchor = time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

// makeSpan builds a minimal Span with the given TraceID, SpanID, and EndTime offset.
// parentID may be empty for root spans. Duration is fixed at 30s to exceed the stall
// threshold (10s + 0.5s skew) when spans form a completed leaf.
func makeSpan(traceID, spanID, parentID string, endOffset time.Duration) telemetry.Span {
	end := anchor.Add(endOffset)
	start := end.Add(-30 * time.Second)
	return telemetry.Span{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentID,
		System:       "anthropic",
		StartTime:    start,
		EndTime:      end,
		Status:       "done",
		Attrs:        map[string]string{},
	}
}

// TestBuildLiveSnapshots_GroupsByTrace verifies that 3 traces × 2 spans each
// produce exactly 3 SessionSnapshots (one per TraceID).
func TestBuildLiveSnapshots_GroupsByTrace(t *testing.T) {
	spans := []telemetry.Span{
		makeSpan("trace-A", "a1", "", 1*time.Hour),
		makeSpan("trace-A", "a2", "a1", 2*time.Hour),
		makeSpan("trace-B", "b1", "", 3*time.Hour),
		makeSpan("trace-B", "b2", "b1", 4*time.Hour),
		makeSpan("trace-C", "c1", "", 5*time.Hour),
		makeSpan("trace-C", "c2", "c1", 6*time.Hour),
	}

	snaps := BuildLiveSnapshots(spans, signal.DefaultExtractOpts())

	if len(snaps) != 3 {
		t.Fatalf("expected 3 snapshots (one per trace), got %d", len(snaps))
	}

	seen := make(map[string]bool)
	for _, s := range snaps {
		seen[s.SessionID] = true
	}
	for _, id := range []string{"trace-A", "trace-B", "trace-C"} {
		if !seen[id] {
			t.Errorf("snapshot for %q not found", id)
		}
	}
}

// TestBuildLiveSnapshots_CapturedAtUsesMaxEndTime asserts that CapturedAt of
// each snapshot equals the maximum EndTime of spans in that trace.
func TestBuildLiveSnapshots_CapturedAtUsesMaxEndTime(t *testing.T) {
	wantA := anchor.Add(2 * time.Hour) // a2 ends later than a1
	wantB := anchor.Add(4 * time.Hour) // b2 ends later than b1

	spans := []telemetry.Span{
		makeSpan("trace-A", "a1", "", 1*time.Hour),
		makeSpan("trace-A", "a2", "a1", 2*time.Hour),
		makeSpan("trace-B", "b1", "", 3*time.Hour),
		makeSpan("trace-B", "b2", "b1", 4*time.Hour),
	}

	snaps := BuildLiveSnapshots(spans, signal.DefaultExtractOpts())

	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}

	byID := make(map[string]time.Time)
	for _, s := range snaps {
		if len(s.Entries) != 1 {
			t.Errorf("snapshot %q: expected 1 entry, got %d", s.SessionID, len(s.Entries))
			continue
		}
		byID[s.SessionID] = s.Entries[0].CapturedAt
	}

	if got := byID["trace-A"]; !got.Equal(wantA) {
		t.Errorf("trace-A CapturedAt = %v, want %v", got, wantA)
	}
	if got := byID["trace-B"]; !got.Equal(wantB) {
		t.Errorf("trace-B CapturedAt = %v, want %v", got, wantB)
	}
}

// TestBuildLiveSnapshots_EmptySpans_ReturnsNil confirms nil input yields nil output.
func TestBuildLiveSnapshots_EmptySpans_ReturnsNil(t *testing.T) {
	result := BuildLiveSnapshots(nil, signal.DefaultExtractOpts())
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

// TestBuildLiveSnapshots_ZeroEndTimeTrace_Skipped confirms that a trace whose
// spans all have zero EndTime is excluded from the result.
func TestBuildLiveSnapshots_ZeroEndTimeTrace_Skipped(t *testing.T) {
	// trace-Z has no EndTime set (in-progress); trace-A is complete.
	running := telemetry.Span{
		TraceID:   "trace-Z",
		SpanID:    "z1",
		System:    "anthropic",
		StartTime: anchor,
		// EndTime intentionally zero — trace still in progress.
		Status: "running",
		Attrs:  map[string]string{},
	}
	complete := makeSpan("trace-A", "a1", "", 1*time.Hour)

	snaps := BuildLiveSnapshots([]telemetry.Span{running, complete}, signal.DefaultExtractOpts())

	for _, s := range snaps {
		if s.SessionID == "trace-Z" {
			t.Errorf("trace-Z (zero EndTime) should be skipped, but appeared in snapshots")
		}
	}
	found := false
	for _, s := range snaps {
		if s.SessionID == "trace-A" {
			found = true
		}
	}
	if !found {
		t.Error("trace-A (complete) should appear in snapshots but was not found")
	}
}

// TestBuildLiveSnapshots_ExtractsSignals verifies that a synthetic stalled-leaf
// trace produces at least one stalled_node signal in the resulting snapshot.
// Span duration is 30s which exceeds the default stall threshold (10s + 0.5s skew).
func TestBuildLiveSnapshots_ExtractsSignals(t *testing.T) {
	// Single-span trace: the root is also the leaf, duration 30s → stalled_node fires.
	end := anchor.Add(1 * time.Hour)
	stalledSpan := telemetry.Span{
		TraceID:   "trace-stall",
		SpanID:    "only-span",
		System:    "anthropic",
		StartTime: end.Add(-30 * time.Second),
		EndTime:   end,
		Status:    "done",
		Attrs:     map[string]string{},
	}

	snaps := BuildLiveSnapshots([]telemetry.Span{stalledSpan}, signal.DefaultExtractOpts())

	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	snap := snaps[0]
	if len(snap.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(snap.Entries))
	}

	var found bool
	for _, sig := range snap.Entries[0].Signals {
		if sig.Type == signal.SignalStalledNode {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stalled_node signal in snapshot for 30s leaf span, got signals: %v",
			snap.Entries[0].Signals)
	}
}

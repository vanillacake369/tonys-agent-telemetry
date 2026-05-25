package signal_test

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// stalledNodeForest is a shared test fixture:
//   - one root span (duration 30s)
//   - one intermediate child (duration 1s)
//   - one deep leaf (duration 20s, no children) — the stalled span
func stalledNodeForest() signal.Forest {
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	root := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-1",
			SpanID:    "root",
			StartTime: now.Add(-30 * time.Second),
			EndTime:   now,
			Status:    "done",
		},
	}
	intermediate := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-1",
			SpanID:       "intermediate",
			ParentSpanID: "root",
			StartTime:    now.Add(-2 * time.Second),
			EndTime:      now.Add(-1 * time.Second),
			Status:       "done",
		},
	}
	leaf := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-1",
			SpanID:       "leaf",
			ParentSpanID: "intermediate",
			StartTime:    now.Add(-25 * time.Second),
			EndTime:      now.Add(-5 * time.Second), // 20s duration
			Status:       "done",
		},
	}
	intermediate.Children = []*telemetry.SpanNode{leaf}
	root.Children = []*telemetry.SpanNode{intermediate}
	return signal.Forest{"trace-1": {root}}
}

// TestStalledNode_Positive is SIGNALS_SPEC T1: one stalled leaf, one fast leaf.
// Confidence = min(1.0, (20 - 0.5) / 20) ≈ 0.975 with default 500ms skew and
// 10s threshold.
func TestStalledNode_Positive(t *testing.T) {
	forest := stalledNodeForest()
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)

	var stalled []signal.Signal
	for _, s := range signals {
		if s.Type == signal.SignalStalledNode {
			stalled = append(stalled, s)
		}
	}

	if len(stalled) != 1 {
		t.Fatalf("expected 1 stalled_node signal, got %d", len(stalled))
	}
	s := stalled[0]
	if len(s.SpanIDs) != 1 || s.SpanIDs[0] != "leaf" {
		t.Errorf("SpanIDs = %v, want [leaf]", s.SpanIDs)
	}
	// adjustedDuration = 20s - 0.5s = 19.5s; threshold = 10s
	// confidence = min(1.0, 19.5 / 20) = 0.975
	wantConf := 0.975
	if abs := s.Confidence - wantConf; abs < -0.001 || abs > 0.001 {
		t.Errorf("Confidence = %.4f, want ~%.4f", s.Confidence, wantConf)
	}
	if s.TraceID != "trace-1" {
		t.Errorf("TraceID = %q, want trace-1", s.TraceID)
	}
	if s.ProviderTier != "full" {
		t.Errorf("ProviderTier = %q, want full", s.ProviderTier)
	}
}

// TestStalledNode_ShortLeafNotEmitted ensures the 1s intermediate span
// does NOT trigger a stalled_node signal.
func TestStalledNode_ShortLeafNotEmitted(t *testing.T) {
	forest := stalledNodeForest()
	opts := signal.DefaultExtractOpts()
	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalStalledNode && len(s.SpanIDs) > 0 && s.SpanIDs[0] == "intermediate" {
			t.Error("intermediate span (1s) must not emit stalled_node; it is not a leaf anyway")
		}
	}
}

// TestStalledNode_InProgressTrace is SIGNALS_SPEC T2: trace fully in-progress;
// expect zero signals.
func TestStalledNode_InProgressTrace(t *testing.T) {
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	root := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-ip",
			SpanID:    "root",
			StartTime: now.Add(-30 * time.Second),
			// EndTime zero = still running
			Status: "running",
		},
	}
	leaf := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-ip",
			SpanID:       "leaf",
			ParentSpanID: "root",
			StartTime:    now.Add(-25 * time.Second),
			// EndTime zero = still running
			Status: "running",
		},
	}
	root.Children = []*telemetry.SpanNode{leaf}
	forest := signal.Forest{"trace-ip": {root}}
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalStalledNode {
			t.Errorf("expected no stalled_node for in-progress trace, got %+v", s)
		}
	}
}

// TestStalledNode_E3_ZeroDuration is edge case E3: zero duration span skipped.
func TestStalledNode_E3_ZeroDuration(t *testing.T) {
	now := time.Now()
	root := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-e3",
			SpanID:    "root",
			StartTime: now.Add(-5 * time.Second),
			EndTime:   now, // root ended so traceEndTime is non-zero
			Status:    "done",
		},
	}
	leaf := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-e3",
			SpanID:       "leaf-zerodur",
			ParentSpanID: "root",
			StartTime:    now,
			EndTime:      now, // same start/end = 0 duration
			Status:       "done",
		},
	}
	root.Children = []*telemetry.SpanNode{leaf}
	forest := signal.Forest{"trace-e3": {root}}
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalStalledNode {
			t.Errorf("zero-duration leaf must not emit stalled_node, got %+v", s)
		}
	}
}

// TestStalledNode_E4_SingleSpanTrace is edge case E4: root with no children
// is itself a leaf; subject to stall detection.
func TestStalledNode_E4_SingleSpanTrace(t *testing.T) {
	now := time.Now()
	root := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-e4",
			SpanID:    "only-span",
			StartTime: now.Add(-20 * time.Second),
			EndTime:   now, // 20s duration
			Status:    "done",
		},
	}
	forest := signal.Forest{"trace-e4": {root}}
	opts := signal.DefaultExtractOpts() // StallThreshold=10s, ClockSkew=500ms

	signals := signal.Extract(forest, opts)
	var stalled []signal.Signal
	for _, s := range signals {
		if s.Type == signal.SignalStalledNode {
			stalled = append(stalled, s)
		}
	}
	if len(stalled) != 1 {
		t.Fatalf("single-span trace with 20s should fire stalled_node; got %d signals", len(stalled))
	}
	if stalled[0].SpanIDs[0] != "only-span" {
		t.Errorf("SpanIDs[0] = %q, want only-span", stalled[0].SpanIDs[0])
	}
}

// TestStalledNode_E7_ZeroStartTime is edge case E7: StartTime is zero; skip.
func TestStalledNode_E7_ZeroStartTime(t *testing.T) {
	now := time.Now()
	root := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-e7",
			SpanID:    "root",
			StartTime: now.Add(-5 * time.Second),
			EndTime:   now,
			Status:    "done",
		},
	}
	leaf := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-e7",
			SpanID:       "bad-leaf",
			ParentSpanID: "root",
			// StartTime zero — invalid span
			EndTime: now,
			Status:  "done",
		},
	}
	root.Children = []*telemetry.SpanNode{leaf}
	forest := signal.Forest{"trace-e7": {root}}
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalStalledNode && len(s.SpanIDs) > 0 && s.SpanIDs[0] == "bad-leaf" {
			t.Error("zero-StartTime leaf must be skipped")
		}
	}
}

// TestStalledNode_E6_MultipleOrphanRootsAllStall is edge case E6.
func TestStalledNode_E6_MultipleOrphanRootsAllStall(t *testing.T) {
	now := time.Now()
	makeOrphan := func(id string) *telemetry.SpanNode {
		return &telemetry.SpanNode{
			Span: telemetry.Span{
				TraceID:   "trace-e6",
				SpanID:    id,
				StartTime: now.Add(-20 * time.Second),
				EndTime:   now,
				Status:    "done",
			},
		}
	}
	orphan1 := makeOrphan("orphan-1")
	orphan2 := makeOrphan("orphan-2")
	forest := signal.Forest{"trace-e6": {orphan1, orphan2}}
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	count := 0
	for _, s := range signals {
		if s.Type == signal.SignalStalledNode {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 stalled_node signals for 2 orphan roots, got %d", count)
	}
}

// TestStalledNode_ClockSkew_NegativeAdjusted is edge case E5: ClockSkewTolerance
// larger than duration produces negative adjusted value → no signal.
func TestStalledNode_ClockSkew_NegativeAdjusted(t *testing.T) {
	now := time.Now()
	root := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-skew",
			SpanID:    "root",
			StartTime: now.Add(-12 * time.Second),
			EndTime:   now,
			Status:    "done",
		},
	}
	leaf := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      "trace-skew",
			SpanID:       "leaf",
			ParentSpanID: "root",
			StartTime:    now.Add(-11 * time.Second),
			EndTime:      now, // 11s raw
			Status:       "done",
		},
	}
	root.Children = []*telemetry.SpanNode{leaf}
	forest := signal.Forest{"trace-skew": {root}}
	opts := signal.DefaultExtractOpts()
	opts.ClockSkewTolerance = 12 * time.Second // subtract 12s → adjusted = -1s

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalStalledNode {
			t.Errorf("negative adjusted duration must not emit stalled_node, got %+v", s)
		}
	}
}

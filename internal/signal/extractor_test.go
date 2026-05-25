package signal_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestExtract_T8_ZeroSignalCleanForest is T8: 3 traces, all durations < 5s,
// no errors, no overlapping tool sequences, no installed skills.
// Expect len(result) == 0.
func TestExtract_T8_ZeroSignalCleanForest(t *testing.T) {
	forest := makeTraceForest(3, []string{"read_file", "write_file"})
	opts := signal.DefaultExtractOpts() // InstalledSkills is nil → skipped

	signals := signal.Extract(forest, opts)
	if len(signals) != 0 {
		t.Errorf("expected 0 signals on clean forest, got %d: %+v", len(signals), signals)
	}
}

// TestExtract_T9_MixedProviderForest is T9: one trace combining claudecode
// spans (with gen_ai.tool.name) and vllm-like orphan roots (no parent ID,
// no tool name). Verify:
//   - no panic on missing Attrs keys
//   - orphan roots are not treated as siblings of each other
//   - stalled_node fires only if duration check passes
func TestExtract_T9_MixedProviderForest(t *testing.T) {
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)

	// claudecode root with a child tool span (short duration — no stall)
	claudeRoot := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-mixed",
			SpanID:    "claude-root",
			System:    "anthropic",
			StartTime: now.Add(-3 * time.Second),
			EndTime:   now,
			Status:    "done",
			Attrs:     map[string]string{"gen_ai.tool.name": "bash"},
		},
	}

	// vllm-like orphan root: no ParentSpanID, no tool name, no Attrs map
	vllmOrphan1 := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-mixed",
			SpanID:    "vllm-orphan-1",
			System:    "vllm",
			StartTime: now.Add(-2 * time.Second),
			EndTime:   now,
			Status:    "done",
			// Attrs intentionally nil — must not panic
		},
	}
	vllmOrphan2 := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-mixed",
			SpanID:    "vllm-orphan-2",
			System:    "vllm",
			StartTime: now.Add(-4 * time.Second),
			EndTime:   now,
			Status:    "done",
			// Attrs intentionally nil
		},
	}

	// All three are roots (orphans); no shared parent
	forest := signal.Forest{"trace-mixed": {claudeRoot, vllmOrphan1, vllmOrphan2}}
	opts := signal.DefaultExtractOpts()

	// Must not panic.
	var signals []signal.Signal
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Extract panicked on mixed-provider forest: %v", r)
			}
		}()
		signals = signal.Extract(forest, opts)
	}()

	// Orphan roots have no shared parent → no duplicate_subagent_work between them.
	for _, s := range signals {
		if s.Type == signal.SignalDuplicateSubagentWork {
			t.Errorf("orphan roots must not produce duplicate_subagent_work; got %+v", s)
		}
	}
	// No stalled spans (all durations < 10s − 0.5s threshold).
	for _, s := range signals {
		if s.Type == signal.SignalStalledNode {
			t.Errorf("all durations < threshold; must not emit stalled_node; got %+v", s)
		}
	}
}

// TestExtract_T10_Determinism calls Extract twice on the same Forest and
// asserts byte-identical JSON output (SIGNALS_SPEC §5).
func TestExtract_T10_Determinism(t *testing.T) {
	forest := stalledNodeForest()
	opts := signal.DefaultExtractOpts()

	sig1 := signal.Extract(forest, opts)
	sig2 := signal.Extract(forest, opts)

	if len(sig1) != len(sig2) {
		t.Fatalf("non-deterministic result count: first=%d, second=%d", len(sig1), len(sig2))
	}

	// Fix EmittedAt to the same value for JSON comparison (EmittedAt is clock-driven).
	fixedTime := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	for i := range sig1 {
		sig1[i].EmittedAt = fixedTime
		sig2[i].EmittedAt = fixedTime
	}

	j1, err1 := json.Marshal(sig1)
	j2, err2 := json.Marshal(sig2)
	if err1 != nil || err2 != nil {
		t.Fatalf("marshal error: %v / %v", err1, err2)
	}
	if string(j1) != string(j2) {
		t.Errorf("non-deterministic JSON output:\nfirst:  %s\nsecond: %s", j1, j2)
	}
}

// TestExtract_OutputOrder verifies signals are sorted by (TraceID, SpanIDs[0], Type)
// and cross-trace signals (unused_installed_skill) sort last.
func TestExtract_OutputOrder(t *testing.T) {
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)

	// Build a forest with stalled spans in two traces + an unused skill.
	rootA := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-b", // lexicographically second
			SpanID:    "root-b",
			StartTime: now.Add(-25 * time.Second),
			EndTime:   now,
			Status:    "done",
		},
	}
	rootB := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-a", // lexicographically first
			SpanID:    "root-a",
			StartTime: now.Add(-25 * time.Second),
			EndTime:   now,
			Status:    "done",
		},
	}
	// Add more traces so MinSessions gate is satisfied.
	forest := make(signal.Forest)
	forest["trace-a"] = []*telemetry.SpanNode{rootB}
	forest["trace-b"] = []*telemetry.SpanNode{rootA}
	for i := 2; i < 5; i++ {
		id := traceIDFor(i + 10)
		root := &telemetry.SpanNode{
			Span: telemetry.Span{
				TraceID:   id,
				SpanID:    "r-" + id,
				StartTime: now.Add(-2 * time.Second),
				EndTime:   now,
				Status:    "done",
			},
		}
		forest[id] = []*telemetry.SpanNode{root}
	}

	opts := signal.DefaultExtractOpts()
	opts.InstalledSkills = []string{"unused-skill"}
	opts.StallThreshold = 5 * time.Second
	opts.ClockSkewTolerance = 0

	signals := signal.Extract(forest, opts)

	// Verify unused_installed_skill appears last.
	for i, s := range signals {
		if s.Type == signal.SignalUnusedInstalledSkill && i < len(signals)-1 {
			// Check if any non-unused signal follows it.
			for _, later := range signals[i+1:] {
				if later.Type != signal.SignalUnusedInstalledSkill {
					t.Errorf("unused_installed_skill at index %d is not last; %s follows it", i, later.Type)
				}
			}
		}
	}

	// Verify trace-scoped signals are ordered by TraceID.
	lastTraceID := ""
	for _, s := range signals {
		if s.TraceID == "" {
			continue
		}
		if s.TraceID < lastTraceID {
			t.Errorf("signals not sorted by TraceID: %q comes after %q", s.TraceID, lastTraceID)
		}
		lastTraceID = s.TraceID
	}
}

// TestExtract_EmittedAt_SingleCallShared asserts all signals from one Extract
// call share the same EmittedAt value (SIGNALS_SPEC §5, note about single
// time.Now() per call).
func TestExtract_EmittedAt_SingleCallShared(t *testing.T) {
	forest := stalledNodeForest()
	opts := signal.DefaultExtractOpts()

	// Add installed skills to trigger unused_installed_skill too.
	opts.InstalledSkills = []string{"no-such-skill"}
	opts.MinSessionsForUnusedSkill = 1

	signals := signal.Extract(forest, opts)
	if len(signals) == 0 {
		t.Skip("no signals emitted; adjust fixture if needed")
	}
	first := signals[0].EmittedAt
	for i, s := range signals {
		if !s.EmittedAt.Equal(first) {
			t.Errorf("signal[%d].EmittedAt = %v, want same as signal[0] = %v", i, s.EmittedAt, first)
		}
	}
}

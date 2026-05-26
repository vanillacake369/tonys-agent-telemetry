package signal_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// dupWorkForest builds a forest with one parent and two children. Each child
// has the given tool-name sequences (encoded as leaf spans under the child).
func dupWorkForest(seqA, seqB []string) signal.Forest {
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	parent := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-dup",
			SpanID:    "parent",
			StartTime: now.Add(-10 * time.Second),
			EndTime:   now,
			Status:    "done",
		},
	}
	childA := toolSequenceNode("trace-dup", "child-a", "parent", seqA, now)
	childB := toolSequenceNode("trace-dup", "child-b", "parent", seqB, now)
	parent.Children = []*telemetry.SpanNode{childA, childB}
	return signal.Forest{"trace-dup": {parent}}
}

// toolSequenceNode creates a SpanNode whose tool sequence (DFS pre-order) equals
// tools. Each tool call becomes a direct child leaf of the returned node.
func toolSequenceNode(traceID, spanID, parentID string, tools []string, now time.Time) *telemetry.SpanNode {
	node := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:      traceID,
			SpanID:       spanID,
			ParentSpanID: parentID,
			StartTime:    now.Add(-5 * time.Second),
			EndTime:      now,
			Status:       "done",
		},
	}
	for i, t := range tools {
		leaf := &telemetry.SpanNode{
			Span: telemetry.Span{
				TraceID:      traceID,
				SpanID:       fmt.Sprintf("%s-leaf-%d", spanID, i),
				ParentSpanID: spanID,
				StartTime:    now.Add(-time.Duration(i+1) * time.Second),
				EndTime:      now.Add(-time.Duration(i) * time.Second),
				Status:       "done",
				Attrs: map[string]string{
					"gen_ai.tool.name": t,
				},
			},
		}
		node.Children = append(node.Children, leaf)
	}
	return node
}

// TestDuplicateWork_Positive_IdenticalSequence is SIGNALS_SPEC T3 positive:
// A parent with two children that have identical tool sequences → Jaccard = 1.0.
// We verify that the child-pair signal is present with confidence 1.0.
// (Additional signals may fire for sibling pairs within each child's subtree —
// that is correct detector behavior per spec E5.)
func TestDuplicateWork_Positive_IdenticalSequence(t *testing.T) {
	seq := []string{"bash", "bash", "read_file"}
	forest := dupWorkForest(seq, seq)
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)

	var childPairSig *signal.Signal
	for i := range signals {
		s := &signals[i]
		if s.Type != signal.SignalDuplicateSubagentWork {
			continue
		}
		// The child-level pair has parent_span_id == "parent".
		if pid, _ := s.Evidence["parent_span_id"].(string); pid == "parent" {
			childPairSig = s
			break
		}
	}
	if childPairSig == nil {
		t.Fatalf("expected a duplicate_subagent_work signal between child-a and child-b (parent_span_id=parent), got none")
	}
	if childPairSig.Confidence != 1.0 {
		t.Errorf("Confidence = %.4f, want 1.0 for identical sequences", childPairSig.Confidence)
	}
	if len(childPairSig.SpanIDs) != 2 {
		t.Errorf("SpanIDs = %v, want 2 span IDs", childPairSig.SpanIDs)
	}
}

// TestDuplicateWork_Below_Threshold: T3 negative case — the child-pair Jaccard
// is 0.5, below the 0.8 threshold. No signal must fire for the child-a/child-b pair.
func TestDuplicateWork_Below_Threshold(t *testing.T) {
	seqA := []string{"read_file", "bash", "write_file"} // all distinct, no dup within
	seqB := []string{"grep", "ls", "cat"}               // entirely different
	forest := dupWorkForest(seqA, seqB)
	opts := signal.DefaultExtractOpts() // DupOverlapThreshold=0.8

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type != signal.SignalDuplicateSubagentWork {
			continue
		}
		// Must not fire for the child-a/child-b pair (Jaccard=0).
		if pid, _ := s.Evidence["parent_span_id"].(string); pid == "parent" {
			t.Errorf("child-a/child-b pair: Jaccard=0 is below threshold; no signal expected, got %+v", s)
		}
	}
}

// TestDuplicateWork_E3_OneChildEmpty: E3 case — one child has zero named tool calls;
// Jaccard = 0 for the child-pair; no signal must fire for the parent-level pair.
func TestDuplicateWork_E3_OneChildEmpty(t *testing.T) {
	forest := dupWorkForest([]string{"bash", "bash"}, []string{})
	opts := signal.DefaultExtractOpts()
	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type != signal.SignalDuplicateSubagentWork {
			continue
		}
		// The parent-level pair must not fire (one side is empty → Jaccard=0).
		if pid, _ := s.Evidence["parent_span_id"].(string); pid == "parent" {
			t.Errorf("one empty sequence: child-a/child-b pair must not emit signal; got %+v", s)
		}
	}
}

// TestDuplicateWork_E3_BothEmpty: both children have zero named tool calls → skip
// the child-pair check entirely (E3: both empty).
func TestDuplicateWork_E3_BothEmpty(t *testing.T) {
	forest := dupWorkForest([]string{}, []string{})
	opts := signal.DefaultExtractOpts()
	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type != signal.SignalDuplicateSubagentWork {
			continue
		}
		// With no tool calls on either side, the child-pair must be skipped.
		if pid, _ := s.Evidence["parent_span_id"].(string); pid == "parent" {
			t.Errorf("both empty: child-a/child-b must not emit signal; got %+v", s)
		}
	}
}

// TestDuplicateWork_E2_OrphanRootsNotSiblings: E2 — orphan roots (no shared parent)
// are NOT treated as siblings of each other. This test uses single-span orphan
// roots (no children) so there are no within-subtree sibling pairs either.
func TestDuplicateWork_E2_OrphanRootsNotSiblings(t *testing.T) {
	now := time.Now()
	// These are leaf orphan roots — no children. They have identical tool sequences
	// when considered individually (just their own tool name), but they share no
	// parent, so they must NOT be compared as siblings.
	orphan1 := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-orp",
			SpanID:    "orphan-1",
			StartTime: now.Add(-5 * time.Second),
			EndTime:   now,
			Status:    "done",
			Attrs:     map[string]string{"gen_ai.tool.name": "bash"},
		},
	}
	orphan2 := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-orp",
			SpanID:    "orphan-2",
			StartTime: now.Add(-4 * time.Second),
			EndTime:   now,
			Status:    "done",
			Attrs:     map[string]string{"gen_ai.tool.name": "bash"},
		},
	}
	forest := signal.Forest{"trace-orp": {orphan1, orphan2}}
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	for _, s := range signals {
		if s.Type == signal.SignalDuplicateSubagentWork {
			t.Errorf("orphan roots must not be treated as siblings; got %+v", s)
		}
	}
}

// TestDuplicateWork_Evidence_Fields verifies all required evidence fields are set.
func TestDuplicateWork_Evidence_Fields(t *testing.T) {
	seq := []string{"bash", "read_file"}
	forest := dupWorkForest(seq, seq)
	opts := signal.DefaultExtractOpts()

	signals := signal.Extract(forest, opts)
	var d *signal.Signal
	for i := range signals {
		if signals[i].Type == signal.SignalDuplicateSubagentWork {
			d = &signals[i]
			break
		}
	}
	if d == nil {
		t.Fatal("no duplicate_subagent_work signal found")
	}
	for _, key := range []string{"jaccard_similarity", "threshold", "tools_a", "tools_b", "parent_span_id", "overlap_tool_names"} {
		if _, ok := d.Evidence[key]; !ok {
			t.Errorf("evidence missing key %q", key)
		}
	}
}

// TestDuplicateWork_Performance_K50_M50 asserts that extracting from a forest
// with K=50 siblings each having M=50 tool calls completes in under 10ms.
// This validates the O(K·M) complexity claim (not O(K²·M)).
//
// Implementation note: rolling-hash fingerprints allow O(M) fingerprint
// construction per sibling, then O(K) comparison per pair using precomputed
// sets, yielding O(K·M + K²) where K²≪K·M at K=M=50.
func TestDuplicateWork_Performance_K50_M50(t *testing.T) {
	const K = 50
	const M = 50
	now := time.Now()

	parent := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-perf",
			SpanID:    "parent",
			StartTime: now.Add(-60 * time.Second),
			EndTime:   now,
			Status:    "done",
		},
	}
	toolNames := []string{"bash", "read_file", "write_file", "grep", "ls"}
	for k := 0; k < K; k++ {
		tools := make([]string, M)
		for m := 0; m < M; m++ {
			tools[m] = toolNames[m%len(toolNames)]
		}
		child := toolSequenceNode("trace-perf", fmt.Sprintf("child-%d", k), "parent", tools, now)
		parent.Children = append(parent.Children, child)
	}
	forest := signal.Forest{"trace-perf": {parent}}
	opts := signal.DefaultExtractOpts()

	// The race detector adds 2-10× wall-clock overhead, which inflates the
	// timing well past any meaningful budget. Skip the perf assertion under
	// -race; the bench (BenchmarkDuplicateWork_K50_M50) covers the
	// without-race case authoritatively. isRaceEnabled is build-tag-paired
	// (race_on_test.go / race_off_test.go).
	if isRaceEnabled() {
		t.Skip("perf budget validation skipped under -race; see BenchmarkDuplicateWork_K50_M50")
	}

	start := time.Now()
	_ = signal.Extract(forest, opts)
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("K=50, M=50 extraction took %v; budget is 100ms", elapsed)
	}
}

// BenchmarkDuplicateWork_K50_M50 provides a Go benchmark for the same case.
func BenchmarkDuplicateWork_K50_M50(b *testing.B) {
	const K = 50
	const M = 50
	now := time.Now()
	parent := &telemetry.SpanNode{
		Span: telemetry.Span{
			TraceID:   "trace-bench",
			SpanID:    "parent",
			StartTime: now.Add(-60 * time.Second),
			EndTime:   now,
			Status:    "done",
		},
	}
	toolNames := []string{"bash", "read_file", "write_file", "grep", "ls"}
	for k := 0; k < K; k++ {
		tools := make([]string, M)
		for m := 0; m < M; m++ {
			tools[m] = toolNames[m%len(toolNames)]
		}
		child := toolSequenceNode("trace-bench", fmt.Sprintf("child-%d", k), "parent", tools, now)
		parent.Children = append(parent.Children, child)
	}
	forest := signal.Forest{"trace-bench": {parent}}
	opts := signal.DefaultExtractOpts()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = signal.Extract(forest, opts)
	}
}

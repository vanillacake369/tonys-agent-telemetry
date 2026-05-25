package recommender

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
)

// ---------------------------------------------------------------------------
// fixture helpers
// ---------------------------------------------------------------------------

func makeSignal(id string, st signal.SignalType, evidence map[string]any) signal.Signal {
	return signal.Signal{
		ID:           id,
		Type:         st,
		TraceID:      "trace-001",
		SpanIDs:      []string{"span-001"},
		Evidence:     evidence,
		Confidence:   0.9,
		EmittedAt:    time.Now(),
		ProviderTier: "full",
	}
}

func makeItem(id, title string, tags []string, maturity int) catalog.Item {
	return catalog.Item{
		ID:            id,
		Title:         title,
		Type:          catalog.ItemTypeSkill,
		Tags:          tags,
		MaturityLevel: maturity,
	}
}

// ---------------------------------------------------------------------------
// basic per-signal-type tests
// ---------------------------------------------------------------------------

func TestEngine_StalledNode_ProducesRecommendation(t *testing.T) {
	eng := NewEngine()
	sigs := []signal.Signal{
		makeSignal("sig-stall-001", signal.SignalStalledNode, map[string]any{
			"tool_name": "bash",
		}),
	}
	items := []catalog.Item{
		makeItem("skill/shell-perf", "Shell Performance Patterns", []string{"shell", "performance"}, 4),
		makeItem("skill/other", "Unrelated Item", []string{"ml", "vision"}, 2),
	}

	recs := eng.Recommend(sigs, items)

	if len(recs) == 0 {
		t.Fatal("Recommend(stalled_node + bash): expected at least one recommendation, got none")
	}
	for _, r := range recs {
		if r.SignalID == "" {
			t.Errorf("Recommendation missing SignalID: %+v", r)
		}
		if r.CatalogItemID == "" {
			t.Errorf("Recommendation missing CatalogItemID: %+v", r)
		}
	}
}

func TestEngine_DuplicateSubagentWork_ProducesRecommendation(t *testing.T) {
	eng := NewEngine()
	sigs := []signal.Signal{
		makeSignal("sig-dup-001", signal.SignalDuplicateSubagentWork, map[string]any{
			"jaccard_similarity": 0.9,
		}),
	}
	items := []catalog.Item{
		makeItem("skill/orchestration", "Orchestration Patterns", []string{"orchestration", "fan-out"}, 3),
	}

	recs := eng.Recommend(sigs, items)

	if len(recs) == 0 {
		t.Fatal("Recommend(duplicate_subagent_work): expected at least one recommendation")
	}
	if recs[0].SignalID != "sig-dup-001" {
		t.Errorf("wrong SignalID: got %q, want %q", recs[0].SignalID, "sig-dup-001")
	}
}

func TestEngine_UnusedInstalledSkill_ProducesRecommendation(t *testing.T) {
	eng := NewEngine()
	sigs := []signal.Signal{
		makeSignal("sig-unused-001", signal.SignalUnusedInstalledSkill, map[string]any{
			"skill_name": "test-driven-flow",
		}),
	}
	items := []catalog.Item{
		// Matches the skill name tag directly (extra tag from refinement).
		makeItem("skill/test-driven-flow", "Test-Driven Flow", []string{"skill-utilization", "test-driven-flow"}, 3),
		// Also matches on base tag.
		makeItem("skill/skill-adoption", "Skill Adoption Guide", []string{"skill-utilization"}, 2),
	}

	recs := eng.Recommend(sigs, items)

	if len(recs) == 0 {
		t.Fatal("Recommend(unused_installed_skill): expected at least one recommendation")
	}
	for _, r := range recs {
		if r.SignalID == "" || r.CatalogItemID == "" {
			t.Errorf("Recommendation missing citation: SignalID=%q CatalogItemID=%q", r.SignalID, r.CatalogItemID)
		}
	}
}

func TestEngine_FailedHandoff_ProducesRecommendation(t *testing.T) {
	eng := NewEngine()
	sigs := []signal.Signal{
		makeSignal("sig-handoff-001", signal.SignalFailedHandoff, map[string]any{
			"tool_name": "bash",
		}),
	}
	items := []catalog.Item{
		makeItem("skill/retry", "Retry Patterns", []string{"error-recovery", "retry-patterns"}, 4),
	}

	recs := eng.Recommend(sigs, items)

	if len(recs) == 0 {
		t.Fatal("Recommend(failed_handoff): expected at least one recommendation")
	}
	if recs[0].SignalID != "sig-handoff-001" {
		t.Errorf("wrong SignalID: got %q, want %q", recs[0].SignalID, "sig-handoff-001")
	}
}

// ---------------------------------------------------------------------------
// edge-case / contract tests
// ---------------------------------------------------------------------------

func TestEngine_EmptySignals_ReturnsEmpty(t *testing.T) {
	eng := NewEngine()
	items := []catalog.Item{makeItem("skill/foo", "Foo", []string{"shell"}, 3)}
	recs := eng.Recommend(nil, items)
	if len(recs) != 0 {
		t.Errorf("Recommend(nil signals): expected empty, got %d recommendations", len(recs))
	}
}

func TestEngine_EmptyCatalog_ReturnsEmpty(t *testing.T) {
	eng := NewEngine()
	sigs := []signal.Signal{makeSignal("sig-001", signal.SignalStalledNode, nil)}
	recs := eng.Recommend(sigs, nil)
	if len(recs) != 0 {
		t.Errorf("Recommend(nil catalog): expected empty, got %d recommendations", len(recs))
	}
}

func TestEngine_NoMatchingTags_ReturnsEmpty(t *testing.T) {
	eng := NewEngine()
	sigs := []signal.Signal{
		makeSignal("sig-001", signal.SignalStalledNode, nil),
	}
	// Items have tags totally disjoint from stalled_node's candidate tags.
	items := []catalog.Item{
		makeItem("skill/ml", "ML Training Patterns", []string{"gpu", "distributed-training"}, 3),
		makeItem("skill/db", "DB Tuning", []string{"postgres", "indexing"}, 4),
	}

	recs := eng.Recommend(sigs, items)

	if len(recs) != 0 {
		t.Errorf("Recommend(no matching tags): expected 0 recommendations, got %d", len(recs))
	}
}

func TestEngine_MaxPerSignal_Respected(t *testing.T) {
	eng := NewEngine()
	eng.MaxPerSignal = 2

	sigs := []signal.Signal{
		makeSignal("sig-001", signal.SignalStalledNode, map[string]any{"tool_name": "bash"}),
	}
	// 5 items all matching well.
	items := []catalog.Item{
		makeItem("skill/a", "A", []string{"shell", "performance"}, 4),
		makeItem("skill/b", "B", []string{"shell", "performance"}, 3),
		makeItem("skill/c", "C", []string{"shell", "performance"}, 2),
		makeItem("skill/d", "D", []string{"shell", "performance"}, 1),
		makeItem("skill/e", "E", []string{"shell", "performance"}, 5),
	}

	recs := eng.Recommend(sigs, items)

	if len(recs) > 2 {
		t.Errorf("MaxPerSignal=2 violated: got %d recommendations for one signal", len(recs))
	}
}

func TestEngine_ThresholdFilters_LowScores(t *testing.T) {
	eng := NewEngine()
	eng.Threshold = 0.9 // very high; only perfect matches pass

	sigs := []signal.Signal{
		makeSignal("sig-001", signal.SignalStalledNode, nil),
	}
	// Items with partial overlap — Jaccard will be below 0.9.
	items := []catalog.Item{
		makeItem("skill/partial", "Partial", []string{"performance", "other1", "other2", "other3"}, 3),
	}

	recs := eng.Recommend(sigs, items)

	if len(recs) != 0 {
		t.Errorf("Threshold=0.9: expected 0 low-scoring recommendations, got %d", len(recs))
	}
}

// recFingerprint is a comparable snapshot of a Recommendation that excludes
// CreatedAt. We use this for determinism checks because CreatedAt is set to
// time.Now() inside each Recommend call and will legitimately differ across two
// separate invocations. The determinism guarantee covers ordering, scoring, and
// citation assignment — not the wall-clock timestamp.
type recFingerprint struct {
	SignalID      string
	CatalogItemID string
	Title         string
	Reasoning     string
	Score         float64
}

func toFingerprints(recs []Recommendation) []recFingerprint {
	out := make([]recFingerprint, len(recs))
	for i, r := range recs {
		out[i] = recFingerprint{
			SignalID:      r.SignalID,
			CatalogItemID: r.CatalogItemID,
			Title:         r.Title,
			Reasoning:     r.Reasoning,
			Score:         r.Score,
		}
	}
	return out
}

// TestEngine_Determinism asserts that calling Recommend twice on identical inputs
// produces identical ordering, scores, and citations. CreatedAt is excluded from
// comparison because it is set to time.Now() inside each call and will differ
// across separate invocations — that is expected wall-clock behaviour, not a
// non-determinism bug. The regression guard here is the ordering and scoring logic.
func TestEngine_Determinism(t *testing.T) {
	eng := NewEngine()
	sigs := []signal.Signal{
		makeSignal("sig-stall-001", signal.SignalStalledNode, map[string]any{"tool_name": "bash"}),
		makeSignal("sig-dup-001", signal.SignalDuplicateSubagentWork, map[string]any{}),
		makeSignal("sig-unused-001", signal.SignalUnusedInstalledSkill, map[string]any{"skill_name": "alpha"}),
		makeSignal("sig-handoff-001", signal.SignalFailedHandoff, map[string]any{}),
	}
	items := []catalog.Item{
		makeItem("skill/shell", "Shell", []string{"shell", "performance"}, 4),
		makeItem("skill/orch", "Orchestration", []string{"orchestration", "fan-out"}, 3),
		makeItem("skill/retry", "Retry", []string{"error-recovery", "retry-patterns"}, 4),
		makeItem("skill/utilization", "Utilization", []string{"skill-utilization"}, 2),
	}

	recs1 := eng.Recommend(sigs, items)
	recs2 := eng.Recommend(sigs, items)

	fp1 := toFingerprints(recs1)
	fp2 := toFingerprints(recs2)

	b1, err1 := json.Marshal(fp1)
	b2, err2 := json.Marshal(fp2)
	if err1 != nil || err2 != nil {
		t.Fatalf("json.Marshal errors: %v / %v", err1, err2)
	}
	if string(b1) != string(b2) {
		t.Errorf("Recommend is non-deterministic (ordering/scoring/citations differ):\nrun1: %s\nrun2: %s", b1, b2)
	}
	if !reflect.DeepEqual(fp1, fp2) {
		t.Errorf("Recommend reflect.DeepEqual on fingerprints failed between two identical calls")
	}
}

// TestEngine_Determinism_ByteIdentical_WithFixedClock asserts that injecting a
// fixed clock makes every field (including CreatedAt) byte-identical between
// runs. This is the contract Phase 3 snapshot persistence requires when
// storing Recommendation across processes (QA finding Res-4 — caller-injected
// CreatedAt).
func TestEngine_Determinism_ByteIdentical_WithFixedClock(t *testing.T) {
	fixed := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	eng := NewEngine()
	eng.Now = func() time.Time { return fixed }

	sigs := []signal.Signal{
		makeSignal("sig-stall-001", signal.SignalStalledNode, map[string]any{"tool_name": "bash"}),
		makeSignal("sig-dup-001", signal.SignalDuplicateSubagentWork, map[string]any{}),
	}
	items := []catalog.Item{
		makeItem("skill/shell", "Shell", []string{"shell", "performance"}, 4),
		makeItem("skill/orch", "Orchestration", []string{"orchestration", "fan-out"}, 3),
	}

	recs1 := eng.Recommend(sigs, items)
	recs2 := eng.Recommend(sigs, items)

	// Full byte-identical comparison including CreatedAt.
	b1, _ := json.Marshal(recs1)
	b2, _ := json.Marshal(recs2)
	if string(b1) != string(b2) {
		t.Errorf("with fixed clock, two Recommend calls should produce byte-identical JSON:\nrun1: %s\nrun2: %s", b1, b2)
	}
	for i, r := range recs1 {
		if !r.CreatedAt.Equal(fixed) {
			t.Errorf("recs1[%d].CreatedAt = %v, want fixed clock %v", i, r.CreatedAt, fixed)
		}
	}
}

// TestEngine_GAGate_EmptySignalID asserts that a Signal with an empty ID
// cannot produce a Recommendation (the path is unreachable because the engine
// skips signals with empty IDs — documented in engine.go).
func TestEngine_GAGate_EmptySignalID(t *testing.T) {
	eng := NewEngine()

	// Signal with empty ID — the engine must not emit a Recommendation for it.
	badSig := signal.Signal{
		ID:           "", // violates contract
		Type:         signal.SignalStalledNode,
		TraceID:      "trace-001",
		SpanIDs:      []string{"span-001"},
		Evidence:     map[string]any{"tool_name": "bash"},
		Confidence:   0.9,
		EmittedAt:    time.Now(),
		ProviderTier: "full",
	}
	items := []catalog.Item{
		makeItem("skill/shell", "Shell Patterns", []string{"shell", "performance"}, 4),
	}

	recs := eng.Recommend([]signal.Signal{badSig}, items)

	if len(recs) != 0 {
		t.Errorf("GA gate: Recommend with empty-ID signal produced %d recommendations, want 0", len(recs))
	}
}

// TestEngine_AllSignalTypesCovered verifies all four signal types produce at
// least one recommendation when catalog items are well-matched.
func TestEngine_AllSignalTypesCovered(t *testing.T) {
	eng := NewEngine()

	sigs := []signal.Signal{
		makeSignal("sig-stall-01", signal.SignalStalledNode, map[string]any{"tool_name": "bash"}),
		makeSignal("sig-dup-01", signal.SignalDuplicateSubagentWork, map[string]any{}),
		makeSignal("sig-unused-01", signal.SignalUnusedInstalledSkill, map[string]any{"skill_name": "alpha"}),
		makeSignal("sig-handoff-01", signal.SignalFailedHandoff, map[string]any{}),
	}
	// One well-matching item per signal type.
	items := []catalog.Item{
		makeItem("skill/shell-perf", "Shell Performance", []string{"shell", "performance"}, 4),
		makeItem("skill/orchestration", "Orchestration", []string{"orchestration", "fan-out"}, 3),
		makeItem("skill/skill-util", "Skill Utilization", []string{"skill-utilization"}, 3),
		makeItem("skill/retry", "Retry Patterns", []string{"error-recovery", "retry-patterns"}, 4),
	}

	recs := eng.Recommend(sigs, items)

	if len(recs) < 4 {
		t.Errorf("expected at least 4 recommendations (one per signal type), got %d", len(recs))
	}

	// Every recommendation must carry both citations.
	for i, r := range recs {
		if r.SignalID == "" {
			t.Errorf("recs[%d] missing SignalID", i)
		}
		if r.CatalogItemID == "" {
			t.Errorf("recs[%d] missing CatalogItemID", i)
		}
	}
}

// TestEngine_OutputSortOrder verifies the deterministic sort order:
// score DESC, then CatalogItemID ASC.
func TestEngine_OutputSortOrder(t *testing.T) {
	eng := NewEngine()
	eng.MaxPerSignal = 10

	sigs := []signal.Signal{
		makeSignal("sig-001", signal.SignalStalledNode, map[string]any{"tool_name": "bash"}),
	}
	// Two items with identical perfect-overlap tags; sort by CatalogItemID ASC.
	items := []catalog.Item{
		makeItem("skill/zzz-last", "ZZZ", []string{"shell", "performance"}, 3),
		makeItem("skill/aaa-first", "AAA", []string{"shell", "performance"}, 3),
	}

	recs := eng.Recommend(sigs, items)

	if len(recs) < 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(recs))
	}
	if recs[0].CatalogItemID != "skill/aaa-first" {
		t.Errorf("sort order: expected first rec CatalogItemID=%q, got %q", "skill/aaa-first", recs[0].CatalogItemID)
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

// BenchmarkRecommend_100Signals_1000Items measures engine throughput to ensure
// no O(n²) cliff at modest scale. The inner loop is O(signals × items), which
// is acceptable — this benchmark guards against accidental O(n³) or worse.
func BenchmarkRecommend_100Signals_1000Items(b *testing.B) {
	eng := NewEngine()

	sigTypes := []signal.SignalType{
		signal.SignalStalledNode,
		signal.SignalDuplicateSubagentWork,
		signal.SignalUnusedInstalledSkill,
		signal.SignalFailedHandoff,
	}

	sigs := make([]signal.Signal, 100)
	for i := range sigs {
		sigs[i] = makeSignal(
			"sig-bench-"+intToStr(i),
			sigTypes[i%len(sigTypes)],
			map[string]any{"tool_name": "bash", "skill_name": "bench-skill"},
		)
	}

	allTags := [][]string{
		{"shell", "performance"},
		{"orchestration", "fan-out"},
		{"skill-utilization"},
		{"error-recovery", "retry-patterns"},
		{"file-io", "responsiveness"},
	}
	items := make([]catalog.Item, 1000)
	for i := range items {
		items[i] = makeItem(
			"skill/bench-"+intToStr(i),
			"Bench Item "+intToStr(i),
			allTags[i%len(allTags)],
			(i%5)+1,
		)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = eng.Recommend(sigs, items)
	}
}

// intToStr converts an int to a string without importing strconv in test helpers.
// Uses a simple recursive approach sufficient for small integers.
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 10)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	if neg {
		digits = append(digits, '-')
	}
	// reverse
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}

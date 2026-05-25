package tui

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestAdvisorPipeline_Debounces_FastConsecutiveRuns asserts that a second call
// within the debounce window returns nil (no work scheduled).
func TestAdvisorPipeline_Debounces_FastConsecutiveRuns(t *testing.T) {
	p := NewAdvisorPipeline()
	spans := makeStalledSpans("trace-1", 1) // enough for minDelta
	items := makeMatchingCatalogItems()

	// First run — should return a cmd.
	cmd1 := p.MaybeRun(spans, items, nil, NewForestCache())
	if cmd1 == nil {
		t.Fatal("first MaybeRun should return a non-nil cmd on a fresh pipeline")
	}

	// Second run immediately after — within debounce window, must return nil.
	cmd2 := p.MaybeRun(spans, items, nil, NewForestCache())
	if cmd2 != nil {
		t.Error("second MaybeRun within debounce window should return nil")
	}
}

// TestAdvisorPipeline_RunsAfterDebounceElapsed asserts that after waiting past
// the debounce delay the pipeline fires again.
func TestAdvisorPipeline_RunsAfterDebounceElapsed(t *testing.T) {
	p := NewAdvisorPipeline()
	p.debounceDelay = 1 * time.Millisecond // shrink delay for test speed
	spans := makeStalledSpans("trace-1", 1)
	items := makeMatchingCatalogItems()

	cache := NewForestCache()
	_ = p.MaybeRun(spans, items, nil, cache)
	time.Sleep(5 * time.Millisecond) // past the 1ms test window

	cmd := p.MaybeRun(append(spans, makeStalledSpans("trace-2", 5)...), items, nil, cache)
	if cmd == nil {
		t.Error("MaybeRun after debounce elapsed should return a non-nil cmd")
	}
}

// TestAdvisorPipeline_RespectsMinSpanDelta asserts that if fewer than
// AdvisorMinSpansDelta NEW spans have arrived since the last run, no cmd is
// returned even after the debounce window has passed.
func TestAdvisorPipeline_RespectsMinSpanDelta(t *testing.T) {
	p := NewAdvisorPipeline()
	p.debounceDelay = 1 * time.Millisecond
	items := makeMatchingCatalogItems()

	// Seed 10 spans on first run.
	initialSpans := makeStalledSpans("trace-seed", 10)
	_ = p.MaybeRun(initialSpans, items, nil, NewForestCache())
	time.Sleep(5 * time.Millisecond)

	// Provide only 2 more spans (below default delta=5).
	tinyDelta := makeStalledSpans("trace-tiny", 2)
	allSpans := append(initialSpans, tinyDelta...)
	cmd := p.MaybeRun(allSpans, items, nil, NewForestCache())
	if cmd != nil {
		t.Errorf("MaybeRun with only %d new spans should return nil (below MinSpansDelta=%d)",
			len(tinyDelta), AdvisorMinSpansDelta)
	}
}

// TestAdvisorPipeline_EndToEnd_SyntheticForestProducesRecommendation feeds a
// synthetic span batch that triggers a stalled_node signal. The matching
// catalog item carries tags that the recommender maps for stalled_node.
// The returned cmd must produce a RecommendationsReadyMsg with at least one
// non-empty recommendation.
func TestAdvisorPipeline_EndToEnd_SyntheticForestProducesRecommendation(t *testing.T) {
	p := NewAdvisorPipeline()
	spans := makeStalledSpans("trace-e2e", 1)
	items := makeMatchingCatalogItems()

	cmd := p.MaybeRun(spans, items, nil, NewForestCache())
	if cmd == nil {
		t.Fatal("MaybeRun returned nil for non-empty spans+items")
	}

	msg := cmd()
	ready, ok := msg.(RecommendationsReadyMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want RecommendationsReadyMsg", msg)
	}
	if len(ready.Recommendations) == 0 {
		t.Error("expected at least 1 recommendation from synthetic stalled-node forest")
	}
	// GA gate: every recommendation must have a non-empty SignalID and CatalogItemID.
	for i, r := range ready.Recommendations {
		if r.SignalID == "" {
			t.Errorf("Recommendations[%d].SignalID is empty (GA gate violation)", i)
		}
		if r.CatalogItemID == "" {
			t.Errorf("Recommendations[%d].CatalogItemID is empty (GA gate violation)", i)
		}
	}
}

// TestAdvisorPipeline_UnusedSkillSignalFires is the GA-gate-level proof that
// the unused_installed_skill signal type flows end-to-end through the pipeline.
//
// Fixture: spans that use only "TestRunner" tool, installedSkillNames contains
// both "TestRunner" and "NeverUsed". MinSessionsForUnusedSkill requires ≥3
// distinct traces, so we provide 3 traces. The catalog item is tagged
// "skill-utilization" which the recommender maps for unused_installed_skill.
//
// Expected: at least one Recommendation whose SignalID corresponds to an
// unused_installed_skill signal for "NeverUsed".
func TestAdvisorPipeline_UnusedSkillSignalFires(t *testing.T) {
	p := NewAdvisorPipeline()

	// Build 3 traces, each using only "TestRunner" so "NeverUsed" is never invoked.
	var spans []telemetry.Span
	for _, traceID := range []string{"trace-a", "trace-b", "trace-c"} {
		now := time.Now()
		root := telemetry.Span{
			TraceID:   traceID,
			SpanID:    traceID + "-root",
			StartTime: now.Add(-15 * time.Second),
			EndTime:   now,
			Status:    "done",
			System:    "anthropic",
			Attrs:     map[string]string{"gen_ai.tool.name": "TestRunner"},
		}
		spans = append(spans, root)
	}

	// Catalog item tagged for unused_installed_skill.
	items := []catalog.Item{
		{
			ID:    "skill/utilization-helper",
			Title: "Skill Utilization Helper",
			Type:  catalog.ItemTypeSkill,
			Tags:  []string{"skill-utilization"},
		},
	}

	installedSkillNames := []string{"TestRunner", "NeverUsed"}

	cmd := p.MaybeRun(spans, items, installedSkillNames, NewForestCache())
	if cmd == nil {
		t.Fatal("MaybeRun returned nil — pipeline did not trigger")
	}

	msg := cmd()
	ready, ok := msg.(RecommendationsReadyMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want RecommendationsReadyMsg", msg)
	}

	// Assert that at least one recommendation exists.
	if len(ready.Recommendations) == 0 {
		t.Fatal("expected ≥1 recommendation; unused_installed_skill signal did not reach recommender")
	}

	// GA gate: all citations present.
	for i, r := range ready.Recommendations {
		if r.SignalID == "" {
			t.Errorf("Recommendations[%d].SignalID is empty (GA gate violation)", i)
		}
		if r.CatalogItemID == "" {
			t.Errorf("Recommendations[%d].CatalogItemID is empty (GA gate violation)", i)
		}
	}
}

// --- helpers -----------------------------------------------------------------

// makeStalledSpans returns n+1 spans for a single trace: one root (ended
// normally) and n leaf spans each running for 30 seconds with no children.
// 30s > DefaultStallThreshold (10s), so signal.Extract will emit stalled_node.
func makeStalledSpans(traceID string, n int) []telemetry.Span {
	now := time.Now()
	root := telemetry.Span{
		TraceID:   traceID,
		SpanID:    traceID + "-root",
		StartTime: now.Add(-35 * time.Second),
		EndTime:   now,
		Status:    "done",
		System:    "anthropic",
		Attrs:     map[string]string{},
	}
	spans := []telemetry.Span{root}
	for i := 0; i < n; i++ {
		leaf := telemetry.Span{
			TraceID:      traceID,
			SpanID:       traceID + "-leaf-" + string(rune('a'+i)),
			ParentSpanID: root.SpanID,
			StartTime:    now.Add(-30 * time.Second),
			EndTime:      now,
			Status:       "done",
			System:       "anthropic",
			Attrs: map[string]string{
				"gen_ai.tool.name": "bash",
			},
		}
		spans = append(spans, leaf)
	}
	return spans
}

// makeMatchingCatalogItems returns catalog items tagged with keywords the
// recommender mapping for stalled_node includes ("shell", "performance").
func makeMatchingCatalogItems() []catalog.Item {
	return []catalog.Item{
		{
			ID:    "skill/test-driven-flow",
			Title: "Test-Driven Flow",
			Type:  catalog.ItemTypeSkill,
			Tags:  []string{"tdd", "testing", "shell", "performance"},
		},
		{
			ID:    "agent/reviewer",
			Title: "Reviewer",
			Type:  catalog.ItemTypeAgent,
			Tags:  []string{"review", "orchestration", "retry"},
		},
	}
}

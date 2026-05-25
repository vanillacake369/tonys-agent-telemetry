package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/recommender"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// makeRecommendations builds n Recommendation values with predictable titles
// and non-empty citations (GA gate: SignalID + CatalogItemID both required).
func makeRecommendations(n int) []recommender.Recommendation {
	recs := make([]recommender.Recommendation, n)
	for i := 0; i < n; i++ {
		recs[i] = recommender.Recommendation{
			SignalID:      "sig-" + string(rune('a'+i)),
			CatalogItemID: "skill/item-" + string(rune('a'+i)),
			Title:         "Recommendation Title " + string(rune('A'+i)),
			Reasoning:     "stalled_node in trace trace-1 triggered this",
			Score:         0.9 - float64(i)*0.1,
			CreatedAt:     time.Now(),
		}
	}
	return recs
}

// TestSkillsTab_RendersAdvisorRecommendations injects 3 recommendations and
// asserts that each title and the "Advisor" header appear in View().
func TestSkillsTab_RendersAdvisorRecommendations(t *testing.T) {
	s := NewSkillsTab()
	s = s.SetSize(120, 40).(SkillsTab)

	// Pre-load catalog so the catalog section renders (avoids "Loading catalog" dominating).
	items := makeCatalogItems(catalog.MinViableEntries)
	s, _ = updateSkillsTab(t, s, CatalogLoadedMsg{Items: items, FetchedAt: time.Now()})

	recs := makeRecommendations(3)
	s, _ = updateSkillsTab(t, s, RecommendationsReadyMsg{Recommendations: recs})

	view := s.View()
	if !strings.Contains(view, "Advisor") {
		t.Errorf("View() does not contain 'Advisor' header.\nView excerpt: %.800s", view)
	}
	for i, r := range recs {
		if !strings.Contains(view, r.Title) {
			t.Errorf("View() missing recommendation[%d] title %q.\nView excerpt: %.800s", i, r.Title, view)
		}
	}
}

// TestSkillsTab_RendersAdvisorEmptyState asserts that when no recommendations
// are present, the single-line empty-state message appears.
func TestSkillsTab_RendersAdvisorEmptyState(t *testing.T) {
	s := NewSkillsTab()
	s = s.SetSize(120, 40).(SkillsTab)

	view := s.View()
	if !strings.Contains(view, "no signals match catalog yet") {
		t.Errorf("View() does not show empty-state line.\nView excerpt: %.800s", view)
	}
}

// TestSkillsTab_AdvisorShowsCitations asserts that at least one recommendation's
// rendered output contains the SignalID so users can trace the evidence chain.
func TestSkillsTab_AdvisorShowsCitations(t *testing.T) {
	s := NewSkillsTab()
	s = s.SetSize(120, 40).(SkillsTab)

	items := makeCatalogItems(catalog.MinViableEntries)
	s, _ = updateSkillsTab(t, s, CatalogLoadedMsg{Items: items, FetchedAt: time.Now()})

	recs := makeRecommendations(1)
	s, _ = updateSkillsTab(t, s, RecommendationsReadyMsg{Recommendations: recs})

	view := s.View()
	sig := recs[0].SignalID
	if !strings.Contains(view, sig) {
		t.Errorf("View() does not contain SignalID %q for evidence chain.\nView excerpt: %.800s", sig, view)
	}
}

// TestSkillsTab_AdvisorReplacesOnSecondMsg injects one set of recommendations
// then a different set; only the second set should appear in View().
func TestSkillsTab_AdvisorReplacesOnSecondMsg(t *testing.T) {
	s := NewSkillsTab()
	s = s.SetSize(120, 40).(SkillsTab)

	items := makeCatalogItems(catalog.MinViableEntries)
	s, _ = updateSkillsTab(t, s, CatalogLoadedMsg{Items: items, FetchedAt: time.Now()})

	first := makeRecommendations(1)
	s, _ = updateSkillsTab(t, s, RecommendationsReadyMsg{Recommendations: first})

	second := []recommender.Recommendation{
		{
			SignalID:      "sig-new",
			CatalogItemID: "skill/new-item",
			Title:         "New Recommendation After Replace",
			Reasoning:     "replacement test",
			Score:         0.99,
			CreatedAt:     time.Now(),
		},
	}
	s, _ = updateSkillsTab(t, s, RecommendationsReadyMsg{Recommendations: second})

	view := s.View()
	if strings.Contains(view, first[0].Title) {
		t.Errorf("View() still shows first-set title %q after replacement", first[0].Title)
	}
	if !strings.Contains(view, second[0].Title) {
		t.Errorf("View() does not show second-set title %q after replacement", second[0].Title)
	}
}

// TestApp_EndToEnd_SpanBatchTriggersRecommendation instantiates the App,
// injects SpanBatchMsg with synthetic stalled-node spans and a CatalogLoadedMsg
// with matching items, then triggers the pipeline and asserts the Advisor
// section in the Skills tab contains at least one recommendation.
//
// Because the real debounce is 500ms, this test wires a zero-delay pipeline
// via the App field directly to avoid time.Sleep in CI.
func TestApp_EndToEnd_SpanBatchTriggersRecommendation(t *testing.T) {
	a := NewApp()
	a.width, a.height = 120, 40
	a = a.propagateSize()

	// Zero out the debounce delay so MaybeRun fires immediately.
	a.advisor.debounceDelay = 0

	spans := makeStalledSpans("trace-e2e", 5)
	items := makeMatchingCatalogItems()

	// Load catalog into SkillsTab.
	updated, _ := a.Update(CatalogLoadedMsg{Items: items, FetchedAt: time.Now()})
	a = updated.(App)

	// Inject span batch — pipeline fires synchronously because debounceDelay=0.
	updated, cmd := a.Update(SpanBatchMsg{Spans: spans})
	a = updated.(App)

	// Execute the pipeline cmd if one was produced.
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			updated2, _ := a.Update(msg)
			a = updated2.(App)
		}
	}

	// Switch to Skills tab and render.
	a.activeTab = TabSkills
	view := a.View()

	if !strings.Contains(view, "Advisor") {
		t.Errorf("App.View() Skills tab does not contain 'Advisor' after pipeline run.\nView excerpt: %.800s", view)
	}
}

// TestApp_EndToEnd_FailedHandoffProducesRecommendation builds a synthetic forest
// containing a failed_handoff pattern — an error span followed by a retry of the
// same tool at the same depth — and asserts the advisor produces ≥1 recommendation
// citing the failed_handoff signal. The catalog item is tagged "error-recovery"
// which maps to failed_handoff in the recommender.
func TestApp_EndToEnd_FailedHandoffProducesRecommendation(t *testing.T) {
	a := NewApp()
	a.width, a.height = 120, 40
	a = a.propagateSize()

	// Zero debounce so MaybeRun fires immediately.
	a.advisor.debounceDelay = 0

	spans := makeFailedHandoffSpans("trace-fh")
	items := makeFailedHandoffCatalogItems()

	// Load catalog.
	updated, _ := a.Update(CatalogLoadedMsg{Items: items, FetchedAt: time.Now()})
	a = updated.(App)

	// Inject span batch.
	updated, cmd := a.Update(SpanBatchMsg{Spans: spans})
	a = updated.(App)

	// Execute the pipeline cmd.
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			updated2, _ := a.Update(msg)
			a = updated2.(App)
		}
	}

	a.activeTab = TabSkills
	view := a.View()
	if !strings.Contains(view, "Advisor") {
		t.Errorf("Skills tab does not contain 'Advisor' after failed_handoff pipeline run.\nView: %.800s", view)
	}
}

// TestApp_EndToEnd_DuplicateWorkProducesRecommendation builds a synthetic forest
// with two sibling subagents performing overlapping tool sequences (Jaccard ≥ 0.8)
// and asserts the advisor produces ≥1 recommendation with an orchestration-tagged
// catalog item (the tag that maps to duplicate_subagent_work).
func TestApp_EndToEnd_DuplicateWorkProducesRecommendation(t *testing.T) {
	a := NewApp()
	a.width, a.height = 120, 40
	a = a.propagateSize()

	// Zero debounce so MaybeRun fires immediately.
	a.advisor.debounceDelay = 0

	spans := makeDuplicateWorkSpans("trace-dup")
	items := makeDuplicateWorkCatalogItems()

	// Load catalog.
	updated, _ := a.Update(CatalogLoadedMsg{Items: items, FetchedAt: time.Now()})
	a = updated.(App)

	// Inject span batch.
	updated, cmd := a.Update(SpanBatchMsg{Spans: spans})
	a = updated.(App)

	// Execute the pipeline cmd.
	if cmd != nil {
		msg := cmd()
		if msg != nil {
			updated2, _ := a.Update(msg)
			a = updated2.(App)
		}
	}

	a.activeTab = TabSkills
	view := a.View()
	if !strings.Contains(view, "Advisor") {
		t.Errorf("Skills tab does not contain 'Advisor' after duplicate_subagent_work pipeline run.\nView: %.800s", view)
	}
}

// --- additional helper -------------------------------------------------------

// stalledSpanForTrace builds a minimal Span that will produce a stalled_node
// signal at signal.Extract time. Exported so other test files can reuse it.
func stalledSpanForTrace(traceID, spanID string) telemetry.Span {
	now := time.Now()
	return telemetry.Span{
		TraceID:   traceID,
		SpanID:    spanID,
		StartTime: now.Add(-30 * time.Second),
		EndTime:   now,
		Status:    "done",
		System:    "anthropic",
		Attrs:     map[string]string{"gen_ai.tool.name": "bash"},
	}
}

// makeFailedHandoffSpans builds a minimal span set for a single trace that
// triggers a failed_handoff signal. Structure:
//
//	root
//	  ├── child-error  (status=error, tool=retry-tool, ends at T+5s)
//	  └── child-retry  (status=done,  tool=retry-tool, starts at T+10s)
//
// The retry child starts after the error child ends, satisfying SIGNALS_SPEC §3.4.
func makeFailedHandoffSpans(traceID string) []telemetry.Span {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	root := telemetry.Span{
		TraceID:   traceID,
		SpanID:    traceID + "-root",
		StartTime: base,
		EndTime:   base.Add(20 * time.Second),
		Status:    "done",
		System:    "anthropic",
		Attrs:     map[string]string{},
	}
	errorSpan := telemetry.Span{
		TraceID:      traceID,
		SpanID:       traceID + "-err",
		ParentSpanID: root.SpanID,
		StartTime:    base.Add(1 * time.Second),
		EndTime:      base.Add(5 * time.Second),
		Status:       "error",
		System:       "anthropic",
		Attrs:        map[string]string{"gen_ai.tool.name": "retry-tool"},
	}
	retrySpan := telemetry.Span{
		TraceID:      traceID,
		SpanID:       traceID + "-retry",
		ParentSpanID: root.SpanID,
		StartTime:    base.Add(10 * time.Second),
		EndTime:      base.Add(15 * time.Second),
		Status:       "done",
		System:       "anthropic",
		Attrs:        map[string]string{"gen_ai.tool.name": "retry-tool"},
	}
	return []telemetry.Span{root, errorSpan, retrySpan}
}

// makeFailedHandoffCatalogItems returns a catalog item tagged "error-recovery"
// which the recommender maps for the failed_handoff signal.
func makeFailedHandoffCatalogItems() []catalog.Item {
	return []catalog.Item{
		{
			ID:    "agent/reviewer",
			Title: "Error Recovery Agent",
			Type:  catalog.ItemTypeAgent,
			Tags:  []string{"error-recovery", "retry-patterns"},
		},
	}
}

// makeDuplicateWorkSpans builds a minimal span set for a single trace that
// triggers a duplicate_subagent_work signal. Two sibling spans each having
// the same set of tool calls in their subtrees (Jaccard = 1.0 ≥ 0.8).
//
//	root
//	  ├── subagent-a  (tool=bash → tool=read)
//	  └── subagent-b  (tool=bash → tool=read)
func makeDuplicateWorkSpans(traceID string) []telemetry.Span {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	root := telemetry.Span{
		TraceID:   traceID,
		SpanID:    traceID + "-root",
		StartTime: base,
		EndTime:   base.Add(30 * time.Second),
		Status:    "done",
		System:    "anthropic",
		Attrs:     map[string]string{},
	}
	// Subagent A with two tool calls.
	subA := telemetry.Span{
		TraceID:      traceID,
		SpanID:       traceID + "-sub-a",
		ParentSpanID: root.SpanID,
		StartTime:    base.Add(1 * time.Second),
		EndTime:      base.Add(10 * time.Second),
		Status:       "done",
		System:       "anthropic",
		Attrs:        map[string]string{"gen_ai.tool.name": "bash"},
	}
	subAChild := telemetry.Span{
		TraceID:      traceID,
		SpanID:       traceID + "-sub-a-read",
		ParentSpanID: subA.SpanID,
		StartTime:    base.Add(2 * time.Second),
		EndTime:      base.Add(5 * time.Second),
		Status:       "done",
		System:       "anthropic",
		Attrs:        map[string]string{"gen_ai.tool.name": "read"},
	}
	// Subagent B with identical tool calls (Jaccard = 1.0).
	subB := telemetry.Span{
		TraceID:      traceID,
		SpanID:       traceID + "-sub-b",
		ParentSpanID: root.SpanID,
		StartTime:    base.Add(1 * time.Second),
		EndTime:      base.Add(10 * time.Second),
		Status:       "done",
		System:       "anthropic",
		Attrs:        map[string]string{"gen_ai.tool.name": "bash"},
	}
	subBChild := telemetry.Span{
		TraceID:      traceID,
		SpanID:       traceID + "-sub-b-read",
		ParentSpanID: subB.SpanID,
		StartTime:    base.Add(2 * time.Second),
		EndTime:      base.Add(5 * time.Second),
		Status:       "done",
		System:       "anthropic",
		Attrs:        map[string]string{"gen_ai.tool.name": "read"},
	}
	return []telemetry.Span{root, subA, subAChild, subB, subBChild}
}

// makeDuplicateWorkCatalogItems returns a catalog item tagged "orchestration"
// which the recommender maps for the duplicate_subagent_work signal.
func makeDuplicateWorkCatalogItems() []catalog.Item {
	return []catalog.Item{
		{
			ID:    "agent/orchestration-helper",
			Title: "Orchestration Helper",
			Type:  catalog.ItemTypeAgent,
			Tags:  []string{"orchestration", "fan-out"},
		},
	}
}

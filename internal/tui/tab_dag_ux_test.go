// Package tui — tab_dag_ux_test.go: TDD tests for the three UX fixes from
// smoke-test user complaints (τ-1 help banners, τ-2 overview mode, τ-3 search bar).
package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// ── τ-1: Help banners ─────────────────────────────────────────────────────

// TestDAGTab_TracesView_ShowsHelpBanner asserts that the traces view (top-level
// list) contains the key shortcuts that describe what actions are available.
func TestDAGTab_TracesView_ShowsHelpBanner(t *testing.T) {
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: []telemetry.Span{
		{TraceID: "t1", SpanID: "a", System: "anthropic"},
	}})
	d = tab.(*DAGTab)
	if d.mode != dagViewTraces {
		t.Fatalf("expected dagViewTraces mode, got %d", d.mode)
	}

	v := d.View()
	plain := stripAnsi(v)

	for _, want := range []string{"/", "search", "Enter"} {
		if !strings.Contains(plain, want) {
			t.Errorf("traces help banner missing %q; rendered:\n%s", want, truncate(plain, 400))
		}
	}
}

// TestDAGTab_GraphView_ShowsHelpBanner asserts that the graph view contains
// both "g: overview" and "n/N" navigation hints.
func TestDAGTab_GraphView_ShowsHelpBanner(t *testing.T) {
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: []telemetry.Span{
		{TraceID: "t1", SpanID: "a", System: "anthropic"},
		{TraceID: "t1", SpanID: "b", ParentSpanID: "a", System: "anthropic"},
	}})
	d = tab.(*DAGTab)
	// Open graph view.
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)
	if d.mode != dagViewGraph {
		t.Fatalf("expected dagViewGraph mode, got %d", d.mode)
	}

	v := d.View()
	plain := stripAnsi(v)

	if !strings.Contains(plain, "g: overview") {
		t.Errorf("graph help banner missing 'g: overview'; rendered:\n%s", truncate(plain, 600))
	}
	if !strings.Contains(plain, "n/N") {
		t.Errorf("graph help banner missing 'n/N'; rendered:\n%s", truncate(plain, 600))
	}
}

// ── τ-2: Overview mode ────────────────────────────────────────────────────

// TestDAGTab_PressingG_TogglesOverviewMode sends 'g' from graph mode and
// asserts the mode transitions to dagViewOverview; 'g' again returns to graph.
func TestDAGTab_PressingG_TogglesOverviewMode(t *testing.T) {
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: []telemetry.Span{
		{TraceID: "t1", SpanID: "a"},
	}})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → graph
	d = tab.(*DAGTab)
	if d.mode != dagViewGraph {
		t.Fatalf("pre-condition: expected dagViewGraph, got %d", d.mode)
	}

	// Press g: should enter overview.
	tab, _ = d.Update(keyMsg("g"))
	d = tab.(*DAGTab)
	if d.mode != dagViewOverview {
		t.Errorf("after g: mode = %d, want dagViewOverview (%d)", d.mode, dagViewOverview)
	}

	// Press g again: should return to graph.
	tab, _ = d.Update(keyMsg("g"))
	d = tab.(*DAGTab)
	if d.mode != dagViewGraph {
		t.Errorf("after second g: mode = %d, want dagViewGraph (%d)", d.mode, dagViewGraph)
	}
}

// TestDAGTab_OverviewRendersAllTraceNodes creates a trace with 5 spans and
// asserts the overview render contains one line indicator per span.
func TestDAGTab_OverviewRendersAllTraceNodes(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root", Status: "done",
			Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "s1", ParentSpanID: "root", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "t", SpanID: "s2", ParentSpanID: "root", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Read"}},
		{TraceID: "t", SpanID: "s3", ParentSpanID: "s2", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Edit"}},
		{TraceID: "t", SpanID: "s4", ParentSpanID: "s2", Status: "error",
			Attrs: map[string]string{"gen_ai.tool.name": "Write"}},
	}

	d := NewDAGTab().SetSize(120, 40).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → graph
	d = tab.(*DAGTab)
	tab, _ = d.Update(keyMsg("g")) // → overview
	d = tab.(*DAGTab)

	if d.mode != dagViewOverview {
		t.Fatalf("expected dagViewOverview, got %d", d.mode)
	}

	v := d.View()
	plain := stripAnsi(v)

	// Count tree connector markers — one per non-root span (depth > 0).
	connectors := strings.Count(plain, "└─")
	if connectors < 4 {
		t.Errorf("expected at least 4 '└─' connectors for 4 child spans, got %d; rendered:\n%s",
			connectors, truncate(plain, 800))
	}

	// Also check all span labels appear.
	for _, want := range []string{"chat", "Bash", "Read", "Edit", "Write"} {
		if !strings.Contains(plain, want) {
			t.Errorf("overview missing %q; rendered:\n%s", want, truncate(plain, 800))
		}
	}
}

// TestDAGTab_OverviewRespectsResize asserts that at width=80 no line in the
// overview render overflows (plain-text width <= 80).
func TestDAGTab_OverviewRespectsResize(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root", Status: "done",
			Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "s1", ParentSpanID: "root", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "t", SpanID: "s2", ParentSpanID: "root", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Read"}},
	}

	d := NewDAGTab().SetSize(80, 30).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter}) // → graph
	d = tab.(*DAGTab)
	tab, _ = d.Update(keyMsg("g")) // → overview
	d = tab.(*DAGTab)

	v := d.View()
	for i, line := range strings.Split(v, "\n") {
		w := len([]rune(stripAnsi(line)))
		if w > 80 {
			t.Errorf("line %d width=%d > 80: %q", i, w, stripAnsi(line))
		}
	}
}

// ── τ-3: Search bar ────────────────────────────────────────────────────────

// TestDAGTab_SearchInputModeShowsBar presses '/' in graph mode and asserts
// the view now shows the "/" prefix input bar.
func TestDAGTab_SearchInputModeShowsBar(t *testing.T) {
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: []telemetry.Span{
		{TraceID: "t", SpanID: "a", Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
	}})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	// Press '/' to enter search input mode.
	tab, _ = d.Update(keyMsg("/"))
	d = tab.(*DAGTab)
	if !d.dagSearchInputting {
		t.Fatalf("expected dagSearchInputting=true after '/'")
	}

	v := d.View()
	plain := stripAnsi(v)
	// The search bar must show a "/" prefix with the cursor placeholder.
	if !strings.Contains(plain, "/") {
		t.Errorf("search input bar missing '/'; rendered:\n%s", truncate(plain, 400))
	}
	if !strings.Contains(plain, "_") {
		t.Errorf("search input bar missing cursor '_'; rendered:\n%s", truncate(plain, 400))
	}
}

// TestDAGTab_SearchConfirmedShowsMatchCount sets up a trace with 3 "bash"-
// matching spans, confirms a search, and asserts "match 1 of 3" in the render.
func TestDAGTab_SearchConfirmedShowsMatchCount(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "r", Status: "done",
			Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "b1", ParentSpanID: "r", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "t", SpanID: "b2", ParentSpanID: "b1", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "t", SpanID: "b3", ParentSpanID: "b2", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
	}

	d := NewDAGTab().SetSize(120, 40).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	// Directly set confirmed search state to avoid typing simulation.
	d.dagSearchQuery = "bash"
	d.dagSearchActive = true
	d.dagSearchBuf = ""
	d.dagSearchInputting = false
	d.rebuildSearchMatches()
	d.dagSearchIdx = 0
	d.graphCache = ""
	d.graphCacheKey = ""

	v := d.View()
	plain := stripAnsi(v)

	if !strings.Contains(plain, "match 1 of 3") {
		t.Errorf("search bar should show 'match 1 of 3'; rendered:\n%s", truncate(plain, 800))
	}
}

// TestDAGTab_SearchNoMatchesMessage asserts that a search with no hits shows
// "no matches" in the rendered view.
func TestDAGTab_SearchNoMatchesMessage(t *testing.T) {
	d := NewDAGTab().SetSize(120, 40).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: []telemetry.Span{
		{TraceID: "t", SpanID: "a", Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
	}})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	d.dagSearchQuery = "nonexistent"
	d.dagSearchActive = true
	d.dagSearchBuf = ""
	d.dagSearchInputting = false
	d.rebuildSearchMatches()
	d.dagSearchIdx = 0
	d.graphCache = ""
	d.graphCacheKey = ""

	v := d.View()
	plain := stripAnsi(v)

	if !strings.Contains(plain, "no matches") {
		t.Errorf("search bar should show 'no matches'; rendered:\n%s", truncate(plain, 600))
	}
}

// TestDAGTab_NShiftsMatchCounter presses 'n' after confirming a search and
// asserts the counter advances to "match 2 of 3".
func TestDAGTab_NShiftsMatchCounter(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "r", Status: "done",
			Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "b1", ParentSpanID: "r", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "t", SpanID: "b2", ParentSpanID: "b1", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "t", SpanID: "b3", ParentSpanID: "b2", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
	}

	d := NewDAGTab().SetSize(120, 40).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	// Set confirmed search with 3 matches.
	d.dagSearchQuery = "bash"
	d.dagSearchActive = true
	d.rebuildSearchMatches()
	d.dagSearchIdx = 0
	d.graphCache = ""
	d.graphCacheKey = ""

	// Press 'n' to advance.
	tab, _ = d.Update(keyMsg("n"))
	d = tab.(*DAGTab)

	v := d.View()
	plain := stripAnsi(v)

	if !strings.Contains(plain, "match 2 of 3") {
		t.Errorf("after n: search bar should show 'match 2 of 3'; rendered:\n%s", truncate(plain, 800))
	}
}

// ── Additional: overview cursor navigation ─────────────────────────────────

// TestDAGTab_OverviewJKNavigatesCursor verifies j/k move the cursor in
// overview mode.
func TestDAGTab_OverviewJKNavigatesCursor(t *testing.T) {
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "a"},
		{TraceID: "t", SpanID: "b", ParentSpanID: "a"},
		{TraceID: "t", SpanID: "c", ParentSpanID: "b"},
	}
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)
	tab, _ = d.Update(keyMsg("g"))
	d = tab.(*DAGTab)
	if d.mode != dagViewOverview {
		t.Fatalf("expected overview mode")
	}
	if d.overviewCursor != 0 {
		t.Fatalf("initial overviewCursor should be 0, got %d", d.overviewCursor)
	}
	tab, _ = d.Update(keyMsg("j"))
	d = tab.(*DAGTab)
	if d.overviewCursor != 1 {
		t.Errorf("after j: overviewCursor = %d, want 1", d.overviewCursor)
	}
	tab, _ = d.Update(keyMsg("k"))
	d = tab.(*DAGTab)
	if d.overviewCursor != 0 {
		t.Errorf("after k: overviewCursor = %d, want 0", d.overviewCursor)
	}
}

// TestDAGTab_OverviewEnterReturnsToGraphAtSelectedNode asserts that pressing
// Enter in overview mode returns to dagViewGraph with nodeCursor set to the
// selected span's index.
func TestDAGTab_OverviewEnterReturnsToGraphAtSelectedNode(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "a"},
		{TraceID: "t", SpanID: "b", ParentSpanID: "a"},
		{TraceID: "t", SpanID: "c", ParentSpanID: "b"},
	}
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)
	tab, _ = d.Update(keyMsg("g"))
	d = tab.(*DAGTab)

	// Move cursor to 2nd line (index 1 → span "b").
	tab, _ = d.Update(keyMsg("j"))
	d = tab.(*DAGTab)
	if d.overviewCursor != 1 {
		t.Fatalf("cursor should be 1, got %d", d.overviewCursor)
	}

	// Press Enter to return to graph.
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)
	if d.mode != dagViewGraph {
		t.Errorf("expected dagViewGraph after Enter, got %d", d.mode)
	}
	// nodeCursor should point to span "b" (index 1 in flat DFS order).
	if d.nodeCursor != 1 {
		t.Errorf("nodeCursor = %d, want 1 (span b)", d.nodeCursor)
	}
}

// TestDAGTab_OverviewDuration asserts that spans with a known duration show
// the duration in the overview render.
func TestDAGTab_OverviewDuration(t *testing.T) {
	start := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)
	end := start.Add(2100 * time.Millisecond) // 2.1s
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root", StartTime: start, EndTime: end,
			Attrs: map[string]string{"gen_ai.operation.name": "session"}},
	}
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)
	tab, _ = d.Update(keyMsg("g"))
	d = tab.(*DAGTab)

	v := d.View()
	plain := stripAnsi(v)
	if !strings.Contains(plain, "2.1s") {
		t.Errorf("overview should show duration '2.1s'; rendered:\n%s", truncate(plain, 400))
	}
}

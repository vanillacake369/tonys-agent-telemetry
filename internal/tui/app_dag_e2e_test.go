package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// These tests exercise App.Update END-TO-END with key messages targeting
// the DAG tab. Previous DAG tests called DAGTab.Update directly, which
// bypasses the entire App.Update routing layer where the "open graph"
// bug actually lives in production.

func TestAppE2E_EnterOpensGraphFromTracesList(t *testing.T) {
	a := NewApp()
	a.width, a.height = 120, 30
	a = a.propagateSize()

	// Switch to DAG tab.
	a.activeTab = TabDAG

	// Deliver a batch of spans via the App-level routing.
	updated, _ := a.Update(SpanBatchMsg{Spans: []telemetry.Span{
		{TraceID: "trace-1", SpanID: "u1", System: "anthropic"},
		{TraceID: "trace-1", SpanID: "u2", ParentSpanID: "u1", System: "anthropic"},
	}})
	a = updated.(App)

	d := a.tabs[TabDAG].(*DAGTab)
	if d.mode != dagViewTraces {
		t.Fatalf("after batch: mode = %d, want dagViewTraces (%d)", d.mode, dagViewTraces)
	}
	if len(d.traces) != 1 {
		t.Fatalf("traces rebuilt: got %d, want 1", len(d.traces))
	}

	// Press Enter via App.Update.
	updated, _ = a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = updated.(App)

	// THE CRITICAL ASSERTION: DAGTab.mode should have transitioned to
	// dagViewGraph. If routing dropped the KeyMsg, this fails.
	d = a.tabs[TabDAG].(*DAGTab)
	if d.mode != dagViewGraph {
		t.Errorf("after App.Update(Enter): mode = %d, want dagViewGraph (%d)", d.mode, dagViewGraph)
	}
	if d.activeTrace != "trace-1" {
		t.Errorf("activeTrace = %q, want trace-1", d.activeTrace)
	}
}

// TestAppE2E_FullFlow_5ThenEnter replays the user's exact failure path:
// press '5' to switch to the DAG tab, then press Enter to open the graph.
// Distinct from TestAppE2E_EnterOpensGraphFromTracesList which set
// activeTab manually.
func TestAppE2E_FullFlow_5ThenEnter(t *testing.T) {
	var a tea.Model = NewApp()
	app := a.(App)
	app.width, app.height = 120, 30
	app = app.propagateSize()
	a = app

	// Spans arrive first (backfill).
	a, _ = a.Update(SpanBatchMsg{Spans: []telemetry.Span{
		{TraceID: "t1", SpanID: "a", System: "anthropic"},
		{TraceID: "t1", SpanID: "b", ParentSpanID: "a", System: "anthropic"},
	}})

	// User presses '5'.
	a, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	if got := a.(App).activeTab; got != TabDAG {
		t.Fatalf("after '5': activeTab = %d, want %d (TabDAG)", got, TabDAG)
	}

	d := a.(App).tabs[TabDAG].(*DAGTab)
	if d.mode != dagViewTraces {
		t.Fatalf("on DAG tab arrival: mode = %d, want dagViewTraces", d.mode)
	}
	if len(d.traces) != 1 {
		t.Fatalf("traces should be populated; got %d", len(d.traces))
	}

	// User presses Enter.
	a, _ = a.Update(tea.KeyMsg{Type: tea.KeyEnter})

	d = a.(App).tabs[TabDAG].(*DAGTab)
	if d.mode != dagViewGraph {
		t.Errorf("after Enter on DAG tab: mode = %d, want dagViewGraph (%d). "+
			"This is the user-reported 'open graph 작동 안 함' bug.",
			d.mode, dagViewGraph)
	}

	// Critically: also verify that App.View() renders the graph view,
	// not the traces table. The user sees the rendered string — internal
	// state being correct does not guarantee the screen updates.
	view := a.(App).View()
	// Graph mode help contains both "enter/l" and "detail" — these are
	// unique to the graph view's key bar.
	if !strings.Contains(view, "detail") || !strings.Contains(view, "yank") {
		t.Errorf("App.View() after Enter does not contain graph-mode help "+
			"('detail' + 'yank'). User sees stale render. View dump:\n%s", view)
	}
}

func TestAppE2E_JKAdvancesCursorOnDAGTab(t *testing.T) {
	a := NewApp()
	a.width, a.height = 120, 30
	a = a.propagateSize()
	a.activeTab = TabDAG

	// Three traces.
	updated, _ := a.Update(SpanBatchMsg{Spans: []telemetry.Span{
		{TraceID: "t1", SpanID: "a"},
		{TraceID: "t2", SpanID: "b"},
		{TraceID: "t3", SpanID: "c"},
	}})
	a = updated.(App)

	updated, _ = a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	a = updated.(App)

	d := a.tabs[TabDAG].(*DAGTab)
	if d.traceCursor != 1 {
		t.Errorf("after 'j': cursor = %d, want 1", d.traceCursor)
	}
}

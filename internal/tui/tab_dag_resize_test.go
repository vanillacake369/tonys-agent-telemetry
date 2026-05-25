package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

func dagFromSpans(t *testing.T, spans []telemetry.Span, w, h int) *DAGTab {
	t.Helper()
	d := NewDAGTab().SetSize(w, h).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return tab.(*DAGTab)
}

// maxLineW computes the longest rendered line in VISIBLE COLUMNS — i.e.
// rune count after ANSI stripping. byte length is wrong here because
// box-drawing characters (┌─┐│) are 3 bytes each in UTF-8.
func maxLineW(s string) int {
	max := 0
	for _, line := range strings.Split(s, "\n") {
		w := len([]rune(stripAnsi(line)))
		if w > max {
			max = w
		}
	}
	return max
}

// TestDAGResize_InvalidatesCacheOnSizeChange — when the panel shrinks
// AND the graph is large enough that nodeW must change, the rendered
// output must reflow. Small graphs (which already fit) intentionally
// stay the same on resize — that's correct behavior.
func TestDAGResize_InvalidatesCacheOnSizeChange(t *testing.T) {
	// Deep multi-branch: maxCol=4 needs ~126 cols at default nodeW=22.
	// At 60-col panel, nodeW must shrink to fit.
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "n0"},
		{TraceID: "t", SpanID: "n1", ParentSpanID: "n0"},
		{TraceID: "t", SpanID: "n1b", ParentSpanID: "n0"},
		{TraceID: "t", SpanID: "n2", ParentSpanID: "n1"},
		{TraceID: "t", SpanID: "n2b", ParentSpanID: "n1"},
		{TraceID: "t", SpanID: "n3", ParentSpanID: "n2"},
		{TraceID: "t", SpanID: "n3b", ParentSpanID: "n2"},
		{TraceID: "t", SpanID: "n4", ParentSpanID: "n3"},
		{TraceID: "t", SpanID: "n4b", ParentSpanID: "n3"},
	}
	d := dagFromSpans(t, spans, 200, 40)
	wide := d.renderGraph(196)
	wideW := maxLineW(wide)

	tab := d.SetSize(60, 20)
	d = tab.(*DAGTab)
	narrow := d.renderGraph(56)
	narrowW := maxLineW(narrow)

	t.Logf("wide render: %d cols ; narrow render: %d cols", wideW, narrowW)
	if wide == narrow {
		t.Error("identical output for 196 vs 56 — cache or layout not size-aware")
	}
	if wideW <= narrowW {
		t.Errorf("wide should be wider than narrow: wide=%d narrow=%d", wideW, narrowW)
	}
	if narrowW > 60 {
		t.Errorf("narrow render width %d exceeds 60-col panel", narrowW)
	}
}

// TestDAGResize_StaysWithinPanelWidth — content rendered for width=W
// must not exceed W cols on any line, otherwise lipgloss wraps the box
// chars and the user sees garbled output after a tmux split or pane
// shrink.
func TestDAGResize_StaysWithinPanelWidth(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root", System: "anthropic"},
		{TraceID: "t", SpanID: "c1", ParentSpanID: "root", System: "anthropic"},
		{TraceID: "t", SpanID: "c2", ParentSpanID: "root", System: "anthropic"},
		{TraceID: "t", SpanID: "c3", ParentSpanID: "root", System: "anthropic"},
	}
	for _, panel := range []int{40, 60, 80, 120, 160} {
		d := dagFromSpans(t, spans, panel, 30)
		out := d.renderGraph(panel - 4)
		got := maxLineW(out)
		if got > panel {
			t.Errorf("panel=%d: rendered line width=%d, exceeds panel — graph will overflow and lipgloss will wrap", panel, got)
		} else {
			t.Logf("panel=%d → max line width %d (fits)", panel, got)
		}
	}
}

// TestDAGResize_DeepChainAtNarrowWidth — a depth-6 chain with default
// 22-char boxes needs ~132 cols. At 80-col width the graph must
// gracefully degrade, not blow up.
func TestDAGResize_DeepChainAtNarrowWidth(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "r"},
	}
	prev := "r"
	for i := 0; i < 5; i++ {
		// 2 children at each level — forces depth via branching.
		for c := 0; c < 2; c++ {
			id := prev + ".c" + string(rune('a'+c))
			spans = append(spans, telemetry.Span{TraceID: "t", SpanID: id, ParentSpanID: prev})
		}
		prev = prev + ".ca" // descend along first child for chain depth
	}
	d := dagFromSpans(t, spans, 80, 30)
	out := d.renderGraph(76)
	got := maxLineW(out)
	t.Logf("depth-6 fanout at panel=80 → max line width %d", got)
}

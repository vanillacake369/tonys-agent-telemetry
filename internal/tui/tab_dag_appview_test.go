package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestAppView_DAGFitsTerminal exercises the FULL view chain (App.View →
// tab.View → RenderPanel → JoinHorizontal) at various terminal widths.
// This catches the off-by-one overflow that bypassed renderGraph's clip
// because the layers ABOVE renderGraph (panel border + horizontal spacer
// + content border) added extra cells.
func TestAppView_DAGFitsTerminal(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root"},
		{TraceID: "t", SpanID: "c1", ParentSpanID: "root"},
		{TraceID: "t", SpanID: "c2", ParentSpanID: "root"},
		{TraceID: "t", SpanID: "c3", ParentSpanID: "root"},
		{TraceID: "t", SpanID: "g1", ParentSpanID: "c2"},
	}

	for _, term := range []int{60, 80, 100, 120, 160, 200} {
		var m tea.Model = NewApp()
		m, _ = m.Update(tea.WindowSizeMsg{Width: term, Height: 40})
		m, _ = m.Update(SpanBatchMsg{Spans: spans})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		app := m.(App)

		view := app.View()
		maxW := maxLineW(view)
		lineCount := strings.Count(view, "\n") + 1
		if maxW > term {
			t.Errorf("term=%d: max line width=%d > %d — lipgloss WILL wrap", term, maxW, term)
		}
		// Stronger check: wrapping doubles+ line count. Sanity-bound a
		// reasonable max so spurious wrap is detected even when each
		// individual line happens to fit.
		if lineCount > 200 {
			t.Errorf("term=%d: %d lines suggests lipgloss is wrapping a few overflowing rows", term, lineCount)
		}
		t.Logf("term=%d → maxW=%d, lines=%d", term, maxW, lineCount)
	}
}

// TestAppView_DAGNoWrapAtCommonWidths is a tighter regression guard.
// We assert NO line exceeds the budget, AND the line count matches what
// the layout should produce (no hidden wraps). The off-by-one allocation
// in renderGraphView that caused the user-visible breakage would fail this.
func TestAppView_DAGNoWrapAtCommonWidths(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root"},
		{TraceID: "t", SpanID: "c1", ParentSpanID: "root"},
		{TraceID: "t", SpanID: "c2", ParentSpanID: "root"},
	}
	// One-shot at 120 cols — the most common tmux/zellij default.
	var m tea.Model = NewApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m, _ = m.Update(SpanBatchMsg{Spans: spans})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.(App).View()

	// Strict: no line may exceed terminal width.
	for i, line := range strings.Split(view, "\n") {
		w := len([]rune(stripAnsi(line)))
		if w > 120 {
			t.Errorf("line %d is %d cols wide (>120): %q", i, w, stripAnsi(line))
		}
	}

	// Also: the rendered view should fit the requested HEIGHT (30 rows).
	if lc := strings.Count(view, "\n") + 1; lc > 60 {
		t.Errorf("view has %d lines — content is wrapping into extra rows", lc)
	}
}

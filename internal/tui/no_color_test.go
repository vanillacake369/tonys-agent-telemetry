package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestDAGTab_RendersWithoutColor verifies that with NO_COLOR=1:
// (a) rendering does not panic,
// (b) every rendered line fits within the terminal width (no layout corruption
//
//	when ANSI strips out and lipgloss falls back to plain text),
//
// (c) status icons / text fallbacks still distinguish error/done/running
//
//	(✗ / ✓ / ▶ symbols are present in the output).
func TestDAGTab_RendersWithoutColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	// Same fixture as the resize test.
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root", System: "anthropic", Status: "done",
			Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "c1", ParentSpanID: "root", System: "anthropic", Status: "running",
			Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "t", SpanID: "c2", ParentSpanID: "root", System: "anthropic", Status: "error",
			Attrs: map[string]string{"gen_ai.tool.name": "Read"}},
		{TraceID: "t", SpanID: "c3", ParentSpanID: "c1", System: "anthropic", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "Edit"}},
	}

	const termW = 120
	const termH = 30

	d := NewDAGTab().SetSize(termW, termH).(*DAGTab)
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)

	// (a) Rendering must not panic — covered by the test reaching here.
	rendered := d.renderGraph(termW - 4)

	plain := stripAnsi(rendered)

	// (b) No line must exceed terminal width.
	for i, line := range strings.Split(plain, "\n") {
		w := len([]rune(line))
		if w > termW {
			t.Errorf("NO_COLOR=1: line %d width %d exceeds terminal width %d: %q", i, w, termW, line)
		}
	}

	// Enter graph view via Update so we can test the full View path too.
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	fullView := d.View()
	if fullView == "" {
		t.Error("NO_COLOR=1: View() returned empty string")
	}

	// (c) Status icons must be present in the traces list view.
	// Use separate traces — one per status — so all 3 icons appear.
	d2 := NewDAGTab().SetSize(termW, termH).(*DAGTab)
	multiTraceSpans := []telemetry.Span{
		// trace-done: one done span (no error/running escalation)
		{TraceID: "trace-done", SpanID: "d1", Status: "done"},
		// trace-running: one running span
		{TraceID: "trace-run", SpanID: "r1", Status: "running"},
		// trace-error: one error span
		{TraceID: "trace-err", SpanID: "e1", Status: "error"},
	}
	tab2, _ := d2.Update(SpanBatchMsg{Spans: multiTraceSpans})
	d2 = tab2.(*DAGTab)
	tracesView := d2.renderTracesView()
	plainTraces := stripAnsi(tracesView)

	// statusIcon renders ✓, ▶, ✗ — these must survive NO_COLOR stripping
	// (lipgloss strips color but keeps the rune characters).
	mustContain := []string{"✓", "▶", "✗"}
	for _, sym := range mustContain {
		if !strings.Contains(plainTraces, sym) {
			t.Errorf("NO_COLOR=1: traces view missing status symbol %q; output:\n%s", sym, plainTraces)
		}
	}
}

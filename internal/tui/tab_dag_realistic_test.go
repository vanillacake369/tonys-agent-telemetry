package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestDAGShape_DeepBranchedSession recreates roughly the trace shape
// the user showed in their screenshot: a long chain interrupted by
// deep nested branches.
func TestDAGShape_DeepBranchedSession(t *testing.T) {
	// Build: linear chain of 5 chat messages, then a branch with 3-deep
	// nested chat-and-tool calls. Total ~15 spans, maxCol ~4.
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "n1", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "n2", ParentSpanID: "n1", Attrs: map[string]string{"gen_ai.operation.name": "chat"}, InputTokens: 3, OutputTokens: 1},
		{TraceID: "t", SpanID: "n3", ParentSpanID: "n2", Attrs: map[string]string{"gen_ai.operation.name": "chat"}, InputTokens: 3, OutputTokens: 1},
		{TraceID: "t", SpanID: "n4", ParentSpanID: "n3", Attrs: map[string]string{"gen_ai.operation.name": "chat"}, InputTokens: 3, OutputTokens: 1},
		// branch
		{TraceID: "t", SpanID: "n5a", ParentSpanID: "n4", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "n5b", ParentSpanID: "n4", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		// n5a chain
		{TraceID: "t", SpanID: "n6", ParentSpanID: "n5a", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "n7", ParentSpanID: "n6", Attrs: map[string]string{"gen_ai.operation.name": "chat"}, InputTokens: 1, OutputTokens: 63},
		// branch deeper
		{TraceID: "t", SpanID: "n8a", ParentSpanID: "n7", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "n8b", ParentSpanID: "n7", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		// further deep branch from n8b
		{TraceID: "t", SpanID: "n9", ParentSpanID: "n8b", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
	}

	for _, term := range []int{80, 100, 120, 160} {
		var m tea.Model = NewApp()
		m, _ = m.Update(tea.WindowSizeMsg{Width: term, Height: 40})
		m, _ = m.Update(SpanBatchMsg{Spans: spans})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

		view := m.(App).View()
		maxW := maxLineW(view)
		clipCount := strings.Count(view, "›")

		// Hard contract: no line exceeds the terminal width.
		if maxW > term {
			t.Errorf("term=%d: maxW=%d > %d (overflow)", term, maxW, term)
		}
		// Soft check: clip indicator '›' should be rare. The user's screen
		// had a '›' on every other line; that means our nodeW shrink
		// didn't pull the graph below panel width.
		linesWithBoxes := strings.Count(stripAnsi(view), "┌")
		if clipCount > linesWithBoxes*2 {
			t.Logf("term=%d: %d '›' clips for %d boxes — clip is doing too much work, nodeW shrink should fit naturally",
				term, clipCount, linesWithBoxes)
		}
		t.Logf("term=%d: maxW=%d, clips=%d, boxes=%d", term, maxW, clipCount, linesWithBoxes)
	}
}

// TestDAGShape_VisualAt80 dumps the actual rendered output at 80 cols
// (narrow, common in split panes) so a human can eyeball whether the
// graph is now legible.
func TestDAGShape_VisualAt80(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "c1", ParentSpanID: "root", Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "t", SpanID: "c2", ParentSpanID: "root", Attrs: map[string]string{"gen_ai.tool.name": "Read"}},
		{TraceID: "t", SpanID: "g1", ParentSpanID: "c2", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "g2", ParentSpanID: "g1", Attrs: map[string]string{"gen_ai.tool.name": "Edit"}},
	}
	var m tea.Model = NewApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m, _ = m.Update(SpanBatchMsg{Spans: spans})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	view := m.(App).View()
	t.Logf("\n--- DAG at 80×30 (deep branched) ---\n%s", view)
}

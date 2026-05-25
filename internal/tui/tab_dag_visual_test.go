package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestDAGTab_VisualSnapshot is a verbose helper: run with `go test -v -run
// TestDAGTab_VisualSnapshot` to see the rendered graph and confirm the new
// 2D layout looks like n8n/airflow rather than nested bullet points.
func TestDAGTab_VisualSnapshot(t *testing.T) {
	d := NewDAGTab().SetSize(160, 50).(*DAGTab)
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root123", System: "anthropic", Model: "claude-sonnet-4-6", InputTokens: 1500, OutputTokens: 420, Status: "done", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "bash1", ParentSpanID: "root123", System: "anthropic", Status: "done", Attrs: map[string]string{"gen_ai.tool.name": "Bash"}},
		{TraceID: "t", SpanID: "read1", ParentSpanID: "root123", System: "anthropic", Status: "done", Attrs: map[string]string{"gen_ai.tool.name": "Read"}},
		{TraceID: "t", SpanID: "chat2", ParentSpanID: "read1", System: "anthropic", InputTokens: 200, OutputTokens: 50, Status: "done", Attrs: map[string]string{"gen_ai.operation.name": "chat"}},
		{TraceID: "t", SpanID: "edit1", ParentSpanID: "chat2", System: "anthropic", Status: "running", Attrs: map[string]string{"gen_ai.tool.name": "Edit"}},
		{TraceID: "t", SpanID: "web1", ParentSpanID: "root123", System: "anthropic", Status: "error", Attrs: map[string]string{"gen_ai.tool.name": "WebFetch"}},
	}
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	rendered := d.renderGraph(140)
	t.Logf("\n%s", rendered)

	// Sanity: rendering must contain at least one box border char (this
	// distinguishes from the previous nested-indent rendering which used
	// only └─→ arrows, no box chars).
	hasBox := false
	for _, r := range rendered {
		if r == '┌' || r == '┐' || r == '└' || r == '┘' {
			hasBox = true
			break
		}
	}
	if !hasBox {
		t.Error("rendered graph must contain box-drawing border chars; " +
			"got the old nested-list output, not the new 2D graph")
	}
}

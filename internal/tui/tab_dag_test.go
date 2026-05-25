package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

func TestDAGTab_EmptyView(t *testing.T) {
	d := NewDAGTab()
	d = d.SetSize(80, 20).(*DAGTab)
	v := d.View()
	if !strings.Contains(v, "DAG") && !strings.Contains(v, "No telemetry") {
		t.Errorf("empty view should mention DAG or 'No telemetry', got %q", truncate(v, 200))
	}
}

func TestDAGTab_AccumulatesSpans(t *testing.T) {
	d := NewDAGTab()
	d = d.SetSize(80, 20).(*DAGTab)
	tab, _ := d.Update(SpanCollectedMsg{Span: telemetry.Span{
		TraceID: "t1", SpanID: "a", System: "anthropic",
	}})
	d = tab.(*DAGTab)
	tab, _ = d.Update(SpanCollectedMsg{Span: telemetry.Span{
		TraceID: "t1", SpanID: "b", ParentSpanID: "a", System: "anthropic",
	}})
	d = tab.(*DAGTab)

	if len(d.spans) != 2 {
		t.Fatalf("spans = %d, want 2", len(d.spans))
	}
	v := d.View()
	// First 8 chars of TraceID t1 (which is shorter, so full) should appear.
	if !strings.Contains(v, "t1") {
		t.Errorf("view should contain trace id, got: %q", truncate(v, 300))
	}
	if !strings.Contains(v, "spans=2") {
		t.Errorf("view should show span count, got: %q", truncate(v, 300))
	}
}

func TestDAGTab_MultipleTracesSortedByStartTime(t *testing.T) {
	older := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 5, 25, 12, 0, 0, 0, time.UTC)

	d := NewDAGTab()
	d = d.SetSize(120, 30).(*DAGTab)
	tab, _ := d.Update(SpanCollectedMsg{Span: telemetry.Span{
		TraceID: "older-trace-id", SpanID: "x", StartTime: older,
	}})
	d = tab.(*DAGTab)
	tab, _ = d.Update(SpanCollectedMsg{Span: telemetry.Span{
		TraceID: "newer-trace-id", SpanID: "y", StartTime: newer,
	}})
	d = tab.(*DAGTab)

	v := d.View()
	// newer should appear before older
	idxNewer := strings.Index(v, "newer-tr") // short ID prefix
	idxOlder := strings.Index(v, "older-tr")
	if idxNewer < 0 || idxOlder < 0 {
		t.Fatalf("missing trace IDs in view: idxNewer=%d idxOlder=%d", idxNewer, idxOlder)
	}
	if idxNewer >= idxOlder {
		t.Errorf("newer should appear before older: newer at %d, older at %d", idxNewer, idxOlder)
	}
}

func TestTreeStats(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "1"},
		{TraceID: "t", SpanID: "2", ParentSpanID: "1"},
		{TraceID: "t", SpanID: "3", ParentSpanID: "2"},
		{TraceID: "t", SpanID: "4", ParentSpanID: "1"},
	}
	root := telemetry.BuildTrees(spans)["t"]
	count, depth := treeStats(root)
	if count != 4 {
		t.Errorf("count = %d, want 4", count)
	}
	if depth != 3 {
		t.Errorf("depth = %d, want 3", depth)
	}
}

func TestDAGTab_DoesNotPanicOnNoKeyMessage(t *testing.T) {
	d := NewDAGTab()
	tab, cmd := d.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if tab == nil {
		t.Fatal("nil tab")
	}
	_ = cmd
}

// truncate trims s to n runes for use in test error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

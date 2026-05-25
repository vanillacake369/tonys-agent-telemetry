package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// ── MatchNode unit tests ────────────────────────────────────────────────────

func TestMatchNode_MatchesToolName(t *testing.T) {
	node := &telemetry.SpanNode{Span: telemetry.Span{
		Attrs: map[string]string{"gen_ai.tool.name": "Bash"},
	}}
	if !MatchNode(node, "bash") {
		t.Error("MatchNode: expected match on gen_ai.tool.name 'Bash' with query 'bash'")
	}
}

func TestMatchNode_MatchesOperationName(t *testing.T) {
	node := &telemetry.SpanNode{Span: telemetry.Span{
		Attrs: map[string]string{"gen_ai.operation.name": "chat"},
	}}
	if !MatchNode(node, "CHAT") {
		t.Error("MatchNode: expected case-insensitive match on gen_ai.operation.name 'chat'")
	}
}

func TestMatchNode_MatchesModel(t *testing.T) {
	node := &telemetry.SpanNode{Span: telemetry.Span{
		Model: "claude-sonnet-4-6",
	}}
	if !MatchNode(node, "sonnet") {
		t.Error("MatchNode: expected match on Model 'claude-sonnet-4-6' with query 'sonnet'")
	}
}

func TestMatchNode_MatchesStatus(t *testing.T) {
	node := &telemetry.SpanNode{Span: telemetry.Span{Status: "error"}}
	if !MatchNode(node, "error") {
		t.Error("MatchNode: expected match on Status 'error'")
	}
}

func TestMatchNode_NoMatchReturnsfalse(t *testing.T) {
	node := &telemetry.SpanNode{Span: telemetry.Span{
		Model:  "claude-opus",
		Status: "done",
		Attrs:  map[string]string{"gen_ai.tool.name": "Read"},
	}}
	if MatchNode(node, "python") {
		t.Error("MatchNode: expected no match for unrelated query 'python'")
	}
}

func TestMatchNode_EmptyQueryMatchesAll(t *testing.T) {
	node := &telemetry.SpanNode{Span: telemetry.Span{}}
	if !MatchNode(node, "") {
		t.Error("MatchNode: empty query should match everything")
	}
}

func TestMatchNode_NilNodeDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MatchNode panicked on nil node: %v", r)
		}
	}()
	result := MatchNode(nil, "bash")
	if result {
		t.Error("MatchNode: nil node should not match")
	}
}

// ── Integration: DAG search highlights matching spans ──────────────────────

// TestDAGSearch_HighlightsMatchingNodes loads 5 spans into a DAGTab,
// initiates a search for "bash", and asserts:
//   - at least one rendered line contains the search highlight marker (*),
//   - non-matching spans do not carry the marker.
func TestDAGSearch_HighlightsMatchingNodes(t *testing.T) {
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)

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
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)

	// Open graph view.
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	// Activate search mode for the graph.
	d.dagSearchQuery = "bash"
	d.dagSearchActive = true

	rendered := d.renderGraph(116)
	plain := stripAnsiSeq(rendered)

	// At least one line should contain the search marker (* prefix).
	hasMarker := strings.Contains(plain, "*")
	if !hasMarker {
		t.Errorf("search for 'bash' should mark matching node with '*'; rendered:\n%s", plain)
	}

	// Lines that contain "Read" or "Edit" or "Write" should NOT have the marker
	// on the same box line. We check line-by-line.
	lines := strings.Split(plain, "\n")
	for _, line := range lines {
		if strings.Contains(line, "*") {
			// This line has the marker — it should mention Bash (directly or via label).
			// We can't assert exact label because nodeLabel may truncate, but the
			// marker should not appear in lines that exclusively mention non-bash tools.
			nonBashTools := []string{" Read", " Edit", " Write", " chat"}
			for _, nbt := range nonBashTools {
				if strings.Contains(line, nbt) && !strings.Contains(line, "Bash") {
					t.Errorf("search marker found on non-bash node line: %q", line)
				}
			}
		}
	}
}

// TestDAGSearch_SearchModeTogglesOnSlash verifies that pressing "/" in
// dagViewGraph mode activates DAG search input.
func TestDAGSearch_SearchModeTogglesOnSlash(t *testing.T) {
	d := NewDAGTab().SetSize(120, 30).(*DAGTab)

	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "r", Status: "done"},
		{TraceID: "t", SpanID: "c", ParentSpanID: "r", Status: "done"},
	}
	tab, _ := d.Update(SpanBatchMsg{Spans: spans})
	d = tab.(*DAGTab)
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d = tab.(*DAGTab)

	if d.mode != dagViewGraph {
		t.Fatalf("expected dagViewGraph mode, got %d", d.mode)
	}

	// Press "/" to activate search.
	tab, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	d = tab.(*DAGTab)

	if !d.dagSearchInputting {
		t.Error("after '/' in graph view, dagSearchInputting should be true")
	}
}

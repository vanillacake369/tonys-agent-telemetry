package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

// ── RenderDAG: nil / single root ─────────────────────────────────────────────

func TestRenderDAG_NilRoot(t *testing.T) {
	out := RenderDAG(nil, 80)
	if out == "" {
		t.Error("RenderDAG(nil) should return a non-empty fallback string")
	}
	if strings.Contains(out, "👤") {
		t.Error("RenderDAG(nil) should not render the User node")
	}
}

func TestRenderDAG_SingleRoot_NoChildren(t *testing.T) {
	root := &data.DAGNode{
		ID:        "abc123",
		AgentType: "architect",
		Status:    "done",
	}
	out := RenderDAG(root, 80)

	// User node must appear.
	if !strings.Contains(out, "👤 User") {
		t.Error("output should contain '👤 User'")
	}
	// The root connector must appear.
	if !strings.Contains(out, "└─►") {
		t.Error("output should contain '└─►' connector")
	}
	// architect icon must appear.
	if !strings.Contains(out, "🏗️") {
		t.Errorf("output should contain architect icon 🏗️, got:\n%s", out)
	}
	// No branch connectors — there are no children.
	if strings.Contains(out, "├─►") {
		t.Error("single root should not contain '├─►' connectors")
	}
}

// ── RenderDAG: children ───────────────────────────────────────────────────────

func TestRenderDAG_WithChildren_CorrectConnectors(t *testing.T) {
	root := &data.DAGNode{
		ID:        "root",
		AgentType: "architect",
		Status:    "done",
		Children: []*data.DAGNode{
			{ID: "c1", AgentType: "researcher", Status: "done"},
			{ID: "c2", AgentType: "implementer", Status: "running"},
		},
	}
	out := RenderDAG(root, 80)

	// Non-last child uses ├─►.
	if !strings.Contains(out, "├─►") {
		t.Errorf("output should contain '├─►' for non-last child, got:\n%s", out)
	}
	// Last child uses └─►.
	if !strings.Contains(out, "└─►") {
		t.Errorf("output should contain '└─►' for last child, got:\n%s", out)
	}
	// Both child agent icons.
	if !strings.Contains(out, "🔍") {
		t.Error("output should contain researcher icon 🔍")
	}
	if !strings.Contains(out, "⚙️") {
		t.Error("output should contain implementer icon ⚙️")
	}
}

// ── RenderDAG: nested children ────────────────────────────────────────────────

func TestRenderDAG_NestedChildren_CorrectIndentation(t *testing.T) {
	// Two children at level-1; first child also has a child at level-2.
	// This produces │ continuation lines so that the second level-1 child
	// appears connected even after the first child's sub-tree is printed.
	root := &data.DAGNode{
		ID:        "root",
		AgentType: "architect",
		Status:    "done",
		Children: []*data.DAGNode{
			{
				ID:        "c1",
				AgentType: "researcher",
				Status:    "done",
				Children: []*data.DAGNode{
					{ID: "c1a", AgentType: "tester", Status: "done"},
				},
			},
			{
				ID:        "c2",
				AgentType: "implementer",
				Status:    "running",
			},
		},
	}
	out := RenderDAG(root, 80)

	// tester node must appear deeper in the tree.
	if !strings.Contains(out, "🧪") {
		t.Errorf("output should contain tester icon 🧪, got:\n%s", out)
	}
	// The non-last child (researcher) uses ├─► which causes │ continuation lines
	// for its sub-children so that implementer appears connected below.
	if !strings.Contains(out, "│") {
		t.Errorf("output should contain '│' continuation for nested children, got:\n%s", out)
	}
	// implementer must also appear.
	if !strings.Contains(out, "⚙️") {
		t.Errorf("output should contain implementer icon ⚙️, got:\n%s", out)
	}
}

// ── Agent icons ───────────────────────────────────────────────────────────────

func TestRenderDAG_AgentIconsApplied(t *testing.T) {
	cases := []struct {
		agentType string
		wantIcon  string
	}{
		{"architect", "🏗️"},
		{"implementer", "⚙️"},
		{"reviewer", "🔎"},
		{"tester", "🧪"},
		{"refactorer", "♻️"},
		{"researcher", "🔍"},
		{"cross-validator", "✅"},
		{"unknown-agent", "🤖"},
	}

	for _, tc := range cases {
		t.Run(tc.agentType, func(t *testing.T) {
			root := &data.DAGNode{
				ID:        "r",
				AgentType: tc.agentType,
				Status:    "done",
			}
			out := RenderDAG(root, 80)
			if !strings.Contains(out, tc.wantIcon) {
				t.Errorf("agentType=%q: expected icon %q in output:\n%s", tc.agentType, tc.wantIcon, out)
			}
		})
	}
}

// ── Status indicators ─────────────────────────────────────────────────────────

func TestRenderDAG_StatusIndicators(t *testing.T) {
	cases := []struct {
		status    string
		wantBadge string
	}{
		{"done", "✅ done"},
		{"running", "🔄 running"},
		{"pending", "⏳ pending"},
		{"error", "❌ error"},
	}

	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			root := &data.DAGNode{
				ID:        "r",
				AgentType: "implementer",
				Status:    tc.status,
			}
			out := RenderDAG(root, 80)
			// Strip ANSI for comparison since lipgloss may add color codes.
			stripped := stripANSI(out)
			if !strings.Contains(stripped, tc.wantBadge) {
				t.Errorf("status=%q: expected badge %q in output:\n%s", tc.status, tc.wantBadge, stripped)
			}
		})
	}
}

// ── Token count formatting ────────────────────────────────────────────────────

func TestRenderDAG_TokenCountFormatted(t *testing.T) {
	root := &data.DAGNode{
		ID:         "r",
		AgentType:  "implementer",
		Status:     "done",
		TokenCount: 15200,
	}
	out := RenderDAG(root, 80)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "15.2k tok") {
		t.Errorf("expected '15.2k tok' in output:\n%s", stripped)
	}
}

func TestRenderDAG_SmallTokenCount(t *testing.T) {
	root := &data.DAGNode{
		ID:         "r",
		AgentType:  "implementer",
		Status:     "done",
		TokenCount: 500,
	}
	out := RenderDAG(root, 80)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "500 tok") {
		t.Errorf("expected '500 tok' in output:\n%s", stripped)
	}
}

// ── Cycle detection ───────────────────────────────────────────────────────────

func TestRenderDAG_CycleShowsCycleLabel(t *testing.T) {
	// Build a structure where a child has the same ID as the root.
	// The visited map on the active path will detect this as a cycle.
	child := &data.DAGNode{ID: "root", AgentType: "tester", Status: "done"}
	root := &data.DAGNode{
		ID:        "root",
		AgentType: "architect",
		Status:    "done",
		Children:  []*data.DAGNode{child},
	}
	out := RenderDAG(root, 80)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "[cycle]") {
		t.Errorf("expected '[cycle]' in output for cyclic node, got:\n%s", stripped)
	}
}

// ── Max depth ─────────────────────────────────────────────────────────────────

func TestRenderDAG_MaxDepthLimit(t *testing.T) {
	// Build a chain 12 levels deep (exceeds dagMaxDepth=10).
	var buildChain func(depth int) *data.DAGNode
	buildChain = func(depth int) *data.DAGNode {
		node := &data.DAGNode{
			ID:        fmt.Sprintf("node-%d", depth),
			AgentType: "implementer",
			Status:    "done",
		}
		if depth > 0 {
			node.Children = []*data.DAGNode{buildChain(depth - 1)}
		}
		return node
	}
	root := buildChain(12)
	out := RenderDAG(root, 80)
	stripped := stripANSI(out)
	if !strings.Contains(stripped, "[max depth reached]") {
		t.Errorf("expected '[max depth reached]' for deep tree, got:\n%s", stripped)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// stripANSI removes ANSI escape sequences from a string for plain-text comparison.
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
				inEscape = false
			}
			continue
		}
		result.WriteByte(c)
	}
	return result.String()
}

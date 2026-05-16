package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

const dagMaxDepth = 10

// RenderDAG converts a DAGNode tree into an ASCII string suitable for display
// in a scrollable viewport. width is reserved for future wrapping support.
// This is a pure function with no side effects.
func RenderDAG(root *data.DAGNode, width int) string {
	if root == nil {
		return lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true).
			Render("No DAG data available")
	}

	var sb strings.Builder

	// Render User node at the top.
	sb.WriteString("👤 User\n")
	sb.WriteString(" └─► ")

	visited := map[string]struct{}{}
	sb.WriteString(renderDAGNode(root, "", visited, 0))

	return sb.String()
}

// renderDAGNode recursively renders a single node and its children.
// prefix is the indentation string prepended before each child connector.
// visited prevents infinite loops on cycles.
// depth limits recursion to dagMaxDepth levels.
func renderDAGNode(node *data.DAGNode, prefix string, visited map[string]struct{}, depth int) string {
	if depth > dagMaxDepth {
		return lipgloss.NewStyle().Foreground(dimColor).Render("[max depth reached]") + "\n"
	}
	if node == nil {
		return ""
	}

	// Cycle detection using node ID.
	if node.ID != "" {
		if _, seen := visited[node.ID]; seen {
			return lipgloss.NewStyle().Foreground(dimColor).Render("[cycle]") + "\n"
		}
		visited[node.ID] = struct{}{}
		defer delete(visited, node.ID)
	}

	var sb strings.Builder

	// Format the node header line: icon + name + status + tokens.
	icon := agentIcon(node.AgentType)
	label := dagNodeLabel(node)
	sb.WriteString(icon + " " + label + "\n")

	// Render tools line if present, using │ continuation when there are children.
	if len(node.Tools) > 0 {
		var toolIndent string
		if len(node.Children) > 0 {
			toolIndent = prefix + "│    "
		} else {
			toolIndent = prefix + "     "
		}
		sb.WriteString(toolIndent + dagToolsLine(node.Tools) + "\n")
	}

	// Render children with ArgoCD-style box-drawing connectors.
	for i, child := range node.Children {
		childIsLast := i == len(node.Children)-1

		var connector string
		var childPrefix string
		if childIsLast {
			connector = "└─► "
			childPrefix = prefix + "     "
		} else {
			connector = "├─► "
			childPrefix = prefix + "│    "
		}

		sb.WriteString(prefix + connector)
		sb.WriteString(renderDAGNode(child, childPrefix, visited, depth+1))
	}

	return sb.String()
}

// dagNodeLabel formats a node's name, status indicator, and token count.
func dagNodeLabel(node *data.DAGNode) string {
	name := node.AgentType
	if name == "" {
		name = node.Description
	}
	if name == "" {
		name = node.ID
	}
	if name == "" {
		name = "unknown"
	}

	var parts []string
	parts = append(parts, name)

	statusStr := dagStatusBadge(node.Status)
	if statusStr != "" {
		parts = append(parts, "["+statusStr+"]")
	}

	if node.TokenCount > 0 {
		parts = append(parts, dagFormatTokens(node.TokenCount)+" tok")
	}

	return strings.Join(parts, " ")
}

// dagStatusBadge returns a colored status string with icon.
func dagStatusBadge(status string) string {
	switch strings.ToLower(status) {
	case "done":
		return lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#2D8A2D", Dark: "#4CAF50"}).
			Render("✅ done")
	case "running":
		return lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFC107"}).
			Render("🔄 running")
	case "pending":
		return lipgloss.NewStyle().
			Foreground(dimColor).
			Render("⏳ pending")
	case "error":
		return lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF6B6B"}).
			Render("❌ error")
	default:
		return status
	}
}

// dagToolsLine formats the tools list for display under a node.
func dagToolsLine(tools []string) string {
	if len(tools) == 0 {
		return ""
	}
	// Show up to 3 tool names; append count if more exist.
	const maxTools = 3
	display := tools
	if len(tools) > maxTools {
		display = tools[:maxTools]
	}
	line := "└─ Tools: " + strings.Join(display, ", ")
	if len(tools) > maxTools {
		line += fmt.Sprintf(" (%d)", len(tools))
	}
	return lipgloss.NewStyle().Foreground(dimColor).Render(line)
}

// dagFormatTokens formats a token count as "15.2k" for ≥1000, or plain decimal below.
func dagFormatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000.0)
	}
	return fmt.Sprintf("%d", n)
}

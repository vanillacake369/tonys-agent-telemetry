package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// SpanCollectedMsg is sent by main.go for each Span produced by registered
// ProviderIngestors. The active App routes it to the DAG tab.
type SpanCollectedMsg struct {
	Span telemetry.Span
}

// DAGTab visualizes collected Spans grouped by TraceID, using
// telemetry.BuildTrees to reconstruct parent-child relationships.
type DAGTab struct {
	width    int
	height   int
	spans    []telemetry.Span
	viewport viewport.Model
}

// NewDAGTab returns an initialised DAGTab.
func NewDAGTab() *DAGTab {
	return &DAGTab{viewport: viewport.New(80, 20)}
}

func (d *DAGTab) Init() tea.Cmd { return nil }

func (d *DAGTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case SpanCollectedMsg:
		d.spans = append(d.spans, msg.Span)
		d.refreshViewport()
		return d, nil
	case tea.KeyMsg:
		var cmd tea.Cmd
		d.viewport, cmd = d.viewport.Update(msg)
		return d, cmd
	}
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

func (d *DAGTab) View() string {
	if len(d.spans) == 0 {
		return RenderEmptyState("No telemetry spans collected yet — launch a Claude Code session or wait for backfill.", d.width, d.height)
	}
	panelH := max(3, d.height)
	return RenderPanel("Agent DAG", d.viewport.View(), d.width, panelH, true)
}

func (d *DAGTab) SetSize(width, height int) TabModel {
	d.width = width
	d.height = height
	d.viewport.Width = max(1, width-2)
	d.viewport.Height = max(1, height-2)
	d.refreshViewport()
	return d
}

func (d *DAGTab) refreshViewport() {
	if len(d.spans) == 0 {
		d.viewport.SetContent("")
		return
	}
	d.viewport.SetContent(renderTraceList(d.spans, max(40, d.viewport.Width-2)))
	d.viewport.GotoTop()
}

// renderTraceList groups spans by TraceID, reconstructs each tree, and
// renders one summary line per trace plus a small ASCII tree.
func renderTraceList(spans []telemetry.Span, width int) string {
	trees := telemetry.BuildTrees(spans)
	if len(trees) == 0 {
		return "(no traces)"
	}

	// Build a stable display order: by earliest StartTime descending (most
	// recent first). Spans without StartTime sort to the end.
	traceIDs := make([]string, 0, len(trees))
	for id := range trees {
		traceIDs = append(traceIDs, id)
	}
	sort.SliceStable(traceIDs, func(i, j int) bool {
		ti := trees[traceIDs[i]].Span.StartTime
		tj := trees[traceIDs[j]].Span.StartTime
		if ti.IsZero() != tj.IsZero() {
			return !ti.IsZero() // non-zero first
		}
		return ti.After(tj)
	})

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)

	var sb strings.Builder
	for _, id := range traceIDs {
		root := trees[id]
		spanCount, depth := treeStats(root)
		shortID := id
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		header := fmt.Sprintf("%s  spans=%d  depth=%d  system=%s",
			headerStyle.Render(shortID), spanCount, depth, root.Span.System)
		sb.WriteString(header)
		sb.WriteString("\n")
		renderTreeASCII(&sb, root, "", true, dimStyle)
		sb.WriteString("\n")
	}
	return sb.String()
}

// treeStats counts the number of spans in a tree and the max depth.
func treeStats(node *telemetry.SpanNode) (count, depth int) {
	if node == nil {
		return 0, 0
	}
	count = 1
	maxChild := 0
	for _, c := range node.Children {
		cc, cd := treeStats(c)
		count += cc
		if cd > maxChild {
			maxChild = cd
		}
	}
	depth = 1 + maxChild
	return
}

// renderTreeASCII writes a tree representation into sb using box-drawing
// characters. last indicates whether this node is the last child of its
// parent (controls │/└ vs ├).
func renderTreeASCII(sb *strings.Builder, node *telemetry.SpanNode, prefix string, last bool, style lipgloss.Style) {
	if node == nil {
		return
	}
	branch := "├── "
	childPrefix := "│   "
	if last {
		branch = "└── "
		childPrefix = "    "
	}
	label := nodeLabel(node.Span)
	sb.WriteString(style.Render(prefix + branch + label))
	sb.WriteString("\n")
	for i, child := range node.Children {
		renderTreeASCII(sb, child, prefix+childPrefix, i == len(node.Children)-1, style)
	}
}

// nodeLabel returns a short human-readable label for a Span.
func nodeLabel(s telemetry.Span) string {
	tool := s.Attrs["gen_ai.tool.name"]
	op := s.Attrs["gen_ai.operation.name"]
	tokens := ""
	if s.InputTokens > 0 || s.OutputTokens > 0 {
		tokens = fmt.Sprintf("  [%d/%d]", s.InputTokens, s.OutputTokens)
	}
	shortID := s.SpanID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	if tool != "" {
		return fmt.Sprintf("%s  tool=%s%s", shortID, tool, tokens)
	}
	if op != "" {
		return fmt.Sprintf("%s  op=%s%s", shortID, op, tokens)
	}
	return shortID + tokens
}

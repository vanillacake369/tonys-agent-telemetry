// Package tui — tab_dag_overview.go: compact "all-spans-at-a-glance" overview
// for the DAG tab. Toggled with 'g' from dagViewGraph mode.
//
// SRP: this file owns only the overview data structure, key handling, and
// render function. All DAG state fields live in tab_dag.go (DAGTab struct).
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// overviewLine is the display metadata for one line in the compact overview.
type overviewLine struct {
	spanID string // original span ID, used to select the node when returning to graph
	text   string // pre-rendered plain display string (depth-indented)
	depth  int    // indentation depth
}

// buildOverviewLines constructs a slice of overviewLine from the flat depth-
// first node slice that DAGTab already maintains. Each line shows:
//
//	[●] <label> (<dur>) [— <error>]
//
// with └─ tree connectors driven by depth.
// DRY: reuses the same flatNodes ordering already computed by flattenTrace.
func buildOverviewLines(nodes []*telemetry.SpanNode) []overviewLine {
	if len(nodes) == 0 {
		return nil
	}

	// Compute depth for each node from the flat list. We reconstruct depth by
	// checking how many ancestors are in the flatNodes set.
	depthOf := make(map[string]int, len(nodes))
	idSet := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		idSet[n.Span.SpanID] = true
	}
	for _, n := range nodes {
		d := 0
		p := n.Span.ParentSpanID
		for p != "" && idSet[p] {
			d++
			// Walk up by finding the parent node's parent.
			found := false
			for _, m := range nodes {
				if m.Span.SpanID == p {
					p = m.Span.ParentSpanID
					found = true
					break
				}
			}
			if !found {
				break
			}
		}
		depthOf[n.Span.SpanID] = d
	}

	lines := make([]overviewLine, 0, len(nodes))
	for _, n := range nodes {
		depth := depthOf[n.Span.SpanID]

		// Status icon.
		var icon string
		switch n.Span.Status {
		case "error":
			icon = "✗"
		case "running":
			icon = "▶"
		default:
			icon = "●"
		}

		// Label: prefer tool name, then operation name, then "span".
		label := n.Span.Attrs["gen_ai.tool.name"]
		if label == "" {
			label = n.Span.Attrs["gen_ai.operation.name"]
		}
		if label == "" {
			label = "span"
		}

		// Duration suffix.
		durStr := ""
		if dur := n.Span.Duration(); dur > 0 {
			durStr = " (" + formatDurShort(dur) + ")"
		}

		// Error suffix.
		errStr := ""
		if n.Span.Status == "error" {
			if errMsg := n.Span.Attrs["error.message"]; errMsg != "" {
				// Truncate to keep lines readable.
				if len(errMsg) > 40 {
					errMsg = errMsg[:37] + "..."
				}
				errStr = " — " + errMsg
			}
		}

		// Tree connector prefix.
		var prefix string
		if depth == 0 {
			prefix = ""
		} else {
			prefix = strings.Repeat("  ", depth-1) + "  └─ "
		}

		text := fmt.Sprintf("%s[%s] %s%s%s", prefix, icon, label, durStr, errStr)

		lines = append(lines, overviewLine{
			spanID: n.Span.SpanID,
			text:   text,
			depth:  depth,
		})
	}
	return lines
}

// formatDurShort formats a duration concisely: ms below 1s, otherwise seconds
// with one decimal place.
func formatDurShort(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// handleKeyOverview processes key events while in dagViewOverview mode.
func (d *DAGTab) handleKeyOverview(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch s := msg.String(); s {
	case "j", "down":
		if d.overviewCursor < len(d.overviewLines)-1 {
			d.overviewCursor++
		}
	case "k", "up":
		if d.overviewCursor > 0 {
			d.overviewCursor--
		}
	case "enter":
		// Return to graph mode with cursor positioned on selected span.
		if len(d.overviewLines) > 0 && d.overviewCursor < len(d.overviewLines) {
			targetID := d.overviewLines[d.overviewCursor].spanID
			for i, n := range d.flatNodes {
				if n.Span.SpanID == targetID {
					d.nodeCursor = i
					break
				}
			}
			// Invalidate graph cache so the cursor jump is immediately visible.
			d.graphCache = ""
			d.graphCacheKey = ""
		}
		d.mode = dagViewGraph
	case "g", "esc":
		// Toggle back to graph view.
		d.mode = dagViewGraph
	}
	return d, nil
}

// renderOverviewView renders the compact one-line-per-span tree overview.
// Keeps renderGraphView (SRP) and graph render cache untouched.
func (d *DAGTab) renderOverviewView() string {
	if len(d.overviewLines) == 0 {
		return RenderEmptyState("(no spans in this trace)", d.width, d.height)
	}

	cursorStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)

	var sb strings.Builder

	// Help line at the top.
	help := d.renderHelp([]helpKey{
		{"j/k", "navigate"}, {"Enter", "select & go to graph"}, {"g/Esc", "back to graph"},
	})
	sb.WriteString(help)
	sb.WriteString("\n\n")

	contentBudget := d.width - 4
	if contentBudget < 20 {
		contentBudget = 20
	}

	for i, ol := range d.overviewLines {
		if i == d.overviewCursor {
			marker := cursorStyle.Render("> ")
			line := cursorStyle.Render(ol.text)
			// Clip to content budget to prevent overflow.
			plain := stripAnsiSeq(marker + line)
			if len([]rune(plain)) > contentBudget {
				runes := []rune(plain)
				line = string(runes[:contentBudget-1]) + "›"
				sb.WriteString(line)
			} else {
				sb.WriteString(marker + line)
			}
		} else {
			line := dimStyle.Render("  " + ol.text)
			plain := stripAnsiSeq(line)
			if len([]rune(plain)) > contentBudget {
				runes := []rune(plain)
				line = string(runes[:contentBudget-1]) + "›"
			}
			sb.WriteString(line)
		}
		if i < len(d.overviewLines)-1 {
			sb.WriteString("\n")
		}
	}

	return RenderPanel(
		fmt.Sprintf("Overview · %s · %d spans", shortID(d.activeTrace), len(d.flatNodes)),
		sb.String(),
		d.width, max(3, d.height), true)
}

package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

// DAGSessionsLoadedMsg is sent when session discovery completes for the DAG tab.
type DAGSessionsLoadedMsg struct {
	Sessions []data.Session
	Err      error
}

// DAGLoadedMsg is sent when ParseDAG completes for a selected session.
type DAGLoadedMsg struct {
	DAG *data.DAGNode
	Err error
}

// DAGTab implements TabModel for the DAG tab.
// It shows an ASCII agent orchestration tree for the selected session.
type DAGTab struct {
	sessions    []data.Session
	selectedIdx int
	dag         *data.DAGNode
	viewport    viewport.Model
	width       int
	height      int
	loading     bool
	err         error
}

// NewDAGTab creates an initialised DAGTab ready to be displayed.
func NewDAGTab() DAGTab {
	vp := viewport.New(80, 20)
	return DAGTab{
		viewport: vp,
		loading:  true,
	}
}

// Init loads sessions asynchronously, then auto-selects the most recent one.
func (d DAGTab) Init() tea.Cmd {
	return func() tea.Msg {
		sessions, err := data.DiscoverSessions()
		return DAGSessionsLoadedMsg{Sessions: sessions, Err: err}
	}
}

// Update handles messages and key events.
func (d DAGTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case DAGSessionsLoadedMsg:
		d.loading = false
		d.err = msg.Err
		d.sessions = msg.Sessions
		d.selectedIdx = 0
		if len(d.sessions) > 0 {
			cmds = append(cmds, d.loadDAGCmd())
		}
		return d, tea.Batch(cmds...)

	case DAGLoadedMsg:
		d.err = msg.Err
		d.dag = msg.DAG
		d.refreshViewport()
		return d, nil

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyLeft || msg.String() == "h":
			if d.selectedIdx > 0 {
				d.selectedIdx--
				cmds = append(cmds, d.loadDAGCmd())
			}
			return d, tea.Batch(cmds...)

		case msg.Type == tea.KeyRight || msg.String() == "l":
			if d.selectedIdx < len(d.sessions)-1 {
				d.selectedIdx++
				cmds = append(cmds, d.loadDAGCmd())
			}
			return d, tea.Batch(cmds...)
		}

		// Delegate viewport scrolling keys.
		var vpCmd tea.Cmd
		d.viewport, vpCmd = d.viewport.Update(msg)
		if vpCmd != nil {
			cmds = append(cmds, vpCmd)
		}
		return d, tea.Batch(cmds...)
	}

	// Delegate other messages (e.g., mouse) to the viewport.
	var vpCmd tea.Cmd
	d.viewport, vpCmd = d.viewport.Update(msg)
	if vpCmd != nil {
		cmds = append(cmds, vpCmd)
	}
	return d, tea.Batch(cmds...)
}

// loadDAGCmd returns a Cmd that parses the DAG for the currently selected session.
func (d DAGTab) loadDAGCmd() tea.Cmd {
	if len(d.sessions) == 0 || d.selectedIdx >= len(d.sessions) {
		return nil
	}
	sess := d.sessions[d.selectedIdx]
	sessionDir := filepath.Dir(sess.FilePath)
	return func() tea.Msg {
		dag, err := data.ParseDAG(sessionDir)
		return DAGLoadedMsg{DAG: dag, Err: err}
	}
}

// refreshViewport rebuilds the viewport content from the current DAG.
func (d *DAGTab) refreshViewport() {
	contentWidth := d.width
	if contentWidth <= 0 {
		contentWidth = 80
	}
	d.viewport.SetContent(RenderDAG(d.dag, contentWidth))
	d.viewport.GotoTop()
}

// SetSize updates the viewport and stored dimensions.
func (d DAGTab) SetSize(width, height int) TabModel {
	d.width = width
	d.height = height

	headerHeight := 2  // session header + hint line
	footerHeight := 2  // separator + status line
	vpHeight := height - headerHeight - footerHeight
	if vpHeight < 1 {
		vpHeight = 1
	}

	d.viewport.Width = width
	d.viewport.Height = vpHeight

	// Refresh content with new width if DAG is available.
	if d.dag != nil {
		d.viewport.SetContent(RenderDAG(d.dag, width))
	}

	return d
}

// View renders the DAG tab.
func (d DAGTab) View() string {
	if d.loading {
		return lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true).
			Render("Loading sessions...")
	}

	if d.err != nil && d.dag == nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF6B6B"}).
			Render(fmt.Sprintf("Error: %s", d.err))
	}

	if len(d.sessions) == 0 {
		return lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true).
			Render("No sessions found")
	}

	header := d.renderHeader()
	content := d.viewport.View()
	footer := d.renderFooter()

	return strings.Join([]string{header, content, footer}, "\n")
}

// renderHeader renders the session selector bar.
func (d DAGTab) renderHeader() string {
	headerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(borderColor).
		Width(d.width - 2).
		Padding(0, 1)

	var sessionLabel string
	if len(d.sessions) > 0 && d.selectedIdx < len(d.sessions) {
		sess := d.sessions[d.selectedIdx]
		shortID := sess.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		ts := sess.Timestamp.Format("01-02 15:04")
		sessionLabel = fmt.Sprintf("Session: %s... (%s)", shortID, ts)
	} else {
		sessionLabel = "No session selected"
	}

	hint := lipgloss.NewStyle().Foreground(dimColor).Render(" ◄ ► to switch sessions")
	return headerStyle.Render(sessionLabel + hint)
}

// renderFooter renders the summary status bar.
func (d DAGTab) renderFooter() string {
	footerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(borderColor).
		Width(d.width - 2).
		Padding(0, 1)

	var parts []string

	if d.dag != nil {
		agentCount := countDAGNodes(d.dag) - 1 // subtract root
		if agentCount < 0 {
			agentCount = 0
		}
		totalTokens := sumDAGTokens(d.dag)
		parts = append(parts,
			fmt.Sprintf("Total: %d agents", agentCount),
			dagFormatTokens(totalTokens)+" tokens",
		)
	}

	parts = append(parts, "◄►:switch  ↑↓/j/k:scroll")

	return footerStyle.Render(strings.Join(parts, " │ "))
}

// countDAGNodes counts all nodes in the DAG tree (including root).
func countDAGNodes(node *data.DAGNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += countDAGNodes(child)
	}
	return count
}

// sumDAGTokens sums TokenCount across all nodes.
func sumDAGTokens(node *data.DAGNode) int {
	if node == nil {
		return 0
	}
	total := node.TokenCount
	for _, child := range node.Children {
		total += sumDAGTokens(child)
	}
	return total
}

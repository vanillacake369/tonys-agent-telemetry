package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
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

	case event.EventMsg:
		return d.handleEventMsg(msg)

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

// handleEventMsg reacts to real-time FIFO events from the hook handler.
func (d DAGTab) handleEventMsg(msg event.EventMsg) (TabModel, tea.Cmd) {
	switch msg.Event.HookType {
	case "SubagentStop":
		// Reload the DAG for the currently selected session to reflect the new state.
		return d, d.loadDAGCmd()

	case "SessionStart":
		// A new session appeared — reload the session list.
		return d, func() tea.Msg {
			sessions, err := data.DiscoverSessions()
			return DAGSessionsLoadedMsg{Sessions: sessions, Err: err}
		}

	case "Stop":
		// The current session completed — reload DAG to show final state.
		return d, d.loadDAGCmd()
	}

	return d, nil
}

// loadDAGCmd returns a Cmd that parses the DAG for the currently selected session.
func (d DAGTab) loadDAGCmd() tea.Cmd {
	if len(d.sessions) == 0 || d.selectedIdx >= len(d.sessions) {
		return nil
	}
	sess := d.sessions[d.selectedIdx]
	return func() tea.Msg {
		dag, err := data.ParseDAG(sess.FilePath)
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

	// Header: 2 lines (session label row + blank separation)
	// Stats : 2 lines (separator + summary)
	// Viewport fills the rest.
	headerHeight := 2
	statsHeight := 2
	vpHeight := max(1, height-headerHeight-statsHeight)

	d.viewport.Width = max(1, width)
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
			Foreground(colorDim).
			Italic(true).
			Render("Loading sessions...")
	}

	if d.err != nil && d.dag == nil {
		return lipgloss.NewStyle().
			Foreground(colorError).
			Render(fmt.Sprintf("Error: %s", d.err))
	}

	if len(d.sessions) == 0 {
		return lipgloss.NewStyle().
			Foreground(colorDim).
			Italic(true).
			Render("No sessions found")
	}

	header := d.renderHeader()
	content := d.viewport.View()
	stats := d.renderStats()

	// header = 2 lines, stats = 2 lines (matching SetSize accounting)
	return strings.Join([]string{header, "", content, "", stats}, "\n")
}

// renderHeader renders the session selector bar — dim text, no border.
func (d DAGTab) renderHeader() string {
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

	hint := lipgloss.NewStyle().Foreground(colorDim).Render("  ◄ ► to switch sessions")
	label := lipgloss.NewStyle().Foreground(colorText).Render(sessionLabel)

	line := label + hint
	return lipgloss.NewStyle().Width(max(0, d.width)).Render(line)
}

// renderStats renders a single summary line below the viewport.
func (d DAGTab) renderStats() string {
	var parts []string

	if d.dag != nil {
		agentCount := countDAGNodes(d.dag) - 1 // subtract root
		if agentCount < 0 {
			agentCount = 0
		}
		totalTokens := sumDAGTokens(d.dag)
		parts = append(parts,
			fmt.Sprintf("%d agents", agentCount),
			dagFormatTokens(totalTokens)+" tokens",
		)
	}

	text := strings.Join(parts, "  │  ")
	return lipgloss.NewStyle().
		Foreground(colorDim).
		Width(max(0, d.width)).
		Render(text)
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

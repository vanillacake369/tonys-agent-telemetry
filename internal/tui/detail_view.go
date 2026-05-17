package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

// DetailLoadedMsg is sent when ParseFullConversation completes.
type DetailLoadedMsg struct {
	Turns []data.DetailTurn
	Err   error
}

// DetailView is a zellij-style overlay that shows the full conversation
// of a single session. It covers ~90% of the terminal, leaving a thin
// border to remind the user they're in an overlay.
type DetailView struct {
	session       data.Session
	turns         []data.DetailTurn
	toolCollapsed map[int]bool // turn index → collapsed state (default true)
	turnLineStart []int        // line number where each turn starts (for {/} jump)
	viewport      viewport.Model
	width         int
	height        int
	loading       bool
	err           error
	keys          KeyMap
	pendingG      bool // true when 'g' was pressed, waiting for second key
}

// NewDetailView creates a detail view for the given session.
func NewDetailView(session data.Session, width, height int) DetailView {
	vp := viewport.New(max(1, width-4), max(1, height-6))
	vp.Style = lipgloss.NewStyle()
	return DetailView{
		session:       session,
		viewport:      vp,
		width:         width,
		height:        height,
		loading:       true,
		toolCollapsed: make(map[int]bool),
		keys:          DefaultKeyMap(),
	}
}

// Init returns a command to load the full conversation.
func (d DetailView) Init() tea.Cmd {
	session := d.session
	return func() tea.Msg {
		p := data.GetProvider(session.Provider)
		if p == nil {
			return DetailLoadedMsg{Err: fmt.Errorf("unknown provider: %s", session.Provider)}
		}
		turns, err := p.ParseFullConversation(session.FilePath)
		return DetailLoadedMsg{Turns: turns, Err: err}
	}
}

// Update handles messages for the detail overlay.
func (d DetailView) Update(msg tea.Msg) (DetailView, tea.Cmd) {
	switch msg := msg.(type) {
	case DetailLoadedMsg:
		d.loading = false
		d.err = msg.Err
		d.turns = msg.Turns
		// Default: all tool blocks collapsed.
		for i, t := range d.turns {
			if len(t.ToolCalls) > 0 {
				d.toolCollapsed[i] = true
			}
		}
		d.rebuildContent()
		return d, nil

	case tea.KeyMsg:
		// Handle pending 'g' for gg combo.
		if d.pendingG {
			d.pendingG = false
			if msg.String() == "g" {
				// gg → go to top
				d.viewport.GotoTop()
				return d, nil
			}
			// Not 'g' — fall through to normal handling.
		}

		switch {
		case key.Matches(msg, d.keys.Escape):
			// Signal to close overlay — handled by parent.
			return d, nil

		case msg.String() == "t":
			// Toggle tool collapse for the nearest tool turn.
			d.toggleNearestTool()
			d.rebuildContent()
			return d, nil

		// ── Vim motions ──
		case msg.String() == "g":
			// First 'g' press — wait for second key.
			d.pendingG = true
			return d, nil

		case msg.String() == "G":
			// G → go to bottom
			d.viewport.GotoBottom()
			return d, nil

		case msg.String() == "{":
			// Jump to previous turn boundary.
			d.jumpToPrevTurn()
			return d, nil

		case msg.String() == "}":
			// Jump to next turn boundary.
			d.jumpToNextTurn()
			return d, nil

		case msg.Type == tea.KeyCtrlD:
			// Half-page down.
			half := max(1, d.viewport.Height/2)
			d.viewport.LineDown(half)
			return d, nil

		case msg.Type == tea.KeyCtrlU:
			// Half-page up.
			half := max(1, d.viewport.Height/2)
			d.viewport.LineUp(half)
			return d, nil

		case msg.Type == tea.KeyCtrlF || msg.Type == tea.KeyPgDown:
			// Full page down.
			d.viewport.LineDown(d.viewport.Height)
			return d, nil

		case msg.Type == tea.KeyCtrlB || msg.Type == tea.KeyPgUp:
			// Full page up.
			d.viewport.LineUp(d.viewport.Height)
			return d, nil
		}

		// Delegate scrolling to viewport (j/k/up/down).
		var vpCmd tea.Cmd
		d.viewport, vpCmd = d.viewport.Update(msg)
		return d, vpCmd
	}

	var vpCmd tea.Cmd
	d.viewport, vpCmd = d.viewport.Update(msg)
	return d, vpCmd
}

// SetSize updates dimensions.
func (d *DetailView) SetSize(width, height int) {
	d.width = width
	d.height = height
	d.viewport.Width = max(1, width-4)
	d.viewport.Height = max(1, height-6)
	if len(d.turns) > 0 {
		d.rebuildContent()
	}
}

// View renders the overlay.
func (d DetailView) View() string {
	overlayW := max(20, d.width-4)
	overlayH := max(8, d.height-2)
	innerW := max(10, overlayW-2)

	// Header line.
	header := d.renderHeader(innerW)

	// Body.
	var body string
	if d.loading {
		body = lipgloss.Place(innerW, max(1, overlayH-4),
			lipgloss.Center, lipgloss.Center,
			lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render("Loading conversation..."))
	} else if d.err != nil {
		body = lipgloss.NewStyle().Foreground(colorError).Width(innerW).Render("Error: " + d.err.Error())
	} else {
		body = d.viewport.View()
	}

	// Footer with key hints and scroll position.
	footer := d.renderFooter(innerW)

	inner := header + "\n" + body + "\n" + footer

	// Overlay border.
	overlayStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Width(innerW).
		Height(max(1, overlayH-2)).
		Padding(0, 1)

	rendered := overlayStyle.Render(inner)

	// Center on screen.
	return lipgloss.Place(d.width, d.height,
		lipgloss.Center, lipgloss.Center,
		rendered,
		lipgloss.WithWhitespaceForeground(lipgloss.Color("#333333")),
		lipgloss.WithWhitespaceChars("░"),
	)
}

// renderHeader renders the session info bar.
func (d DetailView) renderHeader(width int) string {
	s := d.session
	provLabel := string(s.Provider)
	switch s.Provider {
	case data.ProviderClaude:
		provLabel = "Claude"
	case data.ProviderCodex:
		provLabel = "Codex"
	case data.ProviderGemini:
		provLabel = "Gemini"
	}

	left := fmt.Sprintf("%s  %s", provLabel, s.ID)
	if len(left) > width/2 {
		left = left[:width/2]
	}

	var right string
	if s.Model != "" {
		right += s.Model + "  "
	}
	right += fmt.Sprintf("%d turns", s.TurnCount)
	if !s.Timestamp.IsZero() {
		right += "  " + s.Timestamp.Format("2006-01-02 15:04")
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Width(width)

	gap := max(0, width-lipgloss.Width(left)-lipgloss.Width(right))
	line := left + strings.Repeat(" ", gap) + right
	return headerStyle.Render(line)
}

// renderFooter renders the bottom hint bar with scroll position.
func (d DetailView) renderFooter(width int) string {
	hints := "j/k:scroll  {/}:turn  gg/G:top/bottom  ^d/^u:half-page  t:tools  Esc:close"
	scrollPct := ""
	if d.viewport.TotalLineCount() > 0 {
		pct := int(d.viewport.ScrollPercent() * 100)
		scrollPct = fmt.Sprintf("%d%%", pct)
	}

	footerStyle := lipgloss.NewStyle().
		Foreground(colorDim).
		Width(width)

	gap := max(0, width-lipgloss.Width(hints)-lipgloss.Width(scrollPct))
	line := hints + strings.Repeat(" ", gap) + scrollPct
	return footerStyle.Render(line)
}

// rebuildContent renders all turns into the viewport and records line offsets.
func (d *DetailView) rebuildContent() {
	contentW := max(10, d.viewport.Width-2)
	var sb strings.Builder
	d.turnLineStart = make([]int, 0, len(d.turns))
	lineCount := 0

	for i, turn := range d.turns {
		if i > 0 {
			sb.WriteString("\n")
			lineCount++
		}
		d.turnLineStart = append(d.turnLineStart, lineCount)
		rendered := d.renderTurn(i, turn, contentW)
		sb.WriteString(rendered)
		lineCount += strings.Count(rendered, "\n")
	}

	d.viewport.SetContent(sb.String())
}

// jumpToNextTurn scrolls to the start of the next turn relative to current position.
func (d *DetailView) jumpToNextTurn() {
	if len(d.turnLineStart) == 0 {
		return
	}
	currentLine := d.viewport.YOffset
	for _, lineStart := range d.turnLineStart {
		if lineStart > currentLine {
			d.viewport.SetYOffset(lineStart)
			return
		}
	}
	// Already past last turn — go to bottom.
	d.viewport.GotoBottom()
}

// jumpToPrevTurn scrolls to the start of the previous turn relative to current position.
func (d *DetailView) jumpToPrevTurn() {
	if len(d.turnLineStart) == 0 {
		return
	}
	currentLine := d.viewport.YOffset
	for i := len(d.turnLineStart) - 1; i >= 0; i-- {
		if d.turnLineStart[i] < currentLine {
			d.viewport.SetYOffset(d.turnLineStart[i])
			return
		}
	}
	// Already at first turn — go to top.
	d.viewport.GotoTop()
}

// renderTurn renders a single turn.
func (d DetailView) renderTurn(idx int, turn data.DetailTurn, width int) string {
	var sb strings.Builder

	// Role header with separator.
	roleStyle := lipgloss.NewStyle().Bold(true)
	sepStyle := lipgloss.NewStyle().Foreground(colorDim)

	var roleLabel string
	switch turn.Role {
	case "user":
		roleLabel = roleStyle.Foreground(colorPrimary).Render("[User]")
	case "assistant":
		label := "[Assistant]"
		if turn.Model != "" {
			label = fmt.Sprintf("[Assistant · %s]", turn.Model)
		}
		roleLabel = roleStyle.Foreground(colorSuccess).Render(label)
	default:
		roleLabel = roleStyle.Render("[" + turn.Role + "]")
	}

	// Token info for assistant turns.
	var tokenInfo string
	if turn.TokensIn > 0 || turn.TokensOut > 0 {
		tokenInfo = lipgloss.NewStyle().Foreground(colorDim).
			Render(fmt.Sprintf("  in:%d out:%d", turn.TokensIn, turn.TokensOut))
	}

	sb.WriteString(sepStyle.Render(strings.Repeat("─", min(width, 60))))
	sb.WriteString("\n")
	sb.WriteString(roleLabel + tokenInfo)
	sb.WriteString("\n")

	// Thinking block (collapsed by default display).
	if turn.Thinking != "" {
		thinkStyle := lipgloss.NewStyle().Foreground(colorDim).Italic(true)
		thinkingLines := strings.Count(turn.Thinking, "\n") + 1
		preview := turn.Thinking
		if len([]rune(preview)) > 120 {
			preview = string([]rune(preview)[:120]) + "…"
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		sb.WriteString(thinkStyle.Render(fmt.Sprintf("  [thinking: %d lines] %s", thinkingLines, preview)))
		sb.WriteString("\n")
	}

	// Main content.
	if turn.Content != "" {
		// Wrap long lines to fit viewport width.
		lines := strings.Split(turn.Content, "\n")
		for _, line := range lines {
			// Soft-wrap lines that exceed width.
			wrapped := softWrap(line, max(10, width-2))
			sb.WriteString("  " + wrapped + "\n")
		}
	}

	// Tool calls.
	if len(turn.ToolCalls) > 0 {
		collapsed := d.toolCollapsed[idx]
		toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))

		if collapsed {
			// Show compact summary.
			var names []string
			for _, tc := range turn.ToolCalls {
				names = append(names, tc.Name)
			}
			summary := strings.Join(names, ", ")
			sb.WriteString(toolStyle.Render(fmt.Sprintf("  [%d tools: %s] (t to expand)", len(turn.ToolCalls), summary)))
			sb.WriteString("\n")
		} else {
			// Show each tool call with input.
			for _, tc := range turn.ToolCalls {
				sb.WriteString(toolStyle.Render(fmt.Sprintf("  [Tool: %s]", tc.Name)))
				sb.WriteString("\n")
				if tc.Input != "" {
					inputStyle := lipgloss.NewStyle().Foreground(colorDim)
					// Show truncated input, wrapped.
					wrapped := softWrap(tc.Input, max(10, width-6))
					sb.WriteString(inputStyle.Render("    " + wrapped))
					sb.WriteString("\n")
				}
			}
		}
	}

	return sb.String()
}

// toggleNearestTool toggles the collapsed state of the tool block
// nearest to the current scroll position.
func (d *DetailView) toggleNearestTool() {
	// Find turns that have tools.
	var toolTurns []int
	for i, t := range d.turns {
		if len(t.ToolCalls) > 0 {
			toolTurns = append(toolTurns, i)
		}
	}
	if len(toolTurns) == 0 {
		return
	}

	// Toggle the first one, or if all are in same state, toggle all.
	// Simple approach: toggle all.
	allCollapsed := true
	for _, idx := range toolTurns {
		if !d.toolCollapsed[idx] {
			allCollapsed = false
			break
		}
	}
	for _, idx := range toolTurns {
		d.toolCollapsed[idx] = !allCollapsed
	}
}

// softWrap wraps a single line of text to fit within maxWidth cells.
func softWrap(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}

	var result strings.Builder
	runes := []rune(s)
	lineWidth := 0
	for _, r := range runes {
		rw := lipgloss.Width(string(r))
		if lineWidth+rw > maxWidth {
			result.WriteRune('\n')
			result.WriteString("  ") // continuation indent
			lineWidth = 2
		}
		result.WriteRune(r)
		lineWidth += rw
	}
	return result.String()
}

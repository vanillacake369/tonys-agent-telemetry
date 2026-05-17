package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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
	pendingG      bool   // true when 'g' was pressed, waiting for second key
	verbose       bool   // false=compact (default), true=verbose tool display
	query         string // active search query (for highlight + jump)
	matchTurns    []int  // indices of turns containing the query
	matchIdx      int    // current position in matchTurns (for n/N navigation)
	searching     bool   // true when "/" search input is active
	searchInput   textinput.Model
}

// NewDetailView creates a detail view for the given session.
// query is the search term from the session list (empty if opened without search).
func NewDetailView(session data.Session, width, height int, query string) DetailView {
	vp := viewport.New(max(1, width-4), max(1, height-6))
	vp.Style = lipgloss.NewStyle()

	ti := textinput.New()
	ti.Placeholder = "search..."
	ti.CharLimit = 100
	ti.Width = max(20, width/3)

	return DetailView{
		session:       session,
		viewport:      vp,
		width:         width,
		height:        height,
		loading:       true,
		toolCollapsed: make(map[int]bool),
		keys:          DefaultKeyMap(),
		query:         query,
		searchInput:   ti,
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
		// Build match index and auto-scroll to first match if query is set.
		if d.query != "" {
			d.buildMatchIndex()
			if len(d.matchTurns) > 0 && len(d.turnLineStart) > d.matchTurns[0] {
				d.viewport.SetYOffset(d.turnLineStart[d.matchTurns[0]])
			}
		}
		return d, nil

	case tea.KeyMsg:
		// ── Search input mode ──
		if d.searching {
			switch {
			case msg.Type == tea.KeyEscape:
				// Cancel search, keep old query.
				d.searching = false
				d.searchInput.Blur()
				return d, nil
			case msg.Type == tea.KeyEnter:
				// Confirm search.
				d.searching = false
				d.searchInput.Blur()
				newQuery := d.searchInput.Value()
				if newQuery != d.query {
					d.query = newQuery
					d.buildMatchIndex()
					d.matchIdx = 0
					// Jump to first match.
					if len(d.matchTurns) > 0 && d.matchTurns[0] < len(d.turnLineStart) {
						d.viewport.SetYOffset(d.turnLineStart[d.matchTurns[0]])
					}
				}
				return d, nil
			default:
				// Forward to text input.
				var tiCmd tea.Cmd
				d.searchInput, tiCmd = d.searchInput.Update(msg)
				return d, tiCmd
			}
		}

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
			// If there's an active query, clear it first. Second Esc closes overlay.
			if d.query != "" {
				d.query = ""
				d.matchTurns = nil
				d.searchInput.SetValue("")
				return d, nil
			}
			// Signal to close overlay — handled by parent.
			return d, nil

		case msg.String() == "/":
			// Open inline search.
			d.searching = true
			d.searchInput.SetValue(d.query)
			d.searchInput.Focus()
			return d, textinput.Blink

		case msg.String() == "t":
			// Toggle compact/verbose mode.
			d.verbose = !d.verbose
			d.rebuildContent()
			return d, nil

		case msg.String() == "n":
			// Jump to next match.
			if len(d.matchTurns) > 0 {
				d.matchIdx = (d.matchIdx + 1) % len(d.matchTurns)
				ti := d.matchTurns[d.matchIdx]
				if ti < len(d.turnLineStart) {
					d.viewport.SetYOffset(d.turnLineStart[ti])
				}
			}
			return d, nil

		case msg.String() == "N":
			// Jump to previous match.
			if len(d.matchTurns) > 0 {
				d.matchIdx = (d.matchIdx - 1 + len(d.matchTurns)) % len(d.matchTurns)
				ti := d.matchTurns[d.matchIdx]
				if ti < len(d.turnLineStart) {
					d.viewport.SetYOffset(d.turnLineStart[ti])
				}
			}
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

// renderFooter renders the bottom hint bar with scroll position and match info.
func (d DetailView) renderFooter(width int) string {
	// Search input mode: show the input inline.
	if d.searching {
		searchStyle := lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)
		prefix := searchStyle.Render("/")
		inputView := d.searchInput.View()
		return lipgloss.NewStyle().Width(width).Render(prefix + inputView)
	}

	mode := "compact"
	if d.verbose {
		mode = "verbose"
	}
	hints := fmt.Sprintf("j/k:scroll  {/}:turn  /:search  t:%s  Esc:close", mode)
	if len(d.matchTurns) > 0 {
		hints = fmt.Sprintf("n/N:match(%d/%d)  ", d.matchIdx+1, len(d.matchTurns)) + hints
	}

	scrollPct := ""
	if d.viewport.TotalLineCount() > 0 {
		pct := int(d.viewport.ScrollPercent() * 100)
		scrollPct = fmt.Sprintf("%d%%", pct)
	}

	footerStyle := lipgloss.NewStyle().
		Foreground(colorDim).
		Width(width)

	// Truncate hints if too wide.
	maxHints := max(10, width-lipgloss.Width(scrollPct)-2)
	if lipgloss.Width(hints) > maxHints {
		hints = truncateToWidth(hints, maxHints)
	}

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

// renderTurn renders a single turn in compact or verbose mode.
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
		if d.verbose && turn.Model != "" {
			roleLabel = roleStyle.Foreground(colorSuccess).Render(fmt.Sprintf("[Assistant · %s]", turn.Model))
		} else {
			// Compact: short model suffix.
			suffix := shortModel(turn.Model)
			if suffix != "" {
				roleLabel = roleStyle.Foreground(colorSuccess).Render("[Assistant]") +
					lipgloss.NewStyle().Foreground(colorDim).Render("  "+suffix)
			} else {
				roleLabel = roleStyle.Foreground(colorSuccess).Render("[Assistant]")
			}
		}
	default:
		roleLabel = roleStyle.Render("[" + turn.Role + "]")
	}

	// Token info — compact format.
	var tokenInfo string
	if turn.TokensIn > 0 || turn.TokensOut > 0 {
		tokenInfo = lipgloss.NewStyle().Foreground(colorDim).
			Render(fmt.Sprintf("  %s→%s", compactTokens(turn.TokensIn), compactTokens(turn.TokensOut)))
	}

	sb.WriteString(sepStyle.Render(strings.Repeat("─", min(width, 60))))
	sb.WriteString("\n")
	sb.WriteString(roleLabel + tokenInfo)
	sb.WriteString("\n")

	// Thinking block — hidden in compact mode, summary in verbose.
	if turn.Thinking != "" {
		if d.verbose {
			thinkStyle := lipgloss.NewStyle().Foreground(colorDim).Italic(true)
			thinkingLines := strings.Count(turn.Thinking, "\n") + 1
			preview := turn.Thinking
			if len([]rune(preview)) > 200 {
				preview = string([]rune(preview)[:200]) + "…"
			}
			preview = strings.ReplaceAll(preview, "\n", " ")
			sb.WriteString(thinkStyle.Render(fmt.Sprintf("  [thinking: %d lines] %s", thinkingLines, preview)))
			sb.WriteString("\n")
		}
		// Compact: thinking entirely hidden for clean readability.
	}

	// Main content.
	if turn.Content != "" {
		lines := strings.Split(turn.Content, "\n")
		for _, line := range lines {
			wrapped := softWrap(line, max(10, width-2))
			sb.WriteString("  " + wrapped + "\n")
		}
	}

	// Tool calls.
	if len(turn.ToolCalls) > 0 {
		toolStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))

		if !d.verbose {
			// Compact: one line with Name(key_arg) format.
			sb.WriteString(toolStyle.Render("  ┊ " + abbreviateTools(turn.ToolCalls)))
			sb.WriteString("\n")
		} else {
			// Verbose: each tool call with full input.
			for _, tc := range turn.ToolCalls {
				sb.WriteString(toolStyle.Render(fmt.Sprintf("  [Tool: %s]", tc.Name)))
				sb.WriteString("\n")
				if tc.Input != "" {
					inputStyle := lipgloss.NewStyle().Foreground(colorDim)
					wrapped := softWrap(tc.Input, max(10, width-6))
					sb.WriteString(inputStyle.Render("    " + wrapped))
					sb.WriteString("\n")
				}
			}
		}
	}

	return sb.String()
}

// abbreviateTools creates a compact one-line summary of tool calls.
// Format: "Bash(go test), Read(main.go), Edit(main.go)"
func abbreviateTools(tools []data.ToolCall) string {
	var parts []string
	for _, tc := range tools {
		arg := extractKeyArg(tc.Name, tc.Input)
		if arg != "" {
			parts = append(parts, fmt.Sprintf("%s(%s)", tc.Name, arg))
		} else {
			parts = append(parts, tc.Name)
		}
	}
	return strings.Join(parts, ", ")
}

// extractKeyArg pulls the most informative short argument from tool input JSON.
func extractKeyArg(toolName, input string) string {
	if input == "" {
		return ""
	}
	// Quick JSON field extraction without full unmarshal.
	switch toolName {
	case "Bash":
		return extractJSONField(input, "command", 30)
	case "Read", "Edit", "Write", "MultiEdit":
		fp := extractJSONField(input, "file_path", 0)
		if fp != "" {
			// Show just the filename.
			parts := strings.Split(fp, "/")
			return parts[len(parts)-1]
		}
		return ""
	case "Grep":
		return extractJSONField(input, "pattern", 20)
	case "Glob":
		return extractJSONField(input, "pattern", 20)
	case "Agent":
		return extractJSONField(input, "description", 25)
	default:
		return ""
	}
}

// extractJSONField is a lightweight extractor for a single string field from JSON.
// maxLen of 0 means no truncation.
func extractJSONField(jsonStr, field string, maxLen int) string {
	// Look for "field":"value" or "field": "value"
	key := fmt.Sprintf(`"%s"`, field)
	idx := strings.Index(jsonStr, key)
	if idx < 0 {
		return ""
	}
	// Find the colon after the key.
	rest := jsonStr[idx+len(key):]
	colonIdx := strings.Index(rest, ":")
	if colonIdx < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[colonIdx+1:])
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	// Find closing quote.
	end := strings.Index(rest[1:], `"`)
	if end < 0 {
		return ""
	}
	val := rest[1 : end+1]
	// Unescape basic sequences.
	val = strings.ReplaceAll(val, `\"`, `"`)
	val = strings.ReplaceAll(val, `\\`, `\`)
	if maxLen > 0 && len([]rune(val)) > maxLen {
		val = string([]rune(val)[:maxLen]) + "…"
	}
	return val
}

// shortModel returns a compact model name suffix.
func shortModel(model string) string {
	if model == "" {
		return ""
	}
	// "claude-opus-4-6" → "opus-4-6"
	// "claude-sonnet-4-6" → "sonnet-4-6"
	// "gpt-5" → "gpt-5"
	parts := strings.Split(model, "-")
	if len(parts) >= 3 && parts[0] == "claude" {
		return strings.Join(parts[1:], "-")
	}
	return model
}

// compactTokens formats token count in compact K/M format.
func compactTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// buildMatchIndex finds all turn indices whose content contains the query.
func (d *DetailView) buildMatchIndex() {
	d.matchTurns = nil
	d.matchIdx = 0
	if d.query == "" {
		return
	}
	lowerQ := strings.ToLower(d.query)
	for i, turn := range d.turns {
		text := strings.ToLower(turn.Content + " " + turn.Thinking)
		if strings.Contains(text, lowerQ) {
			d.matchTurns = append(d.matchTurns, i)
		}
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

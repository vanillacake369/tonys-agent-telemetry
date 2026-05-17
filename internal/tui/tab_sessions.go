package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/platform"
)

// SessionsLoadedMsg is sent when DiscoverSessions completes.
type SessionsLoadedMsg struct {
	Sessions []data.Session
	Err      error
}

// PreviewLoadedMsg is sent when ParseConversationPreview completes.
type PreviewLoadedMsg struct {
	Turns []data.Turn
	Err   error
}

// FileChangesLoadedMsg is sent when ParseFileChanges completes.
type FileChangesLoadedMsg struct {
	Changes []data.FileChange
	Err     error
}

// OpenDetailMsg is sent to the App to open a detail overlay.
type OpenDetailMsg struct {
	Session data.Session
}

// SessionsTab implements TabModel for the Sessions tab.
type SessionsTab struct {
	sessions    []data.Session
	filtered    []data.Session
	searchIndex []string // pre-computed lowercase search targets, one per session
	cursor      int
	searchInput textinput.Model
	preview     []data.Turn
	fileChanges []data.FileChange
	width       int
	height      int
	loading     bool
	err         error
	keys        KeyMap
}

// NewSessionsTab creates an initialised SessionsTab.
func NewSessionsTab() SessionsTab {
	ti := textinput.New()
	ti.Placeholder = "/ to search"
	ti.CharLimit = 200
	ti.Width = 30

	return SessionsTab{
		searchInput: ti,
		loading:     true,
		keys:        DefaultKeyMap(),
	}
}

// Init loads sessions asynchronously from all providers.
func (s SessionsTab) Init() tea.Cmd {
	return func() tea.Msg {
		sessions, err := data.DiscoverAllSessions()
		return SessionsLoadedMsg{Sessions: sessions, Err: err}
	}
}

// Update handles messages and key events.
func (s SessionsTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case SessionsLoadedMsg:
		s.loading = false
		s.err = msg.Err
		s.sessions = msg.Sessions
		s.filtered = msg.Sessions
		s.cursor = 0
		s.fileChanges = nil
		s.searchIndex = make([]string, len(s.sessions))
		for i, sess := range s.sessions {
			s.searchIndex[i] = strings.ToLower(strings.Join([]string{
				sess.SearchText, sess.CWD, sess.GitBranch, sess.Model,
			}, " "))
		}
		return s, s.loadPreviewCmd()

	case PreviewLoadedMsg:
		if msg.Err == nil {
			s.preview = msg.Turns
		} else {
			s.preview = nil
		}
		return s, s.loadFileChangesCmd()

	case FileChangesLoadedMsg:
		if msg.Err == nil {
			s.fileChanges = msg.Changes
		} else {
			s.fileChanges = nil
		}
		return s, nil

	case SearchFocusMsg:
		s.searchInput.Focus()
		return s, textinput.Blink

	case SearchBlurMsg:
		s.searchInput.Blur()
		return s, nil

	case tea.KeyMsg:
		// Block Enter in search mode — Esc to exit search, Enter is for actions.
		if s.searchInput.Focused() && msg.Type == tea.KeyEnter {
			return s, nil
		}

		// When search input is focused, forward keys to the text input.
		if s.searchInput.Focused() {
			var tiCmd tea.Cmd
			s.searchInput, tiCmd = s.searchInput.Update(msg)
			cmds = append(cmds, tiCmd)
			s.applyFilter()
			if s.cursor >= len(s.filtered) {
				s.cursor = max(0, len(s.filtered)-1)
			}
			s.fileChanges = nil
			cmds = append(cmds, s.loadPreviewCmd())
			return s, tea.Batch(cmds...)
		}

		// Navigation mode: handle single-key bindings.
		switch {
		case key.Matches(msg, s.keys.Refresh):
			s.loading = true
			s.sessions = nil
			s.filtered = nil
			s.preview = nil
			s.cursor = 0
			return s, s.Init()

		case key.Matches(msg, s.keys.View):
			if len(s.filtered) > 0 && s.cursor < len(s.filtered) {
				session := s.filtered[s.cursor]
				return s, func() tea.Msg { return OpenDetailMsg{Session: session} }
			}
			return s, nil

		case key.Matches(msg, s.keys.Enter):
			if len(s.filtered) > 0 && s.cursor < len(s.filtered) {
				session := s.filtered[s.cursor]
				if p := data.GetProvider(session.Provider); p != nil {
					cmd := p.ResumeCommand(session)
					_ = platform.Detect().OpenPane(cmd)
				}
			}
			return s, nil

		case key.Matches(msg, s.keys.Fork):
			if len(s.filtered) > 0 && s.cursor < len(s.filtered) {
				session := s.filtered[s.cursor]
				cmd := resumeForkCommand(session)
				_ = platform.Detect().OpenPane(cmd)
			}
			return s, nil

		case key.Matches(msg, s.keys.Copy):
			if len(s.filtered) > 0 && s.cursor < len(s.filtered) {
				session := s.filtered[s.cursor]
				_ = platform.CopyToClipboard(session.ID)
			}
			return s, nil

		case key.Matches(msg, s.keys.Up):
			if s.cursor > 0 {
				s.cursor--
				s.fileChanges = nil
				return s, s.loadPreviewCmd()
			}
			return s, nil

		case key.Matches(msg, s.keys.Down):
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
				s.fileChanges = nil
				return s, s.loadPreviewCmd()
			}
			return s, nil
		}

		return s, nil
	}

	// Pass through to text input for non-key messages.
	var tiCmd tea.Cmd
	s.searchInput, tiCmd = s.searchInput.Update(msg)
	cmds = append(cmds, tiCmd)
	return s, tea.Batch(cmds...)
}

// applyFilter uses case-insensitive substring match across all session content.
// Fuzzy matching is too loose on long SearchText (2KB) — substring is more precise.
// Search targets are pre-computed in searchIndex at load time to avoid redundant
// ToLower + Join work on every keystroke.
func (s *SessionsTab) applyFilter() {
	query := strings.ToLower(s.searchInput.Value())
	if query == "" {
		s.filtered = s.sessions
		return
	}

	filtered := make([]data.Session, 0)
	for i, idx := range s.searchIndex {
		if strings.Contains(idx, query) {
			filtered = append(filtered, s.sessions[i])
		}
	}
	s.filtered = filtered
}

// resumeForkCommand returns the fork/new-session command based on provider.
func resumeForkCommand(session data.Session) string {
	cwd := session.CWD
	cdPrefix := ""
	if cwd != "" {
		cdPrefix = fmt.Sprintf("cd %q && ", cwd)
	}
	switch session.Provider {
	case data.ProviderClaude:
		return cdPrefix + fmt.Sprintf("claude --resume %s --fork-session", session.ID)
	case data.ProviderCodex:
		return cdPrefix + fmt.Sprintf("codex fork %s", session.ID)
	case data.ProviderGemini:
		// Gemini doesn't have fork; just start a new session in same dir.
		return cdPrefix + "gemini"
	default:
		return cdPrefix + "echo 'unknown provider'"
	}
}

// loadPreviewCmd returns a tea.Cmd that loads the conversation preview for the
// currently selected session. Returns nil when there is no selected session.
func (s SessionsTab) loadPreviewCmd() tea.Cmd {
	if len(s.filtered) == 0 || s.cursor >= len(s.filtered) {
		return nil
	}
	session := s.filtered[s.cursor]
	return func() tea.Msg {
		p := data.GetProvider(session.Provider)
		if p == nil {
			return PreviewLoadedMsg{Err: fmt.Errorf("unknown provider")}
		}
		turns, err := p.ParseConversationPreview(session.FilePath, 5)
		return PreviewLoadedMsg{Turns: turns, Err: err}
	}
}

// loadFileChangesCmd returns a tea.Cmd that loads file changes for the
// currently selected session. Returns nil when there is no selected session.
func (s SessionsTab) loadFileChangesCmd() tea.Cmd {
	if len(s.filtered) == 0 || s.cursor >= len(s.filtered) {
		return nil
	}
	session := s.filtered[s.cursor]
	return func() tea.Msg {
		p := data.GetProvider(session.Provider)
		if p == nil {
			return FileChangesLoadedMsg{Err: fmt.Errorf("unknown provider")}
		}
		changes, err := p.ParseFileChanges(session.FilePath)
		return FileChangesLoadedMsg{Changes: changes, Err: err}
	}
}

// SetSize stores the terminal dimensions.
func (s SessionsTab) SetSize(width, height int) TabModel {
	s.width = width
	s.height = height
	// Panel border (2) + left padding (1) + cursor prefix (2) = 5
	// Use split layout width for left panel, not full tab width
	leftW, _, _ := SplitLayout(width, 40)
	s.searchInput.Width = max(1, leftW-7)
	return s
}

// View renders the sessions tab.
func (s SessionsTab) View() string {
	if s.width == 0 || s.height == 0 {
		if s.loading {
			return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("Loading sessions...")
		}
		return "Sessions"
	}

	if s.loading {
		return RenderLoadingState(s.width, s.height)
	}

	if s.err != nil {
		return renderErrorState(s.err, s.width)
	}

	// Layout: search input is embedded inside the Sessions panel (1 line).
	// The full height is used for the list+preview panels.
	listHeight := max(3, s.height)

	leftW, rightW, showPreview := SplitLayout(s.width, 40)

	var splitView string
	if showPreview {
		leftContent := s.renderSessionListWithSearch(max(1, leftW-2), max(1, listHeight-2))
		leftPanel := RenderPanel("Sessions", leftContent, leftW, listHeight, true)
		rightContent := s.renderPreview(max(1, rightW-2), max(1, listHeight-2))
		rightPanel := RenderPanel("Preview", rightContent, rightW, listHeight, false)
		splitView = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	} else {
		leftContent := s.renderSessionListWithSearch(max(1, leftW-2), max(1, listHeight-2))
		splitView = RenderPanel("Sessions", leftContent, leftW, listHeight, true)
	}

	return splitView
}

// renderSessionListWithSearch renders search + column header + session list.
func (s SessionsTab) renderSessionListWithSearch(width, height int) string {
	searchLine := " " + s.searchInput.View()
	headerLine := s.renderColumnHeader(width)
	headerLines := 1
	if width >= 50 {
		headerLines = 2 // header + separator line
	}
	listHeight := max(1, height-1-headerLines) // -1 search, -N header
	listContent := s.renderSessionList(width, listHeight)
	return searchLine + "\n" + headerLine + "\n" + listContent
}

// renderColumnHeader renders a btop/k9s-style column header row.
// Column widths must match those used in formatSessionLine (SSoT).
// The " ▸ " cursor prefix is 3 chars, added by RenderListItem outside formatSessionLine.
func (s SessionsTab) renderColumnHeader(width int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true)
	dimSep := lipgloss.NewStyle().Foreground(colorDim)

	// 3-char cursor prefix + content columns must align.
	const cursor = "   " // matches RenderListItem's "   " indent
	const srcW = 7       // matches formatSessionLine's srcW

	if width < 35 {
		return headerStyle.Render(cursor + PadToWidth("SRC", srcW) + "PROMPT")
	}
	if width < 50 {
		return headerStyle.Render(cursor + PadToWidth("SRC", srcW) + PadToWidth("DATE", 12) + "PROMPT")
	}
	// Wide: SRC(7) + date(12) + project(13) + stats(8) + prompt
	header := cursor +
		PadToWidth("SRC", srcW) +
		PadToWidth("DATE", 12) +
		PadToWidth("PROJECT", 13) +
		PadToWidth("T  DUR", 8) +
		"PROMPT"
	return headerStyle.Render(header) +
		"\n" + dimSep.Render(strings.Repeat("─", min(width, 80)))
}

// renderSessionList renders the filtered sessions list.
func (s SessionsTab) renderSessionList(width, height int) string {
	if len(s.filtered) == 0 {
		return RenderEmptyState("No sessions found", width, height)
	}

	// Determine scroll window so the cursor is always visible.
	scrollOffset := s.cursor - height + 1
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	query := s.searchInput.Value()
	baseStyle := lipgloss.NewStyle().Foreground(colorText)

	var rows []string
	for i := scrollOffset; i < len(s.filtered) && i < scrollOffset+height; i++ {
		sess := s.filtered[i]
		// Account for the 3-char " ▸ " / "   " prefix in RenderListItem.
		line := s.formatSessionLine(sess, max(1, width-3))
		if query != "" {
			line = HighlightMatch(line, query, baseStyle)
		}
		rows = append(rows, RenderListItem(line, i == s.cursor, width))
	}

	return strings.Join(rows, "\n")
}

// providerLabel returns a fixed-width display label for the session's provider.
// All labels are exactly 6 chars wide for column alignment.
func providerLabel(p data.ProviderName) string {
	switch p {
	case data.ProviderClaude:
		return "Claude"
	case data.ProviderCodex:
		return "Codex "
	case data.ProviderGemini:
		return "Gemini"
	default:
		return "      "
	}
}

// formatSessionLine formats a single session entry with responsive column widths.
// Uses PadToWidth for CJK-safe alignment (double-width chars = 2 cells).
// Narrow (<35): prompt only. Medium (<50): date + prompt. Wide: date + project + stats + prompt.
// When a search query matches in SearchText but NOT in FirstPrompt, the matching
// context from SearchText is shown instead of the first prompt.
func (s SessionsTab) formatSessionLine(sess data.Session, maxWidth int) string {
	timestamp := sess.Timestamp.Format("01-02 15:04")
	project := filepath.Base(sess.CWD)
	if project == "" || project == "." {
		project = filepath.Base(sess.ProjectDir)
	}
	srcLabel := providerLabel(sess.Provider)

	// Determine which text to display as the prompt column.
	prompt := strings.ReplaceAll(sess.FirstPrompt, "\n", " ")
	query := s.searchInput.Value()
	if query != "" && !strings.Contains(strings.ToLower(prompt), strings.ToLower(query)) {
		// Match is in SearchText but not in FirstPrompt — show surrounding context.
		ctx := findMatchContext(sess.SearchText, query, maxWidth/2)
		if ctx != "" {
			prompt = "…" + strings.ReplaceAll(ctx, "\n", " ") + "…"
		}
	}

	// trunc cuts plain text to w runes — NO ANSI escape sequences.
	// formatSessionLine must return plain text so HighlightMatch works correctly.
	trunc := func(str string, w int) string {
		return truncateToWidth(str, w)
	}

	// All modes include SRC(7) column: 6-char label + 1 space.
	const srcW = 7

	if maxWidth < 35 {
		remaining := max(1, maxWidth-srcW)
		return PadToWidth(srcLabel, srcW) + trunc(prompt, remaining)
	}

	if maxWidth < 50 {
		// Medium: SRC(7) + date(12) + prompt(remaining)
		dateCol := PadToWidth(timestamp, 12)
		remaining := max(1, maxWidth-srcW-12)
		return PadToWidth(srcLabel, srcW) + dateCol + trunc(prompt, remaining)
	}

	// Wide: SRC(7) + date(12) + project(13) + stats(8) + prompt(remaining)
	dateCol := PadToWidth(timestamp, 12)
	projCol := PadToWidth(trunc(project, 12), 13) // 12 data + 1 separator space

	// Stats column: turn count and duration (e.g. "3t 4m "), fixed width 8.
	var statsRaw string
	if sess.TurnCount > 0 || sess.Duration > 0 {
		minutes := int(sess.Duration.Minutes())
		statsRaw = fmt.Sprintf("%dt %dm ", sess.TurnCount, minutes)
	}
	statsCol := PadToWidth(trunc(statsRaw, 8), 8)

	prefix := PadToWidth(srcLabel, srcW) + dateCol + projCol + statsCol
	prefixWidth := lipgloss.Width(prefix)
	remaining := max(1, maxWidth-prefixWidth)
	return prefix + trunc(prompt, remaining)
}

// findMatchContext finds query in text and returns the surrounding context
// centered on the match, up to maxLen runes. Uses rune-based indexing
// to avoid splitting multi-byte CJK characters.
func findMatchContext(text, query string, maxLen int) string {
	if text == "" || query == "" || maxLen <= 0 {
		return ""
	}
	runes := []rune(text)
	lowerRunes := []rune(strings.ToLower(text))
	queryRunes := []rune(strings.ToLower(query))

	idx := runeIndex(lowerRunes, queryRunes)
	if idx < 0 {
		return ""
	}
	// Center the match in the context window.
	start := idx - maxLen/2
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}

// renderPreview renders the right panel with the conversation preview and file changes.
func (s SessionsTab) renderPreview(width, height int) string {
	if len(s.filtered) == 0 {
		return RenderEmptyState("No session selected", width, height)
	}

	if len(s.preview) == 0 {
		return RenderEmptyState("No preview available", width, height)
	}

	// Render turns.
	var lines []string
	for _, turn := range s.preview {
		roleStyle := lipgloss.NewStyle().Bold(true)
		var roleLabel string
		switch turn.Role {
		case "user":
			roleLabel = roleStyle.Foreground(colorPrimary).Render("[User]")
		case "assistant":
			roleLabel = roleStyle.Foreground(colorSuccess).Render("[Asst]")
		default:
			roleLabel = roleStyle.Render("[" + turn.Role + "]")
		}

		// Wrap content to fit in width.
		contentWidth := max(10, width-8) // account for role label
		content := truncateToWidth(
			strings.ReplaceAll(turn.Content, "\n", " "),
			contentWidth,
		)

		lines = append(lines, fmt.Sprintf("%s %s", roleLabel, content))
	}

	// Render file changes section below conversation preview.
	if len(s.fileChanges) > 0 {
		dimStyle := lipgloss.NewStyle().Foreground(colorDim)
		lines = append(lines, "")
		lines = append(lines, dimStyle.Render("── Files Changed ──"))

		const maxShown = 10
		shown := s.fileChanges
		extra := 0
		if len(shown) > maxShown {
			extra = len(shown) - maxShown
			shown = shown[:maxShown]
		}
		for _, fc := range shown {
			var icon string
			switch fc.Action {
			case "write":
				icon = "✨"
			case "edit":
				icon = "✏️"
			default:
				icon = "📄"
			}
			suffix := ""
			if fc.Action == "read" {
				suffix = " (read)"
			} else if fc.Action == "write" {
				suffix = " (new)"
			}
			pathWidth := max(1, width-6)
			entry := icon + " " + truncateToWidth(fc.Path, pathWidth) + suffix
			lines = append(lines, entry)
		}
		if extra > 0 {
			lines = append(lines, fmt.Sprintf("  + %d more", extra))
		}
	}

	previewContent := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Width(max(0, width)).
		Height(max(0, height)).
		Render(previewContent)
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

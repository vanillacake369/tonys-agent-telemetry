package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
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

// SessionsTab implements TabModel for the Sessions tab.
type SessionsTab struct {
	sessions    []data.Session
	filtered    []data.Session
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

// Init loads sessions asynchronously.
func (s SessionsTab) Init() tea.Cmd {
	return func() tea.Msg {
		sessions, err := data.DiscoverSessions()
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

		case key.Matches(msg, s.keys.Enter):
			if len(s.filtered) > 0 && s.cursor < len(s.filtered) {
				session := s.filtered[s.cursor]
				cmd := fmt.Sprintf("cd %q && claude --resume %s", session.CWD, session.ID)
				_ = platform.Detect().OpenPane(cmd)
			}
			return s, nil

		case key.Matches(msg, s.keys.Fork):
			if len(s.filtered) > 0 && s.cursor < len(s.filtered) {
				session := s.filtered[s.cursor]
				cmd := fmt.Sprintf("cd %q && claude --resume %s --fork-session", session.CWD, session.ID)
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

// applyFilter applies fuzzy filtering to sessions based on current search input.
func (s *SessionsTab) applyFilter() {
	query := s.searchInput.Value()
	if query == "" {
		s.filtered = s.sessions
		return
	}

	// Build search targets combining FirstPrompt, CWD, GitBranch, and Model.
	targets := make([]string, len(s.sessions))
	for i, sess := range s.sessions {
		targets[i] = strings.Join([]string{
			sess.FirstPrompt,
			sess.CWD,
			sess.GitBranch,
			sess.Model,
		}, " ")
	}

	matches := fuzzy.Find(query, targets)
	filtered := make([]data.Session, 0, len(matches))
	for _, m := range matches {
		filtered = append(filtered, s.sessions[m.Index])
	}
	s.filtered = filtered
}

// loadPreviewCmd returns a tea.Cmd that loads the conversation preview for the
// currently selected session. Returns nil when there is no selected session.
func (s SessionsTab) loadPreviewCmd() tea.Cmd {
	if len(s.filtered) == 0 || s.cursor >= len(s.filtered) {
		return nil
	}
	filePath := s.filtered[s.cursor].FilePath
	return func() tea.Msg {
		turns, err := data.ParseConversationPreview(filePath, 5)
		return PreviewLoadedMsg{Turns: turns, Err: err}
	}
}

// loadFileChangesCmd returns a tea.Cmd that loads file changes for the
// currently selected session. Returns nil when there is no selected session.
func (s SessionsTab) loadFileChangesCmd() tea.Cmd {
	if len(s.filtered) == 0 || s.cursor >= len(s.filtered) {
		return nil
	}
	filePath := s.filtered[s.cursor].FilePath
	return func() tea.Msg {
		changes, err := data.ParseFileChanges(filePath)
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

// renderSessionListWithSearch renders a search input line followed by the session list.
// The search input is embedded at the top of the panel content area.
func (s SessionsTab) renderSessionListWithSearch(width, height int) string {
	searchLine := " " + s.searchInput.View()
	listHeight := max(1, height-1) // -1 for the search line
	listContent := s.renderSessionList(width, listHeight)
	return searchLine + "\n" + listContent
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

	var rows []string
	for i := scrollOffset; i < len(s.filtered) && i < scrollOffset+height; i++ {
		sess := s.filtered[i]
		// Account for the 3-char " ▸ " / "   " prefix in RenderListItem.
		line := s.formatSessionLine(sess, max(1, width-3))
		rows = append(rows, RenderListItem(line, i == s.cursor, width))
	}

	return strings.Join(rows, "\n")
}

// formatSessionLine formats a single session entry with responsive column widths.
// Uses lipgloss.MaxWidth for CJK-aware truncation (double-width chars = 2 cells).
// Narrow (<35): prompt only. Medium (<50): date + prompt. Wide: date + project + stats + prompt.
func (s SessionsTab) formatSessionLine(sess data.Session, maxWidth int) string {
	timestamp := sess.Timestamp.Format("01-02 15:04")
	project := filepath.Base(sess.CWD)
	if project == "" || project == "." {
		project = filepath.Base(sess.ProjectDir)
	}
	prompt := strings.ReplaceAll(sess.FirstPrompt, "\n", " ")

	trunc := func(s string, w int) string {
		return lipgloss.NewStyle().MaxWidth(w).Render(s)
	}

	if maxWidth < 35 {
		return trunc(prompt, maxWidth)
	}

	if maxWidth < 50 {
		dateStr := timestamp + " "
		remaining := max(1, maxWidth-12)
		return dateStr + trunc(prompt, remaining)
	}

	// Wide: date(12) + project(12) + stats(8) + prompt(remaining)
	dateCol := timestamp + " "
	projCol := trunc(project, 12)
	// Pad project to fixed width
	projPad := 12 - lipgloss.Width(projCol)
	if projPad > 0 {
		projCol += strings.Repeat(" ", projPad)
	}
	// Stats column: turn count and duration (e.g. "3t 4m ")
	statsCol := ""
	if sess.TurnCount > 0 || sess.Duration > 0 {
		minutes := int(sess.Duration.Minutes())
		statsCol = fmt.Sprintf("%dt %dm ", sess.TurnCount, minutes)
		// Pad stats to fixed width of 8
		statsPad := 8 - lipgloss.Width(statsCol)
		if statsPad > 0 {
			statsCol += strings.Repeat(" ", statsPad)
		} else if statsPad < 0 {
			statsCol = trunc(statsCol, 8)
		}
	} else {
		statsCol = strings.Repeat(" ", 8)
	}
	prefix := dateCol + projCol + " " + statsCol
	prefixWidth := lipgloss.Width(prefix)
	remaining := max(1, maxWidth-prefixWidth)
	return prefix + trunc(prompt, remaining)
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

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

// SessionsTab implements TabModel for the Sessions tab.
type SessionsTab struct {
	sessions    []data.Session
	filtered    []data.Session
	cursor      int
	searchInput textinput.Model
	preview     []data.Turn
	width       int
	height      int
	loading     bool
	err         error
	keys        KeyMap
}

// NewSessionsTab creates an initialised SessionsTab.
func NewSessionsTab() SessionsTab {
	ti := textinput.New()
	ti.Placeholder = "Search sessions..."
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 40

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
		return s, s.loadPreviewCmd()

	case PreviewLoadedMsg:
		if msg.Err == nil {
			s.preview = msg.Turns
		} else {
			s.preview = nil
		}
		return s, nil

	case tea.KeyMsg:
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
				cmd := fmt.Sprintf("claude --resume %s", session.ID)
				_ = platform.Detect().OpenPane(cmd)
			}
			return s, nil

		case key.Matches(msg, s.keys.ForkSession):
			if len(s.filtered) > 0 && s.cursor < len(s.filtered) {
				session := s.filtered[s.cursor]
				cmd := fmt.Sprintf("claude --resume %s --fork-session", session.ID)
				_ = platform.Detect().OpenPane(cmd)
			}
			return s, nil

		case key.Matches(msg, s.keys.CopyClip):
			if len(s.filtered) > 0 && s.cursor < len(s.filtered) {
				session := s.filtered[s.cursor]
				_ = platform.CopyToClipboard(session.ID)
			}
			return s, nil

		case msg.Type == tea.KeyUp || msg.String() == "k":
			if s.cursor > 0 {
				s.cursor--
				return s, s.loadPreviewCmd()
			}
			return s, nil

		case msg.Type == tea.KeyDown || msg.String() == "j":
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
				return s, s.loadPreviewCmd()
			}
			return s, nil
		}

		// Delegate remaining keys to the text input.
		var tiCmd tea.Cmd
		s.searchInput, tiCmd = s.searchInput.Update(msg)
		cmds = append(cmds, tiCmd)
		s.applyFilter()
		if s.cursor >= len(s.filtered) {
			s.cursor = max(0, len(s.filtered)-1)
		}
		cmds = append(cmds, s.loadPreviewCmd())
		return s, tea.Batch(cmds...)
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

// SetSize stores the terminal dimensions.
func (s SessionsTab) SetSize(width, height int) TabModel {
	s.width = width
	s.height = height
	s.searchInput.Width = width - 4
	return s
}

// View renders the sessions tab.
func (s SessionsTab) View() string {
	if s.loading {
		return lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true).
			Render("Loading sessions...")
	}

	if s.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF6B6B"}).
			Render(fmt.Sprintf("Error: %s", s.err))
	}

	// Calculate available heights.
	// search bar: 1 line + 1 border = 2
	// status bar: 1 line + 1 border = 2
	// content: remaining
	searchBarHeight := 3
	statusBarHeight := 2
	listHeight := s.height - searchBarHeight - statusBarHeight
	if listHeight < 1 {
		listHeight = 1
	}

	searchBar := s.renderSearchBar()
	mainContent := s.renderMainContent(listHeight)
	statusBar := s.renderSessionStatusBar()

	return strings.Join([]string{searchBar, mainContent, statusBar}, "\n")
}

// renderSearchBar renders the search input area.
func (s SessionsTab) renderSearchBar() string {
	searchStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(s.width - 2)

	return searchStyle.Render(s.searchInput.View())
}

// renderMainContent renders the split list/preview area.
func (s SessionsTab) renderMainContent(height int) string {
	if s.width == 0 {
		return s.renderSessionList(40, height)
	}

	leftWidth := s.width * 40 / 100
	rightWidth := s.width - leftWidth - 1 // -1 for separator

	left := s.renderSessionList(leftWidth, height)
	right := s.renderPreview(rightWidth, height)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// renderSessionList renders the left panel with the filtered sessions list.
func (s SessionsTab) renderSessionList(width, height int) string {
	listStyle := lipgloss.NewStyle().
		Width(width).
		Height(height)

	if len(s.filtered) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true).
			Render("No sessions found")
		return listStyle.Render(empty)
	}

	// Determine scroll window so the cursor is always visible.
	scrollOffset := s.cursor - height + 1
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	var rows []string
	for i := scrollOffset; i < len(s.filtered) && i < scrollOffset+height; i++ {
		sess := s.filtered[i]
		line := s.formatSessionLine(sess, width-2)

		if i == s.cursor {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Width(width).
				Render("> "+line))
		} else {
			rows = append(rows, lipgloss.NewStyle().
				Foreground(dimColor).
				Width(width).
				Render("  "+line))
		}
	}

	return listStyle.Render(strings.Join(rows, "\n"))
}

// formatSessionLine formats a single session entry for display.
// Format: MM-DD HH:MM │ project │ first-prompt-truncated
func (s SessionsTab) formatSessionLine(sess data.Session, maxWidth int) string {
	timestamp := sess.Timestamp.Format("01-02 15:04")
	project := filepath.Base(sess.CWD)
	if project == "" || project == "." {
		project = filepath.Base(sess.ProjectDir)
	}

	prefix := fmt.Sprintf("%s │ %s │ ", timestamp, project)
	remaining := maxWidth - len(prefix)
	if remaining < 1 {
		remaining = 1
	}

	prompt := sess.FirstPrompt
	if len([]rune(prompt)) > remaining {
		runes := []rune(prompt)
		prompt = string(runes[:remaining])
	}

	return prefix + prompt
}

// renderPreview renders the right panel with the conversation preview.
func (s SessionsTab) renderPreview(width, height int) string {
	previewStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(borderColor).
		PaddingLeft(1)

	if len(s.filtered) == 0 {
		return previewStyle.Render(
			lipgloss.NewStyle().
				Foreground(dimColor).
				Italic(true).
				Render("No session selected"),
		)
	}

	if len(s.preview) == 0 {
		return previewStyle.Render(
			lipgloss.NewStyle().
				Foreground(dimColor).
				Italic(true).
				Render("No preview available"),
		)
	}

	// Render turns.
	var lines []string
	for _, turn := range s.preview {
		roleStyle := lipgloss.NewStyle().Bold(true)
		var roleLabel string
		switch turn.Role {
		case "user":
			roleLabel = roleStyle.Foreground(primaryColor).Render("[User]")
		case "assistant":
			roleLabel = roleStyle.Foreground(dimColor).Render("[Asst]")
		default:
			roleLabel = roleStyle.Render("[" + turn.Role + "]")
		}

		// Wrap content to fit in width.
		contentWidth := width - 8 // account for border, padding, role label
		if contentWidth < 10 {
			contentWidth = 10
		}
		content := turn.Content
		if len([]rune(content)) > contentWidth {
			runes := []rune(content)
			content = string(runes[:contentWidth]) + "..."
		}
		// Replace newlines with space for single-line display.
		content = strings.ReplaceAll(content, "\n", " ")

		lines = append(lines, fmt.Sprintf("%s %s", roleLabel, content))
	}

	return previewStyle.Render(strings.Join(lines, "\n"))
}

// renderSessionStatusBar renders the key hint bar at the bottom.
func (s SessionsTab) renderSessionStatusBar() string {
	hints := "Enter:resume  ^F:fork  ^Y:copy  ^R:refresh"
	hintStyle := lipgloss.NewStyle().
		Foreground(dimColor).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(borderColor).
		Width(s.width - 2).
		Padding(0, 1)

	return hintStyle.Render(hints)
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

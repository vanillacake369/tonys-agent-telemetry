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
	ti.Placeholder = "Search sessions... (press / to focus)"
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

	case SearchFocusMsg:
		s.searchInput.Focus()
		return s, textinput.Blink

	case SearchBlurMsg:
		s.searchInput.Blur()
		return s, nil

	case tea.KeyMsg:
		// When search input is focused, forward keys to the text input.
		if s.searchInput.Focused() {
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
				return s, s.loadPreviewCmd()
			}
			return s, nil

		case key.Matches(msg, s.keys.Down):
			if s.cursor < len(s.filtered)-1 {
				s.cursor++
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

// SetSize stores the terminal dimensions.
func (s SessionsTab) SetSize(width, height int) TabModel {
	s.width = width
	s.height = height
	s.searchInput.Width = max(1, width-4)
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

	// Layout:
	//   search bar : 1 line
	//   gap        : 1 line
	//   list+preview: remaining height - 1 (hint line at bottom)
	//   hint bar   : 1 line
	const hintHeight = 1
	const searchHeight = 1
	const gapHeight = 1
	listHeight := max(1, s.height-searchHeight-gapHeight-hintHeight)

	searchBar := RenderSearchBar(s.searchInput, s.width, "", s.searchInput.Focused())
	leftW, rightW, showPreview := SplitLayout(s.width, 40)

	var splitView string
	if showPreview {
		leftContent := s.renderSessionList(max(1, leftW-2), max(1, listHeight-2))
		leftPanel := RenderPanel("Sessions", leftContent, leftW, listHeight, true)
		rightContent := s.renderPreview(max(1, rightW-2), max(1, listHeight-2))
		rightPanel := RenderPanel("Preview", rightContent, rightW, listHeight, false)
		splitView = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	} else {
		leftContent := s.renderSessionList(max(1, leftW-2), max(1, listHeight-2))
		splitView = RenderPanel("Sessions", leftContent, leftW, listHeight, true)
	}
	hintBar := RenderHintBar("↵:resume  f:fork  y:copy  r:refresh", s.width)

	return strings.Join([]string{searchBar, "", splitView, hintBar}, "\n")
}

// renderSessionList renders the left panel with the filtered sessions list.
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
		line := s.formatSessionLine(sess, max(1, width-2))
		rows = append(rows, RenderListItem(line, i == s.cursor, width))
	}

	return strings.Join(rows, "\n")
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

	prompt := truncateToWidth(sess.FirstPrompt, remaining)
	return prefix + prompt
}

// renderPreview renders the right panel with the conversation preview.
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
			roleLabel = roleStyle.Foreground(colorDim).Render("[Asst]")
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

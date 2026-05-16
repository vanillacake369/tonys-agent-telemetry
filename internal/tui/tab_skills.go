package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/platform"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/skill"
)

// SkillsSearchResultMsg is sent when a Search() call completes.
type SkillsSearchResultMsg struct {
	Skills []skill.Skill
	Err    error
}

// SkillReadmeMsg is sent when FetchReadme completes.
type SkillReadmeMsg struct {
	Content string
	Err     error
}

// LocalSkillsLoadedMsg is sent when the initial local scan completes.
type LocalSkillsLoadedMsg struct {
	Skills []skill.Skill
	Err    error
}

// skillsDebounceMsg is an internal tick used to trigger a debounced search.
type skillsDebounceMsg struct {
	query  string
	sortBy skill.SortBy
}

// SkillsTab implements TabModel for the Skills tab.
type SkillsTab struct {
	fetcher      *skill.Fetcher
	skills       []skill.Skill
	filtered     []skill.Skill
	cursor       int
	searchInput  textinput.Model
	preview      string
	sortBy       skill.SortBy
	width        int
	height       int
	loading      bool
	err          error
	cancelFn     context.CancelFunc
	lastKeystroke time.Time
	keys         KeyMap
}

// NewSkillsTab creates an initialised SkillsTab.
func NewSkillsTab() SkillsTab {
	ti := textinput.New()
	ti.Placeholder = "Search skills..."
	ti.Focus()
	ti.CharLimit = 200
	ti.Width = 40

	return SkillsTab{
		fetcher:     skill.NewFetcher(),
		searchInput: ti,
		loading:     true,
		keys:        DefaultKeyMap(),
	}
}

// Init scans local skills asynchronously.
func (t SkillsTab) Init() tea.Cmd {
	return func() tea.Msg {
		skills, err := skill.ScanLocal()
		return LocalSkillsLoadedMsg{Skills: skills, Err: err}
	}
}

// Update handles messages and key events.
func (t SkillsTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case LocalSkillsLoadedMsg:
		t.loading = false
		t.err = msg.Err
		t.skills = msg.Skills
		t.filtered = msg.Skills
		t.cursor = 0
		return t, t.loadReadmeCmd()

	case SkillsSearchResultMsg:
		t.loading = false
		if msg.Err != nil {
			t.err = msg.Err
		} else {
			t.skills = msg.Skills
			t.filtered = msg.Skills
			if t.cursor >= len(t.filtered) {
				t.cursor = max(0, len(t.filtered)-1)
			}
		}
		return t, t.loadReadmeCmd()

	case SkillReadmeMsg:
		if msg.Err == nil {
			t.preview = msg.Content
		} else {
			t.preview = ""
		}
		return t, nil

	case skillsDebounceMsg:
		// Only fire if the query and sort still match (user hasn't typed more).
		if msg.query == t.searchInput.Value() && msg.sortBy == t.sortBy {
			return t, t.searchCmd(msg.query, msg.sortBy)
		}
		return t, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, t.keys.Refresh):
			t.loading = true
			t.skills = nil
			t.filtered = nil
			t.preview = ""
			t.cursor = 0
			t.cancelInFlight()
			return t, t.Init()

		case key.Matches(msg, t.keys.Enter):
			if len(t.filtered) > 0 && t.cursor < len(t.filtered) {
				s := t.filtered[t.cursor]
				readmeContent := t.preview
				claudeCmd := buildClaudeAnalysisCmd(s, readmeContent)
				_ = platform.Detect().OpenPane(claudeCmd)
			}
			return t, nil

		case key.Matches(msg, t.keys.CopyClip):
			if len(t.filtered) > 0 && t.cursor < len(t.filtered) {
				_ = platform.CopyToClipboard(t.filtered[t.cursor].URL)
			}
			return t, nil

		case msg.Type == tea.KeyCtrlT:
			t.sortBy = nextSortBy(t.sortBy)
			t.cancelInFlight()
			return t, t.searchCmd(t.searchInput.Value(), t.sortBy)

		case msg.Type == tea.KeyUp || msg.String() == "k":
			if t.cursor > 0 {
				t.cursor--
				return t, t.loadReadmeCmd()
			}
			return t, nil

		case msg.Type == tea.KeyDown || msg.String() == "j":
			if t.cursor < len(t.filtered)-1 {
				t.cursor++
				return t, t.loadReadmeCmd()
			}
			return t, nil

		default:
			// Forward to search input.
			prevQuery := t.searchInput.Value()
			var inputCmd tea.Cmd
			t.searchInput, inputCmd = t.searchInput.Update(msg)
			cmds = append(cmds, inputCmd)

			if t.searchInput.Value() != prevQuery {
				t.lastKeystroke = time.Now()
				query := t.searchInput.Value()
				sortBy := t.sortBy
				// Debounce: schedule a search after 300ms.
				cmds = append(cmds, func() tea.Msg {
					time.Sleep(300 * time.Millisecond)
					return skillsDebounceMsg{query: query, sortBy: sortBy}
				})
			}
			return t, tea.Batch(cmds...)
		}
	}

	// Forward to textinput for cursor blink, etc.
	var inputCmd tea.Cmd
	t.searchInput, inputCmd = t.searchInput.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	return t, tea.Batch(cmds...)
}

// searchCmd fires an async Search and cancels any prior in-flight request.
func (t *SkillsTab) searchCmd(query string, sortBy skill.SortBy) tea.Cmd {
	t.cancelInFlight()
	ctx, cancel := context.WithCancel(context.Background())
	t.cancelFn = cancel
	t.loading = true

	fetcher := t.fetcher
	return func() tea.Msg {
		skills, err := fetcher.Search(ctx, query, sortBy)
		return SkillsSearchResultMsg{Skills: skills, Err: err}
	}
}

// cancelInFlight cancels any in-flight context.
func (t *SkillsTab) cancelInFlight() {
	if t.cancelFn != nil {
		t.cancelFn()
		t.cancelFn = nil
	}
}

// loadReadmeCmd fetches README for the currently selected skill.
func (t SkillsTab) loadReadmeCmd() tea.Cmd {
	if len(t.filtered) == 0 || t.cursor >= len(t.filtered) {
		return nil
	}
	s := t.filtered[t.cursor]
	if s.Source == skill.SourceLocal {
		// No remote readme for local skills.
		return func() tea.Msg {
			return SkillReadmeMsg{Content: fmt.Sprintf("Local skill: %s\n\n%s", s.Name, s.Description)}
		}
	}
	// GitHub skill — extract owner/repo from URL.
	repoFull := repoFullName(s.URL)
	if repoFull == "" {
		return nil
	}
	return func() tea.Msg {
		content, err := skill.FetchReadme(context.Background(), repoFull, 10240)
		return SkillReadmeMsg{Content: content, Err: err}
	}
}

// repoFullName extracts "owner/repo" from a GitHub URL.
func repoFullName(url string) string {
	url = strings.TrimRight(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}

// buildClaudeAnalysisCmd builds the claude -p "..." command string.
func buildClaudeAnalysisCmd(s skill.Skill, readmeContent string) string {
	prompt := fmt.Sprintf(
		"Analyze this skill for my workflow (tonys-nix, multi-provider agents, NixOS k8s):\n\nName: %s\nURL: %s\nDescription: %s\n\nREADME:\n%s\n\nTell me: benefits, trade-offs, and how to integrate it.",
		s.Name, s.URL, s.Description, readmeContent,
	)
	return fmt.Sprintf("claude -p %q", prompt)
}

// nextSortBy cycles through sort modes.
func nextSortBy(current skill.SortBy) skill.SortBy {
	switch current {
	case skill.SortByStars:
		return skill.SortByCreated
	case skill.SortByCreated:
		return skill.SortByUpdated
	default:
		return skill.SortByStars
	}
}

// sortByLabel returns the display label for a SortBy value.
func sortByLabel(s skill.SortBy) string {
	switch s {
	case skill.SortByCreated:
		return "Created"
	case skill.SortByUpdated:
		return "Updated"
	default:
		return "Stars"
	}
}

// sortByIcon returns the icon for a SortBy value.
func sortByIcon(s skill.SortBy) string {
	switch s {
	case skill.SortByCreated:
		return "🆕"
	case skill.SortByUpdated:
		return "🔄"
	default:
		return "⭐"
	}
}

// SetSize stores the terminal dimensions.
func (t SkillsTab) SetSize(width, height int) TabModel {
	t.width = width
	t.height = height
	t.searchInput.Width = width - 20
	return t
}

// View renders the skills tab.
func (t SkillsTab) View() string {
	if t.width == 0 || t.height == 0 {
		return "Skills\nLoading..."
	}

	searchBarHeight := 3
	hintBarHeight := 2
	listHeight := t.height - searchBarHeight - hintBarHeight
	if listHeight < 1 {
		listHeight = 1
	}

	searchBar := t.renderSearchBar()
	mainContent := t.renderMainContent(listHeight)
	hintBar := t.renderHintBar()

	return strings.Join([]string{searchBar, mainContent, hintBar}, "\n")
}

// renderSearchBar renders the search input with the current sort mode.
func (t SkillsTab) renderSearchBar() string {
	sortLabel := fmt.Sprintf("Sort: %s %s", sortByIcon(t.sortBy), sortByLabel(t.sortBy))
	sortStyle := lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
	sortRendered := sortStyle.Render(sortLabel)

	inputView := t.searchInput.View()

	// Pad input to fill the remaining width.
	rightWidth := lipgloss.Width(sortRendered) + 2
	inputWidth := t.width - rightWidth - 4
	if inputWidth < 10 {
		inputWidth = 10
	}

	row := lipgloss.JoinHorizontal(
		lipgloss.Center,
		lipgloss.NewStyle().Width(inputWidth).Render(inputView),
		lipgloss.NewStyle().Width(rightWidth).Align(lipgloss.Right).Render(sortRendered),
	)

	searchStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(t.width - 2)

	return searchStyle.Render(row)
}

// renderMainContent renders the split list/preview area.
func (t SkillsTab) renderMainContent(height int) string {
	leftWidth := t.width * 45 / 100
	rightWidth := t.width - leftWidth - 1

	left := t.renderSkillList(leftWidth, height)
	right := t.renderPreview(rightWidth, height)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// renderSkillList renders the left panel with the filtered skill list.
func (t SkillsTab) renderSkillList(width, height int) string {
	listStyle := lipgloss.NewStyle().
		Width(width).
		Height(height)

	if t.loading {
		return listStyle.Render(
			lipgloss.NewStyle().Foreground(dimColor).Italic(true).Render("Loading skills..."),
		)
	}

	if len(t.filtered) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(dimColor).
			Italic(true).
			Render("No skills found")
		return listStyle.Render(empty)
	}

	scrollOffset := t.cursor - height + 1
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	selectedStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor)
	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#333333", Dark: "#CCCCCC"})

	var rows []string
	for i := scrollOffset; i < len(t.filtered) && i < scrollOffset+height; i++ {
		s := t.filtered[i]
		line := formatSkillLine(s, width-3)

		if i == t.cursor {
			rows = append(rows, lipgloss.NewStyle().
				Width(width).
				Render("> "+selectedStyle.Render(line)))
		} else {
			rows = append(rows, lipgloss.NewStyle().
				Width(width).
				Render("  "+normalStyle.Render(line)))
		}
	}

	return listStyle.Render(strings.Join(rows, "\n"))
}

// formatSkillLine formats a single skill entry for display.
func formatSkillLine(s skill.Skill, maxWidth int) string {
	var icon, meta string
	switch s.Source {
	case skill.SourceLocal:
		icon = "📁"
		meta = "local"
	default:
		icon = "📦"
		meta = fmt.Sprintf("⭐%d", s.Stars)
	}

	suffix := fmt.Sprintf("  %s", meta)
	prefix := fmt.Sprintf("%s %s", icon, s.Name)

	available := maxWidth - len([]rune(prefix)) - len([]rune(suffix))
	if available < 0 {
		// Truncate name.
		nameMax := maxWidth - len([]rune(icon)) - len([]rune(suffix)) - 2
		if nameMax < 1 {
			nameMax = 1
		}
		runes := []rune(s.Name)
		if len(runes) > nameMax {
			runes = runes[:nameMax]
		}
		prefix = fmt.Sprintf("%s %s", icon, string(runes))
	}

	return prefix + suffix
}

// renderPreview renders the right preview panel.
func (t SkillsTab) renderPreview(width, height int) string {
	previewStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(borderColor).
		PaddingLeft(1)

	if len(t.filtered) == 0 {
		return previewStyle.Render(
			lipgloss.NewStyle().
				Foreground(dimColor).
				Italic(true).
				Render("No skill selected"),
		)
	}

	if t.preview == "" {
		return previewStyle.Render(
			lipgloss.NewStyle().
				Foreground(dimColor).
				Italic(true).
				Render("Loading preview..."),
		)
	}

	// Wrap/truncate to fit the pane.
	contentWidth := width - 4
	if contentWidth < 10 {
		contentWidth = 10
	}
	lines := strings.Split(t.preview, "\n")
	var displayLines []string
	for _, line := range lines {
		if len([]rune(line)) > contentWidth {
			runes := []rune(line)
			line = string(runes[:contentWidth])
		}
		displayLines = append(displayLines, line)
		if len(displayLines) >= height-2 {
			break
		}
	}

	return previewStyle.Render(strings.Join(displayLines, "\n"))
}

// renderHintBar renders the key hint bar at the bottom.
func (t SkillsTab) renderHintBar() string {
	hints := "Enter:analyze  ^T:sort  ^Y:copy  ^R:refresh"
	hintStyle := lipgloss.NewStyle().
		Foreground(dimColor).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(borderColor).
		Width(t.width - 2).
		Padding(0, 1)

	return hintStyle.Render(hints)
}

package tui

import (
	"context"
	"fmt"
	"sort"
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

// GitHubSkillsLoadedMsg is sent when the async GitHub fetch completes.
type GitHubSkillsLoadedMsg struct {
	Skills []skill.Skill
	Query  string // to verify it matches current query
	Err    error
}

// skillsDebounceMsg is an internal tick used to trigger a debounced search.
type skillsDebounceMsg struct {
	query  string
	sortBy skill.SortBy
}

// skillsGitHubDebounceMsg is an internal tick used to trigger a debounced GitHub search.
type skillsGitHubDebounceMsg struct {
	query  string
	sortBy skill.SortBy
}

// SkillsTab implements TabModel for the Skills tab.
type SkillsTab struct {
	fetcher       *skill.Fetcher
	localSkills   []skill.Skill // always available, loaded at init
	githubSkills  []skill.Skill // fetched async on query >= 3 chars
	skills        []skill.Skill // merged: local + github
	filtered      []skill.Skill // after fuzzy filter
	cursor        int
	searchInput   textinput.Model
	preview       string
	sortBy        skill.SortBy
	width         int
	height        int
	loading       bool // initial local load
	githubLoading bool // github fetch in progress
	err           error
	cancelFn      context.CancelFunc
	lastKeystroke time.Time
	keys          KeyMap
}

// NewSkillsTab creates an initialised SkillsTab.
func NewSkillsTab() SkillsTab {
	ti := textinput.New()
	ti.Placeholder = "/ to search"
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
		t.localSkills = msg.Skills
		t.skills = msg.Skills
		t.filtered = msg.Skills
		t.cursor = 0
		return t, t.loadReadmeCmd()

	case GitHubSkillsLoadedMsg:
		t.githubLoading = false
		// Ignore stale results that no longer match the current query.
		if msg.Query != t.searchInput.Value() {
			return t, nil
		}
		if msg.Err == nil {
			t.githubSkills = msg.Skills
			// Merge: local (fuzzy filtered) + github results.
			t.skills = mergeLocalAndGitHub(t.localSkills, msg.Skills, t.sortBy)
			t.applyFilter()
		}
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
		// Legacy debounce: only fire if the query and sort still match.
		if msg.query == t.searchInput.Value() && msg.sortBy == t.sortBy {
			return t, t.searchCmd(msg.query, msg.sortBy)
		}
		return t, nil

	case skillsGitHubDebounceMsg:
		// Only fire GitHub search if the query and sort still match.
		if msg.query == t.searchInput.Value() && msg.sortBy == t.sortBy {
			t.githubLoading = true
			return t, t.fetchGitHubCmd(msg.query, msg.sortBy)
		}
		return t, nil

	case SearchFocusMsg:
		t.searchInput.Focus()
		return t, textinput.Blink

	case SearchBlurMsg:
		t.searchInput.Blur()
		return t, nil

	case tea.KeyMsg:
		// Block Enter/newline in search mode — prevent layout breakage.
		if t.searchInput.Focused() && (msg.Type == tea.KeyEnter) {
			return t, nil
		}

		// When search input is focused, forward keys to the text input.
		if t.searchInput.Focused() {
			prevQuery := t.searchInput.Value()
			var inputCmd tea.Cmd
			t.searchInput, inputCmd = t.searchInput.Update(msg)
			cmds = append(cmds, inputCmd)

			// Strip any newlines that may have been pasted.
			if strings.Contains(t.searchInput.Value(), "\n") {
				t.searchInput.SetValue(strings.ReplaceAll(t.searchInput.Value(), "\n", " "))
			}

			if t.searchInput.Value() != prevQuery {
				t.lastKeystroke = time.Now()
				query := t.searchInput.Value()
				sortBy := t.sortBy

				// Show local results immediately with fuzzy filter.
				t.applyFilter()

				// Schedule GitHub search in background (only for queries >= 3 chars).
				if len(query) >= 3 {
					cmds = append(cmds, t.scheduleGitHubSearch(query, sortBy))
				} else {
					// Query too short — clear GitHub results, show only local.
					t.githubSkills = nil
					t.githubLoading = false
					t.skills = t.localSkills
					t.applyFilter()
				}
			}
			return t, tea.Batch(cmds...)
		}

		// Navigation mode: handle single-key bindings.
		switch {
		case key.Matches(msg, t.keys.Refresh):
			t.loading = true
			t.localSkills = nil
			t.githubSkills = nil
			t.skills = nil
			t.filtered = nil
			t.preview = ""
			t.cursor = 0
			t.githubLoading = false
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

		case key.Matches(msg, t.keys.Open):
			if len(t.filtered) > 0 && t.cursor < len(t.filtered) {
				s := t.filtered[t.cursor]
				if s.URL != "" {
					_ = platform.OpenInBrowser(s.URL)
				}
			}
			return t, nil

		case key.Matches(msg, t.keys.Copy):
			if len(t.filtered) > 0 && t.cursor < len(t.filtered) {
				_ = platform.CopyToClipboard(t.filtered[t.cursor].URL)
			}
			return t, nil

		case key.Matches(msg, t.keys.Sort):
			t.sortBy = nextSortBy(t.sortBy)
			// Re-sort existing results locally (no new fetch).
			t.skills = mergeLocalAndGitHub(t.localSkills, t.githubSkills, t.sortBy)
			t.applyFilter()
			return t, nil

		case key.Matches(msg, t.keys.Up):
			if t.cursor > 0 {
				t.cursor--
				return t, t.loadReadmeCmd()
			}
			return t, nil

		case key.Matches(msg, t.keys.Down):
			if t.cursor < len(t.filtered)-1 {
				t.cursor++
				return t, t.loadReadmeCmd()
			}
			return t, nil
		}

		return t, nil
	}

	// Forward to textinput for cursor blink, etc.
	var inputCmd tea.Cmd
	t.searchInput, inputCmd = t.searchInput.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	return t, tea.Batch(cmds...)
}

// scheduleGitHubSearch schedules a GitHub search with a 500ms debounce.
func (t *SkillsTab) scheduleGitHubSearch(query string, sortBy skill.SortBy) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(500 * time.Millisecond)
		return skillsGitHubDebounceMsg{query: query, sortBy: sortBy}
	}
}

// fetchGitHubCmd fires an async GitHub search and returns a GitHubSkillsLoadedMsg.
func (t *SkillsTab) fetchGitHubCmd(query string, sortBy skill.SortBy) tea.Cmd {
	t.cancelInFlight()
	ctx, cancel := context.WithCancel(context.Background())
	t.cancelFn = cancel

	fetcher := t.fetcher
	return func() tea.Msg {
		skills, err := fetcher.SearchRemote(ctx, query, sortBy)
		return GitHubSkillsLoadedMsg{Skills: skills, Query: query, Err: err}
	}
}

// applyFilter filters local skills by query substring match.
// Remote skills (GitHub/npm/awesome) are always included — they were already
// matched by the API's own search, so re-filtering them drops valid results.
func (t *SkillsTab) applyFilter() {
	query := strings.ToLower(t.searchInput.Value())
	if query == "" {
		t.filtered = t.skills
		if t.cursor >= len(t.filtered) {
			t.cursor = max(0, len(t.filtered)-1)
		}
		return
	}

	filtered := make([]skill.Skill, 0, len(t.skills))
	for _, s := range t.skills {
		// Remote results: always include (API already matched them).
		if s.Source != skill.SourceLocal {
			filtered = append(filtered, s)
			continue
		}
		// Local results: substring match on name + description.
		nameLower := strings.ToLower(s.Name)
		descLower := strings.ToLower(s.Description)
		if strings.Contains(nameLower, query) || strings.Contains(descLower, query) {
			filtered = append(filtered, s)
		}
	}
	t.filtered = filtered
	if t.cursor >= len(t.filtered) {
		t.cursor = max(0, len(t.filtered)-1)
	}
}

// mergeLocalAndGitHub combines local and remote skills. Local first, then remote sorted by sortBy.
func mergeLocalAndGitHub(local, remote []skill.Skill, sortBy skill.SortBy) []skill.Skill {
	// Sort remote results by the chosen criterion.
	sorted := make([]skill.Skill, len(remote))
	copy(sorted, remote)
	sort.SliceStable(sorted, func(i, j int) bool {
		switch sortBy {
		case skill.SortByCreated:
			return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
		case skill.SortByUpdated:
			return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
		default:
			return sorted[i].Stars > sorted[j].Stars
		}
	})

	result := make([]skill.Skill, 0, len(local)+len(sorted))
	result = append(result, local...)
	result = append(result, sorted...)
	return result
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
	leftW, _, _ := SplitLayout(width, 45)
	t.searchInput.Width = max(1, leftW-20)
	return t
}

// View renders the skills tab.
func (t SkillsTab) View() string {
	if t.width == 0 || t.height == 0 {
		if t.loading {
			return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("Loading skills...")
		}
		return "Skills"
	}

	// Layout: search input is embedded inside the Skills panel (1 line).
	// The full height is used for the list+preview panels.
	listHeight := max(3, t.height)

	leftW, rightW, showPreview := SplitLayout(t.width, 45)

	var splitView string
	if showPreview {
		leftContent := t.renderSkillListWithSearch(max(1, leftW-2), max(1, listHeight-2))
		leftPanel := RenderPanel("Skills", leftContent, leftW, listHeight, true)
		rightContent := t.renderPreview(max(1, rightW-2), max(1, listHeight-2))
		rightPanel := RenderPanel("Preview", rightContent, rightW, listHeight, false)
		splitView = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	} else {
		leftContent := t.renderSkillListWithSearch(max(1, leftW-2), max(1, listHeight-2))
		splitView = RenderPanel("Skills", leftContent, leftW, listHeight, true)
	}

	return splitView
}

// renderSkillListWithSearch renders a search input (with sort label) followed by the skill list.
// The search input is embedded at the top of the panel content area.
func (t SkillsTab) renderSkillListWithSearch(width, height int) string {
	sortLabel := fmt.Sprintf("Sort: %s %s", sortByIcon(t.sortBy), sortByLabel(t.sortBy))
	sortRendered := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(sortLabel)
	sortWidth := lipgloss.Width(sortRendered) + 2
	inputWidth := max(1, width-sortWidth-2)
	searchLine := lipgloss.JoinHorizontal(
		lipgloss.Center,
		lipgloss.NewStyle().Width(inputWidth).Render(" "+t.searchInput.View()),
		lipgloss.NewStyle().Width(sortWidth).Align(lipgloss.Right).Render(sortRendered),
	)
	listHeight := max(1, height-1) // -1 for the search line
	listContent := t.renderSkillList(width, listHeight)
	return searchLine + "\n" + listContent
}

// renderSkillList renders the left panel with the filtered skill list.
func (t SkillsTab) renderSkillList(width, height int) string {
	if t.loading {
		return RenderEmptyState("Loading skills...", width, height)
	}

	var rows []string

	// Show the filtered skill items.
	if len(t.filtered) > 0 {
		scrollOffset := t.cursor - height + 1
		if scrollOffset < 0 {
			scrollOffset = 0
		}

		for i := scrollOffset; i < len(t.filtered) && i < scrollOffset+height; i++ {
			s := t.filtered[i]
			line := formatSkillLine(s, max(1, width-3))
			rows = append(rows, RenderListItem(line, i == t.cursor, width))
		}
	}

	// Show GitHub loading indicator if a fetch is in progress.
	if t.githubLoading && len(rows) < height {
		loadingLine := lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("── GitHub results loading... ──")
		rows = append(rows, loadingLine)
	}

	if len(rows) == 0 {
		return RenderEmptyState("No skills found", width, height)
	}

	return strings.Join(rows, "\n")
}

// formatSkillLine formats a single skill entry for display.
func formatSkillLine(s skill.Skill, maxWidth int) string {
	var icon, meta string
	switch s.Source {
	case skill.SourceLocal:
		icon = "📁"
		meta = "local"
	case skill.SourceGitHub:
		icon = "📦"
		meta = fmt.Sprintf("⭐%d", s.Stars)
	case skill.SourceNPM:
		icon = "📦"
		meta = fmt.Sprintf("npm %d", s.Stars)
	case skill.SourceAwesome:
		icon = "📋"
		meta = "awesome"
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

// renderPreview renders the right preview panel content.
func (t SkillsTab) renderPreview(width, height int) string {
	if len(t.filtered) == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("No skill selected")
	}

	if t.preview == "" {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("Loading preview...")
	}

	contentWidth := max(10, width-2)
	content := wrapLines(t.preview, contentWidth, max(1, height))

	return lipgloss.NewStyle().
		Width(max(0, width)).
		Height(max(0, height)).
		Render(content)
}

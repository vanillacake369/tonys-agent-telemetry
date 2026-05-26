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
	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/platform"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/recommender"
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

// CatalogLoadedMsg is sent when a catalog fetch (from cache or network) completes.
// FetchedAt records the time the catalog data was last fetched from upstream;
// it is used to determine staleness relative to catalog.DefaultTTL.
type CatalogLoadedMsg struct {
	Items     []catalog.Item
	FetchedAt time.Time
	Err       error
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
	cancelFn        context.CancelFunc
	lastKeystroke   time.Time
	lastPreviewURL  string // dedup: skip if same URL already fetched/fetching
	previewCancelFn context.CancelFunc
	keys            KeyMap
	wizard          AnalyzeWizard

	// catalog pane state
	catalogItems     []catalog.Item // nil = loading state
	catalogFetchedAt time.Time      // zero = not yet fetched
	catalogErr       error

	// Advisor pane state — populated by RecommendationsReadyMsg routed from App.
	recommendations []recommender.Recommendation
	// pipelineRan flips to true after the first RecommendationsReadyMsg arrives.
	// It distinguishes "no spans ingested yet" (false) from "pipeline ran but
	// produced zero matches" (true, empty recs) so the Advisor empty state can
	// give the user actionable guidance (QA finding U-2).
	pipelineRan bool
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
		wizard:      NewAnalyzeWizard(),
	}
}

// Init scans local skills asynchronously and triggers a catalog load.
func (t SkillsTab) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			skills, err := skill.ScanLocal()
			return LocalSkillsLoadedMsg{Skills: skills, Err: err}
		},
		catalogLoadCmd(),
	)
}

// catalogLoadCmd returns a tea.Cmd that reads the catalog from cache first,
// then triggers an HTTP fetch in the background if the cache is stale.
// DRY: the Skills tab and any future caller both use this path — no inline HTTP.
func catalogLoadCmd() tea.Cmd {
	return func() tea.Msg {
		c := &catalog.Cache{Path: catalog.ResolveCachePath()}

		// Try cache first.
		items, mtime, err := c.Read()
		if err == nil {
			// Cache hit: return cached items; initiate background refresh if stale.
			if c.IsStale(catalog.DefaultTTL) {
				// Stale but still usable: return cached data immediately,
				// then trigger a background fetch via a second command.
				// For simplicity in this pass, we return the stale data and
				// mark FetchedAt as the cache mtime. The "(stale)" prefix in
				// View handles the user-visible indicator.
				return CatalogLoadedMsg{Items: items, FetchedAt: mtime}
			}
			return CatalogLoadedMsg{Items: items, FetchedAt: mtime}
		}

		// Cache miss: try network fetch.
		// Belt-and-suspenders: even though HTTPFetcher sets a client-level
		// timeout, also bound the context so a stuck dial cannot hang
		// "Loading catalog…" indefinitely (QA finding R-2).
		f := catalog.NewHTTPFetcher()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fetched, fetchErr := f.Fetch(ctx)
		if fetchErr != nil {
			return CatalogLoadedMsg{Err: fetchErr, FetchedAt: time.Now()}
		}

		// Persist to cache (best-effort; ignore write errors).
		_ = c.Write(fetched)
		return CatalogLoadedMsg{Items: fetched, FetchedAt: time.Now()}
	}
}

// Update handles messages and key events.
func (t SkillsTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case AnalyzeExecuteMsg:
		cmd, err := buildAnalyzeCmd(msg.Model, msg.Prompt)
		if err != nil {
			t.err = err
			t.wizard.visible = false
			return t, nil
		}
		t.wizard.visible = false
		return t, tea.ExecProcess(cmd, func(execErr error) tea.Msg {
			return AnalyzeFinishedMsg{Err: execErr}
		})

	case AnalyzeFinishedMsg:
		t.err = msg.Err
		return t, nil

	case LocalSkillsLoadedMsg:
		t.loading = false
		t.err = msg.Err
		t.localSkills = msg.Skills
		// If user has an active search with GitHub results, preserve merged view.
		if len(t.githubSkills) > 0 {
			t.skills = mergeLocalAndGitHub(msg.Skills, t.githubSkills, t.sortBy)
			t.applyFilter()
		} else {
			t.skills = msg.Skills
			t.filtered = msg.Skills
			t.cursor = 0
		}
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

	case CatalogLoadedMsg:
		t.catalogErr = msg.Err
		if msg.Err == nil {
			t.catalogItems = msg.Items
		}
		t.catalogFetchedAt = msg.FetchedAt
		return t, nil

	case RecommendationsReadyMsg:
		// Replace (not append) — each pipeline run supersedes the previous result.
		t.recommendations = msg.Recommendations
		// Once a pipeline result arrives (even with zero recs), we know spans
		// existed and signals were extracted. Flip the flag so the Advisor
		// empty-state switches from "ingest sessions" to "no matches yet".
		t.pipelineRan = true
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
		// When the wizard overlay is visible, delegate all keys to it.
		if t.wizard.visible {
			var wizardCmd tea.Cmd
			t.wizard, wizardCmd = t.wizard.Update(msg)
			if wizardCmd != nil {
				cmds = append(cmds, wizardCmd)
			}
			return t, tea.Batch(cmds...)
		}

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
				t.wizard.Show(s, t.preview)
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
// Deduplicates: skips if the same URL is already being fetched.
// Cancels previous fetch if a new one starts.
func (t *SkillsTab) loadReadmeCmd() tea.Cmd {
	if len(t.filtered) == 0 || t.cursor >= len(t.filtered) {
		return nil
	}
	s := t.filtered[t.cursor]

	// Dedup: don't re-fetch same URL.
	if s.URL == t.lastPreviewURL && t.preview != "" {
		return nil
	}
	t.lastPreviewURL = s.URL

	// Cancel previous preview fetch.
	if t.previewCancelFn != nil {
		t.previewCancelFn()
	}

	if s.Source == skill.SourceLocal {
		return func() tea.Msg {
			return SkillReadmeMsg{Content: fmt.Sprintf("Local skill: %s\n\n%s", s.Name, s.Description)}
		}
	}

	repoFull := repoFullName(s.URL)
	if repoFull == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.previewCancelFn = cancel

	return func() tea.Msg {
		content, err := skill.FetchReadme(ctx, repoFull, 10240)
		return SkillReadmeMsg{Content: content, Err: err}
	}
}

// repoFullName extracts "owner/repo" from a GitHub URL.
func repoFullName(u string) string {
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, ".git")
	parts := strings.Split(u, "/")
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
	t.wizard.width = width
	t.wizard.height = height
	return t
}

// catalogMaxItems is the maximum number of catalog entries shown in the list view.
const catalogMaxItems = 20

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

	// Render the wizard overlay centered on top when it is visible.
	if t.wizard.visible {
		overlay := t.wizard.View()
		return lipgloss.Place(
			t.width, t.height,
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceForeground(colorDim),
		)
	}

	catalogSection := t.renderCatalogSection(t.width)
	advisorSection := renderAdvisorSection(t.recommendations, t.width, t.pipelineRan)
	return splitView + "\n" + catalogSection + "\n" + advisorSection
}

// CatalogItems returns the currently loaded catalog items.
// This getter is required by AdvisorPipeline (ι-2) to pass the catalog to
// MaybeRun without reaching into SkillsTab internals from app.go.
func (t SkillsTab) CatalogItems() []catalog.Item {
	return t.catalogItems
}

// LocalSkillNames returns the name of each locally-installed skill.
// This is the SSoT read point for InstalledSkills: app.go reads it here
// once (in the SpanBatchMsg routing) and passes it to AdvisorPipeline.MaybeRun.
// DRY: do not read localSkills anywhere else from app.go.
func (t SkillsTab) LocalSkillNames() []string {
	if len(t.localSkills) == 0 {
		return nil
	}
	names := make([]string, len(t.localSkills))
	for i, s := range t.localSkills {
		names[i] = s.Name
	}
	return names
}

// renderCatalogSection renders the best-practice catalog pane below the skills list.
// The section always ends with catalog.Attribution to satisfy the GA gate requirement.
func (t SkillsTab) renderCatalogSection(width int) string {
	sep := lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("─", min(width, 80)))
	attributionLine := lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render(catalog.Attribution)

	var body string

	// Loading state: catalogItems == nil means no message received yet.
	if t.catalogItems == nil && t.catalogErr == nil {
		body = lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("Loading catalog...")
		return strings.Join([]string{sep, body, attributionLine}, "\n")
	}

	// Error state.
	if t.catalogErr != nil {
		body = lipgloss.NewStyle().Foreground(colorDim).Italic(true).
			Render("Catalog unavailable: " + t.catalogErr.Error())
		return strings.Join([]string{sep, body, attributionLine}, "\n")
	}

	n := len(t.catalogItems)
	minViable := catalog.ResolveMinViable("")

	// Below-minimum warning: show partial warning, no item list.
	if n < minViable {
		warning := lipgloss.NewStyle().Foreground(colorDim).
			Render(fmt.Sprintf("Catalog partial (%d/%d) — refresh required", n, minViable))
		return strings.Join([]string{sep, warning, attributionLine}, "\n")
	}

	// Title row with staleness indicator.
	age := time.Since(t.catalogFetchedAt)
	stale := age >= catalog.DefaultTTL
	ageStr := formatAge(age)

	titleText := fmt.Sprintf("Best-practice catalog (%d items, fetched %s ago)", n, ageStr)
	if stale {
		titleText = "(stale) " + titleText
	}
	titleLine := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(titleText)

	// Item list: up to catalogMaxItems entries.
	var rows []string
	limit := n
	if limit > catalogMaxItems {
		limit = catalogMaxItems
	}
	for _, item := range t.catalogItems[:limit] {
		icon := catalogTypeIcon(item.Type)
		tagsStr := strings.Join(item.Tags, ", ")
		line := fmt.Sprintf("[%s] %s", icon, item.Title)
		if tagsStr != "" {
			line += fmt.Sprintf("  tags: %s", tagsStr)
		}
		rows = append(rows, lipgloss.NewStyle().Foreground(colorText).Render(line))
	}

	parts := []string{sep, titleLine}
	parts = append(parts, rows...)
	parts = append(parts, attributionLine)
	return strings.Join(parts, "\n")
}

// catalogTypeIcon returns a short text icon for a catalog ItemType.
func catalogTypeIcon(t catalog.ItemType) string {
	switch t {
	case catalog.ItemTypeSkill:
		return "skill"
	case catalog.ItemTypeTemplate:
		return "tmpl"
	case catalog.ItemTypeAgent:
		return "agent"
	case catalog.ItemTypeHook:
		return "hook"
	default:
		return "?"
	}
}

// formatAge formats a duration as a human-readable age string.
func formatAge(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// renderSkillListWithSearch renders a search input (with sort label) followed by the skill list.
// The search input is embedded at the top of the panel content area.
// Column widths in the header must match those used in formatSkillLine (SSoT).
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
	headerStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	dimSep := lipgloss.NewStyle().Foreground(colorDim)

	// Wide-mode column widths: same as formatSkillLine (SSoT).
	srcH := PadToWidth("SRC", 4)
	nameH := PadToWidth("NAME", 22)
	metaH := "META"
	headerLine := headerStyle.Render("   " + srcH + nameH + metaH)
	sepLine := dimSep.Render(strings.Repeat("─", min(width, 60)))

	listHeight := max(1, height-3) // search + header + sep
	listContent := t.renderSkillList(width, listHeight)
	return searchLine + "\n" + headerLine + "\n" + sepLine + "\n" + listContent
}

// renderSkillList renders the left panel with the filtered skill list.
func (t SkillsTab) renderSkillList(width, height int) string {
	if t.loading {
		return RenderEmptyState("Loading skills...", width, height)
	}

	query := t.searchInput.Value()
	baseStyle := lipgloss.NewStyle().Foreground(colorText)

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
			if query != "" {
				line = HighlightMatch(line, query, baseStyle)
			}
			rows = append(rows, RenderListItem(line, i == t.cursor, width))
		}
	}

	// Show GitHub loading indicator if a fetch is in progress.
	if t.githubLoading && len(rows) < height {
		loadingLine := lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("── Searching GitHub, registries, npm... ──")
		rows = append(rows, loadingLine)
	}

	if len(rows) == 0 {
		if t.githubLoading {
			return RenderLoadingState(width, height)
		}
		return RenderEmptyState("No skills found", width, height)
	}

	// Show result count at the bottom if there's room.
	if len(t.filtered) > height && len(rows) < height {
		countLine := lipgloss.NewStyle().Foreground(colorDim).
			Render(fmt.Sprintf("── %d results ──", len(t.filtered)))
		rows = append(rows, countLine)
	}

	return strings.Join(rows, "\n")
}

// formatSkillLine formats a single skill entry for display using PadToWidth for
// CJK-safe column alignment. Column widths mirror those in renderSkillListWithSearch (SSoT).
// src(4) + name(22) + meta(remaining).
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

	// src column: icon + space = 4 display cells
	srcCol := PadToWidth(icon, 4)
	// name column: 22 display cells (truncated if needed)
	nameTrunc := lipgloss.NewStyle().MaxWidth(20).Render(s.Name)
	nameCol := PadToWidth(nameTrunc, 22)
	// meta: fills the remainder
	metaCol := fmt.Sprintf("  %s", meta)

	line := srcCol + nameCol + metaCol
	if lipgloss.Width(line) > maxWidth {
		return lipgloss.NewStyle().MaxWidth(maxWidth).Render(line)
	}
	return line
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

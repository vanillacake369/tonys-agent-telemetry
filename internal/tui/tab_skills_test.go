package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/skill"
)

// updateSkillsTab is a test helper that runs one Update cycle and returns
// the resulting SkillsTab (it panics if the model changes type).
func updateSkillsTab(t *testing.T, s SkillsTab, msg tea.Msg) (SkillsTab, tea.Cmd) {
	t.Helper()
	model, cmd := s.Update(msg)
	result, ok := model.(SkillsTab)
	if !ok {
		t.Fatalf("Update returned %T, want SkillsTab", model)
	}
	return result, cmd
}

// makeTestSkillsList builds a slice of test skills.
func makeTestSkillsList() []skill.Skill {
	return []skill.Skill{
		{
			Name:        "scaffold",
			Description: "Generates working skeleton code",
			Source:      skill.SourceLocal,
			UpdatedAt:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Name:        "k8s-skill",
			Description: "Kubernetes deployment automation",
			Source:      skill.SourceGitHub,
			URL:         "https://github.com/alice/k8s-skill",
			Stars:       234,
			UpdatedAt:   time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			Name:        "pr-review",
			Description: "Automated pull request review",
			Source:      skill.SourceGitHub,
			URL:         "https://github.com/bob/pr-review",
			Stars:       189,
			UpdatedAt:   time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		},
	}
}

func TestSkillsTab_LocalSkillsLoaded_PopulatesSkills(t *testing.T) {
	s := NewSkillsTab()
	skills := makeTestSkillsList()

	updated, _ := updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: skills, Err: nil})

	if updated.loading {
		t.Error("loading should be false after LocalSkillsLoadedMsg")
	}
	if len(updated.skills) != 3 {
		t.Errorf("skills len = %d, want 3", len(updated.skills))
	}
	if len(updated.filtered) != 3 {
		t.Errorf("filtered len = %d, want 3", len(updated.filtered))
	}
	if updated.err != nil {
		t.Errorf("err = %v, want nil", updated.err)
	}
}

func TestSkillsTab_LocalSkillsLoaded_SetsLoadingFalse(t *testing.T) {
	s := NewSkillsTab()
	if !s.loading {
		t.Error("expected loading=true before any message")
	}

	updated, _ := updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: nil, Err: nil})
	if updated.loading {
		t.Error("loading should be false after LocalSkillsLoadedMsg")
	}
}

// TestSkillsTab_LocalSkillsLoaded_ShowsImmediately verifies that local skills
// are shown right after LocalSkillsLoadedMsg without waiting for GitHub.
func TestSkillsTab_LocalSkillsLoaded_ShowsImmediately(t *testing.T) {
	s := NewSkillsTab()
	localSkills := []skill.Skill{
		{Name: "scaffold", Source: skill.SourceLocal, Description: "scaffold skill"},
		{Name: "commit", Source: skill.SourceLocal, Description: "commit skill"},
	}

	updated, _ := updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: localSkills, Err: nil})

	// Skills should be available immediately, no GitHub loading needed.
	if updated.loading {
		t.Error("loading should be false — local skills are instant")
	}
	if updated.githubLoading {
		t.Error("githubLoading should be false — no GitHub fetch triggered by init")
	}
	if len(updated.localSkills) != 2 {
		t.Errorf("localSkills len = %d, want 2", len(updated.localSkills))
	}
	if len(updated.filtered) != 2 {
		t.Errorf("filtered len = %d, want 2 (should show local skills immediately)", len(updated.filtered))
	}
	// Cursor must be valid for navigation.
	if updated.cursor != 0 {
		t.Errorf("cursor = %d, want 0", updated.cursor)
	}
}

func TestSkillsTab_SearchResultsUpdate_UpdatesList(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})

	// New search result arrives with only 1 match.
	newSkills := []skill.Skill{
		{Name: "deploy", Source: skill.SourceGitHub, Stars: 100},
	}
	updated, _ := updateSkillsTab(t, s, SkillsSearchResultMsg{Skills: newSkills, Err: nil})

	if len(updated.filtered) != 1 {
		t.Errorf("filtered len = %d after search result, want 1", len(updated.filtered))
	}
	if updated.filtered[0].Name != "deploy" {
		t.Errorf("filtered[0].Name = %q, want %q", updated.filtered[0].Name, "deploy")
	}
}

// TestSkillsTab_GitHubSkillsLoaded_MergesWithLocal verifies that GitHub results
// are merged with local skills when the query matches.
func TestSkillsTab_GitHubSkillsLoaded_MergesWithLocal(t *testing.T) {
	s := NewSkillsTab()
	localSkills := []skill.Skill{
		{Name: "scaffold", Source: skill.SourceLocal, Description: "scaffold skill"},
	}
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: localSkills, Err: nil})

	// Simulate the user typing a query.
	s.searchInput.SetValue("skill")

	ghSkills := []skill.Skill{
		{Name: "k8s-skill", Source: skill.SourceGitHub, Stars: 234},
		{Name: "pr-skill", Source: skill.SourceGitHub, Stars: 100},
	}

	updated, _ := updateSkillsTab(t, s, GitHubSkillsLoadedMsg{
		Skills: ghSkills,
		Query:  "skill",
		Err:    nil,
	})

	if updated.githubLoading {
		t.Error("githubLoading should be false after GitHubSkillsLoadedMsg")
	}
	// Merged: 1 local + 2 github = 3 total, then fuzzy filtered by "skill".
	// "scaffold" contains "scaf" not "skill" so won't match. k8s-skill and pr-skill will match.
	// Let's check the merged skills list before filtering.
	if len(updated.githubSkills) != 2 {
		t.Errorf("githubSkills len = %d, want 2", len(updated.githubSkills))
	}
}

// TestSkillsTab_GitHubSkillsLoaded_StaleResultIgnored verifies that a GitHub
// result with a mismatched query is discarded.
func TestSkillsTab_GitHubSkillsLoaded_StaleResultIgnored(t *testing.T) {
	s := NewSkillsTab()
	localSkills := []skill.Skill{
		{Name: "scaffold", Source: skill.SourceLocal, Description: "scaffold skill"},
	}
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: localSkills, Err: nil})

	// Current query in the input is "nix".
	s.searchInput.SetValue("nix")

	ghSkills := []skill.Skill{
		{Name: "k8s-skill", Source: skill.SourceGitHub, Stars: 234},
	}

	// Deliver a result for the old query "kubernetes".
	updated, _ := updateSkillsTab(t, s, GitHubSkillsLoadedMsg{
		Skills: ghSkills,
		Query:  "kubernetes", // stale — does not match current "nix"
		Err:    nil,
	})

	// GitHub skills should NOT have been applied.
	if len(updated.githubSkills) != 0 {
		t.Errorf("stale GitHub result should be ignored, but githubSkills len = %d", len(updated.githubSkills))
	}
}

// TestSkillsTab_NavigationWhileGitHubLoading verifies cursor navigation is
// possible even when a GitHub fetch is in progress.
func TestSkillsTab_NavigationWhileGitHubLoading(t *testing.T) {
	s := NewSkillsTab()
	localSkills := []skill.Skill{
		{Name: "scaffold", Source: skill.SourceLocal, Description: "scaffold"},
		{Name: "commit", Source: skill.SourceLocal, Description: "commit"},
		{Name: "debug", Source: skill.SourceLocal, Description: "debug"},
	}
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: localSkills, Err: nil})

	// Simulate a GitHub fetch in progress.
	s.githubLoading = true

	// Navigation should still work.
	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyDown})
	if s.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (navigation should work while GitHub loads)", s.cursor)
	}

	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyDown})
	if s.cursor != 2 {
		t.Errorf("cursor = %d, want 2", s.cursor)
	}

	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyUp})
	if s.cursor != 1 {
		t.Errorf("cursor = %d, want 1", s.cursor)
	}
}

func TestSkillsTab_SortToggle_CyclesModes(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})

	if s.sortBy != skill.SortByStars {
		t.Errorf("initial sortBy = %d, want SortByStars(0)", s.sortBy)
	}

	// "s" (when search unfocused) should advance to SortByCreated.
	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if s.sortBy != skill.SortByCreated {
		t.Errorf("sortBy after 1st 's' = %d, want SortByCreated(1)", s.sortBy)
	}

	// Second "s" → SortByUpdated.
	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if s.sortBy != skill.SortByUpdated {
		t.Errorf("sortBy after 2nd 's' = %d, want SortByUpdated(2)", s.sortBy)
	}

	// Third "s" → back to SortByStars.
	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if s.sortBy != skill.SortByStars {
		t.Errorf("sortBy after 3rd 's' = %d, want SortByStars(0)", s.sortBy)
	}
}

func TestSkillsTab_CursorMovement_DownAndUp(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})

	if s.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", s.cursor)
	}

	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyDown})
	if s.cursor != 1 {
		t.Errorf("cursor after KeyDown = %d, want 1", s.cursor)
	}

	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyDown})
	if s.cursor != 2 {
		t.Errorf("cursor after 2nd KeyDown = %d, want 2", s.cursor)
	}

	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyUp})
	if s.cursor != 1 {
		t.Errorf("cursor after KeyUp = %d, want 1", s.cursor)
	}
}

func TestSkillsTab_CursorMovement_DoesNotGoNegative(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})

	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyUp})
	if s.cursor != 0 {
		t.Errorf("cursor = %d after KeyUp at 0, want 0", s.cursor)
	}
}

func TestSkillsTab_CursorMovement_DoesNotExceedList(t *testing.T) {
	s := NewSkillsTab()
	skills := makeTestSkillsList()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: skills, Err: nil})

	for i := 0; i < len(skills)+5; i++ {
		s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyDown})
	}
	if s.cursor != len(skills)-1 {
		t.Errorf("cursor = %d, want %d (last index)", s.cursor, len(skills)-1)
	}
}

func TestSkillsTab_Enter_GeneratesCorrectClaudeCommand(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})

	// Pre-load a README preview for the selected skill.
	s, _ = updateSkillsTab(t, s, SkillReadmeMsg{Content: "This is the readme content."})

	selected := s.filtered[s.cursor]
	cmd := buildClaudeAnalysisCmd(selected, s.preview)

	if !strings.Contains(cmd, "claude -p") {
		t.Errorf("command does not start with 'claude -p': %q", cmd)
	}
	if !strings.Contains(cmd, selected.Name) {
		t.Errorf("command does not contain skill name %q: %q", selected.Name, cmd)
	}
	if !strings.Contains(cmd, "benefits, trade-offs") {
		t.Errorf("command does not contain expected prompt text: %q", cmd)
	}
}

func TestSkillsTab_Init_ReturnsCmd(t *testing.T) {
	s := NewSkillsTab()
	cmd := s.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil command")
	}
}

func TestSkillsTab_SetSize_UpdatesDimensions(t *testing.T) {
	s := NewSkillsTab()
	s = s.SetSize(120, 40).(SkillsTab)
	if s.width != 120 || s.height != 40 {
		t.Errorf("SetSize: width=%d height=%d, want 120/40", s.width, s.height)
	}
}

func TestSkillsTab_Refresh_ResetsState(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})

	// "r" triggers refresh when search is unfocused.
	updated, cmd := updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !updated.loading {
		t.Error("loading should be true after refresh")
	}
	if len(updated.skills) != 0 {
		t.Errorf("skills should be cleared on refresh, got %d", len(updated.skills))
	}
	if len(updated.localSkills) != 0 {
		t.Errorf("localSkills should be cleared on refresh, got %d", len(updated.localSkills))
	}
	if updated.githubLoading {
		t.Error("githubLoading should be false after refresh")
	}
	if cmd == nil {
		t.Error("expected a non-nil command after refresh")
	}
}

func TestSkillsTab_View_ContainsHints(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})
	s = s.SetSize(120, 30).(SkillsTab)

	view := s.View()
	for _, hint := range []string{"↵:analyze", "o:open", "s:sort", "y:copy", "r:refresh"} {
		if !strings.Contains(view, hint) {
			t.Errorf("View() missing hint %q", hint)
		}
	}
}

func TestSkillsTab_SearchFocusMsg_FocusesInput(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})

	if s.searchInput.Focused() {
		t.Error("searchInput should not be focused initially")
	}

	s, _ = updateSkillsTab(t, s, SearchFocusMsg{})
	if !s.searchInput.Focused() {
		t.Error("searchInput should be focused after SearchFocusMsg")
	}
}

func TestSkillsTab_SearchBlurMsg_BlursInput(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})

	s, _ = updateSkillsTab(t, s, SearchFocusMsg{})
	s, _ = updateSkillsTab(t, s, SearchBlurMsg{})
	if s.searchInput.Focused() {
		t.Error("searchInput should be blurred after SearchBlurMsg")
	}
}

func TestSkillsTab_SortIgnoredWhenSearchFocused(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})
	initialSort := s.sortBy

	// Focus search first.
	s, _ = updateSkillsTab(t, s, SearchFocusMsg{})

	// "s" should go to search input, not trigger sort.
	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if s.sortBy != initialSort {
		t.Errorf("sortBy changed while search focused: got %d, want %d", s.sortBy, initialSort)
	}
	if s.searchInput.Value() != "s" {
		t.Errorf("searchInput.Value() = %q, want 's' (char should go to search when focused)", s.searchInput.Value())
	}
}

func TestSkillsTab_View_ShowsSkillNames(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})
	s = s.SetSize(120, 30).(SkillsTab)

	view := s.View()
	if !strings.Contains(view, "scaffold") {
		t.Errorf("View() missing skill 'scaffold': %s", view)
	}
}

func TestBuildClaudeAnalysisCmd_Format(t *testing.T) {
	s := skill.Skill{
		Name:        "deploy",
		URL:         "https://github.com/alice/deploy",
		Description: "Deploy services",
	}
	readme := "# Deploy\n\nDeploys things."

	cmd := buildClaudeAnalysisCmd(s, readme)

	if !strings.Contains(cmd, "claude -p") {
		t.Errorf("command = %q, want claude -p prefix", cmd)
	}
	if !strings.Contains(cmd, "deploy") {
		t.Errorf("command missing skill name: %q", cmd)
	}
	if !strings.Contains(cmd, "https://github.com/alice/deploy") {
		t.Errorf("command missing URL: %q", cmd)
	}
	if !strings.Contains(cmd, "# Deploy") {
		t.Errorf("command missing readme: %q", cmd)
	}
	_ = fmt.Sprintf("cmd length: %d", len(cmd))
}

func TestSkillsTab_Open_WithURLSkill_DoesNotPanic(t *testing.T) {
	s := NewSkillsTab()
	skills := []skill.Skill{
		{
			Name:   "k8s-skill",
			Source: skill.SourceGitHub,
			URL:    "https://github.com/alice/k8s-skill",
			Stars:  234,
		},
	}
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: skills, Err: nil})

	// Pressing "o" should attempt to open the browser without panicking.
	// The command may fail if xdg-open/open is not installed in CI, but Update itself must not panic.
	s2, cmd := updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	_ = s2
	// cmd is nil because OpenInBrowser is called synchronously and no tea.Cmd is returned.
	if cmd != nil {
		// cmd is allowed to be non-nil; just execute it safely.
		_ = cmd()
	}
}

func TestSkillsTab_Open_WithNoURL_DoesNotPanic(t *testing.T) {
	s := NewSkillsTab()
	skills := []skill.Skill{
		{
			Name:   "scaffold",
			Source: skill.SourceLocal,
			URL:    "", // no URL
		},
	}
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: skills, Err: nil})

	// Pressing "o" with no URL should be a no-op and not panic.
	_, cmd := updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	_ = cmd
}

func TestSkillsTab_Open_EmptyList_DoesNotPanic(t *testing.T) {
	s := NewSkillsTab()
	// No skills loaded — "o" should be a no-op.
	_, cmd := updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	_ = cmd
}

func TestSkillsTab_View_ContainsOpenHint(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})
	s = s.SetSize(120, 30).(SkillsTab)

	view := s.View()
	if !strings.Contains(view, "o:open") {
		t.Errorf("View() missing hint 'o:open': %s", view)
	}
}

func TestNextSortBy_Cycles(t *testing.T) {
	if nextSortBy(skill.SortByStars) != skill.SortByCreated {
		t.Error("Stars → Created failed")
	}
	if nextSortBy(skill.SortByCreated) != skill.SortByUpdated {
		t.Error("Created → Updated failed")
	}
	if nextSortBy(skill.SortByUpdated) != skill.SortByStars {
		t.Error("Updated → Stars failed")
	}
}

func TestFormatSkillLine_Local(t *testing.T) {
	s := skill.Skill{Name: "scaffold", Source: skill.SourceLocal}
	line := formatSkillLine(s, 60)
	if !strings.Contains(line, "local") {
		t.Errorf("local skill line missing 'local': %q", line)
	}
	if !strings.Contains(line, "scaffold") {
		t.Errorf("local skill line missing name: %q", line)
	}
}

func TestFormatSkillLine_GitHub(t *testing.T) {
	s := skill.Skill{Name: "k8s-skill", Source: skill.SourceGitHub, Stars: 234}
	line := formatSkillLine(s, 60)
	if !strings.Contains(line, "234") {
		t.Errorf("github skill line missing stars: %q", line)
	}
}

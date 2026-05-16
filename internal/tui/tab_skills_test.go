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

func TestSkillsTab_SortToggle_CyclesModes(t *testing.T) {
	s := NewSkillsTab()
	s, _ = updateSkillsTab(t, s, LocalSkillsLoadedMsg{Skills: makeTestSkillsList(), Err: nil})

	if s.sortBy != skill.SortByStars {
		t.Errorf("initial sortBy = %d, want SortByStars(0)", s.sortBy)
	}

	// Ctrl+T should advance to SortByCreated.
	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyCtrlT})
	if s.sortBy != skill.SortByCreated {
		t.Errorf("sortBy after 1st Ctrl+T = %d, want SortByCreated(1)", s.sortBy)
	}

	// Second Ctrl+T → SortByUpdated.
	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyCtrlT})
	if s.sortBy != skill.SortByUpdated {
		t.Errorf("sortBy after 2nd Ctrl+T = %d, want SortByUpdated(2)", s.sortBy)
	}

	// Third Ctrl+T → back to SortByStars.
	s, _ = updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyCtrlT})
	if s.sortBy != skill.SortByStars {
		t.Errorf("sortBy after 3rd Ctrl+T = %d, want SortByStars(0)", s.sortBy)
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

	updated, cmd := updateSkillsTab(t, s, tea.KeyMsg{Type: tea.KeyCtrlR})
	if !updated.loading {
		t.Error("loading should be true after refresh")
	}
	if len(updated.skills) != 0 {
		t.Errorf("skills should be cleared on refresh, got %d", len(updated.skills))
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
	for _, hint := range []string{"Enter:analyze", "^T:sort", "^Y:copy", "^R:refresh"} {
		if !strings.Contains(view, hint) {
			t.Errorf("View() missing hint %q", hint)
		}
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

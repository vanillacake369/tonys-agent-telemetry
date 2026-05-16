package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/skill"
)

// makeTestSkill returns a simple skill for wizard tests.
func makeTestSkill() skill.Skill {
	return skill.Skill{
		Name:        "superpowers",
		URL:         "https://github.com/obra/superpowers",
		Description: "An agentic skills framework for Claude Code",
		Source:      skill.SourceGitHub,
		Stars:       42,
	}
}

// updateWizard is a test helper that runs one Update cycle on the wizard.
func updateWizard(t *testing.T, w AnalyzeWizard, msg tea.Msg) (AnalyzeWizard, tea.Cmd) {
	t.Helper()
	updated, cmd := w.Update(msg)
	return updated, cmd
}

func TestAnalyzeWizard_InitialState(t *testing.T) {
	w := NewAnalyzeWizard()
	if w.visible {
		t.Error("wizard should not be visible before Show()")
	}
	if w.step != StepModel {
		t.Errorf("initial step = %d, want StepModel(0)", w.step)
	}
	if w.modelCursor != 0 {
		t.Errorf("initial modelCursor = %d, want 0", w.modelCursor)
	}
	if len(w.models) == 0 {
		t.Error("models list should not be empty")
	}
	if w.models[0] != "claude" {
		t.Errorf("models[0] = %q, want \"claude\"", w.models[0])
	}
}

func TestAnalyzeWizard_Show_SetsVisible(t *testing.T) {
	w := NewAnalyzeWizard()
	s := makeTestSkill()
	w.Show(s, "# Readme content")

	if !w.visible {
		t.Error("wizard should be visible after Show()")
	}
	if w.step != StepModel {
		t.Errorf("step after Show = %d, want StepModel(0)", w.step)
	}
	if w.skill.Name != s.Name {
		t.Errorf("skill.Name = %q, want %q", w.skill.Name, s.Name)
	}
}

func TestAnalyzeWizard_Show_PopulatesDefaultPrompt(t *testing.T) {
	w := NewAnalyzeWizard()
	s := makeTestSkill()
	w.Show(s, "# Readme\n\nSome content here.")

	if !strings.Contains(w.defaultPrompt, s.Name) {
		t.Errorf("defaultPrompt missing skill name %q", s.Name)
	}
	if !strings.Contains(w.defaultPrompt, s.URL) {
		t.Errorf("defaultPrompt missing skill URL %q", s.URL)
	}
	if !strings.Contains(w.defaultPrompt, s.Description) {
		t.Errorf("defaultPrompt missing skill description")
	}
	if !strings.Contains(w.defaultPrompt, "README") {
		t.Errorf("defaultPrompt missing README excerpt")
	}
}

func TestAnalyzeWizard_ModelCursorDown(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyDown})
	if w.modelCursor != 1 {
		t.Errorf("modelCursor after Down = %d, want 1", w.modelCursor)
	}
}

func TestAnalyzeWizard_ModelCursorUp(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")
	w.modelCursor = 2

	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyUp})
	if w.modelCursor != 1 {
		t.Errorf("modelCursor after Up = %d, want 1", w.modelCursor)
	}
}

func TestAnalyzeWizard_ModelCursorDoesNotGoNegative(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")
	// Already at 0.
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyUp})
	if w.modelCursor != 0 {
		t.Errorf("modelCursor should stay at 0, got %d", w.modelCursor)
	}
}

func TestAnalyzeWizard_ModelCursorDoesNotExceedList(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")
	// Move past end.
	for i := 0; i < len(w.models)+5; i++ {
		w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyDown})
	}
	if w.modelCursor != len(w.models)-1 {
		t.Errorf("modelCursor = %d, want %d (last model)", w.modelCursor, len(w.models)-1)
	}
}

func TestAnalyzeWizard_EnterOnModelAdvancesToPromptStep(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != StepPrompt {
		t.Errorf("step after Enter on model = %d, want StepPrompt(1)", w.step)
	}
	if !w.visible {
		t.Error("wizard should still be visible after advancing to StepPrompt")
	}
}

func TestAnalyzeWizard_TabOnModelUsesDefaultAndAdvances(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")
	// Move cursor to gemini first.
	w.modelCursor = 2

	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyTab})
	if w.step != StepPrompt {
		t.Errorf("step after Tab on model = %d, want StepPrompt(1)", w.step)
	}
	// Tab should reset cursor to default (claude = 0).
	if w.modelCursor != 0 {
		t.Errorf("modelCursor after Tab = %d, want 0 (default claude)", w.modelCursor)
	}
}

func TestAnalyzeWizard_EscOnModelClosesWizard(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEscape})
	if w.visible {
		t.Error("wizard should not be visible after Esc on model step")
	}
}

func TestAnalyzeWizard_EscOnPromptGoesBackToModelStep(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	// Advance to prompt step.
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter})
	if w.step != StepPrompt {
		t.Fatalf("expected StepPrompt, got %d", w.step)
	}

	// Esc on prompt step should go back to model step, not close.
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEscape})
	if !w.visible {
		t.Error("wizard should still be visible after Esc on prompt step")
	}
	if w.step != StepModel {
		t.Errorf("step after Esc on prompt = %d, want StepModel(0)", w.step)
	}
}

func TestAnalyzeWizard_EnterOnPromptEmitsAnalyzeExecuteMsg(t *testing.T) {
	w := NewAnalyzeWizard()
	s := makeTestSkill()
	w.Show(s, "readme content")

	// Advance to prompt step (using Tab for simplicity — picks default model).
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyTab})
	if w.step != StepPrompt {
		t.Fatalf("expected StepPrompt, got %d", w.step)
	}

	// Press Enter to execute.
	_, cmd := updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("cmd should not be nil after Enter on prompt step")
	}

	msg := cmd()
	execMsg, ok := msg.(AnalyzeExecuteMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want AnalyzeExecuteMsg", msg)
	}
	if execMsg.Model != "claude" {
		t.Errorf("Model = %q, want %q", execMsg.Model, "claude")
	}
	if execMsg.Prompt == "" {
		t.Error("Prompt should not be empty")
	}
}

func TestAnalyzeWizard_TabOnPromptUsesDefaultAndEmitsMsg(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	// Advance to prompt step first.
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter})

	// Tab on prompt step should use default prompt and emit execute message.
	_, cmd := updateWizard(t, w, tea.KeyMsg{Type: tea.KeyTab})
	if cmd == nil {
		t.Fatal("cmd should not be nil after Tab on prompt step")
	}

	msg := cmd()
	execMsg, ok := msg.(AnalyzeExecuteMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want AnalyzeExecuteMsg", msg)
	}
	if execMsg.Prompt == "" {
		t.Error("Prompt should not be empty")
	}
}

func TestAnalyzeWizard_AnalyzeExecuteMsg_ContainsSkillInfo(t *testing.T) {
	w := NewAnalyzeWizard()
	s := makeTestSkill()
	w.Show(s, "# Readme\n\nSkill readme content.")

	// Tab through model step (use default model).
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyTab})

	// Execute with Enter.
	_, cmd := updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	execMsg := cmd().(AnalyzeExecuteMsg)

	if !strings.Contains(execMsg.Prompt, s.Name) {
		t.Errorf("Prompt missing skill name %q", s.Name)
	}
	if !strings.Contains(execMsg.Prompt, s.URL) {
		t.Errorf("Prompt missing skill URL %q", s.URL)
	}
}

func TestAnalyzeWizard_AnalyzeExecuteMsg_ModelMatchesSelection(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	// Move to "codex" (index 1) then Enter.
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyDown})
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter}) // advance to prompt
	_, cmd := updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter}) // execute

	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	execMsg := cmd().(AnalyzeExecuteMsg)
	if execMsg.Model != "codex" {
		t.Errorf("Model = %q, want %q", execMsg.Model, "codex")
	}
}

func TestAnalyzeWizard_WizardNotVisible_UpdateIsNoop(t *testing.T) {
	w := NewAnalyzeWizard()
	// Wizard is not visible — Update should be a no-op.
	w2, cmd := updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter})
	if w2.visible {
		t.Error("invisible wizard should remain invisible after Update")
	}
	if cmd != nil {
		t.Error("invisible wizard should return nil cmd")
	}
}

func TestBuildAnalyzeCmd_Claude(t *testing.T) {
	cmd, err := buildAnalyzeCmd("claude", "analyze this skill")
	if err != nil {
		t.Fatalf("buildAnalyzeCmd error: %v", err)
	}
	if !strings.Contains(cmd, "claude -p") {
		t.Errorf("claude command missing 'claude -p': %q", cmd)
	}
	if !strings.Contains(cmd, analyzeTempFile) {
		t.Errorf("claude command missing temp file path: %q", cmd)
	}
}

func TestBuildAnalyzeCmd_Codex(t *testing.T) {
	cmd, err := buildAnalyzeCmd("codex", "analyze this skill")
	if err != nil {
		t.Fatalf("buildAnalyzeCmd error: %v", err)
	}
	if !strings.Contains(cmd, "curl") {
		t.Errorf("codex command missing 'curl': %q", cmd)
	}
	if !strings.Contains(cmd, "127.0.0.1:4001") {
		t.Errorf("codex command missing proxy address: %q", cmd)
	}
	if !strings.Contains(cmd, "codex") {
		t.Errorf("codex command missing model name: %q", cmd)
	}
}

func TestBuildAnalyzeCmd_Gemini(t *testing.T) {
	cmd, err := buildAnalyzeCmd("gemini", "analyze this skill")
	if err != nil {
		t.Fatalf("buildAnalyzeCmd error: %v", err)
	}
	if !strings.Contains(cmd, "curl") {
		t.Errorf("gemini command missing 'curl': %q", cmd)
	}
	if !strings.Contains(cmd, "gemini") {
		t.Errorf("gemini command missing model name: %q", cmd)
	}
}

func TestAnalyzeWizard_View_ModelStep_ContainsModels(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	view := w.View()
	for _, m := range w.models {
		if !strings.Contains(view, m) {
			t.Errorf("View (model step) missing model %q", m)
		}
	}
}

func TestAnalyzeWizard_View_ModelStep_ContainsSkillName(t *testing.T) {
	w := NewAnalyzeWizard()
	s := makeTestSkill()
	w.Show(s, "readme")

	view := w.View()
	if !strings.Contains(view, s.Name) {
		t.Errorf("View (model step) missing skill name %q", s.Name)
	}
}

func TestAnalyzeWizard_View_PromptStep_ContainsModel(t *testing.T) {
	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	// Advance to prompt step.
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter})

	view := w.View()
	if !strings.Contains(view, "claude") {
		t.Errorf("View (prompt step) missing selected model name: %q", view)
	}
}

func TestAnalyzeWizard_View_NotVisible_ReturnsEmpty(t *testing.T) {
	w := NewAnalyzeWizard()
	// Not shown — View should return empty string.
	if w.View() != "" {
		t.Error("View() should return empty string when wizard is not visible")
	}
}

func TestAnalyzeWizard_DefaultPromptTemplate_HasAllSections(t *testing.T) {
	w := NewAnalyzeWizard()
	s := skill.Skill{
		Name:        "test-skill",
		URL:         "https://github.com/test/skill",
		Description: "A test skill description",
	}
	w.Show(s, "README content here")

	for _, substr := range []string{
		s.Name,
		s.URL,
		s.Description,
		"benefits",
		"workflow",
		"Nix",
	} {
		if !strings.Contains(w.defaultPrompt, substr) {
			t.Errorf("defaultPrompt missing expected content %q", substr)
		}
	}
}

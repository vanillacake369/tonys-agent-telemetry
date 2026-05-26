package tui

import (
	"errors"
	"os/exec"
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

// --- υ-1 TDD: availableAnalyzeModels ----------------------------------------

// TestAvailableModels_OnlyIncludesInstalled verifies that availableAnalyzeModels
// filters through the injected lookPath and only returns models whose binary was
// found. Fake LookPath returns success only for "claude".
func TestAvailableModels_OnlyIncludesInstalled(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	got := availableAnalyzeModels()
	if len(got) != 1 {
		t.Fatalf("availableAnalyzeModels() = %v, want exactly [\"claude\"]", got)
	}
	if got[0] != "claude" {
		t.Errorf("availableAnalyzeModels()[0] = %q, want \"claude\"", got[0])
	}
}

// TestAvailableModels_BothInstalled verifies that both claude and gemini are
// returned when both binaries are present.
func TestAvailableModels_BothInstalled(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		switch file {
		case "claude", "gemini":
			return "/usr/local/bin/" + file, nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	got := availableAnalyzeModels()
	if len(got) != 2 {
		t.Fatalf("availableAnalyzeModels() = %v, want [\"claude\", \"gemini\"]", got)
	}
	found := make(map[string]bool)
	for _, m := range got {
		found[m] = true
	}
	if !found["claude"] {
		t.Error("claude missing from result")
	}
	if !found["gemini"] {
		t.Error("gemini missing from result")
	}
}

// TestAvailableModels_NoneInstalled verifies empty slice when no binary found.
func TestAvailableModels_NoneInstalled(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	got := availableAnalyzeModels()
	if len(got) != 0 {
		t.Errorf("availableAnalyzeModels() = %v, want empty slice", got)
	}
}

// --- υ-1 TDD: buildAnalyzeCmd -----------------------------------------------

// TestBuildAnalyzeCmd_ClaudeIsInteractive asserts the returned *exec.Cmd has
// args ["claude", prompt] with NO "-p" flag (interactive session, not one-shot).
func TestBuildAnalyzeCmd_ClaudeIsInteractive(t *testing.T) {
	prompt := "analyze this skill"
	cmd, err := buildAnalyzeCmd("claude", prompt)
	if err != nil {
		t.Fatalf("buildAnalyzeCmd(claude) error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil *exec.Cmd")
	}
	// Args[0] is the binary path resolved by exec.Command — Args[1] is the first real arg.
	// exec.Command("claude", prompt).Args == ["claude", prompt]
	if len(cmd.Args) < 2 {
		t.Fatalf("cmd.Args = %v, want at least 2 elements", cmd.Args)
	}
	if cmd.Args[0] != "claude" {
		t.Errorf("cmd.Args[0] = %q, want \"claude\"", cmd.Args[0])
	}
	if cmd.Args[1] != prompt {
		t.Errorf("cmd.Args[1] = %q, want prompt %q", cmd.Args[1], prompt)
	}
	// Must NOT contain "-p" in any position.
	for _, a := range cmd.Args {
		if a == "-p" {
			t.Errorf("cmd.Args contains \"-p\" — claude should run interactively, not with -p")
		}
	}
}

// TestBuildAnalyzeCmd_GeminiReturnsCorrectArgs verifies gemini gets --prompt flag.
func TestBuildAnalyzeCmd_GeminiReturnsCorrectArgs(t *testing.T) {
	prompt := "analyze this skill"
	cmd, err := buildAnalyzeCmd("gemini", prompt)
	if err != nil {
		t.Fatalf("buildAnalyzeCmd(gemini) error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected non-nil *exec.Cmd")
	}
	if cmd.Args[0] != "gemini" {
		t.Errorf("cmd.Args[0] = %q, want \"gemini\"", cmd.Args[0])
	}
	// Must contain "--prompt" flag.
	foundFlag := false
	for _, a := range cmd.Args {
		if a == "--prompt" {
			foundFlag = true
		}
	}
	if !foundFlag {
		t.Errorf("cmd.Args = %v, want --prompt flag for gemini", cmd.Args)
	}
	// Last arg should be the prompt.
	last := cmd.Args[len(cmd.Args)-1]
	if last != prompt {
		t.Errorf("last arg = %q, want prompt %q", last, prompt)
	}
}

// TestBuildAnalyzeCmd_UnknownModel_ReturnsError verifies that an unknown model
// string returns a non-nil error and a nil cmd.
func TestBuildAnalyzeCmd_UnknownModel_ReturnsError(t *testing.T) {
	cmd, err := buildAnalyzeCmd("unknown-llm", "some prompt")
	if err == nil {
		t.Error("expected error for unknown model, got nil")
	}
	if cmd != nil {
		t.Errorf("expected nil cmd for unknown model, got %v", cmd)
	}
}

// TestBuildAnalyzeCmd_CodexRemoved_ReturnsError verifies that selecting "codex"
// (removed 2026-05-26 — no current Codex CLI; proxy-api path was broken) returns
// the sentinel error ErrCodexRemoved and does not route via localhost:4001.
func TestBuildAnalyzeCmd_CodexRemoved_ReturnsError(t *testing.T) {
	cmd, err := buildAnalyzeCmd("codex", "some prompt")
	if err == nil {
		t.Fatal("expected ErrCodexRemoved for codex model, got nil")
	}
	if !errors.Is(err, ErrCodexRemoved) {
		t.Errorf("err = %v, want errors.Is(err, ErrCodexRemoved)", err)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd for codex, got %v", cmd)
	}
	// Explicit: must not contain the removed proxy address.
	if strings.Contains(err.Error(), "127.0.0.1") {
		t.Error("codex error must not reference the old proxy address 127.0.0.1:4001")
	}
}

// TestBuildAnalyzeCmd_NoShellInterpolation verifies that the returned cmd does
// NOT use shell interpolation ("bash", "-c", "cat $FILE"). The cmd must be a
// direct exec, not a shell wrapper.
func TestBuildAnalyzeCmd_NoShellInterpolation(t *testing.T) {
	cmd, err := buildAnalyzeCmd("claude", "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, a := range cmd.Args {
		if a == "bash" || a == "sh" {
			t.Errorf("cmd.Args contains %q — must not use shell wrapper", a)
		}
		if strings.Contains(a, "$(cat") || strings.Contains(a, "$FILE") {
			t.Errorf("cmd.Args[%q] contains shell variable substitution — must not use shell interpolation", a)
		}
	}
}

// --- υ-3 TDD: wizard model list reflects availability -----------------------

// TestAnalyzeWizard_RendersOnlyAvailableModels verifies that NewAnalyzeWizard
// populates its models list from availableAnalyzeModels (via the injected
// lookPath), so unavailable binaries are never offered to the user.
func TestAnalyzeWizard_RendersOnlyAvailableModels(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	// Only gemini is available in this scenario.
	lookPath = func(file string) (string, error) {
		if file == "gemini" {
			return "/usr/local/bin/gemini", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	view := w.View()
	if !strings.Contains(view, "gemini") {
		t.Errorf("View missing available model \"gemini\": %q", view)
	}
	if strings.Contains(view, "claude") {
		t.Errorf("View should not show unavailable model \"claude\"")
	}
	if strings.Contains(view, "codex") {
		t.Errorf("View must never show removed model \"codex\"")
	}
}

// TestAnalyzeWizard_NoModelsAvailable_ShowsInstallHint verifies that when no
// model binary is on PATH, the wizard shows a helpful install message instead
// of an empty list.
func TestAnalyzeWizard_NoModelsAvailable_ShowsInstallHint(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	view := w.View()
	if !strings.Contains(view, "claude") || !strings.Contains(view, "gemini") {
		// The hint should mention the CLIs by name so the user knows what to install.
		t.Errorf("install hint should mention 'claude' or 'gemini': %q", view)
	}
	// Must not show the normal model selection cursor.
	if strings.Contains(view, "▸") {
		t.Errorf("View must not show selection cursor when no models are available: %q", view)
	}
}

// TestAnalyzeWizard_SingleModel_AutoSelects verifies that when exactly one model
// is available, it is auto-selected (modelCursor = 0, step goes to prompt on Enter).
func TestAnalyzeWizard_SingleModel_AutoSelects(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	if len(w.models) != 1 {
		t.Fatalf("expected 1 model, got %v", w.models)
	}
	if w.models[0] != "claude" {
		t.Errorf("auto-selected model = %q, want \"claude\"", w.models[0])
	}
	if w.modelCursor != 0 {
		t.Errorf("modelCursor = %d, want 0", w.modelCursor)
	}
}

// --- Preserved existing tests (style unchanged) -----------------------------

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
	// models list may be empty before Show if no CLIs are installed; that's fine.
}

func TestAnalyzeWizard_Show_SetsVisible(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		switch file {
		case "claude", "gemini":
			return "/usr/local/bin/" + file, nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")
	// Need at least 2 models to test cursor movement.
	if len(w.models) < 2 {
		t.Skip("need >=2 available models for cursor test")
	}

	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyDown})
	if w.modelCursor != 1 {
		t.Errorf("modelCursor after Down = %d, want 1", w.modelCursor)
	}
}

func TestAnalyzeWizard_ModelCursorUp(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		switch file {
		case "claude", "gemini":
			return "/usr/local/bin/" + file, nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")
	if len(w.models) < 2 {
		t.Skip("need >=2 available models for cursor test")
	}
	w.modelCursor = 1

	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyUp})
	if w.modelCursor != 0 {
		t.Errorf("modelCursor after Up = %d, want 0", w.modelCursor)
	}
}

func TestAnalyzeWizard_ModelCursorDoesNotGoNegative(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")
	// Already at 0.
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyUp})
	if w.modelCursor != 0 {
		t.Errorf("modelCursor should stay at 0, got %d", w.modelCursor)
	}
}

func TestAnalyzeWizard_ModelCursorDoesNotExceedList(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		switch file {
		case "claude", "gemini":
			return "/usr/local/bin/" + file, nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")
	if len(w.models) == 0 {
		t.Skip("no models available")
	}
	// Move past end.
	for i := 0; i < len(w.models)+5; i++ {
		w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyDown})
	}
	if w.modelCursor != len(w.models)-1 {
		t.Errorf("modelCursor = %d, want %d (last model)", w.modelCursor, len(w.models)-1)
	}
}

func TestAnalyzeWizard_EnterOnModelAdvancesToPromptStep(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")

	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEscape})
	if w.visible {
		t.Error("wizard should not be visible after Esc on model step")
	}
}

func TestAnalyzeWizard_EscOnPromptGoesBackToModelStep(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	// Both claude and gemini available so the user can navigate to index 1.
	lookPath = func(file string) (string, error) {
		switch file {
		case "claude", "gemini":
			return "/usr/local/bin/" + file, nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	w.Show(makeTestSkill(), "readme")
	if len(w.models) < 2 {
		t.Skip("need >=2 models for model-selection test")
	}
	secondModel := w.models[1]

	// Move to second model then Enter.
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyDown})
	w, _ = updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter}) // advance to prompt
	_, cmd := updateWizard(t, w, tea.KeyMsg{Type: tea.KeyEnter}) // execute

	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	execMsg := cmd().(AnalyzeExecuteMsg)
	if execMsg.Model != secondModel {
		t.Errorf("Model = %q, want %q", execMsg.Model, secondModel)
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

func TestAnalyzeWizard_View_ModelStep_ContainsModels(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

	w := NewAnalyzeWizard()
	s := makeTestSkill()
	w.Show(s, "readme")

	view := w.View()
	if !strings.Contains(view, s.Name) {
		t.Errorf("View (model step) missing skill name %q", s.Name)
	}
}

func TestAnalyzeWizard_View_PromptStep_ContainsModel(t *testing.T) {
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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
	orig := lookPath
	t.Cleanup(func() { lookPath = orig })
	lookPath = func(file string) (string, error) {
		if file == "claude" {
			return "/usr/local/bin/claude", nil
		}
		return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
	}

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

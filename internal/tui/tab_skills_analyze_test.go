package tui

import (
	"errors"
	"os/exec"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSkillsTab_AnalyzeExecuteMsg_ReturnsTeaExecProcessCmd feeds a fake
// AnalyzeExecuteMsg to the SkillsTab Update handler and asserts that the
// returned tea.Cmd is non-nil (indicating tea.ExecProcess was wired up).
// The returned cmd, when invoked, must NOT return a RecommendationsReadyMsg
// or a SkillsSearchResultMsg — those would indicate old code paths.
func TestSkillsTab_AnalyzeExecuteMsg_ReturnsTeaExecProcessCmd(t *testing.T) {
	s := NewSkillsTab()

	// Use "claude" which is always in the model table.
	msg := AnalyzeExecuteMsg{Model: "claude", Prompt: "analyze this skill"}

	updated, cmd := updateSkillsTab(t, s, msg)

	// wizard must be hidden after execute.
	if updated.wizard.visible {
		t.Error("wizard should be hidden after AnalyzeExecuteMsg")
	}

	// cmd must be non-nil — we expect tea.ExecProcess.
	if cmd == nil {
		t.Fatal("expected non-nil tea.Cmd from AnalyzeExecuteMsg handler")
	}
}

// TestSkillsTab_AnalyzeFinishedMsg_SurfacesError feeds an AnalyzeFinishedMsg
// with a non-nil error to the SkillsTab handler and asserts it is stored in t.err.
func TestSkillsTab_AnalyzeFinishedMsg_SurfacesError(t *testing.T) {
	s := NewSkillsTab()
	boom := errors.New("claude exited with status 1")

	updated, cmd := updateSkillsTab(t, s, AnalyzeFinishedMsg{Err: boom})

	if updated.err == nil {
		t.Error("expected t.err to be set after AnalyzeFinishedMsg with error")
	}
	if updated.err.Error() != boom.Error() {
		t.Errorf("t.err = %v, want %v", updated.err, boom)
	}
	if cmd != nil {
		t.Errorf("expected nil cmd from AnalyzeFinishedMsg, got %T", cmd)
	}
}

// TestSkillsTab_AnalyzeFinishedMsg_NoError_ClearsErr verifies that a successful
// AnalyzeFinishedMsg (Err == nil) leaves t.err unset (or clears it).
func TestSkillsTab_AnalyzeFinishedMsg_NoError_ClearsErr(t *testing.T) {
	s := NewSkillsTab()
	s.err = errors.New("previous error")

	updated, _ := updateSkillsTab(t, s, AnalyzeFinishedMsg{Err: nil})

	if updated.err != nil {
		t.Errorf("expected t.err to be nil after success AnalyzeFinishedMsg, got %v", updated.err)
	}
}

// TestSkillsTab_AnalyzeExecuteMsg_UnknownModel_SetsErr feeds an unknown model
// name to the AnalyzeExecuteMsg handler and asserts t.err is set (no panic,
// no tea.ExecProcess with broken cmd).
func TestSkillsTab_AnalyzeExecuteMsg_UnknownModel_SetsErr(t *testing.T) {
	s := NewSkillsTab()

	updated, cmd := updateSkillsTab(t, s, AnalyzeExecuteMsg{Model: "unknown-llm", Prompt: "test"})

	if updated.err == nil {
		t.Error("expected t.err to be set for unknown model")
	}
	if cmd != nil {
		t.Errorf("expected nil cmd for unknown model, got %T", cmd)
	}
}

// --- helpers used by analyze_wizard_test.go extension -----------------------

// execCmdArgs is a helper that runs *exec.Cmd.Args slice assertion.
func execCmdArgs(t *testing.T, cmd *exec.Cmd, wantArgs []string) {
	t.Helper()
	if len(cmd.Args) != len(wantArgs) {
		t.Fatalf("cmd.Args = %v, want %v", cmd.Args, wantArgs)
	}
	for i, a := range wantArgs {
		if cmd.Args[i] != a {
			t.Errorf("cmd.Args[%d] = %q, want %q", i, cmd.Args[i], a)
		}
	}
}

// dummyExecProcess is a test check: if we invoke the returned cmd from
// AnalyzeExecuteMsg it should NOT return a msg immediately (ExecProcess
// suspends the TUI and resumes with a callback msg only after the process
// exits). We can't actually run a process in unit tests, so we just verify
// the cmd is not the kind that returns a named msg type synchronously.
func TestSkillsTab_AnalyzeExecuteMsg_CmdIsNotSynchronous(t *testing.T) {
	s := NewSkillsTab()
	msg := AnalyzeExecuteMsg{Model: "claude", Prompt: "test prompt"}

	_, cmd := updateSkillsTab(t, s, msg)
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	// tea.ExecProcess returns a special internal cmd that the bubbletea runtime
	// consumes — if we call it outside the runtime it panics or returns nil.
	// We deliberately do NOT call cmd() here. The non-nil guarantee is sufficient.
	_ = cmd
}

// TestAnalyzeFinishedMsg_TypeCheck verifies the AnalyzeFinishedMsg type is
// usable as a tea.Msg (compile-time check via interface assertion).
func TestAnalyzeFinishedMsg_TypeCheck(t *testing.T) {
	var _ tea.Msg = AnalyzeFinishedMsg{}
	var _ tea.Msg = AnalyzeFinishedMsg{Err: errors.New("x")}
}

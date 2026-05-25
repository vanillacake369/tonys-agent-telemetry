package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// updateHooksTab is a test helper that runs one Update cycle and returns the
// resulting HooksTab (fatals if model changes type).
func updateHooksTab(t *testing.T, h HooksTab, msg tea.Msg) (HooksTab, tea.Cmd) {
	t.Helper()
	model, cmd := h.Update(msg)
	result, ok := model.(HooksTab)
	if !ok {
		t.Fatalf("Update returned %T, want HooksTab", model)
	}
	return result, cmd
}

// makeMockHooksConfig creates a test configuration mimicking real settings.
func makeMockHooksConfig() HooksConfig {
	return HooksConfig{
		Hooks: map[string][]HookGroup{
			"UserPromptSubmit": {
				{
					Matcher: "",
					Hooks: []HookEntry{
						{Type: "command", Command: "~/.claude/hooks/complexity-router.sh", Timeout: 5},
					},
				},
			},
			"PreToolUse": {
				{
					Matcher: "Bash",
					Hooks: []HookEntry{
						{Type: "command", Command: "~/.claude/hooks/cmd-guard.sh", Timeout: 5},
						{Type: "command", Command: "~/.claude/hooks/branch-guard.sh", Timeout: 5},
					},
				},
				{
					Matcher: "Write|Edit|Read",
					Hooks: []HookEntry{
						{Type: "command", Command: "~/.claude/hooks/path-guard.sh", Timeout: 5},
						{Type: "command", Command: "~/.claude/hooks/complexity-gate.sh", Timeout: 5},
					},
				},
			},
			"PostToolUse": {
				{
					Matcher: "Bash",
					Hooks: []HookEntry{
						{Type: "command", Command: "~/.claude/hooks/test-feedback.sh", Timeout: 5},
					},
				},
			},
			"Stop": {
				{
					Matcher: "",
					Hooks: []HookEntry{
						{Type: "command", Command: "~/.claude/hooks/agent-notify.sh claude", Timeout: 5},
					},
				},
			},
		},
		StatusLine: &StatusLineConfig{
			Type:    "command",
			Command: "~/.claude/hooks/statusline.sh",
		},
	}
}

// ── HooksLoadedMsg ──────────────────────────────────────────────────────────

func TestHooksTab_HooksLoadedMsg_PopulatesConfig(t *testing.T) {
	h := NewHooksTab()
	config := makeMockHooksConfig()

	h, _ = updateHooksTab(t, h, HooksLoadedMsg{Config: config})

	if h.loading {
		t.Error("loading should be false after HooksLoadedMsg")
	}
	if h.err != nil {
		t.Errorf("err = %v, want nil", h.err)
	}
	if len(h.config.Hooks) != 4 {
		t.Errorf("hooks events = %d, want 4", len(h.config.Hooks))
	}
}

func TestHooksTab_HooksLoadedMsg_WithError(t *testing.T) {
	h := NewHooksTab()
	h, _ = updateHooksTab(t, h, HooksLoadedMsg{Err: errTest("read error")})

	if h.loading {
		t.Error("loading should be false after error HooksLoadedMsg")
	}
	if h.err == nil {
		t.Error("err should be set after error HooksLoadedMsg")
	}
}

// ── Refresh ────────────────────────────────────────────────────────────────

func TestHooksTab_Refresh_ResetsState(t *testing.T) {
	h := NewHooksTab()
	config := makeMockHooksConfig()
	h, _ = updateHooksTab(t, h, HooksLoadedMsg{Config: config})

	h2, cmd := updateHooksTab(t, h, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if !h2.loading {
		t.Error("loading should be true after refresh")
	}
	if cmd == nil {
		t.Error("refresh should return a non-nil reload cmd")
	}
}

// ── View ───────────────────────────────────────────────────────────────────

func TestHooksTab_View_ContainsWorkflowSections(t *testing.T) {
	h := NewHooksTab()
	h = h.SetSize(120, 40).(HooksTab)
	config := makeMockHooksConfig()
	h, _ = updateHooksTab(t, h, HooksLoadedMsg{Config: config})

	view := h.View()

	for _, section := range []string{
		"Complexity Harness Workflow",
		"UserPromptSubmit",
		"PreToolUse",
		"PostToolUse",
		"Stop",
		"StatusLine",
	} {
		if !strings.Contains(view, section) {
			t.Errorf("View() missing section %q", section)
		}
	}
}

func TestHooksTab_View_ContainsHookScripts(t *testing.T) {
	h := NewHooksTab()
	h = h.SetSize(120, 40).(HooksTab)
	config := makeMockHooksConfig()
	h, _ = updateHooksTab(t, h, HooksLoadedMsg{Config: config})

	view := h.View()

	for _, script := range []string{
		"complexity-router.sh",
		"cmd-guard.sh",
		"path-guard.sh",
		"test-feedback.sh",
		"agent-notify.sh",
	} {
		if !strings.Contains(view, script) {
			t.Errorf("View() missing hook script %q", script)
		}
	}
}

func TestHooksTab_View_ContainsMatchers(t *testing.T) {
	h := NewHooksTab()
	h = h.SetSize(120, 40).(HooksTab)
	config := makeMockHooksConfig()
	h, _ = updateHooksTab(t, h, HooksLoadedMsg{Config: config})

	view := h.View()

	for _, matcher := range []string{"matcher: Bash", "matcher: Write|Edit|Read"} {
		if !strings.Contains(view, matcher) {
			t.Errorf("View() missing matcher %q", matcher)
		}
	}
}

func TestHooksTab_View_LoadingState(t *testing.T) {
	h := NewHooksTab()
	h = h.SetSize(120, 40).(HooksTab)
	view := h.View()
	if view == "" {
		t.Error("View() in loading state should not be empty")
	}
}

// ── SetSize ────────────────────────────────────────────────────────────────

func TestHooksTab_SetSize_UpdatesDimensions(t *testing.T) {
	h := NewHooksTab()
	updated := h.SetSize(100, 30).(HooksTab)

	if updated.width != 100 {
		t.Errorf("width = %d, want 100", updated.width)
	}
	if updated.height != 30 {
		t.Errorf("height = %d, want 30", updated.height)
	}
}

// ── Init ───────────────────────────────────────────────────────────────────

func TestHooksTab_Init_ReturnsCmd(t *testing.T) {
	h := NewHooksTab()
	cmd := h.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil Cmd")
	}
}

// ── App-level hooks tab integration ─────────────────────────────────────

func TestApp_HooksTabHints(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	hints := a.tabHints()
	if !strings.Contains(hints, "r:refresh") {
		t.Errorf("hooks tab hints should contain 'r:refresh', got %q", hints)
	}
}

func TestApp_HooksTab_InTabBar(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 120, Height: 24})
	view := a.View()
	if !strings.Contains(view, "4:Hooks") {
		t.Errorf("View() should contain '4:Hooks' in tab bar")
	}
}

// ── expandHome ─────────────────────────────────────────────────────────────

func TestExpandHome_WithTilde(t *testing.T) {
	result := expandHome("~/test/path")
	if strings.HasPrefix(result, "~") {
		t.Errorf("expandHome should expand ~, got %q", result)
	}
	if !strings.HasSuffix(result, "/test/path") {
		t.Errorf("expandHome should preserve path suffix, got %q", result)
	}
}

func TestExpandHome_WithoutTilde(t *testing.T) {
	result := expandHome("/absolute/path")
	if result != "/absolute/path" {
		t.Errorf("expandHome should not modify absolute paths, got %q", result)
	}
}

// ── renderHookGroup ────────────────────────────────────────────────────────

func TestRenderHookGroup_WildcardMatcher(t *testing.T) {
	group := HookGroup{
		Matcher: "",
		Hooks: []HookEntry{
			{Type: "command", Command: "test.sh", Timeout: 5},
		},
	}
	result := renderHookGroup(group, 80)
	if !strings.Contains(result, "matcher: *") {
		t.Errorf("empty matcher should render as '*', got:\n%s", result)
	}
}

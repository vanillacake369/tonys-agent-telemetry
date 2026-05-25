package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// updateApp is a test helper that runs a single Update cycle and asserts
// that the returned model is still an App (not a different type).
func updateApp(t *testing.T, a App, msg tea.Msg) (App, tea.Cmd) {
	t.Helper()
	model, cmd := a.Update(msg)
	result, ok := model.(App)
	if !ok {
		t.Fatalf("Update returned %T, want App", model)
	}
	return result, cmd
}

func TestApp_InitialTab(t *testing.T) {
	a := NewApp()
	if a.activeTab != TabSessions {
		t.Errorf("activeTab = %d, want %d (TabSessions)", a.activeTab, TabSessions)
	}
}

func TestApp_TabSwitchingByNumber(t *testing.T) {
	cases := []struct {
		name    string
		key     tea.KeyMsg
		wantTab Tab
	}{
		{"1 -> Sessions", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}, TabSessions},
		{"2 -> Skills", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}, TabSkills},
		{"3 -> Cost", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}, TabCost},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := NewApp()
			updated, _ := updateApp(t, a, tc.key)
			if updated.activeTab != tc.wantTab {
				t.Errorf("activeTab = %d, want %d", updated.activeTab, tc.wantTab)
			}
		})
	}
}

func TestApp_TabSwitchingRoundTrip(t *testing.T) {
	a := NewApp()

	// Navigate through all tabs and back using number keys.
	steps := []struct {
		key     tea.KeyMsg
		wantTab Tab
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}, TabSkills},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}, TabCost},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}, TabSessions},
	}

	for _, step := range steps {
		var cmd tea.Cmd
		a, cmd = updateApp(t, a, step.key)
		_ = cmd
		if a.activeTab != step.wantTab {
			t.Errorf("after key %v: activeTab = %d, want %d", step.key, a.activeTab, step.wantTab)
		}
	}
}

func TestApp_TabCyclingWithTabKey(t *testing.T) {
	a := NewApp()
	// Start at Sessions (0), Tab should advance forward.
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyTab})
	if a.activeTab != TabSkills {
		t.Errorf("Tab from Sessions: activeTab = %d, want %d (TabSkills)", a.activeTab, TabSkills)
	}
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyTab})
	if a.activeTab != TabCost {
		t.Errorf("Tab from Skills: activeTab = %d, want %d (TabCost)", a.activeTab, TabCost)
	}
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyTab})
	if a.activeTab != TabSessions {
		t.Errorf("Tab from Cost (wrap): activeTab = %d, want %d (TabSessions)", a.activeTab, TabSessions)
	}
}

func TestApp_TabCyclingWithShiftTabKey(t *testing.T) {
	a := NewApp()
	// Start at Sessions (0), Shift+Tab should go backward (wrap to Cost).
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyShiftTab})
	if a.activeTab != TabCost {
		t.Errorf("Shift+Tab from Sessions: activeTab = %d, want %d (TabCost)", a.activeTab, TabCost)
	}
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyShiftTab})
	if a.activeTab != TabSkills {
		t.Errorf("Shift+Tab from Cost: activeTab = %d, want %d (TabSkills)", a.activeTab, TabSkills)
	}
}

func TestApp_SearchFocusUnfocusCycle(t *testing.T) {
	a := NewApp()
	if a.searchFocused {
		t.Error("searchFocused should be false initially")
	}

	// "/" should focus search.
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !a.searchFocused {
		t.Error("searchFocused should be true after '/'")
	}

	// Esc should unfocus search.
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyEsc})
	if a.searchFocused {
		t.Error("searchFocused should be false after Esc")
	}
}

func TestApp_NumberKeysIgnoredWhenSearchFocused(t *testing.T) {
	a := NewApp()
	// Focus search.
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !a.searchFocused {
		t.Fatal("expected searchFocused = true")
	}

	// Pressing "2" while search is focused should NOT switch tabs.
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if a.activeTab != TabSessions {
		t.Errorf("tab should not switch when search is focused: activeTab = %d", a.activeTab)
	}
}

func TestApp_QuitOnQ(t *testing.T) {
	a := NewApp()
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	// Execute the command and verify it is the tea.Quit sentinel.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("cmd() returned %T, want tea.QuitMsg", msg)
	}
}

func TestApp_QuitOnCtrlC(t *testing.T) {
	a := NewApp()
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("cmd() returned %T, want tea.QuitMsg", msg)
	}
}

func TestApp_WindowResize(t *testing.T) {
	a := NewApp()
	updated, _ := updateApp(t, a, tea.WindowSizeMsg{Width: 120, Height: 40})
	if updated.width != 120 {
		t.Errorf("width = %d, want 120", updated.width)
	}
	if updated.height != 40 {
		t.Errorf("height = %d, want 40", updated.height)
	}
}

func TestApp_ContentHeightCalculation(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	want := 24 - tabBarHeight - statusBarHeight - outerBorderHeight
	if got := a.contentHeight(); got != want {
		t.Errorf("contentHeight = %d, want %d", got, want)
	}
}

func TestApp_ContentHeightMinimum(t *testing.T) {
	// Very small terminal — content height should not go negative.
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 10, Height: 1})
	if h := a.contentHeight(); h < 0 {
		t.Errorf("contentHeight = %d, want >= 0", h)
	}
}

func TestApp_ViewContainsTabNames(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := a.View()
	for _, name := range []string{"Sessions", "Skills", "Cost"} {
		if !strings.Contains(view, name) {
			t.Errorf("View() missing tab name %q", name)
		}
	}
}

func TestApp_ViewContainsNumberedTabPrefixes(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := a.View()
	for _, prefix := range []string{"1:Sessions", "2:Skills", "3:Cost"} {
		if !strings.Contains(view, prefix) {
			t.Errorf("View() missing numbered tab prefix %q", prefix)
		}
	}
}

func TestApp_ViewContainsStatusBar(t *testing.T) {
	// Use a wide terminal to ensure the full status bar including q:quit is visible.
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 150, Height: 24})
	view := a.View()
	if !strings.Contains(view, "q quit") {
		t.Errorf("View() missing status bar hint 'q quit'")
	}
}

func TestApp_ViewStatusBarNormalMode(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 120, Height: 24})
	view := a.View()
	if !strings.Contains(view, "NORMAL") {
		t.Errorf("View() in normal mode should contain 'NORMAL', got view snippet: %s",
			view[len(view)-200:])
	}
}

func TestApp_ViewStatusBarSearchMode(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 120, Height: 24})
	// Focus search.
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	view := a.View()
	if !strings.Contains(view, "SEARCH") {
		t.Errorf("View() in search mode should contain 'SEARCH'")
	}
}

func TestApp_ViewNormalMode_SearchFocusedFalse(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 120, Height: 24})
	if a.searchFocused {
		t.Fatal("searchFocused should be false in normal mode")
	}
	// View() should not panic and should contain NORMAL.
	view := a.View()
	if !strings.Contains(view, "NORMAL") {
		t.Errorf("View() in normal mode should contain 'NORMAL'")
	}
}

func TestApp_ViewSearchMode_SearchFocusedTrue(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 120, Height: 24})
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !a.searchFocused {
		t.Fatal("searchFocused should be true after '/'")
	}
	// View() should not panic and should contain SEARCH.
	view := a.View()
	if !strings.Contains(view, "SEARCH") {
		t.Errorf("View() in search mode should contain 'SEARCH'")
	}
	// NORMAL should not appear in the status bar when in search mode.
	if strings.Contains(view, "NORMAL") {
		t.Errorf("View() in search mode should not contain 'NORMAL'")
	}
}

func TestApp_SkillsTabHints_ContainsOpen(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	hints := a.tabHints()
	if !strings.Contains(hints, "o:open") {
		t.Errorf("skills tab hints should contain 'o:open', got %q", hints)
	}
}

func TestApp_InitReturnsCmd(t *testing.T) {
	a := NewApp()
	// Init should not panic and may return nil.
	_ = a.Init()
}

func TestApp_TabSwitchDoesNotLoseSize(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if a.width != 100 || a.height != 30 {
		t.Errorf("size lost after tab switch: width=%d height=%d", a.width, a.height)
	}
}

func TestApp_FIFOCancelCalledOnQuitMsg(t *testing.T) {
	a := NewApp()
	// fifoCtx must be set by NewApp.
	if a.fifoCtx == nil {
		t.Fatal("fifoCtx should be non-nil after NewApp()")
	}
	if a.fifoCancel == nil {
		t.Fatal("fifoCancel should be non-nil after NewApp()")
	}

	// Verify context is not yet cancelled.
	if a.fifoCtx.Err() != nil {
		t.Fatal("fifoCtx should not be cancelled before QuitMsg")
	}

	// Capture the context before update so we can check it after.
	ctx := a.fifoCtx

	// Deliver a QuitMsg — fifoCancel must be invoked.
	_, _ = a.Update(tea.QuitMsg{})

	if ctx.Err() != context.Canceled {
		t.Error("fifoCancel was not called on tea.QuitMsg")
	}
}

func TestApp_CancelFIFO_CancelsContext(t *testing.T) {
	a := NewApp()
	ctx := a.fifoCtx

	if ctx.Err() != nil {
		t.Fatal("fifoCtx should not be cancelled before CancelFIFO()")
	}

	a.CancelFIFO()

	if ctx.Err() != context.Canceled {
		t.Error("CancelFIFO() did not cancel fifoCtx")
	}
}

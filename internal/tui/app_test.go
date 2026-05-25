package tui

import (
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
	// Tab key cycles through tabOrder forward, wrapping back to Sessions.
	// Sessions → Skills → Cost → Hooks → DAG → Control → Trends → Sessions.
	expected := []Tab{TabSkills, TabCost, TabHooks, TabDAG, TabControl, TabTrends, TabSessions}
	for i, want := range expected {
		a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyTab})
		if a.activeTab != want {
			t.Errorf("step %d: activeTab = %d, want %d", i+1, a.activeTab, want)
		}
	}
}

func TestApp_TabCyclingWithShiftTabKey(t *testing.T) {
	a := NewApp()
	// Shift+Tab cycles through tabOrder backward, wrapping forward to Sessions.
	// Sessions ← Trends ← Control ← DAG ← Hooks ← Cost ← Skills ← Sessions.
	expected := []Tab{TabTrends, TabControl, TabDAG, TabHooks, TabCost, TabSkills, TabSessions}
	for i, want := range expected {
		a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyShiftTab})
		if a.activeTab != want {
			t.Errorf("step %d: activeTab = %d, want %d", i+1, a.activeTab, want)
		}
	}
}

// TestApp_TabCyclingCoversAllTabs asserts that pressing Tab len(tabOrder)
// times visits every tab exactly once and returns to the starting tab.
// Regression guard for QA finding V-1 (tab cycling silently skipping new tabs).
func TestApp_TabCyclingCoversAllTabs(t *testing.T) {
	a := NewApp()
	seen := map[Tab]bool{a.activeTab: true}
	for i := 0; i < len(tabOrder); i++ {
		a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyTab})
		seen[a.activeTab] = true
	}
	if len(seen) != len(tabOrder) {
		t.Errorf("Tab cycling covered %d distinct tabs, want %d (one per tabOrder entry)", len(seen), len(tabOrder))
	}
	if a.activeTab != TabSessions {
		t.Errorf("after %d Tab presses, activeTab = %d, want TabSessions (%d)", len(tabOrder), a.activeTab, TabSessions)
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
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 120, Height: 24})
	view := a.View()
	for _, name := range []string{"Sessions", "Skills", "Cost", "Hooks", "DAG", "Control"} {
		if !strings.Contains(view, name) {
			t.Errorf("View() missing tab name %q", name)
		}
	}
}

func TestApp_ViewContainsNumberedTabPrefixes(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 120, Height: 24})
	view := a.View()
	for _, prefix := range []string{"1:Sessions", "2:Skills", "3:Cost", "4:Hooks", "5:DAG", "^G:Control"} {
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

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

func TestApp_TabSwitching(t *testing.T) {
	cases := []struct {
		name    string
		key     tea.KeyMsg
		wantTab Tab
	}{
		{"ctrl+s -> Sessions", tea.KeyMsg{Type: tea.KeyCtrlS}, TabSessions},
		{"ctrl+a -> Agents", tea.KeyMsg{Type: tea.KeyCtrlA}, TabAgents},
		{"ctrl+d -> DAG", tea.KeyMsg{Type: tea.KeyCtrlD}, TabDAG},
		{"ctrl+k -> Skills", tea.KeyMsg{Type: tea.KeyCtrlK}, TabSkills},
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

	// Navigate through all tabs and back.
	steps := []struct {
		key     tea.KeyMsg
		wantTab Tab
	}{
		{tea.KeyMsg{Type: tea.KeyCtrlA}, TabAgents},
		{tea.KeyMsg{Type: tea.KeyCtrlD}, TabDAG},
		{tea.KeyMsg{Type: tea.KeyCtrlK}, TabSkills},
		{tea.KeyMsg{Type: tea.KeyCtrlS}, TabSessions},
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
	want := 24 - tabBarHeight - statusBarHeight
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
	for _, name := range []string{"Sessions", "Agents", "DAG", "Skills"} {
		if !strings.Contains(view, name) {
			t.Errorf("View() missing tab name %q", name)
		}
	}
}

func TestApp_ViewContainsStatusBar(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := a.View()
	if !strings.Contains(view, "q:quit") {
		t.Errorf("View() missing status bar hint 'q:quit'")
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
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyCtrlA})
	if a.width != 100 || a.height != 30 {
		t.Errorf("size lost after tab switch: width=%d height=%d", a.width, a.height)
	}
}

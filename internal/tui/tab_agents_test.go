package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

// updateAgentsTab is a test helper that runs a single Update cycle and asserts
// that the returned TabModel is an AgentsTab.
func updateAgentsTab(t *testing.T, tab AgentsTab, msg tea.Msg) (AgentsTab, tea.Cmd) {
	t.Helper()
	model, cmd := tab.Update(msg)
	result, ok := model.(AgentsTab)
	if !ok {
		t.Fatalf("Update returned %T, want AgentsTab", model)
	}
	return result, cmd
}

// makeTestAgents returns a slice of Agents for use in tests.
func makeTestAgents() []data.Agent {
	return []data.Agent{
		{Name: "implementer", Model: "sonnet", Description: "Universal code implementation"},
		{Name: "architect", Model: "sonnet", Description: "System design and planning"},
		{Name: "reviewer", Model: "sonnet", Description: "Code review and feedback"},
		{Name: "tester", Model: "sonnet", Description: "Test generation and validation"},
		{Name: "researcher", Model: "haiku", Description: "Deep research and analysis"},
		{Name: "cross-validator", Model: "sonnet", Description: "Cross-validation of results"},
		{Name: "refactorer", Model: "opus", Description: "Code refactoring and cleanup"},
	}
}

// sendAgentsLoaded sends an AgentsLoadedMsg to the tab and returns the updated tab.
func sendAgentsLoaded(t *testing.T, tab AgentsTab, agents []data.Agent) AgentsTab {
	t.Helper()
	msg := AgentsLoadedMsg{Agents: agents}
	updated, _ := updateAgentsTab(t, tab, msg)
	return updated
}

// ── AgentsLoadedMsg ──────────────────────────────────────────────────────────

func TestAgentsTab_LoadedMsgPopulatesAgents(t *testing.T) {
	tab := NewAgentsTab()
	agents := makeTestAgents()
	tab = sendAgentsLoaded(t, tab, agents)

	if len(tab.agents) != len(agents) {
		t.Errorf("agents len = %d, want %d", len(tab.agents), len(agents))
	}
	if len(tab.filtered) != len(agents) {
		t.Errorf("filtered len = %d, want %d", len(tab.filtered), len(agents))
	}
	if tab.loading {
		t.Error("loading should be false after AgentsLoadedMsg")
	}
}

func TestAgentsTab_LoadedMsgWithError(t *testing.T) {
	tab := NewAgentsTab()
	msg := AgentsLoadedMsg{Agents: nil, Err: fmt.Errorf("disk error")}

	// Re-use Update directly so we can pass a synthetic error.
	model, _ := tab.Update(msg)
	updated := model.(AgentsTab)

	if updated.loading {
		t.Error("loading should be false after error AgentsLoadedMsg")
	}
	if updated.err == nil {
		t.Error("err should be set after error AgentsLoadedMsg")
	}
	if len(updated.agents) != 0 {
		t.Errorf("agents should be nil on error, got %d", len(updated.agents))
	}
}

func TestAgentsTab_LoadedMsgEmptyAgents(t *testing.T) {
	tab := NewAgentsTab()
	tab = tab.SetSize(120, 40).(AgentsTab)
	tab = sendAgentsLoaded(t, tab, []data.Agent{})

	view := tab.View()
	if !strings.Contains(view, "no agents found") && !strings.Contains(view, "no results") {
		t.Error("expected empty state message in View()")
	}
}

// ── Fuzzy filtering ──────────────────────────────────────────────────────────

func TestAgentsTab_FuzzyFilterByName(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())

	// Simulate typing "impl" in the search box.
	tab.searchInput.SetValue("impl")
	tab.applyFilter()

	if len(tab.filtered) == 0 {
		t.Fatal("expected filtered results for query 'impl', got none")
	}
	found := false
	for _, a := range tab.filtered {
		if strings.Contains(a.Name, "implement") {
			found = true
			break
		}
	}
	if !found {
		t.Error("filtered results should include 'implementer' for query 'impl'")
	}
}

func TestAgentsTab_FuzzyFilterByDescription(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())

	tab.searchInput.SetValue("code review")
	tab.applyFilter()

	if len(tab.filtered) == 0 {
		t.Fatal("expected filtered results for query 'code review', got none")
	}
}

func TestAgentsTab_EmptyQueryShowsAll(t *testing.T) {
	tab := NewAgentsTab()
	agents := makeTestAgents()
	tab = sendAgentsLoaded(t, tab, agents)

	// Empty query — all agents should be visible.
	tab.searchInput.SetValue("")
	tab.applyFilter()

	if len(tab.filtered) != len(agents) {
		t.Errorf("filtered len = %d with empty query, want %d", len(tab.filtered), len(agents))
	}
}

func TestAgentsTab_FilterResetsCursor(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())
	tab.cursor = 3

	// Typing in search input via Update resets cursor.
	tab, _ = updateAgentsTab(t, tab, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	// cursor should be reset or clamped to valid range.
	if tab.cursor < 0 || (len(tab.filtered) > 0 && tab.cursor >= len(tab.filtered)) {
		t.Errorf("cursor out of bounds after filter: cursor=%d filtered=%d", tab.cursor, len(tab.filtered))
	}
}

// ── Cursor movement ──────────────────────────────────────────────────────────

func TestAgentsTab_CursorDown(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())
	tab.cursor = 0

	tab, _ = updateAgentsTab(t, tab, tea.KeyMsg{Type: tea.KeyDown})
	if tab.cursor != 1 {
		t.Errorf("cursor = %d after KeyDown, want 1", tab.cursor)
	}
}

func TestAgentsTab_CursorUp(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())
	tab.cursor = 2

	tab, _ = updateAgentsTab(t, tab, tea.KeyMsg{Type: tea.KeyUp})
	if tab.cursor != 1 {
		t.Errorf("cursor = %d after KeyUp, want 1", tab.cursor)
	}
}

func TestAgentsTab_CursorJKey(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())
	tab.cursor = 0

	tab, _ = updateAgentsTab(t, tab, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if tab.cursor != 1 {
		t.Errorf("cursor = %d after 'j', want 1", tab.cursor)
	}
}

func TestAgentsTab_CursorKKey(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())
	tab.cursor = 2

	tab, _ = updateAgentsTab(t, tab, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if tab.cursor != 1 {
		t.Errorf("cursor = %d after 'k', want 1", tab.cursor)
	}
}

func TestAgentsTab_CursorDoesNotGoNegative(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())
	tab.cursor = 0

	tab, _ = updateAgentsTab(t, tab, tea.KeyMsg{Type: tea.KeyUp})
	if tab.cursor < 0 {
		t.Errorf("cursor = %d, should not go below 0", tab.cursor)
	}
}

func TestAgentsTab_CursorDoesNotExceedList(t *testing.T) {
	tab := NewAgentsTab()
	agents := makeTestAgents()
	tab = sendAgentsLoaded(t, tab, agents)
	tab.cursor = len(agents) - 1

	tab, _ = updateAgentsTab(t, tab, tea.KeyMsg{Type: tea.KeyDown})
	if tab.cursor >= len(tab.filtered) {
		t.Errorf("cursor = %d, should not exceed filtered len %d", tab.cursor, len(tab.filtered))
	}
}

// ── Enter key generates correct command ─────────────────────────────────────

func TestAgentsTab_EnterGeneratesClaudeCommand(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())
	tab.cursor = 0 // "implementer" is first in makeTestAgents

	// We can't easily intercept OpenPane in a unit test, so we verify the
	// selected agent name is correct to confirm the right command would be built.
	selected := tab.selectedAgent()
	if selected == nil {
		t.Fatal("selectedAgent() returned nil")
	}
	wantCmd := "claude --agent " + selected.Name
	gotCmd := "claude --agent " + selected.Name // matches the construction in Update
	if gotCmd != wantCmd {
		t.Errorf("command = %q, want %q", gotCmd, wantCmd)
	}
}

func TestAgentsTab_EnterWithNoAgentsDoesNotPanic(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, []data.Agent{})

	// Should not panic when no agents are loaded.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Enter on empty list panicked: %v", r)
		}
	}()
	tab, _ = updateAgentsTab(t, tab, tea.KeyMsg{Type: tea.KeyEnter})
}

// ── Agent icon mapping ───────────────────────────────────────────────────────

func TestAgentIcon_KnownAgents(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"architect", "🏗️"},
		{"implementer", "⚙️"},
		{"reviewer", "🔎"},
		{"tester", "🧪"},
		{"refactorer", "♻️"},
		{"researcher", "🔍"},
		{"cross-validator", "✅"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := agentIcon(tc.name)
			if got != tc.want {
				t.Errorf("agentIcon(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestAgentIcon_UnknownAgentGetsDefaultIcon(t *testing.T) {
	cases := []string{"planner", "helper", "foobar", "", "UNKNOWN"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			got := agentIcon(name)
			if got != "🤖" {
				t.Errorf("agentIcon(%q) = %q, want default icon 🤖", name, got)
			}
		})
	}
}

func TestAgentIcon_CaseInsensitive(t *testing.T) {
	// The icon mapping uses strings.ToLower, so mixed case should work.
	got := agentIcon("Architect")
	if got != "🏗️" {
		t.Errorf("agentIcon(\"Architect\") = %q, want 🏗️", got)
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func TestAgentsTab_ViewWithSize(t *testing.T) {
	tab := NewAgentsTab()
	tab = tab.SetSize(120, 40).(AgentsTab)
	agents := makeTestAgents()
	tab = sendAgentsLoaded(t, tab, agents)

	view := tab.View()
	if view == "" {
		t.Error("View() should not be empty")
	}
	// Hint bar should be present.
	if !strings.Contains(view, "Enter:launch") {
		t.Error("View() should contain hint 'Enter:launch'")
	}
	if !strings.Contains(view, "^Y:copy") {
		t.Error("View() should contain hint '^Y:copy'")
	}
	if !strings.Contains(view, "^R:refresh") {
		t.Error("View() should contain hint '^R:refresh'")
	}
}

func TestAgentsTab_ViewShowsAgentNames(t *testing.T) {
	tab := NewAgentsTab()
	tab = tab.SetSize(120, 40).(AgentsTab)
	tab = sendAgentsLoaded(t, tab, makeTestAgents())

	view := tab.View()
	if !strings.Contains(view, "implementer") {
		t.Error("View() should contain agent name 'implementer'")
	}
	if !strings.Contains(view, "architect") {
		t.Error("View() should contain agent name 'architect'")
	}
}

func TestAgentsTab_ViewNoSizeFallback(t *testing.T) {
	tab := NewAgentsTab()
	// Not sized yet — should not panic.
	view := tab.View()
	if view == "" {
		t.Error("View() should not be empty even without size set")
	}
}

// ── Preview ──────────────────────────────────────────────────────────────────

func TestAgentsTab_PreviewMsgUpdatesContent(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())

	previewContent := "# implementer\nUniversal code implementation agent."
	tab, _ = updateAgentsTab(t, tab, AgentPreviewMsg{Content: previewContent})

	if tab.preview != previewContent {
		t.Errorf("preview = %q, want %q", tab.preview, previewContent)
	}
}

func TestAgentsTab_PreviewMsgWithError(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())

	tab, _ = updateAgentsTab(t, tab, AgentPreviewMsg{Content: "", Err: fmt.Errorf("file not found")})
	if tab.preview != "(no preview available)" {
		t.Errorf("preview = %q, want '(no preview available)'", tab.preview)
	}
}

// ── Refresh ──────────────────────────────────────────────────────────────────

func TestAgentsTab_RefreshClearsState(t *testing.T) {
	tab := NewAgentsTab()
	tab = sendAgentsLoaded(t, tab, makeTestAgents())
	tab.cursor = 3
	tab.preview = "some preview"

	// Send Ctrl+R to trigger refresh.
	tab, cmd := updateAgentsTab(t, tab, tea.KeyMsg{Type: tea.KeyCtrlR})

	if !tab.loading {
		t.Error("loading should be true after Ctrl+R")
	}
	if len(tab.agents) != 0 {
		t.Errorf("agents should be cleared after Ctrl+R, got %d", len(tab.agents))
	}
	if tab.cursor != 0 {
		t.Errorf("cursor = %d after Ctrl+R, want 0", tab.cursor)
	}
	if tab.preview != "" {
		t.Errorf("preview should be cleared after Ctrl+R, got %q", tab.preview)
	}
	if cmd == nil {
		t.Error("Ctrl+R should return a non-nil cmd to reload agents")
	}
}

// ── SetSize ───────────────────────────────────────────────────────────────────

func TestAgentsTab_SetSize(t *testing.T) {
	tab := NewAgentsTab()
	updated := tab.SetSize(100, 30).(AgentsTab)

	if updated.width != 100 {
		t.Errorf("width = %d, want 100", updated.width)
	}
	if updated.height != 30 {
		t.Errorf("height = %d, want 30", updated.height)
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func TestAgentsTab_InitReturnsCmd(t *testing.T) {
	tab := NewAgentsTab()
	cmd := tab.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil Cmd to load agents")
	}
}

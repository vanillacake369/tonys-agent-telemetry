package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

// updateDAGTab is a test helper that runs a single Update cycle and asserts
// that the returned TabModel is a DAGTab.
func updateDAGTab(t *testing.T, tab DAGTab, msg tea.Msg) (DAGTab, tea.Cmd) {
	t.Helper()
	model, cmd := tab.Update(msg)
	result, ok := model.(DAGTab)
	if !ok {
		t.Fatalf("Update returned %T, want DAGTab", model)
	}
	return result, cmd
}

// makeTestSessions returns a slice of Sessions for use in tests.
func makeTestSessions() []data.Session {
	return []data.Session{
		{
			ID:          "session-aaa",
			FilePath:    "/tmp/sessions/aaa/session-aaa.jsonl",
			Timestamp:   time.Now(),
			FirstPrompt: "First session",
			Model:       "claude-opus-4-6",
		},
		{
			ID:          "session-bbb",
			FilePath:    "/tmp/sessions/bbb/session-bbb.jsonl",
			Timestamp:   time.Now().Add(-time.Hour),
			FirstPrompt: "Second session",
			Model:       "claude-sonnet-4-6",
		},
	}
}

// makeTestDAG returns a DAGNode tree for use in tests.
func makeTestDAG() *data.DAGNode {
	return &data.DAGNode{
		ID:         "root",
		AgentType:  "architect",
		Status:     "done",
		TokenCount: 5000,
		Children: []*data.DAGNode{
			{
				ID:         "child1",
				AgentType:  "researcher",
				Status:     "done",
				TokenCount: 15200,
				Tools:      []string{"WebFetch", "Grep", "Read"},
			},
			{
				ID:         "child2",
				AgentType:  "implementer",
				Status:     "running",
				TokenCount: 8400,
				Tools:      []string{"Edit", "Write", "Bash"},
			},
		},
	}
}

// ── DAGSessionsLoadedMsg ──────────────────────────────────────────────────────

func TestDAGTab_SessionsLoadedMsg_PopulatesSessions(t *testing.T) {
	tab := NewDAGTab()
	sessions := makeTestSessions()

	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: sessions})

	if len(tab.sessions) != len(sessions) {
		t.Errorf("sessions len = %d, want %d", len(tab.sessions), len(sessions))
	}
	if tab.loading {
		t.Error("loading should be false after DAGSessionsLoadedMsg")
	}
	if tab.selectedIdx != 0 {
		t.Errorf("selectedIdx = %d, want 0 (most recent)", tab.selectedIdx)
	}
}

func TestDAGTab_SessionsLoadedMsg_WithError(t *testing.T) {
	tab := NewDAGTab()
	msg := DAGSessionsLoadedMsg{Sessions: nil, Err: fmt.Errorf("disk error")}

	tab, _ = updateDAGTab(t, tab, msg)

	if tab.loading {
		t.Error("loading should be false after error DAGSessionsLoadedMsg")
	}
	if tab.err == nil {
		t.Error("err should be set after error DAGSessionsLoadedMsg")
	}
}

func TestDAGTab_SessionsLoadedMsg_EmptySessions(t *testing.T) {
	tab := NewDAGTab()
	tab = tab.SetSize(120, 40).(DAGTab)

	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: []data.Session{}})

	view := tab.View()
	if !strings.Contains(view, "No sessions found") {
		t.Errorf("expected 'No sessions found' in View(), got:\n%s", view)
	}
}

// ── DAGLoadedMsg ──────────────────────────────────────────────────────────────

func TestDAGTab_DAGLoadedMsg_PopulatesDAG(t *testing.T) {
	tab := NewDAGTab()
	tab = tab.SetSize(120, 40).(DAGTab)
	dag := makeTestDAG()

	// First send sessions so we have a valid state.
	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: makeTestSessions()})
	// Then send the DAG.
	tab, _ = updateDAGTab(t, tab, DAGLoadedMsg{DAG: dag})

	if tab.dag == nil {
		t.Fatal("dag should be populated after DAGLoadedMsg")
	}
	if tab.dag.ID != "root" {
		t.Errorf("dag.ID = %q, want 'root'", tab.dag.ID)
	}
}

func TestDAGTab_DAGLoadedMsg_WithError(t *testing.T) {
	tab := NewDAGTab()
	tab = tab.SetSize(120, 40).(DAGTab)

	tab, _ = updateDAGTab(t, tab, DAGLoadedMsg{DAG: nil, Err: fmt.Errorf("parse error")})

	if tab.err == nil {
		t.Error("err should be set after error DAGLoadedMsg")
	}
}

// ── Session switching with left/right ─────────────────────────────────────────

func TestDAGTab_RightArrow_AdvancesSession(t *testing.T) {
	tab := NewDAGTab()
	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: makeTestSessions()})
	tab.selectedIdx = 0

	tab, cmd := updateDAGTab(t, tab, tea.KeyMsg{Type: tea.KeyRight})
	if tab.selectedIdx != 1 {
		t.Errorf("selectedIdx = %d after KeyRight, want 1", tab.selectedIdx)
	}
	if cmd == nil {
		t.Error("KeyRight should return a cmd to load DAG")
	}
}

func TestDAGTab_LeftArrow_RetreatsSession(t *testing.T) {
	tab := NewDAGTab()
	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: makeTestSessions()})
	tab.selectedIdx = 1

	tab, cmd := updateDAGTab(t, tab, tea.KeyMsg{Type: tea.KeyLeft})
	if tab.selectedIdx != 0 {
		t.Errorf("selectedIdx = %d after KeyLeft, want 0", tab.selectedIdx)
	}
	if cmd == nil {
		t.Error("KeyLeft should return a cmd to load DAG")
	}
}

func TestDAGTab_RightArrow_DoesNotExceedSessions(t *testing.T) {
	tab := NewDAGTab()
	sessions := makeTestSessions()
	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: sessions})
	tab.selectedIdx = len(sessions) - 1

	tab, _ = updateDAGTab(t, tab, tea.KeyMsg{Type: tea.KeyRight})
	if tab.selectedIdx >= len(sessions) {
		t.Errorf("selectedIdx = %d should not exceed sessions len %d", tab.selectedIdx, len(sessions))
	}
}

func TestDAGTab_LeftArrow_DoesNotGoBelowZero(t *testing.T) {
	tab := NewDAGTab()
	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: makeTestSessions()})
	tab.selectedIdx = 0

	tab, _ = updateDAGTab(t, tab, tea.KeyMsg{Type: tea.KeyLeft})
	if tab.selectedIdx < 0 {
		t.Errorf("selectedIdx = %d, should not go below 0", tab.selectedIdx)
	}
}

func TestDAGTab_HKey_EquivalentToLeftArrow(t *testing.T) {
	tab := NewDAGTab()
	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: makeTestSessions()})
	tab.selectedIdx = 1

	tab, _ = updateDAGTab(t, tab, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if tab.selectedIdx != 0 {
		t.Errorf("selectedIdx = %d after 'h', want 0", tab.selectedIdx)
	}
}

func TestDAGTab_LKey_EquivalentToRightArrow(t *testing.T) {
	tab := NewDAGTab()
	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: makeTestSessions()})
	tab.selectedIdx = 0

	tab, _ = updateDAGTab(t, tab, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if tab.selectedIdx != 1 {
		t.Errorf("selectedIdx = %d after 'l', want 1", tab.selectedIdx)
	}
}

// ── SetSize ───────────────────────────────────────────────────────────────────

func TestDAGTab_SetSize_UpdatesDimensions(t *testing.T) {
	tab := NewDAGTab()
	updated := tab.SetSize(100, 30).(DAGTab)

	if updated.width != 100 {
		t.Errorf("width = %d, want 100", updated.width)
	}
	if updated.height != 30 {
		t.Errorf("height = %d, want 30", updated.height)
	}
}

func TestDAGTab_SetSize_UpdatesViewport(t *testing.T) {
	tab := NewDAGTab()
	updated := tab.SetSize(100, 30).(DAGTab)

	// Viewport width = total - 2 (panel left+right border)
	if updated.viewport.Width != 98 {
		t.Errorf("viewport.Width = %d, want 98", updated.viewport.Width)
	}
	// Viewport height = total - 4 (header+gap+stats+gap) - 2 (panel top+bottom border)
	wantVpHeight := 30 - 4 - 2
	if updated.viewport.Height != wantVpHeight {
		t.Errorf("viewport.Height = %d, want %d", updated.viewport.Height, wantVpHeight)
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func TestDAGTab_InitReturnsCmd(t *testing.T) {
	tab := NewDAGTab()
	cmd := tab.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil Cmd to load sessions")
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func TestDAGTab_ViewLoadingState(t *testing.T) {
	tab := NewDAGTab()
	// loading is true by default.
	view := tab.View()
	if !strings.Contains(view, "Loading") {
		t.Errorf("View() in loading state should contain 'Loading', got:\n%s", view)
	}
}

func TestDAGTab_ViewWithDAG(t *testing.T) {
	tab := NewDAGTab()
	tab = tab.SetSize(120, 40).(DAGTab)
	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: makeTestSessions()})
	tab, _ = updateDAGTab(t, tab, DAGLoadedMsg{DAG: makeTestDAG()})

	view := tab.View()
	if view == "" {
		t.Error("View() should not be empty after loading DAG")
	}
	// Should contain agent icons from the DAG.
	if !strings.Contains(view, "🏗️") && !strings.Contains(view, "🔍") && !strings.Contains(view, "⚙️") {
		t.Errorf("View() should contain agent icons, got:\n%s", view)
	}
}

func TestDAGTab_ViewEmptySessions_ShowsMessage(t *testing.T) {
	tab := NewDAGTab()
	tab = tab.SetSize(120, 40).(DAGTab)
	tab, _ = updateDAGTab(t, tab, DAGSessionsLoadedMsg{Sessions: []data.Session{}})

	view := tab.View()
	if !strings.Contains(view, "No sessions found") {
		t.Errorf("View() should show 'No sessions found', got:\n%s", view)
	}
}

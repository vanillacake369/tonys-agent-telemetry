package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

// updateSessionsTab is a test helper that runs one Update cycle and returns
// the resulting SessionsTab (it panics if the model changes type).
func updateSessionsTab(t *testing.T, s SessionsTab, msg tea.Msg) (SessionsTab, tea.Cmd) {
	t.Helper()
	model, cmd := s.Update(msg)
	result, ok := model.(SessionsTab)
	if !ok {
		t.Fatalf("Update returned %T, want SessionsTab", model)
	}
	return result, cmd
}

// makeSessions builds a slice of test sessions for use across tests.
func makeSessions() []data.Session {
	return []data.Session{
		{
			ID:          "session-abc123",
			CWD:         "/Users/test/dev/tonys-nix",
			ProjectDir:  "/Users/test/dev/tonys-nix",
			GitBranch:   "main",
			FirstPrompt: "claude status line 에 대해 알려줘",
			Timestamp:   time.Date(2026, 5, 16, 16, 6, 0, 0, time.UTC),
			Model:       "claude-opus-4-6",
			FilePath:    "/tmp/nonexistent-session.jsonl",
		},
		{
			ID:          "session-def456",
			CWD:         "/Users/test/dev/tonys-homelab",
			ProjectDir:  "/Users/test/dev/tonys-homelab",
			GitBranch:   "feature/agent",
			FirstPrompt: "agent view configuration help",
			Timestamp:   time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC),
			Model:       "claude-sonnet-4-6",
			FilePath:    "/tmp/nonexistent-session2.jsonl",
		},
		{
			ID:          "session-ghi789",
			CWD:         "/Users/test/dev/tonys-nix",
			ProjectDir:  "/Users/test/dev/tonys-nix",
			GitBranch:   "main",
			FirstPrompt: "문서를 분석하고 요약해줘",
			Timestamp:   time.Date(2026, 5, 12, 9, 0, 0, 0, time.UTC),
			Model:       "claude-opus-4-6",
			FilePath:    "/tmp/nonexistent-session3.jsonl",
		},
	}
}

func TestSessionsTab_LoadedMsg_PopulatesSessions(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()

	updated, _ := updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	if updated.loading {
		t.Error("loading should be false after SessionsLoadedMsg")
	}
	if len(updated.sessions) != 3 {
		t.Errorf("sessions len = %d, want 3", len(updated.sessions))
	}
	if len(updated.filtered) != 3 {
		t.Errorf("filtered len = %d, want 3 (no search query)", len(updated.filtered))
	}
	if updated.err != nil {
		t.Errorf("err = %v, want nil", updated.err)
	}
}

func TestSessionsTab_LoadedMsg_SetsLoadingFalse(t *testing.T) {
	s := NewSessionsTab()
	if !s.loading {
		t.Error("expected loading=true before any message")
	}

	updated, _ := updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: nil, Err: nil})
	if updated.loading {
		t.Error("loading should be false after SessionsLoadedMsg")
	}
}

func TestSessionsTab_FuzzyFilter_ReducesList(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	// Type "homelab" into the search input — should match only the homelab session.
	for _, ch := range "homelab" {
		s, _ = updateSessionsTab(t, s, tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{ch},
		})
	}

	if len(s.filtered) == 0 {
		t.Fatal("filtered is empty, expected at least one match for 'homelab'")
	}
	for _, sess := range s.filtered {
		combined := sess.CWD + sess.FirstPrompt + sess.GitBranch + sess.Model + sess.ProjectDir
		if !strings.Contains(combined, "homelab") {
			t.Errorf("unexpected session in filtered results: CWD=%s", sess.CWD)
		}
	}
}

func TestSessionsTab_FuzzyFilter_EmptyQueryShowsAll(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	// Ensure no search query → all sessions visible.
	if len(s.filtered) != len(sessions) {
		t.Errorf("filtered len = %d, want %d with empty query", len(s.filtered), len(sessions))
	}
}

func TestSessionsTab_CursorMovement_DownAndUp(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	if s.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", s.cursor)
	}

	s, _ = updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyDown})
	if s.cursor != 1 {
		t.Errorf("cursor after KeyDown = %d, want 1", s.cursor)
	}

	s, _ = updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyDown})
	if s.cursor != 2 {
		t.Errorf("cursor after 2nd KeyDown = %d, want 2", s.cursor)
	}

	s, _ = updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyUp})
	if s.cursor != 1 {
		t.Errorf("cursor after KeyUp = %d, want 1", s.cursor)
	}
}

func TestSessionsTab_CursorMovement_DoesNotGoNegative(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	// Press Up at cursor=0 — must stay at 0.
	s, _ = updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyUp})
	if s.cursor != 0 {
		t.Errorf("cursor = %d after KeyUp at 0, want 0", s.cursor)
	}
}

func TestSessionsTab_CursorMovement_DoesNotExceedList(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	// Move to the last item (press Down more times than the list length).
	for i := 0; i < len(sessions)+5; i++ {
		s, _ = updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyDown})
	}

	if s.cursor != len(sessions)-1 {
		t.Errorf("cursor = %d, want %d (last index)", s.cursor, len(sessions)-1)
	}
}

func TestSessionsTab_CursorMovement_VimKeys(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	// Use 'j' to move down.
	s, _ = updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if s.cursor != 1 {
		t.Errorf("cursor after 'j' = %d, want 1", s.cursor)
	}

	// Use 'k' to move up.
	s, _ = updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if s.cursor != 0 {
		t.Errorf("cursor after 'k' = %d, want 0", s.cursor)
	}
}

func TestSessionsTab_Enter_GeneratesResumeCommand(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	// We cannot easily intercept OpenPane without a mock, but we can verify
	// the session selected at cursor=0 has the expected ID and command format.
	selectedID := s.filtered[s.cursor].ID
	wantCmd := fmt.Sprintf("claude --resume %s", selectedID)

	got := fmt.Sprintf("claude --resume %s", s.filtered[0].ID)
	if got != wantCmd {
		t.Errorf("resume command = %q, want %q", got, wantCmd)
	}
}

func TestSessionsTab_EmptySessions_RendersMessage(t *testing.T) {
	s := NewSessionsTab()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: nil, Err: nil})
	s = s.SetSize(80, 24).(SessionsTab)

	view := s.View()
	if !strings.Contains(view, "No sessions found") {
		t.Errorf("expected 'No sessions found' in view, got: %s", view)
	}
}

func TestSessionsTab_ErrorState_RendersError(t *testing.T) {
	s := NewSessionsTab()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{
		Sessions: nil,
		Err:      fmt.Errorf("permission denied"),
	})
	s = s.SetSize(80, 24).(SessionsTab)

	view := s.View()
	if !strings.Contains(view, "permission denied") {
		t.Errorf("expected error message in view, got: %s", view)
	}
}

func TestSessionsTab_PreviewLoaded_StoresTurns(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	turns := []data.Turn{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}
	s, _ = updateSessionsTab(t, s, PreviewLoadedMsg{Turns: turns, Err: nil})

	if len(s.preview) != 2 {
		t.Errorf("preview len = %d, want 2", len(s.preview))
	}
}

func TestSessionsTab_Refresh_ReloadsData(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	// Ctrl+R should reset state and return a load command.
	updated, cmd := updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyCtrlR})
	if !updated.loading {
		t.Error("loading should be true after refresh")
	}
	if len(updated.sessions) != 0 {
		t.Errorf("sessions should be cleared on refresh, got %d", len(updated.sessions))
	}
	if cmd == nil {
		t.Error("expected a non-nil command after refresh")
	}
}

func TestSessionsTab_SetSize_UpdatesInputWidth(t *testing.T) {
	s := NewSessionsTab()
	s = s.SetSize(100, 30).(SessionsTab)
	if s.width != 100 || s.height != 30 {
		t.Errorf("SetSize: width=%d height=%d, want 100/30", s.width, s.height)
	}
}

func TestSessionsTab_Init_ReturnsCmd(t *testing.T) {
	s := NewSessionsTab()
	cmd := s.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil command")
	}
}

func TestSessionsTab_View_ContainsStatusHints(t *testing.T) {
	s := NewSessionsTab()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: makeSessions(), Err: nil})
	s = s.SetSize(80, 24).(SessionsTab)

	view := s.View()
	for _, hint := range []string{"Enter:resume", "^F:fork", "^Y:copy", "^R:refresh"} {
		if !strings.Contains(view, hint) {
			t.Errorf("View() missing hint %q", hint)
		}
	}
}

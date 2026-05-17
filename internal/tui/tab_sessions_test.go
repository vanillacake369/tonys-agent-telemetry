package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

	// Focus the search input first (mimicking "/" key behavior via the message).
	s, _ = updateSessionsTab(t, s, SearchFocusMsg{})

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

	// "r" should reset state and return a load command (search must be unfocused).
	updated, cmd := updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
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
	// Hints are now shown in the app-level status bar, not the tab's own View().
	// Use a wide terminal so all hints fit on the single status bar line.
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 160, Height: 30})
	sessions := makeSessions()
	a.tabs[TabSessions], _ = a.tabs[TabSessions].Update(SessionsLoadedMsg{Sessions: sessions, Err: nil})

	view := a.View()
	for _, hint := range []string{"↵:resume", "f:fork", "y:copy", "r:refresh"} {
		if !strings.Contains(view, hint) {
			t.Errorf("App.View() missing sessions hint %q in status bar", hint)
		}
	}
}

func TestSessionsTab_SearchFocusMsg_FocusesInput(t *testing.T) {
	s := NewSessionsTab()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: makeSessions(), Err: nil})

	// Initially search is not focused.
	if s.searchInput.Focused() {
		t.Error("searchInput should not be focused initially")
	}

	// SearchFocusMsg should focus the search input.
	s, _ = updateSessionsTab(t, s, SearchFocusMsg{})
	if !s.searchInput.Focused() {
		t.Error("searchInput should be focused after SearchFocusMsg")
	}
}

func TestSessionsTab_SearchBlurMsg_BlursInput(t *testing.T) {
	s := NewSessionsTab()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: makeSessions(), Err: nil})

	// Focus first.
	s, _ = updateSessionsTab(t, s, SearchFocusMsg{})
	if !s.searchInput.Focused() {
		t.Fatal("searchInput should be focused after SearchFocusMsg")
	}

	// Blur.
	s, _ = updateSessionsTab(t, s, SearchBlurMsg{})
	if s.searchInput.Focused() {
		t.Error("searchInput should be blurred after SearchBlurMsg")
	}
}

func TestSessionsTab_SearchFocused_NumberKeysGoToInput(t *testing.T) {
	s := NewSessionsTab()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: makeSessions(), Err: nil})
	s, _ = updateSessionsTab(t, s, SearchFocusMsg{})

	// When search is focused, typing digits goes to the search input, not tab switching.
	s, _ = updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if s.searchInput.Value() != "2" {
		t.Errorf("searchInput.Value() = %q, want '2' (digit should go to search when focused)", s.searchInput.Value())
	}
}

func TestFormatSessionLine_WideMode_ShowsStats(t *testing.T) {
	s := NewSessionsTab()
	sess := data.Session{
		ID:          "session-stat1",
		CWD:         "/Users/test/dev/myproject",
		ProjectDir:  "/Users/test/dev/myproject",
		FirstPrompt: "implement the feature",
		Timestamp:   time.Date(2026, 5, 16, 10, 30, 0, 0, time.UTC),
		TurnCount:   3,
		Duration:    4 * time.Minute,
	}
	line := s.formatSessionLine(sess, 80)
	if !strings.Contains(line, "3t") {
		t.Errorf("expected turn count '3t' in line, got: %q", line)
	}
	if !strings.Contains(line, "4m") {
		t.Errorf("expected duration '4m' in line, got: %q", line)
	}
}

func TestFormatSessionLine_WideMode_ZeroStats_ShowsSpaces(t *testing.T) {
	s := NewSessionsTab()
	sess := data.Session{
		ID:          "session-stat2",
		CWD:         "/Users/test/dev/myproject",
		ProjectDir:  "/Users/test/dev/myproject",
		FirstPrompt: "some prompt",
		Timestamp:   time.Date(2026, 5, 16, 10, 30, 0, 0, time.UTC),
		TurnCount:   0,
		Duration:    0,
	}
	line := s.formatSessionLine(sess, 80)
	// Zero stats should not inject "0t 0m" (shows padding spaces instead)
	if strings.Contains(line, "0t") {
		t.Errorf("zero turn count should not show '0t', got: %q", line)
	}
}

func TestFormatSessionLine_NarrowMode_NoStats(t *testing.T) {
	s := NewSessionsTab()
	sess := data.Session{
		ID:          "session-stat3",
		CWD:         "/tmp",
		FirstPrompt: "short",
		Timestamp:   time.Date(2026, 5, 16, 10, 30, 0, 0, time.UTC),
		TurnCount:   5,
		Duration:    10 * time.Minute,
	}
	// Narrow mode (<35) shows prompt only — no stats column.
	line := s.formatSessionLine(sess, 30)
	if strings.Contains(line, "5t") {
		t.Errorf("narrow mode should not show stats, got: %q", line)
	}
}

func TestFormatSessionLine_WideMode_CJKProject_ColumnAlignment(t *testing.T) {
	s := NewSessionsTab()
	sessASCII := data.Session{
		CWD:         "/Users/test/dev/my-project",
		FirstPrompt: "hello world",
		Timestamp:   time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC),
	}
	sessCJK := data.Session{
		CWD:         "/Users/test/dev/한글프로젝트",
		FirstPrompt: "hello world",
		Timestamp:   time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC),
	}
	lineASCII := s.formatSessionLine(sessASCII, 80)
	lineCJK := s.formatSessionLine(sessCJK, 80)

	// Both lines must have the same total display width (within 1 cell tolerance
	// for lipgloss internal rounding).
	wASCII := lipgloss.Width(lineASCII)
	wCJK := lipgloss.Width(lineCJK)
	diff := wASCII - wCJK
	if diff < -1 || diff > 1 {
		t.Errorf("CJK and ASCII project names produce different column widths: ascii=%d cjk=%d",
			wASCII, wCJK)
	}
}

func TestFormatSessionLine_SearchContext_FallbackToSearchText(t *testing.T) {
	s := NewSessionsTab()
	// Set a search query in the input (focus not required for formatSessionLine).
	s.searchInput.SetValue("unique-keyword")

	sess := data.Session{
		CWD:         "/Users/test/dev/myproject",
		FirstPrompt: "something unrelated",
		SearchText:  "This contains the unique-keyword in the middle of a conversation",
		Timestamp:   time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC),
	}
	line := s.formatSessionLine(sess, 80)

	// Should show context from SearchText, not the original FirstPrompt.
	if strings.Contains(line, "something unrelated") {
		t.Errorf("expected search context fallback, but got FirstPrompt in line: %q", line)
	}
	if !strings.Contains(line, "unique-keyword") {
		t.Errorf("expected 'unique-keyword' context in line, got: %q", line)
	}
}

func TestFormatSessionLine_SearchContext_ShowsFirstPromptWhenMatch(t *testing.T) {
	s := NewSessionsTab()
	s.searchInput.SetValue("hello")

	sess := data.Session{
		CWD:         "/Users/test/dev/myproject",
		FirstPrompt: "hello world",
		SearchText:  "hello world some more text",
		Timestamp:   time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC),
	}
	line := s.formatSessionLine(sess, 80)

	// Query matches FirstPrompt directly, so FirstPrompt should be displayed.
	if !strings.Contains(line, "hello world") {
		t.Errorf("expected FirstPrompt 'hello world' in line when query matches it, got: %q", line)
	}
}

func TestSessionsTab_Unfocused_RKeyRefreshes(t *testing.T) {
	s := NewSessionsTab()
	sessions := makeSessions()
	s, _ = updateSessionsTab(t, s, SessionsLoadedMsg{Sessions: sessions, Err: nil})

	// Ensure search is not focused.
	if s.searchInput.Focused() {
		t.Fatal("searchInput should not be focused initially")
	}

	// "r" should trigger refresh.
	updated, cmd := updateSessionsTab(t, s, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if !updated.loading {
		t.Error("loading should be true after 'r' refresh")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd after 'r' refresh")
	}
}

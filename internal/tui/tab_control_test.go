package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/control"
)

// updateControlTab is a test helper that runs one Update cycle and returns
// the resulting ControlTab.
func updateControlTab(t *testing.T, ct ControlTab, msg tea.Msg) (ControlTab, tea.Cmd) {
	t.Helper()
	model, cmd := ct.Update(msg)
	result, ok := model.(ControlTab)
	if !ok {
		t.Fatalf("Update returned %T, want ControlTab", model)
	}
	return result, cmd
}

func TestControlTab_RendersBudgets(t *testing.T) {
	ct := NewControlTab()
	ct = ct.SetSize(120, 30).(ControlTab)

	today := time.Now().UTC()
	ct, _ = updateControlTab(t, ct, ControlRefreshMsg{
		Policy: control.Policy{
			Budget: control.BudgetPolicy{SessionMaxUSD: 5.0, DailyMaxUSD: 50.0, WarnAtFraction: 0.8},
		},
		Budgets: []control.Budget{
			{
				SessionID:    "abc123def456",
				InputTokens:  12400,
				OutputTokens: 8200,
				CostUSD:      0.18,
				UpdatedAt:    today,
			},
			{
				SessionID:    "b021xxxx",
				InputTokens:  45300,
				OutputTokens: 21000,
				CostUSD:      1.42,
				UpdatedAt:    today,
			},
		},
	})

	view := ct.View()

	if !strings.Contains(view, "abc123d") {
		t.Errorf("view missing session abc123d: %s", view)
	}
	if !strings.Contains(view, "b021") {
		t.Errorf("view missing session b021: %s", view)
	}
	if !strings.Contains(view, "0.18") {
		t.Errorf("view missing cost 0.18: %s", view)
	}
	if !strings.Contains(view, "1.42") {
		t.Errorf("view missing cost 1.42: %s", view)
	}
}

func TestControlTab_RendersDenials(t *testing.T) {
	ct := NewControlTab()
	ct = ct.SetSize(120, 30).(ControlTab)

	ct, _ = updateControlTab(t, ct, ControlRefreshMsg{
		Policy: control.DefaultPolicy(),
		Denials: []control.Denial{
			{
				Timestamp: time.Now(),
				SessionID: "sess-1",
				Tool:      "Bash:rm -rf /tmp/data",
				Reason:    "tool_denylisted",
				Detail:    "matched pattern Bash:rm -rf*",
			},
			{
				Timestamp: time.Now(),
				SessionID: "sess-2",
				Tool:      "WebFetch:evil.com",
				Reason:    "budget_exceeded",
				Detail:    "session cost $5.01 >= cap $5.00",
			},
		},
	})

	view := ct.View()

	if !strings.Contains(view, "BLOCKED") {
		t.Errorf("view missing BLOCKED label: %s", view)
	}
	if !strings.Contains(view, "tool_denylisted") {
		t.Errorf("view missing tool_denylisted reason: %s", view)
	}
	if !strings.Contains(view, "budget_exceeded") {
		t.Errorf("view missing budget_exceeded reason: %s", view)
	}
}

func TestControlTab_ReloadKeyRefreshes(t *testing.T) {
	ct := NewControlTab()
	ct.loading = false

	ct2, cmd := updateControlTab(t, ct, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if !ct2.loading {
		t.Error("loading should be true after 'r' refresh key")
	}
	if cmd == nil {
		t.Error("expected a non-nil command after refresh key")
	}
}

func TestControlTab_NoPolicyShowsFailOpen(t *testing.T) {
	ct := NewControlTab()
	ct = ct.SetSize(120, 30).(ControlTab)

	// Empty policy (zero values) means no caps loaded — should show fail-open notice.
	ct, _ = updateControlTab(t, ct, ControlRefreshMsg{
		Policy:  control.DefaultPolicy(), // all zeros
		Budgets: nil,
		Denials: nil,
	})

	view := ct.View()

	if !strings.Contains(view, "fail-open") {
		t.Errorf("view should contain 'fail-open' when no policy loaded: %s", view)
	}
}

func TestControlTab_Title(t *testing.T) {
	ct := NewControlTab()
	if ct.Title() != "Control" {
		t.Errorf("Title() = %q, want Control", ct.Title())
	}
}

func TestControlTab_Init_ReturnsCmd(t *testing.T) {
	ct := NewControlTab()
	cmd := ct.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil command")
	}
}

func TestControlTab_SetSize(t *testing.T) {
	ct := NewControlTab()
	ct2 := ct.SetSize(100, 40).(ControlTab)
	if ct2.width != 100 || ct2.height != 40 {
		t.Errorf("SetSize: width=%d height=%d, want 100/40", ct2.width, ct2.height)
	}
}

func TestApp_CtrlGSwitchesToControl(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 120, Height: 30})

	// Ctrl+G should switch to Control tab.
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyCtrlG})
	if a.activeTab != TabControl {
		t.Errorf("activeTab = %d, want %d (TabControl) after Ctrl+G", a.activeTab, TabControl)
	}
}

func TestControlTab_ProgressBar(t *testing.T) {
	cases := []struct {
		fraction float64
		contains string
	}{
		{0.0, "░"},
		{1.0, "█"},
		{0.5, "█"},
	}
	for _, tc := range cases {
		bar := renderProgressBar(tc.fraction, 10)
		if !strings.Contains(bar, tc.contains) {
			t.Errorf("renderProgressBar(%v, 10) = %q, want %q", tc.fraction, bar, tc.contains)
		}
	}
}

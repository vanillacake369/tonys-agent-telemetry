package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/control"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestVisualSmoke_RealisticDataPushesEachTab seeds each tab with the
// realistic-LOAD-message it expects in production, then asserts View() at
// the standard 80×24 size stays inside the terminal. This catches "tab
// looked fine empty, breaks once data loads" — the actual user scenario.
func TestVisualSmoke_RealisticDataPushesEachTab(t *testing.T) {
	const W, H = 80, 24

	a := NewApp()
	a.width, a.height = W, H
	a = a.propagateSize()
	_ = tea.Model(a) // silence import-not-used in some build configs

	// --- Push realistic data into each tab via its dedicated Msg ---

	// Hooks: feed a config with EVERY event type populated — mirrors a
	// real ~/.claude/settings.json with workflow + statusline + 4 events.
	hooksConfig := HooksConfig{
		Hooks: map[string][]HookGroup{
			"UserPromptSubmit": {{Hooks: []HookEntry{{Type: "command", Command: "~/.claude/hooks/intent.sh", Timeout: 5}}}},
			"PreToolUse": {{Hooks: []HookEntry{
				{Type: "command", Command: "~/.claude/hooks/cmd-guard.sh", Timeout: 5},
				{Type: "command", Command: "~/.claude/hooks/path-guard.sh", Timeout: 5},
			}}},
			"PostToolUse": {{Hooks: []HookEntry{
				{Type: "command", Command: "~/.claude/hooks/test-feedback.sh", Timeout: 5},
				{Type: "command", Command: "~/.claude/hooks/escalation.sh", Timeout: 5},
			}}},
			"Stop": {{Hooks: []HookEntry{{Type: "command", Command: "~/.claude/hooks/agent-notify.sh", Timeout: 5}}}},
		},
		StatusLine: &StatusLineConfig{Type: "command", Command: "~/.claude/statusline.sh"},
	}
	updated, _ := a.Update(HooksLoadedMsg{Config: hooksConfig})
	a = updated.(App)

	// DAG: push spans so traces list + provider badges + multiple rows render.
	now := time.Now()
	spans := []telemetry.Span{
		{TraceID: "trace-A", SpanID: "a1", System: "anthropic", Status: "done",
			StartTime: now.Add(-2 * time.Minute), EndTime: now.Add(-90 * time.Second),
			Attrs: map[string]string{"gen_ai.tool.name": "bash"}},
		{TraceID: "trace-A", SpanID: "a2", ParentSpanID: "a1", System: "anthropic", Status: "done",
			StartTime: now.Add(-90 * time.Second), EndTime: now.Add(-60 * time.Second),
			Attrs: map[string]string{"gen_ai.tool.name": "read"}},
		{TraceID: "trace-B", SpanID: "b1", System: "otlp", Status: "error",
			StartTime: now.Add(-45 * time.Second), EndTime: now.Add(-30 * time.Second),
			Attrs: map[string]string{"gen_ai.tool.name": "git_commit"}},
		{TraceID: "trace-C", SpanID: "c1", System: "anthropic", Status: "done",
			StartTime: now.Add(-20 * time.Second), EndTime: now.Add(-10 * time.Second),
			Attrs: map[string]string{"gen_ai.tool.name": "edit"}},
	}
	updated, _ = a.Update(SpanBatchMsg{Spans: spans})
	a = updated.(App)

	// Control: populated state with policy + budgets + denials so the
	// rendered render path (not empty-state guide) is exercised.
	updated, _ = a.Update(ControlRefreshMsg{
		Policy: control.Policy{Budget: control.BudgetPolicy{SessionMaxUSD: 5.0, DailyMaxUSD: 50.0}},
		Budgets: []control.Budget{
			{SessionID: "session-current", InputTokens: 1000, OutputTokens: 200, CostUSD: 1.23, UpdatedAt: now},
		},
		Denials: []control.Denial{
			{Timestamp: now, SessionID: "session-current", Tool: "Bash", Reason: "tool_denylisted", Detail: "rm -rf /"},
		},
	})
	a = updated.(App)

	a = a.propagateSize()

	// --- Hard-assert each tab fits at 80×24 ---
	for _, tab := range tabOrder {
		ac := a
		ac.activeTab = tab
		view := ac.View()
		lines := strings.Split(view, "\n")

		t.Logf("\n--- tab=%s rows=%d ---\n%s\n--- /%s ---",
			tabNames[tab], len(lines), view, tabNames[tab])

		if len(lines) > H {
			t.Errorf("[tab=%s] rendered %d rows > terminal height %d — top cropped, tab bar hidden",
				tabNames[tab], len(lines), H)
		}
		if len(lines) >= 2 {
			if !strings.Contains(stripAnsi(lines[1]), "1:Sessions") {
				t.Errorf("[tab=%s] tab bar NOT on line 1.\nLine 1: %q",
					tabNames[tab], stripAnsi(lines[1]))
			}
		}
		for i, line := range lines {
			rs := []rune(stripAnsi(line))
			if len(rs) > W {
				t.Errorf("[tab=%s] line %d width %d > terminal %d (wraps, adds row)",
					tabNames[tab], i, len(rs), W)
				break
			}
		}
		var lastNonEmpty string
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(stripAnsi(lines[i])) != "" {
				lastNonEmpty = stripAnsi(lines[i])
				break
			}
		}
		if !strings.Contains(lastNonEmpty, "╰") && !strings.Contains(lastNonEmpty, "└") {
			t.Errorf("[tab=%s] outer border missing close on last line: %q",
				tabNames[tab], lastNonEmpty)
		}
	}
}

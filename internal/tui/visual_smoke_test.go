package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestVisualSmoke_AllTabsWithRealData drives the App with realistic spans
// across MULTIPLE common terminal sizes (80x24, 100x30, 120x40) so the
// "looks fine at one size, broken at another" class of bug — including
// tab-bar-pushed-off-the-top when content overflows — is caught here.
//
// Hard assertion: the tab bar must be on the SECOND visible line of the
// rendered output (line 0 = outer border top, line 1 = tab bar). If any
// content lower in the View pushes the layout taller than `height`, the
// rendered output will exceed `height` rows and the user's terminal will
// crop the top — pushing the tab bar off-screen. This test catches that
// by asserting the total rendered row count is <= height.
func TestVisualSmoke_AllTabsWithRealData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual smoke in -short mode")
	}

	sizes := []struct {
		name string
		w, h int
	}{
		{"narrow-80x24", 80, 24},
		{"common-100x30", 100, 30},
		{"wide-120x40", 120, 40},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			runSmoke(t, sz.w, sz.h)
		})
	}
}

func runSmoke(t *testing.T, width, height int) {
	t.Helper()

	var m tea.Model = NewApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: width, Height: height})

	// Multi-day synthetic spans so Trends populates with real sparklines.
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	twoDayAgo := now.Add(-48 * time.Hour)
	spans := []telemetry.Span{
		{TraceID: "t-d2", SpanID: "d2-root",
			StartTime: twoDayAgo.Add(-30 * time.Second), EndTime: twoDayAgo,
			System: "anthropic", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "bash"}},
		{TraceID: "t-d2", SpanID: "d2-stalled", ParentSpanID: "d2-root",
			StartTime: twoDayAgo.Add(-30 * time.Second), EndTime: twoDayAgo.Add(-2 * time.Second),
			System: "anthropic", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "bash"}},
		{TraceID: "t-d1", SpanID: "d1-root",
			StartTime: dayAgo.Add(-15 * time.Second), EndTime: dayAgo,
			System: "anthropic", Status: "error",
			Attrs: map[string]string{"gen_ai.tool.name": "git_commit"}},
		{TraceID: "t-refactor-auth", SpanID: "root",
			StartTime: now.Add(-30 * time.Second), EndTime: now.Add(-15 * time.Second),
			System: "anthropic", Status: "done",
			Attrs: map[string]string{"gen_ai.operation.name": "agent.session"}},
		{TraceID: "t-refactor-auth", SpanID: "llm1", ParentSpanID: "root",
			StartTime: now.Add(-28 * time.Second), EndTime: now.Add(-25 * time.Second),
			System: "anthropic", Model: "claude-opus", InputTokens: 240, OutputTokens: 891,
			Attrs: map[string]string{"gen_ai.operation.name": "llm.request"}},
		{TraceID: "t-refactor-auth", SpanID: "tool1", ParentSpanID: "llm1",
			StartTime: now.Add(-25 * time.Second), EndTime: now.Add(-23 * time.Second),
			System: "anthropic", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "linter"}},
		{TraceID: "t-refactor-auth", SpanID: "stalled", ParentSpanID: "root",
			StartTime: now.Add(-25 * time.Second), EndTime: now.Add(-2 * time.Second),
			System: "anthropic", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "bash"}},
		{TraceID: "t-refactor-auth", SpanID: "failed", ParentSpanID: "root",
			StartTime: now.Add(-2 * time.Second), EndTime: now,
			System: "anthropic", Status: "error",
			Attrs: map[string]string{"gen_ai.tool.name": "git_commit", "error.type": "merge_conflict"}},
		{TraceID: "t-write-tests", SpanID: "root2",
			StartTime: now.Add(-60 * time.Second), EndTime: now.Add(-50 * time.Second),
			System: "otlp", Status: "done",
			Attrs: map[string]string{"gen_ai.operation.name": "agent.session"}},
	}
	m, _ = m.Update(SpanBatchMsg{Spans: spans})

	// Trigger loadTrendsCmd so Trends has data.
	a := m.(App)
	if cmd := a.loadTrendsCmd(); cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ := a.Update(msg)
			m = updated
		}
	}

	tabs := []Tab{TabSessions, TabSkills, TabCost, TabHooks, TabDAG, TabTrends, TabControl}
	for _, tab := range tabs {
		a := m.(App)
		a.activeTab = tab
		view := a.View()

		lines := strings.Split(view, "\n")
		t.Logf("\n--- tab=%s size=%dx%d rows=%d ---\n%s\n--- /%s ---",
			tabNames[tab], width, height, len(lines), view, tabNames[tab])

		// HARD ASSERTIONS that catch the user-reported breakage:

		// 1. Total row count must not exceed terminal height. If it does,
		//    the user's terminal crops the top → tab bar disappears.
		if len(lines) > height {
			t.Errorf("[size=%dx%d tab=%s] rendered %d rows > terminal height %d — "+
				"top will be cropped, tab bar pushed off-screen",
				width, height, tabNames[tab], len(lines), height)
		}

		// 2. Tab bar must appear on line 1 (line 0 = outer border top).
		if len(lines) < 2 {
			t.Errorf("[size=%dx%d tab=%s] too few lines (%d) to contain a tab bar",
				width, height, tabNames[tab], len(lines))
			continue
		}
		secondLineStripped := stripAnsi(lines[1])
		if !strings.Contains(secondLineStripped, "1:Sessions") {
			t.Errorf("[size=%dx%d tab=%s] tab bar NOT on line 1.\nLine 1: %q",
				width, height, tabNames[tab], secondLineStripped)
		}

		// 3. No line wider than terminal.
		for i, line := range lines {
			rs := []rune(stripAnsi(line))
			if len(rs) > width {
				t.Errorf("[size=%dx%d tab=%s] line %d width %d > terminal %d (overflow)",
					width, height, tabNames[tab], i, len(rs), width)
				break
			}
		}

		// 4. Outer border closed: last non-empty line should contain '╰'.
		var lastNonEmpty string
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.TrimSpace(stripAnsi(lines[i])) != "" {
				lastNonEmpty = stripAnsi(lines[i])
				break
			}
		}
		if !strings.Contains(lastNonEmpty, "╰") && !strings.Contains(lastNonEmpty, "└") {
			t.Errorf("[size=%dx%d tab=%s] outer border missing bottom-close on last line: %q",
				width, height, tabNames[tab], lastNonEmpty)
		}
	}
}

var _ = fmt.Sprintf // keep import for future use

package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestVisualSmoke_AllTabsWithRealData drives the App with synthetic spans
// that look like a real Claude Code session, then dumps every tab's View()
// so a human can scan for layout breakage. Useful when the binary cannot
// be launched interactively (no TTY).
func TestVisualSmoke_AllTabsWithRealData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping visual smoke in -short mode")
	}

	var m tea.Model = NewApp()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Multi-day synthetic spans so Trends buckets actually populate
	// (DefaultBucketDuration=24h, MinBucketsForDisplay=2).
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	twoDayAgo := now.Add(-48 * time.Hour)
	spans := []telemetry.Span{
		// Trace from 2 days ago — produces a stalled_node bucket.
		{TraceID: "t-d2", SpanID: "d2-root",
			StartTime: twoDayAgo.Add(-30 * time.Second), EndTime: twoDayAgo,
			System: "anthropic", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "bash"}},
		{TraceID: "t-d2", SpanID: "d2-stalled", ParentSpanID: "d2-root",
			StartTime: twoDayAgo.Add(-30 * time.Second), EndTime: twoDayAgo.Add(-2 * time.Second),
			System: "anthropic", Status: "done",
			Attrs: map[string]string{"gen_ai.tool.name": "bash"}},
		// Trace from yesterday — produces another bucket.
		{TraceID: "t-d1", SpanID: "d1-root",
			StartTime: dayAgo.Add(-15 * time.Second), EndTime: dayAgo,
			System: "anthropic", Status: "error",
			Attrs: map[string]string{"gen_ai.tool.name": "git_commit"}},
		// Today's realistic refactor trace.
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

	// Simulate pressing key '6' so loadTrendsCmd fires and Trends gets
	// live-span buckets (this is the F4 backfill path the user reported as
	// "9 sessions on disk → empty Trends").
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

		// Truncate but show enough to spot layout problems.
		t.Logf("\n========== Tab: %s ==========\n%s\n========== /Tab: %s (%d bytes) ==========",
			tabNames[tab], view, tabNames[tab], len(view))

		// Hard checks every tab must pass.
		if !strings.Contains(view, "1:Sessions") {
			t.Errorf("[%s] tab bar missing — top-level layout broken", tabNames[tab])
		}
		if !strings.Contains(view, "╭") && !strings.Contains(view, "┌") {
			t.Errorf("[%s] no outer border — Panel wrap broken", tabNames[tab])
		}
		// First-paint sanity at 120×40: no line wider than terminal.
		for _, line := range strings.Split(view, "\n") {
			rs := []rune(stripAnsi(line))
			if len(rs) > 120 {
				t.Errorf("[%s] line %d cols > 120 (overflow): %q", tabNames[tab], len(rs), string(rs)[:min(80, len(string(rs)))])
				break
			}
		}
	}
}

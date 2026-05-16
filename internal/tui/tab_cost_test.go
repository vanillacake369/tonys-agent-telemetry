package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

// updateCostTab is a test helper that runs one Update cycle and returns the
// resulting CostTab (fatals if model changes type).
func updateCostTab(t *testing.T, c CostTab, msg tea.Msg) (CostTab, tea.Cmd) {
	t.Helper()
	model, cmd := c.Update(msg)
	result, ok := model.(CostTab)
	if !ok {
		t.Fatalf("Update returned %T, want CostTab", model)
	}
	return result, cmd
}

// makeMockCosts returns a small slice of SessionCosts for testing.
func makeMockCosts() []data.SessionCost {
	ts := time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC)
	return []data.SessionCost{
		{
			SessionID:    "s1",
			Project:      "/Users/me/proj-a",
			Model:        "claude-opus-4-6",
			InputTokens:  100_000,
			OutputTokens: 50_000,
			TotalTokens:  150_000,
			EstCostUSD:   2.25,
			TurnCount:    5,
			Duration:     30 * time.Minute,
			Timestamp:    ts,
			ToolCalls:    map[string]int{"Bash": 10, "Read": 5},
		},
		{
			SessionID:    "s2",
			Project:      "/Users/me/proj-b",
			Model:        "claude-sonnet-4-6",
			InputTokens:  50_000,
			OutputTokens: 20_000,
			TotalTokens:  70_000,
			EstCostUSD:   0.45,
			TurnCount:    3,
			Duration:     15 * time.Minute,
			Timestamp:    ts.Add(-24 * time.Hour),
			ToolCalls:    map[string]int{"Edit": 8},
		},
	}
}

// ── CostLoadedMsg ─────────────────────────────────────────────────────────────

func TestCostTab_CostLoadedMsg_PopulatesSummary(t *testing.T) {
	c := NewCostTab()
	costs := makeMockCosts()
	summary := data.Summarize(costs)

	c, _ = updateCostTab(t, c, CostLoadedMsg{Costs: costs, Summary: summary})

	if c.loading {
		t.Error("loading should be false after CostLoadedMsg")
	}
	if c.err != nil {
		t.Errorf("err = %v, want nil", c.err)
	}
	if len(c.costs) != 2 {
		t.Errorf("costs len = %d, want 2", len(c.costs))
	}
	if c.summary.TotalSessions != 2 {
		t.Errorf("summary.TotalSessions = %d, want 2", c.summary.TotalSessions)
	}
}

func TestCostTab_CostLoadedMsg_WithError(t *testing.T) {
	c := NewCostTab()
	c, _ = updateCostTab(t, c, CostLoadedMsg{Err: errTest("disk error")})

	if c.loading {
		t.Error("loading should be false after error CostLoadedMsg")
	}
	if c.err == nil {
		t.Error("err should be set after error CostLoadedMsg")
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestCostTab_Refresh_ResetsState(t *testing.T) {
	c := NewCostTab()
	costs := makeMockCosts()
	summary := data.Summarize(costs)
	c, _ = updateCostTab(t, c, CostLoadedMsg{Costs: costs, Summary: summary})

	// "r" should trigger reload.
	c2, cmd := updateCostTab(t, c, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if !c2.loading {
		t.Error("loading should be true after refresh")
	}
	if len(c2.costs) != 0 {
		t.Errorf("costs should be cleared after refresh, got %d", len(c2.costs))
	}
	if cmd == nil {
		t.Error("refresh should return a non-nil reload cmd")
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func TestCostTab_View_ContainsCostSections(t *testing.T) {
	c := NewCostTab()
	c = c.SetSize(120, 40).(CostTab)
	costs := makeMockCosts()
	summary := data.Summarize(costs)
	c, _ = updateCostTab(t, c, CostLoadedMsg{Costs: costs, Summary: summary})

	view := c.View()

	for _, section := range []string{"By Model", "By Project", "Daily", "Top Tools"} {
		if !strings.Contains(view, section) {
			t.Errorf("View() missing section %q", section)
		}
	}
}

func TestCostTab_View_ContainsTotalCost(t *testing.T) {
	c := NewCostTab()
	c = c.SetSize(120, 40).(CostTab)
	costs := makeMockCosts()
	summary := data.Summarize(costs)
	c, _ = updateCostTab(t, c, CostLoadedMsg{Costs: costs, Summary: summary})

	view := c.View()

	// The total cost in the summary row should appear.
	if !strings.Contains(view, "$") {
		t.Errorf("View() should contain cost (dollar sign), got:\n%s", view)
	}
	if !strings.Contains(view, "sessions") {
		t.Errorf("View() should contain 'sessions' count", )
	}
}

func TestCostTab_View_LoadingState(t *testing.T) {
	c := NewCostTab()
	c = c.SetSize(120, 40).(CostTab)
	// loading is true by default.
	view := c.View()
	if view == "" {
		t.Error("View() in loading state should not be empty")
	}
}

// ── SetSize ───────────────────────────────────────────────────────────────────

func TestCostTab_SetSize_UpdatesDimensions(t *testing.T) {
	c := NewCostTab()
	updated := c.SetSize(100, 30).(CostTab)

	if updated.width != 100 {
		t.Errorf("width = %d, want 100", updated.width)
	}
	if updated.height != 30 {
		t.Errorf("height = %d, want 30", updated.height)
	}
}

// ── Init ──────────────────────────────────────────────────────────────────────

func TestCostTab_Init_ReturnsCmd(t *testing.T) {
	c := NewCostTab()
	cmd := c.Init()
	if cmd == nil {
		t.Error("Init() should return a non-nil Cmd")
	}
}

// ── renderBar ─────────────────────────────────────────────────────────────────

func TestRenderBar_FullBar(t *testing.T) {
	bar := renderBar(100.0, 100.0, 20)
	runeLen := len([]rune(bar))
	if runeLen != 20 {
		t.Errorf("renderBar rune length = %d, want 20", runeLen)
	}
	if strings.Contains(bar, "░") {
		t.Errorf("full bar should not contain empty chars: %q", bar)
	}
}

func TestRenderBar_EmptyBar(t *testing.T) {
	bar := renderBar(0.0, 100.0, 20)
	runeLen := len([]rune(bar))
	if runeLen != 20 {
		t.Errorf("renderBar rune length = %d, want 20", runeLen)
	}
	if strings.Contains(bar, "█") {
		t.Errorf("empty bar should not contain filled chars: %q", bar)
	}
}

func TestRenderBar_HalfFilled(t *testing.T) {
	bar := renderBar(50.0, 100.0, 20)
	runeLen := len([]rune(bar))
	if runeLen != 20 {
		t.Errorf("renderBar rune length = %d, want 20", runeLen)
	}
	filledCount := strings.Count(bar, "█")
	if filledCount != 10 {
		t.Errorf("half bar filled count = %d, want 10", filledCount)
	}
}

func TestRenderBar_ZeroMaxValue(t *testing.T) {
	// Should not panic or divide by zero.
	bar := renderBar(50.0, 0.0, 10)
	runeLen := len([]rune(bar))
	if runeLen != 10 {
		t.Errorf("renderBar with zero max: rune length = %d, want 10", runeLen)
	}
}

func TestRenderBar_WidthZero(t *testing.T) {
	bar := renderBar(50.0, 100.0, 0)
	if bar != "" {
		t.Errorf("renderBar with zero width should return empty string, got %q", bar)
	}
}

// ── formatTokenCount ──────────────────────────────────────────────────────────

func TestFormatTokenCount_Millions(t *testing.T) {
	got := formatTokenCount(1_500_000)
	if !strings.Contains(got, "M") {
		t.Errorf("formatTokenCount(1.5M) = %q, want 'M' suffix", got)
	}
}

func TestFormatTokenCount_Thousands(t *testing.T) {
	got := formatTokenCount(312_000)
	if !strings.Contains(got, "K") {
		t.Errorf("formatTokenCount(312K) = %q, want 'K' suffix", got)
	}
}

func TestFormatTokenCount_Small(t *testing.T) {
	got := formatTokenCount(500)
	if got != "500" {
		t.Errorf("formatTokenCount(500) = %q, want '500'", got)
	}
}

// ── App-level cost tab integration ───────────────────────────────────────────

func TestApp_CostTabHints(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	hints := a.tabHints()
	if !strings.Contains(hints, "r:refresh") {
		t.Errorf("cost tab hints should contain 'r:refresh', got %q", hints)
	}
}

func TestApp_CostTab_InTabBar(t *testing.T) {
	a := NewApp()
	a, _ = updateApp(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	view := a.View()
	if !strings.Contains(view, "3:Cost") {
		t.Errorf("View() should contain '3:Cost' in tab bar, got relevant part:\n%s", view)
	}
}

// errTest is a simple error type for testing error states.
type errTest string

func (e errTest) Error() string { return string(e) }

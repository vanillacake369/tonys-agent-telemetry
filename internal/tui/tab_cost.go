package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

// CostLoadedMsg is sent when DiscoverAllCosts completes.
type CostLoadedMsg struct {
	Costs   []data.SessionCost
	Summary data.CostSummary
	Err     error
}

// CostTab implements TabModel for the Cost/Usage tab.
// It shows an aggregated spending dashboard: totals, by-model, by-project,
// daily breakdown, and top tool usage.
type CostTab struct {
	summary  data.CostSummary
	costs    []data.SessionCost
	viewport viewport.Model
	width    int
	height   int
	loading  bool
	err      error
	keys     KeyMap
}

// NewCostTab creates an initialised CostTab ready to be displayed.
func NewCostTab() CostTab {
	vp := viewport.New(80, 20)
	return CostTab{
		viewport: vp,
		loading:  true,
		keys:     DefaultKeyMap(),
	}
}

// Init loads cost data asynchronously.
func (c CostTab) Init() tea.Cmd {
	return func() tea.Msg {
		costs, err := data.DiscoverAllCosts()
		if err != nil {
			return CostLoadedMsg{Err: err}
		}
		summary := data.Summarize(costs)
		return CostLoadedMsg{Costs: costs, Summary: summary}
	}
}

// Update handles messages and key events.
func (c CostTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case CostLoadedMsg:
		c.loading = false
		c.err = msg.Err
		if msg.Err == nil {
			c.costs = msg.Costs
			c.summary = msg.Summary
			c.refreshViewport()
		}
		return c, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, c.keys.Refresh):
			c.loading = true
			c.costs = nil
			c.summary = data.CostSummary{}
			return c, c.Init()
		}

		// Delegate viewport scrolling keys.
		var vpCmd tea.Cmd
		c.viewport, vpCmd = c.viewport.Update(msg)
		if vpCmd != nil {
			cmds = append(cmds, vpCmd)
		}
		return c, tea.Batch(cmds...)
	}

	// Delegate other messages (e.g., mouse) to the viewport.
	var vpCmd tea.Cmd
	c.viewport, vpCmd = c.viewport.Update(msg)
	if vpCmd != nil {
		cmds = append(cmds, vpCmd)
	}
	return c, tea.Batch(cmds...)
}

// SetSize updates stored dimensions and the viewport.
func (c CostTab) SetSize(width, height int) TabModel {
	c.width = width
	c.height = height
	c.viewport.Width = max(1, width-2)   // -2 for panel left+right border
	c.viewport.Height = max(1, height-2) // -2 for panel top+bottom border
	if !c.loading && c.err == nil {
		c.refreshViewport()
	}
	return c
}

// View renders the cost dashboard.
func (c CostTab) View() string {
	if c.loading {
		return RenderLoadingState(c.width, c.height)
	}
	if c.err != nil {
		return renderErrorState(c.err, c.width)
	}

	panelH := max(3, c.height)
	return RenderPanel("Cost/Usage", c.viewport.View(), c.width, panelH, true)
}

// refreshViewport rebuilds the scrollable dashboard content.
func (c *CostTab) refreshViewport() {
	contentW := max(40, c.viewport.Width-2) // -2 for padding
	c.viewport.SetContent(c.renderDashboard(contentW))
	c.viewport.GotoTop()
}

// renderDashboard builds the full dashboard string.
func (c CostTab) renderDashboard(width int) string {
	s := c.summary
	var sb strings.Builder

	// ── Summary row ──
	sb.WriteString(renderSummaryRow(s, width))
	sb.WriteString("\n\n")

	// ── By Model ──
	sb.WriteString(renderSectionHeader("By Model", width))
	sb.WriteString("\n")
	sb.WriteString(renderModelSection(s, width))
	sb.WriteString("\n\n")

	// ── By Project ──
	sb.WriteString(renderSectionHeader("By Project", width))
	sb.WriteString("\n")
	sb.WriteString(renderProjectSection(s, width))
	sb.WriteString("\n\n")

	// ── Daily (last 7 days) ──
	sb.WriteString(renderSectionHeader("Daily (last 7 days)", width))
	sb.WriteString("\n")
	sb.WriteString(renderDailySection(s, width))
	sb.WriteString("\n\n")

	// ── Top Tools ──
	sb.WriteString(renderSectionHeader("Top Tools", width))
	sb.WriteString("\n")
	sb.WriteString(renderToolsSection(s, width))

	return sb.String()
}

// renderSummaryRow renders the single top-level totals line.
func renderSummaryRow(s data.CostSummary, width int) string {
	totalCost := fmt.Sprintf("$%.2f", s.TotalCostUSD)
	totalTok := formatTokenCount(s.TotalTokens)
	sessions := fmt.Sprintf("%d sessions", s.TotalSessions)
	duration := formatDuration(s.TotalDuration)

	parts := []string{totalCost, totalTok + " tokens", sessions, duration}
	line := strings.Join(parts, "  │  ")

	return lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		Width(width).
		Render(line)
}

// renderSectionHeader renders a bold section label.
func renderSectionHeader(title string, width int) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary).
		Width(width).
		Render(title + ":")
}

// renderModelSection renders per-model bars and stats.
func renderModelSection(s data.CostSummary, width int) string {
	if len(s.ByModel) == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("  no data")
	}

	// Sort models by cost DESC.
	type entry struct {
		name string
		ms   data.ModelStats
	}
	var entries []entry
	for name, ms := range s.ByModel {
		entries = append(entries, entry{name, ms})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ms.Cost > entries[j].ms.Cost
	})

	maxCost := entries[0].ms.Cost

	const barWidth = 20
	const modelNameWidth = 18
	var lines []string
	for _, e := range entries {
		pct := 0.0
		if s.TotalCostUSD > 0 {
			pct = e.ms.Cost / s.TotalCostUSD * 100
		}
		bar := renderBar(e.ms.Cost, maxCost, barWidth)
		tok := formatTokenCount(e.ms.Tokens)
		// Use PadToWidth for CJK-safe label column alignment.
		nameCol := PadToWidth(lipgloss.NewStyle().MaxWidth(modelNameWidth).Render(e.name), modelNameWidth)
		line := fmt.Sprintf("  %s %s  $%.2f (%3.0f%%)  %s tok",
			nameCol, bar, e.ms.Cost, pct, tok)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// renderProjectSection renders per-project bars and costs.
func renderProjectSection(s data.CostSummary, width int) string {
	if len(s.ByProject) == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("  no data")
	}

	type entry struct {
		name string
		cost float64
	}
	var entries []entry
	for name, cost := range s.ByProject {
		entries = append(entries, entry{name, cost})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].cost > entries[j].cost
	})

	maxCost := entries[0].cost
	const barWidth = 20

	// Limit display to top 10 projects.
	if len(entries) > 10 {
		entries = entries[:10]
	}

	const nameWidth = 16
	var lines []string
	for _, e := range entries {
		bar := renderBar(e.cost, maxCost, barWidth)
		// Use just the directory base name to avoid long paths.
		name := filepath.Base(e.name)
		// Truncate with "..." when the display width exceeds nameWidth.
		// PadToWidth then ensures the column is exactly nameWidth cells wide
		// even when the name contains CJK double-width characters.
		if lipgloss.Width(name) > nameWidth {
			name = string([]rune(name)[:nameWidth-3]) + "..."
		}
		nameCol := PadToWidth(name, nameWidth)
		line := fmt.Sprintf("  %s %s  $%.2f", nameCol, bar, e.cost)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// renderDailySection renders a 7-day cost bar chart.
func renderDailySection(s data.CostSummary, width int) string {
	if len(s.ByDay) == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("  no data")
	}

	// Build last 7 days in descending order (most recent first).
	today := time.Now()
	type dayEntry struct {
		day  string
		cost float64
	}
	var days []dayEntry
	for i := 0; i < 7; i++ {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		days = append(days, dayEntry{day: d, cost: s.ByDay[d]})
	}

	// Find max for proportional bar.
	var maxCost float64
	for _, d := range days {
		if d.cost > maxCost {
			maxCost = d.cost
		}
	}
	if maxCost == 0 {
		maxCost = 1 // avoid divide-by-zero
	}

	const barWidth = 20
	var lines []string
	for _, d := range days {
		// Format as short day name + date.
		t, _ := time.Parse("2006-01-02", d.day)
		label := t.Format("Mon 01-02")
		bar := renderBar(d.cost, maxCost, barWidth)
		line := fmt.Sprintf("  %s  %s  $%.2f", label, bar, d.cost)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// renderToolsSection renders the top tool usage summary.
func renderToolsSection(s data.CostSummary, width int) string {
	if len(s.TopTools) == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("  no data")
	}

	// Sum total calls for percentage calculation.
	total := 0
	for _, t := range s.TopTools {
		total += t.Count
	}

	// Show up to 8 tools.
	tools := s.TopTools
	if len(tools) > 8 {
		tools = tools[:8]
	}

	var parts []string
	for _, t := range tools {
		pct := 0.0
		if total > 0 {
			pct = float64(t.Count) / float64(total) * 100
		}
		parts = append(parts, fmt.Sprintf("%s %.0f%%", t.Name, pct))
	}
	return "  " + strings.Join(parts, "  ")
}

// renderBar renders a proportional ASCII bar of fixed width.
// filled chars = █, empty chars = ░.
func renderBar(value, maxValue float64, width int) string {
	if maxValue <= 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filled := int(float64(width) * value / maxValue)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// formatTokenCount formats token count as K or M suffix strings.
func formatTokenCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatDuration formats a duration as hours and minutes (e.g. "12.4h").
func formatDuration(d time.Duration) string {
	h := d.Hours()
	if h >= 1 {
		return fmt.Sprintf("%.1fh", h)
	}
	m := d.Minutes()
	if m >= 1 {
		return fmt.Sprintf("%.0fm", m)
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/control"
)

// ControlRefreshMsg is sent when budget/denial data has been reloaded.
type ControlRefreshMsg struct {
	Budgets []control.Budget
	Denials []control.Denial
	Policy  control.Policy
	Err     error
}

// ControlTab implements TabModel for the Control/Governance tab.
type ControlTab struct {
	width, height int
	keys          KeyMap
	budgets       []control.Budget
	denials       []control.Denial
	policy        control.Policy
	policyPath    string
	err           error
	loading       bool
}

// NewControlTab creates an initialised ControlTab.
func NewControlTab() ControlTab {
	return ControlTab{
		keys:       DefaultKeyMap(),
		loading:    true,
		policyPath: controlPolicyPath(),
	}
}

// controlPolicyPath returns the path to policy.toml, respecting XDG_CONFIG_HOME.
func controlPolicyPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "~/.config/tonys-agent-telemetry/policy.toml"
		}
		base = home + "/.config"
	}
	return base + "/tonys-agent-telemetry/policy.toml"
}

func (t ControlTab) Init() tea.Cmd {
	return t.loadCmd()
}

// loadCmd returns a tea.Cmd that loads policy, budgets, and denials from disk.
func (t ControlTab) loadCmd() tea.Cmd {
	return func() tea.Msg {
		pol, _ := control.LoadPolicy()

		cacheDir := control.CacheDir()
		store := control.NewBudgetStore(cacheDir)
		denLog := control.NewDenialLog(cacheDir)

		budgets, err := store.All()
		if err != nil {
			return ControlRefreshMsg{Policy: pol, Err: err}
		}

		denials, err := denLog.Recent(20)
		if err != nil {
			return ControlRefreshMsg{Policy: pol, Budgets: budgets, Err: err}
		}

		return ControlRefreshMsg{Policy: pol, Budgets: budgets, Denials: denials}
	}
}

func (t ControlTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ControlRefreshMsg:
		t.loading = false
		t.err = msg.Err
		t.policy = msg.Policy
		t.budgets = msg.Budgets
		t.denials = msg.Denials
		return t, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, t.keys.Refresh):
			t.loading = true
			return t, t.loadCmd()

		case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'e':
			return t, t.openEditorCmd()

		case msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == 'c':
			return t, t.clearDenialsCmd()
		}
	}
	return t, nil
}

// openEditorCmd opens policy.toml in $EDITOR using tea.ExecProcess.
func (t ControlTab) openEditorCmd() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	p := t.policyPath
	return tea.ExecProcess(editorCommand(editor, p), func(err error) tea.Msg {
		return ControlRefreshMsg{}
	})
}

// clearDenialsCmd removes the denials.log file.
func (t ControlTab) clearDenialsCmd() tea.Cmd {
	return func() tea.Msg {
		cacheDir := control.CacheDir()
		_ = os.Remove(cacheDir + "/denials.log")
		pol, _ := control.LoadPolicy()
		store := control.NewBudgetStore(cacheDir)
		budgets, _ := store.All()
		return ControlRefreshMsg{Policy: pol, Budgets: budgets, Denials: []control.Denial{}}
	}
}

func (t ControlTab) View() string {
	if t.width == 0 || t.height == 0 {
		return "Control"
	}

	if t.loading {
		return RenderLoadingState(t.width, t.height)
	}

	var sb strings.Builder

	// POLICY section.
	sb.WriteString(sectionHeader("POLICY", t.width))
	sb.WriteString("\n")
	sb.WriteString(renderPolicyInfo(t.policy, t.policyPath, t.width))
	sb.WriteString("\n\n")

	// BUDGETS section.
	sb.WriteString(sectionHeader("BUDGETS", t.width))
	sb.WriteString("\n")
	sb.WriteString(renderBudgets(t.budgets, t.policy, t.width))
	sb.WriteString("\n\n")

	// RECENT DENIALS section.
	sb.WriteString(sectionHeader("RECENT DENIALS", t.width))
	sb.WriteString("\n")
	sb.WriteString(renderDenials(t.denials, t.width))

	return lipgloss.NewStyle().
		Width(max(0, t.width)).
		Height(max(0, t.height)).
		Render(sb.String())
}

func (t ControlTab) SetSize(width, height int) TabModel {
	t.width = width
	t.height = height
	return t
}

func (t ControlTab) Title() string { return "Control" }

// sectionHeader renders a bold section header with an underline.
func sectionHeader(title string, width int) string {
	header := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary).Render(title)
	underline := lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("─", min(width, len(title)+2)))
	return header + "\n" + underline
}

// renderPolicyInfo renders the policy summary lines.
func renderPolicyInfo(pol control.Policy, policyPath string, width int) string {
	hint := lipgloss.NewStyle().Foreground(colorDim).Render("e: edit")
	pathLine := lipgloss.NewStyle().Foreground(colorText).Render("POLICY  "+policyPath) + "  " + hint

	var capLine string
	if pol.Budget.SessionMaxUSD == 0 && pol.Budget.DailyMaxUSD == 0 {
		capLine = lipgloss.NewStyle().Foreground(colorWarning).Render("(no policy loaded — fail-open)")
	} else {
		capLine = fmt.Sprintf("Session cap: $%.2f   Daily cap: $%.2f   Warn at: %.0f%%",
			pol.Budget.SessionMaxUSD, pol.Budget.DailyMaxUSD, pol.Budget.WarnAtFraction*100)
	}

	denylistLine := renderPatternLine("Denylist", pol.Tools.Denylist)
	allowlistLine := renderPatternLine("Allowlist", pol.Tools.Allowlist)

	return strings.Join([]string{pathLine, capLine, denylistLine, allowlistLine}, "\n")
}

// renderPatternLine renders "Denylist (N): p1 · p2" or "(empty — all tools allowed)".
func renderPatternLine(label string, patterns []string) string {
	if len(patterns) == 0 {
		return fmt.Sprintf("%s:     %s", label,
			lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("(empty — all tools allowed)"))
	}
	joined := strings.Join(patterns, "  ·  ")
	return fmt.Sprintf("%s (%d):  %s", label, len(patterns), joined)
}

// renderBudgets renders the daily total bar and per-session table.
func renderBudgets(budgets []control.Budget, pol control.Policy, width int) string {
	if len(budgets) == 0 && pol.Budget.DailyMaxUSD == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("No budget data yet")
	}

	// Compute today's total.
	today := time.Now().UTC().Format("2006-01-02")
	var dailyTotal float64
	var todayBudgets []control.Budget
	for _, b := range budgets {
		if b.UpdatedAt.UTC().Format("2006-01-02") == today {
			dailyTotal += b.CostUSD
			todayBudgets = append(todayBudgets, b)
		}
	}

	var sb strings.Builder

	// Daily total bar.
	dailyCap := pol.Budget.DailyMaxUSD
	if dailyCap > 0 {
		fraction := dailyTotal / dailyCap
		bar := renderProgressBar(fraction, 18)
		sb.WriteString(fmt.Sprintf("Today's total: $%.2f / $%.2f  %s %.1f%%\n\n",
			dailyTotal, dailyCap, bar, fraction*100))
	} else {
		sb.WriteString(fmt.Sprintf("Today's total: $%.2f  (no daily cap)\n\n", dailyTotal))
	}

	if len(todayBudgets) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("No sessions today"))
		return sb.String()
	}

	// Session table header.
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorDim)
	sb.WriteString(fmt.Sprintf("%-12s  %-24s  %-20s  %s\n",
		headerStyle.Render("Session"),
		headerStyle.Render("Tokens (in / out)"),
		headerStyle.Render("Cost / Cap"),
		""))

	for _, b := range todayBudgets {
		shortID := b.SessionID
		if len(shortID) > 8 {
			shortID = shortID[:8] + "..."
		}
		inK := float64(b.InputTokens) / 1000
		outK := float64(b.OutputTokens) / 1000
		cap := pol.Budget.SessionMaxUSD
		var costCap string
		if cap > 0 {
			costCap = fmt.Sprintf("$%.2f / $%.2f", b.CostUSD, cap)
		} else {
			costCap = fmt.Sprintf("$%.4f", b.CostUSD)
		}
		sb.WriteString(fmt.Sprintf("%-12s  %-24s  %s\n",
			shortID,
			fmt.Sprintf("%.1fk / %.1fk", inK, outK),
			costCap))
	}

	return sb.String()
}

// renderDenials renders the recent denials list.
func renderDenials(denials []control.Denial, width int) string {
	if len(denials) == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("No denials logged")
	}

	var sb strings.Builder
	for _, d := range denials {
		ts := d.Timestamp.Local().Format("15:04")
		reason := lipgloss.NewStyle().Foreground(colorError).Render("BLOCKED (" + d.Reason + ")")
		tool := d.Tool
		if len(tool) > 30 {
			tool = tool[:30] + "..."
		}
		sb.WriteString(fmt.Sprintf("%s  %-32s  %s\n", ts, tool, reason))
	}
	return sb.String()
}

// renderProgressBar renders a simple ASCII progress bar of width cells.
// fraction is clamped to [0,1].
func renderProgressBar(fraction float64, width int) string {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	filled := int(float64(width) * fraction)
	empty := width - filled
	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return "[" + bar + "]"
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// editorCommand builds an *exec.Cmd to open file in editor.
func editorCommand(editor, file string) *exec.Cmd {
	return exec.Command(editor, file)
}

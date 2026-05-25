package tui

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/control"
)

// examplePolicyTOML is the canonical sample policy that ships with the
// binary. Used as the seed when the user presses 'e' against a non-existent
// policy file, and rendered inline in the Control tab when no policy is
// loaded so users immediately see what's configurable.
//
//go:embed policy_example.toml
var examplePolicyTOML string

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

// openEditorCmd opens policy.toml in $EDITOR. If the file does not exist,
// it is first created from the embedded example so the user lands on a
// commented template instead of a blank file.
func (t ControlTab) openEditorCmd() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	p := t.policyPath

	// Ensure parent dir + file exist with sensible defaults before editing.
	if _, err := os.Stat(p); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(examplePolicyTOML), 0o644)
	}

	return tea.ExecProcess(editorCommand(editor, p), func(err error) tea.Msg {
		// Reload after editor exits so changes show up immediately.
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

	// First-run state — no policy AND no recorded activity. Show the
	// full setup guide. Once any data exists (e.g. user ran the hook
	// handler once and triggered a fail-open denial log entry), fall
	// through to the normal layout so they don't lose visibility.
	policyEmpty := t.policy.Budget.SessionMaxUSD == 0 && t.policy.Budget.DailyMaxUSD == 0 &&
		len(t.policy.Tools.Denylist) == 0 && len(t.policy.Tools.Allowlist) == 0
	if policyEmpty && len(t.denials) == 0 && len(t.budgets) == 0 {
		return t.renderEmptyStateGuide()
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
	sb.WriteString("\n\n")

	// Help footer (k9s-style).
	sb.WriteString(renderControlHelp(t.width))

	return lipgloss.NewStyle().
		Width(max(0, t.width)).
		Height(max(0, t.height)).
		Render(sb.String())
}

// renderEmptyStateGuide is shown when the user has not yet created a
// policy file. Inspired by k9s's "no resources found" guidance: tell the
// user what to do, where the file lives, and what's configurable.
func (t ControlTab) renderEmptyStateGuide() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	warnStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
	pathStyle := lipgloss.NewStyle().Foreground(colorText).Underline(true)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	keyStyle := lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)

	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Control Plane — runtime governance for Claude Code"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", min(t.width, 70)))
	sb.WriteString("\n\n")

	sb.WriteString(warnStyle.Render("⚠  No policy loaded — running in fail-open mode."))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("   Every tool call is allowed. No budget caps apply."))
	sb.WriteString("\n\n")

	sb.WriteString("Policy file: " + pathStyle.Render(t.policyPath))
	sb.WriteString("\n\n")

	sb.WriteString("Press " + keyStyle.Render("e") + " to create a starter file and open it in $EDITOR.")
	sb.WriteString("\n")
	sb.WriteString("Press " + keyStyle.Render("r") + " to reload after editing.")
	sb.WriteString("\n\n")

	// Section: what's configurable.
	sb.WriteString(titleStyle.Render("What's configurable"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", min(t.width, 70)))
	sb.WriteString("\n")
	sb.WriteString(renderConfigSchema())
	sb.WriteString("\n\n")

	// Section: example.
	sb.WriteString(titleStyle.Render("Example (will be written when you press 'e')"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", min(t.width, 70)))
	sb.WriteString("\n")
	sb.WriteString(renderExampleTOML(t.width))
	sb.WriteString("\n\n")

	sb.WriteString(renderControlHelp(t.width))

	return lipgloss.NewStyle().
		Width(max(0, t.width)).
		Height(max(0, t.height)).
		Render(sb.String())
}

// renderConfigSchema renders a compact reference of the available settings.
// Mirrors the structure of policy_example.toml but in tabular form.
func renderConfigSchema() string {
	keyStyle := lipgloss.NewStyle().Foreground(colorPrimary)
	descStyle := lipgloss.NewStyle().Foreground(colorText)
	var sb strings.Builder
	rows := []struct {
		key, desc string
	}{
		{"[budget]", ""},
		{"  session_max_usd", "USD cap per Claude Code session (0 = unlimited)"},
		{"  daily_max_usd", "USD cap per UTC day across all sessions"},
		{"  warn_at_fraction", "Warn (not block) at this fraction (e.g. 0.8 = 80%)"},
		{"[tools]", ""},
		{"  denylist", "Glob patterns to block — e.g. \"Bash:rm -rf*\""},
		{"  allowlist", "If non-empty, ONLY these patterns are allowed"},
		{"[models.pricing]", ""},
		{"  <model> = {input=N,output=N}", "Per-model price (USD per 1M tokens)"},
	}
	for _, r := range rows {
		if r.desc == "" {
			sb.WriteString(keyStyle.Render(r.key))
			sb.WriteString("\n")
		} else {
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				keyStyle.Render(fmt.Sprintf("%-32s", strings.TrimPrefix(r.key, "  "))),
				descStyle.Render(r.desc)))
		}
	}
	return sb.String()
}

// renderExampleTOML renders the embedded policy_example.toml with dim
// styling so it visually separates from "what you'd type".
func renderExampleTOML(width int) string {
	style := lipgloss.NewStyle().Foreground(colorDim)
	lines := strings.Split(examplePolicyTOML, "\n")
	// Trim to a reasonable preview (avoid scroll overflow on small terminals).
	const maxLines = 24
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], style.Render("  …"))
	}
	return style.Render(strings.Join(lines, "\n"))
}

// renderControlHelp renders a k9s-style key reference footer.
func renderControlHelp(width int) string {
	keyStyle := lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorDim)
	pairs := []struct{ k, d string }{
		{"e", "edit policy"},
		{"r", "reload"},
		{"c", "clear denial log"},
	}
	var parts []string
	for _, p := range pairs {
		parts = append(parts, keyStyle.Render("<"+p.k+">")+" "+descStyle.Render(p.d))
	}
	return strings.Join(parts, "  ")
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

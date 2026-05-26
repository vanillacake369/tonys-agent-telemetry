package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HooksLoadedMsg is sent when hook settings have been parsed.
type HooksLoadedMsg struct {
	Config HooksConfig
	Err    error
}

// HooksConfig represents the parsed hook/harness configuration.
type HooksConfig struct {
	Hooks      map[string][]HookGroup `json:"hooks"`
	StatusLine *StatusLineConfig      `json:"statusLine,omitempty"`
}

// HookGroup represents a matcher + list of hooks for a given event.
type HookGroup struct {
	Matcher string      `json:"matcher"`
	Hooks   []HookEntry `json:"hooks"`
}

// HookEntry is a single hook command definition.
type HookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// StatusLineConfig represents the statusLine configuration.
type StatusLineConfig struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// HooksTab implements TabModel for the Hooks/Harness visualizer tab.
type HooksTab struct {
	config   HooksConfig
	viewport viewport.Model
	width    int
	height   int
	loading  bool
	err      error
	keys     KeyMap
}

// NewHooksTab creates an initialised HooksTab.
func NewHooksTab() HooksTab {
	vp := viewport.New(80, 20)
	return HooksTab{
		viewport: vp,
		loading:  true,
		keys:     DefaultKeyMap(),
	}
}

// Init loads hook configuration from ~/.claude/settings.json.
func (h HooksTab) Init() tea.Cmd {
	return func() tea.Msg {
		config, err := loadHooksConfig()
		return HooksLoadedMsg{Config: config, Err: err}
	}
}

// loadHooksConfig reads and parses the Claude settings file.
func loadHooksConfig() (HooksConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return HooksConfig{}, fmt.Errorf("cannot determine home dir: %w", err)
	}

	path := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return HooksConfig{}, fmt.Errorf("cannot read settings: %w", err)
	}

	var raw struct {
		Hooks      map[string]json.RawMessage `json:"hooks"`
		StatusLine *StatusLineConfig          `json:"statusLine"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return HooksConfig{}, fmt.Errorf("invalid JSON: %w", err)
	}

	config := HooksConfig{
		Hooks:      make(map[string][]HookGroup),
		StatusLine: raw.StatusLine,
	}

	for event, rawGroups := range raw.Hooks {
		var groups []HookGroup
		if err := json.Unmarshal(rawGroups, &groups); err != nil {
			continue
		}
		config.Hooks[event] = groups
	}

	return config, nil
}

// Update handles messages and key events.
func (h HooksTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case HooksLoadedMsg:
		h.loading = false
		h.err = msg.Err
		if msg.Err == nil {
			h.config = msg.Config
			h.refreshViewport()
		}
		return h, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, h.keys.Refresh):
			h.loading = true
			return h, h.Init()
		}

		var vpCmd tea.Cmd
		h.viewport, vpCmd = h.viewport.Update(msg)
		return h, vpCmd
	}

	var vpCmd tea.Cmd
	h.viewport, vpCmd = h.viewport.Update(msg)
	return h, vpCmd
}

// SetSize updates stored dimensions and the viewport.
func (h HooksTab) SetSize(width, height int) TabModel {
	h.width = width
	h.height = height
	h.viewport.Width = max(1, width-2)
	h.viewport.Height = max(1, height-2)
	if !h.loading && h.err == nil {
		h.refreshViewport()
	}
	return h
}

// View renders the hooks tab.
func (h HooksTab) View() string {
	if h.loading {
		return RenderLoadingState(h.width, h.height)
	}
	if h.err != nil {
		return renderErrorState(h.err, h.width)
	}

	panelH := max(3, h.height)
	return RenderPanel("Hooks & Harness", h.viewport.View(), h.width, panelH, true)
}

// refreshViewport rebuilds the scrollable dashboard content.
func (h *HooksTab) refreshViewport() {
	contentW := max(40, h.viewport.Width-2)
	h.viewport.SetContent(h.renderDashboard(contentW))
	h.viewport.GotoTop()
}

// renderDashboard builds the full hook/harness visualization.
func (h HooksTab) renderDashboard(width int) string {
	var sb strings.Builder

	// ── Workflow Overview ──
	sb.WriteString(h.renderWorkflowDiagram(width))
	sb.WriteString("\n\n")

	// ── Event sections ──
	eventOrder := []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}
	eventIcons := map[string]string{
		"UserPromptSubmit": "📥",
		"PreToolUse":       "🛡️",
		"PostToolUse":      "📡",
		"Stop":             "🔔",
	}
	eventDesc := map[string]string{
		"UserPromptSubmit": "Triggered when user submits a prompt",
		"PreToolUse":       "Guard hooks that run before tool execution",
		"PostToolUse":      "Sensor hooks that run after tool execution",
		"Stop":             "Triggered when the agent stops",
	}

	for _, event := range eventOrder {
		groups, ok := h.config.Hooks[event]
		if !ok || len(groups) == 0 {
			continue
		}

		icon := eventIcons[event]
		desc := eventDesc[event]
		sb.WriteString(renderHookSectionHeader(icon+" "+event, desc, width))
		sb.WriteString("\n")

		for _, group := range groups {
			sb.WriteString(renderHookGroup(group, width))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// ── Status Line ──
	if h.config.StatusLine != nil {
		sb.WriteString(renderHookSectionHeader("📊 StatusLine", "Dynamic status bar command", width))
		sb.WriteString("\n")
		sb.WriteString(renderScriptEntry(h.config.StatusLine.Command, 0, width))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderWorkflowDiagram renders an ASCII art flow diagram of the harness.
func (h HooksTab) renderWorkflowDiagram(width int) string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	dimStyle := lipgloss.NewStyle().Foreground(colorDim)
	nodeStyle := lipgloss.NewStyle().Foreground(colorText)
	activeStyle := lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Complexity Harness Workflow"))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render(strings.Repeat("─", min(width, 60))))
	sb.WriteString("\n\n")

	// Count hooks per event
	counts := make(map[string]int)
	for event, groups := range h.config.Hooks {
		for _, g := range groups {
			counts[event] += len(g.Hooks)
		}
	}

	// Compact flow diagram
	steps := []struct {
		label string
		event string
		desc  string
	}{
		{"User Prompt", "UserPromptSubmit", "complexity-router injects S/M/L classification"},
		{"Pre-Tool Guard", "PreToolUse", "path-guard, cmd-guard, complexity-gate, cost-gate"},
		{"Tool Execution", "", "Claude executes Read/Write/Edit/Bash/Agent"},
		{"Post-Tool Sensor", "PostToolUse", "auto-lint, test-feedback, escalation-gate"},
		{"Agent Stop", "Stop", "agent-notify sends completion alert"},
	}

	for i, step := range steps {
		count := counts[step.event]
		prefix := "  "
		if count > 0 {
			prefix = activeStyle.Render("● ")
			label := nodeStyle.Render(fmt.Sprintf("%-18s", step.label))
			hookCount := activeStyle.Render(fmt.Sprintf("[%d hooks]", count))
			sb.WriteString(prefix + label + " " + hookCount)
		} else if step.event == "" {
			prefix = dimStyle.Render("◦ ")
			sb.WriteString(prefix + nodeStyle.Render(fmt.Sprintf("%-18s", step.label)))
		} else {
			prefix = dimStyle.Render("○ ")
			sb.WriteString(prefix + dimStyle.Render(fmt.Sprintf("%-18s", step.label)) + " " + dimStyle.Render("[0 hooks]"))
		}
		sb.WriteString("\n")
		sb.WriteString(dimStyle.Render("    " + step.desc))
		sb.WriteString("\n")

		if i < len(steps)-1 {
			sb.WriteString(dimStyle.Render("    │"))
			sb.WriteString("\n")
			sb.WriteString(dimStyle.Render("    ▼"))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderHookSectionHeader renders a section header for a hook event type.
func renderHookSectionHeader(title, description string, width int) string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	descStyle := lipgloss.NewStyle().Foreground(colorDim).Italic(true)

	header := headerStyle.Render(title)
	desc := descStyle.Render("  " + description)
	sep := lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("─", min(width, 50)))

	return header + desc + "\n" + sep
}

// renderHookGroup renders a single matcher group with its hooks.
func renderHookGroup(group HookGroup, width int) string {
	matcherStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true)

	var sb strings.Builder

	matcher := group.Matcher
	if matcher == "" {
		matcher = "*"
	}
	sb.WriteString("  " + matcherStyle.Render("matcher: "+matcher))
	sb.WriteString("\n")

	for _, hook := range group.Hooks {
		sb.WriteString(renderScriptEntry(hook.Command, hook.Timeout, width))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderScriptEntry renders a single hook script entry with its details.
func renderScriptEntry(command string, timeout int, width int) string {
	scriptStyle := lipgloss.NewStyle().Foreground(colorText)
	metaStyle := lipgloss.NewStyle().Foreground(colorDim)

	// Extract just the script name from the full path
	name := filepath.Base(command)
	// Check if script exists
	expandedCmd := expandHome(command)
	status := colorSuccess
	statusIcon := "✓"
	if _, err := os.Stat(expandedCmd); err != nil {
		status = colorError
		statusIcon = "✗"
	}
	statusStyle := lipgloss.NewStyle().Foreground(status)

	var meta string
	if timeout > 0 {
		meta = fmt.Sprintf("timeout: %ds", timeout)
	}

	line := fmt.Sprintf("    %s %s", statusStyle.Render(statusIcon), scriptStyle.Render(name))
	if meta != "" {
		line += "  " + metaStyle.Render(meta)
	}

	return line
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

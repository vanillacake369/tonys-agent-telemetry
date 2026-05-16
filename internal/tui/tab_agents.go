package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/platform"
)

// AgentsLoadedMsg is sent when agent discovery completes.
type AgentsLoadedMsg struct {
	Agents []data.Agent
	Err    error
}

// AgentPreviewMsg is sent when a preview file has been read.
type AgentPreviewMsg struct {
	Content string
	Err     error
}

// agentIcon returns the icon to prepend to an agent's name in the list.
func agentIcon(name string) string {
	switch strings.ToLower(name) {
	case "architect":
		return "🏗️"
	case "implementer":
		return "⚙️"
	case "reviewer":
		return "🔎"
	case "tester":
		return "🧪"
	case "refactorer":
		return "♻️"
	case "researcher":
		return "🔍"
	case "cross-validator":
		return "✅"
	default:
		return "🤖"
	}
}

// AgentsTab implements TabModel for the Agents tab.
// It shows a fuzzy-searchable list of discovered agents with a preview pane.
type AgentsTab struct {
	agents      []data.Agent
	filtered    []data.Agent
	cursor      int
	searchInput textinput.Model
	preview     string
	width       int
	height      int
	loading     bool
	err         error
	keys        KeyMap
}

// NewAgentsTab creates a new AgentsTab ready to be displayed.
func NewAgentsTab() AgentsTab {
	ti := textinput.New()
	ti.Placeholder = "search agents... (press / to focus)"
	ti.CharLimit = 64
	ti.Width = 40

	return AgentsTab{
		searchInput: ti,
		loading:     true,
		keys:        DefaultKeyMap(),
	}
}

// loadAgentsCmd returns a Cmd that discovers agents asynchronously.
func loadAgentsCmd() tea.Cmd {
	return func() tea.Msg {
		agents, err := data.DiscoverAgents()
		return AgentsLoadedMsg{Agents: agents, Err: err}
	}
}

// loadPreviewCmd returns a Cmd that reads the .md file for the given agent name.
func loadPreviewCmd(name string) tea.Cmd {
	return func() tea.Msg {
		path := filepath.Join(data.ClaudeDir(), "agents", name+".md")
		raw, err := os.ReadFile(path)
		if err != nil {
			return AgentPreviewMsg{Content: "", Err: err}
		}
		content := string(raw)
		if len(content) > 2000 {
			content = content[:2000] + "\n…(truncated)"
		}
		return AgentPreviewMsg{Content: content}
	}
}

// applyFilter rebuilds filtered from agents using the current search query.
// An empty query returns all agents unchanged.
func (t *AgentsTab) applyFilter() {
	query := t.searchInput.Value()
	if query == "" {
		t.filtered = make([]data.Agent, len(t.agents))
		copy(t.filtered, t.agents)
		return
	}

	// Build strings to search: "name type description"
	sources := make([]string, len(t.agents))
	for i, a := range t.agents {
		sources[i] = strings.Join([]string{a.Name, a.Type, a.Description}, " ")
	}

	results := fuzzy.Find(query, sources)
	t.filtered = make([]data.Agent, 0, len(results))
	for _, r := range results {
		t.filtered = append(t.filtered, t.agents[r.Index])
	}
}

// clampCursor ensures cursor stays within filtered bounds.
func (t *AgentsTab) clampCursor() {
	if len(t.filtered) == 0 {
		t.cursor = 0
		return
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
	if t.cursor >= len(t.filtered) {
		t.cursor = len(t.filtered) - 1
	}
}

// selectedAgent returns the currently highlighted agent, or nil if none.
func (t AgentsTab) selectedAgent() *data.Agent {
	if len(t.filtered) == 0 || t.cursor < 0 || t.cursor >= len(t.filtered) {
		return nil
	}
	a := t.filtered[t.cursor]
	return &a
}

// Init starts the async agent discovery.
func (t AgentsTab) Init() tea.Cmd {
	return tea.Batch(loadAgentsCmd(), textinput.Blink)
}

// Update handles messages and key presses.
func (t AgentsTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case AgentsLoadedMsg:
		t.loading = false
		t.err = msg.Err
		if msg.Err == nil {
			t.agents = msg.Agents
		}
		t.applyFilter()
		t.clampCursor()
		if a := t.selectedAgent(); a != nil {
			cmds = append(cmds, loadPreviewCmd(a.Name))
		}
		return t, tea.Batch(cmds...)

	case AgentPreviewMsg:
		if msg.Err != nil {
			t.preview = "(no preview available)"
		} else {
			t.preview = msg.Content
		}
		return t, nil

	case SearchFocusMsg:
		t.searchInput.Focus()
		return t, textinput.Blink

	case SearchBlurMsg:
		t.searchInput.Blur()
		return t, nil

	case tea.KeyMsg:
		// When search input is focused, forward keys to the text input.
		if t.searchInput.Focused() {
			prevQuery := t.searchInput.Value()
			var inputCmd tea.Cmd
			t.searchInput, inputCmd = t.searchInput.Update(msg)
			cmds = append(cmds, inputCmd)

			if t.searchInput.Value() != prevQuery {
				t.applyFilter()
				t.cursor = 0
				t.clampCursor()
				if a := t.selectedAgent(); a != nil {
					cmds = append(cmds, loadPreviewCmd(a.Name))
				}
			}
			return t, tea.Batch(cmds...)
		}

		// Navigation mode: handle single-key bindings.
		switch {
		case key.Matches(msg, t.keys.Refresh):
			t.loading = true
			t.agents = nil
			t.filtered = nil
			t.preview = ""
			t.cursor = 0
			return t, loadAgentsCmd()

		case key.Matches(msg, t.keys.Enter):
			if a := t.selectedAgent(); a != nil {
				cmd := fmt.Sprintf("claude --agent %s", a.Name)
				if err := platform.Detect().OpenPane(cmd); err != nil {
					t.err = err
				}
			}
			return t, nil

		case key.Matches(msg, t.keys.Copy):
			if a := t.selectedAgent(); a != nil {
				if err := platform.CopyToClipboard(a.Name); err != nil {
					t.err = err
				}
			}
			return t, nil

		case key.Matches(msg, t.keys.Up):
			t.cursor--
			t.clampCursor()
			if a := t.selectedAgent(); a != nil {
				cmds = append(cmds, loadPreviewCmd(a.Name))
			}
			return t, tea.Batch(cmds...)

		case key.Matches(msg, t.keys.Down):
			t.cursor++
			t.clampCursor()
			if a := t.selectedAgent(); a != nil {
				cmds = append(cmds, loadPreviewCmd(a.Name))
			}
			return t, tea.Batch(cmds...)
		}

		return t, nil
	}

	// Forward to textinput for cursor blink etc.
	var inputCmd tea.Cmd
	t.searchInput, inputCmd = t.searchInput.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}
	return t, tea.Batch(cmds...)
}

// View renders the agents tab as a split layout.
func (t AgentsTab) View() string {
	if t.width == 0 || t.height == 0 {
		if t.loading {
			return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("loading agents...")
		}
		return "Agents\n" + t.renderList(40, 10)
	}

	// Layout:
	//   search bar : 1 line
	//   gap        : 1 line
	//   list+preview: remaining height - 1 (hint)
	//   hint bar   : 1 line
	const hintHeight = 1
	const searchHeight = 1
	const gapHeight = 1
	listHeight := max(1, t.height-searchHeight-gapHeight-hintHeight)

	t.searchInput.Width = max(1, t.width-6)
	searchBar := RenderSearchBar(t.searchInput, t.width, "")

	leftW, rightW, showPreview := SplitLayout(t.width, 50)

	var splitView string
	if showPreview {
		leftContent := t.renderList(max(1, leftW-2), max(1, listHeight-2))
		leftPanel := RenderPanel("Agents", leftContent, leftW, listHeight, true)
		rightContent := t.renderPreview(max(1, rightW-2), max(1, listHeight-2))
		rightPanel := RenderPanel("Preview", rightContent, rightW, listHeight, false)
		splitView = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	} else {
		leftContent := t.renderList(max(1, leftW-2), max(1, listHeight-2))
		splitView = RenderPanel("Agents", leftContent, leftW, listHeight, true)
	}
	hintBar := RenderHintBar("↵:launch  y:copy  r:refresh  ↑/↓ or j/k:navigate", t.width)

	return strings.Join([]string{searchBar, "", splitView, hintBar}, "\n")
}

// renderList returns the agent list lines truncated to fit within the pane.
func (t AgentsTab) renderList(width, height int) string {
	if t.loading {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("loading agents...")
	}
	if t.err != nil && len(t.filtered) == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("no agents found")
	}
	if len(t.filtered) == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("no results")
	}

	// Scroll window: centre cursor in the visible region.
	start := t.cursor - height/2
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(t.filtered) {
		end = len(t.filtered)
		start = end - height
		if start < 0 {
			start = 0
		}
	}

	var lines []string
	for i := start; i < end; i++ {
		a := t.filtered[i]
		icon := agentIcon(a.Name)
		model := a.Model
		if model == "" {
			model = "unknown"
		}
		// Abbreviate model: show only last segment after last "-"
		if idx := strings.LastIndex(model, "-"); idx >= 0 {
			model = model[idx+1:]
		}
		desc := a.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}

		line := fmt.Sprintf("%s  %-20s │ %-8s │ %s", icon, a.Name, model, desc)

		// Truncate to fit width.
		if len(line) > width {
			line = line[:width]
		}

		lines = append(lines, RenderListItem(line, i == t.cursor, width+2))
	}

	return strings.Join(lines, "\n")
}

// renderPreview returns the preview pane content for the selected agent.
func (t AgentsTab) renderPreview(width, height int) string {
	if t.loading {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("loading...")
	}
	if len(t.filtered) == 0 {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("select an agent")
	}
	if t.preview == "" {
		return lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render("no preview available")
	}
	return wrapLines(t.preview, width, height)
}

// SetSize stores the new terminal dimensions.
func (t AgentsTab) SetSize(width, height int) TabModel {
	t.width = width
	t.height = height
	t.searchInput.Width = max(1, width-6)
	return t
}

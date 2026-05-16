package tui

import (
	"context"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
)

// Tab represents the active tab in the TUI.
type Tab int

const (
	TabSessions Tab = iota
	TabAgents
	TabDAG
	TabSkills
)

// tabNames maps each Tab constant to its display label.
var tabNames = map[Tab]string{
	TabSessions: "Sessions",
	TabAgents:   "Agents",
	TabDAG:      "DAG",
	TabSkills:   "Skills",
}

// tabOrder defines the left-to-right display order of tabs.
var tabOrder = []Tab{TabSessions, TabAgents, TabDAG, TabSkills}

// TabModel is the interface that every tab sub-model must implement.
// SetSize returns the updated model (value-receiver implementations must
// return their updated copy so the caller can store it).
type TabModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (TabModel, tea.Cmd)
	View() string
	SetSize(width, height int) TabModel
}

// App is the root Bubble Tea model managing tab switching.
type App struct {
	activeTab  Tab
	tabs       map[Tab]TabModel
	keys       KeyMap
	width      int
	height     int
	fifoEvents <-chan event.Event // nil when FIFO is not active
	fifoCancel context.CancelFunc
}

const (
	tabBarHeight    = 1
	statusBarHeight = 1
)

// NewApp creates and returns an initialised App with placeholder tab models.
func NewApp() App {
	keys := DefaultKeyMap()
	tabs := map[Tab]TabModel{
		TabSessions: NewSessionsTab(),
		TabAgents:   NewAgentsTab(),
		TabDAG:      NewDAGTab(),
		TabSkills:   NewSkillsTab(),
	}
	return App{
		activeTab: TabSessions,
		tabs:      tabs,
		keys:      keys,
	}
}

func (a App) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, m := range a.tabs {
		if cmd := m.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	// Start listening for real-time events if the TUI FIFO exists.
	if info, err := os.Stat(event.DefaultFIFOPath); err == nil && info.Mode()&os.ModeNamedPipe != 0 {
		ctx, cancel := context.WithCancel(context.Background())
		a.fifoCancel = cancel
		a.fifoEvents = event.ReadFIFO(ctx)
		cmds = append(cmds, event.ListenForEvents(ctx, a.fifoEvents))
	}

	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a = a.propagateSize()
		return a, nil

	case event.EventMsg:
		// Forward real-time events to the active tab and keep listening.
		updated, cmd := a.tabs[a.activeTab].Update(msg)
		a.tabs[a.activeTab] = updated
		var listenCmd tea.Cmd
		if a.fifoEvents != nil {
			listenCmd = event.ListenForEvents(context.Background(), a.fifoEvents)
		}
		return a, tea.Batch(cmd, listenCmd)

	case event.FIFOClosedMsg:
		// FIFO channel closed — stop subscribing.
		a.fifoEvents = nil
		return a, nil

	case tea.KeyMsg:
		// Global tab-switching keys are intercepted before delegating to sub-models.
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.TabSessions):
			a.activeTab = TabSessions
			return a, nil
		case key.Matches(msg, a.keys.TabAgents):
			a.activeTab = TabAgents
			return a, nil
		case key.Matches(msg, a.keys.TabDAG):
			a.activeTab = TabDAG
			return a, nil
		case key.Matches(msg, a.keys.TabSkills):
			a.activeTab = TabSkills
			return a, nil
		}
	}

	// Delegate remaining messages to the active tab.
	updated, cmd := a.tabs[a.activeTab].Update(msg)
	a.tabs[a.activeTab] = updated
	return a, cmd
}

func (a App) View() string {
	if a.width == 0 {
		// Not yet sized — render a minimal placeholder to avoid blank screen.
		return renderTabBar(a.activeTab, 80) + "\n" + a.tabs[a.activeTab].View()
	}

	tabBar := renderTabBar(a.activeTab, a.width)
	content := ContentStyle.
		Width(a.width).
		Height(a.contentHeight()).
		Render(a.tabs[a.activeTab].View())
	statusBar := a.renderStatusBar(a.width)

	return strings.Join([]string{tabBar, content, statusBar}, "\n")
}

// contentHeight returns the number of rows available for tab content.
func (a App) contentHeight() int {
	h := a.height - tabBarHeight - statusBarHeight
	if h < 0 {
		return 0
	}
	return h
}

// propagateSize distributes the current terminal dimensions to every tab model.
func (a App) propagateSize() App {
	cw := a.width
	ch := a.contentHeight()
	for tab, m := range a.tabs {
		a.tabs[tab] = m.SetSize(cw, ch)
	}
	return a
}

// tabHints returns the context-sensitive hint string for the active tab.
func (a App) tabHints() string {
	switch a.activeTab {
	case TabSessions:
		return "Enter:resume  ^F:fork  ^Y:copy  ^R:refresh"
	case TabAgents:
		return "Enter:launch  ^Y:copy  ^R:refresh"
	case TabDAG:
		return "◄►:switch  ↑↓:scroll"
	case TabSkills:
		return "Enter:analyze  ^T:sort  ^Y:copy  ^R:refresh"
	}
	return ""
}

// renderTabBar returns the tab bar string for the given active tab and total width.
func renderTabBar(active Tab, width int) string {
	var parts []string
	for i, tab := range tabOrder {
		label := tabNames[tab]
		var rendered string
		if tab == active {
			rendered = ActiveTabStyle.Render(label)
		} else {
			rendered = InactiveTabStyle.Render(label)
		}
		if i < len(tabOrder)-1 {
			rendered += TabSeparatorStyle.Render(tabSeparator)
		}
		parts = append(parts, rendered)
	}
	bar := strings.Join(parts, "")
	return TabBarStyle.Width(width).Render(bar)
}

// renderStatusBar returns a merged status bar showing both global and tab hints.
func (a App) renderStatusBar(width int) string {
	globalHints := "^S:sessions  ^A:agents  ^D:dag  ^K:skills"
	tabSpecific := a.tabHints()
	quitHint := "q:quit"

	var parts []string
	parts = append(parts, globalHints)
	if tabSpecific != "" {
		parts = append(parts, tabSpecific)
	}
	parts = append(parts, quitHint)

	help := strings.Join(parts, "  │  ")
	innerWidth := max(0, width-StatusBarStyle.GetHorizontalPadding())
	return StatusBarStyle.Width(width).Render(
		lipgloss.PlaceHorizontal(innerWidth, lipgloss.Left, help),
	)
}

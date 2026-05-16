package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	activeTab Tab
	tabs      map[Tab]TabModel
	keys      KeyMap
	width     int
	height    int
}

const (
	tabBarHeight    = 1
	statusBarHeight = 1
)

// NewApp creates and returns an initialised App with placeholder tab models.
func NewApp() App {
	keys := DefaultKeyMap()
	tabs := map[Tab]TabModel{
		TabSessions: newPlaceholderTab("Sessions"),
		TabAgents:   newPlaceholderTab("Agents"),
		TabDAG:      newPlaceholderTab("DAG"),
		TabSkills:   newPlaceholderTab("Skills"),
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
	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a = a.propagateSize()
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
		Width(a.width - ContentStyle.GetHorizontalPadding()).
		Height(a.contentHeight()).
		Render(a.tabs[a.activeTab].View())
	statusBar := renderStatusBar(a.width)

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
	cw := a.width - ContentStyle.GetHorizontalPadding()
	ch := a.contentHeight()
	for tab, m := range a.tabs {
		a.tabs[tab] = m.SetSize(cw, ch)
	}
	return a
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

// renderStatusBar returns the status bar string with global key hints.
func renderStatusBar(width int) string {
	help := "^S:sessions  ^A:agents  ^D:dag  ^K:skills  q:quit"
	return StatusBarStyle.Width(width).Render(
		lipgloss.PlaceHorizontal(width-StatusBarStyle.GetHorizontalPadding(), lipgloss.Left, help),
	)
}


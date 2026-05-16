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

// SearchFocusMsg is sent to a tab to tell it to focus its search input.
type SearchFocusMsg struct{}

// SearchBlurMsg is sent to a tab to tell it to blur its search input.
type SearchBlurMsg struct{}

// App is the root Bubble Tea model managing tab switching.
type App struct {
	activeTab     Tab
	tabs          map[Tab]TabModel
	keys          KeyMap
	width         int
	height        int
	searchFocused bool // when true, key events pass directly to the active tab
	fifoEvents    <-chan event.Event // nil when FIFO is not active
	fifoCancel    context.CancelFunc
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
		activeTab:     TabSessions,
		tabs:          tabs,
		keys:          keys,
		searchFocused: false,
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
		// Esc always unfocuses search, regardless of current mode.
		if key.Matches(msg, a.keys.Escape) {
			if a.searchFocused {
				a.searchFocused = false
				updated, cmd := a.tabs[a.activeTab].Update(SearchBlurMsg{})
				a.tabs[a.activeTab] = updated
				return a, cmd
			}
			return a, nil
		}

		// Ctrl+C always quits.
		if msg.Type == tea.KeyCtrlC {
			return a, tea.Quit
		}

		// Tab / Shift+Tab cycle tabs regardless of search focus.
		if key.Matches(msg, a.keys.NextTab) {
			a.activeTab = (a.activeTab + 1) % 4
			return a, nil
		}
		if key.Matches(msg, a.keys.PrevTab) {
			a.activeTab = (a.activeTab + 3) % 4
			return a, nil
		}

		// When search is focused, pass all remaining keys to the active tab.
		if a.searchFocused {
			updated, cmd := a.tabs[a.activeTab].Update(msg)
			a.tabs[a.activeTab] = updated
			return a, cmd
		}

		// Navigation mode: "/" focuses search.
		if key.Matches(msg, a.keys.Search) {
			a.searchFocused = true
			updated, cmd := a.tabs[a.activeTab].Update(SearchFocusMsg{})
			a.tabs[a.activeTab] = updated
			return a, cmd
		}

		// Navigation mode: number keys switch tabs.
		switch {
		case key.Matches(msg, a.keys.Tab1):
			a.activeTab = TabSessions
			return a, nil
		case key.Matches(msg, a.keys.Tab2):
			a.activeTab = TabAgents
			return a, nil
		case key.Matches(msg, a.keys.Tab3):
			a.activeTab = TabDAG
			return a, nil
		case key.Matches(msg, a.keys.Tab4):
			a.activeTab = TabSkills
			return a, nil
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		}

		// Delegate all other navigation-mode keys to the active tab.
		updated, cmd := a.tabs[a.activeTab].Update(msg)
		a.tabs[a.activeTab] = updated
		return a, cmd
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
		return "↵:resume  f:fork  y:copy  r:refresh"
	case TabAgents:
		return "↵:launch  y:copy  r:refresh"
	case TabDAG:
		return "◄►:switch  ↑↓:scroll"
	case TabSkills:
		return "↵:analyze  s:sort  y:copy  r:refresh"
	}
	return ""
}

// renderTabBar returns the tab bar string for the given active tab and total width.
// Uses k9s/btop-style numbered tabs: "1:Sessions │ 2:Agents │ 3:DAG │ 4:Skills"
func renderTabBar(active Tab, width int) string {
	tabDefs := []struct {
		num string
		tab Tab
	}{
		{"1", TabSessions},
		{"2", TabAgents},
		{"3", TabDAG},
		{"4", TabSkills},
	}

	var parts []string
	for i, td := range tabDefs {
		label := td.num + ":" + tabNames[td.tab]
		var rendered string
		if td.tab == active {
			rendered = ActiveTabStyle.Render(label)
		} else {
			rendered = InactiveTabStyle.Render(label)
		}
		if i < len(tabDefs)-1 {
			rendered += TabSeparatorStyle.Render(tabSeparator)
		}
		parts = append(parts, rendered)
	}
	bar := strings.Join(parts, "")
	return TabBarStyle.Width(width).Render(bar)
}

// renderStatusBar returns a merged status bar showing mode indicator and key hints.
func (a App) renderStatusBar(width int) string {
	var help string

	if a.searchFocused {
		// Search mode indicator.
		help = " SEARCH │ type to filter │ esc:back"
	} else {
		// Normal mode: show tab switching hints and context-specific hints.
		globalHints := "NORMAL │ 1:sessions 2:agents 3:dag 4:skills"
		tabSpecific := a.tabHints()
		quitHint := "/:search  q:quit"

		var parts []string
		parts = append(parts, globalHints)
		if tabSpecific != "" {
			parts = append(parts, tabSpecific)
		}
		parts = append(parts, quitHint)

		help = strings.Join(parts, "  │  ")
	}

	innerWidth := max(0, width-StatusBarStyle.GetHorizontalPadding())
	return StatusBarStyle.Width(width).Render(
		lipgloss.PlaceHorizontal(innerWidth, lipgloss.Left, help),
	)
}

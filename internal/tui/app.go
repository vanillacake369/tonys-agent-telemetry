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
	whichKey      WhichKeyOverlay
	fifoEvents    <-chan event.Event // nil when FIFO is not active
	fifoCtx       context.Context
	fifoCancel    context.CancelFunc
}

const (
	tabBarHeight    = 1
	statusBarHeight = 1
	// outerBorderHeight accounts for top+bottom border lines of the outer frame.
	outerBorderHeight = 2
	// outerBorderWidth accounts for left+right border chars of the outer frame.
	outerBorderWidth = 2
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
	ctx, cancel := context.WithCancel(context.Background())
	return App{
		activeTab:     TabSessions,
		tabs:          tabs,
		keys:          keys,
		searchFocused: false,
		fifoCtx:       ctx,
		fifoCancel:    cancel,
	}
}

// CancelFIFO cancels the FIFO context, stopping any goroutines that listen for
// real-time events. Safe to call multiple times. Called from main.go after
// p.Run() returns as defense-in-depth alongside the QuitMsg intercept.
func (a App) CancelFIFO() {
	if a.fifoCancel != nil {
		a.fifoCancel()
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
		a.fifoEvents = event.ReadFIFO(a.fifoCtx)
		cmds = append(cmds, event.ListenForEvents(a.fifoCtx, a.fifoEvents))
	}

	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.QuitMsg); ok {
		if a.fifoCancel != nil {
			a.fifoCancel()
		}
	}

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
		if a.fifoEvents != nil && a.fifoCtx != nil {
			listenCmd = event.ListenForEvents(a.fifoCtx, a.fifoEvents)
		}
		return a, tea.Batch(cmd, listenCmd)

	case event.FIFOClosedMsg:
		// FIFO channel closed — stop subscribing.
		a.fifoEvents = nil
		return a, nil

	case tea.KeyMsg:
		// Ctrl+C always quits, even when overlay or search is active.
		if msg.Type == tea.KeyCtrlC {
			return a, tea.Quit
		}

		// When the which-key overlay is visible, any keypress closes it.
		if a.whichKey.visible {
			a.whichKey.visible = false
			return a, nil
		}

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

		// Navigation mode: "?" opens the which-key overlay.
		if key.Matches(msg, a.keys.Help) {
			a.whichKey.visible = true
			return a, nil
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

	// Broadcast non-key messages to ALL tabs.
	// Each tab's Init() fires at App startup, but the resulting messages
	// (e.g. LocalSkillsLoadedMsg, SessionsLoadedMsg) arrive asynchronously.
	// They must reach the tab that sent the Init(), not just the active tab.
	var cmds []tea.Cmd
	for tab, m := range a.tabs {
		updated, cmd := m.Update(msg)
		a.tabs[tab] = updated
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return a, tea.Batch(cmds...)
}

func (a App) View() string {
	if a.width == 0 {
		// Not yet sized — render a minimal placeholder to avoid blank screen.
		return renderTabBar(a.activeTab, 80) + "\n" + a.tabs[a.activeTab].View()
	}

	// Inner width/height: subtract outer border chars.
	innerW := max(0, a.width-outerBorderWidth)

	tabBar := renderTabBar(a.activeTab, innerW)
	content := ContentStyle.
		Width(innerW).
		Height(a.contentHeight()).
		Render(a.tabs[a.activeTab].View())
	statusBar := a.renderStatusBar(innerW)

	inner := strings.Join([]string{tabBar, content, statusBar}, "\n")

	// Switch outer border color based on mode: primary (bright) in SEARCH mode.
	outerStyle := OuterBorderStyle
	if a.searchFocused {
		outerStyle = outerStyle.BorderForeground(colorPrimary)
	}

	full := outerStyle.
		Width(innerW).
		Render(inner)

	// Render the which-key overlay centered on top of the full view.
	if a.whichKey.visible {
		a.whichKey.width = a.width
		a.whichKey.height = a.height
		overlay := a.whichKey.View()
		return lipgloss.Place(
			a.width, a.height,
			lipgloss.Center, lipgloss.Center,
			overlay,
			lipgloss.WithWhitespaceForeground(colorDim),
		)
	}

	return full
}

// contentHeight returns the number of rows available for tab content.
// It subtracts the tab bar, status bar, and the two outer border rows.
func (a App) contentHeight() int {
	h := a.height - tabBarHeight - statusBarHeight - outerBorderHeight
	if h < 0 {
		return 0
	}
	return h
}

// propagateSize distributes the current terminal dimensions to every tab model.
func (a App) propagateSize() App {
	cw := max(0, a.width-outerBorderWidth)
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
		return "↵:analyze  o:open  s:sort  y:copy  r:refresh"
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
			// Prepend a dot indicator to the active tab.
			rendered = ActiveTabStyle.Render("● " + label)
		} else {
			// Pad with spaces to align with the dot indicator.
			rendered = InactiveTabStyle.Render("  " + label)
		}
		if i < len(tabDefs)-1 {
			rendered += TabSeparatorStyle.Render(tabSeparator)
		}
		parts = append(parts, rendered)
	}
	bar := strings.Join(parts, "")
	return TabBarStyle.Width(width).Render(bar)
}

// renderStatusBar returns a single-line status bar showing mode indicator and key hints.
// Format (normal): NORMAL │ 1:sessions 2:agents 3:dag 4:skills │ <tab hints> │ /:search ?:help q:quit
// Format (search): SEARCH │ type to filter │ esc:back
func (a App) renderStatusBar(width int) string {
	innerWidth := max(0, width-StatusBarStyle.GetHorizontalPadding())

	var help string

	if a.searchFocused {
		// Search mode: bold label with background + hints.
		modeStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1A1A2E")).
			Background(colorPrimary).
			Padding(0, 1)
		mode := modeStyle.Render("SEARCH")
		help = mode + " type to filter │ esc:back"
	} else {
		// Normal mode: responsive status bar — omit sections as width shrinks.
		modeStyle := lipgloss.NewStyle().
			Foreground(colorDim).
			Padding(0, 1)
		mode := modeStyle.Render("NORMAL")
		tabSpecific := a.tabHints()

		// Build from right to left, dropping sections if they don't fit.
		// Priority: mode > tab hints > global hints > tab numbers
		globalHint := "/ search  ? help  q quit"
		tabNums := "1-4:tabs"

		// Try full version first
		var parts []string
		parts = append(parts, mode)
		if innerWidth > 100 {
			parts = append(parts, tabNums)
		}
		if tabSpecific != "" {
			parts = append(parts, tabSpecific)
		}
		if innerWidth > 60 {
			parts = append(parts, globalHint)
		}

		help = strings.Join(parts, " │ ")
	}

	return StatusBarStyle.Width(width).Render(
		lipgloss.PlaceHorizontal(innerWidth, lipgloss.Left, help),
	)
}

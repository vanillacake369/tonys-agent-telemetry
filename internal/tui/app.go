package tui

import tea "github.com/charmbracelet/bubbletea"

// Tab represents the active tab in the TUI.
type Tab int

const (
	TabSessions Tab = iota
	TabAgents
	TabDAG
	TabSkills
)

// App is the root Bubble Tea model managing tab switching.
type App struct {
	activeTab Tab
	width     int
	height    int
}

func NewApp() App {
	return App{activeTab: TabSessions}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "ctrl+s":
			a.activeTab = TabSessions
		case "ctrl+a":
			a.activeTab = TabAgents
		case "ctrl+d":
			a.activeTab = TabDAG
		case "ctrl+k":
			a.activeTab = TabSkills
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
	}
	return a, nil
}

func (a App) View() string {
	tabs := []string{"Sessions", "Agents", "DAG", "Skills"}
	header := ""
	for i, name := range tabs {
		if Tab(i) == a.activeTab {
			header += "[" + name + "] "
		} else {
			header += " " + name + "  "
		}
	}
	header += "\n"

	body := "TODO: implement tab content"

	return header + "\n" + body
}

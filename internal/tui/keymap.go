package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the TUI.
type KeyMap struct {
	TabSessions key.Binding
	TabAgents   key.Binding
	TabDAG      key.Binding
	TabSkills   key.Binding
	Quit        key.Binding
	Enter       key.Binding
	Back        key.Binding
	ForkSession key.Binding
	NewSession  key.Binding
	CopyClip    key.Binding
	Refresh     key.Binding
}

// DefaultKeyMap returns the default key bindings for the application.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		TabSessions: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("^S", "sessions"),
		),
		TabAgents: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("^A", "agents"),
		),
		TabDAG: key.NewBinding(
			key.WithKeys("ctrl+d"),
			key.WithHelp("^D", "dag"),
		),
		TabSkills: key.NewBinding(
			key.WithKeys("ctrl+k"),
			key.WithHelp("^K", "skills"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		ForkSession: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("^F", "fork session"),
		),
		NewSession: key.NewBinding(
			key.WithKeys("ctrl+n"),
			key.WithHelp("^N", "new session"),
		),
		CopyClip: key.NewBinding(
			key.WithKeys("ctrl+y"),
			key.WithHelp("^Y", "copy"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("^R", "refresh"),
		),
	}
}

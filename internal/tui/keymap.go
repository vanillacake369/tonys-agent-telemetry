package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the TUI.
type KeyMap struct {
	// Tab switching (work only when search is unfocused)
	Tab1       key.Binding // "1" → Sessions
	Tab2       key.Binding // "2" → Skills
	Tab3       key.Binding // "3" → Cost
	TabControl key.Binding // "ctrl+g" → Control (Governance)
	NextTab    key.Binding // "tab" → cycle forward
	PrevTab    key.Binding // "shift+tab" → cycle backward

	// Navigation
	Up    key.Binding // "k", "up"
	Down  key.Binding // "j", "down"
	Left  key.Binding // "h", "left"
	Right key.Binding // "l", "right"

	// Actions (single-key, only when search is unfocused)
	Enter   key.Binding // "enter"
	View    key.Binding // "v" → open detail overlay
	Copy    key.Binding // "y"
	Refresh key.Binding // "r"
	Fork    key.Binding // "f"
	Sort    key.Binding // "s"
	Open    key.Binding // "o" → open in browser
	Search  key.Binding // "/" → focus search input
	Escape  key.Binding // "esc" → unfocus search input
	Help    key.Binding // "?"
	Quit    key.Binding // "q", "ctrl+c"
}

// DefaultKeyMap returns the default key bindings for the application.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Tab1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "sessions"),
		),
		Tab2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "skills"),
		),
		Tab3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "cost"),
		),
		TabControl: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("ctrl+g", "control"),
		),
		NextTab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev tab"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		View: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "view detail"),
		),
		Copy: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Fork: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "fork session"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "sort"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

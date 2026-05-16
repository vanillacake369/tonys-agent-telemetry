package tui

import "github.com/charmbracelet/lipgloss"

var (
	// primaryColor is the accent color used for active elements.
	primaryColor = lipgloss.AdaptiveColor{Light: "#5C4AE4", Dark: "#7B6CF6"}

	// dimColor is used for inactive/secondary elements.
	dimColor = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#555555"}

	// borderColor is used for separators and borders.
	borderColor = lipgloss.AdaptiveColor{Light: "#D9D9D9", Dark: "#383838"}

	// statusBgColor is the background for the status bar.
	statusBgColor = lipgloss.AdaptiveColor{Light: "#EFEFEF", Dark: "#1A1A2E"}

	// ActiveTabStyle renders the currently selected tab label.
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Underline(true).
			Foreground(primaryColor).
			Padding(0, 1)

	// InactiveTabStyle renders unselected tab labels.
	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(dimColor).
				Padding(0, 1)

	// TabBarStyle wraps the entire tab bar row.
	TabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(borderColor)

	// ContentStyle wraps the main content area between the tab bar and status bar.
	ContentStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// StatusBarStyle renders the bottom help/status row.
	StatusBarStyle = lipgloss.NewStyle().
			Background(statusBgColor).
			Foreground(dimColor).
			Padding(0, 1)

	// TabSeparatorStyle renders the vertical separator between tab labels.
	TabSeparatorStyle = lipgloss.NewStyle().
				Foreground(dimColor)
)

// tabSeparator is the character placed between tab labels.
const tabSeparator = " │ "

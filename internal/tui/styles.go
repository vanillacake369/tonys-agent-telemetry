package tui

import "github.com/charmbracelet/lipgloss"

var (
	// colorPrimary is the accent color used for active elements.
	colorPrimary = lipgloss.AdaptiveColor{Light: "#5A4FCF", Dark: "#8B7CF6"}

	// colorDim is used for inactive/secondary elements.
	colorDim = lipgloss.AdaptiveColor{Light: "#777777", Dark: "#666666"}

	// colorBorder is used for separators and borders.
	colorBorder = lipgloss.AdaptiveColor{Light: "#E0E0E0", Dark: "#333333"}

	// colorBg is the background for the status bar.
	colorBg = lipgloss.AdaptiveColor{Light: "#F5F5F5", Dark: "#1A1A2E"}

	// colorSuccess is used for successful status indicators.
	colorSuccess = lipgloss.AdaptiveColor{Light: "#2D8A2D", Dark: "#4CAF50"}

	// colorWarning is used for warning status indicators.
	colorWarning = lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFC107"}

	// colorError is used for error status indicators.
	colorError = lipgloss.AdaptiveColor{Light: "#CC0000", Dark: "#FF5555"}

	// colorText is the default text color.
	colorText = lipgloss.AdaptiveColor{Light: "#333333", Dark: "#E0E0E0"}

	// Keep legacy aliases so other files that still reference them compile.
	primaryColor  = colorPrimary
	dimColor      = colorDim
	borderColor   = colorBorder
	statusBgColor = colorBg

	// ActiveTabStyle renders the currently selected tab label.
	// Bold + primary color with a dot indicator prefix (applied in renderTabBar).
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	// InactiveTabStyle renders unselected tab labels.
	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorDim).
				Padding(0, 1)

	// TabBarStyle wraps the entire tab bar row.
	// No border — clean text-only row.
	TabBarStyle = lipgloss.NewStyle()

	// ContentStyle wraps the main content area.
	// No padding — tabs fill the entire allocated space.
	ContentStyle = lipgloss.NewStyle()

	// StatusBarStyle renders the bottom help/status row.
	StatusBarStyle = lipgloss.NewStyle().
			Background(colorBg).
			Foreground(colorDim).
			Padding(0, 1)

	// TabSeparatorStyle renders the vertical separator between tab labels.
	TabSeparatorStyle = lipgloss.NewStyle().
				Foreground(colorDim)

	// PanelStyle renders an inactive panel with a rounded border.
	PanelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder)

	// ActivePanelStyle renders the focused panel with a primary-color border.
	ActivePanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary)

	// PanelHeaderStyle renders the title inside a panel's top border.
	PanelHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPrimary)

	// OuterBorderStyle wraps the entire application in a rounded border.
	OuterBorderStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder)
)

// tabSeparator is the character placed between tab labels.
const tabSeparator = " │ "

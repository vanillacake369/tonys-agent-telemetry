package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// WhichKeyOverlay renders a centered help overlay listing all key bindings.
// It is toggled by pressing "?" in normal mode and dismissed by any keypress.
type WhichKeyOverlay struct {
	visible bool
	width   int
	height  int
}

// View renders the which-key overlay panel.
// Returns an empty string when the overlay is not visible.
func (w WhichKeyOverlay) View() string {
	if !w.visible {
		return ""
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorPrimary)

	categoryStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText)

	keyStyle := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(colorDim)

	dimSep := lipgloss.NewStyle().Foreground(colorBorder).Render("  ")

	renderRow := func(k1, d1, k2, d2 string) string {
		left := keyStyle.Render(k1) + " " + descStyle.Render(d1)
		if k2 == "" {
			return "  " + left
		}
		right := keyStyle.Render(k2) + " " + descStyle.Render(d2)
		return "  " + left + dimSep + right
	}

	lines := []string{
		titleStyle.Render("Keybindings"),
		"",
		categoryStyle.Render("Navigation"),
		renderRow("j/↓", "Move down    ", "k/↑", "Move up"),
		renderRow("h/←", "Previous     ", "l/→", "Next"),
		renderRow("Enter", "Action      ", "Esc", "Back"),
		"",
		categoryStyle.Render("Tabs"),
		renderRow("1", "Sessions     ", "2", "Agents"),
		renderRow("3", "DAG          ", "4", "Skills"),
		renderRow("Tab", "Next tab     ", "Shift+Tab", "Previous tab"),
		"",
		categoryStyle.Render("Actions"),
		renderRow("y", "Copy to clipboard", "r", "Refresh data"),
		renderRow("f", "Fork session     ", "s", "Sort (Skills)"),
		renderRow("/", "Search           ", "?", "This help"),
		renderRow("q", "Quit             ", "", ""),
		"",
		descStyle.Italic(true).Render("Press any key to close"),
	}

	content := strings.Join(lines, "\n")

	panelStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 3)

	return panelStyle.Render(content)
}

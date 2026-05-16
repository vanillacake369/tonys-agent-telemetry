package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PlaceholderTab is a stub TabModel used until a real implementation is wired in.
type PlaceholderTab struct {
	name   string
	width  int
	height int
}

// newPlaceholderTab creates a PlaceholderTab with the given display name.
func newPlaceholderTab(name string) PlaceholderTab {
	return PlaceholderTab{name: name}
}

func (p PlaceholderTab) Init() tea.Cmd {
	return nil
}

func (p PlaceholderTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	return p, nil
}

func (p PlaceholderTab) View() string {
	if p.width == 0 || p.height == 0 {
		return p.name + "\nComing soon..."
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor)

	hintStyle := lipgloss.NewStyle().
		Foreground(dimColor).
		Italic(true)

	title := titleStyle.Render(p.name)
	hint := hintStyle.Render("Coming soon...")

	// Center both lines vertically.
	inner := title + "\n" + hint
	lines := strings.Split(inner, "\n")
	topPad := (p.height - len(lines)) / 2
	if topPad < 0 {
		topPad = 0
	}

	var rows []string
	for i := 0; i < topPad; i++ {
		rows = append(rows, "")
	}
	rows = append(rows, lines...)

	// Center each line horizontally.
	result := make([]string, len(rows))
	for i, row := range rows {
		result[i] = lipgloss.PlaceHorizontal(p.width, lipgloss.Center, row)
	}

	return strings.Join(result, "\n")
}

func (p PlaceholderTab) SetSize(width, height int) TabModel {
	p.width = width
	p.height = height
	return p
}

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

const (
	// MinSplitWidth is the minimum tab-content width for showing a split view.
	// Must account for two panel borders (4 chars each) + separator.
	// Below this threshold the preview pane is hidden and the list takes full width.
	MinSplitWidth = 90

	// MinPreviewWidth is the absolute minimum width needed to show any preview.
	MinPreviewWidth = 50
)

// SplitLayout calculates left and right widths based on available terminal width.
// leftPct is the percentage (0-100) of total width to allocate to the left pane.
// Returns (leftWidth, rightWidth, showPreview).
func SplitLayout(totalWidth, leftPct int) (int, int, bool) {
	if totalWidth < MinPreviewWidth {
		return ClampInt(totalWidth, 1, totalWidth), 0, false
	}
	if totalWidth < MinSplitWidth {
		return ClampInt(totalWidth, 1, totalWidth), 0, false
	}
	left := totalWidth * leftPct / 100
	right := totalWidth - left - 1 // -1 for separator char
	return ClampInt(left, 1, totalWidth-1), ClampInt(right, 1, totalWidth-1), true
}

// ClampInt ensures val is between minVal and maxVal (inclusive).
func ClampInt(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

// RenderSearchBar renders a consistent search input bar used across tabs.
// rightLabel is an optional string shown right-aligned (e.g. "Sort: ⭐ Stars").
// When rightLabel is empty, the input fills the full width.
// When focused is true, a primary-color bottom border is drawn to signal active search mode.
func RenderSearchBar(input textinput.Model, width int, rightLabel string, focused bool) string {
	if width < 4 {
		return ""
	}

	inputView := input.View()

	baseStyle := lipgloss.NewStyle().
		Foreground(colorText).
		Width(max(0, width))
	if focused {
		baseStyle = baseStyle.
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorPrimary)
	}

	if rightLabel == "" {
		return baseStyle.Render(" " + inputView)
	}

	sortStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	sortRendered := sortStyle.Render(rightLabel)
	sortWidth := lipgloss.Width(sortRendered) + 2

	inputWidth := max(1, width-sortWidth-2)

	row := lipgloss.JoinHorizontal(
		lipgloss.Center,
		lipgloss.NewStyle().Width(inputWidth).Render(" "+inputView),
		lipgloss.NewStyle().Width(sortWidth).Align(lipgloss.Right).Render(sortRendered),
	)

	return baseStyle.Width(max(0, width)).Render(row)
}

// RenderSplitView renders left and right panels side by side separated by a
// thin vertical bar. When showPreview is false only the left panel is shown
// at the full available width.
func RenderSplitView(left, right string, leftWidth, rightWidth, height int, showPreview bool) string {
	if !showPreview || rightWidth <= 0 {
		return lipgloss.NewStyle().
			Width(max(0, leftWidth)).
			Height(max(0, height)).
			Render(left)
	}

	sep := lipgloss.NewStyle().
		Foreground(colorBorder).
		Height(max(0, height)).
		Render("│")

	leftPane := lipgloss.NewStyle().
		Width(max(0, leftWidth)).
		Height(max(0, height)).
		Render(left)

	rightPane := lipgloss.NewStyle().
		Width(max(0, rightWidth)).
		Height(max(0, height)).
		Render(right)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, sep, rightPane)
}

// RenderPanel wraps content in a rounded-border panel with a header title in
// the top-left corner. When active is true the border uses the primary accent
// color instead of the dim border color.
//
// width and height are the outer dimensions of the panel (including the border
// characters). Content is clipped / padded to fit inside.
func RenderPanel(title, content string, width, height int, active bool) string {
	if width < 4 || height < 3 {
		return content
	}

	style := PanelStyle
	if active {
		style = ActivePanelStyle
	}

	// Inner dimensions: subtract 2 for left+right borders and top+bottom borders.
	innerW := max(0, width-2)
	innerH := max(0, height-2)

	rendered := style.
		Width(innerW).
		Height(innerH).
		Render(content)

	// Inject the title into the rendered output's first line so it appears
	// inside the top border after the "╭" character.
	if title != "" {
		header := PanelHeaderStyle.Render(" " + title + " ")
		lines := strings.SplitN(rendered, "\n", 2)
		if len(lines) >= 1 {
			topLine := []rune(lines[0])
			headerRunes := []rune(header)
			// Replace runes starting at position 1 (after the corner char).
			for i, r := range headerRunes {
				pos := 1 + i
				if pos < len(topLine)-1 { // keep trailing corner char
					topLine[pos] = r
				}
			}
			lines[0] = string(topLine)
			rendered = strings.Join(lines, "\n")
		}
	}

	return rendered
}

// RenderHintBar renders a single hint line padded to the given width.
func RenderHintBar(hints string, width int) string {
	return lipgloss.NewStyle().
		Foreground(colorDim).
		Width(max(0, width)).
		Render(hints)
}

// RenderListItem renders a single list entry with a cursor indicator.
// selected items get a full-width background highlight with "▸" prefix;
// others show 3-space indent matching the arrow width.
// Uses MaxWidth to prevent line wrapping with CJK double-width characters.
func RenderListItem(text string, selected bool, width int) string {
	if selected {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colorPrimary).
			Bold(true).
			MaxWidth(max(0, width)).
			Width(max(0, width)).
			Render(" ▸ " + text)
	}
	return lipgloss.NewStyle().
		Foreground(colorText).
		MaxWidth(max(0, width)).
		Width(max(0, width)).
		Render("   " + text)
}

// RenderPreviewPane renders the right preview panel with a left border separator.
func RenderPreviewPane(content string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Width(max(0, width)).
		Height(max(0, height)).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(colorBorder).
		PaddingLeft(1).
		Render(content)
}

// RenderEmptyState renders a centered italic dim message for empty states.
func RenderEmptyState(message string, width, height int) string {
	styled := lipgloss.NewStyle().
		Foreground(colorDim).
		Italic(true).
		Render(message)
	return lipgloss.Place(
		max(0, width),
		max(0, height),
		lipgloss.Center,
		lipgloss.Center,
		styled,
	)
}

// RenderLoadingState renders a centered styled loading indicator.
func RenderLoadingState(width, height int) string {
	spinner := lipgloss.NewStyle().
		Foreground(colorPrimary).
		Bold(true).
		Render("● Loading")
	subtitle := lipgloss.NewStyle().
		Foreground(colorDim).
		Render("Fetching data...")
	content := spinner + "\n" + subtitle
	return lipgloss.Place(
		max(0, width),
		max(0, height),
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

// renderErrorState renders an error message in error color.
func renderErrorState(err error, width int) string {
	return lipgloss.NewStyle().
		Foreground(colorError).
		Width(max(0, width)).
		Render("Error: " + err.Error())
}

// HighlightMatch renders text with matching substrings highlighted.
// query is the search term (case-insensitive match).
// Matched portions get bold + a gold highlight color; rest stays in baseStyle.
// When query is empty the full text is rendered with baseStyle unchanged.
func HighlightMatch(text, query string, baseStyle lipgloss.Style) string {
	if query == "" {
		return baseStyle.Render(text)
	}
	lower := strings.ToLower(text)
	lowerQ := strings.ToLower(query)

	highlightStyle := baseStyle.Bold(true).Foreground(lipgloss.Color("#FFD700"))

	var result strings.Builder
	pos := 0
	for {
		idx := strings.Index(lower[pos:], lowerQ)
		if idx < 0 {
			result.WriteString(baseStyle.Render(text[pos:]))
			break
		}
		// Before match
		if idx > 0 {
			result.WriteString(baseStyle.Render(text[pos : pos+idx]))
		}
		// Match
		result.WriteString(highlightStyle.Render(text[pos+idx : pos+idx+len(lowerQ)]))
		pos += idx + len(lowerQ)
	}
	return result.String()
}

// PadToWidth pads text with spaces to exactly targetWidth display cells.
// Uses lipgloss.Width() which correctly accounts for CJK double-width characters.
// When text already meets or exceeds targetWidth it is truncated via MaxWidth.
func PadToWidth(text string, targetWidth int) string {
	if targetWidth <= 0 {
		return ""
	}
	currentWidth := lipgloss.Width(text)
	if currentWidth >= targetWidth {
		return lipgloss.NewStyle().MaxWidth(targetWidth).Render(text)
	}
	return text + strings.Repeat(" ", targetWidth-currentWidth)
}

// truncateToWidth truncates a string to at most maxWidth display cells,
// appending "…" when the string is shortened.
// Uses lipgloss.Width() to correctly account for CJK double-width characters.
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return "…"
	}
	// Walk runes, accumulating display width.
	runes := []rune(s)
	width := 0
	for i, r := range runes {
		rw := lipgloss.Width(string(r))
		if width+rw > maxWidth-1 { // -1 for "…"
			return string(runes[:i]) + "…"
		}
		width += rw
	}
	return s
}

// wrapLines wraps/truncates a multi-line string so each line fits in width,
// and the total number of lines does not exceed maxLines.
func wrapLines(content string, width, maxLines int) string {
	if width <= 0 || maxLines <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	var out []string
	for _, line := range lines {
		if len([]rune(line)) > width {
			line = string([]rune(line)[:width])
		}
		out = append(out, line)
		if len(out) >= maxLines {
			break
		}
	}
	return strings.Join(out, "\n")
}

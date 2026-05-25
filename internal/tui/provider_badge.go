// Package tui — provider_badge.go is the SSoT for provider badge strings.
// Used wherever a Span/Trace is displayed; do not duplicate badge logic elsewhere.
package tui

import "github.com/charmbracelet/lipgloss"

// providerBadgeDefs maps a system/provider identifier to a (tag, color) pair.
// Tags are exactly 3 chars so columns stay aligned at any width.
var providerBadgeDefs = []struct {
	system string
	tag    string
	color  lipgloss.TerminalColor
}{
	{
		system: "anthropic",
		tag:    "CC ",
		color:  lipgloss.AdaptiveColor{Light: "#5A4FCF", Dark: "#8B7CF6"},
	},
	{
		system: "otlp",
		tag:    "OTL",
		color:  lipgloss.AdaptiveColor{Light: "#1565C0", Dark: "#64B5F6"},
	},
	{
		system: "vllm",
		tag:    "VLM",
		color:  lipgloss.AdaptiveColor{Light: "#2D8A2D", Dark: "#4CAF50"},
	},
	{
		system: "ollama",
		tag:    "OLM",
		color:  lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFC107"},
	},
}

// unknownBadgeColor is used when the provider is not in the known set.
var unknownBadgeColor lipgloss.TerminalColor = lipgloss.AdaptiveColor{
	Light: "#777777",
	Dark:  "#666666",
}

// ProviderBadge returns a styled 3-char tag for the given system/provider name.
// Known providers get a distinct color to aid multi-provider scan:
//   - "anthropic" → "CC " (purple, Claude Code)
//   - "otlp"      → "OTL" (blue)
//   - "vllm"      → "VLM" (green)
//   - "ollama"    → "OLM" (amber)
//   - else        → "???" (dim)
func ProviderBadge(system string) string {
	for _, def := range providerBadgeDefs {
		if system == def.system {
			return lipgloss.NewStyle().Foreground(def.color).Bold(true).Render(def.tag)
		}
	}
	return lipgloss.NewStyle().Foreground(unknownBadgeColor).Render("???")
}

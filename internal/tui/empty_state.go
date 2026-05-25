// Package tui — empty_state.go: provider-specific empty-state guide.
// This is the SSoT for first-run UX messaging. No other file should embed
// provider ingest instructions.
package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderProviderEmptyState returns a multi-line styled block listing each of
// the 4 supported providers with a one-line ingest tip. The caller is
// responsible for centering / placing it within the panel.
//
// Invariant: the returned string (after ANSI stripping) MUST contain each of
// "claudecode", "otlp", "vllm", "ollama" — asserted by TestRenderProviderEmptyState_ContainsAllProviders.
func RenderProviderEmptyState() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorText)
	tipStyle := lipgloss.NewStyle().Foreground(colorDim)
	badgeStyle := func(c lipgloss.TerminalColor) lipgloss.Style {
		return lipgloss.NewStyle().Bold(true).Foreground(c)
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("No telemetry spans collected yet."))
	sb.WriteString("\n\n")
	sb.WriteString(headerStyle.Render("How to start ingesting from each provider:"))
	sb.WriteString("\n\n")

	type entry struct {
		badge string
		color lipgloss.TerminalColor
		name  string
		tip   string
	}

	entries := []entry{
		{
			badge: "CC ",
			color: lipgloss.AdaptiveColor{Light: "#5A4FCF", Dark: "#8B7CF6"},
			name:  "claudecode",
			tip:   "Launch claude in any project directory. Spans arrive automatically via JSONL backfill.",
		},
		{
			badge: "OTL",
			color: lipgloss.AdaptiveColor{Light: "#1565C0", Dark: "#64B5F6"},
			name:  "otlp",
			tip:   "Configure your app's OTel SDK to export to http://127.0.0.1:4318 (set TONYS_OTLP_BIND to change).",
		},
		{
			badge: "VLM",
			color: lipgloss.AdaptiveColor{Light: "#2D8A2D", Dark: "#4CAF50"},
			name:  "vllm",
			tip:   "Run vLLM with --otlp-traces-endpoint http://127.0.0.1:4318, or enable the OTel SDK for full fidelity.",
		},
		{
			badge: "OLM",
			color: lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#FFC107"},
			name:  "ollama",
			tip:   "Ensure Ollama is running; spans are ingested via /api/ps polling automatically.",
		},
	}

	for _, e := range entries {
		badge := badgeStyle(e.color).Render(e.badge)
		name := lipgloss.NewStyle().Bold(true).Foreground(e.color).Render(e.name)
		sb.WriteString("  " + badge + " " + name + "\n")
		sb.WriteString("     " + tipStyle.Render(e.tip) + "\n\n")
	}

	return sb.String()
}

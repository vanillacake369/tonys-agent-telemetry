package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/recommender"
)

// renderAdvisorSection renders the Advisor pane for the Skills tab.
//
// It is intentionally a standalone function (not a method on SkillsTab) so that
// it can be unit-tested independently of the full tab state. The caller
// (SkillsTab.View) simply passes in the current recommendations slice and the
// available width.
//
// Layout (80-column friendly):
//
//	━━━ Advisor ━━━
//	(3 recommendations based on your recent activity)
//
//	▸ [skill] Test-Driven Flow            score 0.78
//	  Triggered by: sig-abc (stalled_node)
//	  Why: shell, performance
//
// SRP: only rendering here — no mutation, no tea.Cmd.
// Micro-module: keep this file ≤100 lines.
func renderAdvisorSection(recs []recommender.Recommendation, width int) string {
	sep := lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("━", min(width, 80)))

	if len(recs) == 0 {
		emptyLine := lipgloss.NewStyle().Foreground(colorDim).Italic(true).
			Render("Advisor — no signals match catalog yet. Keep working; recommendations appear after debounce.")
		return strings.Join([]string{sep, emptyLine}, "\n")
	}

	headerText := fmt.Sprintf("━━━ Advisor (%d recommendations based on your recent activity) ━━━", len(recs))
	header := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(headerText)

	var rows []string
	rows = append(rows, sep, header)

	for _, r := range recs {
		// Title line with score right-aligned.
		titleLine := fmt.Sprintf("▸ [%s] %-36s score %.2f",
			catalogTypeFromCatalogItemID(r.CatalogItemID), r.Title, r.Score)
		rows = append(rows,
			lipgloss.NewStyle().Foreground(colorText).Render(titleLine),
		)

		// Citation line (evidence chain — GA gate requirement).
		citationLine := fmt.Sprintf("  Triggered by signal: %s", r.SignalID)
		rows = append(rows,
			lipgloss.NewStyle().Foreground(colorDim).Render(citationLine),
		)
	}

	// Attribution footer — CC-BY-SA-4.0 share-alike requires attribution
	// alongside any displayed catalog-derived content (QA finding A-3).
	attr := lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render(catalog.Attribution)
	rows = append(rows, attr)

	return strings.Join(rows, "\n")
}

// catalogTypeFromCatalogItemID extracts the type prefix from IDs of the form
// "<type>/<slug>" (e.g. "skill/test-driven-flow" → "skill"). Returns "?" for
// any other format.
func catalogTypeFromCatalogItemID(id string) string {
	if i := strings.Index(id, "/"); i > 0 {
		return id[:i]
	}
	return "?"
}

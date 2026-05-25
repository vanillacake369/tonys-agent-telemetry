package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/recommender"
)

// Advisor empty-state strings — exposed as constants so tests can assert
// substring presence without coupling to formatting changes.
//
// SSoT: changing these strings here updates both the UI and the tests.
const (
	advisorEmptyNoSpans   = "Advisor — ingest sessions to see recommendations (run claude sessions or use --replay <file>)."
	advisorEmptyNoMatches = "Advisor — signals extracted but no catalog matches yet. Keep working; recommendations appear after more activity."
	advisorDAGNavHint     = "(press 5 to view DAG)"
)

// renderAdvisorSection renders the Advisor pane for the Skills tab.
//
// pipelineRan indicates whether the advisor pipeline has produced output
// at least once. False → no spans / pipeline never ran → show the "ingest
// sessions" guide. True with empty recs → "no matches yet". Non-empty recs
// → render the list with citations.
//
// SRP: only rendering here — no mutation, no tea.Cmd.
// Micro-module: keep this file ≤120 lines.
func renderAdvisorSection(recs []recommender.Recommendation, width int, pipelineRan bool) string {
	sep := lipgloss.NewStyle().Foreground(colorDim).Render(strings.Repeat("━", min(width, 80)))

	if len(recs) == 0 {
		msg := advisorEmptyNoSpans
		if pipelineRan {
			msg = advisorEmptyNoMatches
		}
		emptyLine := lipgloss.NewStyle().Foreground(colorDim).Italic(true).Render(msg)
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
		// Trace-anchored signals (most common case) carry a TraceID — render a
		// "press 5 to view DAG" nav hint so users can trace the evidence back
		// to the source span. Cross-session signals (e.g. unused_installed_skill)
		// have an empty TraceID — render a different phrasing.
		var citationLine string
		if r.TraceID != "" {
			citationLine = fmt.Sprintf("  Triggered by signal %s in trace %s %s",
				r.SignalID, r.TraceID, advisorDAGNavHint)
		} else {
			citationLine = fmt.Sprintf("  Triggered by signal %s (cross-session — no specific trace)",
				r.SignalID)
		}
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

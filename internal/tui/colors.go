// Package tui — colors.go is the SSoT for all status/duration color values.
// No other file should hard-code these hex codes or threshold constants.
package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// Threshold constants for slow-span coloring (milliseconds).
const (
	SlowMs = int64(5000) // >= SlowMs → red
	WarnMs = int64(1000) // >= WarnMs (but < SlowMs) → yellow
)

// Named color palette for status/duration indicators.
// These are the canonical values; all styling must reference these vars,
// not inline hex strings.
var (
	// colorStatusError — error spans and durations ≥ SlowMs.
	colorStatusError lipgloss.TerminalColor = lipgloss.AdaptiveColor{
		Light: "#CC0000",
		Dark:  "#FF5555",
	}

	// colorStatusWarn — running spans and durations ≥ WarnMs (< SlowMs).
	colorStatusWarn lipgloss.TerminalColor = lipgloss.AdaptiveColor{
		Light: "#B8860B",
		Dark:  "#FFC107",
	}

	// colorStatusOK — done/default spans and fast durations.
	colorStatusOK lipgloss.TerminalColor = lipgloss.AdaptiveColor{
		Light: "#2D8A2D",
		Dark:  "#4CAF50",
	}

	// colorStatusInfo — informational / neutral highlights.
	colorStatusInfo lipgloss.TerminalColor = lipgloss.AdaptiveColor{
		Light: "#1565C0",
		Dark:  "#64B5F6",
	}
)

// StatusColor returns the canonical lipgloss color for a span's status field.
// Used by both the traces list and graph nodes so both surfaces share the SSoT.
func StatusColor(s telemetry.Span) lipgloss.TerminalColor {
	switch s.Status {
	case "error":
		return colorStatusError
	case "running":
		return colorStatusWarn
	default:
		return colorStatusOK
	}
}

// DurationColor returns a color reflecting how slow the duration is.
//   - durationMs >= SlowMs → red (error)
//   - durationMs >= WarnMs → yellow (warning)
//   - otherwise → green (ok / default)
func DurationColor(durationMs int64) lipgloss.TerminalColor {
	switch {
	case durationMs >= SlowMs:
		return colorStatusError
	case durationMs >= WarnMs:
		return colorStatusWarn
	default:
		return colorStatusOK
	}
}

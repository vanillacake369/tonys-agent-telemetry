package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/trends"
)

// TrendsLoadedMsg carries aggregated Bucket data into the Trends tab.
// Phase κ owns the production side (signalstore → trends.Aggregate → this Msg).
type TrendsLoadedMsg struct {
	Buckets []trends.Bucket
}

// sparklineRamp is the Unicode block-element ramp used for sparklines.
// Index 0 = empty/zero, index 8 = full bar (█).
const sparklineRamp = " ▁▂▃▄▅▆▇█"

// knownSignalTypes lists the four v0 signal types in display order.
// SSoT: changing this slice changes what rows appear in the Trends tab.
var knownSignalTypes = []signal.SignalType{
	signal.SignalStalledNode,
	signal.SignalDuplicateSubagentWork,
	signal.SignalFailedHandoff,
	signal.SignalUnusedInstalledSkill,
}

// TrendsTab renders longitudinal signal counts.
type TrendsTab struct {
	buckets []trends.Bucket
	width   int
	height  int
}

func NewTrendsTab() *TrendsTab { return &TrendsTab{} }

func (t *TrendsTab) Init() tea.Cmd { return nil }

func (t *TrendsTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	if m, ok := msg.(TrendsLoadedMsg); ok {
		t.buckets = m.Buckets
	}
	return t, nil
}

// View renders the Trends tab. It shows:
//   - A header line with bucket count and lookback window.
//   - An empty-state message when fewer than MinBucketsForDisplay buckets exist.
//   - Per-signal-type rows with sparkline + last value + delta vs mean.
//   - A fidelity tier legend at the bottom.
func (t *TrendsTab) View() string {
	nonEmpty := countNonEmpty(t.buckets)

	if nonEmpty < trends.MinBucketsForDisplay {
		return renderTrendsEmptyState(len(t.buckets), nonEmpty)
	}

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("Trends — %d buckets over %d days\n\n",
		len(t.buckets), trends.DefaultLookbackDays))

	// Column headers (ν-5: added Start column)
	sb.WriteString(fmt.Sprintf("%-28s  %-12s  %6s  %6s  %12s\n",
		"Signal Type", "Sparkline", "Start", "Last", "Δ vs avg"))
	sb.WriteString(strings.Repeat("─", min(t.width, 80)))
	sb.WriteString("\n")

	// One row per known signal type, sorted deterministically.
	for _, sigType := range knownSignalTypes {
		row := renderSignalRow(sigType, t.buckets)
		sb.WriteString(row)
		sb.WriteString("\n")
	}

	// Fidelity tier legend.
	sb.WriteString("\n")
	sb.WriteString(renderFidelityTierLegend())

	return sb.String()
}

// SetSize updates the stored terminal dimensions.
func (t *TrendsTab) SetSize(w, h int) TabModel {
	t.width, t.height = w, h
	return t
}

// countNonEmpty returns the number of buckets where IsEmpty() == false.
func countNonEmpty(buckets []trends.Bucket) int {
	n := 0
	for _, b := range buckets {
		if !b.IsEmpty() {
			n++
		}
	}
	return n
}

// renderTrendsEmptyState renders the friendly not-enough-data message.
// It includes the current/required counts so users understand the threshold.
func renderTrendsEmptyState(have, nonEmpty int) string {
	return fmt.Sprintf(
		"Trends — not enough data yet (%d/%d required)."+
			" Run more sessions to see longitudinal data.",
		nonEmpty, trends.MinBucketsForDisplay,
	)
}

// renderSignalRow produces one text row for a signal type.
// Format: <label padded>  <sparkline>  <start value>  <last value>  <delta vs avg>
func renderSignalRow(sigType signal.SignalType, buckets []trends.Bucket) string {
	// Extract the per-bucket counts for this signal type.
	counts := make([]int, len(buckets))
	for i, b := range buckets {
		counts[i] = b.Counts[sigType]
	}

	// Compute sparkline string.
	sparkline := buildSparkline(counts)

	// Start value = count in the first bucket (ν-5).
	start := 0
	if len(counts) > 0 {
		start = counts[0]
	}

	// Last value = count in the final bucket.
	last := 0
	if len(counts) > 0 {
		last = counts[len(counts)-1]
	}

	// Average over all buckets (float, for delta computation).
	avg := mean(counts)

	delta := float64(last) - avg
	deltaStr := formatDelta(delta)

	// Truncate the label to 26 chars so the table stays aligned.
	label := truncateLabel(string(sigType), 26)

	return fmt.Sprintf("%-28s  %-12s  %6d  %6d  %12s", label, sparkline, start, last, deltaStr)
}

// buildSparkline converts a slice of counts into a string of block elements.
// The ramp maps min..max linearly; zero-count buckets always use space (' ').
func buildSparkline(counts []int) string {
	if len(counts) == 0 {
		return ""
	}

	maxVal := maxInt(counts)
	if maxVal == 0 {
		return strings.Repeat("_", len(counts))
	}

	runes := []rune(sparklineRamp)
	steps := len(runes) - 1 // 8 steps (indices 1..8)

	var sb strings.Builder
	for _, c := range counts {
		if c == 0 {
			sb.WriteByte('_')
			continue
		}
		idx := (c * steps) / maxVal
		if idx < 1 {
			idx = 1
		}
		if idx > steps {
			idx = steps
		}
		sb.WriteRune(runes[idx])
	}
	return sb.String()
}

// mean returns the arithmetic mean of a slice. Returns 0 for empty slices.
func mean(counts []int) float64 {
	if len(counts) == 0 {
		return 0
	}
	sum := 0
	for _, c := range counts {
		sum += c
	}
	return float64(sum) / float64(len(counts))
}

// maxInt returns the maximum value in the slice, or 0 if empty.
func maxInt(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return m
}

// formatDelta formats a float delta as "(Δ +n)", "(Δ -n)", or "(Δ ±0)".
func formatDelta(d float64) string {
	switch {
	case d > 0.5:
		return fmt.Sprintf("(Δ +%.0f vs avg)", d)
	case d < -0.5:
		return fmt.Sprintf("(Δ %.0f vs avg)", d)
	default:
		return "(Δ ±0 vs avg)"
	}
}

// truncateLabel clips s to at most n characters, appending ".." if truncated.
func truncateLabel(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-2]) + ".."
}

// renderFidelityTierLegend returns the one-line provider data-fidelity note.
// Per PIVOT_PLAN Phase 3: vllm and ollama contribute aggregate/presence data only.
func renderFidelityTierLegend() string {
	providers := []struct {
		name string
		tier string
	}{
		{"claudecode", "full"},
		{"otlp", "full"},
		{"vllm", "aggregate"},
		{"ollama", "presence"},
	}

	// Sort for deterministic output.
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].name < providers[j].name
	})

	parts := make([]string, len(providers))
	for i, p := range providers {
		parts[i] = p.name + ":" + p.tier
	}

	return "Fidelity tier — " + strings.Join(parts, "  ") +
		"  (vllm/ollama contribute aggregate/presence data only)\n" +
		"If all sparklines are flat at zero for a provider, the provider's tier limit" +
		" may not produce these signal types — see Fidelity tier above."
}

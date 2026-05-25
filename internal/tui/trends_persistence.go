package tui

import (
	"fmt"
	"os"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// FlushInterval is the period between periodic signal extraction and
// persistence runs. SSoT: change this constant to adjust flush cadence.
const FlushInterval = 5 * time.Minute

// TrendsPersistence orchestrates periodic extraction of signals from the
// current span buffer and persistence to the signal store.
//
// SRP: this file owns persistence triggering only. Aggregation lives in
// internal/trends/aggregate.go. UI rendering lives in tab_trends.go.
//
// Micro-module: keep this file ≤100 lines.
type TrendsPersistence struct {
	store     *signalstore.Store
	mu        sync.Mutex // protects lastFlush; FlushCmd closure runs in a separate goroutine
	lastFlush time.Time  // zero = never flushed
}

// NewTrendsPersistence constructs a TrendsPersistence backed by store.
func NewTrendsPersistence(store *signalstore.Store) *TrendsPersistence {
	return &TrendsPersistence{store: store}
}

// FlushCmd returns a tea.Cmd that — when executed by the Bubble Tea runtime —
// performs BuildForests/Extract/Append asynchronously and returns nil (the
// flush is fire-and-forget). Errors are logged to stderr rather than
// propagated upward so a write failure never crashes the TUI.
//
// Empty spans are skipped: writing a zero-signal snapshot provides no value
// to longitudinal analysis and would inflate the bucket Sessions counts.
//
// The lastFlush timestamp is updated inside the closure (running in the cmd's
// goroutine) under mu to avoid a data race with any future reader on the
// main goroutine.
func (p *TrendsPersistence) FlushCmd(sessionID string, spans []telemetry.Span) tea.Cmd {
	if len(spans) == 0 {
		return nil
	}

	// Capture a snapshot of spans for the closure (avoid data races).
	spansCopy := make([]telemetry.Span, len(spans))
	copy(spansCopy, spans)

	return func() tea.Msg {
		forest := telemetry.BuildForests(spansCopy)
		sigs := signal.Extract(forest, signal.DefaultExtractOpts())

		// Don't write an empty snapshot — no signals means no useful data.
		if len(sigs) == 0 {
			return nil
		}

		if err := p.store.Append(sessionID, sigs); err != nil {
			// Log the error to stderr and continue; persistence failures must never
			// crash or stall the TUI (PIVOT_PLAN Phase 3 degradation policy).
			// Note: do NOT advance lastFlush on failure — the next tick should retry.
			fmt.Fprintf(os.Stderr, "trends: flush error for session %q: %v\n", sessionID, err)
			return nil
		}

		// Only record lastFlush after a successful append. Mutex guards concurrent
		// FlushCmd executions running in different goroutines (μ-3 race safety).
		p.mu.Lock()
		p.lastFlush = time.Now()
		p.mu.Unlock()

		return nil
	}
}

// NextTick returns a tea.Cmd that fires after FlushInterval and produces a
// TrendsFlushTickMsg. The App wires this into its Init and re-schedules it
// on every receipt.
func (p *TrendsPersistence) NextTick() tea.Cmd {
	return tea.Tick(FlushInterval, func(t time.Time) tea.Msg {
		return TrendsFlushTickMsg{At: t}
	})
}

// TrendsFlushTickMsg is produced by TrendsPersistence.NextTick. The App
// handles it by calling FlushCmd with the current span buffer and
// re-scheduling the next tick.
type TrendsFlushTickMsg struct {
	At time.Time
}

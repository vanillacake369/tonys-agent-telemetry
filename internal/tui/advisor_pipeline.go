package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/catalog"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/recommender"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// AdvisorDebounceDelay is the minimum elapsed time between consecutive pipeline
// runs. SSoT: change this constant to adjust the debounce for all callers.
const AdvisorDebounceDelay = 500 * time.Millisecond

// AdvisorMinSpansDelta is the minimum number of NEW spans (net increase since
// the last successful run) required to trigger the pipeline. SSoT: change this
// constant to adjust the threshold for all callers.
const AdvisorMinSpansDelta = 5

// AdvisorPipeline orchestrates the debounced extract→recommend pipeline.
// It is stateful: it tracks the time of the last run and the span count seen
// at that time so it can enforce both the debounce window and the span-delta
// minimum.
//
// SRP: this file owns pipeline orchestration only. Rendering lives in
// tab_skills_advisor.go (renderAdvisorSection).
//
// Micro-module: keep this file ≤150 lines.
type AdvisorPipeline struct {
	lastRun       time.Time     // zero means never run
	lastSpanCount int           // span count at last successful run
	debounceDelay time.Duration // overridable in tests; production value = AdvisorDebounceDelay
}

// NewAdvisorPipeline returns a zero-value pipeline ready for use.
// debounceDelay is set to the production constant; override in tests.
func NewAdvisorPipeline() *AdvisorPipeline {
	return &AdvisorPipeline{
		debounceDelay: AdvisorDebounceDelay,
	}
}

// MaybeRun returns a tea.Cmd that runs the extract→recommend pipeline if and
// only if both conditions are met:
//
//  1. At least AdvisorDebounceDelay has elapsed since the last run (or this is
//     the first run ever).
//  2. The number of spans has grown by at least AdvisorMinSpansDelta since the
//     last run.
//
// When either condition is not met the method returns nil (no work scheduled).
//
// installedSkillNames is the list of locally-installed skill names (from
// SkillsTab.LocalSkillNames()). It is threaded into signal.ExtractOpts so
// the unused_installed_skill detector fires when an installed skill is never
// invoked across the current session forest. Passing nil disables that detector.
//
// DRY: InstalledSkills is read from the SkillsTab in one place (app.go) and
// passed here; it is not duplicated elsewhere.
//
// The returned tea.Cmd, when executed by the Bubble Tea runtime, returns a
// RecommendationsReadyMsg that App.Update routes to TabSkills.
//
// DRY: signal.Extract is called exactly once per trigger inside the cmd closure.
func (p *AdvisorPipeline) MaybeRun(spans []telemetry.Span, items []catalog.Item, installedSkillNames []string) tea.Cmd {
	now := time.Now()

	// Debounce guard: skip if called too soon after the last run.
	if !p.lastRun.IsZero() && now.Sub(p.lastRun) < p.debounceDelay {
		return nil
	}

	// Span-delta guard: skip if insufficient new spans arrived.
	newSpans := len(spans) - p.lastSpanCount
	if !p.lastRun.IsZero() && newSpans < AdvisorMinSpansDelta {
		return nil
	}

	// Record the run now (before the async cmd) so that rapid MaybeRun calls
	// after this one respect the debounce even if the cmd hasn't executed yet.
	p.lastRun = now
	p.lastSpanCount = len(spans)

	// Capture local copies for the closure; avoid capturing the pointer receiver
	// so the cmd is safe to execute on a different goroutine.
	spansCopy := make([]telemetry.Span, len(spans))
	copy(spansCopy, spans)
	itemsCopy := make([]catalog.Item, len(items))
	copy(itemsCopy, items)
	skillNamesCopy := make([]string, len(installedSkillNames))
	copy(skillNamesCopy, installedSkillNames)

	return func() tea.Msg {
		forest := telemetry.BuildForests(spansCopy)
		opts := signal.DefaultExtractOpts()
		opts.InstalledSkills = skillNamesCopy
		sigs := signal.Extract(forest, opts)
		recs := recommender.NewEngine().Recommend(sigs, itemsCopy)
		return RecommendationsReadyMsg{Recommendations: recs}
	}
}

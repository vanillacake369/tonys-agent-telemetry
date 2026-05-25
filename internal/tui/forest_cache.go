package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// ForestCache memoises BuildForests + Extract results keyed on
// (len(spans), opts hash). Multiple concurrent callers share one result
// per generation.
//
// Lifecycle: each call to Get either reuses a cached result if inputs are
// unchanged, or recomputes and stores. Cache is invalidated whenever the
// span buffer grows (the simplest correct heuristic — we never lose data).
//
// SRP: this file owns caching only. Rendering and pipeline orchestration live
// in their respective files.
//
// Micro-module: keep this file ≤80 lines.
type ForestCache struct {
	mu            sync.Mutex
	lastSpanCount int
	lastOptsKey   string // hash of ExtractOpts; identical opts → same key
	cachedSignals []signal.Signal
	// computeCount is incremented on every recompute; used in tests only.
	computeCount int
}

// NewForestCache returns a zero-value cache ready for use.
func NewForestCache() *ForestCache {
	return &ForestCache{}
}

// Get returns BuildForests+Extract output for the given spans/opts,
// computing them only if the inputs differ from the last call.
func (c *ForestCache) Get(spans []telemetry.Span, opts signal.ExtractOpts) []signal.Signal {
	key := optsKey(opts)
	count := len(spans)

	c.mu.Lock()
	defer c.mu.Unlock()

	if count == c.lastSpanCount && key == c.lastOptsKey && c.cachedSignals != nil {
		return c.cachedSignals
	}

	forest := telemetry.BuildForests(spans)
	sigs := signal.Extract(forest, opts)

	c.lastSpanCount = count
	c.lastOptsKey = key
	c.cachedSignals = sigs
	c.computeCount++

	return sigs
}

// optsKey returns a stable string key for the given ExtractOpts that
// captures all fields affecting signal extraction output.
func optsKey(opts signal.ExtractOpts) string {
	skills := strings.Join(opts.InstalledSkills, ",")
	return fmt.Sprintf("%d|%v|%d|%s|%v",
		opts.StallThreshold,
		opts.DupOverlapThreshold,
		opts.MinSessionsForUnusedSkill,
		skills,
		opts.ClockSkewTolerance,
	)
}

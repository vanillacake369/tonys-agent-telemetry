// Package signal implements the Signal Extractor v0 as specified in
// SIGNALS_SPEC.md. It scans a fully-built telemetry.Forest and emits a
// flat, deterministic list of typed Signal values.
//
// Entry point: Extract(forest, opts) []Signal
// No I/O. No global state. No goroutines. Pure function.
package signal

import (
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// SignalType enumerates the four v0 signal kinds (SIGNALS_SPEC §2).
type SignalType string

const (
	SignalStalledNode           SignalType = "stalled_node"
	SignalDuplicateSubagentWork SignalType = "duplicate_subagent_work"
	SignalUnusedInstalledSkill  SignalType = "unused_installed_skill"
	SignalFailedHandoff         SignalType = "failed_handoff"
)

// Signal is the envelope emitted for every detected behavioral event.
// It is the unit of evidence consumed by the Phase 2 recommender.
//
// ID is a deterministic hash derived from (Type, TraceID, sorted SpanIDs)
// so that re-extracting from the same Forest yields byte-identical IDs
// (required for Phase 3 idempotent persistence, SIGNALS_SPEC §8).
//
// ProviderTier is set per SIGNALS_SPEC §8 locked decision (2026-05-26):
// values "full", "aggregate", "presence". The extractor sets it to "full"
// for all v0 signals; lower-tier providers produce zero signals.
type Signal struct {
	ID           string         `json:"id"`
	Type         SignalType     `json:"type"`
	TraceID      string         `json:"trace_id"`    // empty for cross-trace signals (unused_installed_skill)
	SpanIDs      []string       `json:"span_ids"`    // ordered; empty slice, never nil
	Evidence     map[string]any `json:"evidence"`    // signal-type-specific payload (see §3)
	Confidence   float64        `json:"confidence"`  // [0.0, 1.0]
	EmittedAt    time.Time      `json:"emitted_at"`  // wall clock at extraction time
	ProviderTier string         `json:"provider_tier"` // "full" | "aggregate" | "presence"
}

// Forest is the type alias for the output of telemetry.BuildForests.
// The extractor receives this directly; it does not call BuildForests itself.
type Forest = map[string][]*telemetry.SpanNode

// ExtractOpts holds all tunables. Callers should start from DefaultExtractOpts()
// and override only what they need. Zero values are not valid defaults.
type ExtractOpts struct {
	// StallThreshold is the minimum leaf-span duration that triggers stalled_node.
	// Default: 10s.
	StallThreshold time.Duration

	// DupOverlapThreshold is the minimum Jaccard similarity between two sibling
	// subagent tool sequences to count as duplicate work.
	// Default: 0.8 (80%). Range: (0, 1].
	DupOverlapThreshold float64

	// MinSessionsForUnusedSkill is the number of distinct TraceIDs that must
	// be present before unused_installed_skill fires.
	// Default: 3.
	MinSessionsForUnusedSkill int

	// InstalledSkills is the list of skill names found on disk (e.g. from
	// skill.ScanLocal()). The extractor does no filesystem I/O; the caller
	// resolves this before calling Extract. If nil or empty, unused_installed_skill
	// detection is skipped.
	InstalledSkills []string

	// ClockSkewTolerance is subtracted from computed durations before threshold
	// comparison, to account for provider clock skew. Default: 500ms.
	ClockSkewTolerance time.Duration
}

// DefaultExtractOpts returns the canonical v0 defaults (SIGNALS_SPEC §2).
func DefaultExtractOpts() ExtractOpts {
	return ExtractOpts{
		StallThreshold:            DefaultStallThreshold,
		DupOverlapThreshold:       DefaultDupOverlapThreshold,
		MinSessionsForUnusedSkill: DefaultMinSessionsForUnusedSkill,
		InstalledSkills:           nil,
		ClockSkewTolerance:        DefaultClockSkewTolerance,
	}
}

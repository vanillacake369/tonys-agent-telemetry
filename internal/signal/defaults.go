package signal

import "time"

// Default thresholds and tunables for the Signal Extractor v0.
// This is the Single Source of Truth (SSoT) for all configurable values
// referenced throughout the package. Detectors import these constants
// rather than hard-coding literals.
const (
	// DefaultStallThreshold is the minimum adjusted leaf-span duration that
	// triggers a stalled_node signal (SIGNALS_SPEC §3.1).
	DefaultStallThreshold = 10 * time.Second

	// DefaultDupOverlapThreshold is the minimum Jaccard similarity between
	// two sibling subagent tool sequences for duplicate_subagent_work
	// (SIGNALS_SPEC §3.2). Value 0.8 = 80%.
	DefaultDupOverlapThreshold = 0.8

	// DefaultMinSessionsForUnusedSkill is the minimum number of distinct
	// TraceIDs required before unused_installed_skill fires
	// (SIGNALS_SPEC §3.3).
	DefaultMinSessionsForUnusedSkill = 3

	// DefaultClockSkewTolerance is subtracted from raw durations before
	// threshold comparison, to account for provider clock drift
	// (SIGNALS_SPEC §3.1 edge case E5).
	DefaultClockSkewTolerance = 500 * time.Millisecond

	// ProviderTierFull is the tier value for providers with per-call span
	// topology (claudecode, otlp). All v0 signals require Full tier.
	ProviderTierFull = "full"

	// ProviderTierAggregate is the tier for providers with aggregate-only
	// metrics (vllm Prometheus scrape). No v0 signal types available.
	ProviderTierAggregate = "aggregate"

	// ProviderTierPresence is the tier for providers with presence-only
	// data (ollama /api/ps poll). No v0 signal types available.
	ProviderTierPresence = "presence"

	// failedHandoffBaseConfidence is the base confidence for a failed_handoff
	// signal before optional boosts (SIGNALS_SPEC §3.4).
	failedHandoffBaseConfidence = 0.7

	// failedHandoffErrorTypeBoost is added when error.type attr is non-empty.
	failedHandoffErrorTypeBoost = 0.2

	// failedHandoffRetryAlsoErroredBoost is added when the retry span also errored.
	failedHandoffRetryAlsoErroredBoost = 0.1
)

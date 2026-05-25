package signal

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

// deriveID computes a deterministic signal ID from (Type, TraceID, SpanIDs).
// SpanIDs are sorted before hashing so that the order in which detectors
// append them does not affect the ID. The ID is the first 16 hex characters
// of the SHA-256 digest of the canonical string representation.
//
// Canonical format: "<type>|<traceID>|<spanID1>,<spanID2>,..."
// For signals with no SpanIDs (unused_installed_skill), the span section is empty.
//
// This function is the SSoT for ID derivation per SIGNALS_SPEC §8:
// "Signal.ID needed for deduplication across snapshots … derived from
// hash(TraceID + SpanIDs[0] + Type)". We use sorted SpanIDs (not just [0])
// for multi-span signals to ensure stability regardless of detection order.
func deriveID(signalType SignalType, traceID string, spanIDs []string) string {
	sorted := make([]string, len(spanIDs))
	copy(sorted, spanIDs)
	sort.Strings(sorted)

	key := string(signalType) + "|" + traceID + "|" + strings.Join(sorted, ",")
	digest := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", digest[:8]) // 16 hex chars, 64-bit collision resistance
}

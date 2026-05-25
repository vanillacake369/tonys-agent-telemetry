package trends

import (
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
)

// bucketIndex returns the zero-based index of the bucket that t belongs to,
// given the series starting at from with window size duration.
// Returns -1 when t is outside [from, from+numBuckets*duration).
// This is the single helper that determines which bucket a timestamp falls into —
// DRY contract: all callers use only this function for bucket assignment.
func bucketIndex(t, from time.Time, duration time.Duration) int {
	if t.Before(from) {
		return -1
	}
	offset := t.Sub(from)
	return int(offset / duration)
}

// Aggregate buckets the given SessionSnapshots into windows of `duration`
// starting at `from` (inclusive) and ending at `to` (exclusive). Empty windows
// are emitted as Buckets with IsEmpty() == true so the UI can render gaps.
//
// Per SIGNALS_SPEC §8, SnapshotEntry.CapturedAt is the time key; every Signal
// inside an entry inherits that timestamp for bucket assignment.
//
// Complexity: O(E * S) where E = total SnapshotEntries across all sessions
// and S = total Signals per entry. The number of buckets N is computed once
// from the [from,to) range and is independent of input size.
func Aggregate(sessions []signalstore.SessionSnapshot, from, to time.Time, duration time.Duration) []Bucket {
	// Compute the number of buckets needed.
	span := to.Sub(from)
	if span <= 0 || duration <= 0 {
		return nil
	}
	numBuckets := int((span + duration - 1) / duration) // ceiling division

	// Pre-allocate the bucket slice.
	buckets := make([]Bucket, numBuckets)
	for i := range buckets {
		buckets[i] = Bucket{
			Start:    from.Add(time.Duration(i) * duration),
			Duration: duration,
		}
	}

	// sessionSets[i] tracks distinct SessionIDs that contributed to bucket i.
	sessionSets := make([]map[string]struct{}, numBuckets)
	for i := range sessionSets {
		sessionSets[i] = make(map[string]struct{})
	}

	// Walk every session's entries and distribute signals into buckets.
	for _, session := range sessions {
		for _, entry := range session.Entries {
			idx := bucketIndex(entry.CapturedAt, from, duration)
			if idx < 0 || idx >= numBuckets {
				// Out of range — ignored per spec.
				continue
			}
			// Ensure Counts is initialised before writing into it.
			if buckets[idx].Counts == nil {
				buckets[idx].Counts = make(map[signal.SignalType]int)
			}
			for _, sig := range entry.Signals {
				buckets[idx].Counts[sig.Type]++
			}
			sessionSets[idx][session.SessionID] = struct{}{}
		}
	}

	// Populate Sessions count from the deduplicated sets.
	for i := range buckets {
		buckets[i].Sessions = len(sessionSets[i])
	}

	return buckets
}

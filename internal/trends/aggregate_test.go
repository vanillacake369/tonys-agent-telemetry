package trends

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
)

// baseTime is a fixed anchor for test timestamps — 2026-01-01 00:00:00 UTC.
var baseTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// makeSnapshot builds a SnapshotEntry with CapturedAt = baseTime + dayOffset days.
func makeSnapshot(dayOffset int, sigs ...signal.Signal) signalstore.SnapshotEntry {
	return signalstore.SnapshotEntry{
		CapturedAt: baseTime.Add(time.Duration(dayOffset) * 24 * time.Hour),
		Signals:    sigs,
	}
}

// makeSig creates a Signal of the given type with default non-zero fields.
func makeSig(t signal.SignalType) signal.Signal {
	return signal.Signal{Type: t, SpanIDs: []string{}}
}

// TestAggregate_BasicBucketing checks that 3 sessions across 5 days produce
// the correct per-bucket Counts. Sessions on day 0, 2, 4 → buckets 0, 2, 4
// should each have counts; buckets 1, 3 are empty.
func TestAggregate_BasicBucketing(t *testing.T) {
	from := baseTime
	to := baseTime.Add(5 * 24 * time.Hour)

	sessions := []signalstore.SessionSnapshot{
		{SessionID: "s1", Entries: []signalstore.SnapshotEntry{makeSnapshot(0, makeSig(signal.SignalStalledNode))}},
		{SessionID: "s2", Entries: []signalstore.SnapshotEntry{makeSnapshot(2, makeSig(signal.SignalFailedHandoff))}},
		{SessionID: "s3", Entries: []signalstore.SnapshotEntry{makeSnapshot(4, makeSig(signal.SignalStalledNode), makeSig(signal.SignalStalledNode))}},
	}

	buckets := Aggregate(sessions, from, to, DefaultBucketDuration)

	if len(buckets) != 5 {
		t.Fatalf("expected 5 buckets, got %d", len(buckets))
	}

	// Bucket 0: 1 stalled_node from s1
	if c := buckets[0].Counts[signal.SignalStalledNode]; c != 1 {
		t.Errorf("bucket 0 stalled_node = %d, want 1", c)
	}
	// Bucket 1: empty
	if !buckets[1].IsEmpty() {
		t.Errorf("bucket 1 should be empty, got %+v", buckets[1])
	}
	// Bucket 2: 1 failed_handoff from s2
	if c := buckets[2].Counts[signal.SignalFailedHandoff]; c != 1 {
		t.Errorf("bucket 2 failed_handoff = %d, want 1", c)
	}
	// Bucket 3: empty
	if !buckets[3].IsEmpty() {
		t.Errorf("bucket 3 should be empty")
	}
	// Bucket 4: 2 stalled_node from s3
	if c := buckets[4].Counts[signal.SignalStalledNode]; c != 2 {
		t.Errorf("bucket 4 stalled_node = %d, want 2", c)
	}
}

// TestAggregate_EmptyWindowsEmitted checks that a gap day produces a Bucket
// with IsEmpty() == true so the UI can render a blank gap.
func TestAggregate_EmptyWindowsEmitted(t *testing.T) {
	from := baseTime
	to := baseTime.Add(3 * 24 * time.Hour)

	// Only sessions on days 0 and 2; day 1 is empty.
	sessions := []signalstore.SessionSnapshot{
		{SessionID: "a", Entries: []signalstore.SnapshotEntry{makeSnapshot(0, makeSig(signal.SignalStalledNode))}},
		{SessionID: "b", Entries: []signalstore.SnapshotEntry{makeSnapshot(2, makeSig(signal.SignalStalledNode))}},
	}

	buckets := Aggregate(sessions, from, to, DefaultBucketDuration)
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}
	if !buckets[1].IsEmpty() {
		t.Errorf("gap bucket (index 1) should be IsEmpty(), got %+v", buckets[1])
	}
}

// TestAggregate_MultiSignalCounts verifies that one snapshot with mixed signal
// types is counted per type separately.
func TestAggregate_MultiSignalCounts(t *testing.T) {
	from := baseTime
	to := baseTime.Add(1 * 24 * time.Hour)

	entry := makeSnapshot(0,
		makeSig(signal.SignalStalledNode),
		makeSig(signal.SignalStalledNode),
		makeSig(signal.SignalFailedHandoff),
		makeSig(signal.SignalDuplicateSubagentWork),
	)
	sessions := []signalstore.SessionSnapshot{
		{SessionID: "x", Entries: []signalstore.SnapshotEntry{entry}},
	}

	buckets := Aggregate(sessions, from, to, DefaultBucketDuration)
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	b := buckets[0]
	if b.Counts[signal.SignalStalledNode] != 2 {
		t.Errorf("stalled_node = %d, want 2", b.Counts[signal.SignalStalledNode])
	}
	if b.Counts[signal.SignalFailedHandoff] != 1 {
		t.Errorf("failed_handoff = %d, want 1", b.Counts[signal.SignalFailedHandoff])
	}
	if b.Counts[signal.SignalDuplicateSubagentWork] != 1 {
		t.Errorf("duplicate_subagent_work = %d, want 1", b.Counts[signal.SignalDuplicateSubagentWork])
	}
}

// TestAggregate_SessionsField_DistinctIDsOnly verifies that two snapshots with
// the same SessionID in one bucket are counted as Sessions=1, not 2.
func TestAggregate_SessionsField_DistinctIDsOnly(t *testing.T) {
	from := baseTime
	to := baseTime.Add(1 * 24 * time.Hour)

	sessions := []signalstore.SessionSnapshot{
		{
			SessionID: "dup-session",
			Entries: []signalstore.SnapshotEntry{
				makeSnapshot(0, makeSig(signal.SignalStalledNode)),
				makeSnapshot(0, makeSig(signal.SignalStalledNode)), // same session, second entry
			},
		},
	}

	buckets := Aggregate(sessions, from, to, DefaultBucketDuration)
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].Sessions != 1 {
		t.Errorf("Sessions = %d, want 1 (same sessionID should deduplicate)", buckets[0].Sessions)
	}
}

// TestAggregate_OutOfRangeSnapshots_Ignored checks that snapshots with
// CapturedAt before `from` are not counted in any bucket.
func TestAggregate_OutOfRangeSnapshots_Ignored(t *testing.T) {
	from := baseTime.Add(1 * 24 * time.Hour) // starts at day 1
	to := baseTime.Add(2 * 24 * time.Hour)

	// This snapshot is at day 0 — before `from`.
	sessions := []signalstore.SessionSnapshot{
		{SessionID: "old", Entries: []signalstore.SnapshotEntry{makeSnapshot(0, makeSig(signal.SignalStalledNode))}},
	}

	buckets := Aggregate(sessions, from, to, DefaultBucketDuration)
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket for [day1, day2), got %d", len(buckets))
	}
	if !buckets[0].IsEmpty() {
		t.Errorf("bucket should be empty (out-of-range snapshot not counted), got %+v", buckets[0])
	}
}

// TestAggregate_HandlesEmptyInput checks that empty sessions produces all-empty buckets.
func TestAggregate_HandlesEmptyInput(t *testing.T) {
	from := baseTime
	to := baseTime.Add(3 * 24 * time.Hour)

	buckets := Aggregate(nil, from, to, DefaultBucketDuration)
	if len(buckets) != 3 {
		t.Fatalf("expected 3 empty buckets for 3-day range, got %d", len(buckets))
	}
	for i, b := range buckets {
		if !b.IsEmpty() {
			t.Errorf("bucket %d should be empty for nil input", i)
		}
	}
}

// TestAggregate_HandlesMultiProviderTier checks that signals with ProviderTier
// "aggregate" and "presence" are counted normally (not dropped).
// Phase 3 policy: lower-tier signals degrade gracefully but are NOT dropped.
func TestAggregate_HandlesMultiProviderTier(t *testing.T) {
	from := baseTime
	to := baseTime.Add(1 * 24 * time.Hour)

	aggregateSig := signal.Signal{
		Type:         signal.SignalStalledNode,
		ProviderTier: "aggregate",
		SpanIDs:      []string{},
	}
	presenceSig := signal.Signal{
		Type:         signal.SignalFailedHandoff,
		ProviderTier: "presence",
		SpanIDs:      []string{},
	}

	sessions := []signalstore.SessionSnapshot{
		{SessionID: "m1", Entries: []signalstore.SnapshotEntry{
			{CapturedAt: baseTime, Signals: []signal.Signal{aggregateSig, presenceSig}},
		}},
	}

	buckets := Aggregate(sessions, from, to, DefaultBucketDuration)
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].Counts[signal.SignalStalledNode] != 1 {
		t.Errorf("aggregate-tier signal not counted: stalled_node = %d, want 1", buckets[0].Counts[signal.SignalStalledNode])
	}
	if buckets[0].Counts[signal.SignalFailedHandoff] != 1 {
		t.Errorf("presence-tier signal not counted: failed_handoff = %d, want 1", buckets[0].Counts[signal.SignalFailedHandoff])
	}
}

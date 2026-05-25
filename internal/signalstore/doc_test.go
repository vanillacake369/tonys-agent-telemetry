package signalstore_test

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
)

// TestDocWireFormat asserts that the example wire format documented in doc.go
// can be parsed into the expected Go types. A future Phase 3 implementer
// should be able to copy this example as a test fixture.
func TestDocWireFormat_ExampleParsesCorrectly(t *testing.T) {
	// Canonical 3-line example:
	//   line 1: Header JSON
	//   line 2: SnapshotEntry JSON
	//   line 3: SnapshotEntry JSON (empty signals = valid)
	headerLine := `{"schema_version":"1","written_at":"2026-05-26T09:00:00Z","producer":"tonys-agent-telemetry dev"}`
	entryLine1 := `{"captured_at":"2026-05-26T09:00:01Z","signals":[{"id":"abc12345","type":"stalled_node","trace_id":"t1","span_ids":["s1"],"evidence":{"stall_duration_ms":14200},"confidence":0.71,"emitted_at":"2026-05-26T09:00:00Z","provider_tier":"full"}]}`
	entryLine2 := `{"captured_at":"2026-05-26T09:00:02Z","signals":[]}`

	lines := strings.Join([]string{headerLine, entryLine1, entryLine2}, "\n") + "\n"

	sc := bufio.NewScanner(strings.NewReader(lines))

	// Line 1: header.
	if !sc.Scan() {
		t.Fatal("no header line")
	}
	var hdr signalstore.Header
	if err := json.Unmarshal(sc.Bytes(), &hdr); err != nil {
		t.Fatalf("parse header: %v", err)
	}
	if hdr.SchemaVersion != "1" {
		t.Errorf("SchemaVersion: %q", hdr.SchemaVersion)
	}
	wantTime := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	if !hdr.WrittenAt.Equal(wantTime) {
		t.Errorf("WrittenAt: %v", hdr.WrittenAt)
	}

	// Lines 2+: entries.
	var entries []signalstore.SnapshotEntry
	for sc.Scan() {
		var e signalstore.SnapshotEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("parse entry: %v", err)
		}
		entries = append(entries, e)
	}
	if sc.Err() != nil {
		t.Fatal(sc.Err())
	}

	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	// First entry has one signal.
	if len(entries[0].Signals) != 1 {
		t.Fatalf("entry[0] signals = %d, want 1", len(entries[0].Signals))
	}
	got := entries[0].Signals[0]
	if got.Type != signal.SignalStalledNode {
		t.Errorf("type: %q", got.Type)
	}
	if got.ProviderTier != "full" {
		t.Errorf("provider_tier: %q", got.ProviderTier)
	}

	// Second entry has zero signals (empty snapshot is valid).
	if len(entries[1].Signals) != 0 {
		t.Fatalf("entry[1] signals = %d, want 0", len(entries[1].Signals))
	}
}

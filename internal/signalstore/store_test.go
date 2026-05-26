//go:build !windows

package signalstore_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
)

// makeSignal builds a minimal Signal for use in tests.
func makeSignal(typ signal.SignalType, traceID string) signal.Signal {
	return signal.Signal{
		ID:           "test-id-" + traceID,
		Type:         typ,
		TraceID:      traceID,
		SpanIDs:      []string{"span-1"},
		Evidence:     map[string]any{"key": "value"},
		Confidence:   0.9,
		EmittedAt:    time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC),
		ProviderTier: "full",
	}
}

func TestStore_AppendThenLoadSession_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	sigs := []signal.Signal{makeSignal(signal.SignalStalledNode, "trace-1")}
	if err := store.Append("session-1", sigs); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, err := store.LoadSession("session-1")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}

	gotSigs := entries[0].Signals
	if len(gotSigs) != len(sigs) {
		t.Fatalf("len(signals) = %d, want %d", len(gotSigs), len(sigs))
	}
	if gotSigs[0].ID != sigs[0].ID {
		t.Errorf("Signal.ID: got %q want %q", gotSigs[0].ID, sigs[0].ID)
	}
	if gotSigs[0].Type != sigs[0].Type {
		t.Errorf("Signal.Type: got %q want %q", gotSigs[0].Type, sigs[0].Type)
	}
	if gotSigs[0].TraceID != sigs[0].TraceID {
		t.Errorf("Signal.TraceID: got %q want %q", gotSigs[0].TraceID, sigs[0].TraceID)
	}
	if gotSigs[0].Confidence != sigs[0].Confidence {
		t.Errorf("Signal.Confidence: got %v want %v", gotSigs[0].Confidence, sigs[0].Confidence)
	}
}

func TestStore_MultipleAppends_PreserveOrder(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	first := []signal.Signal{makeSignal(signal.SignalStalledNode, "trace-1")}
	second := []signal.Signal{makeSignal(signal.SignalFailedHandoff, "trace-2")}
	third := []signal.Signal{makeSignal(signal.SignalDuplicateSubagentWork, "trace-3")}

	for _, batch := range [][]signal.Signal{first, second, third} {
		if err := store.Append("session-order", batch); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	entries, err := store.LoadSession("session-order")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}

	wantTypes := []signal.SignalType{
		signal.SignalStalledNode,
		signal.SignalFailedHandoff,
		signal.SignalDuplicateSubagentWork,
	}
	for i, e := range entries {
		if len(e.Signals) != 1 {
			t.Errorf("[%d] len(signals) = %d, want 1", i, len(e.Signals))
			continue
		}
		if e.Signals[0].Type != wantTypes[i] {
			t.Errorf("[%d] type: got %q want %q", i, e.Signals[0].Type, wantTypes[i])
		}
	}
}

func TestStore_LoadSession_MissingFile_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	entries, err := store.LoadSession("does-not-exist")
	if err != nil {
		t.Fatalf("LoadSession on missing file: %v", err)
	}
	if entries == nil {
		t.Fatal("LoadSession returned nil; want empty (non-nil) slice")
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(entries))
	}
}

func TestStore_LoadSession_SchemaMismatch_ReturnsErrSchemaMismatch(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	// Write a file with a bad header directly.
	badHeader := signalstore.Header{
		SchemaVersion: "99",
		WrittenAt:     time.Now(),
		Producer:      "test",
	}
	b, err := json.Marshal(badHeader)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bad-session.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write(append(b, '\n'))
	f.Close()

	_, err = store.LoadSession("bad-session")
	if !errors.Is(err, signalstore.ErrSchemaMismatch) {
		t.Errorf("expected ErrSchemaMismatch, got %v", err)
	}
}

func TestStore_LoadRange_FiltersByTime(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	base := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)

	// Write JSONL files directly so CapturedAt is under test control.
	writeSessionFile := func(t *testing.T, sessionID string, capturedAt time.Time, sigs []signal.Signal) {
		t.Helper()
		hdr := signalstore.Header{
			SchemaVersion: signalstore.CurrentSchemaVersion,
			WrittenAt:     capturedAt,
			Producer:      "test",
		}
		entry := signalstore.SnapshotEntry{
			CapturedAt: capturedAt,
			Signals:    sigs,
		}
		name, err := signalstore.SessionFilename(sessionID)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		if err := enc.Encode(hdr); err != nil {
			t.Fatal(err)
		}
		if err := enc.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}

	// session-early: captured at base - 2h (outside range)
	writeSessionFile(t, "session-early", base.Add(-2*time.Hour),
		[]signal.Signal{makeSignal(signal.SignalStalledNode, "trace-early")})

	// session-in: captured at base (inside range)
	writeSessionFile(t, "session-in", base,
		[]signal.Signal{makeSignal(signal.SignalFailedHandoff, "trace-in")})

	// session-late: captured at base + 3h (outside range)
	writeSessionFile(t, "session-late", base.Add(3*time.Hour),
		[]signal.Signal{makeSignal(signal.SignalDuplicateSubagentWork, "trace-late")})

	from := base.Add(-1 * time.Hour)
	to := base.Add(1 * time.Hour)

	snapshots, err := store.LoadRange(from, to)
	if err != nil {
		t.Fatalf("LoadRange: %v", err)
	}

	// Only session-in should appear.
	if len(snapshots) != 1 {
		t.Fatalf("len(snapshots) = %d, want 1", len(snapshots))
	}
	if snapshots[0].SessionID != "session-in" {
		t.Errorf("SessionID = %q, want %q", snapshots[0].SessionID, "session-in")
	}

	_ = store // used for NewStoreAt construction
}

func TestStore_LoadRange_SkipsUnreadableSession_DoesNotFail(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	// Write one valid session.
	sigs := []signal.Signal{makeSignal(signal.SignalStalledNode, "trace-good")}
	if err := store.Append("session-good", sigs); err != nil {
		t.Fatal(err)
	}

	// Write a corrupt session file (not valid JSONL).
	corruptPath := filepath.Join(dir, "session-corrupt.jsonl")
	if err := os.WriteFile(corruptPath, []byte("not-json\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	from := time.Time{}
	to := time.Now().Add(24 * time.Hour)

	snapshots, err := store.LoadRange(from, to)
	if err != nil {
		t.Fatalf("LoadRange must not fail on unreadable session, got: %v", err)
	}

	// Only the valid session should appear.
	if len(snapshots) != 1 {
		t.Fatalf("len(snapshots) = %d, want 1", len(snapshots))
	}
	if snapshots[0].SessionID != "session-good" {
		t.Errorf("SessionID = %q, want session-good", snapshots[0].SessionID)
	}
}

func TestStore_Append_PreservesEvidenceMap(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	sig := makeSignal(signal.SignalStalledNode, "trace-ev")
	sig.Evidence = map[string]any{
		"str_field":  "hello",
		"int_field":  float64(42), // JSON numbers unmarshal to float64
		"bool_field": true,
		"nested": map[string]any{
			"inner": "value",
		},
	}

	if err := store.Append("session-ev", []signal.Signal{sig}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.LoadSession("session-ev")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || len(entries[0].Signals) != 1 {
		t.Fatal("unexpected entry count")
	}

	got := entries[0].Signals[0].Evidence
	if got["str_field"] != "hello" {
		t.Errorf("str_field: %v", got["str_field"])
	}
	if got["int_field"] != float64(42) {
		t.Errorf("int_field: %v", got["int_field"])
	}
	if got["bool_field"] != true {
		t.Errorf("bool_field: %v", got["bool_field"])
	}
	nested, ok := got["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested not map[string]any: %T", got["nested"])
	}
	if nested["inner"] != "value" {
		t.Errorf("nested.inner: %v", nested["inner"])
	}
}

func TestStore_Append_PreservesTimezone(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	// Use a non-UTC location to test timezone round-trip.
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone not available: %v", err)
	}

	sig := makeSignal(signal.SignalStalledNode, "trace-tz")
	nyTime := time.Date(2026, 5, 26, 9, 0, 0, 0, loc)
	sig.EmittedAt = nyTime

	if err := store.Append("session-tz", []signal.Signal{sig}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.LoadSession("session-tz")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || len(entries[0].Signals) != 1 {
		t.Fatal("unexpected entry count")
	}

	got := entries[0].Signals[0].EmittedAt
	// time.Time.Equal compares the underlying instant regardless of location.
	if !got.Equal(nyTime) {
		t.Errorf("EmittedAt: got %v want %v", got, nyTime)
	}
}

func TestStore_Append_PreservesProviderTier(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	sig := makeSignal(signal.SignalStalledNode, "trace-tier")
	sig.ProviderTier = "aggregate"

	if err := store.Append("session-tier", []signal.Signal{sig}); err != nil {
		t.Fatal(err)
	}

	entries, err := store.LoadSession("session-tier")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || len(entries[0].Signals) != 1 {
		t.Fatal("unexpected entry count")
	}
	if entries[0].Signals[0].ProviderTier != "aggregate" {
		t.Errorf("ProviderTier: got %q want %q", entries[0].Signals[0].ProviderTier, "aggregate")
	}
}

func TestStore_Append_AcceptsEmptySignalsSlice(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	if err := store.Append("session-empty", []signal.Signal{}); err != nil {
		t.Fatalf("Append with empty slice: %v", err)
	}

	entries, err := store.LoadSession("session-empty")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if entries[0].Signals == nil {
		t.Error("Signals should be non-nil empty slice, got nil")
	}
}

func TestStore_Concurrent_Appends_SerializeViaFlock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("flock test not applicable on Windows")
	}

	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	const goroutines = 8
	const appendsEach = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < appendsEach; i++ {
				sig := makeSignal(signal.SignalStalledNode, fmt.Sprintf("trace-g%d-i%d", g, i))
				if err := store.Append("session-concurrent", []signal.Signal{sig}); err != nil {
					// Non-fatal: report but don't stop other goroutines.
					t.Errorf("goroutine %d append %d: %v", g, i, err)
				}
			}
		}()
	}
	wg.Wait()

	entries, err := store.LoadSession("session-concurrent")
	if err != nil {
		t.Fatalf("LoadSession after concurrent appends: %v", err)
	}

	total := goroutines * appendsEach
	if len(entries) != total {
		t.Errorf("len(entries) = %d, want %d (no interleaved writes)", len(entries), total)
	}

	// Verify every entry is valid JSON (no interleaving corrupted lines).
	for i, e := range entries {
		if len(e.Signals) != 1 {
			t.Errorf("[%d] len(signals) = %d, want 1", i, len(e.Signals))
		}
	}

	// Ensure no two entries have the same signal — all goroutine/iteration combos present.
	seen := make(map[string]bool)
	for _, e := range entries {
		for _, s := range e.Signals {
			if seen[s.TraceID] {
				t.Errorf("duplicate TraceID %q in concurrent appends", s.TraceID)
			}
			seen[s.TraceID] = true
		}
	}
}

// ── ν-1: file rotation tests ──────────────────────────────────────────────────

// TestStore_Append_RotatesAtMaxBytes verifies that Append rotates the session
// file (renames it to <sessionID>.<unixtime>.jsonl) when the file would exceed
// MaxFileBytes, and then writes the new entry to a fresh file.
func TestStore_Append_RotatesAtMaxBytes(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)
	signalstore.SetMaxFileBytesForTest(1)       // trigger rotation after first write
	defer signalstore.SetMaxFileBytesForTest(0) // reset to production default

	sig1 := makeSignal(signal.SignalStalledNode, "trace-rot-1")
	if err := store.Append("rot-session", []signal.Signal{sig1}); err != nil {
		t.Fatalf("first Append: %v", err)
	}

	sig2 := makeSignal(signal.SignalFailedHandoff, "trace-rot-2")
	if err := store.Append("rot-session", []signal.Signal{sig2}); err != nil {
		t.Fatalf("second Append (should rotate): %v", err)
	}

	// Exactly two .jsonl files should exist under the store root.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var jsonlFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") {
			jsonlFiles = append(jsonlFiles, e.Name())
		}
	}
	if len(jsonlFiles) != 2 {
		t.Fatalf("expected 2 .jsonl files after rotation, got %d: %v", len(jsonlFiles), jsonlFiles)
	}

	// The active file must be named exactly <sanitized-id>.jsonl.
	activeName, _ := signalstore.SessionFilename("rot-session")
	hasActive := false
	hasArchive := false
	for _, name := range jsonlFiles {
		if name == activeName {
			hasActive = true
		} else if strings.HasPrefix(name, "rot-session.") && strings.HasSuffix(name, ".jsonl") {
			hasArchive = true
		}
	}
	if !hasActive {
		t.Errorf("active file %q not found among %v", activeName, jsonlFiles)
	}
	if !hasArchive {
		t.Errorf("archive file (rot-session.<unixtime>.jsonl) not found among %v", jsonlFiles)
	}
}

// TestStore_LoadSession_ReadsAcrossRotations asserts that LoadSession returns
// entries from both the active file and any archived rotation files, in order.
func TestStore_LoadSession_ReadsAcrossRotations(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)
	signalstore.SetMaxFileBytesForTest(1)
	defer signalstore.SetMaxFileBytesForTest(0)

	sig1 := makeSignal(signal.SignalStalledNode, "trace-cross-1")
	sig2 := makeSignal(signal.SignalFailedHandoff, "trace-cross-2")

	if err := store.Append("cross-session", []signal.Signal{sig1}); err != nil {
		t.Fatalf("first Append: %v", err)
	}
	if err := store.Append("cross-session", []signal.Signal{sig2}); err != nil {
		t.Fatalf("second Append: %v", err)
	}

	entries, err := store.LoadSession("cross-session")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2 (across rotation)", len(entries))
	}

	// Order: archived file first (older), active file second (newer).
	if entries[0].Signals[0].TraceID != "trace-cross-1" {
		t.Errorf("[0] TraceID = %q, want trace-cross-1", entries[0].Signals[0].TraceID)
	}
	if entries[1].Signals[0].TraceID != "trace-cross-2" {
		t.Errorf("[1] TraceID = %q, want trace-cross-2", entries[1].Signals[0].TraceID)
	}
}

// TestStore_LoadRange_IncludesArchivedRotations asserts that LoadRange also
// reads archived rotation files and includes their entries in results.
func TestStore_LoadRange_IncludesArchivedRotations(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)
	signalstore.SetMaxFileBytesForTest(1)
	defer signalstore.SetMaxFileBytesForTest(0)

	base := time.Date(2026, 5, 26, 10, 0, 0, 0, time.UTC)

	sig1 := makeSignal(signal.SignalStalledNode, "trace-range-1")
	sig2 := makeSignal(signal.SignalFailedHandoff, "trace-range-2")

	if err := store.Append("range-session", []signal.Signal{sig1}); err != nil {
		t.Fatalf("first Append: %v", err)
	}
	if err := store.Append("range-session", []signal.Signal{sig2}); err != nil {
		t.Fatalf("second Append: %v", err)
	}

	from := base.Add(-24 * time.Hour)
	to := base.Add(24 * time.Hour)
	snapshots, err := store.LoadRange(from, to)
	if err != nil {
		t.Fatalf("LoadRange: %v", err)
	}

	// All entries must be present (from both files).
	totalEntries := 0
	for _, ss := range snapshots {
		if strings.HasPrefix(ss.SessionID, "range-session") {
			totalEntries += len(ss.Entries)
		}
	}
	if totalEntries < 2 {
		t.Errorf("LoadRange returned %d entries for range-session across rotations, want >= 2", totalEntries)
	}
}

// ── ν-2: concurrent read/write flock safety ───────────────────────────────────

// TestStore_ConcurrentReadDuringWrite_NoPartialEntries runs a writer goroutine
// alongside N reader goroutines and asserts that no structurally truncated
// entries (missing required fields) are ever returned.
func TestStore_ConcurrentReadDuringWrite_NoPartialEntries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("flock test not applicable on Windows")
	}
	t.Parallel()

	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)

	// Seed the file so LoadSession has something to read.
	seed := makeSignal(signal.SignalStalledNode, "trace-seed")
	if err := store.Append("rw-session", []signal.Signal{seed}); err != nil {
		t.Fatalf("seed Append: %v", err)
	}

	const iterations = 20

	var wg sync.WaitGroup

	// Writer goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			sig := makeSignal(signal.SignalFailedHandoff, fmt.Sprintf("trace-w%d", i))
			_ = store.Append("rw-session", []signal.Signal{sig})
		}
	}()

	// Reader goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			entries, err := store.LoadSession("rw-session")
			if err != nil {
				// Schema mismatch or other hard error: skip iteration.
				continue
			}
			for _, e := range entries {
				// A structurally valid entry must have a non-zero CapturedAt.
				if e.CapturedAt.IsZero() {
					t.Errorf("iteration %d: entry has zero CapturedAt (truncated?): %+v", i, e)
				}
				// Signals must never be nil (see store.go contract).
				if e.Signals == nil {
					t.Errorf("iteration %d: entry.Signals is nil (truncated?): %+v", i, e)
				}
			}
		}
	}()

	wg.Wait()
}

// ── ν-4: corrupted JSONL / schema-mismatch entry tests ───────────────────────

// TestStore_LoadSession_CorruptedEntryInMiddle_SkipsAndContinues writes
// header + entry1 + garbage-line + entry2 and asserts LoadSession returns
// [entry1, entry2] (corruption silently skipped).
func TestStore_LoadSession_CorruptedEntryInMiddle_SkipsAndContinues(t *testing.T) {
	dir := t.TempDir()

	hdr := signalstore.Header{
		SchemaVersion: signalstore.CurrentSchemaVersion,
		WrittenAt:     time.Now().UTC(),
		Producer:      "test",
	}
	entry1 := signalstore.SnapshotEntry{
		CapturedAt: time.Now().UTC(),
		Signals:    []signal.Signal{makeSignal(signal.SignalStalledNode, "trace-c1")},
	}
	entry2 := signalstore.SnapshotEntry{
		CapturedAt: time.Now().Add(time.Second).UTC(),
		Signals:    []signal.Signal{makeSignal(signal.SignalFailedHandoff, "trace-c2")},
	}

	hdrB, _ := json.Marshal(hdr)
	e1B, _ := json.Marshal(entry1)
	e2B, _ := json.Marshal(entry2)

	content := string(hdrB) + "\n" +
		string(e1B) + "\n" +
		"NOT-VALID-JSON{{{garbage\n" +
		string(e2B) + "\n"

	name, _ := signalstore.SessionFilename("corrupt-middle")
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store := signalstore.NewStoreAt(dir)
	entries, err := store.LoadSession("corrupt-middle")
	if err != nil {
		t.Fatalf("LoadSession must not fail on corrupt middle entry: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2 (skipped corruption)", len(entries))
	}
	if entries[0].Signals[0].TraceID != "trace-c1" {
		t.Errorf("[0] TraceID = %q, want trace-c1", entries[0].Signals[0].TraceID)
	}
	if entries[1].Signals[0].TraceID != "trace-c2" {
		t.Errorf("[1] TraceID = %q, want trace-c2", entries[1].Signals[0].TraceID)
	}
}

// TestStore_LoadSession_TruncatedLastLine_NoPanic writes header + entry1 +
// partial-JSON last line (missing closing brace) and asserts no panic and
// that LoadSession returns [entry1].
func TestStore_LoadSession_TruncatedLastLine_NoPanic(t *testing.T) {
	dir := t.TempDir()

	hdr := signalstore.Header{
		SchemaVersion: signalstore.CurrentSchemaVersion,
		WrittenAt:     time.Now().UTC(),
		Producer:      "test",
	}
	entry1 := signalstore.SnapshotEntry{
		CapturedAt: time.Now().UTC(),
		Signals:    []signal.Signal{makeSignal(signal.SignalStalledNode, "trace-trunc")},
	}

	hdrB, _ := json.Marshal(hdr)
	e1B, _ := json.Marshal(entry1)
	truncatedEntry := `{"captured_at":"2026-05-26T09:00:01Z","signals":[{"id":"x`

	content := string(hdrB) + "\n" + string(e1B) + "\n" + truncatedEntry + "\n"

	name, _ := signalstore.SessionFilename("trunc-session")
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	store := signalstore.NewStoreAt(dir)

	// Must not panic.
	entries, err := store.LoadSession("trunc-session")
	if err != nil {
		t.Fatalf("LoadSession must not fail on truncated last line: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1 (truncated last line skipped)", len(entries))
	}
	if entries[0].Signals[0].TraceID != "trace-trunc" {
		t.Errorf("TraceID = %q, want trace-trunc", entries[0].Signals[0].TraceID)
	}
}

// TestStore_LoadRange_WrongSchemaVersion_SkipsFile writes a file with
// schema_version "99" alongside a valid file and asserts LoadRange returns
// results from the valid file only (no error).
func TestStore_LoadRange_WrongSchemaVersion_SkipsFile(t *testing.T) {
	dir := t.TempDir()

	// Write the invalid-schema file.
	badHdr := signalstore.Header{
		SchemaVersion: "99",
		WrittenAt:     time.Now().UTC(),
		Producer:      "test",
	}
	badB, _ := json.Marshal(badHdr)
	if err := os.WriteFile(filepath.Join(dir, "bad-schema-v99.jsonl"), append(badB, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	// Write a valid file.
	store := signalstore.NewStoreAt(dir)
	sig := makeSignal(signal.SignalStalledNode, "trace-valid")
	if err := store.Append("good-schema", []signal.Signal{sig}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	from := time.Time{}
	to := time.Now().Add(24 * time.Hour)
	snapshots, err := store.LoadRange(from, to)
	if err != nil {
		t.Fatalf("LoadRange must not fail on schema-mismatch file: %v", err)
	}

	// Find any snapshot that came from the bad-schema file.
	for _, ss := range snapshots {
		if ss.SessionID == "bad-schema-v99" {
			t.Errorf("LoadRange should skip schema-99 file, but got session %q", ss.SessionID)
		}
	}

	// The good session must be present.
	found := false
	for _, ss := range snapshots {
		if ss.SessionID == "good-schema" {
			found = true
		}
	}
	if !found {
		t.Errorf("LoadRange did not include valid session 'good-schema'; got: %v",
			func() []string {
				var ids []string
				for _, ss := range snapshots {
					ids = append(ids, ss.SessionID)
				}
				sort.Strings(ids)
				return ids
			}())
	}
}

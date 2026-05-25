package tui

import (
	"sync"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// buildSpanForTest creates a minimal Span suitable for signal extraction.
func buildSpanForTest(traceID, spanID string, dur time.Duration) telemetry.Span {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return telemetry.Span{
		TraceID:   traceID,
		SpanID:    spanID,
		System:    "anthropic",
		StartTime: start,
		EndTime:   start.Add(dur),
		Status:    "done",
		Attrs:     map[string]string{},
	}
}

// TestPersistence_FlushWritesToStore verifies that FlushCmd persists signals
// when non-trivial spans are provided that trigger at least one signal.
// We use a span duration > DefaultStallThreshold to guarantee a stalled_node signal.
func TestPersistence_FlushWritesToStore(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)
	p := NewTrendsPersistence(store)

	// A single leaf span lasting 30s (well over the 10s stall threshold).
	spans := []telemetry.Span{
		buildSpanForTest("trace-1", "span-1", 30*time.Second),
	}

	// FlushCmd now returns a cmd closure; invoke it to perform the work.
	cmd := p.FlushCmd("test-session", spans)
	if cmd != nil {
		_ = cmd()
	}

	// Verify the store received an Append (file should now exist with data).
	entries, err := store.LoadSession("test-session")
	if err != nil {
		t.Fatalf("LoadSession error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one SnapshotEntry after FlushCmd, got none")
	}
}

// TestPersistence_FlushHandlesEmptySpans verifies that passing an empty spans
// slice does NOT write an empty snapshot to the store.
func TestPersistence_FlushHandlesEmptySpans(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)
	p := NewTrendsPersistence(store)

	// No spans → FlushCmd returns nil immediately (no work deferred).
	cmd := p.FlushCmd("empty-session", nil)
	if cmd != nil {
		_ = cmd()
	}

	entries, err := store.LoadSession("empty-session")
	if err != nil {
		t.Fatalf("LoadSession error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no entries for empty spans, got %d", len(entries))
	}
}

// TestPersistence_FlushErrorIsLogged_NotPanic verifies that a store write failure
// (simulated by using a path that doesn't exist as a directory) does not panic.
func TestPersistence_FlushErrorIsLogged_NotPanic(t *testing.T) {
	// Use a non-existent directory so Append will fail.
	store := signalstore.NewStoreAt("/nonexistent-path-xyz-abc")
	p := NewTrendsPersistence(store)

	// A stalling span to ensure signal extraction produces output.
	spans := []telemetry.Span{
		buildSpanForTest("trace-err", "span-err", 30*time.Second),
	}

	// This should not panic, even if the store write fails.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("FlushCmd panicked: %v", r)
		}
	}()
	cmd := p.FlushCmd("error-session", spans)
	if cmd != nil {
		_ = cmd()
	}
}

// TestPersistence_FlushCmdReturnsNonNilCmdForWork asserts that calling FlushCmd
// with non-empty spans returns a non-nil cmd, proving the work is deferred
// (not done synchronously on the calling goroutine).
func TestPersistence_FlushCmdReturnsNonNilCmdForWork(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)
	p := NewTrendsPersistence(store)

	spans := []telemetry.Span{
		buildSpanForTest("trace-deferred", "span-d", 30*time.Second),
	}

	cmd := p.FlushCmd("deferred-session", spans)
	if cmd == nil {
		t.Error("FlushCmd with non-empty spans must return a non-nil cmd (work must be deferred)")
	}
}

// TestPersistence_ConcurrentFlushCmds_NoRace runs two flush cmds concurrently
// and asserts no data race on lastFlush. Run with -race to activate the detector.
func TestPersistence_ConcurrentFlushCmds_NoRace(t *testing.T) {
	dir := t.TempDir()
	store := signalstore.NewStoreAt(dir)
	p := NewTrendsPersistence(store)

	spans := []telemetry.Span{
		buildSpanForTest("trace-race-1", "span-r1", 30*time.Second),
	}

	cmd1 := p.FlushCmd("race-session-1", spans)
	cmd2 := p.FlushCmd("race-session-2", spans)

	var wg sync.WaitGroup
	if cmd1 != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cmd1()
		}()
	}
	if cmd2 != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cmd2()
		}()
	}
	wg.Wait()
	// No assertions needed beyond no data race (enforced by -race flag).
}

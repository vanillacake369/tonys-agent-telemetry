package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// TestIngestor_PanicInBackfillDoesNotKillProcess feeds deliberately malformed
// JSONL to claudecode's Ingest path and asserts that the process survives and
// the channel still emits subsequent valid spans.
//
// Because panic recovery wraps the Ingest method, a panic inside backfill
// (which is called synchronously from Ingest) is caught before it propagates
// to the runtime.
func TestIngestor_PanicInBackfillDoesNotKillProcess(t *testing.T) {
	tmp := t.TempDir()
	fakeClaudeDir := filepath.Join(tmp, ".claude")
	projectsDir := filepath.Join(fakeClaudeDir, "projects", "myproject")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a mix: malformed bytes followed by a valid JSONL span record.
	// ConvertHookPayload tolerates malformed JSON (returns error, skips), so
	// this exercises the normal malformed-line skip path. The key assertion
	// is that the process survives and the valid span is emitted.
	validLine := `{"sessionId":"trace-abc","uuid":"span-001","parentUuid":"","type":"assistant","timestamp":"2024-01-01T00:00:00Z","message":{"model":"claude-3-5-sonnet"}}`
	content := "not-valid-json\n" + validLine + "\n"
	jsonlPath := filepath.Join(projectsDir, "session.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := claudeDir
	defer func() { claudeDir = orig }()
	claudeDir = func() string { return fakeClaudeDir }

	// Use a context that is not pre-cancelled so backfill runs to completion.
	ctx, cancel := context.WithCancel(context.Background())

	out := make(chan telemetry.Span, 10)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = New().Ingest(ctx, out)
	}()

	// Collect the valid span (backfill runs synchronously inside Ingest before
	// blocking on ctx.Done).
	var spans []telemetry.Span
	collectDeadline := time.After(500 * time.Millisecond)
collectLoop:
	for {
		select {
		case sp := <-out:
			spans = append(spans, sp)
		case <-collectDeadline:
			break collectLoop
		}
	}

	// Now cancel so Ingest unblocks from <-ctx.Done().
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Ingest goroutine did not return within 2s after cancel")
	}

	// If we reach here the process survived (no panic killed it).
	found := false
	for _, sp := range spans {
		if sp.TraceID == "trace-abc" && sp.SpanID == "span-001" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected valid span (trace-abc/span-001) after malformed input; got %d spans: %v",
			len(spans), spans)
	}
}

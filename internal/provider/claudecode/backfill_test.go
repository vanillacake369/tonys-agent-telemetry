package claudecode

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// writeJSONL writes lines into a JSONL file inside dir.
func writeJSONL(t *testing.T, dir, sessionFile string, lines []string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, sessionFile)
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func setupFakeClaude(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	fake := filepath.Join(tmp, ".claude")
	if err := os.MkdirAll(filepath.Join(fake, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	orig := claudeDir
	t.Cleanup(func() { claudeDir = orig })
	claudeDir = func() string { return fake }
	return fake
}

func collect(t *testing.T, ch <-chan telemetry.Span, timeout time.Duration) []telemetry.Span {
	t.Helper()
	var got []telemetry.Span
	deadline := time.After(timeout)
	for {
		select {
		case s, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, s)
		case <-deadline:
			return got
		}
	}
}

func TestBackfill_EmptyProjectsDir(t *testing.T) {
	setupFakeClaude(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	out := make(chan telemetry.Span, 8)
	if err := New().backfill(ctx, out); err != nil {
		t.Errorf("backfill error: %v", err)
	}
	close(out)
	if got := collect(t, out, 100*time.Millisecond); len(got) != 0 {
		t.Errorf("got %d spans, want 0", len(got))
	}
}

func TestBackfill_OneFileWithThreeMessages(t *testing.T) {
	fake := setupFakeClaude(t)
	projectDir := filepath.Join(fake, "projects", "-fake-proj")
	writeJSONL(t, projectDir, "sess1.jsonl", []string{
		`{"sessionId":"s1","uuid":"u1","type":"user","timestamp":"2026-05-25T10:00:00Z"}`,
		`{"sessionId":"s1","uuid":"u2","parentUuid":"u1","type":"assistant","message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50}},"timestamp":"2026-05-25T10:00:05Z"}`,
		`{"sessionId":"s1","uuid":"u3","parentUuid":"u2","type":"user","timestamp":"2026-05-25T10:00:10Z"}`,
	})

	out := make(chan telemetry.Span, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := New().backfill(ctx, out); err != nil {
		t.Errorf("backfill error: %v", err)
	}
	close(out)

	got := collect(t, out, 100*time.Millisecond)
	if len(got) != 3 {
		t.Fatalf("got %d spans, want 3", len(got))
	}
	if got[0].SpanID != "u1" || got[1].SpanID != "u2" || got[2].SpanID != "u3" {
		t.Errorf("span order wrong: %v", []string{got[0].SpanID, got[1].SpanID, got[2].SpanID})
	}
	if got[1].Model != "claude-sonnet-4-6" || got[1].InputTokens != 100 {
		t.Errorf("assistant span: model=%s tokens=%d", got[1].Model, got[1].InputTokens)
	}
}

func TestBackfill_TwoFilesAcrossProjects(t *testing.T) {
	fake := setupFakeClaude(t)
	writeJSONL(t, filepath.Join(fake, "projects", "-p1"), "s1.jsonl", []string{
		`{"sessionId":"s1","uuid":"u1","type":"user"}`,
	})
	writeJSONL(t, filepath.Join(fake, "projects", "-p2"), "s2.jsonl", []string{
		`{"sessionId":"s2","uuid":"u1","type":"user"}`,
		`{"sessionId":"s2","uuid":"u2","type":"assistant","message":{"model":"m"}}`,
	})

	out := make(chan telemetry.Span, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = New().backfill(ctx, out)
	close(out)

	got := collect(t, out, 100*time.Millisecond)
	if len(got) != 3 {
		t.Errorf("got %d spans, want 3", len(got))
	}
}

func TestBackfill_MalformedLineSkipped(t *testing.T) {
	fake := setupFakeClaude(t)
	writeJSONL(t, filepath.Join(fake, "projects", "-p"), "s.jsonl", []string{
		`not json`,
		`{"sessionId":"s","uuid":"u","type":"user"}`,
		`also not json`,
	})

	out := make(chan telemetry.Span, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = New().backfill(ctx, out)
	close(out)

	got := collect(t, out, 100*time.Millisecond)
	if len(got) != 1 || got[0].SpanID != "u" {
		t.Errorf("got %v, want 1 span with SpanID=u", got)
	}
}

func TestBackfill_SkipsRecordsWithoutTraceOrSpan(t *testing.T) {
	fake := setupFakeClaude(t)
	writeJSONL(t, filepath.Join(fake, "projects", "-p"), "s.jsonl", []string{
		`{"type":"user"}`,                            // no sessionId, no uuid
		`{"sessionId":"s","type":"user"}`,            // no uuid
		`{"uuid":"u","type":"user"}`,                 // no sessionId
		`{"sessionId":"s","uuid":"u","type":"user"}`, // valid
	})

	out := make(chan telemetry.Span, 8)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = New().backfill(ctx, out)
	close(out)

	got := collect(t, out, 100*time.Millisecond)
	if len(got) != 1 {
		t.Errorf("got %d spans, want 1 (3 skipped for missing fields)", len(got))
	}
}

func TestBackfill_ContextCancelStopsEmission(t *testing.T) {
	fake := setupFakeClaude(t)
	// Generate many lines so cancel can interrupt mid-scan.
	var lines []string
	for n := 0; n < 1000; n++ {
		lines = append(lines, `{"sessionId":"s","uuid":"u`+itoa(n)+`","type":"user"}`)
	}
	writeJSONL(t, filepath.Join(fake, "projects", "-p"), "s.jsonl", lines)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()                      // always release the context, even if the test fails early
	out := make(chan telemetry.Span, 1) // small buffer so backfill blocks on send

	done := make(chan error, 1)
	go func() { done <- New().backfill(ctx, out) }()

	// Drain a couple then cancel.
	<-out
	<-out
	cancel()

	// backfill should return promptly.
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("backfill returned %v, want context.Canceled", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("backfill did not return within 500ms of cancel")
	}
}

func TestIngest_IncludesBackfillOutput(t *testing.T) {
	fake := setupFakeClaude(t)
	writeJSONL(t, filepath.Join(fake, "projects", "-p"), "s.jsonl", []string{
		`{"sessionId":"s","uuid":"u1","type":"user"}`,
		`{"sessionId":"s","uuid":"u2","type":"assistant","message":{"model":"m"}}`,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // ensure context is always released
	out := make(chan telemetry.Span, 8)
	go func() {
		_ = New().Ingest(ctx, out)
		close(out)
	}()

	// Collect for a bit then cancel.
	deadline := time.After(300 * time.Millisecond)
	var got []telemetry.Span
loop:
	for {
		select {
		case s, ok := <-out:
			if !ok {
				break loop
			}
			got = append(got, s)
		case <-deadline:
			cancel()
		}
	}

	if len(got) < 2 {
		t.Errorf("got %d spans, want >= 2 (backfill should emit at least 2)", len(got))
	}
}

// itoa is a tiny helper to avoid importing strconv in test fixtures.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}

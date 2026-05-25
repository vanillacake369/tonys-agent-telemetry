package claudecode

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

func TestIngestor_ProviderID(t *testing.T) {
	if got := New().ProviderID(); got != "claudecode" {
		t.Errorf("ProviderID = %q, want claudecode", got)
	}
}

func TestIngestor_DetectMissingDir(t *testing.T) {
	orig := claudeDir
	defer func() { claudeDir = orig }()
	claudeDir = func() string { return "/nonexistent/path/that/should/not/exist" }

	if New().Detect(context.Background()) {
		t.Error("Detect = true for missing dir, want false")
	}
}

func TestIngestor_DetectExistingDir(t *testing.T) {
	tmp := t.TempDir()
	fakeClaudeDir := filepath.Join(tmp, ".claude")
	if err := mkdir(fakeClaudeDir); err != nil {
		t.Fatal(err)
	}

	orig := claudeDir
	defer func() { claudeDir = orig }()
	claudeDir = func() string { return fakeClaudeDir }

	if !New().Detect(context.Background()) {
		t.Error("Detect = false for existing dir, want true")
	}
}

func TestIngestor_IngestReturnsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan telemetry.Span, 1)
	done := make(chan error, 1)
	go func() { done <- New().Ingest(ctx, out) }()

	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Ingest returned %v, want context.Canceled", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Ingest did not return within 200ms of cancel")
	}
}

// mkdir is a tiny test helper using os.MkdirAll, factored out so tests don't
// each import "os".
func mkdir(path string) error {
	return mkdirAll(path)
}

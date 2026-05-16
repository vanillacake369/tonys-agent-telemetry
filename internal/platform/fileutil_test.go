package platform

import (
	"os"
	"strings"
	"testing"
)

func TestFileMtime_ExistingFile(t *testing.T) {
	// Create a temp file to stat
	f, err := os.CreateTemp("", "platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	mtime, err := FileMtime(f.Name())
	if err != nil {
		t.Fatalf("FileMtime failed on existing file: %v", err)
	}
	if mtime.IsZero() {
		t.Fatal("FileMtime returned zero time")
	}
}

func TestFileMtime_Nonexistent(t *testing.T) {
	_, err := FileMtime("/tmp/tonys-agent-telemetry-does-not-exist-xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestHomeDir_NonEmpty(t *testing.T) {
	h := HomeDir()
	if h == "" {
		t.Fatal("HomeDir returned empty string")
	}
}

func TestHomeDir_StartsWithSlash(t *testing.T) {
	h := HomeDir()
	if !strings.HasPrefix(h, "/") {
		t.Fatalf("HomeDir %q does not start with /", h)
	}
}

func TestClaudeDir_EndsWithDotClaude(t *testing.T) {
	d := ClaudeDir()
	if !strings.HasSuffix(d, "/.claude") {
		t.Fatalf("ClaudeDir %q does not end with /.claude", d)
	}
}

func TestClaudeDir_UnderHomeDir(t *testing.T) {
	home := HomeDir()
	claude := ClaudeDir()
	if !strings.HasPrefix(claude, home) {
		t.Fatalf("ClaudeDir %q is not under HomeDir %q", claude, home)
	}
}

package tui

import (
	"os"
	"strings"
	"testing"
)

// TestResolveSessionID_DeterministicForSameCWD verifies that two calls from
// the same working directory return identical session IDs.
func TestResolveSessionID_DeterministicForSameCWD(t *testing.T) {
	id1 := ResolveSessionID()
	id2 := ResolveSessionID()
	if id1 != id2 {
		t.Errorf("ResolveSessionID non-deterministic: %q vs %q", id1, id2)
	}
}

// TestResolveSessionID_DifferentForDifferentCWD verifies that two different
// absolute paths produce different session IDs.
func TestResolveSessionID_DifferentForDifferentCWD(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	id1 := resolveSessionIDFromPath(dir1)
	id2 := resolveSessionIDFromPath(dir2)

	if id1 == id2 {
		t.Errorf("ResolveSessionID produced identical IDs for different dirs: %q", id1)
	}
}

// TestResolveSessionID_FilesystemSafe verifies the returned ID contains no
// slashes, does not start with a dot, and is at most 32 characters long.
func TestResolveSessionID_FilesystemSafe(t *testing.T) {
	id := ResolveSessionID()
	if strings.Contains(id, "/") {
		t.Errorf("sessionID contains slash: %q", id)
	}
	if strings.HasPrefix(id, ".") {
		t.Errorf("sessionID starts with dot: %q", id)
	}
	if len(id) > 32 {
		t.Errorf("sessionID too long (%d chars > 32): %q", len(id), id)
	}
}

// TestResolveSessionID_FallbackToConstant documents and exercises the fallback
// path. We cannot make os.Getwd() fail in a running process, so we test the
// exported helper directly with an empty path, which triggers the fallback.
func TestResolveSessionID_FallbackToConstant(t *testing.T) {
	id := resolveSessionIDFromPath("")
	const fallback = "tui-session"
	if id != fallback {
		t.Errorf("empty path fallback: got %q, want %q", id, fallback)
	}
}

// TestResolveSessionID_PrefixFormat verifies the "proj-" prefix and 8-hex-char suffix.
func TestResolveSessionID_PrefixFormat(t *testing.T) {
	dir := t.TempDir()
	id := resolveSessionIDFromPath(dir)
	if !strings.HasPrefix(id, "proj-") {
		t.Errorf("sessionID missing 'proj-' prefix: %q", id)
	}
	suffix := strings.TrimPrefix(id, "proj-")
	if len(suffix) != 8 {
		t.Errorf("sessionID suffix should be 8 hex chars, got %d: %q", len(suffix), suffix)
	}
}

// TestResolveSessionID_RealCWD_NoRace exercises concurrent calls from the real
// working directory. Run with -race to activate the race detector.
func TestResolveSessionID_RealCWD_NoRace(t *testing.T) {
	// Ensure the real CWD is available.
	if _, err := os.Getwd(); err != nil {
		t.Skip("os.Getwd unavailable")
	}

	const goroutines = 16
	results := make(chan string, goroutines)
	for range goroutines {
		go func() {
			results <- ResolveSessionID()
		}()
	}

	first := <-results
	for range goroutines - 1 {
		id := <-results
		if id != first {
			t.Errorf("concurrent calls disagree: %q vs %q", first, id)
		}
	}
}

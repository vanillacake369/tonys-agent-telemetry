package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// fallbackSessionID is returned when os.Getwd fails or the path is empty.
const fallbackSessionID = "tui-session"

// ResolveSessionID returns a session identifier derived from the current
// working directory. The same project always resolves to the same ID; the
// ID is short and filesystem-safe (no slashes).
//
// Format: "proj-<first-8-of-sha256-of-abs-cwd>". Falls back to "tui-session"
// if os.Getwd() fails.
func ResolveSessionID() string {
	cwd, err := os.Getwd()
	if err != nil {
		return fallbackSessionID
	}
	return resolveSessionIDFromPath(cwd)
}

// resolveSessionIDFromPath is the pure, testable core of ResolveSessionID.
// An empty path returns the fallback constant.
func resolveSessionIDFromPath(absPath string) string {
	if absPath == "" {
		return fallbackSessionID
	}
	sum := sha256.Sum256([]byte(absPath))
	return "proj-" + hex.EncodeToString(sum[:])[:8]
}

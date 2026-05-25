package signalstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// maxSessionIDLen is the maximum number of characters allowed in the
// sanitized session ID (before the ".jsonl" suffix). This is the SSoT
// for the length cap documented in doc.go.
const maxSessionIDLen = 200

// ResolveStorePath returns the directory under which session JSONL files are
// stored. Resolution order:
//
//  1. Value of TONYS_SIGNAL_STORE environment variable, if set and non-empty.
//  2. os.UserCacheDir() + "/tonys-agent-telemetry/signals"
//
// The directory is created (with permission 0700) if it does not exist.
// Signal data may contain sensitive behavioral information; 0700 prevents
// other users on shared systems from reading it.
func ResolveStorePath() (string, error) {
	if override := os.Getenv(defaultStoreEnvVar); override != "" {
		if err := os.MkdirAll(override, 0o700); err != nil {
			return "", err
		}
		return override, nil
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(cacheDir, defaultStoreDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// SessionFilename returns the sanitized filename (including the ".jsonl"
// suffix) for a given sessionID. It does not include the directory component.
//
// Sanitization rules (documented in doc.go):
//   - Empty sessionID returns an error.
//   - All '/' characters are replaced with '_'.
//   - A leading '.' is replaced with '_'.
//   - The sanitized base name is truncated to maxSessionIDLen characters.
func SessionFilename(sessionID string) (string, error) {
	if sessionID == "" {
		return "", errors.New("signalstore: sessionID must not be empty")
	}

	// Replace directory separators.
	safe := strings.ReplaceAll(sessionID, "/", "_")
	// Replace backslashes too (Windows path safety).
	safe = strings.ReplaceAll(safe, `\`, "_")

	// Prevent hidden files.
	if strings.HasPrefix(safe, ".") {
		safe = "_" + safe[1:]
	}

	// Cap length.
	runes := []rune(safe)
	if len(runes) > maxSessionIDLen {
		runes = runes[:maxSessionIDLen]
	}
	safe = string(runes)

	return safe + ".jsonl", nil
}

// sessionFilePath joins the store root with the sanitized session filename.
func sessionFilePath(root, sessionID string) (string, error) {
	name, err := SessionFilename(sessionID)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

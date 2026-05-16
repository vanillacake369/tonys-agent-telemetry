package platform

import (
	"os"
	"path/filepath"
	"time"
)

// FileMtime returns the modification time of the file at path.
// Returns an error if the file does not exist or cannot be statted.
func FileMtime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// HomeDir returns the current user's home directory.
// Falls back to the HOME environment variable if os.UserHomeDir fails.
func HomeDir() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return home
	}
	// Fallback to $HOME
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return ""
}

// ClaudeDir returns the path to the ~/.claude directory.
func ClaudeDir() string {
	return filepath.Join(HomeDir(), ".claude")
}

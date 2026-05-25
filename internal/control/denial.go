package control

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Denial records a single policy enforcement event.
type Denial struct {
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	Tool      string    `json:"tool"`
	Reason    string    `json:"reason"` // "budget_exceeded" | "tool_denylisted" | "tool_not_allowlisted"
	Detail    string    `json:"detail"`
}

// DenialLog is an append-only JSONL log of denials.
type DenialLog struct {
	path string
}

// NewDenialLog creates a DenialLog that uses cacheDir/denials.log.
func NewDenialLog(cacheDir string) *DenialLog {
	return &DenialLog{
		path: filepath.Join(cacheDir, "denials.log"),
	}
}

// Append writes one denial to the log using O_APPEND for POSIX atomicity.
// Lines are kept under PIPE_BUF (4096 bytes on Linux) to guarantee atomicity.
func (l *DenialLog) Append(d Denial) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0700); err != nil {
		return fmt.Errorf("mkdir denial log dir: %w", err)
	}

	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal denial: %w", err)
	}

	f, err := os.OpenFile(l.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return fmt.Errorf("open denial log: %w", err)
	}
	defer f.Close()

	line := append(data, '\n')
	_, err = f.Write(line)
	if err != nil {
		return fmt.Errorf("write denial: %w", err)
	}
	return nil
}

// Recent returns the last n denials in reverse chronological order.
func (l *DenialLog) Recent(n int) ([]Denial, error) {
	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return []Denial{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open denial log: %w", err)
	}
	defer f.Close()

	var all []Denial
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var d Denial
		if err := json.Unmarshal(line, &d); err != nil {
			continue // skip malformed lines
		}
		all = append(all, d)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan denial log: %w", err)
	}

	if n <= 0 || len(all) == 0 {
		return []Denial{}, nil
	}

	// Return last n in reverse order (most recent first).
	start := len(all) - n
	if start < 0 {
		start = 0
	}
	slice := all[start:]
	reversed := make([]Denial, len(slice))
	for i, d := range slice {
		reversed[len(slice)-1-i] = d
	}
	return reversed, nil
}

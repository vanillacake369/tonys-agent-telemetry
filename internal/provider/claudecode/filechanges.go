package claudecode

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

// FileChange represents a single file modification event extracted from a session.
type FileChange struct {
	Path      string
	Operation string // "write", "read", "delete", etc.
	Timestamp string
}

// ParseFileChanges extracts file operation events from a session JSONL file.
func ParseFileChanges(path string) ([]FileChange, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var changes []FileChange
	scanner := newJSONLScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg rawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Type != "assistant" || msg.Message == nil {
			continue
		}
		var blocks []rawContentBlock
		if err := json.Unmarshal(msg.Message.Content, &blocks); err != nil {
			continue
		}
		for _, block := range blocks {
			if block.Type != "tool_use" {
				continue
			}
			switch block.Name {
			case "Write", "Edit", "MultiEdit", "Read":
				var input struct {
					Path string `json:"path"`
				}
				if err := json.Unmarshal(block.Input, &input); err != nil {
					continue
				}
				if input.Path == "" {
					continue
				}
				changes = append(changes, FileChange{
					Path:      input.Path,
					Operation: strings.ToLower(block.Name),
					Timestamp: msg.Timestamp,
				})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("claudecode: scanner error in %s: %v", path, err)
	}

	return changes, nil
}

package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

// rawMessage is the envelope for every JSONL line in a Claude session.
type rawMessage struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionId"`
	CWD       string          `json:"cwd"`
	GitBranch string          `json:"gitBranch"`
	Version   string          `json:"version"`
	UUID      string          `json:"uuid"`
	AgentID   string          `json:"agentId"`
	Operation string          `json:"operation"`
	Content   json.RawMessage `json:"content"`
	Message   *rawInnerMsg    `json:"message"`
}

// rawInnerMsg is the inner message object for user/assistant types.
type rawInnerMsg struct {
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"` // string or []rawContentBlock
	Usage   *rawUsage       `json:"usage"`
}

// rawUsage captures token usage from assistant messages.
type rawUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

const maxFirstPromptLen = 100

// parseTimestamp parses an RFC3339 timestamp string. Returns zero time on error.
func parseTimestamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}

// truncate truncates a UTF-8 string to at most n runes.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n])
}

// SessionHeader holds the metadata extracted from the header of a session file.
// This is claudecode's internal type; data.Session is the TUI view-model.
type SessionHeader struct {
	ID          string
	CWD         string
	GitBranch   string
	FirstPrompt string
	Timestamp   time.Time
	Model       string
	Version     string
	FilePath    string
}

// ParseSessionHeader reads the first few lines of a JSONL file to extract
// session metadata. Corrupted lines are skipped. Missing fields are zero values.
// Returns an error only if the file cannot be opened or contains no usable data.
func ParseSessionHeader(filePath string) (*SessionHeader, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	session := &SessionHeader{FilePath: filePath}
	foundAny := false
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
		if msg.SessionID != "" && session.ID == "" {
			session.ID = msg.SessionID
		}
		if msg.CWD != "" && session.CWD == "" {
			session.CWD = msg.CWD
		}
		if msg.GitBranch != "" && session.GitBranch == "" {
			session.GitBranch = msg.GitBranch
		}
		if msg.Version != "" && session.Version == "" {
			session.Version = msg.Version
		}
		if ts := parseTimestamp(msg.Timestamp); !ts.IsZero() && session.Timestamp.IsZero() {
			session.Timestamp = ts
		}

		if msg.Type == "assistant" && msg.Message != nil {
			if msg.Message.Model != "" && session.Model == "" {
				session.Model = msg.Message.Model
			}
		}

		if msg.Type == "user" && msg.Message != nil && session.FirstPrompt == "" {
			text := extractTextFromContent(msg.Message.Content, false)
			text = strings.ReplaceAll(text, "\n", " ")
			text = strings.ReplaceAll(text, "\r", "")
			if text != "" {
				session.FirstPrompt = truncate(text, maxFirstPromptLen)
			}
		}

		foundAny = true

		if session.ID != "" && session.CWD != "" && session.Model != "" && session.FirstPrompt != "" {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		if !foundAny {
			return nil, err
		}
	}

	if !foundAny {
		return nil, fmt.Errorf("no usable data in %s", filePath)
	}

	return session, nil
}

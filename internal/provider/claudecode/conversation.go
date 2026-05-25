package claudecode

import (
	"encoding/json"
	"log"
	"os"
	"strings"
)

// rawContentBlock is one element of a content array.
type rawContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Thinking string          `json:"thinking"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
}

const (
	maxContentLen        = 300
	thinkingPlaceholder  = "[thinking...]"
)

// extractTextFromContent extracts plain text from a content value that is
// either a plain JSON string or a []rawContentBlock array.
// thinking blocks are replaced with thinkingPlaceholder.
func extractTextFromContent(raw json.RawMessage, skipThinking bool) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []rawContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		case "thinking":
			if !skipThinking && b.Thinking != "" {
				parts = append(parts, b.Thinking)
			} else if skipThinking {
				parts = append(parts, thinkingPlaceholder)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// Turn is a single user or assistant turn for preview.
type Turn struct {
	Role    string
	Content string
}

// ParseConversationPreview extracts the first maxTurns user/assistant turns.
// thinking content blocks are replaced with "[thinking...]".
// Long messages are truncated to 300 characters.
func ParseConversationPreview(path string, maxTurns int) ([]Turn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var turns []Turn
	scanner := newJSONLScanner(f)

	for scanner.Scan() && len(turns) < maxTurns {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg rawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Type != "user" && msg.Type != "assistant" {
			continue
		}
		if msg.Message == nil {
			continue
		}
		text := extractTextFromContent(msg.Message.Content, true)
		if text == "" {
			continue
		}
		turns = append(turns, Turn{
			Role:    msg.Type,
			Content: truncate(text, maxContentLen),
		})
	}
	if err := scanner.Err(); err != nil {
		log.Printf("claudecode: scanner error in %s: %v", path, err)
	}

	return turns, nil
}

// ParseFullConversation extracts all user/assistant turns from a JSONL file.
func ParseFullConversation(path string) ([]Turn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var turns []Turn
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
		if msg.Type != "user" && msg.Type != "assistant" {
			continue
		}
		if msg.Message == nil {
			continue
		}
		text := extractTextFromContent(msg.Message.Content, false)
		if text == "" {
			continue
		}
		turns = append(turns, Turn{
			Role:    msg.Type,
			Content: text,
		})
	}
	if err := scanner.Err(); err != nil {
		log.Printf("claudecode: scanner error in %s: %v", path, err)
	}

	return turns, nil
}

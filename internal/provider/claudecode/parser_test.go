package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockJSONLDir creates a temp directory with a mock session JSONL file.
func mockJSONLDir(t *testing.T, lines []string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test-session.jsonl")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return dir, path
}

func userLine(sessionID, cwd, branch, version, content string, ts time.Time) string {
	msg := map[string]interface{}{
		"type":      "user",
		"timestamp": ts.Format(time.RFC3339Nano),
		"sessionId": sessionID,
		"cwd":       cwd,
		"gitBranch": branch,
		"version":   version,
		"message": map[string]interface{}{
			"role":    "user",
			"content": content,
		},
	}
	b, _ := json.Marshal(msg)
	return string(b)
}

func assistantLine(sessionID, model string, ts time.Time) string {
	msg := map[string]interface{}{
		"type":      "assistant",
		"timestamp": ts.Format(time.RFC3339Nano),
		"sessionId": sessionID,
		"message": map[string]interface{}{
			"role":  "assistant",
			"model": model,
			"content": []map[string]interface{}{
				{"type": "text", "text": "This is an assistant response."},
			},
		},
	}
	b, _ := json.Marshal(msg)
	return string(b)
}

func assistantThinkingLine(sessionID string, ts time.Time) string {
	msg := map[string]interface{}{
		"type":      "assistant",
		"timestamp": ts.Format(time.RFC3339Nano),
		"sessionId": sessionID,
		"message": map[string]interface{}{
			"role":  "assistant",
			"model": "claude-opus-4-6",
			"content": []map[string]interface{}{
				{"type": "thinking", "thinking": "Let me think about this carefully..."},
				{"type": "text", "text": "Here is my answer."},
			},
		},
	}
	b, _ := json.Marshal(msg)
	return string(b)
}

// --- ParseSessionHeader tests ---

func TestParseSessionHeader_ValidJSONL(t *testing.T) {
	ts := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)
	lines := []string{
		userLine("session-abc", "/home/user/project", "main", "2.1.81", "Hello world", ts),
		assistantLine("session-abc", "claude-opus-4-6", ts.Add(time.Second)),
	}
	_, path := mockJSONLDir(t, lines)

	s, err := ParseSessionHeader(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "session-abc" {
		t.Errorf("ID = %q, want %q", s.ID, "session-abc")
	}
	if s.CWD != "/home/user/project" {
		t.Errorf("CWD = %q, want %q", s.CWD, "/home/user/project")
	}
	if s.GitBranch != "main" {
		t.Errorf("GitBranch = %q, want %q", s.GitBranch, "main")
	}
	if s.Version != "2.1.81" {
		t.Errorf("Version = %q, want %q", s.Version, "2.1.81")
	}
	if s.FirstPrompt != "Hello world" {
		t.Errorf("FirstPrompt = %q, want %q", s.FirstPrompt, "Hello world")
	}
	if s.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", s.Model, "claude-opus-4-6")
	}
	if !s.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", s.Timestamp, ts)
	}
	if s.FilePath != path {
		t.Errorf("FilePath = %q, want %q", s.FilePath, path)
	}
}

func TestParseSessionHeader_CorruptedLine(t *testing.T) {
	ts := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)
	lines := []string{
		"THIS IS NOT JSON {{{{",
		userLine("session-xyz", "/tmp/proj", "feature", "2.1.0", "A prompt", ts),
	}
	_, path := mockJSONLDir(t, lines)

	s, err := ParseSessionHeader(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "session-xyz" {
		t.Errorf("ID = %q, want %q", s.ID, "session-xyz")
	}
}

func TestParseSessionHeader_EmptyFile(t *testing.T) {
	_, path := mockJSONLDir(t, []string{})
	_, err := ParseSessionHeader(path)
	if err == nil {
		t.Error("expected error for empty file, got nil")
	}
}

// --- ParseConversationPreview tests ---

func TestParseConversationPreview_ThinkingBlocksSkipped(t *testing.T) {
	ts := time.Now()
	lines := []string{
		assistantThinkingLine("s1", ts),
	}
	_, path := mockJSONLDir(t, lines)

	turns, err := ParseConversationPreview(path, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(turns) == 0 {
		t.Fatal("expected at least one turn")
	}
	for _, turn := range turns {
		if turn.Role == "assistant" {
			if turn.Content == "Let me think about this carefully..." {
				t.Error("thinking content should not appear verbatim — should be placeholder")
			}
		}
	}
}

func TestParseConversationPreview_MaxTurnsRespected(t *testing.T) {
	ts := time.Now()
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, userLine("s1", "/tmp", "main", "1.0", "message", ts.Add(time.Duration(i)*time.Second)))
	}
	_, path := mockJSONLDir(t, lines)

	turns, err := ParseConversationPreview(path, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(turns) != 3 {
		t.Errorf("expected exactly 3 turns, got %d", len(turns))
	}
}

func TestParseConversationPreview_EmptyFile(t *testing.T) {
	_, path := mockJSONLDir(t, []string{})
	turns, err := ParseConversationPreview(path, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(turns) != 0 {
		t.Errorf("expected 0 turns for empty file, got %d", len(turns))
	}
}

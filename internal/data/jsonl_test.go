package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockJSONLDir creates a temp directory with a mock session JSONL file.
// Returns the dir path and a cleanup function.
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

// userLine returns a minimal user JSONL line.
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

// assistantLine returns a minimal assistant JSONL line with model info.
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

// assistantThinkingLine returns an assistant message with a thinking block + text block.
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

// agentToolUseLine returns an assistant message with an Agent tool_use block.
func agentToolUseLine(sessionID, subagentType, description string, ts time.Time) string {
	inputBytes, _ := json.Marshal(map[string]interface{}{
		"subagent_type":     subagentType,
		"description":       description,
		"run_in_background": true,
	})
	msg := map[string]interface{}{
		"type":      "assistant",
		"timestamp": ts.Format(time.RFC3339Nano),
		"sessionId": sessionID,
		"message": map[string]interface{}{
			"role":  "assistant",
			"model": "claude-opus-4-6",
			"content": []map[string]interface{}{
				{
					"type":  "tool_use",
					"id":    "toolu_test123",
					"name":  "Agent",
					"input": json.RawMessage(inputBytes),
				},
			},
		},
	}
	b, _ := json.Marshal(msg)
	return string(b)
}

// queueOperationLine returns a queue-operation enqueue event.
func queueOperationLine(sessionID, taskID, status string, ts time.Time) string {
	content := "<task-notification><task-id>" + taskID + "</task-id><status>" + status + "</status></task-notification>"
	msg := map[string]interface{}{
		"type":      "queue-operation",
		"operation": "enqueue",
		"timestamp": ts.Format(time.RFC3339Nano),
		"sessionId": sessionID,
		"content":   content,
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

func TestParseSessionHeader_FirstPromptTruncated(t *testing.T) {
	ts := time.Now()
	longPrompt := string(make([]byte, 200))
	for i := range longPrompt {
		longPrompt = longPrompt[:i] + "a" + longPrompt[i+1:]
	}
	// Build a 200-char prompt
	longPrompt = ""
	for i := 0; i < 200; i++ {
		longPrompt += "x"
	}
	lines := []string{
		userLine("s1", "/tmp", "main", "1.0", longPrompt, ts),
	}
	_, path := mockJSONLDir(t, lines)
	s, err := ParseSessionHeader(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len([]rune(s.FirstPrompt)) > 100 {
		t.Errorf("FirstPrompt length = %d, want <= 100", len([]rune(s.FirstPrompt)))
	}
}

func TestParseSessionHeader_MissingFields(t *testing.T) {
	// A line with no sessionId, no cwd, etc.
	lines := []string{`{"type":"file-history-snapshot","timestamp":"2026-05-16T07:00:00Z"}`}
	_, path := mockJSONLDir(t, lines)

	s, err := ParseSessionHeader(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.ID != "" {
		t.Errorf("expected empty ID for missing field, got %q", s.ID)
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

func TestParseConversationPreview_SkipsNonConversation(t *testing.T) {
	ts := time.Now()
	lines := []string{
		`{"type":"file-history-snapshot","timestamp":"2026-05-16T07:00:00Z"}`,
		`{"type":"system","timestamp":"2026-05-16T07:00:01Z","subtype":"stop_hook_summary"}`,
		userLine("s1", "/tmp", "main", "1.0", "real user message", ts),
	}
	_, path := mockJSONLDir(t, lines)
	turns, err := ParseConversationPreview(path, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(turns) != 1 {
		t.Errorf("expected 1 turn (skipping non-conversation), got %d", len(turns))
	}
	if turns[0].Content != "real user message" {
		t.Errorf("content = %q, want %q", turns[0].Content, "real user message")
	}
}

// --- ParseDAG tests ---

// mockSessionDir creates a temp dir structure for ParseDAG tests.
// Returns the session directory path containing the JSONL file.
func mockSessionDir(t *testing.T, sessionID string, mainLines []string, subagents map[string]struct {
	metaAgentType   string
	metaDescription string
	status          string
}) string {
	t.Helper()
	dir := t.TempDir()

	// Write main JSONL.
	mainPath := filepath.Join(dir, sessionID+".jsonl")
	content := ""
	for _, l := range mainLines {
		content += l + "\n"
	}
	if err := os.WriteFile(mainPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile main JSONL: %v", err)
	}

	// Write subagent files if any.
	if len(subagents) > 0 {
		subDir := filepath.Join(dir, sessionID, "subagents")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("MkdirAll subagents: %v", err)
		}
		for agentID, info := range subagents {
			meta := map[string]string{
				"agentType":   info.metaAgentType,
				"description": info.metaDescription,
			}
			b, _ := json.Marshal(meta)
			metaPath := filepath.Join(subDir, "agent-"+agentID+".meta.json")
			if err := os.WriteFile(metaPath, b, 0644); err != nil {
				t.Fatalf("WriteFile meta: %v", err)
			}
			// Write empty subagent JSONL.
			jsonlPath := filepath.Join(subDir, "agent-"+agentID+".jsonl")
			if err := os.WriteFile(jsonlPath, []byte{}, 0644); err != nil {
				t.Fatalf("WriteFile subagent jsonl: %v", err)
			}
		}
	}

	return dir
}

func TestParseDAG_NoSubagents(t *testing.T) {
	ts := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)
	sessionID := "session-solo"
	lines := []string{
		userLine(sessionID, "/tmp", "main", "1.0", "prompt", ts),
		assistantLine(sessionID, "claude-opus-4-6", ts.Add(time.Second)),
	}
	dir := mockSessionDir(t, sessionID, lines, nil)

	root, err := ParseDAG(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root == nil {
		t.Fatal("expected non-nil root")
	}
	if len(root.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(root.Children))
	}
}

func TestParseDAG_WithSubagents(t *testing.T) {
	ts := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)
	sessionID := "session-multi"
	lines := []string{
		userLine(sessionID, "/tmp", "main", "1.0", "orchestrate", ts),
		agentToolUseLine(sessionID, "implementer", "Implement the feature", ts.Add(time.Second)),
		agentToolUseLine(sessionID, "researcher", "Research the topic", ts.Add(2*time.Second)),
		queueOperationLine(sessionID, "agent-id-001", "completed", ts.Add(3*time.Second)),
	}
	subagents := map[string]struct {
		metaAgentType   string
		metaDescription string
		status          string
	}{
		"agent-id-001": {metaAgentType: "implementer", metaDescription: "Implement the feature", status: "completed"},
	}
	dir := mockSessionDir(t, sessionID, lines, subagents)

	root, err := ParseDAG(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root == nil {
		t.Fatal("expected non-nil root")
	}
	if len(root.Children) == 0 {
		t.Error("expected at least one child node from Agent tool_use events")
	}

	// At least one child should have agentType set.
	foundImplementer := false
	for _, child := range root.Children {
		if child.AgentType == "implementer" {
			foundImplementer = true
		}
	}
	if !foundImplementer {
		t.Error("expected a child with AgentType=implementer")
	}
}

func TestParseDAG_CorruptedLines(t *testing.T) {
	ts := time.Now()
	sessionID := "session-corrupt"
	lines := []string{
		"NOT JSON AT ALL",
		userLine(sessionID, "/tmp", "main", "1.0", "prompt", ts),
		"ALSO BAD JSON",
	}
	dir := mockSessionDir(t, sessionID, lines, nil)

	root, err := ParseDAG(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if root == nil {
		t.Fatal("expected non-nil root")
	}
}

// --- Real data integration tests ---

func TestParseSessionHeader_RealFile(t *testing.T) {
	// Uses the fixture file checked into the repo.
	path := filepath.Join(projectRoot(t), "test", "testdata", "mock-session.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture not found: %v", err)
	}
	s, err := ParseSessionHeader(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil session")
	}
	// The fixture should have a non-empty session ID.
	if s.ID == "" {
		t.Log("warning: no sessionId found in fixture (may be expected for this file)")
	}
}

// projectRoot walks up from the test file's directory to find the module root.
func projectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod")
		}
		dir = parent
	}
}

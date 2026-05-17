package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexParseSessionHeader(t *testing.T) {
	// Create a temporary JSONL file with Codex format.
	dir := t.TempDir()
	content := `{"timestamp":"2026-05-14T06:50:14.944Z","type":"session_meta","payload":{"id":"test-session-123","timestamp":"2026-05-14T06:49:06.425Z","cwd":"/Users/test/dev/project","originator":"codex_cli_rs","cli_version":"0.116.0","source":"cli","model_provider":"openai","git":{"branch":"main"}}}
{"timestamp":"2026-05-14T06:50:15.000Z","type":"event_msg","payload":{"type":"user_message","message":"fix the login bug"}}
{"timestamp":"2026-05-14T06:50:20.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":200,"output_tokens":500,"reasoning_output_tokens":50,"total_tokens":1500}}}}
{"timestamp":"2026-05-14T06:51:00.000Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}
`
	filePath := filepath.Join(dir, "rollout-test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p := &CodexProvider{}

	// Test session header parsing.
	session, err := p.parseSessionHeader(filePath)
	if err != nil {
		t.Fatalf("parseSessionHeader error: %v", err)
	}
	if session.ID != "test-session-123" {
		t.Errorf("expected ID test-session-123, got %s", session.ID)
	}
	if session.CWD != "/Users/test/dev/project" {
		t.Errorf("expected CWD /Users/test/dev/project, got %s", session.CWD)
	}
	if session.GitBranch != "main" {
		t.Errorf("expected branch main, got %s", session.GitBranch)
	}
	if session.Provider != ProviderCodex {
		t.Errorf("expected provider codex, got %s", session.Provider)
	}
	if session.TurnCount != 1 {
		t.Errorf("expected 1 turn, got %d", session.TurnCount)
	}
	if session.FirstPrompt != "fix the login bug" {
		t.Errorf("expected 'fix the login bug', got %q", session.FirstPrompt)
	}

	// Test cost parsing.
	cost, err := p.parseSessionCost(filePath)
	if err != nil {
		t.Fatalf("parseSessionCost error: %v", err)
	}
	if cost.InputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", cost.InputTokens)
	}
	if cost.OutputTokens != 500 {
		t.Errorf("expected 500 output tokens, got %d", cost.OutputTokens)
	}
	if cost.CacheRead != 200 {
		t.Errorf("expected 200 cache read, got %d", cost.CacheRead)
	}
	if cost.TurnCount != 1 {
		t.Errorf("expected 1 turn, got %d", cost.TurnCount)
	}
	if cost.Provider != ProviderCodex {
		t.Errorf("expected provider codex, got %s", cost.Provider)
	}
}

func TestCodexConversationPreview(t *testing.T) {
	dir := t.TempDir()
	content := `{"timestamp":"2026-05-14T06:50:14.944Z","type":"session_meta","payload":{"id":"test-123"}}
{"timestamp":"2026-05-14T06:50:15.000Z","type":"event_msg","payload":{"type":"user_message","message":"hello world"}}
{"timestamp":"2026-05-14T06:50:16.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hi there!"}]}}
{"timestamp":"2026-05-14T06:50:17.000Z","type":"event_msg","payload":{"type":"user_message","message":"thanks"}}
`
	filePath := filepath.Join(dir, "test.jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p := &CodexProvider{}
	turns, err := p.ParseConversationPreview(filePath, 5)
	if err != nil {
		t.Fatalf("ParseConversationPreview error: %v", err)
	}
	if len(turns) != 3 {
		t.Fatalf("expected 3 turns, got %d", len(turns))
	}
	if turns[0].Role != "user" || turns[0].Content != "hello world" {
		t.Errorf("turn 0: got role=%s content=%q", turns[0].Role, turns[0].Content)
	}
	if turns[1].Role != "assistant" || turns[1].Content != "Hi there!" {
		t.Errorf("turn 1: got role=%s content=%q", turns[1].Role, turns[1].Content)
	}
	if turns[2].Role != "user" || turns[2].Content != "thanks" {
		t.Errorf("turn 2: got role=%s content=%q", turns[2].Role, turns[2].Content)
	}
}

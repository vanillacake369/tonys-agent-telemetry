package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeminiParseLogsFile(t *testing.T) {
	dir := t.TempDir()
	content := `[
  {"sessionId":"sess-1","messageId":0,"type":"user","message":"hello gemini","timestamp":"2026-05-10T10:00:00.000Z"},
  {"sessionId":"sess-1","messageId":1,"type":"user","message":"how are you","timestamp":"2026-05-10T10:05:00.000Z"},
  {"sessionId":"sess-2","messageId":0,"type":"user","message":"new session","timestamp":"2026-05-11T09:00:00.000Z"}
]`
	logsPath := filepath.Join(dir, "logs.json")
	if err := os.WriteFile(logsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p := &GeminiProvider{}
	projectPaths := map[string]string{"testproj": "/Users/test/dev/testproj"}
	sessions, err := p.parseLogsFile(logsPath, "testproj", projectPaths)
	if err != nil {
		t.Fatalf("parseLogsFile error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	// First session (sess-1) has 2 turns.
	s1 := sessions[0]
	if s1.ID != "sess-1" {
		t.Errorf("expected sess-1, got %s", s1.ID)
	}
	if s1.TurnCount != 2 {
		t.Errorf("expected 2 turns, got %d", s1.TurnCount)
	}
	if s1.Provider != ProviderGemini {
		t.Errorf("expected gemini provider, got %s", s1.Provider)
	}
	if s1.CWD != "/Users/test/dev/testproj" {
		t.Errorf("expected CWD /Users/test/dev/testproj, got %s", s1.CWD)
	}
	if s1.FirstPrompt != "hello gemini" {
		t.Errorf("expected 'hello gemini', got %q", s1.FirstPrompt)
	}

	// Second session (sess-2) has 1 turn.
	s2 := sessions[1]
	if s2.ID != "sess-2" {
		t.Errorf("expected sess-2, got %s", s2.ID)
	}
	if s2.TurnCount != 1 {
		t.Errorf("expected 1 turn, got %d", s2.TurnCount)
	}
}

func TestGeminiConversationPreview(t *testing.T) {
	dir := t.TempDir()
	content := `[
  {"sessionId":"sess-1","messageId":0,"type":"user","message":"first message","timestamp":"2026-05-10T10:00:00.000Z"},
  {"sessionId":"sess-1","messageId":1,"type":"user","message":"second message","timestamp":"2026-05-10T10:01:00.000Z"},
  {"sessionId":"sess-2","messageId":0,"type":"user","message":"other session","timestamp":"2026-05-10T10:02:00.000Z"},
  {"sessionId":"sess-1","messageId":2,"type":"user","message":"third message","timestamp":"2026-05-10T10:03:00.000Z"}
]`
	logsPath := filepath.Join(dir, "logs.json")
	if err := os.WriteFile(logsPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	p := &GeminiProvider{}
	// Use encoded path with session ID — should only show sess-1 messages.
	encoded := encodeGeminiFilePath(logsPath, "sess-1")
	turns, err := p.ParseConversationPreview(encoded, 2)
	if err != nil {
		t.Fatalf("ParseConversationPreview error: %v", err)
	}
	if len(turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(turns))
	}
	// Should be the first 2 messages from sess-1 only.
	if turns[0].Content != "first message" {
		t.Errorf("expected 'first message', got %q", turns[0].Content)
	}
	if turns[1].Content != "second message" {
		t.Errorf("expected 'second message', got %q", turns[1].Content)
	}
}

func TestGeminiAvailable(t *testing.T) {
	p := &GeminiProvider{}
	// Should check for tmp directory
	_ = p.Available() // just ensure no panic
}

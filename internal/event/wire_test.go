package event

import (
	"testing"
)

func TestParseLine_V1_StillSupported(t *testing.T) {
	line := `PostToolUse	{"tool":"Bash","sessionId":"abc"}`
	ev, err := parseLine(line)
	if err != nil {
		t.Fatalf("parseLine v1: unexpected error: %v", err)
	}
	if ev.HookType != "PostToolUse" {
		t.Errorf("HookType = %q, want %q", ev.HookType, "PostToolUse")
	}
	if string(ev.Payload) != `{"tool":"Bash","sessionId":"abc"}` {
		t.Errorf("Payload = %q, want raw JSON", string(ev.Payload))
	}
}

func TestParseLine_V2_NewFormat(t *testing.T) {
	line := `{"v":2,"trace_id":"t1","span_id":"s1","system":"anthropic","model":"claude-sonnet-4-6","status":"done","start_time":"2026-05-25T10:00:00Z","end_time":"2026-05-25T10:00:02Z","attrs":{"claudecode.hook_type":"PostToolUse"}}`
	ev, err := parseLine(line)
	if err != nil {
		t.Fatalf("parseLine v2: unexpected error: %v", err)
	}
	if ev.HookType != V2SpanHookType {
		t.Errorf("HookType = %q, want %q", ev.HookType, V2SpanHookType)
	}
	if string(ev.Payload) != line {
		t.Errorf("Payload should be the raw v2 JSON line")
	}
}

func TestParseLine_Malformed_LogsAndSkips(t *testing.T) {
	// Garbage line without tab separator and not valid v2 JSON should return error.
	line := `this is garbage and not valid JSON or tab-separated`
	_, err := parseLine(line)
	if err == nil {
		t.Error("expected error for malformed line, got nil")
	}
}

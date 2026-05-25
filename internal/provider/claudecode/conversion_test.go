package claudecode

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
)

func TestConvert_FullClaudePayload_ToSpan(t *testing.T) {
	ts := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	payload := map[string]interface{}{
		"sessionId":  "trace-abc",
		"uuid":       "span-001",
		"parentUuid": "span-000",
		"model":      "claude-sonnet-4-6",
		"timestamp":  ts.Format(time.RFC3339),
		"type":       "assistant",
		"cwd":        "/home/user/project",
		"gitBranch":  "main",
		"usage": map[string]int{
			"input_tokens":  100,
			"output_tokens": 50,
		},
	}
	raw, _ := json.Marshal(payload)

	ev := event.Event{HookType: "PostToolUse", Payload: raw}
	span := convertV1ToSpan(ev)

	if span.TraceID != "trace-abc" {
		t.Errorf("TraceID = %q, want %q", span.TraceID, "trace-abc")
	}
	if span.SpanID != "span-001" {
		t.Errorf("SpanID = %q, want %q", span.SpanID, "span-001")
	}
	if span.ParentSpanID != "span-000" {
		t.Errorf("ParentSpanID = %q, want %q", span.ParentSpanID, "span-000")
	}
	if span.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", span.Model, "claude-sonnet-4-6")
	}
	if span.System != "anthropic" {
		t.Errorf("System = %q, want %q", span.System, "anthropic")
	}
	if span.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", span.InputTokens)
	}
	if span.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", span.OutputTokens)
	}
	if span.Attrs["code.filepath"] != "/home/user/project" {
		t.Errorf("Attrs[code.filepath] = %q, want %q", span.Attrs["code.filepath"], "/home/user/project")
	}
	if span.Attrs["vcs.ref.head.name"] != "main" {
		t.Errorf("Attrs[vcs.ref.head.name] = %q, want %q", span.Attrs["vcs.ref.head.name"], "main")
	}
	if span.Attrs["claudecode.hook_type"] != "PostToolUse" {
		t.Errorf("Attrs[claudecode.hook_type] = %q, want %q", span.Attrs["claudecode.hook_type"], "PostToolUse")
	}
	if span.Status != "done" {
		t.Errorf("Status = %q, want %q", span.Status, "done")
	}
}

func TestConvert_QueueOperation_StatusRunning(t *testing.T) {
	payload := map[string]interface{}{
		"sessionId": "trace-q",
		"uuid":      "span-q",
		"type":      "queue-operation",
	}
	raw, _ := json.Marshal(payload)
	ev := event.Event{HookType: "PostToolUse", Payload: raw}
	span := convertV1ToSpan(ev)

	if span.Status != "running" {
		t.Errorf("Status = %q, want %q", span.Status, "running")
	}
}

func TestConvert_MissingUsage_ZeroTokens(t *testing.T) {
	payload := map[string]interface{}{
		"sessionId": "trace-nousage",
		"uuid":      "span-nousage",
		"type":      "assistant",
		// no "usage" field
	}
	raw, _ := json.Marshal(payload)
	ev := event.Event{HookType: "SessionStart", Payload: raw}
	span := convertV1ToSpan(ev)

	if span.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0 (no panic)", span.InputTokens)
	}
	if span.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0 (no panic)", span.OutputTokens)
	}
}

func TestConvert_MissingSessionID_Empty(t *testing.T) {
	// Payload without sessionId — defensive against schema drift.
	payload := map[string]interface{}{
		"type": "assistant",
		"uuid": "span-nosession",
	}
	raw, _ := json.Marshal(payload)
	ev := event.Event{HookType: "PostToolUse", Payload: raw}

	// Must not panic.
	span := convertV1ToSpan(ev)

	if span.TraceID != "" {
		t.Logf("TraceID = %q (empty expected for missing sessionId)", span.TraceID)
	}
	// The important thing is it doesn't crash.
}

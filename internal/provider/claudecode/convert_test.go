package claudecode

import (
	"testing"
	"time"
)

func TestConvertHookPayload_FullAssistantTurn(t *testing.T) {
	raw := []byte(`{
		"sessionId":"sess-abc123",
		"uuid":"msg-1",
		"parentUuid":"msg-0",
		"type":"assistant",
		"cwd":"/home/user/project",
		"gitBranch":"main",
		"timestamp":"2026-05-25T14:30:00Z",
		"message":{"model":"claude-sonnet-4-6","usage":{"input_tokens":1500,"output_tokens":420}}
	}`)
	span, err := ConvertHookPayload("PostToolUse", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if span.TraceID != "sess-abc123" {
		t.Errorf("TraceID = %q, want sess-abc123", span.TraceID)
	}
	if span.SpanID != "msg-1" || span.ParentSpanID != "msg-0" {
		t.Errorf("SpanID/ParentSpanID = %q/%q, want msg-1/msg-0", span.SpanID, span.ParentSpanID)
	}
	if span.System != "anthropic" {
		t.Errorf("System = %q, want anthropic", span.System)
	}
	if span.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q", span.Model)
	}
	if span.InputTokens != 1500 || span.OutputTokens != 420 {
		t.Errorf("tokens = %d/%d, want 1500/420", span.InputTokens, span.OutputTokens)
	}
	if span.Status != "done" {
		t.Errorf("Status = %q, want done", span.Status)
	}
	if span.Attrs["code.filepath"] != "/home/user/project" {
		t.Errorf("code.filepath attr = %q", span.Attrs["code.filepath"])
	}
	if span.Attrs["vcs.ref.head.name"] != "main" {
		t.Errorf("vcs.ref attr = %q", span.Attrs["vcs.ref.head.name"])
	}
	if span.Attrs["claudecode.hook_type"] != "PostToolUse" {
		t.Errorf("hook_type attr = %q", span.Attrs["claudecode.hook_type"])
	}
	want := time.Date(2026, 5, 25, 14, 30, 0, 0, time.UTC)
	if !span.StartTime.Equal(want) {
		t.Errorf("StartTime = %v, want %v", span.StartTime, want)
	}
}

func TestConvertHookPayload_QueueOperationIsRunning(t *testing.T) {
	raw := []byte(`{"sessionId":"s","uuid":"u","type":"queue-operation"}`)
	span, _ := ConvertHookPayload("PreToolUse", raw)
	if span.Status != "running" {
		t.Errorf("Status = %q, want running", span.Status)
	}
}

func TestConvertHookPayload_MissingUsageZeroTokens(t *testing.T) {
	raw := []byte(`{"sessionId":"s","uuid":"u","type":"assistant","message":{"model":"m"}}`)
	span, err := ConvertHookPayload("", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if span.InputTokens != 0 || span.OutputTokens != 0 {
		t.Errorf("tokens = %d/%d, want 0/0", span.InputTokens, span.OutputTokens)
	}
	if span.Model != "m" {
		t.Errorf("Model = %q", span.Model)
	}
}

func TestConvertHookPayload_MalformedJSONReturnsError(t *testing.T) {
	_, err := ConvertHookPayload("Hook", []byte("not json"))
	if err == nil {
		t.Error("want error for malformed JSON, got nil")
	}
}

func TestConvertHookPayload_ToolNameAttr(t *testing.T) {
	raw := []byte(`{"sessionId":"s","uuid":"u","tool_name":"Bash","type":"queue-operation"}`)
	span, _ := ConvertHookPayload("PreToolUse", raw)
	if span.Attrs["gen_ai.tool.name"] != "Bash" {
		t.Errorf("tool.name attr = %q, want Bash", span.Attrs["gen_ai.tool.name"])
	}
}

func TestConvertHookPayload_EmptyTimestampLeavesTimesZero(t *testing.T) {
	raw := []byte(`{"sessionId":"s","uuid":"u","type":"assistant"}`)
	span, _ := ConvertHookPayload("", raw)
	if !span.StartTime.IsZero() || !span.EndTime.IsZero() {
		t.Errorf("times should be zero when timestamp missing")
	}
}

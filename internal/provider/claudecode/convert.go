package claudecode

import (
	"encoding/json"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// hookPayload mirrors the subset of fields we care about from a Claude Code
// hook stdin payload. Fields use Claude's actual JSON keys (camelCase mix).
type hookPayload struct {
	SessionID  string          `json:"sessionId"`
	UUID       string          `json:"uuid"`
	ParentUUID string          `json:"parentUuid"`
	Type       string          `json:"type"` // "user" | "assistant" | "queue-operation"
	CWD        string          `json:"cwd"`
	GitBranch  string          `json:"gitBranch"`
	Timestamp  string          `json:"timestamp"`
	Message    *hookInnerMsg   `json:"message,omitempty"`
	ToolName   string          `json:"tool_name,omitempty"`
	ToolInput  json.RawMessage `json:"tool_input,omitempty"`
}

type hookInnerMsg struct {
	Model string     `json:"model"`
	Usage *hookUsage `json:"usage,omitempty"`
}

type hookUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ConvertHookPayload translates a raw Claude Code hook JSON payload into a
// telemetry.Span. hookType is the Claude event name (e.g. "PreToolUse",
// "PostToolUse", "SessionStart"). Missing fields produce zero values rather
// than errors — Claude payloads vary across hook types and versions.
//
// Returns an error only when the input is not valid JSON. Schema drift in
// individual fields is tolerated silently.
func ConvertHookPayload(hookType string, raw []byte) (telemetry.Span, error) {
	var p hookPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return telemetry.Span{}, err
	}

	attrs := map[string]string{
		"gen_ai.operation.name": "chat",
	}
	if hookType != "" {
		attrs["claudecode.hook_type"] = hookType
	}
	if p.CWD != "" {
		attrs["code.filepath"] = p.CWD
	}
	if p.GitBranch != "" {
		attrs["vcs.ref.head.name"] = p.GitBranch
	}
	if p.ToolName != "" {
		attrs["gen_ai.tool.name"] = p.ToolName
	}

	status := "done"
	if p.Type == "queue-operation" {
		status = "running"
	}

	span := telemetry.Span{
		TraceID:      p.SessionID,
		SpanID:       p.UUID,
		ParentSpanID: p.ParentUUID,
		System:       "anthropic",
		Status:       status,
		Attrs:        attrs,
	}

	if p.Message != nil {
		span.Model = p.Message.Model
		if p.Message.Usage != nil {
			span.InputTokens = p.Message.Usage.InputTokens
			span.OutputTokens = p.Message.Usage.OutputTokens
		}
	}

	if p.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339Nano, p.Timestamp); err == nil {
			span.StartTime = t
			span.EndTime = t // hook fires at one point in time
		}
	}

	return span, nil
}

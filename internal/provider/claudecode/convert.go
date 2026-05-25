package claudecode

import (
	"encoding/json"
	"log"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// v2SpanWire is the v2 FIFO wire format.
type v2SpanWire struct {
	V            int               `json:"v"`
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id"`
	System       string            `json:"system"`
	Model        string            `json:"model"`
	InputTokens  int               `json:"input_tokens"`
	OutputTokens int               `json:"output_tokens"`
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time"`
	Status       string            `json:"status"`
	Attrs        map[string]string `json:"attrs"`
}

// v1HookPayload is a representative subset of the v1 Claude hook payload.
// Fields absent in the payload are zero values (no panic).
type v1HookPayload struct {
	SessionID   string    `json:"sessionId"`
	UUID        string    `json:"uuid"`
	ParentUUID  string    `json:"parentUuid"`
	Model       string    `json:"model"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	CWD         string    `json:"cwd"`
	GitBranch   string    `json:"gitBranch"`
	Usage       *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// sessionMetaToSpan converts a back-filled SessionMeta to a telemetry.Span.
func sessionMetaToSpan(s SessionMeta) telemetry.Span {
	attrs := make(map[string]string)
	if s.CWD != "" {
		attrs["code.filepath"] = s.CWD
	}
	if s.GitBranch != "" {
		attrs["vcs.ref.head.name"] = s.GitBranch
	}
	if s.FirstPrompt != "" {
		attrs["gen_ai.request.content"] = s.FirstPrompt
	}
	return telemetry.Span{
		TraceID:   s.ID,
		SpanID:    s.ID,
		System:    "anthropic",
		Model:     s.Model,
		StartTime: s.Timestamp,
		EndTime:   s.Timestamp,
		Status:    "done",
		Attrs:     attrs,
	}
}

// eventToSpan converts an event.Event (v1 or v2) to a telemetry.Span.
func eventToSpan(ev event.Event) telemetry.Span {
	if ev.HookType == event.V2SpanHookType {
		return parseV2Span(ev.Payload)
	}
	return convertV1ToSpan(ev)
}

// parseV2Span unmarshals a v2 wire format JSON into a telemetry.Span.
func parseV2Span(raw json.RawMessage) telemetry.Span {
	var w v2SpanWire
	if err := json.Unmarshal(raw, &w); err != nil {
		log.Printf("claudecode: failed to parse v2 span: %v", err)
		return telemetry.Span{Status: "error"}
	}
	return telemetry.Span{
		TraceID:      w.TraceID,
		SpanID:       w.SpanID,
		ParentSpanID: w.ParentSpanID,
		System:       w.System,
		Model:        w.Model,
		InputTokens:  w.InputTokens,
		OutputTokens: w.OutputTokens,
		StartTime:    w.StartTime,
		EndTime:      w.EndTime,
		Status:       w.Status,
		Attrs:        w.Attrs,
	}
}

// convertV1ToSpan converts a v1 Claude hook Event to a telemetry.Span.
// Fields absent in the payload are zero values.
func convertV1ToSpan(ev event.Event) telemetry.Span {
	var p v1HookPayload
	if err := json.Unmarshal(ev.Payload, &p); err != nil {
		log.Printf("claudecode: failed to parse v1 payload for hook %s: %v", ev.HookType, err)
		return telemetry.Span{Status: "error"}
	}

	status := "done"
	if p.Type == "queue-operation" {
		status = "running"
	}

	attrs := make(map[string]string)
	if ev.HookType != "" {
		attrs["claudecode.hook_type"] = ev.HookType
	}
	if p.CWD != "" {
		attrs["code.filepath"] = p.CWD
	}
	if p.GitBranch != "" {
		attrs["vcs.ref.head.name"] = p.GitBranch
	}
	attrs["gen_ai.operation.name"] = "chat"

	inputTokens, outputTokens := 0, 0
	if p.Usage != nil {
		inputTokens = p.Usage.InputTokens
		outputTokens = p.Usage.OutputTokens
	}

	return telemetry.Span{
		TraceID:      p.SessionID,
		SpanID:       p.UUID,
		ParentSpanID: p.ParentUUID,
		System:       "anthropic",
		Model:        p.Model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		StartTime:    p.Timestamp,
		EndTime:      p.Timestamp,
		Status:       status,
		Attrs:        attrs,
	}
}

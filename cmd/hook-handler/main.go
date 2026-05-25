package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
)

// debug logs a message to stderr when TAT_DEBUG=1 is set.
func debug(format string, args ...any) {
	if os.Getenv("TAT_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[tat-hook] "+format+"\n", args...)
	}
}

// v2SpanWire is the v2 wire format written to the FIFO.
type v2SpanWire struct {
	V        int               `json:"v"`
	TraceID  string            `json:"trace_id"`
	SpanID   string            `json:"span_id"`
	System   string            `json:"system"`
	Model    string            `json:"model"`
	Status   string            `json:"status"`
	StartTime time.Time        `json:"start_time"`
	EndTime  time.Time         `json:"end_time"`
	Attrs    map[string]string `json:"attrs"`
}

// rawHookPayload holds the fields we extract from Claude hook stdin JSON.
type rawHookPayload struct {
	SessionID string `json:"sessionId"`
	UUID      string `json:"uuid"`
	Model     string `json:"model"`
	CWD       string `json:"cwd"`
	GitBranch string `json:"gitBranch"`
	Type      string `json:"type"`
}

func main() {
	// Hook handler must ALWAYS exit 0 to never block Claude Code.
	defer os.Exit(0)

	// m3: set umask to 0077 before any file creation so the FIFO is not group/world accessible.
	syscall.Umask(0077)

	hookType := ""
	if len(os.Args) > 1 {
		hookType = os.Args[1]
	}

	debug("invoked with hookType=%q", hookType)

	// m1: limit stdin to 1 MiB to prevent OOM from oversized payloads.
	payload, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		debug("failed to read stdin: %v", err)
		return
	}

	debug("payload=%d bytes", len(payload))

	// Convert to v2 span format and write as a raw JSON line.
	// If conversion fails, fall back to v1 tab-separated format.
	wireBytes, isV2 := toV2Wire(hookType, payload)

	var writeErr error
	if isV2 {
		writeErr = event.WriteSpanLineToFIFO(wireBytes, 2*time.Second)
	} else {
		writeErr = event.WriteToFIFO(payload, hookType, 2*time.Second)
	}

	if writeErr != nil {
		// Silent failure — never block Claude.
		debug("WriteToFIFO error: %v", writeErr)
		return
	}

	debug("event written successfully")
}

// toV2Wire converts a raw Claude hook payload to v2 JSON wire format.
// Returns (v2json, true) on success, or (raw, false) if parsing fails.
// When false, the caller should fall back to v1 WriteToFIFO.
func toV2Wire(hookType string, raw []byte) ([]byte, bool) {
	var p rawHookPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return raw, false
	}

	now := time.Now().UTC()
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

	status := "done"
	if p.Type == "queue-operation" {
		status = "running"
	}

	wire := v2SpanWire{
		V:         2,
		TraceID:   p.SessionID,
		SpanID:    p.UUID,
		System:    "anthropic",
		Model:     p.Model,
		Status:    status,
		StartTime: now,
		EndTime:   now,
		Attrs:     attrs,
	}

	b, err := json.Marshal(wire)
	if err != nil {
		return raw, false
	}
	return b, true
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/control"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
)

// debug logs a message to stderr when TAT_DEBUG=1 is set.
func debug(format string, args ...any) {
	if os.Getenv("TAT_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[tat-hook] "+format+"\n", args...)
	}
}

// preToolUsePayload holds the fields we care about from a PreToolUse hook.
type preToolUsePayload struct {
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

// postToolUsePayload holds the fields we care about from a PostToolUse hook.
type postToolUsePayload struct {
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	Usage     struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func main() {
	hookType := ""
	if len(os.Args) > 1 {
		hookType = os.Args[1]
	}

	debug("invoked with hookType=%q", hookType)

	payload, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
	if err != nil {
		debug("failed to read stdin: %v", err)
		os.Exit(0)
	}

	debug("payload=%d bytes", len(payload))

	// Write to FIFO for TUI observability (unchanged — always happens).
	if err := event.WriteToFIFO(payload, hookType, 2*time.Second); err != nil {
		debug("WriteToFIFO error: %v", err)
	}

	// Engine setup — any failure is fail-open (exit 0).
	eng, setupErr := setupEngine()
	if setupErr != nil {
		debug("engine setup failed: %v — fail-open", setupErr)
		os.Exit(0)
	}

	switch hookType {
	case "PreToolUse":
		handlePreToolUse(eng, payload)
	case "PostToolUse":
		handlePostToolUse(eng, payload)
	default:
		debug("unknown hook type %q — exit 0", hookType)
		os.Exit(0)
	}
}

// setupEngine loads policy and creates a ready Engine. Returns error on any setup failure.
func setupEngine() (*control.Engine, error) {
	pol, err := control.LoadPolicy()
	if err != nil {
		// LoadPolicy already logged and returned DefaultPolicy — still usable.
		// We only fail-open if we get a truly unrecoverable error.
		debug("policy load warning: %v", err)
	}

	cacheDir := control.CacheDir()
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, fmt.Errorf("mkdir cache dir: %w", err)
	}

	budgets := control.NewBudgetStore(cacheDir)
	denials := control.NewDenialLog(cacheDir)
	return control.NewEngine(pol, budgets, denials), nil
}

// handlePreToolUse evaluates the PreToolUse decision and exits accordingly.
func handlePreToolUse(eng *control.Engine, payload []byte) {
	var p preToolUsePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		debug("cannot parse PreToolUse payload: %v — fail-open", err)
		os.Exit(0)
	}

	// Build an input summary from the tool_input JSON (first 200 chars).
	inputSummary := inputSummaryFrom(p.ToolName, p.ToolInput)

	d := eng.PreToolUse(p.SessionID, p.ToolName, inputSummary)
	debug("PreToolUse decision: action=%s reason=%s", d.Action, d.Reason)

	switch d.Action {
	case "deny":
		fmt.Fprintf(os.Stderr, "BLOCKED by tonys-agent-telemetry: %s (%s)\n", d.Reason, d.Detail)
		os.Exit(2)
	case "warn":
		fmt.Fprintf(os.Stderr, "WARNING tonys-agent-telemetry: %s (%s)\n", d.Reason, d.Detail)
		os.Exit(0)
	default:
		os.Exit(0)
	}
}

// handlePostToolUse updates the budget from a PostToolUse event.
func handlePostToolUse(eng *control.Engine, payload []byte) {
	var p postToolUsePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		debug("cannot parse PostToolUse payload: %v — fail-open", err)
		os.Exit(0)
	}

	if err := eng.PostToolUse(p.SessionID, p.Model, p.Usage.InputTokens, p.Usage.OutputTokens); err != nil {
		debug("PostToolUse budget update error: %v — fail-open", err)
	}
	os.Exit(0)
}

// inputSummaryFrom extracts a concise string from tool_input JSON.
// For Bash, returns the command. For Read/Write/Edit, returns the file path.
// Falls back to the raw JSON truncated to 200 chars.
func inputSummaryFrom(toolName string, raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		s := string(raw)
		if len(s) > 200 {
			s = s[:200]
		}
		return s
	}

	// Tool-specific key extraction.
	key := ""
	switch toolName {
	case "Bash":
		key = "command"
	case "Read", "Write", "Edit", "MultiEdit":
		key = "file_path"
	case "WebFetch", "WebSearch":
		key = "url"
	}

	if key != "" {
		if v, ok := m[key]; ok {
			var s string
			if err := json.Unmarshal(v, &s); err == nil {
				if len(s) > 200 {
					s = s[:200]
				}
				return s
			}
		}
	}

	// Generic fallback: first string value found.
	for _, v := range m {
		var s string
		if err := json.Unmarshal(v, &s); err == nil && s != "" {
			if len(s) > 200 {
				s = s[:200]
			}
			return s
		}
	}
	return ""
}

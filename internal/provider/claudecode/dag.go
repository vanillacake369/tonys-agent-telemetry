package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// rawAgentInput is the input to the "Agent" tool_use.
type rawAgentInput struct {
	SubagentType    string `json:"subagent_type"`
	Description     string `json:"description"`
	RunInBackground bool   `json:"run_in_background"`
}

// DAGNode represents a node in the agent execution DAG.
// Mirrors data.DAGNode; the data package retains its own type until S5.
type DAGNode struct {
	ID          string
	AgentType   string
	Description string
	Status      string // "running", "done", "pending", "error"
	TokenCount  int
	StartTime   time.Time
	Duration    time.Duration
	Tools       []string
	Children    []*DAGNode
	ParentID    string
}

var taskIDRe = regexp.MustCompile(`<task-id>(.*?)</task-id>`)
var statusRe = regexp.MustCompile(`<status>(.*?)</status>`)

// parseQueueStatus extracts task-id and status from the XML-like content
// of a queue-operation enqueue event.
func parseQueueStatus(content json.RawMessage) (taskID, status string) {
	if len(content) == 0 {
		return "", ""
	}
	var s string
	if err := json.Unmarshal(content, &s); err != nil {
		s = string(content)
	}
	if m := taskIDRe.FindStringSubmatch(s); len(m) == 2 {
		taskID = m[1]
	}
	if m := statusRe.FindStringSubmatch(s); len(m) == 2 {
		status = m[1]
	}
	return taskID, status
}

// normalizeStatus maps raw status strings to canonical values.
func normalizeStatus(s string) string {
	switch strings.ToLower(s) {
	case "completed", "done":
		return "done"
	case "running":
		return "running"
	case "error", "failed", "cancelled":
		return "error"
	case "pending":
		return "pending"
	default:
		return s
	}
}

// ParseDAG reads a session JSONL file and its corresponding subagent files to build a DAG.
func ParseDAG(sessionFilePath string) (*DAGNode, error) {
	sessionID := strings.TrimSuffix(filepath.Base(sessionFilePath), ".jsonl")
	projectDir := filepath.Dir(sessionFilePath)

	if _, err := os.Stat(sessionFilePath); err != nil {
		return nil, fmt.Errorf("session file not found %s: %w", sessionFilePath, err)
	}

	root := &DAGNode{
		ID:     sessionID,
		Status: "done",
	}

	// First pass: read queue-operation events to build status map.
	statusMap := map[string]string{}
	{
		f, err := os.Open(sessionFilePath)
		if err != nil {
			return nil, err
		}
		scanner := newJSONLScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var msg rawMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}
			if msg.Type == "queue-operation" && msg.Operation == "enqueue" {
				taskID, status := parseQueueStatus(msg.Content)
				if taskID != "" && status != "" {
					statusMap[taskID] = status
				}
			}
		}
		f.Close()
	}

	// Second pass: read agent tool_use events to build child nodes.
	{
		f, err := os.Open(sessionFilePath)
		if err != nil {
			return nil, err
		}
		scanner := newJSONLScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var msg rawMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}

			if root.StartTime.IsZero() {
				if ts := parseTimestamp(msg.Timestamp); !ts.IsZero() {
					root.StartTime = ts
				}
			}

			if msg.Type != "assistant" || msg.Message == nil {
				continue
			}

			var blocks []rawContentBlock
			if err := json.Unmarshal(msg.Message.Content, &blocks); err != nil {
				continue
			}

			for _, block := range blocks {
				if block.Type != "tool_use" || block.Name != "Agent" {
					continue
				}
				var input rawAgentInput
				if err := json.Unmarshal(block.Input, &input); err != nil {
					continue
				}
				ts := parseTimestamp(msg.Timestamp)
				child := &DAGNode{
					AgentType:   input.SubagentType,
					Description: input.Description,
					Status:      "pending",
					StartTime:   ts,
				}
				root.Children = append(root.Children, child)
			}
		}
		f.Close()
	}

	// Load subagent metadata.
	subagentsDir := projectDir + "/" + sessionID + "/subagents"
	if subEntries, err := os.ReadDir(subagentsDir); err == nil {
		for _, e := range subEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".meta.json") {
				continue
			}
			agentID := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "agent-"), ".meta.json")
			metaPath := subagentsDir + "/" + e.Name()
			metaData, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}
			var meta struct {
				AgentType   string `json:"agentType"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal(metaData, &meta); err != nil {
				continue
			}

			status := statusMap[agentID]
			if status == "" {
				status = "running"
			}

			matched := false
			for _, child := range root.Children {
				if child.ID == "" &&
					(strings.EqualFold(child.AgentType, meta.AgentType) ||
						strings.EqualFold(child.Description, meta.Description)) {
					child.ID = agentID
					child.Status = normalizeStatus(status)
					matched = true
					break
				}
			}
			if !matched {
				child := &DAGNode{
					ID:          agentID,
					AgentType:   meta.AgentType,
					Description: meta.Description,
					Status:      normalizeStatus(status),
				}
				root.Children = append(root.Children, child)
			}
		}
	}

	// Load token counts from subagent JSONL files.
	if subEntries, err := os.ReadDir(subagentsDir); err == nil {
		for _, e := range subEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			agentID := strings.TrimSuffix(strings.TrimPrefix(e.Name(), "agent-"), ".jsonl")
			tokens, tools := readSubagentStats(subagentsDir + "/" + e.Name())
			for _, child := range root.Children {
				if child.ID == agentID {
					child.TokenCount = tokens
					child.Tools = tools
					break
				}
			}
		}
	}

	return root, nil
}

// readSubagentStats reads a subagent JSONL to sum token counts and collect tool names.
func readSubagentStats(path string) (totalTokens int, tools []string) {
	f, err := os.Open(path)
	if err != nil {
		return 0, nil
	}
	defer f.Close()

	toolSet := map[string]struct{}{}
	scanner := newJSONLScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg rawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Type == "assistant" && msg.Message != nil && msg.Message.Usage != nil {
			u := msg.Message.Usage
			totalTokens += u.InputTokens + u.OutputTokens +
				u.CacheCreationInputTokens + u.CacheReadInputTokens
		}
		if msg.Type == "assistant" && msg.Message != nil {
			var blocks []rawContentBlock
			if err := json.Unmarshal(msg.Message.Content, &blocks); err == nil {
				for _, b := range blocks {
					if b.Type == "tool_use" && b.Name != "" {
						toolSet[b.Name] = struct{}{}
					}
				}
			}
		}
	}
	for name := range toolSet {
		tools = append(tools, name)
	}
	return totalTokens, tools
}

package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// rawMessage is the envelope for every JSONL line in a Claude session.
type rawMessage struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionId"`
	CWD       string          `json:"cwd"`
	GitBranch string          `json:"gitBranch"`
	Version   string          `json:"version"`
	UUID      string          `json:"uuid"`
	AgentID   string          `json:"agentId"`
	Operation string          `json:"operation"`
	Content   json.RawMessage `json:"content"`
	Message   *rawInnerMsg    `json:"message"`
}

// rawInnerMsg is the inner message object for user/assistant types.
type rawInnerMsg struct {
	Role    string          `json:"role"`
	Model   string          `json:"model"`
	Content json.RawMessage `json:"content"` // string or []rawContentBlock
	Usage   *rawUsage       `json:"usage"`
}

// rawUsage captures token usage from assistant messages.
type rawUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// rawContentBlock is one element of a content array.
type rawContentBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Thinking string         `json:"thinking"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
}

// rawAgentInput is the input to the "Agent" tool_use.
type rawAgentInput struct {
	SubagentType    string `json:"subagent_type"`
	Description     string `json:"description"`
	RunInBackground bool   `json:"run_in_background"`
}

// Turn is a single user or assistant turn for preview.
type Turn struct {
	Role    string // "user" or "assistant"
	Content string // truncated text
}

const maxFirstPromptLen = 100
const thinkingPlaceholder = "[thinking...]"

// parseTimestamp parses an RFC3339 timestamp string. Returns zero time on error.
func parseTimestamp(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, _ = time.Parse(time.RFC3339, s)
	}
	return t
}

// truncate truncates a UTF-8 string to at most n runes.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n])
}

// extractTextFromContent extracts plain text from a content value that is
// either a plain JSON string or a []rawContentBlock array.
// thinking blocks are replaced with thinkingPlaceholder.
func extractTextFromContent(raw json.RawMessage, skipThinking bool) string {
	if len(raw) == 0 {
		return ""
	}
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try array of blocks
	var blocks []rawContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		case "thinking":
			if !skipThinking && b.Thinking != "" {
				parts = append(parts, b.Thinking)
			} else if skipThinking {
				parts = append(parts, thinkingPlaceholder)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// ParseSessionHeader reads the entire JSONL file to extract session metadata
// and compute session statistics (turn count, duration).
// Corrupted lines are skipped. Missing fields are zero values.
// Returns an error only if the file cannot be opened or contains no usable data.
func ParseSessionHeader(filepath string) (*Session, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	session := &Session{FilePath: filepath}
	foundAny := false
	var lastTimestamp time.Time
	var searchParts []string
	const maxSearchTextLen = 2000 // cap total search text to limit memory
	scanner := bufio.NewScanner(f)
	// Increase buffer size for long lines.
	buf := make([]byte, 1<<20)
	scanner.Buffer(buf, 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg rawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			// Corrupted line — skip.
			continue
		}
		// Populate common session fields from any message type.
		if msg.SessionID != "" && session.ID == "" {
			session.ID = msg.SessionID
		}
		if msg.CWD != "" && session.CWD == "" {
			session.CWD = msg.CWD
		}
		if msg.GitBranch != "" && session.GitBranch == "" {
			session.GitBranch = msg.GitBranch
		}
		if msg.Version != "" && session.Version == "" {
			session.Version = msg.Version
		}
		if ts := parseTimestamp(msg.Timestamp); !ts.IsZero() {
			if session.Timestamp.IsZero() {
				session.Timestamp = ts
			}
			lastTimestamp = ts
		}

		// Extract model from assistant messages.
		if msg.Type == "assistant" && msg.Message != nil {
			if msg.Message.Model != "" && session.Model == "" {
				session.Model = msg.Message.Model
			}
		}

		// Count user turns, extract prompts, and build search text.
		if msg.Type == "user" && msg.Message != nil {
			session.TurnCount++
			text := extractTextFromContent(msg.Message.Content, false)
			text = strings.ReplaceAll(text, "\n", " ")
			text = strings.ReplaceAll(text, "\r", "")

			if text != "" {
				if session.FirstPrompt == "" {
					session.FirstPrompt = truncate(text, maxFirstPromptLen)
				}
				// Collect all user messages for full-text search.
				currentLen := 0
				for _, p := range searchParts {
					currentLen += len(p)
				}
				if currentLen < maxSearchTextLen {
					searchParts = append(searchParts, truncate(text, 200))
				}
			}
		}

		foundAny = true
	}

	if err := scanner.Err(); err != nil {
		// Partial read (e.g. truncated last line during concurrent write) — return what we have.
		if !foundAny {
			return nil, err
		}
	}

	if !foundAny {
		return nil, fmt.Errorf("no usable data in %s", filepath)
	}

	// Build search text from all collected user messages.
	session.SearchText = strings.Join(searchParts, " ")

	// Compute duration from first to last observed timestamp.
	if !session.Timestamp.IsZero() && lastTimestamp.After(session.Timestamp) {
		session.Duration = lastTimestamp.Sub(session.Timestamp)
	}

	return session, nil
}

// ParseConversationPreview extracts the first maxTurns user/assistant turns.
// thinking content blocks are replaced with "[thinking...]".
// Long messages are truncated to 300 characters.
func ParseConversationPreview(filepath string, maxTurns int) ([]Turn, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	const maxContentLen = 300
	var turns []Turn
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1<<20)
	scanner.Buffer(buf, 1<<20)

	for scanner.Scan() && len(turns) < maxTurns {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg rawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Type != "user" && msg.Type != "assistant" {
			continue
		}
		if msg.Message == nil {
			continue
		}
		text := extractTextFromContent(msg.Message.Content, true)
		if text == "" {
			continue
		}
		turns = append(turns, Turn{
			Role:    msg.Type,
			Content: truncate(text, maxContentLen),
		})
	}
	// Ignore scanner.Err() for truncated-line tolerance.

	return turns, nil
}

// FileChange represents a file that was accessed or modified during a session.
type FileChange struct {
	Path   string
	Action string // "write", "edit", or "read"
}

// actionRank maps action names to precedence for deduplication.
// Higher rank wins when the same file appears multiple times.
var actionRank = map[string]int{
	"write": 3,
	"edit":  2,
	"read":  1,
}

// ParseFileChanges extracts unique file paths from tool_use events in the JSONL.
// For each tool_use with name "Edit", "Write", or "Read", input.file_path is extracted.
// When the same path appears multiple times, write > edit > read takes precedence.
// Results are returned in first-occurrence order.
func ParseFileChanges(filepath string) ([]FileChange, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Track order of first occurrence and best action per path.
	type entry struct {
		action string
		order  int
	}
	seen := map[string]entry{}
	var order int

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1<<20)
	scanner.Buffer(buf, 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg rawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Type != "assistant" || msg.Message == nil {
			continue
		}
		var blocks []rawContentBlock
		if err := json.Unmarshal(msg.Message.Content, &blocks); err != nil {
			continue
		}
		for _, block := range blocks {
			if block.Type != "tool_use" {
				continue
			}
			var action string
			switch block.Name {
			case "Write":
				action = "write"
			case "Edit", "MultiEdit":
				action = "edit"
			case "Read":
				action = "read"
			default:
				continue
			}
			// Extract file_path from the tool input.
			var input struct {
				FilePath string `json:"file_path"`
			}
			if err := json.Unmarshal(block.Input, &input); err != nil || input.FilePath == "" {
				continue
			}
			if prev, exists := seen[input.FilePath]; exists {
				// Keep higher-rank action.
				if actionRank[action] > actionRank[prev.action] {
					seen[input.FilePath] = entry{action: action, order: prev.order}
				}
			} else {
				seen[input.FilePath] = entry{action: action, order: order}
				order++
			}
		}
	}
	// Ignore scanner.Err() for truncated-line tolerance.

	// Build result slice in first-occurrence order.
	result := make([]FileChange, len(seen))
	for path, e := range seen {
		result[e.order] = FileChange{Path: path, Action: e.action}
	}
	return result, nil
}

// agentTaskStatus maps a task ID to its completion status from queue-operation events.
type agentTaskStatus struct {
	AgentID string
	Status  string
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

// ParseDAG reads a session JSONL file and its corresponding subagent files to build a DAG.
// Agent tool_use events create child nodes. queue-operation events update status.
// Returns the root node. If no subagents are found, the root is a single node.
//
// sessionFilePath is the path to the session .jsonl file.
// The subagents directory is at: {projectDir}/{sessionID}/subagents/
func ParseDAG(sessionFilePath string) (*DAGNode, error) {
	sessionID := strings.TrimSuffix(filepath.Base(sessionFilePath), ".jsonl")
	projectDir := filepath.Dir(sessionFilePath)

	mainJSONL := sessionFilePath

	// Verify the file exists.
	if _, err := os.Stat(mainJSONL); err != nil {
		return nil, fmt.Errorf("session file not found %s: %w", mainJSONL, err)
	}

	root := &DAGNode{
		ID:     sessionID,
		Status: "done",
	}

	// First pass: read queue-operation events to build status map.
	statusMap := map[string]string{} // task-id (agentId) → status
	{
		f, err := os.Open(mainJSONL)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		buf := make([]byte, 1<<20)
		scanner.Buffer(buf, 1<<20)
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
	// Also populate root metadata from the session.
	{
		f, err := os.Open(mainJSONL)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		buf := make([]byte, 1<<20)
		scanner.Buffer(buf, 1<<20)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var msg rawMessage
			if err := json.Unmarshal([]byte(line), &msg); err != nil {
				continue
			}

			// Capture root session metadata.
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

	// Load subagent metadata to populate IDs and resolve statuses.
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

			// Find matching child by agentType/description or create one.
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
				// Subagent not spawned via Agent tool_use in the main file,
				// or couldn't correlate — add as child.
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
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1<<20)
	scanner.Buffer(buf, 1<<20)
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

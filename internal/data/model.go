package data

import "time"

// Session represents metadata extracted from a Claude Code session JSONL file.
type Session struct {
	ID          string
	ProjectDir  string        // derived from path
	CWD         string
	GitBranch   string
	FirstPrompt string        // first user message (truncated to 100 chars)
	SearchText  string        // all user messages concatenated (for full-text search)
	Timestamp   time.Time
	Model       string        // e.g. "claude-opus-4-6"
	Version     string
	FilePath    string        // absolute path to .jsonl
	TurnCount   int           // number of user messages
	Duration    time.Duration // last timestamp - first timestamp
}

// Agent represents a configured Claude Code agent.
type Agent struct {
	Name        string
	Type        string    // from .meta.json agentType
	Description string    // from .meta.json or agent .md file
	Model       string    // from claude agents output or agent definition
}

// DAGNode represents a node in the agent execution DAG.
type DAGNode struct {
	ID          string
	AgentType   string
	Description string
	Status      string    // "running", "done", "pending", "error"
	TokenCount  int
	StartTime   time.Time
	Duration    time.Duration
	Tools       []string  // tool names used
	Children    []*DAGNode
	ParentID    string
}

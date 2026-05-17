package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GeminiProvider implements Provider for Google Gemini CLI (~/.gemini).
type GeminiProvider struct{}

func init() {
	RegisterProvider(&GeminiProvider{})
}

func (p *GeminiProvider) Name() ProviderName { return ProviderGemini }

func (p *GeminiProvider) DataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".gemini")
	}
	return filepath.Join(home, ".gemini")
}

func (p *GeminiProvider) Available() bool {
	_, err := os.Stat(filepath.Join(p.DataDir(), "tmp"))
	return err == nil
}

// geminiLogEntry is a single entry in Gemini's logs.json.
type geminiLogEntry struct {
	SessionID string `json:"sessionId"`
	MessageID int    `json:"messageId"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// geminiProjects maps absolute path → project name.
type geminiProjects struct {
	Projects map[string]string `json:"projects"`
}

func (p *GeminiProvider) DiscoverSessions() ([]Session, error) {
	tmpDir := filepath.Join(p.DataDir(), "tmp")
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, err
	}

	// Load project path mapping.
	projectPaths := p.loadProjectPaths()

	var sessions []Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectName := entry.Name()
		logsPath := filepath.Join(tmpDir, projectName, "logs.json")
		projectSessions, err := p.parseLogsFile(logsPath, projectName, projectPaths)
		if err != nil {
			continue
		}
		sessions = append(sessions, projectSessions...)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp.After(sessions[j].Timestamp)
	})
	return sessions, nil
}

func (p *GeminiProvider) DiscoverCosts() ([]SessionCost, error) {
	// Gemini CLI does not store token usage data.
	// Return sessions with zero cost but correct metadata for tracking.
	sessions, err := p.DiscoverSessions()
	if err != nil {
		return nil, err
	}

	var costs []SessionCost
	for _, s := range sessions {
		costs = append(costs, SessionCost{
			SessionID: s.ID,
			Provider:  ProviderGemini,
			Project:   s.ProjectDir,
			Model:     s.Model,
			TurnCount: s.TurnCount,
			Duration:  s.Duration,
			Timestamp: s.Timestamp,
			ToolCalls: make(map[string]int),
		})
	}
	return costs, nil
}

// geminiFilePathSeparator separates the logs.json path from the session ID.
const geminiFilePathSeparator = "#"

// encodeGeminiFilePath combines the logs.json path with a session ID.
func encodeGeminiFilePath(logsPath, sessionID string) string {
	return logsPath + geminiFilePathSeparator + sessionID
}

// decodeGeminiFilePath splits the encoded path into logsPath and sessionID.
func decodeGeminiFilePath(encoded string) (logsPath, sessionID string) {
	idx := strings.LastIndex(encoded, geminiFilePathSeparator)
	if idx < 0 {
		return encoded, ""
	}
	return encoded[:idx], encoded[idx+1:]
}

func (p *GeminiProvider) ParseConversationPreview(filePath string, maxTurns int) ([]Turn, error) {
	logsPath, sessionID := decodeGeminiFilePath(filePath)

	raw, err := os.ReadFile(logsPath)
	if err != nil {
		return nil, err
	}

	var entries []geminiLogEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}

	// Filter entries to the specific session.
	const maxContentLen = 300
	var turns []Turn
	for _, e := range entries {
		if sessionID != "" && e.SessionID != sessionID {
			continue
		}
		if e.Type == "user" && e.Message != "" {
			turns = append(turns, Turn{
				Role:    "user",
				Content: truncate(e.Message, maxContentLen),
			})
			if len(turns) >= maxTurns {
				break
			}
		}
	}
	return turns, nil
}

func (p *GeminiProvider) ParseFullConversation(filePath string) ([]DetailTurn, error) {
	logsPath, sessionID := decodeGeminiFilePath(filePath)

	raw, err := os.ReadFile(logsPath)
	if err != nil {
		return nil, err
	}

	var entries []geminiLogEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}

	var turns []DetailTurn
	for _, e := range entries {
		if sessionID != "" && e.SessionID != sessionID {
			continue
		}
		if e.Type == "user" && e.Message != "" {
			turns = append(turns, DetailTurn{
				Role:      "user",
				Content:   e.Message,
				Timestamp: e.Timestamp,
			})
		}
	}
	return turns, nil
}

func (p *GeminiProvider) ParseFileChanges(filePath string) ([]FileChange, error) {
	// Gemini stores tool outputs in tool-outputs/session-{id}/ directories.
	// We can scan for file patterns but there's no structured file change data.
	logsPath, _ := decodeGeminiFilePath(filePath)
	dir := filepath.Dir(logsPath)
	toolOutputsDir := filepath.Join(dir, "tool-outputs")
	entries, err := os.ReadDir(toolOutputsDir)
	if err != nil {
		return nil, nil // Not an error — just no tool outputs
	}

	var changes []FileChange
	seen := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionDir := filepath.Join(toolOutputsDir, entry.Name())
		files, _ := os.ReadDir(sessionDir)
		for _, f := range files {
			name := f.Name()
			// Parse tool name from filename pattern: {tool}_{tool}_{timestamp}_{idx}_{hash}.txt
			parts := strings.SplitN(name, "_", 3)
			if len(parts) >= 1 && !seen[parts[0]] {
				seen[parts[0]] = true
				changes = append(changes, FileChange{
					Path:   parts[0],
					Action: "read", // Gemini tool outputs are results, not edits
				})
			}
		}
	}
	return changes, nil
}

func (p *GeminiProvider) ResumeCommand(session Session) string {
	if session.CWD != "" {
		return fmt.Sprintf("cd %q && gemini --resume %s", session.CWD, session.ID)
	}
	return fmt.Sprintf("gemini --resume %s", session.ID)
}

// loadProjectPaths reads ~/.gemini/projects.json to get path → name mapping.
func (p *GeminiProvider) loadProjectPaths() map[string]string {
	projFile := filepath.Join(p.DataDir(), "projects.json")
	data, err := os.ReadFile(projFile)
	if err != nil {
		return nil
	}
	var gp geminiProjects
	if err := json.Unmarshal(data, &gp); err != nil {
		return nil
	}
	// Reverse: name → path (for matching directory names to absolute paths)
	result := make(map[string]string)
	for path, name := range gp.Projects {
		result[name] = path
	}
	return result
}

// parseLogsFile parses a Gemini logs.json and groups entries by sessionId.
func (p *GeminiProvider) parseLogsFile(logsPath, projectName string, projectPaths map[string]string) ([]Session, error) {
	data, err := os.ReadFile(logsPath)
	if err != nil {
		return nil, err
	}

	var entries []geminiLogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// Resolve project dir from projects.json.
	projectDir := ""
	if projectPaths != nil {
		projectDir = projectPaths[projectName]
	}
	if projectDir == "" {
		// Try .project_root file
		rootFile := filepath.Join(filepath.Dir(logsPath), "..", "history", projectName, ".project_root")
		if b, err := os.ReadFile(rootFile); err == nil {
			projectDir = strings.TrimSpace(string(b))
		}
	}

	// Group entries by sessionId.
	type sessionGroup struct {
		entries []geminiLogEntry
	}
	groups := map[string]*sessionGroup{}
	var order []string
	for _, e := range entries {
		if e.SessionID == "" {
			continue
		}
		g, exists := groups[e.SessionID]
		if !exists {
			g = &sessionGroup{}
			groups[e.SessionID] = g
			order = append(order, e.SessionID)
		}
		g.entries = append(g.entries, e)
	}

	var sessions []Session
	for _, sid := range order {
		g := groups[sid]
		if len(g.entries) == 0 {
			continue
		}

		s := Session{
			ID:         sid,
			Provider:   ProviderGemini,
			ProjectDir: projectDir,
			CWD:        projectDir,
			FilePath:   encodeGeminiFilePath(logsPath, sid),
			Model:      "gemini-2.5-pro",
			TurnCount:  len(g.entries),
		}

		// First entry's message is the first prompt.
		first := g.entries[0]
		s.FirstPrompt = truncate(strings.ReplaceAll(first.Message, "\n", " "), maxFirstPromptLen)
		if ts := parseTimestamp(first.Timestamp); !ts.IsZero() {
			s.Timestamp = ts
		}

		// Build search text.
		var searchParts []string
		for _, e := range g.entries {
			if e.Message != "" {
				searchParts = append(searchParts, truncate(e.Message, 80))
			}
		}
		s.SearchText = strings.Join(searchParts, " ")

		// Compute duration from first to last timestamp.
		if len(g.entries) > 1 {
			last := g.entries[len(g.entries)-1]
			if ts := parseTimestamp(last.Timestamp); !ts.IsZero() && !s.Timestamp.IsZero() {
				s.Duration = ts.Sub(s.Timestamp)
			}
		}

		sessions = append(sessions, s)
	}

	return sessions, nil
}

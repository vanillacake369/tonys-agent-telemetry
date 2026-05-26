package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// CodexProvider implements Provider for OpenAI Codex CLI (~/.codex).
type CodexProvider struct{}

func init() {
	RegisterProvider(&CodexProvider{})
}

func (p *CodexProvider) Name() ProviderName { return ProviderCodex }

func (p *CodexProvider) DataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".codex")
	}
	return filepath.Join(home, ".codex")
}

func (p *CodexProvider) Available() bool {
	_, err := os.Stat(filepath.Join(p.DataDir(), "sessions"))
	return err == nil
}

// codexSessionMeta is the session_meta payload in Codex JSONL.
type codexSessionMeta struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	CWD        string `json:"cwd"`
	CLIVersion string `json:"cli_version"`
	Git        struct {
		Branch string `json:"branch"`
	} `json:"git"`
}

// codexLine is the top-level envelope for Codex JSONL.
type codexLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// codexEventPayload covers event_msg payloads.
type codexEventPayload struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Info    *struct {
		TotalTokenUsage struct {
			InputTokens       int `json:"input_tokens"`
			CachedInputTokens int `json:"cached_input_tokens"`
			OutputTokens      int `json:"output_tokens"`
			ReasoningTokens   int `json:"reasoning_output_tokens"`
			TotalTokens       int `json:"total_tokens"`
		} `json:"total_token_usage"`
	} `json:"info"`
}

// codexResponsePayload covers response_item payloads.
type codexResponsePayload struct {
	Type    string `json:"type"`
	Role    string `json:"role"`
	Phase   string `json:"phase"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// codexFunctionCall covers function_call payloads in response_items.
type codexFunctionCall struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func (p *CodexProvider) DiscoverSessions() ([]Session, error) {
	sessionsDir := filepath.Join(p.DataDir(), "sessions")
	files, err := findCodexJSONLFiles(sessionsDir)
	if err != nil {
		return nil, err
	}

	const workers = 4
	jobCh := make(chan string, len(files))
	for _, f := range files {
		jobCh <- f
	}
	close(jobCh)

	resultCh := make(chan Session, len(files))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobCh {
				s, err := p.parseSessionHeader(f)
				if err != nil {
					continue
				}
				resultCh <- *s
			}
		}()
	}
	wg.Wait()
	close(resultCh)

	var sessions []Session
	for s := range resultCh {
		sessions = append(sessions, s)
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp.After(sessions[j].Timestamp)
	})
	return sessions, nil
}

func (p *CodexProvider) DiscoverCosts() ([]SessionCost, error) {
	sessionsDir := filepath.Join(p.DataDir(), "sessions")
	files, err := findCodexJSONLFiles(sessionsDir)
	if err != nil {
		return nil, err
	}

	const workers = 4
	jobCh := make(chan string, len(files))
	for _, f := range files {
		jobCh <- f
	}
	close(jobCh)

	resultCh := make(chan SessionCost, len(files))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobCh {
				c, err := p.parseSessionCost(f)
				if err != nil {
					continue
				}
				resultCh <- *c
			}
		}()
	}
	wg.Wait()
	close(resultCh)

	var costs []SessionCost
	for c := range resultCh {
		costs = append(costs, c)
	}
	sort.Slice(costs, func(i, j int) bool {
		return costs[i].Timestamp.After(costs[j].Timestamp)
	})
	return costs, nil
}

func (p *CodexProvider) ParseConversationPreview(filePath string, maxTurns int) ([]Turn, error) {
	f, err := os.Open(filePath)
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
		var cl codexLine
		if err := json.Unmarshal([]byte(line), &cl); err != nil {
			continue
		}

		switch cl.Type {
		case "event_msg":
			var ep codexEventPayload
			if err := json.Unmarshal(cl.Payload, &ep); err != nil {
				continue
			}
			if ep.Type == "user_message" && ep.Message != "" {
				turns = append(turns, Turn{
					Role:    "user",
					Content: truncate(ep.Message, maxContentLen),
				})
			}
		case "response_item":
			var rp codexResponsePayload
			if err := json.Unmarshal(cl.Payload, &rp); err != nil {
				continue
			}
			if rp.Role == "assistant" {
				var text string
				for _, c := range rp.Content {
					if c.Type == "output_text" && c.Text != "" {
						text += c.Text
					}
				}
				if text != "" {
					turns = append(turns, Turn{
						Role:    "assistant",
						Content: truncate(text, maxContentLen),
					})
				}
			}
		}
	}
	return turns, nil
}

func (p *CodexProvider) ParseFullConversation(filePath string) ([]DetailTurn, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	const maxToolInputLen = 200
	var turns []DetailTurn
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1<<20)
	scanner.Buffer(buf, 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var cl codexLine
		if err := json.Unmarshal([]byte(line), &cl); err != nil {
			continue
		}
		switch cl.Type {
		case "event_msg":
			var ep codexEventPayload
			if err := json.Unmarshal(cl.Payload, &ep); err != nil {
				continue
			}
			if ep.Type == "user_message" && ep.Message != "" {
				turns = append(turns, DetailTurn{
					Role:      "user",
					Content:   ep.Message,
					Timestamp: cl.Timestamp,
				})
			}
		case "response_item":
			var rp codexResponsePayload
			if err := json.Unmarshal(cl.Payload, &rp); err != nil {
				continue
			}
			if rp.Role == "assistant" {
				dt := DetailTurn{
					Role:      "assistant",
					Timestamp: cl.Timestamp,
				}
				for _, c := range rp.Content {
					if c.Type == "output_text" && c.Text != "" {
						dt.Content += c.Text
					}
				}
				if dt.Content != "" {
					turns = append(turns, dt)
				}
			}
			// Tool calls
			var fc codexFunctionCall
			if err := json.Unmarshal(cl.Payload, &fc); err == nil && fc.Type == "function_call" && fc.Name != "" {
				tc := ToolCall{Name: fc.Name}
				var args struct {
					Arguments string `json:"arguments"`
				}
				_ = json.Unmarshal(cl.Payload, &args)
				if args.Arguments != "" {
					input := args.Arguments
					if len(input) > maxToolInputLen {
						input = input[:maxToolInputLen] + "…"
					}
					tc.Input = input
				}
				// Attach to last assistant turn or create new one.
				if len(turns) > 0 && turns[len(turns)-1].Role == "assistant" {
					turns[len(turns)-1].ToolCalls = append(turns[len(turns)-1].ToolCalls, tc)
				} else {
					turns = append(turns, DetailTurn{
						Role:      "assistant",
						ToolCalls: []ToolCall{tc},
						Timestamp: cl.Timestamp,
					})
				}
			}
		}
	}
	return turns, nil
}

func (p *CodexProvider) ParseFileChanges(filePath string) ([]FileChange, error) {
	// Codex uses exec_command + apply_patch; no direct file_path extraction
	// like Claude. We scan for function_call with write-related names.
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1<<20)
	scanner.Buffer(buf, 1<<20)

	type entry struct {
		action string
		order  int
	}
	seen := map[string]entry{}
	var order int

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var cl codexLine
		if err := json.Unmarshal([]byte(line), &cl); err != nil {
			continue
		}
		if cl.Type != "response_item" {
			continue
		}
		var fc codexFunctionCall
		if err := json.Unmarshal(cl.Payload, &fc); err != nil {
			continue
		}
		if fc.Type != "function_call" {
			continue
		}
		// Extract arguments to look for file paths
		var args struct {
			Command  string `json:"command"`
			FilePath string `json:"file_path"`
		}
		// Codex function_call has arguments field
		var fullPayload struct {
			Arguments string `json:"arguments"`
		}
		if err := json.Unmarshal(cl.Payload, &fullPayload); err != nil {
			continue
		}
		if fullPayload.Arguments != "" {
			_ = json.Unmarshal([]byte(fullPayload.Arguments), &args)
		}

		if args.FilePath != "" {
			action := "edit"
			if fc.Name == "write_file" {
				action = "write"
			} else if fc.Name == "read_file" {
				action = "read"
			}
			if prev, exists := seen[args.FilePath]; exists {
				if actionRank[action] > actionRank[prev.action] {
					seen[args.FilePath] = entry{action: action, order: prev.order}
				}
			} else {
				seen[args.FilePath] = entry{action: action, order: order}
				order++
			}
		}
	}

	result := make([]FileChange, len(seen))
	for path, e := range seen {
		result[e.order] = FileChange{Path: path, Action: e.action}
	}
	return result, nil
}

func (p *CodexProvider) ResumeCommand(session Session) string {
	if session.CWD != "" {
		return fmt.Sprintf("cd %q && codex resume %s", session.CWD, session.ID)
	}
	return fmt.Sprintf("codex resume %s", session.ID)
}

// parseSessionHeader parses a Codex JSONL file for session metadata.
func (p *CodexProvider) parseSessionHeader(filePath string) (*Session, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	session := &Session{
		FilePath: filePath,
		Provider: ProviderCodex,
	}
	foundAny := false
	var lastTimestamp time.Time
	var searchParts []string
	var searchTextLen int
	const maxSearchTextLen = 20000
	const maxPerTurn = 80

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1<<20)
	scanner.Buffer(buf, 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var cl codexLine
		if err := json.Unmarshal([]byte(line), &cl); err != nil {
			continue
		}

		if ts := parseTimestamp(cl.Timestamp); !ts.IsZero() {
			if session.Timestamp.IsZero() {
				session.Timestamp = ts
			}
			lastTimestamp = ts
		}

		switch cl.Type {
		case "session_meta":
			var meta codexSessionMeta
			if err := json.Unmarshal(cl.Payload, &meta); err == nil {
				session.ID = meta.ID
				if meta.CWD != "" {
					session.CWD = meta.CWD
					session.ProjectDir = meta.CWD
				}
				if meta.Git.Branch != "" {
					session.GitBranch = meta.Git.Branch
				}
				session.Version = meta.CLIVersion
				session.Model = "gpt-5" // Codex uses GPT-5
				if ts := parseTimestamp(meta.Timestamp); !ts.IsZero() {
					session.Timestamp = ts
				}
			}
		case "event_msg":
			var ep codexEventPayload
			if err := json.Unmarshal(cl.Payload, &ep); err == nil {
				if ep.Type == "user_message" && ep.Message != "" {
					session.TurnCount++
					text := strings.ReplaceAll(ep.Message, "\n", " ")
					if session.FirstPrompt == "" {
						session.FirstPrompt = truncate(text, maxFirstPromptLen)
					}
					if searchTextLen < maxSearchTextLen {
						chunk := truncate(text, maxPerTurn)
						searchParts = append(searchParts, chunk)
						searchTextLen += len(chunk)
					}
				}
			}
		}
		foundAny = true
	}

	if !foundAny {
		return nil, fmt.Errorf("no usable data in %s", filePath)
	}

	session.SearchText = strings.Join(searchParts, " ")
	if !session.Timestamp.IsZero() && lastTimestamp.After(session.Timestamp) {
		session.Duration = lastTimestamp.Sub(session.Timestamp)
	}

	return session, nil
}

// parseSessionCost parses a Codex JSONL file for cost data.
func (p *CodexProvider) parseSessionCost(filePath string) (*SessionCost, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := &SessionCost{
		Provider:  ProviderCodex,
		ToolCalls: make(map[string]int),
	}

	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1<<20)
	scanner.Buffer(buf, 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var cl codexLine
		if err := json.Unmarshal([]byte(line), &cl); err != nil {
			continue
		}

		if sc.Timestamp.IsZero() {
			if ts := parseTimestamp(cl.Timestamp); !ts.IsZero() {
				sc.Timestamp = ts
			}
		}

		switch cl.Type {
		case "session_meta":
			var meta codexSessionMeta
			if err := json.Unmarshal(cl.Payload, &meta); err == nil {
				sc.SessionID = meta.ID
				sc.Project = meta.CWD
				sc.Model = "gpt-5"
				if ts := parseTimestamp(meta.Timestamp); !ts.IsZero() {
					sc.Timestamp = ts
				}
			}
		case "event_msg":
			var ep codexEventPayload
			if err := json.Unmarshal(cl.Payload, &ep); err == nil {
				if ep.Type == "user_message" {
					sc.TurnCount++
				}
				if ep.Type == "token_count" && ep.Info != nil {
					u := ep.Info.TotalTokenUsage
					sc.InputTokens = u.InputTokens
					sc.OutputTokens = u.OutputTokens
					sc.CacheRead = u.CachedInputTokens
					sc.TotalTokens = u.TotalTokens
				}
			}
		case "response_item":
			var fc codexFunctionCall
			if err := json.Unmarshal(cl.Payload, &fc); err == nil {
				if fc.Type == "function_call" && fc.Name != "" {
					sc.ToolCalls[fc.Name]++
				}
			}
		}
	}

	sc.EstCostUSD = CalculateCostCodex(sc.InputTokens, sc.OutputTokens, sc.CacheRead)
	return sc, nil
}

// CalculateCostCodex estimates cost for Codex/GPT-5 sessions.
// Pricing approximation for GPT-5 (per million tokens).
func CalculateCostCodex(input, output, cached int) float64 {
	const (
		inputPerM  = 2.0
		outputPerM = 8.0
		cachedPerM = 0.5
	)
	return (float64(input)*inputPerM +
		float64(output)*outputPerM +
		float64(cached)*cachedPerM) / 1_000_000
}

// findCodexJSONLFiles walks the Codex sessions directory tree
// (sessions/{year}/{month}/{day}/*.jsonl).
func findCodexJSONLFiles(sessionsDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

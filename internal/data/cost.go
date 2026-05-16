package data

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// modelPricing holds pricing per million tokens (USD) for a model.
type modelPricing struct {
	InputPerM      float64
	OutputPerM     float64
	CacheReadPerM  float64
	CacheWritePerM float64
}

// ModelPricing maps model name to USD-per-million-token rates.
// Source: https://docs.anthropic.com/en/docs/about-claude/models
var ModelPricing = map[string]modelPricing{
	"claude-opus-4-6":   {InputPerM: 15.0, OutputPerM: 75.0, CacheReadPerM: 1.5, CacheWritePerM: 18.75},
	"claude-sonnet-4-6": {InputPerM: 3.0, OutputPerM: 15.0, CacheReadPerM: 0.3, CacheWritePerM: 3.75},
	"claude-haiku-4-5":  {InputPerM: 0.8, OutputPerM: 4.0, CacheReadPerM: 0.08, CacheWritePerM: 1.0},
}

// SessionCost holds aggregated cost data for one session.
type SessionCost struct {
	SessionID    string
	Project      string
	Model        string         // primary model used
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	TotalTokens  int
	EstCostUSD   float64
	TurnCount    int
	Duration     time.Duration  // sum of turn_duration fields
	Timestamp    time.Time      // session start time
	ToolCalls    map[string]int // tool name → count
}

// ModelStats holds aggregated stats for a single model.
type ModelStats struct {
	Tokens int
	Cost   float64
	Turns  int
}

// ToolStat holds a tool name and its total invocation count.
type ToolStat struct {
	Name  string
	Count int
}

// CostSummary holds aggregated stats across all sessions.
type CostSummary struct {
	TotalCostUSD  float64
	TotalTokens   int
	TotalSessions int
	TotalTurns    int
	TotalDuration time.Duration
	ByModel       map[string]ModelStats
	ByProject     map[string]float64 // project → cost
	ByDay         map[string]float64 // "2006-01-02" → cost
	TopTools      []ToolStat
}

// CalculateCost computes an estimated USD cost from token counts and model name.
// Falls back to sonnet pricing for unknown models.
func CalculateCost(model string, input, output, cacheRead, cacheWrite int) float64 {
	p, ok := ModelPricing[model]
	if !ok {
		p = ModelPricing["claude-sonnet-4-6"]
	}
	return (float64(input)*p.InputPerM +
		float64(output)*p.OutputPerM +
		float64(cacheRead)*p.CacheReadPerM +
		float64(cacheWrite)*p.CacheWritePerM) / 1_000_000
}

// rawTurnDuration is used to extract turn_duration from system subtype messages.
type rawTurnDuration struct {
	Duration float64 `json:"duration"`
}

// ParseSessionCost reads a session JSONL file and aggregates cost data.
// It scans the file once, summing token usage from assistant messages and
// counting tool_use invocations.
func ParseSessionCost(filePath string) (*SessionCost, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := &SessionCost{
		ToolCalls: make(map[string]int),
	}

	// Derive SessionID from filename.
	sc.SessionID = strings.TrimSuffix(filepath.Base(filePath), ".jsonl")
	// Derive project from grandparent directory name.
	sc.Project = projectDirFromPath(filepath.Base(filepath.Dir(filePath)))

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

		// Capture session start time from first message.
		if sc.Timestamp.IsZero() {
			if ts := parseTimestamp(msg.Timestamp); !ts.IsZero() {
				sc.Timestamp = ts
			}
		}

		// Extract token usage and tool calls from assistant messages.
		if msg.Type == "assistant" && msg.Message != nil {
			if sc.Model == "" && msg.Message.Model != "" {
				sc.Model = msg.Message.Model
			}
			if msg.Message.Usage != nil {
				u := msg.Message.Usage
				sc.InputTokens += u.InputTokens
				sc.OutputTokens += u.OutputTokens
				sc.CacheRead += u.CacheReadInputTokens
				sc.CacheWrite += u.CacheCreationInputTokens
			}
			sc.TurnCount++

			// Count tool_use invocations.
			var blocks []rawContentBlock
			if err := json.Unmarshal(msg.Message.Content, &blocks); err == nil {
				for _, b := range blocks {
					if b.Type == "tool_use" && b.Name != "" {
						sc.ToolCalls[b.Name]++
					}
				}
			}
		}

		// Extract turn_duration from system subtype messages.
		if msg.Type == "system" {
			var dur rawTurnDuration
			if err := json.Unmarshal(msg.Content, &dur); err == nil && dur.Duration > 0 {
				sc.Duration += time.Duration(dur.Duration * float64(time.Second))
			}
		}
	}
	// Ignore scanner.Err() for truncated-line tolerance (same pattern as ParseConversationPreview).

	sc.TotalTokens = sc.InputTokens + sc.OutputTokens + sc.CacheRead + sc.CacheWrite
	sc.EstCostUSD = CalculateCost(sc.Model, sc.InputTokens, sc.OutputTokens, sc.CacheRead, sc.CacheWrite)

	return sc, nil
}

// DiscoverAllCosts scans all session JSONL files and returns cost data sorted
// by timestamp DESC. Uses 4 parallel workers matching the DiscoverSessions pattern.
func DiscoverAllCosts() ([]SessionCost, error) {
	projectsDir := filepath.Join(ClaudeDir(), "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	type work struct {
		filePath string
	}

	var jobs []work
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subDir := filepath.Join(projectsDir, entry.Name())
		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, se := range subEntries {
			if se.IsDir() || !strings.HasSuffix(se.Name(), ".jsonl") {
				continue
			}
			jobs = append(jobs, work{filePath: filepath.Join(subDir, se.Name())})
		}
	}

	const workers = 4
	jobCh := make(chan work, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	resultCh := make(chan SessionCost, len(jobs))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				c, err := ParseSessionCost(j.filePath)
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

// Summarize aggregates a slice of SessionCosts into a CostSummary.
func Summarize(costs []SessionCost) CostSummary {
	s := CostSummary{
		ByModel:   make(map[string]ModelStats),
		ByProject: make(map[string]float64),
		ByDay:     make(map[string]float64),
	}

	toolCounts := make(map[string]int)

	for _, c := range costs {
		s.TotalCostUSD += c.EstCostUSD
		s.TotalTokens += c.TotalTokens
		s.TotalSessions++
		s.TotalTurns += c.TurnCount
		s.TotalDuration += c.Duration

		// Aggregate by model.
		ms := s.ByModel[c.Model]
		ms.Tokens += c.TotalTokens
		ms.Cost += c.EstCostUSD
		ms.Turns += c.TurnCount
		s.ByModel[c.Model] = ms

		// Aggregate by project.
		s.ByProject[c.Project] += c.EstCostUSD

		// Aggregate by day.
		if !c.Timestamp.IsZero() {
			day := c.Timestamp.Format("2006-01-02")
			s.ByDay[day] += c.EstCostUSD
		}

		// Aggregate tool counts.
		for name, count := range c.ToolCalls {
			toolCounts[name] += count
		}
	}

	// Build sorted TopTools slice.
	for name, count := range toolCounts {
		s.TopTools = append(s.TopTools, ToolStat{Name: name, Count: count})
	}
	sort.Slice(s.TopTools, func(i, j int) bool {
		return s.TopTools[i].Count > s.TopTools[j].Count
	})

	return s
}

package data

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── CalculateCost ─────────────────────────────────────────────────────────────

func TestCalculateCost_KnownModel_Sonnet(t *testing.T) {
	// 1M input + 1M output for sonnet-4-6: $3.0 + $15.0 = $18.0
	got := CalculateCost("claude-sonnet-4-6", 1_000_000, 1_000_000, 0, 0)
	want := 18.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("CalculateCost(sonnet, 1M, 1M) = %.4f, want %.4f", got, want)
	}
}

func TestCalculateCost_KnownModel_Opus(t *testing.T) {
	// 500K input + 500K output for opus-4-6: ($15 * 0.5) + ($75 * 0.5) = $7.5 + $37.5 = $45.0
	got := CalculateCost("claude-opus-4-6", 500_000, 500_000, 0, 0)
	want := 45.0
	if math.Abs(got-want) > 0.001 {
		t.Errorf("CalculateCost(opus, 500K, 500K) = %.4f, want %.4f", got, want)
	}
}

func TestCalculateCost_KnownModel_Haiku(t *testing.T) {
	// 1M input + 1M output for haiku-4-5: $0.8 + $4.0 = $4.8
	got := CalculateCost("claude-haiku-4-5", 1_000_000, 1_000_000, 0, 0)
	want := 4.8
	if math.Abs(got-want) > 0.001 {
		t.Errorf("CalculateCost(haiku, 1M, 1M) = %.4f, want %.4f", got, want)
	}
}

func TestCalculateCost_WithCacheTokens(t *testing.T) {
	// Sonnet: 100K input + 0 output + 100K cache_read + 100K cache_write
	// = ($3 * 0.1) + 0 + ($0.3 * 0.1) + ($3.75 * 0.1)
	// = $0.30 + $0.03 + $0.375 = $0.705
	got := CalculateCost("claude-sonnet-4-6", 100_000, 0, 100_000, 100_000)
	want := 0.705
	if math.Abs(got-want) > 0.001 {
		t.Errorf("CalculateCost(sonnet, cache) = %.4f, want %.4f", got, want)
	}
}

func TestCalculateCost_UnknownModel_FallsBackToSonnet(t *testing.T) {
	// Unknown model should use sonnet pricing.
	known := CalculateCost("claude-sonnet-4-6", 1_000_000, 0, 0, 0)
	unknown := CalculateCost("claude-unknown-model", 1_000_000, 0, 0, 0)
	if math.Abs(known-unknown) > 0.0001 {
		t.Errorf("unknown model cost = %.4f, want %.4f (sonnet fallback)", unknown, known)
	}
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	got := CalculateCost("claude-sonnet-4-6", 0, 0, 0, 0)
	if got != 0.0 {
		t.Errorf("CalculateCost(all zero) = %.4f, want 0.0", got)
	}
}

// ── ParseSessionCost ──────────────────────────────────────────────────────────

// assistantLineWithUsage returns an assistant JSONL line with usage data and optional tool_use.
func assistantLineWithUsage(sessionID, model string, input, output, cacheRead, cacheWrite int, tools []string, ts time.Time) string {
	content := []map[string]interface{}{
		{"type": "text", "text": "response"},
	}
	for _, name := range tools {
		content = append(content, map[string]interface{}{
			"type":  "tool_use",
			"id":    "toolu_" + name,
			"name":  name,
			"input": json.RawMessage(`{}`),
		})
	}
	msg := map[string]interface{}{
		"type":      "assistant",
		"timestamp": ts.Format(time.RFC3339Nano),
		"sessionId": sessionID,
		"message": map[string]interface{}{
			"role":    "assistant",
			"model":   model,
			"content": content,
			"usage": map[string]interface{}{
				"input_tokens":                input,
				"output_tokens":               output,
				"cache_read_input_tokens":     cacheRead,
				"cache_creation_input_tokens": cacheWrite,
			},
		},
	}
	b, _ := json.Marshal(msg)
	return string(b)
}

func TestParseSessionCost_BasicAggregation(t *testing.T) {
	ts := time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	path := filepath.Join(dir, "test-cost.jsonl")

	lines := []string{
		userLine("sess-cost-1", "/dev/proj", "main", "1.0", "hello", ts),
		assistantLineWithUsage("sess-cost-1", "claude-sonnet-4-6", 1000, 500, 200, 100, []string{"Bash", "Read"}, ts.Add(time.Second)),
		assistantLineWithUsage("sess-cost-1", "claude-sonnet-4-6", 2000, 1000, 0, 0, []string{"Edit"}, ts.Add(2*time.Second)),
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sc, err := ParseSessionCost(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sc == nil {
		t.Fatal("expected non-nil SessionCost")
	}

	// Check aggregated token counts.
	if sc.InputTokens != 3000 {
		t.Errorf("InputTokens = %d, want 3000", sc.InputTokens)
	}
	if sc.OutputTokens != 1500 {
		t.Errorf("OutputTokens = %d, want 1500", sc.OutputTokens)
	}
	if sc.CacheRead != 200 {
		t.Errorf("CacheRead = %d, want 200", sc.CacheRead)
	}
	if sc.CacheWrite != 100 {
		t.Errorf("CacheWrite = %d, want 100", sc.CacheWrite)
	}
	if sc.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", sc.TurnCount)
	}
	if sc.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want 'claude-sonnet-4-6'", sc.Model)
	}

	// Verify tool counts.
	if sc.ToolCalls["Bash"] != 1 {
		t.Errorf("ToolCalls[Bash] = %d, want 1", sc.ToolCalls["Bash"])
	}
	if sc.ToolCalls["Read"] != 1 {
		t.Errorf("ToolCalls[Read] = %d, want 1", sc.ToolCalls["Read"])
	}
	if sc.ToolCalls["Edit"] != 1 {
		t.Errorf("ToolCalls[Edit] = %d, want 1", sc.ToolCalls["Edit"])
	}

	// Estimated cost must be positive.
	if sc.EstCostUSD <= 0 {
		t.Errorf("EstCostUSD = %.6f, want > 0", sc.EstCostUSD)
	}
}

func TestParseSessionCost_TotalTokensCalculated(t *testing.T) {
	ts := time.Now()
	dir := t.TempDir()
	path := filepath.Join(dir, "tokens.jsonl")

	lines := []string{
		assistantLineWithUsage("s1", "claude-sonnet-4-6", 100, 50, 10, 5, nil, ts),
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sc, err := ParseSessionCost(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantTotal := 100 + 50 + 10 + 5
	if sc.TotalTokens != wantTotal {
		t.Errorf("TotalTokens = %d, want %d", sc.TotalTokens, wantTotal)
	}
}

func TestParseSessionCost_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sc, err := ParseSessionCost(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty file should return zero values.
	if sc.InputTokens != 0 || sc.OutputTokens != 0 {
		t.Errorf("expected zero tokens for empty file, got input=%d output=%d", sc.InputTokens, sc.OutputTokens)
	}
	if sc.EstCostUSD != 0.0 {
		t.Errorf("expected zero cost for empty file, got %.6f", sc.EstCostUSD)
	}
}

func TestParseSessionCost_SessionIDFromFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "my-session-abc123.jsonl")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sc, err := ParseSessionCost(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sc.SessionID != "my-session-abc123" {
		t.Errorf("SessionID = %q, want 'my-session-abc123'", sc.SessionID)
	}
}

func TestParseSessionCost_TimestampFromFirstMessage(t *testing.T) {
	ts := time.Date(2026, 5, 17, 8, 30, 0, 0, time.UTC)
	dir := t.TempDir()
	path := filepath.Join(dir, "ts-test.jsonl")

	lines := []string{
		userLine("s1", "/tmp", "main", "1.0", "hello", ts),
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sc, err := ParseSessionCost(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sc.Timestamp.Equal(ts) {
		t.Errorf("Timestamp = %v, want %v", sc.Timestamp, ts)
	}
}

// ── Summarize ─────────────────────────────────────────────────────────────────

func makeMockSessionCosts() []SessionCost {
	ts := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	return []SessionCost{
		{
			SessionID:    "s1",
			Project:      "/Users/me/tonys-nix",
			Model:        "claude-opus-4-6",
			InputTokens:  100_000,
			OutputTokens: 50_000,
			TotalTokens:  150_000,
			EstCostUSD:   2.25,
			TurnCount:    5,
			Duration:     30 * time.Minute,
			Timestamp:    ts,
			ToolCalls:    map[string]int{"Bash": 10, "Read": 5},
		},
		{
			SessionID:    "s2",
			Project:      "/Users/me/tonys-nix",
			Model:        "claude-sonnet-4-6",
			InputTokens:  50_000,
			OutputTokens: 20_000,
			TotalTokens:  70_000,
			EstCostUSD:   0.45,
			TurnCount:    3,
			Duration:     15 * time.Minute,
			Timestamp:    ts.AddDate(0, 0, -1),
			ToolCalls:    map[string]int{"Edit": 8, "Bash": 3},
		},
		{
			SessionID:    "s3",
			Project:      "/Users/me/tonys-blog",
			Model:        "claude-sonnet-4-6",
			InputTokens:  20_000,
			OutputTokens: 10_000,
			TotalTokens:  30_000,
			EstCostUSD:   0.21,
			TurnCount:    2,
			Duration:     5 * time.Minute,
			Timestamp:    ts.AddDate(0, 0, -2),
			ToolCalls:    map[string]int{"Read": 4, "Write": 2},
		},
	}
}

func TestSummarize_TotalCost(t *testing.T) {
	costs := makeMockSessionCosts()
	s := Summarize(costs)

	wantCost := 2.25 + 0.45 + 0.21
	if math.Abs(s.TotalCostUSD-wantCost) > 0.001 {
		t.Errorf("TotalCostUSD = %.4f, want %.4f", s.TotalCostUSD, wantCost)
	}
}

func TestSummarize_TotalTokens(t *testing.T) {
	costs := makeMockSessionCosts()
	s := Summarize(costs)

	want := 150_000 + 70_000 + 30_000
	if s.TotalTokens != want {
		t.Errorf("TotalTokens = %d, want %d", s.TotalTokens, want)
	}
}

func TestSummarize_TotalSessions(t *testing.T) {
	costs := makeMockSessionCosts()
	s := Summarize(costs)

	if s.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d, want 3", s.TotalSessions)
	}
}

func TestSummarize_TotalTurns(t *testing.T) {
	costs := makeMockSessionCosts()
	s := Summarize(costs)

	if s.TotalTurns != 10 {
		t.Errorf("TotalTurns = %d, want 10", s.TotalTurns)
	}
}

func TestSummarize_TotalDuration(t *testing.T) {
	costs := makeMockSessionCosts()
	s := Summarize(costs)

	want := 50 * time.Minute
	if s.TotalDuration != want {
		t.Errorf("TotalDuration = %v, want %v", s.TotalDuration, want)
	}
}

func TestSummarize_ByModel(t *testing.T) {
	costs := makeMockSessionCosts()
	s := Summarize(costs)

	opus := s.ByModel["claude-opus-4-6"]
	if opus.Tokens != 150_000 {
		t.Errorf("ByModel[opus].Tokens = %d, want 150000", opus.Tokens)
	}
	if math.Abs(opus.Cost-2.25) > 0.001 {
		t.Errorf("ByModel[opus].Cost = %.4f, want 2.25", opus.Cost)
	}
	if opus.Turns != 5 {
		t.Errorf("ByModel[opus].Turns = %d, want 5", opus.Turns)
	}

	sonnet := s.ByModel["claude-sonnet-4-6"]
	if sonnet.Tokens != 100_000 {
		t.Errorf("ByModel[sonnet].Tokens = %d, want 100000", sonnet.Tokens)
	}
}

func TestSummarize_ByProject(t *testing.T) {
	costs := makeMockSessionCosts()
	s := Summarize(costs)

	nixCost := s.ByProject["/Users/me/tonys-nix"]
	wantNix := 2.25 + 0.45
	if math.Abs(nixCost-wantNix) > 0.001 {
		t.Errorf("ByProject[tonys-nix] = %.4f, want %.4f", nixCost, wantNix)
	}

	blogCost := s.ByProject["/Users/me/tonys-blog"]
	if math.Abs(blogCost-0.21) > 0.001 {
		t.Errorf("ByProject[tonys-blog] = %.4f, want 0.21", blogCost)
	}
}

func TestSummarize_ByDay(t *testing.T) {
	costs := makeMockSessionCosts()
	s := Summarize(costs)

	ts := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	day0 := ts.Format("2006-01-02")
	day1 := ts.AddDate(0, 0, -1).Format("2006-01-02")
	day2 := ts.AddDate(0, 0, -2).Format("2006-01-02")

	if math.Abs(s.ByDay[day0]-2.25) > 0.001 {
		t.Errorf("ByDay[%s] = %.4f, want 2.25", day0, s.ByDay[day0])
	}
	if math.Abs(s.ByDay[day1]-0.45) > 0.001 {
		t.Errorf("ByDay[%s] = %.4f, want 0.45", day1, s.ByDay[day1])
	}
	if math.Abs(s.ByDay[day2]-0.21) > 0.001 {
		t.Errorf("ByDay[%s] = %.4f, want 0.21", day2, s.ByDay[day2])
	}
}

func TestSummarize_TopTools_SortedByCount(t *testing.T) {
	costs := makeMockSessionCosts()
	s := Summarize(costs)

	// Bash: 10+3=13, Read: 5+4=9, Edit: 8, Write: 2
	if len(s.TopTools) == 0 {
		t.Fatal("expected TopTools to be populated")
	}
	if s.TopTools[0].Name != "Bash" {
		t.Errorf("TopTools[0].Name = %q, want 'Bash'", s.TopTools[0].Name)
	}
	if s.TopTools[0].Count != 13 {
		t.Errorf("TopTools[0].Count = %d, want 13", s.TopTools[0].Count)
	}
}

func TestSummarize_EmptyInput(t *testing.T) {
	s := Summarize(nil)
	if s.TotalCostUSD != 0 {
		t.Errorf("empty Summarize: TotalCostUSD = %.4f, want 0", s.TotalCostUSD)
	}
	if s.TotalSessions != 0 {
		t.Errorf("empty Summarize: TotalSessions = %d, want 0", s.TotalSessions)
	}
	if len(s.TopTools) != 0 {
		t.Errorf("empty Summarize: TopTools should be empty, got %d", len(s.TopTools))
	}
}

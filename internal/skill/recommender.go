package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/data"
)

// Recommendation represents a recommended skill with its reasoning.
type Recommendation struct {
	RepoURL   string `json:"repo_url"`
	Reasoning string `json:"reasoning"`
	Score     int    `json:"score"`
}

// GlobalContext holds environment and multi-provider session data.
type GlobalContext struct {
	CWD            string
	ProjectFiles   []string
	ProviderConfigs map[string]string // provider -> summary of config/status
	RecentSessions []data.Session
}

// Recommender handles environment analysis and skill matching.
type Recommender struct {
	cachePath string
}

func NewRecommender() *Recommender {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		// Fallback to home dir if XDG cache unresolvable.
		home, _ := os.UserHomeDir()
		cacheRoot = filepath.Join(home, ".cache")
	}
	cache := filepath.Join(cacheRoot, "tonys-agent-telemetry", "skill_recommendations.json")
	return &Recommender{cachePath: cache}
}

type cachedRecommendation struct {
	Timestamp       time.Time        `json:"timestamp"`
	WorkflowID      string           `json:"workflow_id"`
	Recommendations []Recommendation `json:"recommendations"`
}

// GetRecommendations analyzes the environment across all providers and returns suitable skills.
func (r *Recommender) GetRecommendations(ctx context.Context, cwd string) ([]Recommendation, error) {
	gCtx := r.gatherGlobalContext(cwd)
	workflowID := r.computeWorkflowID(gCtx)
	
	if cached, ok := r.readCache(workflowID); ok {
		return cached, nil
	}

	// Complex workflow: Analyzing multi-provider patterns
	recs := r.analyzeWorkflow(gCtx)
	
	r.writeCache(workflowID, recs)
	return recs, nil
}

func (r *Recommender) gatherGlobalContext(cwd string) GlobalContext {
	gCtx := GlobalContext{
		CWD:             cwd,
		ProviderConfigs: make(map[string]string),
	}

	// 1. Project files (shallow scan)
	entries, _ := os.ReadDir(cwd)
	for _, e := range entries {
		if !e.IsDir() {
			gCtx.ProjectFiles = append(gCtx.ProjectFiles, e.Name())
		}
	}

	// 2. Provider Configs
	home, _ := os.UserHomeDir()
	
	// Claude config
	if _, err := os.Stat(filepath.Join(home, ".claude", "config.json")); err == nil {
		gCtx.ProviderConfigs["claude"] = "Configured (config.json present)"
	}
	
	// Gemini config
	if _, err := os.Stat(filepath.Join(home, ".gemini", "tmp")); err == nil {
		gCtx.ProviderConfigs["gemini"] = "Active (tmp logs found)"
	}

	// Codex config
	if _, err := os.Stat(filepath.Join(home, ".codex", "sessions")); err == nil {
		gCtx.ProviderConfigs["codex"] = "Active (sessions found)"
	}

	// 3. Multi-provider Sessions (Top 20 most recent)
	allSessions, _ := data.DiscoverAllSessions()
	if len(allSessions) > 20 {
		gCtx.RecentSessions = allSessions[:20]
	} else {
		gCtx.RecentSessions = allSessions
	}

	return gCtx
}

func (r *Recommender) computeWorkflowID(gCtx GlobalContext) string {
	// ID includes CWD, providers count, and latest session ID
	latestID := "none"
	if len(gCtx.RecentSessions) > 0 {
		latestID = gCtx.RecentSessions[0].ID
	}
	return fmt.Sprintf("%s:%d:%s", gCtx.CWD, len(gCtx.ProviderConfigs), latestID)
}

func (r *Recommender) analyzeWorkflow(gCtx GlobalContext) []Recommendation {
	var recs []Recommendation
	
	// 1. Identify "The Power User" (Multi-provider usage)
	if len(gCtx.ProviderConfigs) >= 2 {
		recs = append(recs, Recommendation{
			RepoURL:   "",  // TODO: pending awesome-skills registry — was: vanillacake369/awesome-skills/agent-orchestration",
			Reasoning: fmt.Sprintf("You are using %d different AI agents (Claude, Gemini, etc.). This skill set optimizes multi-agent coordination and context sharing.", len(gCtx.ProviderConfigs)),
			Score:     95,
		})
	}

	// 2. Project Analysis
	hasNix := false
	hasGo := false
	for _, f := range gCtx.ProjectFiles {
		if strings.HasSuffix(f, ".nix") {
			hasNix = true
		}
		if f == "go.mod" {
			hasGo = true
		}
	}

	if hasNix && hasGo {
		recs = append(recs, Recommendation{
			RepoURL:   "",  // TODO: pending awesome-skills registry — was: vanillacake369/awesome-skills/nix-go",
			Reasoning: "Detected a Go project in a Nix environment. These skills provide optimized devshells and hermetic build workflows.",
			Score:     90,
		})
	}

	// 3. Workflow Pattern Extraction from SearchText
	combinedText := ""
	for _, s := range gCtx.RecentSessions {
		combinedText += " " + s.SearchText
	}
	combinedText = strings.ToLower(combinedText)

	if strings.Contains(combinedText, "k8s") || strings.Contains(combinedText, "kubernetes") {
		recs = append(recs, Recommendation{
			RepoURL:   "",  // TODO: pending awesome-skills registry — was: vanillacake369/awesome-skills/k8s-operator",
			Reasoning: "Your conversation history across providers shows heavy focus on Kubernetes. These skills help with CRD management and controller patterns.",
			Score:     85,
		})
	}

	if strings.Contains(combinedText, "tui") || strings.Contains(combinedText, "bubbletea") {
		recs = append(recs, Recommendation{
			RepoURL:   "",  // TODO: pending awesome-skills registry — was: vanillacake369/awesome-skills/tui-design",
			Reasoning: "Frequent mentions of TUI and Bubble Tea detected. This repository contains advanced components for rich terminal interfaces.",
			Score:     88,
		})
	}

	return recs
}

func (r *Recommender) readCache(id string) ([]Recommendation, bool) {
	data, err := os.ReadFile(r.cachePath)
	if err != nil {
		return nil, false
	}
	var cache map[string]cachedRecommendation
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, false
	}
	entry, ok := cache[id]
	if !ok || time.Since(entry.Timestamp) > 12*time.Hour { // Faster expiry for global context
		return nil, false
	}
	return entry.Recommendations, true
}

func (r *Recommender) writeCache(id string, recs []Recommendation) {
	_ = os.MkdirAll(filepath.Dir(r.cachePath), 0755)
	
	var cache map[string]cachedRecommendation
	data, err := os.ReadFile(r.cachePath)
	if err == nil {
		_ = json.Unmarshal(data, &cache)
	}
	if cache == nil {
		cache = make(map[string]cachedRecommendation)
	}
	
	cache[id] = cachedRecommendation{
		Timestamp:       time.Now(),
		WorkflowID:      id,
		Recommendations: recs,
	}
	
	newData, _ := json.MarshalIndent(cache, "", "  ")
	_ = os.WriteFile(r.cachePath, newData, 0644)
}

package control

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// Budget holds accumulated token usage and cost for one session.
type Budget struct {
	SessionID    string    `json:"session_id"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BudgetStore persists budget state to a JSON file with cross-process flock safety.
type BudgetStore struct {
	path string
	mu   sync.Mutex
}

// NewBudgetStore creates a BudgetStore that uses cacheDir/budgets.json.
func NewBudgetStore(cacheDir string) *BudgetStore {
	return &BudgetStore{
		path: filepath.Join(cacheDir, "budgets.json"),
	}
}

// cacheDir returns the canonical cache directory, respecting XDG_CACHE_HOME.
func CacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return os.TempDir()
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "tonys-agent-telemetry")
}

// Get reads the current budget for a session. Returns zero Budget if not found.
func (s *BudgetStore) Get(sessionID string) (Budget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	budgets, err := s.readLocked()
	if err != nil {
		return Budget{}, err
	}
	b, ok := budgets[sessionID]
	if !ok {
		return Budget{SessionID: sessionID}, nil
	}
	return b, nil
}

// Add credits the session with token usage and recomputes USD cost.
// Atomic across processes via flock.
func (s *BudgetStore) Add(sessionID, model string, inputTokens, outputTokens int, pricing map[string]ModelPrice) (Budget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fl := flock.New(s.path + ".lock")
	if err := fl.Lock(); err != nil {
		return Budget{}, fmt.Errorf("budget flock: %w", err)
	}
	defer fl.Unlock()

	budgets, err := s.readNoLock()
	if err != nil {
		return Budget{}, err
	}

	b := budgets[sessionID]
	b.SessionID = sessionID
	b.InputTokens += inputTokens
	b.OutputTokens += outputTokens

	if p, ok := pricing[model]; ok {
		b.CostUSD += float64(inputTokens)*p.Input/1_000_000 + float64(outputTokens)*p.Output/1_000_000
	}
	b.UpdatedAt = time.Now().UTC()

	budgets[sessionID] = b

	if err := s.writeNoLock(budgets); err != nil {
		return Budget{}, err
	}
	return b, nil
}

// DailyTotal returns the sum of cost_usd for all sessions updated today (UTC).
func (s *BudgetStore) DailyTotal() (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	budgets, err := s.readLocked()
	if err != nil {
		return 0, err
	}

	today := time.Now().UTC().Format("2006-01-02")
	var total float64
	for _, b := range budgets {
		if b.UpdatedAt.UTC().Format("2006-01-02") == today {
			total += b.CostUSD
		}
	}
	return total, nil
}

// All returns all current sessions (for TUI display).
func (s *BudgetStore) All() ([]Budget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	budgets, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	out := make([]Budget, 0, len(budgets))
	for _, b := range budgets {
		out = append(out, b)
	}
	return out, nil
}

// readLocked reads the budget file without acquiring the process-level mutex
// (caller must hold it). Uses flock for cross-process safety.
func (s *BudgetStore) readLocked() (map[string]Budget, error) {
	fl := flock.New(s.path + ".lock")
	if err := fl.RLock(); err != nil {
		return nil, fmt.Errorf("budget flock read: %w", err)
	}
	defer fl.Unlock()
	return s.readNoLock()
}

// readNoLock reads budgets.json without any locking (caller responsible).
func (s *BudgetStore) readNoLock() (map[string]Budget, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return map[string]Budget{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read budgets.json: %w", err)
	}
	var budgets map[string]Budget
	if err := json.Unmarshal(data, &budgets); err != nil {
		return nil, fmt.Errorf("parse budgets.json: %w", err)
	}
	return budgets, nil
}

// writeNoLock writes budgets.json without any locking (caller responsible).
func (s *BudgetStore) writeNoLock(budgets map[string]Budget) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("mkdir budgets dir: %w", err)
	}
	data, err := json.MarshalIndent(budgets, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal budgets: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("write budgets.json: %w", err)
	}
	return nil
}

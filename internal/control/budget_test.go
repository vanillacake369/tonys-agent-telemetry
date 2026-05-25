package control

import (
	"sync"
	"testing"
	"time"
)

func testPricing() map[string]ModelPrice {
	return map[string]ModelPrice{
		"claude-sonnet-4-6": {Input: 3.0, Output: 15.0},
	}
}

func TestBudgetStore_AddAccumulates(t *testing.T) {
	dir := t.TempDir()
	store := NewBudgetStore(dir)
	pricing := testPricing()

	b1, err := store.Add("sess-1", "claude-sonnet-4-6", 1000, 500, pricing)
	if err != nil {
		t.Fatalf("first Add: %v", err)
	}
	if b1.InputTokens != 1000 || b1.OutputTokens != 500 {
		t.Errorf("after first Add: tokens = %d/%d, want 1000/500", b1.InputTokens, b1.OutputTokens)
	}

	b2, err := store.Add("sess-1", "claude-sonnet-4-6", 2000, 1000, pricing)
	if err != nil {
		t.Fatalf("second Add: %v", err)
	}
	if b2.InputTokens != 3000 || b2.OutputTokens != 1500 {
		t.Errorf("after second Add: tokens = %d/%d, want 3000/1500", b2.InputTokens, b2.OutputTokens)
	}

	// cost = (3000 * 3.0 + 1500 * 15.0) / 1_000_000 = (9000 + 22500) / 1_000_000 = 0.0315
	want := (3000*3.0 + 1500*15.0) / 1_000_000
	if diff := b2.CostUSD - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("CostUSD = %v, want %v", b2.CostUSD, want)
	}
}

func TestBudgetStore_AddNewSession(t *testing.T) {
	dir := t.TempDir()
	store := NewBudgetStore(dir)

	got, err := store.Get("brand-new-session")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.InputTokens != 0 || got.OutputTokens != 0 || got.CostUSD != 0 {
		t.Errorf("new session not zeroed: %+v", got)
	}
}

func TestBudgetStore_DailyTotalRespectsUTCDate(t *testing.T) {
	dir := t.TempDir()
	store := NewBudgetStore(dir)
	pricing := testPricing()

	// Add a session today.
	_, err := store.Add("today-session", "claude-sonnet-4-6", 1000, 500, pricing)
	if err != nil {
		t.Fatalf("Add today: %v", err)
	}

	// Manually inject a session from yesterday into the file.
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	budgets, err := store.readNoLock()
	if err != nil {
		t.Fatal(err)
	}
	budgets["yesterday-session"] = Budget{
		SessionID:    "yesterday-session",
		InputTokens:  1000,
		OutputTokens: 500,
		CostUSD:      0.01,
		UpdatedAt:    yesterday,
	}
	if err := store.writeNoLock(budgets); err != nil {
		t.Fatal(err)
	}

	total, err := store.DailyTotal()
	if err != nil {
		t.Fatalf("DailyTotal: %v", err)
	}

	todayCost := (1000*3.0 + 500*15.0) / 1_000_000
	if diff := total - todayCost; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("DailyTotal = %v, want %v (yesterday excluded)", total, todayCost)
	}
}

func TestBudgetStore_ConcurrentAddSafe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewBudgetStore(dir)
	pricing := testPricing()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := store.Add("concurrent-session", "claude-sonnet-4-6", 100, 50, pricing)
			if err != nil {
				t.Errorf("concurrent Add: %v", err)
			}
		}()
	}
	wg.Wait()

	final, err := store.Get("concurrent-session")
	if err != nil {
		t.Fatalf("Get after concurrent Adds: %v", err)
	}
	if final.InputTokens != goroutines*100 {
		t.Errorf("InputTokens = %d, want %d", final.InputTokens, goroutines*100)
	}
	if final.OutputTokens != goroutines*50 {
		t.Errorf("OutputTokens = %d, want %d", final.OutputTokens, goroutines*50)
	}

	wantCost := float64(goroutines) * (100*3.0 + 50*15.0) / 1_000_000
	if diff := final.CostUSD - wantCost; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("CostUSD = %v, want %v", final.CostUSD, wantCost)
	}
}

func TestBudgetStore_UnknownModelUsesZeroPricing(t *testing.T) {
	dir := t.TempDir()
	store := NewBudgetStore(dir)
	pricing := testPricing()

	b, err := store.Add("sess-unknown", "claude-unknown-model", 5000, 2000, pricing)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if b.InputTokens != 5000 || b.OutputTokens != 2000 {
		t.Errorf("tokens: got %d/%d, want 5000/2000", b.InputTokens, b.OutputTokens)
	}
	if b.CostUSD != 0 {
		t.Errorf("CostUSD = %v, want 0 for unknown model", b.CostUSD)
	}
}

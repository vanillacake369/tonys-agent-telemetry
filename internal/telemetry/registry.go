package telemetry

import (
	"context"
	"sync"
)

// Registry holds a list of registered ProviderIngestors and dispatches
// detection + ingestion in registration order (which doubles as priority for
// port-collision resolution: register the more-specific detector first).
type Registry struct {
	mu       sync.RWMutex
	all      []ProviderIngestor
	detected []ProviderIngestor
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds an ingestor to the registry. Must be called before StartAll.
func (r *Registry) Register(i ProviderIngestor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.all = append(r.all, i)
}

// StartAll runs Detect on every registered ingestor in registration order.
// For each one that reports detected, launches its Ingest in a goroutine
// writing to out. Returns when all Detect calls have completed; Ingest
// goroutines run until ctx is cancelled.
func (r *Registry) StartAll(ctx context.Context, out chan<- Span) {
	r.mu.Lock()
	candidates := append([]ProviderIngestor(nil), r.all...)
	r.mu.Unlock()

	var detected []ProviderIngestor
	for _, ing := range candidates {
		if ing.Detect(ctx) {
			detected = append(detected, ing)
			go func(i ProviderIngestor) { _ = i.Ingest(ctx, out) }(ing)
		}
	}

	r.mu.Lock()
	r.detected = detected
	r.mu.Unlock()
}

// Detected returns a snapshot of currently active ingestors (those for which
// Detect returned true in the most recent StartAll).
func (r *Registry) Detected() []ProviderIngestor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]ProviderIngestor(nil), r.detected...)
}

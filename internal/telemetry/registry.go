package telemetry

import (
	"context"
	"sync"
)

type Registry struct {
	mu       sync.RWMutex
	all      []ProviderIngestor
	detected []ProviderIngestor
}

func NewRegistry() *Registry { return &Registry{} }

// Register adds an ingestor. Call before StartAll.
func (r *Registry) Register(i ProviderIngestor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.all = append(r.all, i)
}

// StartAll runs Detect on each registered ingestor (sequential, in registration
// order, which is the priority order for collision resolution). For each
// detected ingestor, launches Ingest in a goroutine. Returns when all detection
// completes; Ingest goroutines run until ctx is cancelled.
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

// Detected returns the providers currently active.
func (r *Registry) Detected() []ProviderIngestor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]ProviderIngestor(nil), r.detected...)
}

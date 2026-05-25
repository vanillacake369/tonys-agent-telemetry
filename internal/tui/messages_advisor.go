package tui

import (
	"github.com/vanillacake369/tonys-agent-telemetry/internal/recommender"
)

// RecommendationsReadyMsg carries Recommender output to the Skills tab's
// Advisor pane. Phase ι owns the production side (signal.Extract →
// recommender.Engine.Recommend → this Msg) and consumption (Skills tab).
type RecommendationsReadyMsg struct {
	Recommendations []recommender.Recommendation
}

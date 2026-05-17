package data

// ProviderName identifies an AI agent provider.
type ProviderName string

const (
	ProviderClaude ProviderName = "claude"
	ProviderCodex  ProviderName = "codex"
	ProviderGemini ProviderName = "gemini"
)

// Provider is the abstraction for discovering and parsing session data
// from different AI agent CLI tools (Claude, Codex, Gemini).
type Provider interface {
	// Name returns the provider identifier.
	Name() ProviderName

	// DataDir returns the root data directory for this provider.
	DataDir() string

	// Available reports whether this provider's data directory exists.
	Available() bool

	// DiscoverSessions finds all sessions stored by this provider.
	DiscoverSessions() ([]Session, error)

	// DiscoverCosts scans all sessions and returns cost data.
	DiscoverCosts() ([]SessionCost, error)

	// ParseConversationPreview extracts the first maxTurns from a session file.
	ParseConversationPreview(filePath string, maxTurns int) ([]Turn, error)

	// ParseFullConversation returns all turns with full content for the detail view.
	ParseFullConversation(filePath string) ([]DetailTurn, error)

	// ParseFileChanges extracts file paths from tool_use events.
	ParseFileChanges(filePath string) ([]FileChange, error)

	// ResumeCommand returns the shell command to resume a session.
	ResumeCommand(session Session) string
}

// registry holds all registered providers.
var registry []Provider

// RegisterProvider adds a provider to the global registry.
func RegisterProvider(p Provider) {
	registry = append(registry, p)
}

// Providers returns all registered providers.
func Providers() []Provider {
	return registry
}

// AvailableProviders returns only providers whose data directories exist.
func AvailableProviders() []Provider {
	var result []Provider
	for _, p := range registry {
		if p.Available() {
			result = append(result, p)
		}
	}
	return result
}

// DiscoverAllSessions aggregates sessions from all available providers,
// sorted by timestamp DESC.
func DiscoverAllSessions() ([]Session, error) {
	var all []Session
	for _, p := range AvailableProviders() {
		sessions, err := p.DiscoverSessions()
		if err != nil {
			continue
		}
		all = append(all, sessions...)
	}
	sortSessionsByTime(all)
	return all, nil
}

// DiscoverAllCostsMulti aggregates costs from all available providers,
// sorted by timestamp DESC.
func DiscoverAllCostsMulti() ([]SessionCost, error) {
	var all []SessionCost
	for _, p := range AvailableProviders() {
		costs, err := p.DiscoverCosts()
		if err != nil {
			continue
		}
		all = append(all, costs...)
	}
	sortCostsByTime(all)
	return all, nil
}

// GetProvider returns the provider for the given name, or nil if not found.
func GetProvider(name ProviderName) Provider {
	for _, p := range registry {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// sortSessionsByTime sorts sessions by timestamp DESC.
func sortSessionsByTime(sessions []Session) {
	for i := 1; i < len(sessions); i++ {
		for j := i; j > 0 && sessions[j].Timestamp.After(sessions[j-1].Timestamp); j-- {
			sessions[j], sessions[j-1] = sessions[j-1], sessions[j]
		}
	}
}

// sortCostsByTime sorts costs by timestamp DESC.
func sortCostsByTime(costs []SessionCost) {
	for i := 1; i < len(costs); i++ {
		for j := i; j > 0 && costs[j].Timestamp.After(costs[j-1].Timestamp); j-- {
			costs[j], costs[j-1] = costs[j-1], costs[j]
		}
	}
}

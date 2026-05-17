package data

import (
	"fmt"
	"os"
)

// ClaudeProvider implements Provider for Claude Code (~/.claude).
type ClaudeProvider struct{}

func init() {
	RegisterProvider(&ClaudeProvider{})
}

func (p *ClaudeProvider) Name() ProviderName { return ProviderClaude }

func (p *ClaudeProvider) DataDir() string { return ClaudeDir() }

func (p *ClaudeProvider) Available() bool {
	_, err := os.Stat(p.DataDir())
	return err == nil
}

func (p *ClaudeProvider) DiscoverSessions() ([]Session, error) {
	sessions, err := DiscoverSessions()
	if err != nil {
		return nil, err
	}
	for i := range sessions {
		sessions[i].Provider = ProviderClaude
	}
	return sessions, nil
}

func (p *ClaudeProvider) DiscoverCosts() ([]SessionCost, error) {
	costs, err := DiscoverAllCosts()
	if err != nil {
		return nil, err
	}
	for i := range costs {
		costs[i].Provider = ProviderClaude
	}
	return costs, nil
}

func (p *ClaudeProvider) ParseConversationPreview(filePath string, maxTurns int) ([]Turn, error) {
	return ParseConversationPreview(filePath, maxTurns)
}

func (p *ClaudeProvider) ParseFullConversation(filePath string) ([]DetailTurn, error) {
	return ParseFullConversation(filePath)
}

func (p *ClaudeProvider) ParseFileChanges(filePath string) ([]FileChange, error) {
	return ParseFileChanges(filePath)
}

func (p *ClaudeProvider) ResumeCommand(session Session) string {
	if session.CWD != "" {
		return fmt.Sprintf("cd %q && claude --resume %s", session.CWD, session.ID)
	}
	return fmt.Sprintf("claude --resume %s", session.ID)
}

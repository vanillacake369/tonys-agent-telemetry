package data

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverAgents finds all configured agents from ~/.claude/agents/*.md files.
// It parses YAML frontmatter for name, model, and description fields.
// Falls back gracefully: missing fields are left as zero values.
func DiscoverAgents() ([]Agent, error) {
	return discoverAgentsInDir(filepath.Join(ClaudeDir(), "agents"))
}

// discoverAgentsInDir finds agents in the given directory.
// This is the testable core of DiscoverAgents.
func discoverAgentsInDir(agentsDir string) ([]Agent, error) {
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, err
	}

	var agents []Agent
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(agentsDir, entry.Name())
		agent, err := parseAgentFile(path)
		if err != nil {
			continue
		}
		// Derive name from filename if not set in frontmatter.
		if agent.Name == "" {
			agent.Name = strings.TrimSuffix(entry.Name(), ".md")
		}
		agents = append(agents, *agent)
	}

	return agents, nil
}

// parseAgentFile reads a Claude agent markdown file and extracts frontmatter fields.
// The frontmatter is delimited by "---" lines and contains YAML key: value pairs.
func parseAgentFile(path string) (*Agent, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	agent := &Agent{}
	scanner := bufio.NewScanner(f)

	// Read first line — must be "---" to have frontmatter.
	if !scanner.Scan() {
		return agent, nil
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return agent, nil
	}

	// Read until closing "---".
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		key, value, ok := parseFrontmatterLine(line)
		if !ok {
			continue
		}
		switch key {
		case "name":
			agent.Name = value
		case "description":
			agent.Description = value
		case "model":
			agent.Model = value
		}
	}

	return agent, scanner.Err()
}

// parseFrontmatterLine parses a single "key: value" YAML line.
// Returns ok=false if the line is not a simple key-value pair.
func parseFrontmatterLine(line string) (key, value string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])
	// Remove surrounding quotes if present.
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	return key, value, true
}

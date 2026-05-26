package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverAgents_NonEmpty(t *testing.T) {
	// Integration test: reads real ~/.claude/agents/ directory.
	agentsDir := filepath.Join(ClaudeDir(), "agents")
	if _, err := os.Stat(agentsDir); err != nil {
		t.Skipf("~/.claude/agents not found: %v", err)
	}

	agents, err := DiscoverAgents()
	if err != nil {
		t.Fatalf("DiscoverAgents: %v", err)
	}
	if len(agents) == 0 {
		t.Error("expected at least one agent, got 0")
	}
}

func TestDiscoverAgents_ParsesFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := `---
name: my-agent
description: A useful agent for testing
model: sonnet
color: blue
---

Agent body content here.
`
	if err := os.WriteFile(filepath.Join(dir, "my-agent.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	agents, err := discoverAgentsInDir(dir)
	if err != nil {
		t.Fatalf("discoverAgentsInDir: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	a := agents[0]
	if a.Name != "my-agent" {
		t.Errorf("Name = %q, want %q", a.Name, "my-agent")
	}
	if a.Description != "A useful agent for testing" {
		t.Errorf("Description = %q, want %q", a.Description, "A useful agent for testing")
	}
	if a.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", a.Model, "sonnet")
	}
}

func TestDiscoverAgents_FallbackNameFromFilename(t *testing.T) {
	dir := t.TempDir()
	// Agent file without a name in frontmatter.
	content := `---
description: An agent without a name in frontmatter
model: opus
---
Body.
`
	if err := os.WriteFile(filepath.Join(dir, "unnamed-agent.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	agents, err := discoverAgentsInDir(dir)
	if err != nil {
		t.Fatalf("discoverAgentsInDir: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "unnamed-agent" {
		t.Errorf("Name = %q, want %q (derived from filename)", agents[0].Name, "unnamed-agent")
	}
}

func TestDiscoverAgents_SkipsNonMdFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	mdContent := "---\nname: real-agent\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "real-agent.md"), []byte(mdContent), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	agents, err := discoverAgentsInDir(dir)
	if err != nil {
		t.Fatalf("discoverAgentsInDir: %v", err)
	}
	if len(agents) != 1 {
		t.Errorf("expected 1 agent (skipping config.json), got %d", len(agents))
	}
}

func TestDiscoverAgents_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	// File with no frontmatter delimiters.
	content := "Just plain markdown content without frontmatter.\n"
	if err := os.WriteFile(filepath.Join(dir, "plain.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	agents, err := discoverAgentsInDir(dir)
	if err != nil {
		t.Fatalf("discoverAgentsInDir: %v", err)
	}
	// Should still return an agent with name derived from filename.
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Name != "plain" {
		t.Errorf("Name = %q, want %q", agents[0].Name, "plain")
	}
}

func TestDiscoverAgents_MultipleAgents(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"architect.md":   "---\nname: architect\ndescription: Plans things\nmodel: sonnet\n---\n",
		"implementer.md": "---\nname: implementer\ndescription: Implements things\nmodel: opus\n---\n",
		"reviewer.md":    "---\nname: reviewer\ndescription: Reviews things\n---\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}

	agents, err := discoverAgentsInDir(dir)
	if err != nil {
		t.Fatalf("discoverAgentsInDir: %v", err)
	}
	if len(agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(agents))
	}
}

func TestDiscoverAgents_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	agents, err := discoverAgentsInDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("expected 0 agents in empty dir, got %d", len(agents))
	}
}

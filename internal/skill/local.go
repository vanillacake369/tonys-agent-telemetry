package skill

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/platform"
)

// ScanLocal reads ~/.claude/skills/*/SKILL.md and ~/.claude/commands/*.md
// and returns them as Skill structs with Source=SourceLocal.
func ScanLocal() ([]Skill, error) {
	claudeDir := platform.ClaudeDir()

	var skills []Skill

	// Scan ~/.claude/skills/*/SKILL.md
	skillsDir := filepath.Join(claudeDir, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillFile := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
			if info, err := os.Stat(skillFile); err == nil {
				desc := readFirstLine(skillFile)
				skills = append(skills, Skill{
					Name:      entry.Name(),
					Source:    SourceLocal,
					UpdatedAt: info.ModTime(),
					CreatedAt: info.ModTime(),
					Description: desc,
				})
			}
		}
	}

	// Scan ~/.claude/commands/*.md
	commandsDir := filepath.Join(claudeDir, "commands")
	if entries, err := os.ReadDir(commandsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			fullPath := filepath.Join(commandsDir, name)
			info, err := entry.Info()
			if err != nil {
				continue
			}
			baseName := strings.TrimSuffix(name, ".md")
			desc := readFirstLine(fullPath)
			skills = append(skills, Skill{
				Name:        baseName,
				Source:      SourceLocal,
				UpdatedAt:   info.ModTime(),
				CreatedAt:   info.ModTime(),
				Description: desc,
			})
		}
	}

	return skills, nil
}

// readFirstLine reads the first non-empty, non-heading line from a file
// to use as a short description.
func readFirstLine(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > 100 {
			line = line[:97] + "..."
		}
		return line
	}
	// Fall back to the first heading if no plain text found.
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			line = strings.TrimLeft(line, "# ")
			if len(line) > 100 {
				line = line[:97] + "..."
			}
			return line
		}
	}
	return ""
}

// skillsDir returns ~/.claude/skills for testing overrides.
var _ = time.Now // keep time import used via Skill.CreatedAt

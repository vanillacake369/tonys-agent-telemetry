package skill

import (
	"os"
	"path/filepath"
	"strings"

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

// readFirstLine extracts a short description from a Markdown file.
// If the file begins with YAML frontmatter (delimited by "---"), the value of
// the "description:" key is returned. Otherwise the first non-empty,
// non-heading body line is used. Falls back to the first heading text.
func readFirstLine(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(raw)
	lines := strings.Split(content, "\n")

	// Check for YAML frontmatter starting with "---".
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			// Closing delimiter — stop scanning frontmatter.
			if line == "---" {
				break
			}
			// Look for a "description:" key.
			if strings.HasPrefix(line, "description:") {
				value := strings.TrimSpace(line[len("description:"):])
				// Strip surrounding quotes if present.
				if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
					value = value[1 : len(value)-1]
				} else if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
					value = value[1 : len(value)-1]
				}
				if len(value) > 100 {
					value = value[:97] + "..."
				}
				return value
			}
		}
	}

	// No frontmatter description — fall back to first non-empty, non-heading body line.
	inFrontmatter := strings.TrimSpace(lines[0]) == "---"
	pastFrontmatter := !inFrontmatter
	frontmatterClosed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			continue
		}
		if inFrontmatter && !frontmatterClosed {
			if trimmed == "---" {
				frontmatterClosed = true
				pastFrontmatter = true
			}
			continue
		}
		if !pastFrontmatter {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(trimmed) > 100 {
			trimmed = trimmed[:97] + "..."
		}
		return trimmed
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


package catalog

import (
	"regexp"
	"strings"
)

// reSectionHeader matches a category section header like "### Skills (64)".
// Capture group 1 = category name (e.g. "Skills"), group 2 = count (ignored).
var reSectionHeader = regexp.MustCompile(`^###\s+(\w[\w\s-]*?)\s+\(\d+\)$`)

// reEntryHeader matches a linked catalog entry line:
//
//	- **[title](path/to/file.md)** *complexity* • time
//
// Capture group 1 = title (slug), group 2 = path relative to examples/.
var reEntryHeader = regexp.MustCompile(`^-\s+\*\*\[([^\]]+)\]\(([^)]+)\)\*\*`)

// reDescriptionLine matches a description continuation line (two-space indent).
var reDescriptionLine = regexp.MustCompile(`^\s{2,}(.+)$`)

// sectionToItemType maps upstream CATALOG.md section names to our ItemType constants.
// "Commands" and "Workflows" and "Scripts" all map to ItemTypeTemplate because they
// represent reusable prompt-templates rather than agent or hook entries.
// Any unrecognised section name is skipped entirely.
var sectionToItemType = map[string]ItemType{
	"agents":    ItemTypeAgent,
	"commands":  ItemTypeTemplate,
	"skills":    ItemTypeSkill,
	"hooks":     ItemTypeHook,
	"workflows": ItemTypeTemplate,
	"scripts":   ItemTypeTemplate,
}

// sectionNameToType normalises a raw section header name to a known ItemType.
// Returns "", false if the section should be skipped.
func sectionNameToType(rawName string) (ItemType, bool) {
	lower := strings.ToLower(strings.TrimSpace(rawName))
	t, ok := sectionToItemType[lower]
	return t, ok
}

// slugFromPath derives a stable slug from a relative file path by stripping
// the leading directory component(s) and the file extension, then trimming
// well-known suffixes (README, SKILL). This keeps IDs readable and comparable
// across cache refreshes for the same upstream file.
//
// Examples:
//
//	"agents/adr-writer.md"            → "adr-writer"
//	"skills/design-patterns/SKILL.md" → "design-patterns"
//	"hooks/bash/auto-checkpoint.sh"   → "auto-checkpoint"
func slugFromPath(path string) string {
	// Strip the section-category prefix (first path component is the section dir).
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return strings.ToLower(strings.TrimSpace(path))
	}

	// For multi-level paths, use the deepest meaningful segment:
	// - If the last segment is a well-known stub (SKILL.md, README.md, index.md),
	//   use the parent directory name instead.
	last := parts[len(parts)-1]
	// Strip extension.
	if dotIdx := strings.LastIndex(last, "."); dotIdx > 0 {
		last = last[:dotIdx]
	}

	stubs := map[string]bool{
		"SKILL":   true,
		"README":  true,
		"INDEX":   true,
		"CATALOG": true,
	}
	if stubs[last] && len(parts) >= 2 {
		// Fall back to parent directory name.
		last = parts[len(parts)-2]
	}

	return last
}

// buildSourceURL constructs the canonical upstream GitHub URL for a relative path.
// path is relative to the examples/ directory within the upstream repository.
func buildSourceURL(relPath string) string {
	const base = "https://github.com/FlorianBruniaux/claude-code-ultimate-guide/blob/main/examples/"
	return base + relPath
}

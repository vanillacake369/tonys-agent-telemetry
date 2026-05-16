package skill

import (
	"os"
	"path/filepath"
	"testing"
)

// scanLocalDir is a testable variant of ScanLocal that accepts a root directory
// instead of always using ~/.claude.
func scanLocalDir(claudeDir string) ([]Skill, error) {
	var skills []Skill

	// Scan {claudeDir}/skills/*/SKILL.md
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
					Name:        entry.Name(),
					Source:      SourceLocal,
					UpdatedAt:   info.ModTime(),
					CreatedAt:   info.ModTime(),
					Description: desc,
				})
			}
		}
	}

	// Scan {claudeDir}/commands/*.md
	commandsDir := filepath.Join(claudeDir, "commands")
	if entries, err := os.ReadDir(commandsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if len(name) < 4 || name[len(name)-3:] != ".md" {
				continue
			}
			fullPath := filepath.Join(commandsDir, name)
			info, err := entry.Info()
			if err != nil {
				continue
			}
			baseName := name[:len(name)-3]
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

func TestScanLocal_WithMockDirectory(t *testing.T) {
	// Create a temporary directory structure.
	root := t.TempDir()

	// Create skills/deploy/SKILL.md
	skillsDir := filepath.Join(root, "skills", "deploy")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"),
		[]byte("# Deploy Skill\n\nDeploys services automatically.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create skills/review/SKILL.md
	reviewDir := filepath.Join(root, "skills", "review")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reviewDir, "SKILL.md"),
		[]byte("# Review Skill\n\nReviews pull requests.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create commands/commit.md
	commandsDir := filepath.Join(root, "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "commit.md"),
		[]byte("Smart git commit command.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	skills, err := scanLocalDir(root)
	if err != nil {
		t.Fatalf("scanLocalDir: %v", err)
	}

	if len(skills) < 3 {
		t.Errorf("got %d skills, want at least 3", len(skills))
	}

	// Verify all are local.
	for _, s := range skills {
		if s.Source != SourceLocal {
			t.Errorf("skill %q has source %q, want %q", s.Name, s.Source, SourceLocal)
		}
	}

	// Verify names.
	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}
	for _, want := range []string{"deploy", "review", "commit"} {
		if !names[want] {
			t.Errorf("expected skill %q in results, got: %v", want, skills)
		}
	}
}

func TestScanLocal_EmptyDirectory_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	// No subdirectories created — ScanLocal should return empty, not error.
	skills, err := scanLocalDir(root)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestScanLocal_SkillsWithDescription(t *testing.T) {
	root := t.TempDir()

	skillDir := filepath.Join(root, "skills", "scaffold")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	content := "# Scaffold\n\nGenerates working skeleton code.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	skills, err := scanLocalDir(root)
	if err != nil {
		t.Fatalf("scanLocalDir: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Description != "Generates working skeleton code." {
		t.Errorf("description = %q, want %q", skills[0].Description, "Generates working skeleton code.")
	}
}

func TestScanLocal_FrontmatterDescription(t *testing.T) {
	root := t.TempDir()

	// Skill with YAML frontmatter description.
	skillDir := filepath.Join(root, "skills", "planning")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	skillContent := "---\ndescription: \"Create detailed technical plans\"\n---\n# architectural-planning\nUse when designing features...\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Command with YAML frontmatter description.
	commandsDir := filepath.Join(root, "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	commandContent := "---\ndescription: \"코드 품질, 성능, 보안을 분석하고 개선한다\"\n---\n$ARGUMENTS\n...\n"
	if err := os.WriteFile(filepath.Join(commandsDir, "enhance.md"), []byte(commandContent), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	skills, err := scanLocalDir(root)
	if err != nil {
		t.Fatalf("scanLocalDir: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}

	byName := make(map[string]string)
	for _, s := range skills {
		byName[s.Name] = s.Description
	}

	if byName["planning"] != "Create detailed technical plans" {
		t.Errorf("planning description = %q, want %q", byName["planning"], "Create detailed technical plans")
	}
	if byName["enhance"] != "코드 품질, 성능, 보안을 분석하고 개선한다" {
		t.Errorf("enhance description = %q, want %q", byName["enhance"], "코드 품질, 성능, 보안을 분석하고 개선한다")
	}
}

func TestReadFirstLine_NoDashDash(t *testing.T) {
	// Verifies that "---" delimiters are NOT returned as descriptions.
	f := filepath.Join(t.TempDir(), "test.md")
	content := "---\ndescription: \"my description\"\n---\n# Title\nSome body text.\n"
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got := readFirstLine(f)
	if got == "---" {
		t.Error("readFirstLine returned \"---\" — frontmatter delimiter leaked as description")
	}
	if got != "my description" {
		t.Errorf("readFirstLine = %q, want %q", got, "my description")
	}
}

func TestScanLocal_RealClaudeDir_NonFatal(t *testing.T) {
	// This test calls the real ScanLocal() — it is non-fatal because ~/.claude
	// may not have skills/ or commands/ on CI.
	skills, err := ScanLocal()
	if err != nil {
		t.Logf("ScanLocal returned error (may be expected): %v", err)
	}
	// If the real directory exists and has content, results should be Source=local.
	for _, s := range skills {
		if s.Source != SourceLocal {
			t.Errorf("real skill %q has source %q, want local", s.Name, s.Source)
		}
	}
	t.Logf("ScanLocal found %d local skills", len(skills))
}

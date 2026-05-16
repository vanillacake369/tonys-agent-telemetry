package data

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverSessions_NonEmpty(t *testing.T) {
	// Integration test: reads real ~/.claude/projects/ directory.
	claudeProjectsDir := filepath.Join(ClaudeDir(), "projects")
	if _, err := os.Stat(claudeProjectsDir); err != nil {
		t.Skipf("~/.claude/projects not found: %v", err)
	}

	sessions, err := DiscoverSessions()
	if err != nil {
		t.Fatalf("DiscoverSessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Error("expected at least one session, got 0")
	}
}

func TestDiscoverSessions_SortedByTimestampDesc(t *testing.T) {
	claudeProjectsDir := filepath.Join(ClaudeDir(), "projects")
	if _, err := os.Stat(claudeProjectsDir); err != nil {
		t.Skipf("~/.claude/projects not found: %v", err)
	}

	sessions, err := DiscoverSessions()
	if err != nil {
		t.Fatalf("DiscoverSessions: %v", err)
	}
	for i := 1; i < len(sessions); i++ {
		if sessions[i-1].Timestamp.Before(sessions[i].Timestamp) {
			t.Errorf("sessions not sorted DESC: [%d]=%v < [%d]=%v",
				i-1, sessions[i-1].Timestamp, i, sessions[i].Timestamp)
			break
		}
	}
}

func TestDiscoverProjectSessions_FiltersByProject(t *testing.T) {
	// Use a mock directory to test filtering logic deterministically.
	root := t.TempDir()
	projectsDir := filepath.Join(root, "projects")

	// Create two project directories with different encoded names.
	proj1Encoded := "-tmp-project-alpha"
	proj2Encoded := "-tmp-project-beta"
	for _, enc := range []string{proj1Encoded, proj2Encoded} {
		dir := filepath.Join(projectsDir, enc)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}

	ts := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)

	// Write a session for project alpha.
	alpha := userLine("alpha-sess", "/tmp/project-alpha", "main", "2.0", "alpha prompt", ts) + "\n"
	if err := os.WriteFile(filepath.Join(projectsDir, proj1Encoded, "alpha-sess.jsonl"), []byte(alpha), 0644); err != nil {
		t.Fatalf("WriteFile alpha: %v", err)
	}

	// Write a session for project beta.
	beta := userLine("beta-sess", "/tmp/project-beta", "main", "2.0", "beta prompt", ts) + "\n"
	if err := os.WriteFile(filepath.Join(projectsDir, proj2Encoded, "beta-sess.jsonl"), []byte(beta), 0644); err != nil {
		t.Fatalf("WriteFile beta: %v", err)
	}

	// Filter for project alpha only.
	alphaProjectDir := projectDirFromPath(proj1Encoded)
	sessions, err := discoverSessionsIn(projectsDir, alphaProjectDir)
	if err != nil {
		t.Fatalf("discoverSessionsIn: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session for alpha, got %d", len(sessions))
	}
	if sessions[0].ProjectDir != alphaProjectDir {
		t.Errorf("ProjectDir = %q, want %q", sessions[0].ProjectDir, alphaProjectDir)
	}
	if sessions[0].ID != "alpha-sess" {
		t.Errorf("ID = %q, want %q", sessions[0].ID, "alpha-sess")
	}
}

func TestDiscoverSessions_MockDirectory(t *testing.T) {
	// Use a mock directory structure to test the logic without relying on real data.
	root := t.TempDir()
	projectsDir := filepath.Join(root, "projects")

	projEncoded := "-tmp-myproject"
	projDir := filepath.Join(projectsDir, projEncoded)
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	ts := time.Date(2026, 5, 16, 7, 0, 0, 0, time.UTC)
	content := userLine("sess-001", "/tmp/myproject", "main", "2.0", "test prompt", ts) + "\n"
	content += assistantLine("sess-001", "claude-opus-4-6", ts.Add(time.Second)) + "\n"
	if err := os.WriteFile(filepath.Join(projDir, "sess-001.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Override ClaudeDir indirectly by using discoverSessionsIn.
	sessions, err := discoverSessionsIn(projectsDir, "")
	if err != nil {
		t.Fatalf("discoverSessionsIn: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	s := sessions[0]
	if s.ID != "sess-001" {
		t.Errorf("ID = %q, want %q", s.ID, "sess-001")
	}
	if s.ProjectDir != "/tmp/myproject" {
		t.Errorf("ProjectDir = %q, want %q", s.ProjectDir, "/tmp/myproject")
	}
	if s.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", s.Model, "claude-opus-4-6")
	}
}

func TestDiscoverSessions_SkipsCorruptedFiles(t *testing.T) {
	root := t.TempDir()
	projectsDir := filepath.Join(root, "projects")
	projDir := filepath.Join(projectsDir, "-tmp-corrupt")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Write a completely corrupted file.
	if err := os.WriteFile(filepath.Join(projDir, "bad.jsonl"), []byte("NOT JSON\nSTILL BAD\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Should not panic or return error — just skip the file.
	sessions, err := discoverSessionsIn(projectsDir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions from corrupted file, got %d", len(sessions))
	}
}

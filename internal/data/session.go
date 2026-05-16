package data

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ClaudeDir returns the Claude Code data directory (~/.claude).
func ClaudeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".claude")
	}
	return filepath.Join(home, ".claude")
}

// projectDirFromPath derives the human-readable project directory from the
// encoded path used by Claude (e.g. "-Users-limjihoon-dev-tonys-nix").
func projectDirFromPath(encodedName string) string {
	// Claude encodes the project path as the directory name with slashes replaced by dashes.
	// e.g. "-Users-limjihoon-dev-tonys-nix" → "/Users/limjihoon/dev/tonys-nix"
	if strings.HasPrefix(encodedName, "-") {
		return strings.ReplaceAll(encodedName, "-", "/")
	}
	return encodedName
}

// DiscoverSessions finds all session JSONL files across all projects in ClaudeDir.
// Headers are parsed in parallel (4 goroutines) for speed.
// Returns sessions sorted by timestamp DESC (most recent first).
func DiscoverSessions() ([]Session, error) {
	projectsDir := filepath.Join(ClaudeDir(), "projects")
	return discoverSessionsIn(projectsDir, "")
}

// DiscoverProjectSessions finds sessions for a specific project directory.
// projectDir should be an absolute path like "/Users/you/dev/myproject".
func DiscoverProjectSessions(projectDir string) ([]Session, error) {
	projectsDir := filepath.Join(ClaudeDir(), "projects")
	return discoverSessionsIn(projectsDir, projectDir)
}

// discoverSessionsIn is the shared implementation for discovering sessions.
// If filterProject is non-empty only sessions whose CWD or ProjectDir match are returned.
func discoverSessionsIn(projectsDir string, filterProject string) ([]Session, error) {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	type work struct {
		filePath   string
		projectDir string
	}

	var jobs []work
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		encoded := entry.Name()
		projectDir := projectDirFromPath(encoded)

		if filterProject != "" && projectDir != filterProject {
			continue
		}

		subDir := filepath.Join(projectsDir, encoded)
		subEntries, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}
		for _, se := range subEntries {
			if se.IsDir() || !strings.HasSuffix(se.Name(), ".jsonl") {
				continue
			}
			jobs = append(jobs, work{
				filePath:   filepath.Join(subDir, se.Name()),
				projectDir: projectDir,
			})
		}
	}

	const workers = 4
	jobCh := make(chan work, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	resultCh := make(chan Session, len(jobs))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				s, err := ParseSessionHeader(j.filePath)
				if err != nil {
					continue
				}
				s.ProjectDir = j.projectDir
				resultCh <- *s
			}
		}()
	}

	wg.Wait()
	close(resultCh)

	var sessions []Session
	for s := range resultCh {
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp.After(sessions[j].Timestamp)
	})

	return sessions, nil
}

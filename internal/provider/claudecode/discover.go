package claudecode

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
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
	if strings.HasPrefix(encodedName, "-") {
		return strings.ReplaceAll(encodedName, "-", "/")
	}
	return encodedName
}

// SessionMeta holds session metadata as discovered by claudecode.
// During S2-S4, callers use data.Session via data.DiscoverSessions().
// This type is used internally by ClaudeCodeIngestor.
type SessionMeta struct {
	ID          string
	ProjectDir  string
	CWD         string
	GitBranch   string
	FirstPrompt string
	Timestamp   time.Time
	Model       string
	Version     string
	FilePath    string
}

// DiscoverSessionMetas finds all session JSONL files across all projects in ClaudeDir.
// Returns sessions sorted by timestamp DESC (most recent first).
func DiscoverSessionMetas() ([]SessionMeta, error) {
	projectsDir := filepath.Join(ClaudeDir(), "projects")
	return discoverSessionMetasIn(projectsDir, "")
}

// discoverSessionMetasIn is the shared implementation.
func discoverSessionMetasIn(projectsDir string, filterProject string) ([]SessionMeta, error) {
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
		projDir := projectDirFromPath(encoded)

		if filterProject != "" && projDir != filterProject {
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
				projectDir: projDir,
			})
		}
	}

	const workers = 4
	jobCh := make(chan work, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	resultCh := make(chan SessionMeta, len(jobs))
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
				resultCh <- SessionMeta{
					ID:          s.ID,
					ProjectDir:  j.projectDir,
					CWD:         s.CWD,
					GitBranch:   s.GitBranch,
					FirstPrompt: s.FirstPrompt,
					Timestamp:   s.Timestamp,
					Model:       s.Model,
					Version:     s.Version,
					FilePath:    s.FilePath,
				}
			}
		}()
	}

	wg.Wait()
	close(resultCh)

	var sessions []SessionMeta
	for s := range resultCh {
		sessions = append(sessions, s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Timestamp.After(sessions[j].Timestamp)
	})

	return sessions, nil
}

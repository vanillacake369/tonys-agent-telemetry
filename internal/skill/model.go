package skill

import "time"

// Source indicates where a skill originates from.
type Source string

const (
	SourceLocal  Source = "local"
	SourceGitHub Source = "github"
)

// Skill represents a Claude Code skill from any source.
type Skill struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Source      Source    `json:"source"`
	URL         string    `json:"url"`
	Stars       int       `json:"stars"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ReadmeURL   string    `json:"readme_url"`
}

// SortBy defines the ordering applied when merging GitHub results.
type SortBy int

const (
	SortByStars   SortBy = iota
	SortByCreated        // newest first
	SortByUpdated        // most recently updated first
)

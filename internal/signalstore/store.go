// Package signalstore — wire format contract documented in doc.go.
//
// Wire format (per file):
//
//	line 1:   Header JSON (see schema.go)
//	line 2..N: SnapshotEntry JSON, one per Append call
//
// SnapshotEntry:
//
//	{ "captured_at": "2026-05-26T...", "signals": [ ...signal.Signal... ] }
package signalstore

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
)

// maxFileBytesOverride allows tests to inject a tiny threshold to trigger
// rotation without writing 64 MiB of data. Zero means "use MaxFileBytes".
// Package-private so only test files in this package can set it.
var maxFileBytesOverride int64

// SetMaxFileBytesForTest sets a test-only override for the rotation threshold.
// Pass 0 to restore the production default (MaxFileBytes).
// Intended for use in test files only.
func SetMaxFileBytesForTest(n int64) {
	maxFileBytesOverride = n
}

// effectiveMaxFileBytes returns the rotation threshold in effect.
// Uses maxFileBytesOverride when non-zero, otherwise MaxFileBytes.
func effectiveMaxFileBytes() int64 {
	if maxFileBytesOverride > 0 {
		return maxFileBytesOverride
	}
	return MaxFileBytes
}

// Store persists Signal slices to JSONL files, one file per session.
type Store struct {
	// Root is the directory under which session JSONL files are written.
	Root string
}

// SnapshotEntry is the wire shape appended to a session file on each Append call.
type SnapshotEntry struct {
	CapturedAt time.Time       `json:"captured_at"`
	Signals    []signal.Signal `json:"signals"`
}

// SessionSnapshot groups all SnapshotEntries for one session, as returned by
// LoadRange.
type SessionSnapshot struct {
	SessionID string
	Entries   []SnapshotEntry
}

// NewStore constructs a Store using the default resolved store path.
// It creates the directory if needed. Use NewStoreAt in tests.
func NewStore() (*Store, error) {
	root, err := ResolveStorePath()
	if err != nil {
		return nil, fmt.Errorf("signalstore.NewStore: %w", err)
	}
	return &Store{Root: root}, nil
}

// NewStoreAt constructs a Store rooted at root without any directory creation.
// Intended for tests: callers pass t.TempDir() directly.
func NewStoreAt(root string) *Store {
	return &Store{Root: root}
}

// Append writes one SnapshotEntry (captured_at=now, signals=sigs) for
// sessionID. If the session file does not yet exist, the Header line is
// written first. An empty signals slice is valid.
//
// On Unix, the session file is exclusively flock-protected for the duration
// of the write, serializing concurrent calls from multiple goroutines or
// processes. On Windows, flock is not available; callers must enforce a
// single-writer-per-session constraint externally.
//
// When the active session file would exceed MaxFileBytes, Append automatically
// rotates: the existing file is renamed to "<sessionID>.<unixtime>.jsonl" (an
// archived copy), and the new entry is written to a fresh file with a new
// header. LoadSession and LoadRange both read ALL files matching the sessionID
// prefix, so no data is lost after rotation.
func (s *Store) Append(sessionID string, sigs []signal.Signal) error {
	path, err := sessionFilePath(s.Root, sessionID)
	if err != nil {
		return err
	}

	// Ensure signals is never nil in the serialized form.
	if sigs == nil {
		sigs = []signal.Signal{}
	}

	fl := flock.New(path + ".lock")
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("signalstore: flock %s: %w", path, err)
	}
	defer fl.Unlock() //nolint:errcheck

	// Rotate when the file would exceed the size threshold.
	if fi, err := os.Stat(path); err == nil {
		if fi.Size() >= effectiveMaxFileBytes() {
			if rotErr := rotateSessionFile(path, sessionID); rotErr != nil {
				return fmt.Errorf("signalstore: rotate %s: %w", path, rotErr)
			}
		}
	}

	// Open for append (O_CREATE | O_WRONLY | O_APPEND).
	needsHeader := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		needsHeader = true
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("signalstore: open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck

	w := bufio.NewWriter(f)

	if needsHeader {
		hdr := Header{
			SchemaVersion: CurrentSchemaVersion,
			WrittenAt:     time.Now().UTC(),
			Producer:      defaultProducer(),
		}
		if err := writeJSONLine(w, hdr); err != nil {
			return fmt.Errorf("signalstore: write header: %w", err)
		}
	}

	entry := SnapshotEntry{
		CapturedAt: time.Now().UTC(),
		Signals:    sigs,
	}
	if err := writeJSONLine(w, entry); err != nil {
		return fmt.Errorf("signalstore: write entry: %w", err)
	}

	return w.Flush()
}

// rotateSessionFile renames the active session file to an archive name of the
// form "<sanitized-sessionID>.<unixtime>.jsonl". This is the single DRY
// implementation of the rotation strategy; Append is its only caller.
func rotateSessionFile(activePath, sessionID string) error {
	dir := filepath.Dir(activePath)
	base, _ := SessionFilename(sessionID) // already sanitized
	baseStem := strings.TrimSuffix(base, ".jsonl")
	archiveName := fmt.Sprintf("%s.%d.jsonl", baseStem, time.Now().UnixNano())
	archivePath := filepath.Join(dir, archiveName)
	return os.Rename(activePath, archivePath)
}

// LoadSession reads all SnapshotEntries for sessionID. It reads BOTH the
// active file ("<sanitized-id>.jsonl") AND any archived rotation files
// ("<sanitized-id>.<unixtime>.jsonl"). Archived files are read in
// chronological order (by filename, which encodes the unix timestamp),
// followed by the active file — so the returned slice is in append order.
//
// Returns an empty (non-nil) slice when no files exist.
// Returns ErrSchemaMismatch when the active file's header version is
// unsupported. Corrupt archived files are skipped with a log warning.
func (s *Store) LoadSession(sessionID string) ([]SnapshotEntry, error) {
	activeName, err := SessionFilename(sessionID)
	if err != nil {
		return nil, err
	}
	activePath := filepath.Join(s.Root, activeName)
	activeStem := strings.TrimSuffix(activeName, ".jsonl")

	// Collect archived rotation files matching "<stem>.<digits>.jsonl".
	archivePaths, err := archiveFilesFor(s.Root, activeStem)
	if err != nil {
		// Directory unreadable — return empty, not an error.
		return []SnapshotEntry{}, nil
	}

	var all []SnapshotEntry

	// Read archived files first (older), then the active file (newest).
	for _, ap := range archivePaths {
		entries, err := readSessionFileAtPath(ap)
		if err != nil {
			log.Printf("signalstore: LoadSession: skipping archive %s: %v", ap, err)
			continue
		}
		all = append(all, entries...)
	}

	// Active file.
	f, err := os.Open(activePath)
	if os.IsNotExist(err) {
		if len(all) == 0 {
			return []SnapshotEntry{}, nil
		}
		return all, nil
	}
	if err != nil {
		return nil, fmt.Errorf("signalstore: open %s: %w", activePath, err)
	}
	defer f.Close() //nolint:errcheck

	activeEntries, err := readSessionFile(f)
	if err != nil {
		return nil, err
	}
	all = append(all, activeEntries...)
	return all, nil
}

// LoadRange walks every session file under Root and returns SessionSnapshots
// whose CapturedAt timestamps have at least one entry within [from, to].
//
// It reads BOTH active session files ("<id>.jsonl") AND archived rotation
// files ("<id>.<unixtime>.jsonl"), grouping all entries for the same session
// ID together into one SessionSnapshot.
//
// Sessions with unreadable files or schema mismatches are skipped with a
// logged warning rather than causing a fatal error — partial data is better
// than no data for time-series analysis.
func (s *Store) LoadRange(from, to time.Time) ([]SessionSnapshot, error) {
	dirEntries, err := os.ReadDir(s.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionSnapshot{}, nil
		}
		return nil, fmt.Errorf("signalstore: read dir %s: %w", s.Root, err)
	}

	// Group entries by sessionID (active file stem).
	// archivePattern: "<stem>.<digits>.jsonl" maps to stem.
	accumulated := make(map[string][]SnapshotEntry)

	for _, de := range dirEntries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}

		// Determine the sessionID stem: active files are "<stem>.jsonl";
		// archived files are "<stem>.<digits>.jsonl".
		stem := strings.TrimSuffix(name, ".jsonl")
		sessionID := stem
		if idx := archiveStemIndex(stem); idx >= 0 {
			sessionID = stem[:idx]
		}

		path := filepath.Join(s.Root, name)
		fileEntries, err := readSessionFileAtPath(path)
		if err != nil {
			log.Printf("signalstore: LoadRange: skipping %s: %v", name, err)
			continue
		}

		accumulated[sessionID] = append(accumulated[sessionID], fileEntries...)
	}

	var results []SessionSnapshot
	for sessionID, allEntries := range accumulated {
		// Sort entries by CapturedAt so merged multi-file data is ordered.
		sort.Slice(allEntries, func(i, j int) bool {
			return allEntries[i].CapturedAt.Before(allEntries[j].CapturedAt)
		})

		// Filter entries to those within [from, to].
		var filtered []SnapshotEntry
		for _, e := range allEntries {
			if (e.CapturedAt.Equal(from) || e.CapturedAt.After(from)) &&
				(e.CapturedAt.Equal(to) || e.CapturedAt.Before(to)) {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) == 0 {
			continue
		}

		results = append(results, SessionSnapshot{
			SessionID: sessionID,
			Entries:   filtered,
		})
	}

	return results, nil
}

// archiveStemIndex returns the index of the '.' that separates the session
// stem from the unix-timestamp suffix in an archive filename stem, or -1 if
// the stem is not an archive name (i.e. it is an active filename stem).
//
// Archive stems have the shape "<session-stem>.<digits>".
func archiveStemIndex(stem string) int {
	for i := len(stem) - 1; i >= 0; i-- {
		if stem[i] == '.' {
			suffix := stem[i+1:]
			if len(suffix) > 0 && isAllDigits(suffix) {
				return i
			}
			return -1
		}
	}
	return -1
}

// isAllDigits returns true when s is non-empty and every byte is an ASCII digit.
func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// archiveFilesFor returns the paths of archived rotation files for the given
// session stem in chronological order (oldest first), sorted by filename.
func archiveFilesFor(dir, activeStem string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	prefix := activeStem + "."
	var paths []string
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		// Verify the middle part (between prefix and .jsonl) is all digits.
		inner := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".jsonl")
		if isAllDigits(inner) {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	sort.Strings(paths) // lexicographic = chronological for fixed-width timestamps
	return paths, nil
}

// readSessionFileAtPath opens path, acquires a SHARED (read) flock to prevent
// reading partially-written data, then delegates to readSessionFile. If the
// shared lock cannot be acquired within a short timeout, the function falls
// back to reading without a lock and logs the contention — partial data is
// better than a blocking read that stalls the TUI.
func readSessionFileAtPath(path string) ([]SnapshotEntry, error) {
	fl := flock.New(path + ".lock")

	// Try a non-blocking shared lock; fall back gracefully on contention.
	locked, err := fl.TryRLock()
	if err != nil {
		log.Printf("signalstore: TryRLock %s: %v (proceeding without lock)", path, err)
	} else if locked {
		defer fl.Unlock() //nolint:errcheck
	} else {
		// Lock held by a writer; log contention and continue without lock.
		log.Printf("signalstore: read contention on %s (writer holds lock); reading without shared lock", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck
	return readSessionFile(f)
}

// readSessionFile parses a JSONL session file from r.
// Line 1 must be a valid Header with a supported schema version.
// Lines 2..N are SnapshotEntry records.
func readSessionFile(r io.Reader) ([]SnapshotEntry, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20) // 1 MiB per line

	// Line 1: header.
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, fmt.Errorf("signalstore: read header: %w", err)
		}
		// Empty file (no header line) — return empty.
		return []SnapshotEntry{}, nil
	}

	var hdr Header
	if err := json.Unmarshal(sc.Bytes(), &hdr); err != nil {
		return nil, fmt.Errorf("signalstore: parse header: %w", err)
	}
	if err := checkSchemaVersion(hdr.SchemaVersion); err != nil {
		return nil, err
	}

	// Lines 2..N: entries.
	var entries []SnapshotEntry
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e SnapshotEntry
		if err := json.Unmarshal(line, &e); err != nil {
			// Malformed entry line — skip with a warning; partial recovery.
			log.Printf("signalstore: skipping malformed entry line: %v", err)
			continue
		}
		// Ensure Signals is non-nil for consumers.
		if e.Signals == nil {
			e.Signals = []signal.Signal{}
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("signalstore: scan entries: %w", err)
	}

	return entries, nil
}

// writeJSONLine marshals v to JSON and writes it followed by a newline to w.
// This helper is the single point of JSONL record serialization, used by
// both Append (for header and entry) to guarantee DRY encoding.
func writeJSONLine(w *bufio.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	return w.WriteByte('\n')
}

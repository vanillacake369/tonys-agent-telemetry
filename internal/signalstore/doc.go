// Package signalstore persists []signal.Signal snapshots to JSONL files,
// one file per session, enabling Phase 3 longitudinal analysis.
//
// # Wire Format
//
// Each file contains exactly one UTF-8-encoded line per record, terminated by
// a newline character ('\n'). The first line is always the Header; all
// subsequent lines are SnapshotEntry records. Both are JSON objects.
//
// Example file (3 content lines):
//
//	{"schema_version":"1","written_at":"2026-05-26T09:00:00Z","producer":"tonys-agent-telemetry v0.1.0"}
//	{"captured_at":"2026-05-26T09:00:01Z","signals":[{"id":"abc12345","type":"stalled_node","trace_id":"t1","span_ids":["s1"],"evidence":{"stall_duration_ms":14200},"confidence":0.71,"emitted_at":"2026-05-26T09:00:00Z","provider_tier":"full"}]}
//	{"captured_at":"2026-05-26T09:00:02Z","signals":[]}
//
// Line 1 (Header): a JSON object with fields schema_version, written_at
// (RFC 3339 with sub-second precision), and producer (binary identity string).
// A reader MUST reject files whose schema_version does not match a supported
// value; it returns ErrSchemaMismatch in that case.
//
// Lines 2..N (SnapshotEntry): each represents one Append call.  Fields:
//   - captured_at: time.Time (RFC 3339) when Append was called.
//   - signals: []signal.Signal — may be empty (a snapshot of a clean run
//     is valid and should not be dropped).
//
// # Session File Naming
//
// Files are named "<sanitized-sessionID>.jsonl" under the store root.
// Sanitization rules:
//   - Empty sessionID is rejected (returns error).
//   - All '/' characters are replaced with '_'.
//   - A leading '.' is replaced with '_' to prevent hidden-file confusion.
//   - The sanitized base name is truncated to 200 characters before the
//     ".jsonl" suffix is appended.
//
// # Automatic File Rotation
//
// When Append detects that the active session file has reached or exceeded
// MaxFileBytes, it automatically rotates the file before writing the new
// entry. Rotation renames the existing active file to
// "<sanitized-sessionID>.<unix-nanosecond-timestamp>.jsonl" (the "archive"
// name), then writes the new entry to a fresh file with a new header.
//
// Example resulting filenames for session "my-session" after one rotation:
//
//	my-session.1716710400000000000.jsonl  ← archived (rotated out)
//	my-session.jsonl                      ← active (new entries go here)
//
// LoadSession and LoadRange both read ALL files that match the sessionID
// prefix (active + all archives), merging entries in chronological order.
// No data is lost after rotation.
//
// # Concurrency Contract
//
// Each Append call acquires an EXCLUSIVE flock (LOCK_EX) on a companion
// "<session>.jsonl.lock" file before writing, serializing concurrent calls
// from multiple goroutines or processes.
//
// Each read call (readSessionFileAtPath, used by LoadSession and LoadRange)
// acquires a SHARED flock (LOCK_SH) on the same companion lock file via
// TryRLock. If the shared lock cannot be acquired (because a writer holds the
// exclusive lock), the reader falls back to proceeding without the lock and
// logs the contention — partial data is better than a stalled TUI.
//
// On platforms where flock is unavailable (Windows), a single-writer-per-
// session-file constraint must be enforced by the caller.
//
// # Schema Versioning Policy
//
// CurrentSchemaVersion = "1". Bump to "2" when any field in Header or
// SnapshotEntry is renamed, removed, or has its semantic changed in a
// backward-incompatible way. Adding optional fields to SnapshotEntry is a
// minor change and does NOT require a version bump.  A reader that encounters
// an unknown schema version must return ErrSchemaMismatch rather than
// silently misparse the data.
package signalstore

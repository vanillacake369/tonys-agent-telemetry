package signalstore

import (
	"errors"
	"time"
)

// CurrentSchemaVersion is the schema version this build writes and reads.
// It is the SSoT for the version string written into every Header line.
// Readers reject any file whose Header.SchemaVersion does not equal this.
const CurrentSchemaVersion = "1"

// supportedSchemaVersions is the set of schema versions this build can
// read. Currently only "1". Extend this set — not CurrentSchemaVersion —
// when a forward-compatible reader is needed.
var supportedSchemaVersions = map[string]bool{
	"1": true,
}

// Header is the first line of every signal store file.
//
// Wire encoding: a single JSON object terminated by '\n'.
// Readers reject files whose SchemaVersion is not in supportedSchemaVersions.
type Header struct {
	SchemaVersion string    `json:"schema_version"`
	WrittenAt     time.Time `json:"written_at"`
	Producer      string    `json:"producer"` // e.g. "tonys-agent-telemetry v0.1.0"
}

// ErrSchemaMismatch is returned when a file's Header.SchemaVersion is not
// in the set of schema versions this build can read.
var ErrSchemaMismatch = errors.New("signalstore: schema version mismatch")

// checkSchemaVersion returns ErrSchemaMismatch when version is unsupported.
func checkSchemaVersion(version string) error {
	if !supportedSchemaVersions[version] {
		return ErrSchemaMismatch
	}
	return nil
}

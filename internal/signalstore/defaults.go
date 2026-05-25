package signalstore

import (
	"runtime/debug"
)

// MaxFileBytes is the maximum size (in bytes) a single session JSONL file may
// reach before Append returns an error. Rotation is a Phase 3 follow-up;
// for now a hard cap prevents unbounded growth.
//
// This is the SSoT for the file size limit. No other file in this package
// should redeclare or override it.
const MaxFileBytes = 64 * 1024 * 1024 // 64 MiB

// defaultStoreDirName is the subdirectory created under os.UserCacheDir().
const defaultStoreDirName = "tonys-agent-telemetry/signals"

// defaultStoreEnvVar is the environment variable that overrides the store path.
const defaultStoreEnvVar = "TONYS_SIGNAL_STORE"

// defaultProducer returns a Producer string for the Header.
// It attempts to read the binary's build info for a version tag; falls back
// to "tonys-agent-telemetry dev" when no build info is available (e.g.,
// when running tests or built without version injection).
func defaultProducer() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return "tonys-agent-telemetry " + info.Main.Version
		}
	}
	return "tonys-agent-telemetry dev"
}

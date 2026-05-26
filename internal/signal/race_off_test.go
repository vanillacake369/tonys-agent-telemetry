//go:build !race

package signal_test

// isRaceEnabled returns false when the race detector is NOT compiled in.
// Build-tag-paired with race_on_test.go so timing-sensitive tests can
// gracefully skip themselves under -race without hard-coded GOMAXPROCS
// or runtime introspection.
func isRaceEnabled() bool { return false }

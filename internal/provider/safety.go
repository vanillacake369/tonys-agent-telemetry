// Package provider contains shared utilities used across all provider
// ingestor implementations.
package provider

import (
	"fmt"
	"runtime/debug"
)

// RecoverIngest is the standard goroutine-panic guard for all provider Ingest
// implementations. Wrap the goroutine body with:
//
//	defer provider.RecoverIngest("claudecode", log.Printf)()
//
// It logs the provider ID, the panic value, and the stack trace, then swallows
// the panic so a single malformed input cannot kill the process.
func RecoverIngest(providerID string, logFn func(format string, args ...any)) func() {
	return func() {
		r := recover()
		if r == nil {
			return
		}
		stack := string(debug.Stack())
		logFn("provider %s: recovered from panic: %v\n%s",
			providerID, fmt.Sprintf("%v", r), stack)
	}
}

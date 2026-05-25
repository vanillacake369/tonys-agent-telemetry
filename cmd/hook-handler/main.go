package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/event"
)

// debug logs a message to stderr when TAT_DEBUG=1 is set.
func debug(format string, args ...any) {
	if os.Getenv("TAT_DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[tat-hook] "+format+"\n", args...)
	}
}

func main() {
	// Hook handler must ALWAYS exit 0 to never block Claude Code.
	defer os.Exit(0)

	hookType := ""
	if len(os.Args) > 1 {
		hookType = os.Args[1]
	}

	debug("invoked with hookType=%q", hookType)

	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		debug("failed to read stdin: %v", err)
		return
	}

	debug("payload=%d bytes", len(payload))

	if err := event.WriteToFIFO(payload, hookType, 2*time.Second); err != nil {
		// Silent failure — never block Claude.
		debug("WriteToFIFO error: %v", err)
		return
	}

	debug("event written successfully")
}

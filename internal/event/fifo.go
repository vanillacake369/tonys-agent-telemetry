package event

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

const DefaultFIFOPath = "/tmp/tonys-agent-telemetry.fifo"

// WriteToFIFO writes hook payload to the FIFO if it exists.
// Returns immediately if FIFO doesn't exist (TUI not running).
func WriteToFIFO(payload []byte, hookType string, timeout time.Duration) error {
	info, err := os.Stat(DefaultFIFOPath)
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		return nil // FIFO doesn't exist, TUI not running — silent no-op
	}

	// Non-blocking open with timeout
	done := make(chan error, 1)
	go func() {
		f, err := os.OpenFile(DefaultFIFOPath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			done <- err
			return
		}
		defer f.Close()

		msg := fmt.Sprintf("%s\t%s\n", hookType, string(payload))
		_, err = f.WriteString(msg)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("fifo write timeout")
	}
}

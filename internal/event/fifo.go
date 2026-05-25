package event

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultFIFOPath = "/tmp/tonys-agent-telemetry.fifo"
	PIDFilePath     = "/tmp/tonys-agent-telemetry.pid"
)

// Event represents a hook event received via FIFO.
type Event struct {
	HookType string          // "SessionStart", "PostToolUse", "SubagentStop", etc.
	Payload  json.RawMessage
}

// CreateFIFO creates the named pipe at DefaultFIFOPath.
// Removes stale pipe if the PID file indicates a dead process.
// Returns an error if another live instance is detected.
func CreateFIFO() error {
	return createFIFOImpl(DefaultFIFOPath, PIDFilePath)
}

// createFIFOImpl is the testable core of CreateFIFO that accepts explicit paths.
func createFIFOImpl(fifoPath, pidPath string) error {
	// Check for an existing PID file.
	if pidBytes, err := os.ReadFile(pidPath); err == nil {
		pidStr := strings.TrimSpace(string(pidBytes))
		pid, parseErr := strconv.Atoi(pidStr)
		if parseErr == nil && pid > 0 {
			proc, findErr := os.FindProcess(pid)
			if findErr == nil {
				// Send signal 0 to check if the process is alive.
				if signalErr := proc.Signal(syscall.Signal(0)); signalErr == nil {
					return fmt.Errorf("another instance running (pid %d)", pid)
				}
			}
		}
		// Dead process — clean up stale files.
		_ = os.Remove(fifoPath)
		_ = os.Remove(pidPath)
	}

	// Remove any leftover FIFO from a previous crash.
	_ = os.Remove(fifoPath)

	if err := syscall.Mkfifo(fifoPath, 0600); err != nil {
		return fmt.Errorf("mkfifo %s: %w", fifoPath, err)
	}

	// Write current PID to PID file.
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		_ = os.Remove(fifoPath)
		return fmt.Errorf("write pid file: %w", err)
	}

	return nil
}

// RemoveFIFO cleans up the named pipe and PID file.
func RemoveFIFO() {
	_ = os.Remove(DefaultFIFOPath)
	_ = os.Remove(PIDFilePath)
}

// ReadFIFO returns a channel that emits Events read from the FIFO at DefaultFIFOPath.
// The goroutine exits when ctx is cancelled.
// Handles: partial reads, malformed data, and FIFO reopening after writer disconnects.
func ReadFIFO(ctx context.Context) <-chan Event {
	return ReadFIFOFromPath(ctx, DefaultFIFOPath)
}

// ReadFIFOFromPath is the internal implementation that accepts an explicit path.
// This allows tests and the claudecode ingestor to override the path.
func ReadFIFOFromPath(ctx context.Context, fifoPath string) <-chan Event {
	ch := make(chan Event, 16)

	go func() {
		defer close(ch)

		for {
			// Check context before attempting to open.
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Opening a FIFO for reading blocks until a writer opens the write end.
			// We use a goroutine so we can honour context cancellation.
			type openResult struct {
				f   *os.File
				err error
			}
			openCh := make(chan openResult, 1)
			go func() {
				f, err := os.Open(fifoPath)
				openCh <- openResult{f, err}
			}()

			var f *os.File
			select {
			case <-ctx.Done():
				return
			case res := <-openCh:
				if res.err != nil {
					// FIFO removed or gone — exit.
					return
				}
				f = res.f
			}

			// Read lines from the open FIFO until EOF (writer disconnected).
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					f.Close()
					return
				default:
				}

				line := scanner.Text()
				ev, err := parseLine(line)
				if err != nil {
					// Skip malformed lines.
					continue
				}

				select {
				case ch <- ev:
				case <-ctx.Done():
					f.Close()
					return
				}
			}
			f.Close()

			// EOF means the writer disconnected; loop to reopen.
		}
	}()

	return ch
}

// V2SpanHookType is the synthetic HookType used for v2 span-shaped FIFO lines.
// Consumers that receive this hook type should treat Payload as a full v2 span JSON.
const V2SpanHookType = "__v2_span__"

// parseLine parses a single line from the FIFO.
// Supports two formats:
//
//	v1 (legacy): "HOOKTYPE\tJSON_PAYLOAD"
//	v2 (span):   {"v":2,"trace_id":"...","span_id":"...",...}
//
// v2 is detected by a leading JSON object whose "v" field equals 2.
// v2 lines are returned with HookType set to V2SpanHookType.
func parseLine(line string) (Event, error) {
	// fast peek: detect v2 before attempting full parse
	var probe struct {
		V int `json:"v"`
	}
	if json.Unmarshal([]byte(line), &probe) == nil && probe.V == 2 {
		return Event{HookType: V2SpanHookType, Payload: json.RawMessage(line)}, nil
	}

	// v1: "HOOKTYPE\tJSON_PAYLOAD"
	idx := strings.IndexByte(line, '\t')
	if idx < 0 {
		return Event{}, fmt.Errorf("missing tab separator in line: %q", line)
	}
	hookType := line[:idx]
	payload := line[idx+1:]
	if hookType == "" {
		return Event{}, fmt.Errorf("empty hook type in line: %q", line)
	}
	if !json.Valid([]byte(payload)) {
		return Event{}, fmt.Errorf("invalid JSON payload in line: %q", line)
	}
	return Event{
		HookType: hookType,
		Payload:  json.RawMessage(payload),
	}, nil
}

// WriteToFIFO writes hook payload to the FIFO if it exists.
// Returns immediately if FIFO doesn't exist (TUI not running).
// Writes in v1 format: "HOOKTYPE\tPAYLOAD\n".
func WriteToFIFO(payload []byte, hookType string, timeout time.Duration) error {
	msg := fmt.Sprintf("%s\t%s\n", hookType, string(payload))
	return writeLineToFIFO([]byte(msg), timeout)
}

// WriteSpanLineToFIFO writes a raw v2 span JSON line to the FIFO.
// The line must be a complete v2 JSON object ({"v":2,...}).
func WriteSpanLineToFIFO(line []byte, timeout time.Duration) error {
	msg := append(line, '\n')
	return writeLineToFIFO(msg, timeout)
}

// writeLineToFIFO writes raw bytes to the FIFO with a timeout.
func writeLineToFIFO(msg []byte, timeout time.Duration) error {
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
		_, err = f.Write(msg)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("fifo write timeout")
	}
}

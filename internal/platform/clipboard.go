package platform

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// clipboardCmd returns the program and args for writing to the system clipboard,
// plus whether it was found. Returns ("", nil, false) when no tool is available.
func clipboardCmd() (string, []string, bool) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("pbcopy"); err == nil {
			return "pbcopy", nil, true
		}
	case "linux":
		// WSL: use clip.exe
		if isWSL() {
			if _, err := exec.LookPath("clip.exe"); err == nil {
				return "clip.exe", nil, true
			}
		}
		// Wayland
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return "wl-copy", nil, true
		}
		// X11
		if _, err := exec.LookPath("xclip"); err == nil {
			return "xclip", []string{"-selection", "clipboard"}, true
		}
	}
	return "", nil, false
}

// isWSL returns true when running inside Windows Subsystem for Linux.
func isWSL() bool {
	return DetectOS() == OSWSL
}

// HasClipboard returns true if a clipboard tool is available on the current system.
func HasClipboard() bool {
	_, _, ok := clipboardCmd()
	return ok
}

// CopyToClipboard copies text to the system clipboard.
// Returns an error if no clipboard tool is available or the command fails.
func CopyToClipboard(text string) error {
	prog, args, ok := clipboardCmd()
	if !ok {
		return fmt.Errorf("platform: no clipboard tool available on this system")
	}

	cmd := exec.Command(prog, args...)
	cmd.Stdin = strings.NewReader(text)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("platform: clipboard command %q failed: %w: %s", prog, err, msg)
		}
		return fmt.Errorf("platform: clipboard command %q failed: %w", prog, err)
	}
	return nil
}

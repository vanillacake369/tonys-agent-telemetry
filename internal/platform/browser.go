package platform

import (
	"fmt"
	"os/exec"
)

// OpenInBrowser opens a URL in the default browser for the current OS.
// It starts the browser process without waiting for it to close.
func OpenInBrowser(url string) error {
	if url == "" {
		return fmt.Errorf("platform: empty URL")
	}

	var cmd *exec.Cmd
	switch DetectOS() {
	case OSDarwin:
		cmd = exec.Command("open", url)
	case OSWSL:
		// Prefer wslview (wslu package) when available; fall back to cmd.exe /c start.
		if _, err := exec.LookPath("wslview"); err == nil {
			cmd = exec.Command("wslview", url)
		} else {
			cmd = exec.Command("cmd.exe", "/c", "start", url)
		}
	default: // Linux
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start() // Don't wait for the browser to close.
}

package platform

import (
	"fmt"
	"os/exec"
)

// buildPaneCommand returns the program and arguments needed to open a new pane
// running the given cmd string. Multiplexer takes priority over emulator.
// This helper is exported only within the package for testability.
func (t TerminalInfo) buildPaneCommand(cmd string) (string, []string) {
	switch t.Multiplexer {
	case MuxZellij:
		return "zellij", []string{"action", "new-pane", "--direction", "right", "--", "bash", "-c", cmd}
	case MuxTmux:
		return "tmux", []string{"split-window", "-h", cmd}
	case MuxScreen:
		return "screen", []string{"-X", "screen", "bash", "-c", cmd}
	}

	// No multiplexer — use emulator-specific command.
	switch t.Emulator {
	case EmulatorWezTerm:
		return "wezterm", []string{"cli", "spawn", "--", "bash", "-c", cmd}
	case EmulatorKitty:
		return "kitty", []string{"@", "launch", "--type=tab", "bash", "-c", cmd}
	case EmulatorITerm2:
		// iTerm2 uses AppleScript via osascript.
		script := fmt.Sprintf(
			`tell application "iTerm2" to tell current window to create tab with default profile command %q`,
			cmd,
		)
		return "osascript", []string{"-e", script}
	}

	// Fallback: exec bash in place.
	return "bash", []string{"-c", cmd}
}

// OpenPane opens a new terminal pane/tab running cmd.
// Priority: multiplexer > emulator > fallback (bash -c).
func (t TerminalInfo) OpenPane(cmd string) error {
	prog, args := t.buildPaneCommand(cmd)
	if _, err := exec.LookPath(prog); err != nil {
		return fmt.Errorf("platform: %q not found in PATH: %w", prog, err)
	}
	c := exec.Command(prog, args...)
	if err := c.Start(); err != nil {
		return fmt.Errorf("platform: failed to open pane with %q: %w", prog, err)
	}
	return nil
}

// OpenPaneRight opens a pane to the right for multiplexers that support direction.
// For emulators without direction support it falls back to OpenPane.
func (t TerminalInfo) OpenPaneRight(cmd string) error {
	// Zellij and tmux both support horizontal splits natively via buildPaneCommand.
	// The right-direction is already encoded in the default buildPaneCommand for zellij/tmux.
	return t.OpenPane(cmd)
}

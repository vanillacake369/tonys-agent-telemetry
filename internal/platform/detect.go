package platform

import (
	"os"
	"os/exec"
	"strings"
)

// OS represents the operating system type.
type OS string

const (
	OSDarwin OS = "darwin"
	OSLinux  OS = "linux"
	OSWSL    OS = "wsl"
)

// Emulator represents the terminal emulator type.
type Emulator string

const (
	EmulatorWezTerm   Emulator = "wezterm"
	EmulatorKitty     Emulator = "kitty"
	EmulatorITerm2    Emulator = "iterm2"
	EmulatorAlacritty Emulator = "alacritty"
	EmulatorGhostty   Emulator = "ghostty"
	EmulatorVSCode    Emulator = "vscode"
	EmulatorTerminal  Emulator = "apple-terminal"
	EmulatorUnknown   Emulator = "unknown"
)

// Multiplexer represents the terminal multiplexer type.
type Multiplexer string

const (
	MuxZellij Multiplexer = "zellij"
	MuxTmux   Multiplexer = "tmux"
	MuxScreen Multiplexer = "screen"
	MuxNone   Multiplexer = "none"
)

// TerminalInfo holds the detected OS, emulator, and multiplexer.
type TerminalInfo struct {
	OS          OS
	Emulator    Emulator
	Multiplexer Multiplexer
}

// DetectOS checks uname and /proc/version for WSL.
// Safe for concurrent use — reads only env and /proc/version.
func DetectOS() OS {
	// Check for WSL first: /proc/version contains "Microsoft" on WSL.
	if data, err := os.ReadFile("/proc/version"); err == nil {
		if strings.Contains(strings.ToLower(string(data)), "microsoft") {
			return OSWSL
		}
	}

	// Use uname to distinguish darwin from linux.
	out, err := exec.Command("uname", "-s").Output()
	if err == nil {
		uname := strings.TrimSpace(string(out))
		switch strings.ToLower(uname) {
		case "darwin":
			return OSDarwin
		case "linux":
			return OSLinux
		}
	}

	// Fallback: check GOOS-equivalent via build tags approach — use runtime.
	return OSLinux
}

// DetectEmulator checks environment variables set by terminal emulators.
// Safe for concurrent use — reads only env vars.
func DetectEmulator() Emulator {
	if v := os.Getenv("WEZTERM_EXECUTABLE"); v != "" {
		return EmulatorWezTerm
	}
	if v := os.Getenv("KITTY_PID"); v != "" {
		return EmulatorKitty
	}
	if v := os.Getenv("ITERM_SESSION_ID"); v != "" {
		return EmulatorITerm2
	}
	if v := os.Getenv("GHOSTTY_RESOURCES_DIR"); v != "" {
		return EmulatorGhostty
	}
	if v := os.Getenv("TERM_PROGRAM"); v != "" {
		switch v {
		case "vscode":
			return EmulatorVSCode
		case "Apple_Terminal":
			return EmulatorTerminal
		}
	}
	if v := os.Getenv("ALACRITTY_WINDOW_ID"); v != "" {
		return EmulatorAlacritty
	}
	return EmulatorUnknown
}

// DetectMultiplexer checks environment variables set by terminal multiplexers.
// Safe for concurrent use — reads only env vars.
func DetectMultiplexer() Multiplexer {
	if v := os.Getenv("ZELLIJ_SESSION_NAME"); v != "" {
		return MuxZellij
	}
	if v := os.Getenv("TMUX"); v != "" {
		return MuxTmux
	}
	if v := os.Getenv("STY"); v != "" {
		return MuxScreen
	}
	return MuxNone
}

// Detect returns complete terminal info by calling all detection functions.
func Detect() TerminalInfo {
	return TerminalInfo{
		OS:          DetectOS(),
		Emulator:    DetectEmulator(),
		Multiplexer: DetectMultiplexer(),
	}
}

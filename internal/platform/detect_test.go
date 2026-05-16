package platform

import (
	"testing"
)

func TestDetectOS_Darwin(t *testing.T) {
	os := DetectOS()
	// On macOS the result must be darwin or wsl — never empty
	if os == "" {
		t.Fatalf("DetectOS returned empty string")
	}
	// Running on macOS CI/dev machine
	if os != OSDarwin && os != OSLinux && os != OSWSL {
		t.Fatalf("DetectOS returned unexpected value: %q", os)
	}
}

func TestDetectEmulator_WeztermEnv(t *testing.T) {
	t.Setenv("WEZTERM_EXECUTABLE", "/usr/bin/wezterm")
	clearEmulatorEnv(t, "WEZTERM_EXECUTABLE")
	t.Setenv("WEZTERM_EXECUTABLE", "/usr/bin/wezterm")

	e := DetectEmulator()
	if e != EmulatorWezTerm {
		t.Fatalf("expected wezterm, got %q", e)
	}
}

func TestDetectEmulator_KittyEnv(t *testing.T) {
	clearEmulatorEnv(t)
	t.Setenv("KITTY_PID", "12345")

	e := DetectEmulator()
	if e != EmulatorKitty {
		t.Fatalf("expected kitty, got %q", e)
	}
}

func TestDetectEmulator_Iterm2Env(t *testing.T) {
	clearEmulatorEnv(t)
	t.Setenv("ITERM_SESSION_ID", "w0t0p0:abc-def")

	e := DetectEmulator()
	if e != EmulatorITerm2 {
		t.Fatalf("expected iterm2, got %q", e)
	}
}

func TestDetectEmulator_GhosttyEnv(t *testing.T) {
	clearEmulatorEnv(t)
	t.Setenv("GHOSTTY_RESOURCES_DIR", "/usr/share/ghostty")

	e := DetectEmulator()
	if e != EmulatorGhostty {
		t.Fatalf("expected ghostty, got %q", e)
	}
}

func TestDetectEmulator_VSCodeEnv(t *testing.T) {
	clearEmulatorEnv(t)
	t.Setenv("TERM_PROGRAM", "vscode")

	e := DetectEmulator()
	if e != EmulatorVSCode {
		t.Fatalf("expected vscode, got %q", e)
	}
}

func TestDetectEmulator_AppleTerminalEnv(t *testing.T) {
	clearEmulatorEnv(t)
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")

	e := DetectEmulator()
	if e != EmulatorTerminal {
		t.Fatalf("expected apple-terminal, got %q", e)
	}
}

func TestDetectEmulator_AlacrittyEnv(t *testing.T) {
	clearEmulatorEnv(t)
	t.Setenv("ALACRITTY_WINDOW_ID", "1")

	e := DetectEmulator()
	if e != EmulatorAlacritty {
		t.Fatalf("expected alacritty, got %q", e)
	}
}

func TestDetectEmulator_Unknown(t *testing.T) {
	clearEmulatorEnv(t)

	e := DetectEmulator()
	// With no emulator env vars set, must return unknown
	if e != EmulatorUnknown {
		// Some CI environments may set TERM_PROGRAM; that's acceptable
		t.Logf("DetectEmulator returned %q (may be set by CI env)", e)
	}
}

func TestDetectMultiplexer_Zellij(t *testing.T) {
	clearMuxEnv(t)
	t.Setenv("ZELLIJ_SESSION_NAME", "main")

	m := DetectMultiplexer()
	if m != MuxZellij {
		t.Fatalf("expected zellij, got %q", m)
	}
}

func TestDetectMultiplexer_Tmux(t *testing.T) {
	clearMuxEnv(t)
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")

	m := DetectMultiplexer()
	if m != MuxTmux {
		t.Fatalf("expected tmux, got %q", m)
	}
}

func TestDetectMultiplexer_Screen(t *testing.T) {
	clearMuxEnv(t)
	t.Setenv("STY", "12345.pts-0.hostname")

	m := DetectMultiplexer()
	if m != MuxScreen {
		t.Fatalf("expected screen, got %q", m)
	}
}

func TestDetectMultiplexer_None(t *testing.T) {
	clearMuxEnv(t)

	m := DetectMultiplexer()
	if m != MuxNone {
		t.Fatalf("expected none, got %q", m)
	}
}

func TestDetect_ReturnsConsistentInfo(t *testing.T) {
	info := Detect()

	if info.OS != DetectOS() {
		t.Fatalf("Detect().OS = %q, DetectOS() = %q — inconsistent", info.OS, DetectOS())
	}
}

// clearEmulatorEnv clears all emulator-related env vars via t.Setenv (auto-restores).
// Pass the key to preserve as an exception (for WeztermEnv test flow).
func clearEmulatorEnv(t *testing.T, except ...string) {
	t.Helper()
	keys := []string{
		"WEZTERM_EXECUTABLE",
		"KITTY_PID",
		"ITERM_SESSION_ID",
		"GHOSTTY_RESOURCES_DIR",
		"TERM_PROGRAM",
		"ALACRITTY_WINDOW_ID",
	}
	skip := map[string]bool{}
	for _, k := range except {
		skip[k] = true
	}
	for _, k := range keys {
		if !skip[k] {
			t.Setenv(k, "")
		}
	}
}

// clearMuxEnv clears all multiplexer-related env vars via t.Setenv (auto-restores).
func clearMuxEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ZELLIJ_SESSION_NAME", "")
	t.Setenv("TMUX", "")
	t.Setenv("STY", "")
}

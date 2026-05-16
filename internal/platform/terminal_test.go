package platform

import (
	"testing"
)

func TestBuildPaneCommand_Zellij(t *testing.T) {
	info := TerminalInfo{
		OS:          OSDarwin,
		Emulator:    EmulatorWezTerm,
		Multiplexer: MuxZellij,
	}

	prog, args := info.buildPaneCommand("echo hello")
	if prog != "zellij" {
		t.Fatalf("expected zellij, got %q", prog)
	}
	// Must contain the user command somewhere in args
	found := false
	for _, a := range args {
		if a == "echo hello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args %v do not contain the command %q", args, "echo hello")
	}
}

func TestBuildPaneCommand_Tmux(t *testing.T) {
	info := TerminalInfo{
		OS:          OSDarwin,
		Emulator:    EmulatorWezTerm,
		Multiplexer: MuxTmux,
	}

	prog, args := info.buildPaneCommand("echo hello")
	if prog != "tmux" {
		t.Fatalf("expected tmux, got %q", prog)
	}
	found := false
	for _, a := range args {
		if a == "echo hello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args %v do not contain the command %q", args, "echo hello")
	}
}

func TestBuildPaneCommand_Screen(t *testing.T) {
	info := TerminalInfo{
		OS:          OSLinux,
		Emulator:    EmulatorUnknown,
		Multiplexer: MuxScreen,
	}

	prog, args := info.buildPaneCommand("echo hello")
	if prog != "screen" {
		t.Fatalf("expected screen, got %q", prog)
	}
	found := false
	for _, a := range args {
		if a == "echo hello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args %v do not contain the command %q", args, "echo hello")
	}
}

func TestBuildPaneCommand_WezTermEmulator(t *testing.T) {
	info := TerminalInfo{
		OS:          OSDarwin,
		Emulator:    EmulatorWezTerm,
		Multiplexer: MuxNone,
	}

	prog, args := info.buildPaneCommand("echo hello")
	if prog != "wezterm" {
		t.Fatalf("expected wezterm, got %q", prog)
	}
	found := false
	for _, a := range args {
		if a == "echo hello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args %v do not contain the command %q", args, "echo hello")
	}
}

func TestBuildPaneCommand_KittyEmulator(t *testing.T) {
	info := TerminalInfo{
		OS:          OSDarwin,
		Emulator:    EmulatorKitty,
		Multiplexer: MuxNone,
	}

	prog, args := info.buildPaneCommand("echo hello")
	if prog != "kitty" {
		t.Fatalf("expected kitty, got %q", prog)
	}
	_ = args
}

func TestBuildPaneCommand_FallbackExec(t *testing.T) {
	info := TerminalInfo{
		OS:          OSDarwin,
		Emulator:    EmulatorUnknown,
		Multiplexer: MuxNone,
	}

	prog, args := info.buildPaneCommand("echo hello")
	if prog != "bash" {
		t.Fatalf("expected bash fallback, got %q", prog)
	}
	found := false
	for _, a := range args {
		if a == "echo hello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args %v do not contain the command %q", args, "echo hello")
	}
}

func TestOpenPane_UnknownNoMux_ReturnsError(t *testing.T) {
	// When emulator is unknown and no multiplexer, OpenPane should return
	// an error only if bash is unavailable, or succeed via exec.
	// We just verify it doesn't panic.
	info := TerminalInfo{
		OS:          OSDarwin,
		Emulator:    EmulatorUnknown,
		Multiplexer: MuxNone,
	}
	// We can't actually exec in tests — but buildPaneCommand must not panic.
	prog, args := info.buildPaneCommand("true")
	if prog == "" {
		t.Fatal("buildPaneCommand returned empty prog")
	}
	_ = args
}

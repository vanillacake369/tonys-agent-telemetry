package platform

import (
	"strings"
	"testing"
)

func TestHasClipboard_Darwin(t *testing.T) {
	// pbcopy is always present on macOS
	if !HasClipboard() {
		t.Fatal("HasClipboard returned false on macOS — pbcopy should be available")
	}
}

func TestCopyToClipboard_Darwin(t *testing.T) {
	err := CopyToClipboard("tonys-agent-telemetry test")
	if err != nil {
		t.Fatalf("CopyToClipboard failed on macOS: %v", err)
	}
}

func TestCopyToClipboard_EmptyString(t *testing.T) {
	// Copying empty string should not error
	err := CopyToClipboard("")
	if err != nil {
		t.Fatalf("CopyToClipboard with empty string failed: %v", err)
	}
}

func TestCopyToClipboard_Multiline(t *testing.T) {
	text := strings.Join([]string{"line1", "line2", "line3"}, "\n")
	err := CopyToClipboard(text)
	if err != nil {
		t.Fatalf("CopyToClipboard with multiline text failed: %v", err)
	}
}

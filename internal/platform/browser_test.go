package platform

import (
	"testing"
)

func TestOpenInBrowser_EmptyURL_ReturnsError(t *testing.T) {
	err := OpenInBrowser("")
	if err == nil {
		t.Fatal("OpenInBrowser with empty URL should return an error, got nil")
	}
}

func TestOpenInBrowser_ValidURL_NoError(t *testing.T) {
	// On the current OS, opening a URL should not immediately return an error.
	// We use a localhost URL that no browser will actually navigate to, but
	// cmd.Start() itself should succeed because the browser binary exists.
	err := OpenInBrowser("https://example.com")
	if err != nil {
		t.Fatalf("OpenInBrowser with valid URL returned error: %v", err)
	}
}

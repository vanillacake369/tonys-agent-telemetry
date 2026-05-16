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

// No test for valid URL — it actually opens a browser window.
// The empty-URL error path is sufficient for unit testing.

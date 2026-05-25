package control

import (
	"testing"
	"time"
)

func TestDenialLog_AppendThenRecent(t *testing.T) {
	dir := t.TempDir()
	log := NewDenialLog(dir)

	d1 := Denial{Timestamp: time.Now(), SessionID: "s1", Tool: "Bash:rm -rf /tmp", Reason: "tool_denylisted", Detail: "matched pattern Bash:rm -rf*"}
	d2 := Denial{Timestamp: time.Now(), SessionID: "s2", Tool: "WebFetch:evil.com", Reason: "tool_denylisted", Detail: "matched pattern WebFetch:*evil.com*"}
	d3 := Denial{Timestamp: time.Now(), SessionID: "s3", Tool: "Bash:dd if=/dev/zero", Reason: "budget_exceeded", Detail: "session cost $5.01 >= cap $5.00"}

	for _, d := range []Denial{d1, d2, d3} {
		if err := log.Append(d); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	recent, err := log.Recent(2)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("Recent(2) returned %d items, want 2", len(recent))
	}

	// Most recent first: d3, d2
	if recent[0].SessionID != "s3" {
		t.Errorf("recent[0].SessionID = %q, want %q", recent[0].SessionID, "s3")
	}
	if recent[1].SessionID != "s2" {
		t.Errorf("recent[1].SessionID = %q, want %q", recent[1].SessionID, "s2")
	}
}

func TestDenialLog_RecentFromEmpty(t *testing.T) {
	dir := t.TempDir()
	log := NewDenialLog(dir)

	recent, err := log.Recent(10)
	if err != nil {
		t.Fatalf("Recent from empty: %v", err)
	}
	if len(recent) != 0 {
		t.Errorf("got %d items from empty log, want 0", len(recent))
	}
}

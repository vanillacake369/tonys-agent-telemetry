package trends

import (
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signal"
)

func TestBucket_IsEmpty(t *testing.T) {
	cases := []struct {
		name string
		b    Bucket
		want bool
	}{
		{"zero value", Bucket{}, true},
		{"only sessions", Bucket{Sessions: 1}, false},
		{"only counts", Bucket{Counts: map[signal.SignalType]int{signal.SignalStalledNode: 1}}, false},
		{"both", Bucket{Sessions: 1, Counts: map[signal.SignalType]int{signal.SignalStalledNode: 1}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.b.IsEmpty(); got != c.want {
				t.Errorf("IsEmpty()=%v want %v", got, c.want)
			}
		})
	}
}

func TestBucket_Total(t *testing.T) {
	b := Bucket{
		Start:    time.Now(),
		Duration: 24 * time.Hour,
		Counts: map[signal.SignalType]int{
			signal.SignalStalledNode:            3,
			signal.SignalDuplicateSubagentWork:  2,
			signal.SignalFailedHandoff:          1,
		},
		Sessions: 4,
	}
	if got := b.Total(); got != 6 {
		t.Errorf("Total()=%d want 6", got)
	}
}

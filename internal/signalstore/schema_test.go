package signalstore_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/signalstore"
)

func TestCurrentSchemaVersion_IsNonEmpty(t *testing.T) {
	if signalstore.CurrentSchemaVersion == "" {
		t.Fatal("CurrentSchemaVersion must not be empty")
	}
}

func TestHeader_JSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 26, 9, 0, 0, 0, time.UTC)
	h := signalstore.Header{
		SchemaVersion: signalstore.CurrentSchemaVersion,
		WrittenAt:     now,
		Producer:      "tonys-agent-telemetry test",
	}

	b, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var got signalstore.Header
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if got.SchemaVersion != h.SchemaVersion {
		t.Errorf("SchemaVersion: got %q want %q", got.SchemaVersion, h.SchemaVersion)
	}
	if !got.WrittenAt.Equal(h.WrittenAt) {
		t.Errorf("WrittenAt: got %v want %v", got.WrittenAt, h.WrittenAt)
	}
	if got.Producer != h.Producer {
		t.Errorf("Producer: got %q want %q", got.Producer, h.Producer)
	}
}

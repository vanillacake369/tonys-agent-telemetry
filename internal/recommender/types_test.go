package recommender

import (
	"reflect"
	"testing"
	"time"
)

// TestRecommendation_HasRequiredEvidenceFields guards against accidental removal
// of the two mandatory citation fields. If either field is dropped from the
// struct, this test fails at compile+reflection time — not at runtime.
func TestRecommendation_HasRequiredEvidenceFields(t *testing.T) {
	rt := reflect.TypeOf(Recommendation{})

	requiredFields := []struct {
		name     string
		wantKind reflect.Kind
	}{
		{"SignalID", reflect.String},
		{"CatalogItemID", reflect.String},
	}

	for _, rf := range requiredFields {
		f, ok := rt.FieldByName(rf.name)
		if !ok {
			t.Errorf("Recommendation is missing required field %q — GA gate: no recommendation without evidence", rf.name)
			continue
		}
		if f.Type.Kind() != rf.wantKind {
			t.Errorf("field %q has kind %v, want %v", rf.name, f.Type.Kind(), rf.wantKind)
		}
	}
}

// TestRecommendation_HasSupportingFields verifies additional required fields
// are present with correct types.
func TestRecommendation_HasSupportingFields(t *testing.T) {
	rt := reflect.TypeOf(Recommendation{})

	cases := []struct {
		name     string
		wantType reflect.Type
	}{
		{"Title", reflect.TypeOf("")},
		{"Reasoning", reflect.TypeOf("")},
		{"Score", reflect.TypeOf(float64(0))},
		{"CreatedAt", reflect.TypeOf(time.Time{})},
	}

	for _, tc := range cases {
		f, ok := rt.FieldByName(tc.name)
		if !ok {
			t.Errorf("Recommendation is missing field %q", tc.name)
			continue
		}
		if f.Type != tc.wantType {
			t.Errorf("field %q has type %v, want %v", tc.name, f.Type, tc.wantType)
		}
	}
}

package planning

import (
	"encoding/json"
	"testing"
)

func TestValidateJSON(t *testing.T) {
	valid := ProjectPlanning{GoalsJSON: json.RawMessage(`{"selling_points":[],"plot_summary":""}`), ConstraintsJSON: json.RawMessage(`{"emotional_tone":""}`)}
	if err := validateJSON(valid); err != nil {
		t.Fatalf("valid JSON rejected: %v", err)
	}
	valid.GoalsJSON = json.RawMessage(`{`)
	if err := validateJSON(valid); err != ErrInvalidJSON {
		t.Fatalf("invalid JSON error = %v", err)
	}
}

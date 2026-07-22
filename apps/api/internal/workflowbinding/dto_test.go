package workflowbinding

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestWorkflowConfigurationSummaryDTOSerializesTypeConfigAsObject(t *testing.T) {
	dto := WorkflowConfigurationSummaryDTO{
		ID:                    uuid.New(),
		Name:                  "test",
		ConnectionID:          uuid.New(),
		ConnectionName:        "conn",
		ConnectionType:        "n8n",
		WorkflowType:          "n8n",
		ApplicableStages:      []string{"chapter_planning"},
		TypeConfig:            json.RawMessage(`{"referenceType":"workflow_id","referenceValue":"wf-1"}`),
		InputContractVersion:  "v1",
		OutputContractVersion: "v1",
		DefaultParameters:     json.RawMessage(`{"temperature":0.7}`),
		IntegrationStatus:     "not_connected",
		Enabled:               true,
		Version:               1,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	}
	out, err := marshalJSON(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(out)
	for _, want := range []string{`"typeConfig":{`, `"defaultParameters":{`, `"referenceType"`, `"temperature":0.7`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	// Must not be Base64 string shapes.
	if strings.Contains(text, `"typeConfig":"`) || strings.Contains(text, `"defaultParameters":"`) {
		t.Fatalf("json.RawMessage fields serialized as strings: %s", text)
	}
}

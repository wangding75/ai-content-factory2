package workflowbinding

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ReadWorkflowConfiguration is the read-only subset of the Iteration 12 global
// workflow configuration needed by the binding read model.  It mirrors the
// frozen WorkflowConfiguration schema and is reused verbatim as the
// workflowConfigurationSummary in both GET and PUT responses.
type ReadWorkflowConfiguration struct {
	ID                    uuid.UUID       `json:"id"`
	Name                  string          `json:"name"`
	ConnectionID          uuid.UUID       `json:"connectionId"`
	ConnectionName        string          `json:"connectionName"`
	ConnectionType        string          `json:"connectionType"`
	WorkflowType          string          `json:"workflowType"`
	ApplicableStages      []string        `json:"applicableStages"`
	TypeConfig            json.RawMessage `json:"typeConfig"`
	InputContractVersion  string          `json:"inputContractVersion"`
	OutputContractVersion string          `json:"outputContractVersion"`
	DefaultParameters     json.RawMessage `json:"defaultParameters"`
	Note                  *string         `json:"note"`
	IntegrationStatus     string          `json:"integrationStatus"`
	Enabled               bool            `json:"enabled"`
	LastVerifiedAt        *time.Time      `json:"lastVerifiedAt"`
	LastErrorCode         *string         `json:"lastErrorCode"`
	LastErrorMessage      *string         `json:"lastErrorMessage"`
	Version               int             `json:"version"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
}

// StageRead is the internal per-stage read model returned by GET and PUT.  The
// HTTP layer converts it to the DTO before serialization so the domain entity
// is never serialized directly.
type StageRead struct {
	Stage                        WorkflowBindingStage
	Bound                        bool
	Binding                      *ProjectWorkflowBinding
	WorkflowConfigurationSummary *ReadWorkflowConfiguration
}

// UnbindResult is the internal DELETE result.
type UnbindResult struct {
	ProjectID                     uuid.UUID
	Stage                         WorkflowBindingStage
	Unbound                       bool
	WorkflowConfigurationRetained bool
}

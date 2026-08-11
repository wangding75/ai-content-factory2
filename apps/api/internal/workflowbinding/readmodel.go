package workflowbinding

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
)

// ReadWorkflowConfiguration is the read-only subset of the Iteration 12 global
// workflow configuration needed by the binding read model.  It mirrors the
// frozen WorkflowConfiguration schema and is reused verbatim as the
// workflowConfigurationSummary in both GET and PUT responses.
type ReadWorkflowConfiguration struct {
	ID                    uuid.UUID             `json:"id"`
	Name                  string                `json:"name"`
	ConnectionID          uuid.UUID             `json:"connectionId"`
	ConnectionName        string                `json:"connectionName"`
	ConnectionType        string                `json:"connectionType"`
	WorkflowType          string                `json:"workflowType"`
	ApplicableStages      []string              `json:"applicableStages"`
	TypeConfig            json.RawMessage       `json:"typeConfig"`
	InputContractVersion  string                `json:"inputContractVersion"`
	OutputContractVersion string                `json:"outputContractVersion"`
	DefaultParameters     json.RawMessage       `json:"defaultParameters"`
	Note                  *string               `json:"note"`
	IntegrationStatus     string                `json:"integrationStatus"`
	ValidationStatus      string                `json:"validationStatus"`
	Enabled               bool                  `json:"enabled"`
	Executable            bool                  `json:"executable"`
	IneligibilityReasons  []NonExecutableReason `json:"ineligibilityReasons"`
	VerifiedVersion       *int                  `json:"verifiedVersion"`
	ValidationDetails     json.RawMessage       `json:"validationDetails"`
	LlmStrategy           string                `json:"llmStrategy"`
	LlmProviderID         *uuid.UUID            `json:"llmProviderId"`
	LlmModel              *string               `json:"llmModel"`
	LastVerifiedAt        *time.Time            `json:"lastVerifiedAt"`
	LastErrorCode         *string               `json:"lastErrorCode"`
	LastErrorMessage      *string               `json:"lastErrorMessage"`
	Version               int                   `json:"version"`
	CreatedAt             time.Time             `json:"createdAt"`
	UpdatedAt             time.Time             `json:"updatedAt"`
}

// StageRead is the internal per-stage read model returned by GET and PUT.  The
// HTTP layer converts it to the DTO before serialization so the domain entity
// is never serialized directly.
type StageRead struct {
	Stage                        WorkflowBindingStage
	Bound                        bool
	Executable                   bool
	IneligibilityReasons         []NonExecutableReason
	Binding                      *ProjectWorkflowBinding
	WorkflowConfigurationSummary *ReadWorkflowConfiguration
}

// WorkflowBindingCandidate is a stage-compatible workflow configuration with
// the evaluated dependency facts the project UI needs to decide whether it can
// be selected.  It deliberately keeps the global configuration read-only.
type WorkflowBindingCandidate struct {
	Stage                 WorkflowBindingStage
	Selectable            bool
	Executable            bool
	IneligibilityReasons  []NonExecutableReason
	WorkflowConfiguration ReadWorkflowConfiguration
	ConnectionSummary     ConnectionSummary
	LlmPolicySummary      LlmPolicySummary
}

// ConnectionSummary exposes only the safe connection facts relevant to a
// binding candidate.  Credentials and transport details are never included.
type ConnectionSummary struct {
	ID               uuid.UUID
	Name             string
	ConnectionType   string
	ValidationStatus string
	Enabled          bool
	Executable       bool
}

// LlmPolicySummary exposes the configured LLM policy without secrets.  For
// strategies that do not use an ACF provider, provider fields are nil.
type LlmPolicySummary struct {
	Strategy         string
	ProviderID       *uuid.UUID
	ProviderName     *string
	ProviderVersion  *int
	Model            *string
	ValidationStatus *string
	Executable       bool
}

type NonExecutableReason struct {
	Code         string        `json:"code"`
	Message      string        `json:"message"`
	RepairAction string        `json:"repairAction,omitempty"`
	RepairTarget *RepairTarget `json:"repairTarget,omitempty"`
}

// RepairTarget is shared with global execution eligibility responses.
type RepairTarget = globalconfig.RepairTarget

// UnbindResult is the internal DELETE result.
type UnbindResult struct {
	ProjectID                     uuid.UUID
	Stage                         WorkflowBindingStage
	Unbound                       bool
	WorkflowConfigurationRetained bool
}

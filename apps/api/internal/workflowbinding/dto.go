package workflowbinding

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// BindingDTO is the wire shape of a single ProjectWorkflowBinding.  It exposes
// exactly the frozen ProjectWorkflowBinding fields in lowerCamelCase and
// nothing else, so the domain entity is never serialized directly.
type BindingDTO struct {
	ID                      uuid.UUID `json:"id"`
	ProjectID               uuid.UUID `json:"projectId"`
	Stage                   string    `json:"stage"`
	WorkflowConfigurationID uuid.UUID `json:"workflowConfigurationId"`
	Version                 int       `json:"version"`
	CreatedAt               time.Time `json:"createdAt"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

func bindingDTO(b ProjectWorkflowBinding) BindingDTO {
	return BindingDTO{
		ID:                      b.ID,
		ProjectID:               b.ProjectID,
		Stage:                   b.Stage.String(),
		WorkflowConfigurationID: b.WorkflowConfigurationID,
		Version:                 b.Version,
		CreatedAt:               b.CreatedAt,
		UpdatedAt:               b.UpdatedAt,
	}
}

// WorkflowConfigurationSummaryDTO is the wire shape of the read-only global
// workflow configuration summary.  It mirrors ReadWorkflowConfiguration and is
// rendered verbatim as workflowConfigurationSummary.
type WorkflowConfigurationSummaryDTO struct {
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

func summaryDTO(s *ReadWorkflowConfiguration) *WorkflowConfigurationSummaryDTO {
	if s == nil {
		return nil
	}
	return &WorkflowConfigurationSummaryDTO{
		ID:                    s.ID,
		Name:                  s.Name,
		ConnectionID:          s.ConnectionID,
		ConnectionName:        s.ConnectionName,
		ConnectionType:        s.ConnectionType,
		WorkflowType:          s.WorkflowType,
		ApplicableStages:      s.ApplicableStages,
		TypeConfig:            s.TypeConfig,
		InputContractVersion:  s.InputContractVersion,
		OutputContractVersion: s.OutputContractVersion,
		DefaultParameters:     s.DefaultParameters,
		Note:                  s.Note,
		IntegrationStatus:     s.IntegrationStatus,
		ValidationStatus:      s.ValidationStatus,
		Enabled:               s.Enabled,
		Executable:            s.Executable,
		IneligibilityReasons:  s.IneligibilityReasons,
		VerifiedVersion:       s.VerifiedVersion,
		ValidationDetails:     s.ValidationDetails,
		LlmStrategy:           s.LlmStrategy,
		LlmProviderID:         s.LlmProviderID,
		LlmModel:              s.LlmModel,
		LastVerifiedAt:        s.LastVerifiedAt,
		LastErrorCode:         s.LastErrorCode,
		LastErrorMessage:      s.LastErrorMessage,
		Version:               s.Version,
		CreatedAt:             s.CreatedAt,
		UpdatedAt:             s.UpdatedAt,
	}
}

// WorkflowBindingStageDTO is the wire shape of a single stage in GET and the
// full response of PUT (create / replace / no-op).  bound is true only when a
// binding exists; workflowConfigurationSummary is the full global workflow for
// bound stages and null for unbound stages.
type WorkflowBindingStageDTO struct {
	Stage                        string                           `json:"stage"`
	Bound                        bool                             `json:"bound"`
	Executable                   bool                             `json:"executable"`
	IneligibilityReasons         []NonExecutableReason            `json:"ineligibilityReasons"`
	Binding                      *BindingDTO                      `json:"binding"`
	WorkflowConfigurationSummary *WorkflowConfigurationSummaryDTO `json:"workflowConfigurationSummary"`
}

func stageDTO(s StageRead) WorkflowBindingStageDTO {
	dto := WorkflowBindingStageDTO{
		Stage:                        s.Stage.String(),
		Bound:                        s.Bound,
		Executable:                   s.Executable,
		IneligibilityReasons:         s.IneligibilityReasons,
		WorkflowConfigurationSummary: summaryDTO(s.WorkflowConfigurationSummary),
	}
	if s.Binding != nil {
		b := bindingDTO(*s.Binding)
		dto.Binding = &b
	}
	return dto
}

// WorkflowBindingCandidateDTO is the wire shape of an evaluated candidate.
// Each nested object is deliberately explicit to preserve the frozen OpenAPI
// contract and prevent credentials from crossing the boundary.
type WorkflowBindingCandidateDTO struct {
	Stage                 string                          `json:"stage"`
	Selectable            bool                            `json:"selectable"`
	Executable            bool                            `json:"executable"`
	IneligibilityReasons  []NonExecutableReason           `json:"ineligibilityReasons"`
	WorkflowConfiguration WorkflowConfigurationSummaryDTO `json:"workflowConfiguration"`
	ConnectionSummary     ConnectionSummaryDTO            `json:"connectionSummary"`
	LlmPolicySummary      LlmPolicySummaryDTO             `json:"llmPolicySummary"`
}

type ConnectionSummaryDTO struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	ConnectionType   string    `json:"connectionType"`
	ValidationStatus string    `json:"validationStatus"`
	Enabled          bool      `json:"enabled"`
	Executable       bool      `json:"executable"`
}

type LlmPolicySummaryDTO struct {
	Strategy         string     `json:"strategy"`
	ProviderID       *uuid.UUID `json:"providerId"`
	ProviderName     *string    `json:"providerName"`
	ProviderVersion  *int       `json:"providerVersion"`
	Model            *string    `json:"model"`
	ValidationStatus *string    `json:"validationStatus"`
	Executable       bool       `json:"executable"`
}

func candidateDTO(c WorkflowBindingCandidate) WorkflowBindingCandidateDTO {
	workflow := summaryDTO(&c.WorkflowConfiguration)
	return WorkflowBindingCandidateDTO{
		Stage:                 c.Stage.String(),
		Selectable:            c.Selectable,
		Executable:            c.Executable,
		IneligibilityReasons:  c.IneligibilityReasons,
		WorkflowConfiguration: *workflow,
		ConnectionSummary: ConnectionSummaryDTO{
			ID: c.ConnectionSummary.ID, Name: c.ConnectionSummary.Name, ConnectionType: c.ConnectionSummary.ConnectionType,
			ValidationStatus: c.ConnectionSummary.ValidationStatus, Enabled: c.ConnectionSummary.Enabled, Executable: c.ConnectionSummary.Executable,
		},
		LlmPolicySummary: LlmPolicySummaryDTO{
			Strategy: c.LlmPolicySummary.Strategy, ProviderID: c.LlmPolicySummary.ProviderID, ProviderName: c.LlmPolicySummary.ProviderName,
			ProviderVersion: c.LlmPolicySummary.ProviderVersion, Model: c.LlmPolicySummary.Model,
			ValidationStatus: c.LlmPolicySummary.ValidationStatus, Executable: c.LlmPolicySummary.Executable,
		},
	}
}

// UnbindResultDTO is the wire shape of the DELETE response.
type UnbindResultDTO struct {
	ProjectID                     uuid.UUID `json:"projectId"`
	Stage                         string    `json:"stage"`
	Unbound                       bool      `json:"unbound"`
	WorkflowConfigurationRetained bool      `json:"workflowConfigurationRetained"`
}

func unbindDTO(r UnbindResult) UnbindResultDTO {
	return UnbindResultDTO{
		ProjectID:                     r.ProjectID,
		Stage:                         r.Stage.String(),
		Unbound:                       r.Unbound,
		WorkflowConfigurationRetained: r.WorkflowConfigurationRetained,
	}
}

// StageDTO converts an internal StageRead to the wire DTO.  Exported so the
// httpserver package can render GET responses without re-implementing the
// mapping (the domain entity must never be serialized directly).
func StageDTO(s StageRead) WorkflowBindingStageDTO { return stageDTO(s) }

// CandidateDTO converts an evaluated candidate to the frozen HTTP shape.
func CandidateDTO(c WorkflowBindingCandidate) WorkflowBindingCandidateDTO { return candidateDTO(c) }

// UnbindDTO converts an internal UnbindResult to the wire DTO.
func UnbindDTO(r UnbindResult) UnbindResultDTO { return unbindDTO(r) }

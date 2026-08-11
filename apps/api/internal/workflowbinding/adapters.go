package workflowbinding

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

// projectAuthorizer adapts the project Repository to the Service contract.
type projectAuthorizer struct{ repo project.Repository }

func (a projectAuthorizer) ExistsForModify(ctx context.Context, id uuid.UUID) error {
	if _, err := a.repo.Get(ctx, id); err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return ErrProjectNotFound
		}
		return err
	}
	return nil
}

// workflowReader adapts the Iteration 12 globalconfig Service.
type workflowReader struct{ svc *globalconfig.Service }

func (w workflowReader) GetWorkflow(ctx context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
	wf, err := w.svc.GetWorkflow(ctx, id)
	if err != nil {
		if errors.Is(err, globalconfig.ErrNotFound) {
			return ReadWorkflowConfiguration{}, ErrConfigurationNotFound
		}
		return ReadWorkflowConfiguration{}, err
	}
	return readWorkflowConfiguration(wf), nil
}

func (w workflowReader) ListWorkflowBindingCandidates(ctx context.Context, stage WorkflowBindingStage, query string, limit, offset int) ([]WorkflowBindingCandidate, int, error) {
	workflows, total, err := w.svc.ListWorkflows(ctx, globalconfig.ListOptions{
		Query:           query,
		ApplicableStage: stage.String(),
		Limit:           limit,
		Offset:          offset,
	})
	if err != nil {
		return nil, 0, err
	}
	candidates := make([]WorkflowBindingCandidate, 0, len(workflows))
	for _, workflow := range workflows {
		connection, err := w.svc.GetConnection(ctx, workflow.ConnectionID)
		if err != nil {
			return nil, 0, err
		}
		policy := LlmPolicySummary{
			Strategy:   workflow.LlmStrategy,
			ProviderID: workflow.LlmProviderID,
			Model:      workflow.LlmModel,
			Executable: workflow.LlmStrategy == "none" || workflow.LlmStrategy == "n8n_managed",
		}
		if workflow.LlmStrategy == "acf_managed" && workflow.LlmProviderID != nil {
			provider, providerErr := w.svc.GetProvider(ctx, *workflow.LlmProviderID)
			if providerErr == nil {
				name, status := provider.Name, provider.ValidationStatus
				version := provider.Version
				policy.ProviderName = &name
				policy.ProviderVersion = &version
				policy.ValidationStatus = &status
				policy.Executable = provider.Executable && !hasCandidateReason(workflow.IneligibilityReasons, "model_unavailable")
			}
		}
		reasons := make([]NonExecutableReason, len(workflow.IneligibilityReasons))
		for index, reason := range workflow.IneligibilityReasons {
			reasons[index] = NonExecutableReason{Code: reason.Code, Message: reason.Message, RepairAction: reason.RepairAction, RepairTarget: reason.RepairTarget}
		}
		candidates = append(candidates, WorkflowBindingCandidate{
			Stage:                 stage,
			Selectable:            workflow.Executable,
			Executable:            workflow.Executable,
			IneligibilityReasons:  reasons,
			WorkflowConfiguration: readWorkflowConfiguration(workflow),
			ConnectionSummary: ConnectionSummary{
				ID:               connection.ID,
				Name:             connection.Name,
				ConnectionType:   connection.ConnectionType,
				ValidationStatus: connection.ValidationStatus,
				Enabled:          connection.Enabled,
				Executable:       connection.Executable,
			},
			LlmPolicySummary: policy,
		})
	}
	return candidates, total, nil
}

func hasCandidateReason(reasons []globalconfig.IneligibilityReason, code string) bool {
	for _, reason := range reasons {
		if strings.EqualFold(reason.Code, code) {
			return true
		}
	}
	return false
}

func readWorkflowConfiguration(wf globalconfig.Workflow) ReadWorkflowConfiguration {
	return ReadWorkflowConfiguration{
		ID:                    wf.ID,
		Name:                  wf.Name,
		ConnectionID:          wf.ConnectionID,
		ConnectionName:        wf.ConnectionName,
		ConnectionType:        wf.ConnectionType,
		WorkflowType:          wf.WorkflowType,
		ApplicableStages:      wf.ApplicableStages,
		TypeConfig:            wf.TypeConfig,
		InputContractVersion:  wf.InputContractVersion,
		OutputContractVersion: wf.OutputContractVersion,
		DefaultParameters:     wf.DefaultParameters,
		Note:                  wf.Note,
		IntegrationStatus:     wf.IntegrationStatus,
		ValidationStatus:      wf.ValidationStatus,
		Enabled:               wf.Enabled,
		Executable:            wf.Executable,
		IneligibilityReasons: func() []NonExecutableReason {
			reasons := make([]NonExecutableReason, len(wf.IneligibilityReasons))
			for index, reason := range wf.IneligibilityReasons {
				reasons[index] = NonExecutableReason{Code: reason.Code, Message: reason.Message, RepairAction: reason.RepairAction, RepairTarget: reason.RepairTarget}
			}
			return reasons
		}(),
		VerifiedVersion:   wf.VerifiedVersion,
		ValidationDetails: wf.ValidationDetails,
		LlmStrategy:       wf.LlmStrategy,
		LlmProviderID:     wf.LlmProviderID,
		LlmModel:          wf.LlmModel,
		LastVerifiedAt:    wf.LastVerifiedAt,
		LastErrorCode:     wf.LastErrorCode,
		LastErrorMessage:  wf.LastErrorMessage,
		Version:           wf.Version,
		CreatedAt:         wf.CreatedAt,
		UpdatedAt:         wf.UpdatedAt,
	}
}

// NewProjectAuthorizer builds the project authorization adapter.
func NewProjectAuthorizer(repo project.Repository) ProjectRepository {
	return projectAuthorizer{repo: repo}
}

// NewWorkflowReader builds the global workflow configuration adapter.
func NewWorkflowReader(svc *globalconfig.Service) WorkflowRepository {
	return workflowReader{svc: svc}
}

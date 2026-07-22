package workflowbinding

import (
	"context"
	"errors"

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
		Enabled:               wf.Enabled,
		LastVerifiedAt:        wf.LastVerifiedAt,
		LastErrorCode:         wf.LastErrorCode,
		LastErrorMessage:      wf.LastErrorMessage,
		Version:               wf.Version,
		CreatedAt:             wf.CreatedAt,
		UpdatedAt:             wf.UpdatedAt,
	}, nil
}

// NewProjectAuthorizer builds the project authorization adapter.
func NewProjectAuthorizer(repo project.Repository) ProjectRepository {
	return projectAuthorizer{repo: repo}
}

// NewWorkflowReader builds the global workflow configuration adapter.
func NewWorkflowReader(svc *globalconfig.Service) WorkflowRepository {
	return workflowReader{svc: svc}
}

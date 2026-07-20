package workflowbinding

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type WorkflowBindingStage string

const (
	StageChapterPlanning   WorkflowBindingStage = "chapter_planning"
	StageContentGeneration WorkflowBindingStage = "content_generation"
	StageReview            WorkflowBindingStage = "review"
	StageRewrite           WorkflowBindingStage = "rewrite"
)

var (
	ErrInvalidStage  = errors.New("invalid workflow binding stage")
	ErrValidation    = errors.New("workflow binding validation failed")
	ErrNoChange      = errors.New("workflow binding unchanged")
)

var validStages = map[WorkflowBindingStage]bool{
	StageChapterPlanning:   true,
	StageContentGeneration: true,
	StageReview:            true,
	StageRewrite:           true,
}

func ParseStage(raw string) (WorkflowBindingStage, error) {
	s := WorkflowBindingStage(raw)
	if !validStages[s] {
		return "", ErrInvalidStage
	}
	return s, nil
}

func (s WorkflowBindingStage) String() string {
	return string(s)
}

func AllStages() []WorkflowBindingStage {
	return []WorkflowBindingStage{
		StageChapterPlanning,
		StageContentGeneration,
		StageReview,
		StageRewrite,
	}
}

type ProjectWorkflowBinding struct {
	ID                      uuid.UUID
	ProjectID               uuid.UUID
	Stage                   WorkflowBindingStage
	WorkflowConfigurationID uuid.UUID
	Version                 int
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

func New(id, projectID, workflowConfigurationID uuid.UUID, stage WorkflowBindingStage) (ProjectWorkflowBinding, error) {
	if id == uuid.Nil {
		return ProjectWorkflowBinding{}, ErrValidation
	}
	if projectID == uuid.Nil {
		return ProjectWorkflowBinding{}, ErrValidation
	}
	if workflowConfigurationID == uuid.Nil {
		return ProjectWorkflowBinding{}, ErrValidation
	}
	if !validStages[stage] {
		return ProjectWorkflowBinding{}, ErrInvalidStage
	}
	now := time.Now().UTC()
	return ProjectWorkflowBinding{
		ID:                      id,
		ProjectID:               projectID,
		Stage:                   stage,
		WorkflowConfigurationID: workflowConfigurationID,
		Version:                 1,
		CreatedAt:               now,
		UpdatedAt:               now,
	}, nil
}

func NewFromDB(id, projectID, workflowConfigurationID uuid.UUID, stage WorkflowBindingStage, version int, createdAt, updatedAt time.Time) (ProjectWorkflowBinding, error) {
	if id == uuid.Nil {
		return ProjectWorkflowBinding{}, ErrValidation
	}
	if projectID == uuid.Nil {
		return ProjectWorkflowBinding{}, ErrValidation
	}
	if workflowConfigurationID == uuid.Nil {
		return ProjectWorkflowBinding{}, ErrValidation
	}
	if !validStages[stage] {
		return ProjectWorkflowBinding{}, ErrInvalidStage
	}
	if version < 1 {
		return ProjectWorkflowBinding{}, ErrValidation
	}
	return ProjectWorkflowBinding{
		ID:                      id,
		ProjectID:               projectID,
		Stage:                   stage,
		WorkflowConfigurationID: workflowConfigurationID,
		Version:                 version,
		CreatedAt:               createdAt,
		UpdatedAt:               updatedAt,
	}, nil
}

func (b ProjectWorkflowBinding) RebindTo(newWorkflowConfigurationID uuid.UUID) (ProjectWorkflowBinding, error) {
	if newWorkflowConfigurationID == uuid.Nil {
		return ProjectWorkflowBinding{}, ErrValidation
	}
	if b.WorkflowConfigurationID == newWorkflowConfigurationID {
		return b, ErrNoChange
	}
	b.WorkflowConfigurationID = newWorkflowConfigurationID
	b.Version++
	b.UpdatedAt = time.Now().UTC()
	return b, nil
}
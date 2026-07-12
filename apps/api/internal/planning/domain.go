package planning

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("project planning not found")
	ErrVersionConflict = errors.New("project planning version conflict")
	ErrInvalidJSON     = errors.New("project planning contains invalid json")
	ErrValidation      = errors.New("project planning validation failed")
)

type ProjectPlanning struct {
	ProjectID       uuid.UUID       `json:"project_id"`
	Premise         string          `json:"premise"`
	Audience        string          `json:"audience"`
	Style           string          `json:"style"`
	GoalsJSON       json.RawMessage `json:"goals_json"`
	ConstraintsJSON json.RawMessage `json:"constraints_json"`
	CreatedBy       string          `json:"-"`
	Version         int             `json:"version"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type SaveRequest struct {
	Premise         *string         `json:"premise"`
	Audience        *string         `json:"audience"`
	Style           *string         `json:"style"`
	GoalsJSON       json.RawMessage `json:"goals_json"`
	ConstraintsJSON json.RawMessage `json:"constraints_json"`
	ExpectedVersion *int            `json:"expected_version"`
}

type Response struct {
	ProjectID       uuid.UUID       `json:"project_id"`
	Premise         string          `json:"premise"`
	Audience        string          `json:"audience"`
	Style           string          `json:"style"`
	GoalsJSON       json.RawMessage `json:"goals_json"`
	ConstraintsJSON json.RawMessage `json:"constraints_json"`
	Version         int             `json:"version"`
	CreatedAt       *time.Time      `json:"created_at"`
	UpdatedAt       *time.Time      `json:"updated_at"`
}

type Repository interface {
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (ProjectPlanning, error)
	Create(ctx context.Context, value ProjectPlanning) (ProjectPlanning, error)
	UpdateWithVersion(ctx context.Context, value ProjectPlanning, expectedVersion int) (ProjectPlanning, error)
}

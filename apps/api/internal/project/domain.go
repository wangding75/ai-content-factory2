package project

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TypeNovel         = "novel"
	StatusPlanning    = "planning"
	StageProjectSetup = "project_setup"
)

var (
	ErrNotFound   = errors.New("project not found")
	ErrValidation = errors.New("project validation failed")
)

type Project struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Description  string    `json:"description"`
	CurrentStage string    `json:"current_stage"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type ListOptions struct {
	Status string
	Query  string
	Limit  int
	Offset int
}
type Workspace struct {
	Project  Project  `json:"project"`
	Progress Progress `json:"progress"`
}
type Progress struct {
	MaterialCount         int `json:"material_count"`
	StorylineCount        int `json:"storyline_count"`
	ConfirmedChapterCount int `json:"confirmed_chapter_count"`
	WorkCount             int `json:"work_count"`
}
type Repository interface {
	Create(context.Context, Project, string) (Project, error)
	List(context.Context, ListOptions) ([]Project, int, error)
	Get(context.Context, uuid.UUID) (Project, error)
	Update(context.Context, uuid.UUID, *string, *string) (Project, error)
}

func New(name, projectType, description string) (Project, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 || projectType != TypeNovel || len(description) > 5000 {
		return Project{}, ErrValidation
	}
	return Project{ID: uuid.New(), Name: name, Type: TypeNovel, Status: StatusPlanning, Description: description, CurrentStage: StageProjectSetup}, nil
}
func ValidateUpdate(name, description *string) error {
	if name == nil && description == nil {
		return ErrValidation
	}
	if name != nil && (strings.TrimSpace(*name) == "" || len(*name) > 120) {
		return ErrValidation
	}
	if description != nil && len(*description) > 5000 {
		return ErrValidation
	}
	return nil
}

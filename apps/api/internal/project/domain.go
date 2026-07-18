package project

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	TypeShortFilm     = "short_film"
	TypeSeries        = "series"
	TypeGraphicText   = "graphic_text"
	TypeImage         = "image"
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
type TypeDescriptor struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	SortOrder   int    `json:"sort_order"`
}

var projectTypes = []TypeDescriptor{
	{Code: TypeNovel, Name: "小说", Description: "用于长篇小说、大纲与章节创作。", Enabled: true, SortOrder: 10},
	{Code: TypeShortFilm, Name: "短片", Description: "用于短片脚本与分镜创作。", Enabled: true, SortOrder: 20},
	{Code: TypeSeries, Name: "剧集", Description: "用于剧集策划与分集创作。", Enabled: true, SortOrder: 30},
	{Code: TypeGraphicText, Name: "图文", Description: "用于图文内容策划与创作。", Enabled: true, SortOrder: 40},
	{Code: TypeImage, Name: "图片", Description: "用于图片内容策划与创作。", Enabled: true, SortOrder: 50},
}

func ProjectTypes() []TypeDescriptor {
	items := make([]TypeDescriptor, len(projectTypes))
	copy(items, projectTypes)
	return items
}
func IsEnabledProjectType(code string) bool {
	for _, item := range projectTypes {
		if item.Code == code {
			return item.Enabled
		}
	}
	return false
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
type ProgressReader interface {
	Progress(context.Context, uuid.UUID) (Progress, error)
}

func New(name, projectType, description string) (Project, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 || !IsEnabledProjectType(projectType) || len(description) > 5000 {
		return Project{}, ErrValidation
	}
	return Project{ID: uuid.New(), Name: name, Type: projectType, Status: StatusPlanning, Description: description, CurrentStage: StageProjectSetup}, nil
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

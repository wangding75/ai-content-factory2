package material

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type ProjectMaterialItem struct {
	Material      Material             `json:"material"`
	Usage         ProjectMaterialUsage `json:"usage"`
	LastUpdatedAt time.Time            `json:"last_updated_at"`
}

type ProjectMaterialTypeCounts struct {
	Character    int `json:"character"`
	Worldview    int `json:"worldview"`
	Location     int `json:"location"`
	Organization int `json:"organization"`
	Item         int `json:"item"`
	Reference    int `json:"reference"`
}

type ProjectMaterialList struct {
	Items      []ProjectMaterialItem     `json:"items"`
	Total      int                       `json:"total"`
	Limit      int                       `json:"limit"`
	Offset     int                       `json:"offset"`
	TypeCounts ProjectMaterialTypeCounts `json:"type_counts"`
}

type ProjectMaterialRepository interface {
	ListProjectMaterials(context.Context, uuid.UUID, ListOptions) ([]ProjectMaterialItem, int, error)
	ProjectMaterialTypeCounts(context.Context, uuid.UUID) (ProjectMaterialTypeCounts, error)
}

type projectFinder interface {
	Get(context.Context, uuid.UUID) (project.Project, error)
}

type ProjectMaterialService struct {
	projects projectFinder
	repo     ProjectMaterialRepository
	pool     *pgxpool.Pool
}

func NewProjectMaterialService(projects projectFinder, repo ProjectMaterialRepository) *ProjectMaterialService {
	return &ProjectMaterialService{projects: projects, repo: repo}
}

func (s *ProjectMaterialService) List(ctx context.Context, projectID uuid.UUID, options ListOptions) (ProjectMaterialList, error) {
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return ProjectMaterialList{}, err
	}
	items, total, err := s.repo.ListProjectMaterials(ctx, projectID, options)
	if err != nil {
		return ProjectMaterialList{}, err
	}
	counts, err := s.repo.ProjectMaterialTypeCounts(ctx, projectID)
	if err != nil {
		return ProjectMaterialList{}, err
	}
	if items == nil {
		items = []ProjectMaterialItem{}
	}
	return ProjectMaterialList{Items: items, Total: total, Limit: options.Limit, Offset: options.Offset, TypeCounts: counts}, nil
}

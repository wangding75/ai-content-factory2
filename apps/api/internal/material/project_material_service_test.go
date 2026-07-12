package material

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type projectMaterialTestProjects struct {
	err error
}

func (p projectMaterialTestProjects) Get(context.Context, uuid.UUID) (project.Project, error) {
	return project.Project{}, p.err
}

type projectMaterialTestRepo struct {
	items  []ProjectMaterialItem
	total  int
	counts ProjectMaterialTypeCounts
	err    error
	seenID uuid.UUID
	seen   ListOptions
}

func (r *projectMaterialTestRepo) ListProjectMaterials(_ context.Context, id uuid.UUID, options ListOptions) ([]ProjectMaterialItem, int, error) {
	r.seenID, r.seen = id, options
	return r.items, r.total, r.err
}
func (r *projectMaterialTestRepo) ProjectMaterialTypeCounts(_ context.Context, id uuid.UUID) (ProjectMaterialTypeCounts, error) {
	r.seenID = id
	return r.counts, r.err
}

func TestProjectMaterialServiceNotFoundAndRepositoryErrors(t *testing.T) {
	id := uuid.New()
	missing := NewProjectMaterialService(projectMaterialTestProjects{err: project.ErrNotFound}, &projectMaterialTestRepo{})
	if _, err := missing.List(context.Background(), id, ListOptions{Limit: 20}); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("missing project error = %v", err)
	}
	repoErr := errors.New("repository unavailable")
	service := NewProjectMaterialService(projectMaterialTestProjects{}, &projectMaterialTestRepo{err: repoErr})
	if _, err := service.List(context.Background(), id, ListOptions{Limit: 20}); !errors.Is(err, repoErr) {
		t.Fatalf("repository error = %v", err)
	}
}

func TestProjectMaterialServiceReturnsScopedItemsPaginationAndCounts(t *testing.T) {
	projectID, otherProjectID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	item := ProjectMaterialItem{Material: Material{ID: uuid.New(), Type: TypeCharacter, Name: "hero"}, Usage: ProjectMaterialUsage{ID: uuid.New(), ProjectID: projectID, UsageType: "lead"}, LastUpdatedAt: now}
	repo := &projectMaterialTestRepo{items: []ProjectMaterialItem{item}, total: 2, counts: ProjectMaterialTypeCounts{Character: 2}}
	service := NewProjectMaterialService(projectMaterialTestProjects{}, repo)
	options := ListOptions{Query: "hero", Type: TypeCharacter, Sort: "name_asc", Limit: 1, Offset: 1}
	result, err := service.List(context.Background(), projectID, options)
	if err != nil || result.Total != 2 || len(result.Items) != 1 || result.Items[0].Usage.ProjectID != projectID || result.Limit != 1 || result.Offset != 1 || result.TypeCounts.Character != 2 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if repo.seenID != projectID || repo.seen != options || result.Items[0].Usage.ProjectID == otherProjectID {
		t.Fatalf("scope was not preserved: %#v", repo)
	}
}

func TestProjectMaterialServiceEmptyListIsNonNil(t *testing.T) {
	service := NewProjectMaterialService(projectMaterialTestProjects{}, &projectMaterialTestRepo{})
	result, err := service.List(context.Background(), uuid.New(), ListOptions{Limit: 20})
	if err != nil || result.Items == nil || len(result.Items) != 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

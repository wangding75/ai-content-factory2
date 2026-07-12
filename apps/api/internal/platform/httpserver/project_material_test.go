package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type fakeProjectMaterials struct {
	result  material.ProjectMaterialList
	err     error
	options material.ListOptions
}

func (f *fakeProjectMaterials) BindExistingMaterial(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ material.ProjectMaterialUsageRequest, _ string, _ string) (material.ProjectMaterialItem, error) {
	return material.ProjectMaterialItem{}, f.err
}

func (f *fakeProjectMaterials) CreateAndBindMaterial(_ context.Context, _ uuid.UUID, _ material.CreateProjectMaterialRequest, _ string, _ string) (material.ProjectMaterialItem, error) {
	return material.ProjectMaterialItem{}, f.err
}

func (f *fakeProjectMaterials) List(_ context.Context, _ uuid.UUID, options material.ListOptions) (material.ProjectMaterialList, error) {
	f.options = options
	return f.result, f.err
}

type fakeProjectMaterialRepository struct{ result material.ProjectMaterialList }

func (f fakeProjectMaterialRepository) ListProjectMaterials(_ context.Context, _ uuid.UUID, _ material.ListOptions) ([]material.ProjectMaterialItem, int, error) {
	return f.result.Items, f.result.Total, nil
}
func (f fakeProjectMaterialRepository) ProjectMaterialTypeCounts(_ context.Context, _ uuid.UUID) (material.ProjectMaterialTypeCounts, error) {
	return f.result.TypeCounts, nil
}
func projectMaterialRequest(handler http.Handler, method, path, projectID string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.SetPathValue("projectId", projectID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestProjectMaterialHandlerListErrorsAndContentType(t *testing.T) {
	id := uuid.New()
	fake := &fakeProjectMaterials{result: material.ProjectMaterialList{Items: []material.ProjectMaterialItem{}, Limit: 20}}
	for _, query := range []string{"?type=bad", "?sort=bad", "?limit=0", "?offset=-1", "?q=" + strings.Repeat("x", 121)} {
		w := projectMaterialRequest(listProjectMaterialsHandler(fake), http.MethodGet, "/api/v1/projects/"+id.String()+"/materials"+query, id.String())
		requireErrorEnvelope(t, w, http.StatusBadRequest)
		if !strings.Contains(w.Header().Get("Content-Type"), "application/json") || !strings.Contains(w.Body.String(), "VALIDATION_ERROR") {
			t.Fatal(w.Body.String())
		}
	}
	w := projectMaterialRequest(listProjectMaterialsHandler(fake), http.MethodGet, "/api/v1/projects/not-a-uuid/materials", "not-a-uuid")
	requireErrorEnvelope(t, w, http.StatusBadRequest)
	if !strings.Contains(w.Body.String(), "INVALID_PROJECT_ID") {
		t.Fatal(w.Body.String())
	}
	fake.err = project.ErrNotFound
	w = projectMaterialRequest(listProjectMaterialsHandler(fake), http.MethodGet, "/api/v1/projects/"+id.String()+"/materials", id.String())
	requireErrorEnvelope(t, w, http.StatusNotFound)
	if !strings.Contains(w.Body.String(), "PROJECT_NOT_FOUND") {
		t.Fatal(w.Body.String())
	}
	fake.err = errors.New("database down")
	w = projectMaterialRequest(listProjectMaterialsHandler(fake), http.MethodGet, "/api/v1/projects/"+id.String()+"/materials", id.String())
	requireErrorEnvelope(t, w, http.StatusInternalServerError)
}

func TestProjectMaterialHandlerRouteNormalAndEmptyList(t *testing.T) {
	repo := newMemoryRepository()
	id := uuid.New()
	repo.projects[id] = project.Project{ID: id}
	result := material.ProjectMaterialList{Items: []material.ProjectMaterialItem{}, Limit: 20}
	service := material.NewProjectMaterialService(project.NewService(repo), fakeProjectMaterialRepository{result: result})
	h := New(":0", project.NewService(repo), service).httpServer.Handler
	w := doRequest(h, http.MethodGet, "/api/v1/projects/"+id.String()+"/materials?q=%20&limit=20&offset=0", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "\"items\":[]") {
		t.Fatalf("response=%d %s", w.Code, w.Body.String())
	}
}

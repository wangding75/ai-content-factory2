package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type memoryRepository struct{ projects map[uuid.UUID]project.Project }

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{projects: map[uuid.UUID]project.Project{}}
}
func (r *memoryRepository) Create(_ context.Context, p project.Project, _ string) (project.Project, error) {
	r.projects[p.ID] = p
	return p, nil
}
func (r *memoryRepository) List(_ context.Context, o project.ListOptions) ([]project.Project, int, error) {
	items := []project.Project{}
	for _, p := range r.projects {
		if o.Status == "" || p.Status == o.Status {
			items = append(items, p)
		}
	}
	return items, len(items), nil
}
func (r *memoryRepository) Get(_ context.Context, id uuid.UUID) (project.Project, error) {
	p, ok := r.projects[id]
	if !ok {
		return project.Project{}, project.ErrNotFound
	}
	return p, nil
}
func (r *memoryRepository) Update(_ context.Context, id uuid.UUID, n, d *string) (project.Project, error) {
	p, ok := r.projects[id]
	if !ok {
		return project.Project{}, project.ErrNotFound
	}
	if n != nil {
		p.Name = *n
	}
	if d != nil {
		p.Description = *d
	}
	r.projects[id] = p
	return p, nil
}
func projectServer(repo *memoryRepository) http.Handler {
	return New(":0", project.NewService(repo)).httpServer.Handler
}
func doRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}
func requireErrorEnvelope(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "\"error\"") || !strings.Contains(w.Body.String(), "\"request_id\"") {
		t.Fatalf("not an error envelope: %s", w.Body.String())
	}
}
func TestProjectHandlersCreateDefaultsAndWorkspace(t *testing.T) {
	repo := newMemoryRepository()
	h := projectServer(repo)
	w := doRequest(h, http.MethodPost, "/api/v1/projects", `{"name":"Novel","type":"novel","description":"draft"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"planning"`) || !strings.Contains(w.Body.String(), `"current_stage":"project_setup"`) {
		t.Fatalf("missing defaults: %s", w.Body.String())
	}
	var id uuid.UUID
	for id = range repo.projects {
	}
	workspace := doRequest(h, http.MethodGet, "/api/v1/projects/"+id.String()+"/workspace", "")
	if workspace.Code != 200 || !strings.Contains(workspace.Body.String(), `"material_count":0`) {
		t.Fatalf("workspace = %d %s", workspace.Code, workspace.Body.String())
	}
}
func TestProjectTypeCatalogueAndCreateValidation(t *testing.T) {
	repo := newMemoryRepository()
	h := projectServer(repo)
	types := doRequest(h, http.MethodGet, "/api/v1/project-types", "")
	if types.Code != http.StatusOK || !strings.Contains(types.Body.String(), `"code":"novel"`) || !strings.Contains(types.Body.String(), `"code":"short_film"`) || !strings.Contains(types.Body.String(), `"name":"小说"`) {
		t.Fatalf("project types = %d %s", types.Code, types.Body.String())
	}
	if strings.Index(types.Body.String(), `"code":"novel"`) > strings.Index(types.Body.String(), `"code":"short_film"`) {
		t.Fatalf("project types are not sorted: %s", types.Body.String())
	}
	created := doRequest(h, http.MethodPost, "/api/v1/projects", `{"name":"Short film","type":"short_film"}`)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"type":"short_film"`) {
		t.Fatalf("create short film = %d %s", created.Code, created.Body.String())
	}
	w := doRequest(h, http.MethodPost, "/api/v1/projects", `{"name":"Bad","type":"disabled"}`)
	requireErrorEnvelope(t, w, http.StatusBadRequest)
	if !strings.Contains(w.Body.String(), `"code":"validation_error"`) {
		t.Fatalf("invalid type error = %s", w.Body.String())
	}
}
func TestProjectHandlersRejectInvalidInputs(t *testing.T) {
	repo := newMemoryRepository()
	h := projectServer(repo)
	requireErrorEnvelope(t, doRequest(h, http.MethodPost, "/api/v1/projects", `{"name":"No","type":"other"}`), 400)
	requireErrorEnvelope(t, doRequest(h, http.MethodGet, "/api/v1/projects/not-a-uuid", ""), 400)
	requireErrorEnvelope(t, doRequest(h, http.MethodPatch, "/api/v1/projects/"+uuid.New().String(), `{"status":"archived"}`), 400)
	requireErrorEnvelope(t, doRequest(h, http.MethodGet, "/api/v1/projects/"+uuid.New().String(), ""), 404)
}
func TestProjectHandlersListStatusAndPatch(t *testing.T) {
	repo := newMemoryRepository()
	h := projectServer(repo)
	p, err := project.New("One", "novel", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(context.Background(), p, "test"); err != nil {
		t.Fatal(err)
	}
	list := doRequest(h, http.MethodGet, "/api/v1/projects?status=planning&limit=1&offset=0", "")
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"total":1`) {
		t.Fatalf("list = %d %s", list.Code, list.Body.String())
	}
	patch := doRequest(h, http.MethodPatch, "/api/v1/projects/"+p.ID.String(), `{"name":"Updated","description":"changed"}`)
	if patch.Code != 200 || !strings.Contains(patch.Body.String(), "Updated") {
		t.Fatalf("patch = %d %s", patch.Code, patch.Body.String())
	}
}

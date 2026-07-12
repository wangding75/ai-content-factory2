package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/planning"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type planningHTTPProjects struct{ ids map[uuid.UUID]bool }

func (r planningHTTPProjects) Get(_ context.Context, id uuid.UUID) (project.Project, error) {
	if !r.ids[id] {
		return project.Project{}, project.ErrNotFound
	}
	return project.Project{ID: id}, nil
}

type planningHTTPRepo struct {
	values map[uuid.UUID]planning.ProjectPlanning
}

func (r *planningHTTPRepo) GetByProjectID(_ context.Context, id uuid.UUID) (planning.ProjectPlanning, error) {
	v, ok := r.values[id]
	if !ok {
		return planning.ProjectPlanning{}, planning.ErrNotFound
	}
	return v, nil
}
func (r *planningHTTPRepo) Create(_ context.Context, v planning.ProjectPlanning) (planning.ProjectPlanning, error) {
	n := time.Now().UTC()
	v.Version = 1
	v.CreatedAt = n
	v.UpdatedAt = n
	r.values[v.ProjectID] = v
	return v, nil
}
func (r *planningHTTPRepo) UpdateWithVersion(_ context.Context, v planning.ProjectPlanning, e int) (planning.ProjectPlanning, error) {
	old, ok := r.values[v.ProjectID]
	if !ok {
		return planning.ProjectPlanning{}, planning.ErrNotFound
	}
	if old.Version != e {
		return planning.ProjectPlanning{}, planning.ErrVersionConflict
	}
	v.Version = old.Version + 1
	v.CreatedAt = old.CreatedAt
	v.UpdatedAt = time.Now().UTC()
	r.values[v.ProjectID] = v
	return v, nil
}

type planningHTTPAudit struct{ entries []audit.Entry }

func (a *planningHTTPAudit) Insert(_ context.Context, e audit.Entry) error {
	a.entries = append(a.entries, e)
	return nil
}

type planningHTTPTx struct {
	repo  *planningHTTPRepo
	audit *planningHTTPAudit
}

func (x planningHTTPTx) Run(_ context.Context, f func(planning.Repository, planning.AuditWriter) error) error {
	return f(x.repo, x.audit)
}

func planningServer(id uuid.UUID) http.Handler {
	projects := planningHTTPProjects{ids: map[uuid.UUID]bool{id: true}}
	repo := &planningHTTPRepo{values: map[uuid.UUID]planning.ProjectPlanning{}}
	audits := &planningHTTPAudit{}
	service := planning.NewService(projects, repo, planningHTTPTx{repo, audits})
	return New(":0", project.NewService(newMemoryRepository()), service).httpServer.Handler
}
func planningBody(premise string, version int) string {
	return fmt.Sprintf("{\"premise\":%q,\"audience\":\"readers\",\"style\":\"plain\",\"goals_json\":{\"selling_points\":[\"hook\"],\"plot_summary\":\"summary\"},\"constraints_json\":{\"emotional_tone\":\"warm\"},\"expected_version\":%d}", premise, version)
}
func TestPlanningHandlersRouteCreateUpdateAndErrors(t *testing.T) {
	id := uuid.New()
	h := planningServer(id)
	path := "/api/v1/projects/" + id.String() + "/planning"
	get := doRequest(h, http.MethodGet, path, "")
	if get.Code != 200 || !strings.Contains(get.Body.String(), "\"version\":0") {
		t.Fatalf("empty get=%d %s", get.Code, get.Body.String())
	}
	create := doRequest(h, http.MethodPut, path, planningBody("one", 0))
	if create.Code != 200 || !strings.Contains(create.Body.String(), "\"version\":1") {
		t.Fatalf("create=%d %s", create.Code, create.Body.String())
	}
	update := doRequest(h, http.MethodPut, path, planningBody("two", 1))
	if update.Code != 200 || !strings.Contains(update.Body.String(), "\"version\":2") {
		t.Fatalf("update=%d %s", update.Code, update.Body.String())
	}
	conflict := doRequest(h, http.MethodPut, path, planningBody("three", 1))
	requireErrorEnvelope(t, conflict, http.StatusConflict)
	if !strings.Contains(conflict.Body.String(), "VERSION_CONFLICT") {
		t.Fatal(conflict.Body.String())
	}
	requireErrorEnvelope(t, doRequest(h, http.MethodGet, "/api/v1/projects/not-a-uuid/planning", ""), http.StatusBadRequest)
	requireErrorEnvelope(t, doRequest(h, http.MethodPut, path, "{\"premise\":"), http.StatusBadRequest)
	requireErrorEnvelope(t, doRequest(h, http.MethodPut, path, "{\"premise\":\"x\",\"audience\":\"a\",\"style\":\"s\",\"goals_json\":{},\"constraints_json\":{},\"expected_version\":2}"), http.StatusBadRequest)
}

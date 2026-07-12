package planning

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type serviceProjects struct{ ids map[uuid.UUID]bool }

func (r serviceProjects) Get(_ context.Context, id uuid.UUID) (project.Project, error) {
	if !r.ids[id] {
		return project.Project{}, project.ErrNotFound
	}
	return project.Project{ID: id}, nil
}

type serviceRepo struct{ values map[uuid.UUID]ProjectPlanning }

func (r *serviceRepo) GetByProjectID(_ context.Context, id uuid.UUID) (ProjectPlanning, error) {
	v, ok := r.values[id]
	if !ok {
		return ProjectPlanning{}, ErrNotFound
	}
	return v, nil
}
func (r *serviceRepo) Create(_ context.Context, v ProjectPlanning) (ProjectPlanning, error) {
	n := time.Now().UTC()
	v.Version = 1
	v.CreatedAt = n
	v.UpdatedAt = n
	r.values[v.ProjectID] = v
	return v, nil
}
func (r *serviceRepo) UpdateWithVersion(_ context.Context, v ProjectPlanning, expected int) (ProjectPlanning, error) {
	old, ok := r.values[v.ProjectID]
	if !ok {
		return ProjectPlanning{}, ErrNotFound
	}
	if old.Version != expected {
		return ProjectPlanning{}, ErrVersionConflict
	}
	v.Version = old.Version + 1
	v.CreatedAt = old.CreatedAt
	v.UpdatedAt = time.Now().UTC()
	r.values[v.ProjectID] = v
	return v, nil
}

type serviceAudit struct {
	entries []audit.Entry
	fail    bool
}

func (a *serviceAudit) Insert(_ context.Context, e audit.Entry) error {
	if a.fail {
		return errors.New("audit failed")
	}
	a.entries = append(a.entries, e)
	return nil
}

type serviceTx struct {
	repo  *serviceRepo
	audit *serviceAudit
}

func (x serviceTx) Run(ctx context.Context, f func(Repository, AuditWriter) error) error {
	snapshot := make(map[uuid.UUID]ProjectPlanning)
	for k, v := range x.repo.values {
		snapshot[k] = v
	}
	old := append([]audit.Entry(nil), x.audit.entries...)
	if err := f(x.repo, x.audit); err != nil {
		x.repo.values = snapshot
		x.audit.entries = old
		return err
	}
	return nil
}

func request(p string, v int) SaveRequest {
	a, s := "readers", "plain"
	return SaveRequest{Premise: &p, Audience: &a, Style: &s, ExpectedVersion: &v, GoalsJSON: json.RawMessage("{\"selling_points\":[\"hook\"],\"plot_summary\":\"summary\"}"), ConstraintsJSON: json.RawMessage("{\"emotional_tone\":\"warm\"}")}
}
func testPlanningService(id uuid.UUID) (*Service, *serviceRepo, *serviceAudit) {
	repo := &serviceRepo{values: map[uuid.UUID]ProjectPlanning{}}
	audit := &serviceAudit{}
	return NewService(serviceProjects{ids: map[uuid.UUID]bool{id: true}}, repo, serviceTx{repo, audit}), repo, audit
}

func TestPlanningServiceEmptyCreateUpdateRetryAndConflict(t *testing.T) {
	id := uuid.New()
	service, repo, audit := testPlanningService(id)
	empty, err := service.GetProjectPlanning(context.Background(), id)
	if err != nil || empty.Version != 0 || empty.CreatedAt != nil {
		t.Fatalf("empty=%#v err=%v", empty, err)
	}
	created, err := service.PutProjectPlanning(context.Background(), id, request("one", 0), "actor")
	if err != nil || created.Version != 1 || len(audit.entries) != 1 {
		t.Fatalf("create=%#v err=%v", created, err)
	}
	updated, err := service.PutProjectPlanning(context.Background(), id, request("two", 1), "actor")
	if err != nil || updated.Version != 2 || len(audit.entries) != 2 {
		t.Fatalf("update=%#v err=%v", updated, err)
	}
	retry, err := service.PutProjectPlanning(context.Background(), id, request("two", 2), "actor")
	if err != nil || retry.Version != 2 || len(audit.entries) != 2 {
		t.Fatalf("retry=%#v err=%v", retry, err)
	}
	if _, err = service.PutProjectPlanning(context.Background(), id, request("three", 1), "actor"); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict=%v", err)
	}
	if repo.values[id].Premise != "two" {
		t.Fatal("conflict changed data")
	}
}

func TestPlanningServiceValidationProjectAndAuditRollback(t *testing.T) {
	id := uuid.New()
	service, repo, audit := testPlanningService(id)
	bad := request("x", 0)
	bad.GoalsJSON = json.RawMessage("{\"selling_points\":[],\"plot_summary\":\"\",\"extra\":true}")
	if _, err := service.PutProjectPlanning(context.Background(), id, bad, "actor"); !errors.Is(err, ErrValidation) {
		t.Fatalf("validation=%v", err)
	}
	if _, err := service.GetProjectPlanning(context.Background(), uuid.New()); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("project=%v", err)
	}
	audit.fail = true
	if _, err := service.PutProjectPlanning(context.Background(), id, request("one", 0), "actor"); err == nil {
		t.Fatal("expected audit failure")
	}
	if len(repo.values) != 0 || len(audit.entries) != 0 {
		t.Fatal("audit failure was not rolled back")
	}
}

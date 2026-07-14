package foreshadowing

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/testpostgres"
)

func TestPostgresRepositoryIntegration(t *testing.T) {
	pool, ctx := testpostgres.Open(t)
	projectA, projectB := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{projectA, projectB} {
		if _, err := pool.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','i04-foreshadowing-test')", id, id.String()); err != nil {
			t.Fatalf("insert project: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM projects WHERE id IN($1,$2)", projectA, projectB)
	})
	one, two := uuid.New(), uuid.New()
	for _, item := range []struct {
		id   uuid.UUID
		name string
	}{{one, "one"}, {two, "two"}} {
		if _, err := pool.Exec(ctx, "INSERT INTO storylines(id,project_id,type,relation,name,status,sort_order,created_by) VALUES($1,$2,'main','root',$3,'active',0,'i04-foreshadowing-test')", item.id, projectA, item.name); err != nil {
			t.Fatalf("insert storyline: %v", err)
		}
	}
	other := uuid.New()
	if _, err := pool.Exec(ctx, "INSERT INTO storylines(id,project_id,type,relation,name,status,sort_order,created_by) VALUES($1,$2,'main','root','other','active',0,'i04-foreshadowing-test')", other, projectB); err != nil {
		t.Fatalf("insert other storyline: %v", err)
	}

	repository := NewPostgresRepository(pool)
	created, err := repository.Create(ctx, Foreshadowing{ID: uuid.New(), ProjectID: projectA, Title: "cross-line", Priority: "low", Status: "planned", PlantedPlotLineID: &one, PayoffPlotLineID: &two, CreatedBy: "i04-foreshadowing-test"})
	if err != nil || created.Version != 1 {
		t.Fatalf("create cross-line reference: value=%#v err=%v", created, err)
	}
	if _, err = repository.Create(ctx, Foreshadowing{ID: uuid.New(), ProjectID: projectA, Title: "missing", Priority: "low", Status: "planned", PlantedPlotLineID: uuidPtr(uuid.New()), CreatedBy: "i04-foreshadowing-test"}); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("missing reference error=%v", err)
	}
	if _, err = repository.Create(ctx, Foreshadowing{ID: uuid.New(), ProjectID: projectA, Title: "cross-project", Priority: "low", Status: "planned", PlantedPlotLineID: &other, CreatedBy: "i04-foreshadowing-test"}); !errors.Is(err, ErrProjectMismatch) {
		t.Fatalf("cross-project reference error=%v", err)
	}
	created.Status = "planted"
	updated, err := repository.UpdateWithVersion(ctx, created, created.Version)
	if err != nil || updated.Version != 2 {
		t.Fatalf("status update: value=%#v err=%v", updated, err)
	}
	unchanged, err := repository.UpdateWithVersion(ctx, updated, updated.Version)
	if err != nil || unchanged.Version != updated.Version {
		t.Fatalf("idempotent update: value=%#v err=%v", unchanged, err)
	}
	updated.Status = "paid_off"
	updated, err = repository.UpdateWithVersion(ctx, updated, updated.Version)
	if err != nil || updated.Version != 3 {
		t.Fatalf("second status update: value=%#v err=%v", updated, err)
	}
	updated.Status = "planned"
	if _, err = repository.UpdateWithVersion(ctx, updated, updated.Version); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("invalid transition error=%v", err)
	}
	if items, err := repository.ListByProject(ctx, projectB); err != nil || len(items) != 0 {
		t.Fatalf("project isolation: items=%#v err=%v", items, err)
	}
}

func uuidPtr(value uuid.UUID) *uuid.UUID { return &value }

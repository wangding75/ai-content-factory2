package storyline

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"github.com/local/ai-content-factory/apps/api/internal/testpostgres"
)

func TestPostgresRepositoryIntegration(t *testing.T) {
	pool, ctx := testpostgres.Open(t)
	projectA, projectB := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{projectA, projectB} {
		if _, err := pool.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','i04-storyline-test')", id, id.String()); err != nil {
			t.Fatalf("insert project: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM projects WHERE id IN($1,$2)", projectA, projectB)
	})

	repository := NewPostgresRepository(pool)
	root, err := repository.Create(ctx, PlotLine{ID: uuid.New(), ProjectID: projectA, Type: "main", Relation: "root", Name: "root", Status: "active", CreatedBy: "i04-storyline-test"})
	if err != nil || root.Version != 1 {
		t.Fatalf("create root: value=%#v err=%v", root, err)
	}
	child, err := repository.Create(ctx, PlotLine{ID: uuid.New(), ProjectID: projectA, ParentID: &root.ID, Type: "child", Relation: "child", Name: "child", Status: "active", SortOrder: 2, CreatedBy: "i04-storyline-test"})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	if _, err = repository.Create(ctx, PlotLine{ID: uuid.New(), ProjectID: projectA, ParentID: uuidPtr(uuid.New()), Type: "child", Relation: "child", Name: "missing", Status: "active", CreatedBy: "i04-storyline-test"}); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("missing parent error=%v", err)
	}
	otherRoot, err := repository.Create(ctx, PlotLine{ID: uuid.New(), ProjectID: projectB, Type: "main", Relation: "root", Name: "other", Status: "active", CreatedBy: "i04-storyline-test"})
	if err != nil {
		t.Fatalf("create other root: %v", err)
	}
	if _, err = repository.Create(ctx, PlotLine{ID: uuid.New(), ProjectID: projectA, ParentID: &otherRoot.ID, Type: "child", Relation: "child", Name: "cross", Status: "active", CreatedBy: "i04-storyline-test"}); !errors.Is(err, ErrProjectMismatch) {
		t.Fatalf("cross-project parent error=%v", err)
	}

	items, err := repository.ListByProject(ctx, projectA)
	if err != nil || len(items) != 2 || items[0].ID != root.ID || items[1].ID != child.ID {
		t.Fatalf("stable list: items=%#v err=%v", items, err)
	}
	root.Name = "changed"
	updated, err := repository.UpdateWithVersion(ctx, root, root.Version)
	if err != nil || updated.Version != 2 {
		t.Fatalf("update: value=%#v err=%v", updated, err)
	}
	unchanged, err := repository.UpdateWithVersion(ctx, updated, updated.Version)
	if err != nil || unchanged.Version != updated.Version {
		t.Fatalf("idempotent update: value=%#v err=%v", unchanged, err)
	}
	if _, err = repository.UpdateWithVersion(ctx, updated, 1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("version conflict error=%v", err)
	}
	if err = material.WithTx(ctx, pool, func(tx pgx.Tx) error {
		_, createErr := NewPostgresRepositoryTx(tx).Create(ctx, PlotLine{ID: uuid.New(), ProjectID: projectA, Type: "main", Relation: "root", Name: "rollback", Status: "active", CreatedBy: "i04-storyline-test"})
		if createErr != nil {
			return createErr
		}
		return pgx.ErrTxClosed
	}); err == nil {
		t.Fatal("expected rollback error")
	}
}

func uuidPtr(value uuid.UUID) *uuid.UUID { return &value }

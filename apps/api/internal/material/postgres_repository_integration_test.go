package material

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
	"github.com/local/ai-content-factory/apps/api/internal/planning"
)

func TestPlanningMaterialsRepositoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	defer pool.Close()
	projectA, projectB := uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{projectA, projectB} {
		if _, err := pool.Exec(ctx, "INSERT INTO projects (id,name,type,created_by) VALUES ($1,$2,'novel','integration-test')", id, "planning materials integration"); err != nil {
			t.Fatalf("insert project: %v", err)
		}
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM project_plannings WHERE project_id IN ($1,$2)", projectA, projectB)
		_, _ = pool.Exec(context.Background(), "DELETE FROM project_material_usages WHERE project_id IN ($1,$2)", projectA, projectB)
		_, _ = pool.Exec(context.Background(), "DELETE FROM materials WHERE created_by='planning-materials-integration'")
		_, _ = pool.Exec(context.Background(), "DELETE FROM idempotency_records WHERE scope='createMaterial' AND idempotency_key='integration-key'")
		_, _ = pool.Exec(context.Background(), "DELETE FROM projects WHERE id IN ($1,$2)", projectA, projectB)
	}()
	assertPlanningMaterialsSchema(t, ctx, pool)

	planningRepo := planning.NewPostgresRepository(pool)
	planningValue := planning.ProjectPlanning{ProjectID: projectA, Premise: "p", Audience: "a", Style: "s", GoalsJSON: json.RawMessage(`{"selling_points":["x"],"plot_summary":"y"}`), ConstraintsJSON: json.RawMessage(`{"emotional_tone":"z"}`), CreatedBy: "planning-materials-integration"}
	createdPlanning, err := planningRepo.Create(ctx, planningValue)
	if err != nil {
		t.Fatalf("create planning: %v", err)
	}
	createdPlanning.Premise = "updated"
	updatedPlanning, err := planningRepo.UpdateWithVersion(ctx, createdPlanning, createdPlanning.Version)
	if err != nil || updatedPlanning.Version != 2 {
		t.Fatalf("update planning: %#v %v", updatedPlanning, err)
	}
	if _, err := planningRepo.UpdateWithVersion(ctx, updatedPlanning, 1); err != planning.ErrVersionConflict {
		t.Fatalf("planning conflict: %v", err)
	}

	idempotencyRepo := idempotency.NewPostgresRepository(pool)
	idempotencyValue := idempotency.Record{ID: uuid.New(), Scope: "createMaterial", Key: "integration-key", RequestHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ResponseStatus: 201, ResponseBody: json.RawMessage(`{}`)}
	if _, err := idempotencyRepo.Create(ctx, idempotencyValue); err != nil {
		t.Fatalf("create idempotency record: %v", err)
	}
	if _, err := idempotencyRepo.Get(ctx, idempotencyValue.Scope, idempotencyValue.Key); err != nil {
		t.Fatalf("get idempotency record: %v", err)
	}
	idempotencyValue.ID = uuid.New()
	if _, err := idempotencyRepo.Create(ctx, idempotencyValue); err != idempotency.ErrConflict {
		t.Fatalf("duplicate idempotency key: %v", err)
	}
	repo := NewPostgresRepository(pool)
	materialA := Material{ID: uuid.New(), Type: TypeCharacter, Name: "shared character", Summary: "summary", ContentJSON: json.RawMessage(`{"age":"30"}`), Tags: []string{"hero", "shared"}, CreatedBy: "planning-materials-integration"}
	createdMaterial, err := repo.Create(ctx, materialA)
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	items, total, err := repo.List(ctx, ListOptions{Query: "shared", Type: TypeCharacter, Sort: "name_asc", Limit: 20, Offset: 0})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("list material: total=%d items=%d err=%v", total, len(items), err)
	}
	createdMaterial.Name = "shared character updated"
	updatedMaterial, err := repo.UpdateWithVersion(ctx, createdMaterial, createdMaterial.Version)
	if err != nil || updatedMaterial.Version != 2 {
		t.Fatalf("update material: %#v %v", updatedMaterial, err)
	}
	if _, err := repo.UpdateWithVersion(ctx, updatedMaterial, 1); err != ErrVersionConflict {
		t.Fatalf("material conflict: %v", err)
	}
	usageA := ProjectMaterialUsage{ID: uuid.New(), ProjectID: projectA, MaterialID: updatedMaterial.ID, UsageType: "浜虹墿瑙掕壊", RoleName: "涓昏", Notes: "", Status: StatusActive, CreatedBy: "planning-materials-integration"}
	if _, err := repo.CreateUsage(ctx, usageA); err != nil {
		t.Fatalf("create usage A: %v", err)
	}
	usageB := usageA
	usageB.ID = uuid.New()
	usageB.ProjectID = projectB
	if _, err := repo.CreateUsage(ctx, usageB); err != nil {
		t.Fatalf("create usage B: %v", err)
	}
	usageA.ID = uuid.New()
	if _, err := repo.CreateUsage(ctx, usageA); err != ErrAlreadyBound {
		t.Fatalf("duplicate usage: %v", err)
	}
	if count, err := repo.CountByMaterial(ctx, updatedMaterial.ID); err != nil || count != 2 {
		t.Fatalf("reference count=%d err=%v", count, err)
	}
	usageForA, err := repo.GetByProjectAndMaterial(ctx, projectA, updatedMaterial.ID)
	if err != nil {
		t.Fatal(err)
	}
	usageForA.Notes = "only project A"
	updatedUsage, err := repo.UpdateUsageWithVersion(ctx, usageForA, usageForA.Version)
	if err != nil || updatedUsage.Version != 2 {
		t.Fatalf("update usage: %#v %v", updatedUsage, err)
	}
	other, err := repo.GetByProjectAndMaterial(ctx, projectB, updatedMaterial.ID)
	if err != nil || other.Notes != "" {
		t.Fatalf("other project usage changed: %#v %v", other, err)
	}
	if err := repo.DeleteUsageWithVersion(ctx, projectA, updatedMaterial.ID, updatedUsage.Version); err != nil {
		t.Fatalf("delete usage: %v", err)
	}
	if _, err := repo.GetByID(ctx, updatedMaterial.ID); err != nil {
		t.Fatalf("material removed with usage: %v", err)
	}
	if count, err := repo.CountByMaterial(ctx, updatedMaterial.ID); err != nil || count != 1 {
		t.Fatalf("remaining references=%d err=%v", count, err)
	}

	rollbackMaterial := Material{ID: uuid.New(), Type: TypeItem, Name: "rollback material", ContentJSON: json.RawMessage(`{}`), Tags: []string{}, CreatedBy: "planning-materials-integration"}
	err = WithTx(ctx, pool, func(tx pgx.Tx) error {
		txRepo := NewPostgresRepositoryTx(tx)
		if _, err := txRepo.Create(ctx, rollbackMaterial); err != nil {
			return err
		}
		_, err := txRepo.CreateUsage(ctx, ProjectMaterialUsage{ID: uuid.New(), ProjectID: uuid.New(), MaterialID: rollbackMaterial.ID, UsageType: "閬撳叿", Status: StatusActive, CreatedBy: "planning-materials-integration"})
		return err
	})
	if err == nil {
		t.Fatal("expected usage foreign key failure")
	}
	if _, err := repo.GetByID(ctx, rollbackMaterial.ID); err != ErrNotFound {
		t.Fatalf("material survived failed material+usage transaction: %v", err)
	}
	auditRollback := Material{ID: uuid.New(), Type: TypeItem, Name: "audit rollback material", ContentJSON: json.RawMessage(`{}`), Tags: []string{}, CreatedBy: "planning-materials-integration"}
	err = WithTx(ctx, pool, func(tx pgx.Tx) error {
		txRepo := NewPostgresRepositoryTx(tx)
		if _, err := txRepo.Create(ctx, auditRollback); err != nil {
			return err
		}
		return audit.NewRepository(tx).Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: "integration-test", Action: "material.created", SubjectType: "material", SubjectID: auditRollback.ID.String(), Payload: json.RawMessage(`{`)})
	})
	if err == nil {
		t.Fatal("expected audit failure")
	}
	if _, err := repo.GetByID(ctx, auditRollback.ID); err != ErrNotFound {
		t.Fatalf("material survived failed audit transaction: %v", err)
	}
}

func assertPlanningMaterialsSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, table := range []string{"project_plannings", "materials", "project_material_usages", "idempotency_records"} {
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass('public.' || $1) IS NOT NULL", table).Scan(&exists); err != nil || !exists {
			t.Fatalf("table %s exists=%v err=%v", table, exists, err)
		}
	}
	var uniqueCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM pg_constraint WHERE conname='project_material_usages_project_material_unique'").Scan(&uniqueCount); err != nil || uniqueCount != 1 {
		t.Fatalf("usage unique constraint=%d err=%v", uniqueCount, err)
	}
}

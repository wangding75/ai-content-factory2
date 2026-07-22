package workflowbinding

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationDatabase = "ai_content_factory_http_test"

func openIntegrationDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration test skipped")
	}
	cfg, err := pgxpool.ParseConfig(u)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	if cfg.ConnConfig.Database != integrationDatabase {
		t.Skipf("TEST_DATABASE_URL targets database %q, not %q; PostgreSQL integration test skipped", cfg.ConnConfig.Database, integrationDatabase)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)
	if err = db.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	return db, ctx
}

// insertProject inserts a minimal project row needed for FK constraints.
func insertProject(t *testing.T, ctx context.Context, db queryer, id uuid.UUID) {
	t.Helper()
	now := time.Now().UTC()
	_, err := db.Exec(ctx, `INSERT INTO projects (id, name, type, description, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'novel', '', 'planning', 'system', $3, $4) ON CONFLICT DO NOTHING`,
		id, "test-project-"+id.String()[:8], now, now)
	if err != nil {
		t.Fatalf("insert project fixture: %v", err)
	}
}

// insertWorkflowConfig inserts a minimal workflow_configuration row needed for FK constraints.
func insertWorkflowConfig(t *testing.T, ctx context.Context, db queryer, wfID, connID uuid.UUID, stages []string) {
	t.Helper()
	now := time.Now().UTC()
	// Insert a minimal connection first.
	_, err := db.Exec(ctx, `INSERT INTO workflow_connections (id, name, connection_type, base_url, auth_type, timeout_seconds, type_config, created_at, updated_at)
		VALUES ($1, $2, 'n8n', 'http://localhost', 'api_key', 30, '{}', $3, $4) ON CONFLICT DO NOTHING`,
		connID, "test-conn-"+connID.String()[:8], now, now)
	if err != nil {
		t.Fatalf("insert connection fixture: %v", err)
	}
	stagesJSON := "[]"
	if len(stages) > 0 {
		stagesJSON = "["
		for i, s := range stages {
			if i > 0 {
				stagesJSON += ","
			}
			stagesJSON += `"` + s + `"`
		}
		stagesJSON += "]"
	}
	_, err = db.Exec(ctx, `INSERT INTO workflow_configurations (id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version, default_parameters, created_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, '{}', 'v1', 'v1', '{}', $5, $6) ON CONFLICT DO NOTHING`,
		wfID, "test-wf-"+wfID.String()[:8], connID, stagesJSON, now, now)
	if err != nil {
		t.Fatalf("insert workflow config fixture: %v", err)
	}
}

// cleanupBinding removes test data inserted during a test.
func cleanupBinding(t *testing.T, ctx context.Context, db queryer, id uuid.UUID) {
	t.Helper()
	_, _ = db.Exec(ctx, "DELETE FROM project_workflow_bindings WHERE id=$1", id)
}

// ── Repository Integration Tests ──────────────────────────────────────────

func TestRepositoryCreateAndListByProject(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	b, err := New(uuid.New(), projectID, wfID, StageChapterPlanning)
	if err != nil {
		t.Fatal(err)
	}
	created, err := repo.Create(ctx, b)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, created.ID) })
	if created.Version != 1 {
		t.Fatalf("Create() version = %d, want 1", created.Version)
	}
	if created.Stage != StageChapterPlanning {
		t.Fatalf("Create() stage = %s, want chapter_planning", created.Stage)
	}

	bindings, err := repo.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 1 {
		t.Fatalf("ListByProject() len = %d, want 1", len(bindings))
	}
	if bindings[0].ID != created.ID {
		t.Fatalf("ListByProject()[0].ID = %s, want %s", bindings[0].ID, created.ID)
	}
}

func TestRepositoryCreateDuplicateProjectStage(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID1 := uuid.New()
	wfID2 := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID1, connID, []string{"chapter_planning"})
	insertWorkflowConfig(t, ctx, pool, wfID2, connID, []string{"chapter_planning"})

	b1, _ := New(uuid.New(), projectID, wfID1, StageChapterPlanning)
	created, err := repo.Create(ctx, b1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, created.ID) })

	b2, _ := New(uuid.New(), projectID, wfID2, StageChapterPlanning)
	_, err = repo.Create(ctx, b2)
	if !errors.Is(err, ErrBindingAlreadyExists) {
		t.Fatalf("Create() duplicate error = %v, want ErrBindingAlreadyExists", err)
	}
}

func TestRepositoryCreateDifferentStagesSameProject(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID1 := uuid.New()
	wfID2 := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID1, connID, []string{"chapter_planning"})
	insertWorkflowConfig(t, ctx, pool, wfID2, connID, []string{"review"})

	b1, _ := New(uuid.New(), projectID, wfID1, StageChapterPlanning)
	created1, err := repo.Create(ctx, b1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, created1.ID) })

	b2, _ := New(uuid.New(), projectID, wfID2, StageReview)
	created2, err := repo.Create(ctx, b2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, created2.ID) })

	bindings, err := repo.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 2 {
		t.Fatalf("ListByProject() len = %d, want 2", len(bindings))
	}
}

func TestRepositoryGetByProjectAndStage(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"review"})

	b, _ := New(uuid.New(), projectID, wfID, StageReview)
	created, err := repo.Create(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, created.ID) })

	found, err := repo.GetByProjectAndStage(ctx, projectID, StageReview)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != created.ID {
		t.Fatalf("GetByProjectAndStage() ID = %s, want %s", found.ID, created.ID)
	}

	_, err = repo.GetByProjectAndStage(ctx, projectID, StageChapterPlanning)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByProjectAndStage() missing error = %v, want ErrNotFound", err)
	}
}

func TestRepositoryReplaceAtomically(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID1 := uuid.New()
	wfID2 := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID1, connID, []string{"review"})
	insertWorkflowConfig(t, ctx, pool, wfID2, connID, []string{"review"})

	b, _ := New(uuid.New(), projectID, wfID1, StageReview)
	created, err := repo.Create(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, created.ID) })

	// Replace with a new workflow ID, passing the current version.
	next := created
	next.WorkflowConfigurationID = wfID2
	replaced, err := repo.Replace(ctx, next.ProjectID, next.Stage, next.Version, wfID2)
	if err != nil {
		t.Fatalf("Replace() error = %v", err)
	}
	if replaced.Version != 2 {
		t.Fatalf("Replace() version = %d, want 2", replaced.Version)
	}
	if replaced.WorkflowConfigurationID != wfID2 {
		t.Fatalf("Replace() workflowConfigurationID = %s, want %s", replaced.WorkflowConfigurationID, wfID2)
	}

	// Replace with stale version should fail.
	_, err = repo.Replace(ctx, next.ProjectID, next.Stage, next.Version, wfID2)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Replace() stale version error = %v, want ErrVersionConflict", err)
	}
}

func TestRepositoryDeleteWithVersion(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"rewrite"})

	b, _ := New(uuid.New(), projectID, wfID, StageRewrite)
	created, err := repo.Create(ctx, b)
	if err != nil {
		t.Fatal(err)
	}

	removed, err := repo.Delete(ctx, projectID, StageRewrite, 1)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if removed.ID != created.ID {
		t.Fatalf("Delete() removed ID = %s, want %s", removed.ID, created.ID)
	}

	// Should not exist after delete.
	_, err = repo.GetByProjectAndStage(ctx, projectID, StageRewrite)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByProjectAndStage() after delete error = %v, want ErrNotFound", err)
	}
}

func TestRepositoryDeleteStaleVersion(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"rewrite"})

	b, _ := New(uuid.New(), projectID, wfID, StageRewrite)
	created, err := repo.Create(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, created.ID) })

	_, err = repo.Delete(ctx, projectID, StageRewrite, 2)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Delete() stale version error = %v, want ErrVersionConflict", err)
	}
}

func TestRepositoryDeleteNonexistent(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	_, err := repo.Delete(ctx, projectID, StageChapterPlanning, 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() nonexistent error = %v, want ErrNotFound", err)
	}
}

func TestRepositoryListByProjectEmpty(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	bindings, err := repo.ListByProject(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 0 {
		t.Fatalf("ListByProject() empty len = %d, want 0", len(bindings))
	}
}

func TestRepositoryCrossProjectIsolation(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	p1 := uuid.New()
	p2 := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, p1)
	insertProject(t, ctx, pool, p2)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	b1, _ := New(uuid.New(), p1, wfID, StageChapterPlanning)
	c1, err := repo.Create(ctx, b1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, c1.ID) })

	// Project 2 should not see Project 1's bindings.
	bindings, err := repo.ListByProject(ctx, p2)
	if err != nil {
		t.Fatal(err)
	}
	if len(bindings) != 0 {
		t.Fatalf("ListByProject() cross-project len = %d, want 0", len(bindings))
	}
}

// ── Compile-time check ──
var _ = pgx.ErrNoRows
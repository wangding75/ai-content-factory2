package workflowbinding

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
)

// ── Mock implementations ──────────────────────────────────────────────────

type mockProjectRepo struct {
	existsFn func(ctx context.Context, id uuid.UUID) error
}

func (m mockProjectRepo) ExistsForModify(ctx context.Context, id uuid.UUID) error {
	if m.existsFn != nil {
		return m.existsFn(ctx, id)
	}
	return nil
}

type mockWorkflowRepo struct {
	getFn func(ctx context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error)
}

func (m mockWorkflowRepo) GetWorkflow(ctx context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
	if m.getFn != nil {
		return m.getFn(ctx, id)
	}
	return ReadWorkflowConfiguration{}, ErrConfigurationNotFound
}

func newDisabledWorkflow(id uuid.UUID, stages []string) ReadWorkflowConfiguration {
	return ReadWorkflowConfiguration{
		ID:                id,
		Name:              "test-disabled",
		ConnectionID:      uuid.New(),
		ConnectionName:    "test-conn",
		ConnectionType:    "n8n",
		WorkflowType:      "n8n",
		ApplicableStages:  stages,
		Enabled:           false,
		IntegrationStatus: "not_connected",
		Version:           1,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
}

func newEnabledWorkflow(id uuid.UUID, stages []string) ReadWorkflowConfiguration {
	wf := newDisabledWorkflow(id, stages)
	wf.Enabled = true
	return wf
}

func newDisabledWorkflowWithStatus(id uuid.UUID, stages []string, integrationStatus string) ReadWorkflowConfiguration {
	wf := newDisabledWorkflow(id, stages)
	wf.IntegrationStatus = integrationStatus
	return wf
}

func newEnabledWorkflowWithStatus(id uuid.UUID, stages []string, integrationStatus string) ReadWorkflowConfiguration {
	wf := newEnabledWorkflow(id, stages)
	wf.IntegrationStatus = integrationStatus
	return wf
}

func countAuditForSubject(t *testing.T, ctx context.Context, db queryer, subjectID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1", subjectID).Scan(&n); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	return n
}

// idempotencyRecordFor reads the cached idempotency record via the shared
// internal/idempotency repository so tests assert against the real store.
func idempotencyRecordFor(t *testing.T, ctx context.Context, db queryer, scope, key string) idempotency.Record {
	t.Helper()
	row := db.QueryRow(ctx, "SELECT id,scope,idempotency_key,request_hash,response_status,response_body,created_at,expires_at FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2", scope, key)
	var rec idempotency.Record
	if err := row.Scan(&rec.ID, &rec.Scope, &rec.Key, &rec.RequestHash, &rec.ResponseStatus, &rec.ResponseBody, &rec.CreatedAt, &rec.ExpiresAt); err != nil {
		t.Fatalf("idempotency record missing: %v", err)
	}
	return rec
}

func countIdempotencyRecords(t *testing.T, ctx context.Context, db queryer, scope, key string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2", scope, key).Scan(&n); err != nil {
		t.Fatalf("count idempotency: %v", err)
	}
	return n
}

func countBindingsForProject(t *testing.T, ctx context.Context, db queryer, projectID uuid.UUID) int {
	t.Helper()
	var n int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM project_workflow_bindings WHERE project_id=$1", projectID).Scan(&n); err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	return n
}

// cleanupProjectBindings removes all bindings for a project so concurrent
// tests do not leak rows across runs.
func cleanupProjectBindings(t *testing.T, ctx context.Context, db queryer, projectID uuid.UUID) {
	t.Helper()
	_, _ = db.Exec(ctx, "DELETE FROM project_workflow_bindings WHERE project_id=$1", projectID)
}

func cleanupIdempotency(t *testing.T, ctx context.Context, db queryer, scope, key string) {
	t.Helper()
	_, _ = db.Exec(ctx, "DELETE FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2", scope, key)
}

func cleanupAuditForSubject(ctx context.Context, db queryer, subjectID string) {
	_, _ = db.Exec(ctx, "DELETE FROM audit_logs WHERE subject_id=$1", subjectID)
}

// ── Service Unit Tests ────────────────────────────────────────────────────

func TestServiceListStagesFourFixedOrder(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	projectID := uuid.New()
	insertProject(t, ctx, pool, projectID)

	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{}, "system")
	items, err := svc.ListStages(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("ListStages() len = %d, want 4", len(items))
	}
	expected := []WorkflowBindingStage{StageChapterPlanning, StageContentGeneration, StageReview, StageRewrite}
	for i, want := range expected {
		if items[i].Stage != want {
			t.Fatalf("ListStages()[%d].Stage = %s, want %s", i, items[i].Stage, want)
		}
		if items[i].Bound {
			t.Fatalf("ListStages()[%d].Bound = true, want false", i)
		}
	}
}

func TestServiceListStagesWithBinding(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	b, _ := New(uuid.New(), projectID, wfID, StageChapterPlanning)
	created, err := repo.Create(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, created.ID) })

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		if id == wfID {
			return wf, nil
		}
		return ReadWorkflowConfiguration{}, ErrConfigurationNotFound
	}}, "system")

	items, err := svc.ListStages(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if !items[0].Bound {
		t.Fatal("ListStages()[0].Bound = false, want true")
	}
	if items[0].Binding == nil {
		t.Fatal("ListStages()[0].Binding is nil")
	}
	if items[0].WorkflowConfigurationSummary == nil {
		t.Fatal("ListStages()[0].WorkflowConfigurationSummary is nil")
	}
}

func TestServicePutFirstBind(t *testing.T) {
	pool, ctx := openIntegrationDB(t)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	result, err := svc.Put(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	t.Cleanup(func() {
		cleanupBinding(t, context.Background(), pool, result.Binding.ID)
		cleanupAuditForSubject(context.Background(), pool, result.Binding.ID.String())
	})
	if !result.Created {
		t.Fatal("Put() Created = false, want true")
	}
	if result.Binding.Version != 1 {
		t.Fatalf("Put() Version = %d, want 1", result.Binding.Version)
	}
	if result.Binding.Stage != StageChapterPlanning {
		t.Fatalf("Put() Stage = %s, want chapter_planning", result.Binding.Stage)
	}

	// Verify audit.
	n := countAuditForSubject(t, ctx, pool, result.Binding.ID.String())
	if n != 1 {
		t.Fatalf("audit count = %d, want 1", n)
	}
}

func TestServicePutReplace(t *testing.T) {
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
	t.Cleanup(func() {
		cleanupBinding(t, context.Background(), pool, created.ID)
		cleanupAuditForSubject(context.Background(), pool, created.ID.String())
	})

	wf2 := newEnabledWorkflow(wfID2, []string{"review"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		if id == wfID2 {
			return wf2, nil
		}
		return ReadWorkflowConfiguration{}, ErrConfigurationNotFound
	}}, "system")

	result, err := svc.Put(ctx, projectID, StageReview, PutRequest{WorkflowConfigurationID: wfID2, ExpectedVersion: intPtr(1)})
	if err != nil {
		t.Fatalf("Put() replace error = %v", err)
	}
	if result.NoChange {
		t.Fatal("Put() replace NoChange = true, want false")
	}
	if result.Binding.Version != 2 {
		t.Fatalf("Put() replace Version = %d, want 2", result.Binding.Version)
	}
	if result.Binding.WorkflowConfigurationID != wfID2 {
		t.Fatalf("Put() replace WorkflowConfigurationID = %s, want %s", result.Binding.WorkflowConfigurationID, wfID2)
	}

	// Verify audit count.
	n := countAuditForSubject(t, ctx, pool, result.Binding.ID.String())
	if n != 1 {
		t.Fatalf("audit count = %d, want 1 (replace audit)", n)
	}
}

func TestServicePutSameWorkflowNoOp(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	b, _ := New(uuid.New(), projectID, wfID, StageChapterPlanning)
	created, err := repo.Create(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupBinding(t, context.Background(), pool, created.ID)
		cleanupAuditForSubject(context.Background(), pool, created.ID.String())
	})

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	result, err := svc.Put(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID, ExpectedVersion: intPtr(1)})
	if err != nil {
		t.Fatalf("Put() no-op error = %v", err)
	}
	if !result.NoChange {
		t.Fatal("Put() same workflow NoChange = false, want true")
	}

	// No audit should be written.
	n := countAuditForSubject(t, ctx, pool, created.ID.String())
	if n != 0 {
		t.Fatalf("audit count = %d, want 0 (no audit for no-op)", n)
	}
}

func TestServiceDelete(t *testing.T) {
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
	t.Cleanup(func() { cleanupAuditForSubject(context.Background(), pool, created.ID.String()) })

	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{}, "system")
	result, err := svc.Delete(ctx, projectID, StageRewrite, DeleteRequest{ExpectedVersion: 1})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !result.Unbound {
		t.Fatal("Delete() Unbound = false, want true")
	}

	// Audit should be written with the binding ID.
	n := countAuditForSubject(t, ctx, pool, created.ID.String())
	if n != 1 {
		t.Fatalf("audit count = %d, want 1 (remove audit)", n)
	}

	// Binding should not exist.
	_, err = repo.GetByProjectAndStage(ctx, projectID, StageRewrite)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByProjectAndStage() after delete error = %v, want ErrNotFound", err)
	}
}

func TestServiceDeleteNotFound(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	projectID := uuid.New()

	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{}, "system")
	_, err := svc.Delete(ctx, projectID, StageChapterPlanning, DeleteRequest{ExpectedVersion: 1})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() nonexistent error = %v, want ErrNotFound", err)
	}
}

func TestServicePutDisabledWorkflow(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	projectID := uuid.New()
	wfID := uuid.New()

	wf := newDisabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	_, err := svc.Put(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID})
	if !errors.Is(err, ErrDisabledWorkflow) {
		t.Fatalf("Put() disabled error = %v, want ErrDisabledWorkflow", err)
	}
}

func TestServicePutStageNotApplicable(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	projectID := uuid.New()
	wfID := uuid.New()

	wf := newEnabledWorkflow(wfID, []string{"review"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	_, err := svc.Put(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID})
	if !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("Put() not applicable error = %v, want ErrNotApplicable", err)
	}
}

func TestServicePutWorkflowNotFound(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	projectID := uuid.New()
	wfID := uuid.New()

	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{}, "system")
	_, err := svc.Put(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID})
	if !errors.Is(err, ErrConfigurationNotFound) {
		t.Fatalf("Put() missing workflow error = %v, want ErrConfigurationNotFound", err)
	}
}

func TestServicePutProjectNotFound(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	projectID := uuid.New()
	wfID := uuid.New()

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{existsFn: func(_ context.Context, id uuid.UUID) error {
		return ErrProjectNotFound
	}}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	_, err := svc.Put(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID})
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("Put() missing project error = %v, want ErrProjectNotFound", err)
	}
}

func TestServicePutVersionConflict(t *testing.T) {
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
	t.Cleanup(func() {
		cleanupBinding(t, context.Background(), pool, created.ID)
		cleanupAuditForSubject(context.Background(), pool, created.ID.String())
	})

	wf2 := newEnabledWorkflow(wfID2, []string{"review"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf2, nil
	}}, "system")

	// Send wrong expectedVersion
	_, err = svc.Put(ctx, projectID, StageReview, PutRequest{WorkflowConfigurationID: wfID2, ExpectedVersion: intPtr(99)})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Put() version conflict error = %v, want ErrVersionConflict", err)
	}
	// The conflict error must carry expected/current/projectId/stage context.
	var conflict *VersionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Put() version conflict is not a VersionConflictError")
	}
	if conflict.ExpectedVersion != 99 || conflict.CurrentVersion != 1 || conflict.Stage != StageReview || conflict.ProjectID != projectID {
		t.Fatalf("conflict details = %+v", conflict)
	}

	// Send missing expectedVersion on existing binding
	_, err = svc.Put(ctx, projectID, StageReview, PutRequest{WorkflowConfigurationID: wfID2})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Put() missing expectedVersion error = %v, want ErrVersionConflict", err)
	}
}

func TestServicePutExpectedVersionOnFirstBind(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	_, err := svc.Put(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID, ExpectedVersion: intPtr(1)})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Put() expectedVersion on first bind error = %v, want ErrValidation", err)
	}
}

func TestServicePutIntegrationErrorDoesNotBlockBinding(t *testing.T) {
	pool, ctx := openIntegrationDB(t)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	wf := newEnabledWorkflowWithStatus(wfID, []string{"chapter_planning"}, "failed")
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	result, err := svc.Put(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID})
	if err != nil {
		t.Fatalf("Put() integration error error = %v, want nil", err)
	}
	t.Cleanup(func() { cleanupBinding(t, context.Background(), pool, result.Binding.ID) })
	if !result.Created {
		t.Fatal("Put() Created = false, want true (integration error does not block)")
	}
}

func TestServicePutConcurrentFirstBind(t *testing.T) {
	pool, ctx := openIntegrationDB(t)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	key := "concurrent-first-bind-key"
	req := PutRequest{WorkflowConfigurationID: wfID}
	t.Cleanup(func() {
		cleanupProjectBindings(t, context.Background(), pool, projectID)
		cleanupIdempotency(t, context.Background(), pool, putScope(svc.actorID, projectID, StageChapterPlanning), key)
		cleanupIdempotency(t, context.Background(), pool, putScope(svc.actorID, projectID, StageChapterPlanning), "different-key")
	})

	// First bind succeeds.
	result, status, err := svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, req, key)
	if err != nil {
		t.Fatal(err)
	}
	if status != 201 {
		t.Fatalf("first bind status = %d, want 201", status)
	}
	t.Cleanup(func() { cleanupAuditForSubject(context.Background(), pool, result.Binding.ID.String()) })

	// Replay with the same key + payload returns the original 201 result and the
	// same binding id, without a duplicate business write or audit.
	result2, status2, err := svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, req, key)
	if err != nil {
		t.Fatalf("PutWithIdempotency() replay error = %v, want nil (same key same payload)", err)
	}
	if status2 != 201 {
		t.Fatalf("replay status = %d, want 201", status2)
	}
	if result2.Binding.ID != result.Binding.ID {
		t.Fatal("replay returned different binding")
	}
	if n := countBindingsForProject(t, ctx, pool, projectID); n != 1 {
		t.Fatalf("bindings = %d, want 1 after replay", n)
	}
	if n := countAuditForSubject(t, ctx, pool, result.Binding.ID.String()); n != 1 {
		t.Fatalf("audit count = %d, want 1 (no duplicate audit on replay)", n)
	}
	if n := countIdempotencyRecords(t, ctx, pool, putScope(svc.actorID, projectID, StageChapterPlanning), key); n != 1 {
		t.Fatalf("idempotency records = %d, want 1", n)
	}

	// A different key for an already-bound stage with no expectedVersion must
	// surface version_conflict (the frozen contract requires expectedVersion for
	// replacement).  The genuine concurrent-first-bind race, where two creates
	// collide on UNIQUE(project_id, stage), is covered by
	// TestServicePutConcurrentFirstBindGoroutines below.
	_, _, err = svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID}, "different-key")
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("PutWithIdempotency() existing-no-expectedVersion error = %v, want ErrVersionConflict", err)
	}
}

// TestServicePutConcurrentFirstBindGoroutines uses real goroutines with a
// shared start barrier: two different idempotency keys race to create the same
// project+stage binding.  Exactly one wins (201) and the loser gets 409
// binding_already_exists; the database ends with one binding and one audit.
func TestServicePutConcurrentFirstBindGoroutines(t *testing.T) {
	pool, ctx := openIntegrationDB(t)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	// Force a true race: both goroutines pass the NotFound check before either
	// inserts, then race the INSERT.  The advisory lock is key-scoped, so the
	// two distinct keys do not serialize each other and the UNIQUE(project_id,
	// stage) constraint decides the winner (201) and loser (409).
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	svc.beforeCreate = func() {
		ready <- struct{}{}
		<-release
	}

	req := PutRequest{WorkflowConfigurationID: wfID}
	keys := []string{"concurrent-goroutine-a", "concurrent-goroutine-b"}
	for _, k := range keys {
		cleanupIdempotency(t, ctx, pool, putScope(svc.actorID, projectID, StageChapterPlanning), k)
	}
	t.Cleanup(func() { cleanupProjectBindings(t, context.Background(), pool, projectID) })

	var wg sync.WaitGroup
	start := make(chan struct{})
	type outcome struct {
		status int
		err    error
	}
	results := make([]outcome, len(keys))
	for i, k := range keys {
		wg.Add(1)
		go func(idx int, key string) {
			defer wg.Done()
			<-start
			_, status, err := svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, req, key)
			results[idx] = outcome{status: status, err: err}
		}(i, k)
	}
	close(start)
	// Wait for both goroutines to reach the pre-INSERT hook, then release them
	// simultaneously so the INSERTs collide on the unique constraint.
	for range 2 {
		<-ready
	}
	close(release)
	wg.Wait()

	successes, conflicts := 0, 0
	for i, r := range results {
		switch {
		case r.err == nil && r.status == 201:
			successes++
		case errors.Is(r.err, ErrBindingAlreadyExists):
			conflicts++
		default:
			t.Fatalf("goroutine %d unexpected outcome: status=%d err=%v", i, r.status, r.err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent first bind: successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}

	if n := countBindingsForProject(t, ctx, pool, projectID); n != 1 {
		t.Fatalf("bindings after concurrency = %d, want 1", n)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE action='project_workflow_binding.create' AND payload->>'projectId'=$1", projectID.String()).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("create audit count = %d, want 1", auditCount)
	}
}

// TestServicePutSameKeyConcurrent uses the same Idempotency-Key from several
// goroutines.  All return the same result; there is exactly one business write,
// one audit, and one idempotency record.
func TestServicePutSameKeyConcurrent(t *testing.T) {
	pool, ctx := openIntegrationDB(t)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	key := "concurrent-same-key"
	cleanupIdempotency(t, ctx, pool, putScope(svc.actorID, projectID, StageChapterPlanning), key)
	t.Cleanup(func() {
		cleanupProjectBindings(t, context.Background(), pool, projectID)
		cleanupIdempotency(t, context.Background(), pool, putScope(svc.actorID, projectID, StageChapterPlanning), key)
	})

	req := PutRequest{WorkflowConfigurationID: wfID}
	const workers = 6
	var wg sync.WaitGroup
	start := make(chan struct{})
	type outcome struct {
		status int
		err    error
		id     uuid.UUID
	}
	results := make([]outcome, workers)
	for i := range workers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			res, status, err := svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, req, key)
			out := outcome{status: status, err: err}
			if err == nil {
				out.id = res.Binding.ID
			}
			results[idx] = out
		}(i)
	}
	close(start)
	wg.Wait()

	var firstID uuid.UUID
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("worker %d error = %v", i, r.err)
		}
		if r.status != 201 {
			t.Fatalf("worker %d status = %d, want 201", i, r.status)
		}
		if firstID == uuid.Nil {
			firstID = r.id
		} else if r.id != firstID {
			t.Fatalf("worker %d returned binding %s, want %s", i, r.id, firstID)
		}
	}
	if n := countBindingsForProject(t, ctx, pool, projectID); n != 1 {
		t.Fatalf("bindings = %d, want 1", n)
	}
	if n := countIdempotencyRecords(t, ctx, pool, putScope(svc.actorID, projectID, StageChapterPlanning), key); n != 1 {
		t.Fatalf("idempotency records = %d, want 1", n)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE action='project_workflow_binding.create' AND payload->>'projectId'=$1", projectID.String()).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("create audit count = %d, want 1", auditCount)
	}
}

func TestServicePutWithIdempotencySameKeySamePayload(t *testing.T) {
	pool, ctx := openIntegrationDB(t)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	key := "idem-test-key-" + uuid.New().String()[:8]
	req := PutRequest{WorkflowConfigurationID: wfID}
	scope := putScope(svc.actorID, projectID, StageChapterPlanning)
	t.Cleanup(func() {
		cleanupProjectBindings(t, context.Background(), pool, projectID)
		cleanupIdempotency(t, context.Background(), pool, scope, key)
	})
	result1, status1, err := svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, req, key)
	if err != nil {
		t.Fatal(err)
	}
	if status1 != 201 {
		t.Fatalf("first status = %d, want 201", status1)
	}
	t.Cleanup(func() { cleanupAuditForSubject(context.Background(), pool, result1.Binding.ID.String()) })

	// Verify idempotency record exists with the correct hash.
	record := idempotencyRecordFor(t, ctx, pool, scope, key)
	if record.RequestHash == "" {
		t.Fatal("idempotency record hash empty")
	}

	// Replay via the idempotent entry point returns the same binding and 201.
	result2, status2, err := svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, req, key)
	if err != nil {
		t.Fatalf("PutWithIdempotency() replay error = %v", err)
	}
	if status2 != 201 {
		t.Fatalf("replay status = %d, want 201", status2)
	}
	if result2.Binding.ID != result1.Binding.ID {
		t.Fatal("replay returned different binding")
	}

	// No duplicate audit, single binding, single idempotency record.
	if n := countAuditForSubject(t, ctx, pool, result1.Binding.ID.String()); n != 1 {
		t.Fatalf("audit count = %d, want 1 (no duplicate audit)", n)
	}
	if n := countBindingsForProject(t, ctx, pool, projectID); n != 1 {
		t.Fatalf("bindings = %d, want 1", n)
	}
	if n := countIdempotencyRecords(t, ctx, pool, scope, key); n != 1 {
		t.Fatalf("idempotency records = %d, want 1", n)
	}
}

func TestServicePutWithIdempotencySameKeyDifferentPayload(t *testing.T) {
	pool, ctx := openIntegrationDB(t)

	projectID := uuid.New()
	connID := uuid.New()
	wfID1 := uuid.New()
	wfID2 := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID1, connID, []string{"chapter_planning"})
	insertWorkflowConfig(t, ctx, pool, wfID2, connID, []string{"chapter_planning"})

	wf1 := newEnabledWorkflow(wfID1, []string{"chapter_planning"})
	wf2 := newEnabledWorkflow(wfID2, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		if id == wfID1 {
			return wf1, nil
		}
		return wf2, nil
	}}, "system")

	key := "idem-diff-key-" + uuid.New().String()[:8]
	scope := putScope(svc.actorID, projectID, StageChapterPlanning)
	t.Cleanup(func() {
		cleanupProjectBindings(t, context.Background(), pool, projectID)
		cleanupIdempotency(t, context.Background(), pool, scope, key)
	})
	result1, _, err := svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID1}, key)
	if err != nil {
		t.Fatalf("first put failed: %v", err)
	}
	t.Cleanup(func() { cleanupAuditForSubject(context.Background(), pool, result1.Binding.ID.String()) })

	_, _, err = svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID2}, key)
	if !errors.Is(err, ErrIdempotencyReused) {
		t.Fatalf("PutWithIdempotency() different payload error = %v, want ErrIdempotencyReused", err)
	}
}

// TestServicePutReplayPreservesStatusCode asserts that a replayed replace (200)
// and a replayed no-op (200) keep their first status code, while a replayed
// create keeps 201.
func TestServicePutReplayPreservesStatusCode(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID1 := uuid.New()
	wfID2 := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID1, connID, []string{"review"})
	insertWorkflowConfig(t, ctx, pool, wfID2, connID, []string{"review"})

	wf1 := newEnabledWorkflow(wfID1, []string{"review"})
	wf2 := newEnabledWorkflow(wfID2, []string{"review"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		if id == wfID1 {
			return wf1, nil
		}
		return wf2, nil
	}}, "system")

	scope := putScope(svc.actorID, projectID, StageReview)
	createKey := "replay-create-" + uuid.New().String()[:8]
	replaceKey := "replay-replace-" + uuid.New().String()[:8]
	noopKey := "replay-noop-" + uuid.New().String()[:8]
	t.Cleanup(func() {
		cleanupProjectBindings(t, context.Background(), pool, projectID)
		for _, k := range []string{createKey, replaceKey, noopKey} {
			cleanupIdempotency(t, context.Background(), pool, scope, k)
		}
	})

	// Create -> 201, replay -> 201.
	created, status, err := svc.PutWithIdempotency(ctx, projectID, StageReview, PutRequest{WorkflowConfigurationID: wfID1}, createKey)
	if err != nil {
		t.Fatal(err)
	}
	if status != 201 {
		t.Fatalf("create status = %d, want 201", status)
	}
	t.Cleanup(func() { cleanupAuditForSubject(context.Background(), pool, created.Binding.ID.String()) })
	if _, s, err := svc.PutWithIdempotency(ctx, projectID, StageReview, PutRequest{WorkflowConfigurationID: wfID1}, createKey); err != nil || s != 201 {
		t.Fatalf("create replay status=%d err=%v, want 201", s, err)
	}

	// Replace -> 200, replay -> 200.
	if _, s, err := svc.PutWithIdempotency(ctx, projectID, StageReview, PutRequest{WorkflowConfigurationID: wfID2, ExpectedVersion: intPtr(1)}, replaceKey); err != nil || s != 200 {
		t.Fatalf("replace status=%d err=%v, want 200", s, err)
	}
	if _, s, err := svc.PutWithIdempotency(ctx, projectID, StageReview, PutRequest{WorkflowConfigurationID: wfID2, ExpectedVersion: intPtr(1)}, replaceKey); err != nil || s != 200 {
		t.Fatalf("replace replay status=%d err=%v, want 200", s, err)
	}

	// No-op (same workflow, correct version) -> 200, replay -> 200.
	if _, s, err := svc.PutWithIdempotency(ctx, projectID, StageReview, PutRequest{WorkflowConfigurationID: wfID2, ExpectedVersion: intPtr(2)}, noopKey); err != nil || s != 200 {
		t.Fatalf("noop status=%d err=%v, want 200", s, err)
	}
	if _, s, err := svc.PutWithIdempotency(ctx, projectID, StageReview, PutRequest{WorkflowConfigurationID: wfID2, ExpectedVersion: intPtr(2)}, noopKey); err != nil || s != 200 {
		t.Fatalf("noop replay status=%d err=%v, want 200", s, err)
	}

	// No-op must not have bumped the version beyond the replace's version 2.
	current, err := repo.GetByProjectAndStage(ctx, projectID, StageReview)
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != 2 {
		t.Fatalf("version after noop = %d, want 2", current.Version)
	}
}

func TestServiceDeleteWithIdempotencySameKeySamePayload(t *testing.T) {
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
	t.Cleanup(func() { cleanupAuditForSubject(context.Background(), pool, created.ID.String()) })

	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{}, "system")
	key := "idem-del-key-" + uuid.New().String()[:8]
	req := DeleteRequest{ExpectedVersion: 1}
	scope := deleteScope(svc.actorID, projectID, StageRewrite)
	t.Cleanup(func() {
		cleanupProjectBindings(t, context.Background(), pool, projectID)
		cleanupIdempotency(t, context.Background(), pool, scope, key)
	})

	result1, status1, err := svc.DeleteWithIdempotency(ctx, projectID, StageRewrite, req, key)
	if err != nil {
		t.Fatal(err)
	}
	if status1 != 200 {
		t.Fatalf("delete status = %d, want 200", status1)
	}
	if !result1.Unbound {
		t.Fatal("DeleteWithIdempotency() Unbound = false, want true")
	}

	// Verify idempotency record exists with the correct hash.
	record := idempotencyRecordFor(t, ctx, pool, scope, key)
	if record.RequestHash == "" {
		t.Fatal("idempotency record hash empty")
	}

	// Replay via the idempotent entry point returns 200 even though the binding
	// is already deleted; it serves the cached first response.
	result2, status2, err := svc.DeleteWithIdempotency(ctx, projectID, StageRewrite, req, key)
	if err != nil {
		t.Fatalf("DeleteWithIdempotency() replay error = %v", err)
	}
	if status2 != 200 {
		t.Fatalf("replay status = %d, want 200", status2)
	}
	if result2.Unbound != result1.Unbound {
		t.Fatal("replay returned different result")
	}

	// No duplicate audit and a single idempotency record.
	if n := countAuditForSubject(t, ctx, pool, created.ID.String()); n != 1 {
		t.Fatalf("audit count = %d, want 1 (no duplicate audit)", n)
	}
	if n := countIdempotencyRecords(t, ctx, pool, scope, key); n != 1 {
		t.Fatalf("idempotency records = %d, want 1", n)
	}
}

// TestServicePutNoOpDoesNotWriteOrAudit verifies that a same-workflow PUT with
// the correct expectedVersion is a true no-op: no version bump, no updatedAt
// change, no audit, and no idempotency business write side effects.
func TestServicePutNoOpDoesNotWriteOrAudit(t *testing.T) {
	pool, ctx := openIntegrationDB(t)
	repo := NewPostgresRepository(pool)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	b, _ := New(uuid.New(), projectID, wfID, StageChapterPlanning)
	created, err := repo.Create(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupBinding(t, context.Background(), pool, created.ID)
		cleanupAuditForSubject(context.Background(), pool, created.ID.String())
	})

	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, "system")

	before, err := repo.GetByProjectAndStage(ctx, projectID, StageChapterPlanning)
	if err != nil {
		t.Fatal(err)
	}
	result, status, err := svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID, ExpectedVersion: intPtr(1)}, "noop-key-"+uuid.New().String()[:8])
	if err != nil {
		t.Fatalf("noop error = %v", err)
	}
	if status != 200 {
		t.Fatalf("noop status = %d, want 200", status)
	}
	if !result.NoChange {
		t.Fatal("noop NoChange = false, want true")
	}
	after, err := repo.GetByProjectAndStage(ctx, projectID, StageChapterPlanning)
	if err != nil {
		t.Fatal(err)
	}
	if after.Version != before.Version {
		t.Fatalf("noop version changed: %d -> %d", before.Version, after.Version)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Fatalf("noop updatedAt changed: %v -> %v", before.UpdatedAt, after.UpdatedAt)
	}
	if n := countAuditForSubject(t, ctx, pool, created.ID.String()); n != 0 {
		t.Fatalf("noop audit count = %d, want 0", n)
	}
}

func TestServiceDeleteVersionConflict(t *testing.T) {
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
	t.Cleanup(func() {
		cleanupBinding(t, context.Background(), pool, created.ID)
		cleanupAuditForSubject(context.Background(), pool, created.ID.String())
	})

	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{}, "system")
	_, err = svc.Delete(ctx, projectID, StageRewrite, DeleteRequest{ExpectedVersion: 99})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Delete() version conflict error = %v, want ErrVersionConflict", err)
	}
}

// TestServiceDeleteVersionConflictDetails verifies the DELETE version conflict
// carries expected/current/projectId/stage context for the 409 details.
func TestServiceDeleteVersionConflictDetails(t *testing.T) {
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
	t.Cleanup(func() {
		cleanupBinding(t, context.Background(), pool, created.ID)
		cleanupAuditForSubject(context.Background(), pool, created.ID.String())
	})

	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{}, "system")
	_, err = svc.Delete(ctx, projectID, StageRewrite, DeleteRequest{ExpectedVersion: 99})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("Delete() error = %v, want ErrVersionConflict", err)
	}
	var conflict *VersionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Delete() conflict is not a VersionConflictError")
	}
	if conflict.ExpectedVersion != 99 || conflict.CurrentVersion != 1 || conflict.Stage != StageRewrite || conflict.ProjectID != projectID {
		t.Fatalf("delete conflict details = %+v", conflict)
	}
}

// TestServiceAuditFailureRollsBackBinding forces an audit INSERT to fail with a
// uniquely named trigger scoped to one actor and one action.  It then verifies
// that the binding, audit row, and idempotency record are all rolled back.
func TestServiceAuditFailureRollsBackBinding(t *testing.T) {
	pool, ctx := openIntegrationDB(t)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	actorID := "audit-fail-" + uuid.New().String()
	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, actorID)

	key := "audit-rollback-key-" + uuid.New().String()[:8]
	scope := putScope(actorID, projectID, StageChapterPlanning)
	t.Cleanup(func() { cleanupIdempotency(t, context.Background(), pool, scope, key) })

	triggerName := "test_audit_failure_" + strings.ReplaceAll(uuid.New().String(), "-", "_")
	funcName := "test_audit_failure_func_" + strings.ReplaceAll(uuid.New().String(), "-", "_")

	// Install a trigger that only blocks audit rows for this actor/action and
	// this projectId inside the payload.
	createFuncSQL := fmt.Sprintf(`CREATE OR REPLACE FUNCTION %s() RETURNS trigger AS $$
BEGIN
	IF NEW.actor_id = '%s' AND NEW.action = 'project_workflow_binding.create' AND NEW.payload->>'projectId' = '%s' THEN
		RAISE EXCEPTION 'forced audit failure';
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;`, funcName, actorID, projectID.String())
	if _, err := pool.Exec(ctx, createFuncSQL); err != nil {
		t.Fatalf("create audit failure function: %v", err)
	}
	createTriggerSQL := fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON audit_logs
		FOR EACH ROW
		WHEN (NEW.actor_id = '%s' AND NEW.action = 'project_workflow_binding.create')
		EXECUTE FUNCTION %s();`, triggerName, actorID, funcName)
	if _, err := pool.Exec(ctx, createTriggerSQL); err != nil {
		t.Fatalf("create audit failure trigger: %v", err)
	}
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanCancel()
		_, _ = pool.Exec(cleanCtx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON audit_logs; DROP FUNCTION IF EXISTS %s() CASCADE", triggerName, funcName))
	})

	var err error
	_, _, err = svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID}, key)
	if err == nil {
		t.Fatal("audit failure expected got nil")
	}
	if !strings.Contains(err.Error(), "forced audit failure") {
		t.Fatalf("expected forced audit failure, got %v", err)
	}
	if n := countBindingsForProject(t, ctx, pool, projectID); n != 0 {
		t.Fatalf("bindings after audit failure = %d, want 0", n)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE action='project_workflow_binding.create' AND payload->>'projectId'=$1", projectID.String()).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("audit after failure = %d, want 0", auditCount)
	}
	if n := countIdempotencyRecords(t, ctx, pool, scope, key); n != 0 {
		t.Fatalf("idempotency records after rollback = %d, want 0", n)
	}
}

// TestServiceIdempotencyRecordFailureRollsBack forces the idempotency record
// INSERT to fail with a uniquely named trigger scoped to the scope+key used by
// this test.  It then verifies that the binding, audit, and idempotency record
// are all rolled back.
func TestServiceIdempotencyRecordFailureRollsBack(t *testing.T) {
	pool, ctx := openIntegrationDB(t)

	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()
	insertProject(t, ctx, pool, projectID)
	insertWorkflowConfig(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	actorID := "idempotency-fail-" + uuid.New().String()
	wf := newEnabledWorkflow(wfID, []string{"chapter_planning"})
	svc := NewService(pool, mockProjectRepo{}, mockWorkflowRepo{getFn: func(_ context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error) {
		return wf, nil
	}}, actorID)

	key := "idem-rollback-key-" + uuid.New().String()[:8]
	scope := putScope(actorID, projectID, StageChapterPlanning)
	t.Cleanup(func() {
		cleanupProjectBindings(t, context.Background(), pool, projectID)
		cleanupIdempotency(t, context.Background(), pool, scope, key)
	})

	triggerName := "test_idempotency_failure_" + strings.ReplaceAll(uuid.New().String(), "-", "_")
	funcName := "test_idempotency_failure_func_" + strings.ReplaceAll(uuid.New().String(), "-", "_")

	// Install a trigger that only blocks this exact scope+key insert.
	createFuncSQL := fmt.Sprintf(`CREATE OR REPLACE FUNCTION %s() RETURNS trigger AS $$
BEGIN
	IF NEW.scope = '%s' AND NEW.idempotency_key = '%s' THEN
		RAISE EXCEPTION 'forced idempotency failure';
	END IF;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;`, funcName, scope, key)
	if _, err := pool.Exec(ctx, createFuncSQL); err != nil {
		t.Fatalf("create idempotency failure function: %v", err)
	}
	createTriggerSQL := fmt.Sprintf(`CREATE TRIGGER %s BEFORE INSERT ON idempotency_records
		FOR EACH ROW
		WHEN (NEW.scope = '%s' AND NEW.idempotency_key = '%s')
		EXECUTE FUNCTION %s();`, triggerName, scope, key, funcName)
	if _, err := pool.Exec(ctx, createTriggerSQL); err != nil {
		t.Fatalf("create idempotency failure trigger: %v", err)
	}
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanCancel()
		_, _ = pool.Exec(cleanCtx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON idempotency_records; DROP FUNCTION IF EXISTS %s() CASCADE", triggerName, funcName))
	})

	var err error
	_, _, err = svc.PutWithIdempotency(ctx, projectID, StageChapterPlanning, PutRequest{WorkflowConfigurationID: wfID}, key)
	if err == nil {
		t.Fatal("idempotency record failure expected, got nil")
	}
	if !strings.Contains(err.Error(), "forced idempotency failure") {
		t.Fatalf("error = %v, want forced idempotency failure", err)
	}
	if n := countBindingsForProject(t, ctx, pool, projectID); n != 0 {
		t.Fatalf("bindings after idempotency rollback = %d, want 0", n)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_logs WHERE action='project_workflow_binding.create' AND payload->>'projectId'=$1", projectID.String()).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("create audit after idempotency rollback = %d, want 0", auditCount)
	}
	if n := countIdempotencyRecords(t, ctx, pool, scope, key); n != 0 {
		t.Fatalf("idempotency records after rollback = %d, want 0", n)
	}
}

func intPtr(v int) *int { return &v }

var _ = pgx.ErrNoRows
var _ = NewPostgresRepository
var _ = newDisabledWorkflowWithStatus
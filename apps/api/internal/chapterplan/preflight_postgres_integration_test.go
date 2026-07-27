package chapterplan

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type preflightBindingReader struct {
	binding workflowbinding.ProjectWorkflowBinding
	err     error
}

func (r preflightBindingReader) GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error) {
	return r.binding, r.err
}

type preflightWorkflowReader struct{ workflow globalconfig.Workflow }

func (r preflightWorkflowReader) GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error) {
	return r.workflow, nil
}

type preflightConnectionReader struct{ connection globalconfig.Connection }

func (r preflightConnectionReader) GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error) {
	return r.connection, nil
}

type preflightRunCreator struct{ calls int }

func (r *preflightRunCreator) CreateRun(context.Context, workflowrun.CreateRunCommand) (workflowrun.WorkflowRun, error) {
	r.calls++
	return workflowrun.WorkflowRun{}, errors.New("external workflow executor must not be called by preflight")
}

type preflightPersistenceState struct {
	WorkflowRuns int
	Batches      int
	Candidates   int
	Revisions    int
	Plans        json.RawMessage
	Projects     json.RawMessage
	Idempotency  json.RawMessage
}

func snapshotPreflightPersistence(t *testing.T, ctx context.Context, db *pgxpool.Pool, projectID uuid.UUID) preflightPersistenceState {
	t.Helper()
	state := preflightPersistenceState{}
	for _, entry := range []struct {
		table string
		dst   *int
	}{
		{"workflow_run_records", &state.WorkflowRuns},
		{"chapter_plan_candidate_batches", &state.Batches},
		{"chapter_plan_candidates", &state.Candidates},
		{"chapter_plan_revisions", &state.Revisions},
	} {
		if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM "+entry.table).Scan(entry.dst); err != nil {
			t.Fatalf("count %s: %v", entry.table, err)
		}
	}
	for _, entry := range []struct {
		query string
		dst   *json.RawMessage
	}{
		{"SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id), '[]'::jsonb) FROM chapter_plans p WHERE project_id=$1", &state.Plans},
		{"SELECT COALESCE(jsonb_agg(to_jsonb(p) ORDER BY p.id), '[]'::jsonb) FROM projects p WHERE id=$1", &state.Projects},
		{"SELECT COALESCE(jsonb_agg(to_jsonb(i) ORDER BY i.id), '[]'::jsonb) FROM idempotency_records i WHERE $1::uuid IS NOT NULL", &state.Idempotency},
	} {
		if err := db.QueryRow(ctx, entry.query, projectID).Scan(entry.dst); err != nil {
			t.Fatalf("snapshot persistence: %v", err)
		}
	}
	return state
}

func requirePreflightPersistenceUnchanged(t *testing.T, before, after preflightPersistenceState) {
	t.Helper()
	if before.WorkflowRuns != after.WorkflowRuns || before.Batches != after.Batches || before.Candidates != after.Candidates || before.Revisions != after.Revisions {
		t.Fatalf("preflight changed table counts: before=%+v after=%+v", before, after)
	}
	if string(before.Plans) != string(after.Plans) {
		t.Fatalf("preflight changed chapter_plans content or version: before=%s after=%s", before.Plans, after.Plans)
	}
	if string(before.Projects) != string(after.Projects) {
		t.Fatalf("preflight changed projects content: before=%s after=%s", before.Projects, after.Projects)
	}
	if string(before.Idempotency) != string(after.Idempotency) {
		t.Fatalf("preflight changed business idempotency records: before=%s after=%s", before.Idempotency, after.Idempotency)
	}
}

func newPostgresPreflightService(t *testing.T, ctx context.Context, db *pgxpool.Pool, f fixture, bindingErr error) (*Service, *preflightRunCreator) {
	t.Helper()
	service, err := NewPostgresService(project.NewPostgresRepository(db), db, "test-hmac-secret-1234567890")
	if err != nil {
		t.Fatal(err)
	}
	workflowID, connectionID := uuid.New(), uuid.New()
	runs := &preflightRunCreator{}
	service.ConfigureChapterPlanningRuntime(
		preflightBindingReader{binding: workflowbinding.ProjectWorkflowBinding{ID: uuid.New(), ProjectID: f.project, Stage: workflowbinding.StageChapterPlanning, WorkflowConfigurationID: workflowID, Version: 1}, err: bindingErr},
		preflightWorkflowReader{workflow: globalconfig.Workflow{Common: globalconfig.Common{ID: workflowID, Enabled: true, Version: 1}, ConnectionID: connectionID, WorkflowType: "n8n"}},
		preflightConnectionReader{connection: globalconfig.Connection{Common: globalconfig.Common{ID: connectionID, Enabled: true, Version: 1}}},
		runs,
	)
	return service, runs
}

func postgresPreflightRequest() PreflightRequest {
	return PreflightRequest{
		GenerationMode:         "full",
		Target:                 GenerationTargetRequest{TargetTotalChapters: 2},
		StorylineSelectionMode: "auto_balanced",
		ContextOptions:         json.RawMessage(`{"includeProjectMaterials":true,"includeUnpaidForeshadowings":true,"includePriorChapterSummaries":true,"coreSettingsOnly":false}`),
		ActorID:                "preflight-postgres-test",
	}
}

func seedPreflightPlan(t *testing.T, ctx context.Context, db *pgxpool.Pool, f fixture) {
	t.Helper()
	repo := mustNewRepo(t, db)
	p := plan(f, 1)
	p.Storylines = []StorylineRef{{ID: f.storylines[0], Relation: "primary"}}
	if err := repo.SaveMock(ctx, Run{ID: uuid.New(), ProjectID: f.project}, []Plan{p}); err != nil {
		t.Fatalf("seed chapter plan: %v", err)
	}
}

func TestPostgresPreflightPassedHasNoSideEffects(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	seedPreflightPlan(t, ctx, db, f)
	service, executor := newPostgresPreflightService(t, ctx, db, f, nil)

	before := snapshotPreflightPersistence(t, ctx, db, f.project)
	result, err := service.Preflight(ctx, f.project, postgresPreflightRequest())
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if !result.Passed || result.Token == "" {
		t.Fatalf("expected passed preflight with token, got %+v", result)
	}
	after := snapshotPreflightPersistence(t, ctx, db, f.project)
	requirePreflightPersistenceUnchanged(t, before, after)
	if executor.calls != 0 {
		t.Fatalf("external workflow executor calls=%d, want 0", executor.calls)
	}
}

func TestPostgresPreflightBlockedHasNoSideEffects(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	seedPreflightPlan(t, ctx, db, f)
	service, executor := newPostgresPreflightService(t, ctx, db, f, errors.New("workflow binding unavailable"))

	before := snapshotPreflightPersistence(t, ctx, db, f.project)
	result, err := service.Preflight(ctx, f.project, postgresPreflightRequest())
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if result.Passed || len(result.Blockers) != 1 || result.Blockers[0].Code != "project_binding_missing" {
		t.Fatalf("expected blocked preflight for missing binding, got %+v", result)
	}
	after := snapshotPreflightPersistence(t, ctx, db, f.project)
	requirePreflightPersistenceUnchanged(t, before, after)
	if executor.calls != 0 {
		t.Fatalf("external workflow executor calls=%d, want 0", executor.calls)
	}
}

package chapterplan

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strings"
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

type mutablePreflightBindingReader struct {
	binding workflowbinding.ProjectWorkflowBinding
}

func (r *mutablePreflightBindingReader) GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error) {
	return r.binding, nil
}

type mutablePreflightWorkflowReader struct{ workflow globalconfig.Workflow }

func (r *mutablePreflightWorkflowReader) GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error) {
	return r.workflow, nil
}

type mutablePreflightConnectionReader struct{ connection globalconfig.Connection }

func (r *mutablePreflightConnectionReader) GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error) {
	return r.connection, nil
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

func TestPostgresPersistedGenerationContextDigestMatchesSnapshot(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	seedPreflightPlan(t, ctx, db, f)

	workflowID, connectionID := uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config) VALUES($1,$2,'n8n','http://internal.example.test/private?token=must-not-persist','api_key',30,$3)", connectionID, "persisted-context-connection-"+connectionID.String(), json.RawMessage(`{"credential":"must-not-persist"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version) VALUES($1,$2,$3,'[\"chapter_planning\"]',$4,'v1','v1')", workflowID, "persisted-context-workflow-"+workflowID.String(), connectionID, json.RawMessage(`{"providerSecret":"must-not-persist"}`)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE id=$1", workflowID)
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", connectionID)
	})
	bindings := &mutablePreflightBindingReader{binding: workflowbinding.ProjectWorkflowBinding{ID: uuid.New(), ProjectID: f.project, Stage: workflowbinding.StageChapterPlanning, WorkflowConfigurationID: workflowID, Version: 7}}
	workflows := &mutablePreflightWorkflowReader{workflow: globalconfig.Workflow{Common: globalconfig.Common{ID: workflowID, Enabled: true, Version: 11}, ConnectionID: connectionID, WorkflowType: "n8n", ApplicableStages: []string{"chapter_planning"}, TypeConfig: json.RawMessage(`{"providerSecret":"must-not-persist"}`), DefaultParameters: json.RawMessage(`{"authorization":"must-not-persist"}`)}}
	connections := &mutablePreflightConnectionReader{connection: globalconfig.Connection{Common: globalconfig.Common{ID: connectionID, Enabled: true, Version: 13}, ConnectionType: "n8n", BaseURL: "http://internal.example.test/private?token=must-not-persist", TypeConfig: json.RawMessage(`{"credential":"must-not-persist"}`)}}
	runs := workflowrun.NewService(workflowrun.NewPostgresRepository(db), project.NewPostgresRepository(db), bindings, workflows, connections)
	service, err := NewPostgresService(project.NewPostgresRepository(db), db, "test-hmac-secret-1234567890")
	if err != nil {
		t.Fatal(err)
	}
	service.ConfigureChapterPlanningRuntime(bindings, workflows, connections, runs)
	request := postgresPreflightRequest()
	request.StorylineSelectionMode = "specified"
	request.StorylineIDs = []uuid.UUID{f.storylines[2], f.storylines[0]}
	wantSelected := sortedUUIDs(request.StorylineIDs)
	preflight, err := service.Preflight(ctx, f.project, request)
	if err != nil || !preflight.Passed {
		t.Fatalf("Preflight result=%+v err=%v", preflight, err)
	}
	run, err := service.CreateChapterPlanningRun(ctx, f.project, request.ActorID, preflight.Token, "persisted-generation-context")
	if err != nil {
		t.Fatalf("CreateChapterPlanningRun: %v", err)
	}

	var persistedPayload json.RawMessage
	if err = db.QueryRow(ctx, "SELECT input_payload FROM workflow_run_records WHERE id=$1", run.ID).Scan(&persistedPayload); err != nil {
		t.Fatalf("read persisted workflow input payload: %v", err)
	}
	var persisted struct {
		GenerationContextDigest string                    `json:"generationContextDigest"`
		GenerationContext       GenerationContextSnapshot `json:"generationContext"`
	}
	if err = json.Unmarshal(persistedPayload, &persisted); err != nil {
		t.Fatalf("decode persisted workflow input payload: %v", err)
	}
	recomputed, err := digestGenerationContext(persisted.GenerationContext)
	if err != nil {
		t.Fatalf("recompute persisted generation context digest: %v", err)
	}
	if recomputed != persisted.GenerationContext.InputDigest || recomputed != persisted.GenerationContextDigest {
		t.Fatalf("persisted digest mismatch: recomputed=%s snapshot=%s payload=%s", recomputed, persisted.GenerationContext.InputDigest, persisted.GenerationContextDigest)
	}

	var input struct {
		Target                 BatchTarget `json:"target"`
		StorylineSelectionMode string      `json:"storylineSelectionMode"`
		StorylineIDs           []uuid.UUID `json:"storylineIds"`
	}
	if err = json.Unmarshal(persisted.GenerationContext.InputSnapshot, &input); err != nil {
		t.Fatalf("decode persisted input snapshot: %v", err)
	}
	if input.Target != preflight.Target || input.StorylineSelectionMode != "specified" || !slices.Equal(input.StorylineIDs, wantSelected) {
		t.Fatalf("persisted normalized input snapshot=%+v", input)
	}
	var storylineSnapshot struct {
		Selected  []uuid.UUID `json:"selected"`
		Available []struct {
			ID uuid.UUID `json:"id"`
		} `json:"available"`
		Materials []struct {
			ID uuid.UUID `json:"material_id"`
		} `json:"materials"`
		Foreshadowings []struct {
			ID uuid.UUID `json:"id"`
		} `json:"foreshadowings"`
	}
	if err = json.Unmarshal(persisted.GenerationContext.StorylineSnapshot, &storylineSnapshot); err != nil {
		t.Fatalf("decode persisted storyline snapshot: %v", err)
	}
	if !slices.Equal(storylineSnapshot.Selected, wantSelected) || len(storylineSnapshot.Available) != len(f.storylines) || len(storylineSnapshot.Materials) != len(f.materials) || len(storylineSnapshot.Foreshadowings) != len(f.foreshadowings) {
		t.Fatalf("persisted storyline references=%+v", storylineSnapshot)
	}
	availableIDs := make([]uuid.UUID, len(storylineSnapshot.Available))
	materialIDs := make([]uuid.UUID, len(storylineSnapshot.Materials))
	foreshadowingIDs := make([]uuid.UUID, len(storylineSnapshot.Foreshadowings))
	for i := range storylineSnapshot.Available {
		availableIDs[i] = storylineSnapshot.Available[i].ID
	}
	for i := range storylineSnapshot.Materials {
		materialIDs[i] = storylineSnapshot.Materials[i].ID
	}
	for i := range storylineSnapshot.Foreshadowings {
		foreshadowingIDs[i] = storylineSnapshot.Foreshadowings[i].ID
	}
	if !slices.Equal(availableIDs, sortedUUIDs(f.storylines)) || !slices.Equal(materialIDs, sortedUUIDs(f.materials)) || !slices.Equal(foreshadowingIDs, sortedUUIDs(f.foreshadowings)) {
		t.Fatalf("persisted source references=%+v", storylineSnapshot)
	}
	var bindingSnapshot struct {
		BindingID      uuid.UUID `json:"workflowBindingId"`
		BindingVersion int       `json:"workflowBindingVersion"`
		ConfigID       uuid.UUID `json:"workflowConfigurationId"`
		ConfigVersion  int       `json:"workflowConfigurationVersion"`
		ConfigSource   string    `json:"workflowConfigurationSource"`
	}
	if err = json.Unmarshal(persisted.GenerationContext.WorkflowBindingSnapshot, &bindingSnapshot); err != nil {
		t.Fatalf("decode persisted workflow binding snapshot: %v", err)
	}
	if bindingSnapshot.BindingID != bindings.binding.ID || bindingSnapshot.BindingVersion != 7 || bindingSnapshot.ConfigID != workflowID || bindingSnapshot.ConfigVersion != 11 || bindingSnapshot.ConfigSource != "n8n" {
		t.Fatalf("persisted workflow binding snapshot=%+v", bindingSnapshot)
	}
	serializedSnapshot := strings.ToLower(string(persisted.GenerationContext.InputSnapshot) + string(persisted.GenerationContext.StorylineSnapshot) + string(persisted.GenerationContext.WorkflowBindingSnapshot))
	for _, forbidden := range []string{"credential", "secret", "token", "authorization", "internal.example.test", "providersecret"} {
		if strings.Contains(serializedSnapshot, forbidden) {
			t.Fatalf("persisted generation context contains sensitive value %q: %s", forbidden, serializedSnapshot)
		}
	}

	persistedBefore := append(json.RawMessage(nil), persistedPayload...)
	if _, err = db.Exec(ctx, "UPDATE storylines SET name='changed storyline' WHERE id=$1", f.storylines[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "UPDATE materials SET name='changed material' WHERE id=$1", f.materials[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "UPDATE foreshadowings SET title='changed foreshadowing' WHERE id=$1", f.foreshadowings[0]); err != nil {
		t.Fatal(err)
	}
	planRepo := mustNewRepo(t, db)
	currentPlan, err := planRepo.GetByID(ctx, mustExistingPlanID(t, ctx, db, f.project))
	if err != nil {
		t.Fatal(err)
	}
	currentPlan.Title = "changed chapter plan"
	if _, err = planRepo.Update(ctx, currentPlan, currentPlan.Version); err != nil {
		t.Fatal(err)
	}
	bindings.binding.Version++
	workflows.workflow.Version++
	workflows.workflow.WorkflowType = "changed-source"
	var persistedAfter json.RawMessage
	if err = db.QueryRow(ctx, "SELECT input_payload FROM workflow_run_records WHERE id=$1", run.ID).Scan(&persistedAfter); err != nil {
		t.Fatal(err)
	}
	if string(persistedBefore) != string(persistedAfter) {
		t.Fatalf("persisted generation context changed after source mutation: before=%s after=%s", persistedBefore, persistedAfter)
	}
}

func mustExistingPlanID(t *testing.T, ctx context.Context, db *pgxpool.Pool, projectID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := db.QueryRow(ctx, "SELECT id FROM chapter_plans WHERE project_id=$1 ORDER BY chapter_no LIMIT 1", projectID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func sortedUUIDs(values []uuid.UUID) []uuid.UUID {
	ordered := append([]uuid.UUID(nil), values...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].String() < ordered[j].String() })
	return ordered
}

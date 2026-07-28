package chapterplan

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

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
	return workflowrun.WorkflowRun{ID: uuid.New()}, nil
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

func TestPostgresPreflightBlocksProjectWithoutStoryline(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	service, executor := newPostgresPreflightService(t, ctx, db, f, nil)

	result, err := service.Preflight(ctx, f.otherProject, postgresPreflightRequest())
	if err != nil {
		t.Fatalf("Preflight: %v", err)
	}
	if result.Passed || result.Token != "" || len(result.Blockers) != 1 || result.Blockers[0].Code != "storyline_reference_invalid" {
		t.Fatalf("unexpected result for project without storylines: %+v", result)
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(result.InputDigest) {
		t.Fatalf("input digest=%q, want SHA-256", result.InputDigest)
	}
	if executor.calls != 0 {
		t.Fatalf("external workflow executor calls=%d, want 0", executor.calls)
	}
}

func TestPreflightTokenActorAndProjectBinding(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	service, _ := newPostgresPreflightService(t, ctx, db, f, nil)
	fixedNow := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	request := postgresPreflightRequest()
	request.ActorID = "actor-a"
	preflight, err := service.Preflight(ctx, f.project, request)
	if err != nil || !preflight.Passed {
		t.Fatalf("Preflight result=%+v err=%v", preflight, err)
	}
	claims, err := VerifyPreflightToken(service.plans.(*Repository).HMACSecret(), preflight.Token, fixedNow)
	if err != nil || claims.ActorID != request.ActorID || claims.ProjectID != f.project || claims.IssuedAt != fixedNow.Unix() || claims.ExpiresAt != fixedNow.Add(10*time.Minute).Unix() {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	if _, err = service.CreateChapterPlanningRun(ctx, f.project, request.ActorID, preflight.Token, "same-actor"); err != nil {
		t.Fatalf("same actor must create run: %v", err)
	}
	if _, err = service.CreateChapterPlanningRun(ctx, f.project, "actor-b", preflight.Token, "other-actor"); !errors.Is(err, ErrPreflightInputChanged) {
		t.Fatalf("different actor error=%v", err)
	}
	if _, err = service.CreateChapterPlanningRun(ctx, uuid.New(), request.ActorID, preflight.Token, "other-project"); !errors.Is(err, ErrPreflightInputChanged) {
		t.Fatalf("different project error=%v", err)
	}
}

func TestPreflightTokenRejectsSnapshotChanges(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	service, _ := newPostgresPreflightService(t, ctx, db, f, nil)
	fixedNow := time.Date(2026, 7, 27, 11, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }
	request := postgresPreflightRequest()
	request.ActorID = "snapshot-actor"
	preflight, err := service.Preflight(ctx, f.project, request)
	if err != nil || !preflight.Passed {
		t.Fatalf("Preflight result=%+v err=%v", preflight, err)
	}
	claims, err := VerifyPreflightToken(service.plans.(*Repository).HMACSecret(), preflight.Token, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*PreflightTokenClaims)
	}{
		{"target", func(c *PreflightTokenClaims) { c.Target.EndChapterNo++ }},
		{"generation-mode", func(c *PreflightTokenClaims) { c.GenerationMode = "append" }},
		{"storyline-selection-mode", func(c *PreflightTokenClaims) { c.StorylineSelectionMode = "specified" }},
		{"storyline-ids", func(c *PreflightTokenClaims) { c.StorylineIDs = []uuid.UUID{uuid.New()} }},
		{"context-options", func(c *PreflightTokenClaims) {
			c.ContextOptions = json.RawMessage(`{"includeProjectMaterials":false,"includeUnpaidForeshadowings":true,"includePriorChapterSummaries":true,"coreSettingsOnly":false}`)
		}},
		{"binding", func(c *PreflightTokenClaims) { c.BindingVersion++ }},
		{"snapshot-digest", func(c *PreflightTokenClaims) { c.InputDigest = strings.Repeat("f", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := claims
			tc.mutate(&changed)
			token, err := SignPreflightToken(service.plans.(*Repository).HMACSecret(), changed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = service.CreateChapterPlanningRun(ctx, f.project, request.ActorID, token, "changed-"+tc.name); !errors.Is(err, ErrPreflightInputChanged) {
				t.Fatalf("changed %s error=%v", tc.name, err)
			}
		})
	}
}

func TestContextOptionsRequireAllStrictBooleanFields(t *testing.T) {
	valid := json.RawMessage(`{"includeProjectMaterials":true,"includeUnpaidForeshadowings":true,"includePriorChapterSummaries":true,"coreSettingsOnly":false}`)
	if !validContextOptions(valid) {
		t.Fatal("complete strict boolean options must be valid")
	}
	for _, raw := range []json.RawMessage{
		nil,
		json.RawMessage(`null`),
		json.RawMessage(`{}`),
		json.RawMessage(`{"includeProjectMaterials":true,"includeUnpaidForeshadowings":true,"includePriorChapterSummaries":true}`),
		json.RawMessage(`{"includeProjectMaterials":true,"includeUnpaidForeshadowings":true,"includePriorChapterSummaries":true,"coreSettingsOnly":null}`),
		json.RawMessage(`{"includeProjectMaterials":"true","includeUnpaidForeshadowings":true,"includePriorChapterSummaries":true,"coreSettingsOnly":false}`),
		json.RawMessage(`{"includeProjectMaterials":true,"includeUnpaidForeshadowings":true,"includePriorChapterSummaries":true,"coreSettingsOnly":false,"extra":true}`),
	} {
		if validContextOptions(raw) {
			t.Fatalf("context options must be rejected: %s", raw)
		}
	}
}

func TestPostgresPreflightStorylineReferenceInvalidReturnsStableDigest(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	service, _ := newPostgresPreflightService(t, ctx, db, f, nil)
	request := postgresPreflightRequest()
	request.StorylineSelectionMode = "specified"

	first, err := service.Preflight(ctx, f.project, request)
	if err != nil {
		t.Fatalf("first Preflight: %v", err)
	}
	second, err := service.Preflight(ctx, f.project, request)
	if err != nil {
		t.Fatalf("second Preflight: %v", err)
	}
	if first.Passed || first.Token != "" || first.Target != (BatchTarget{}) || first.BindingID != uuid.Nil || first.BindingVersion != 0 || len(first.Blockers) != 1 || first.Blockers[0].Code != "storyline_reference_invalid" {
		t.Fatalf("unexpected invalid storyline result: %+v", first)
	}
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(first.InputDigest) {
		t.Fatalf("invalid storyline digest=%q", first.InputDigest)
	}
	if first.InputDigest != second.InputDigest {
		t.Fatalf("invalid storyline digest must be stable: first=%s second=%s", first.InputDigest, second.InputDigest)
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
	replayed, err := service.CreateChapterPlanningRun(ctx, f.project, request.ActorID, preflight.Token, "persisted-generation-context")
	if err != nil || replayed.ID != run.ID {
		t.Fatalf("CreateChapterPlanningRun replay=%+v original=%+v err=%v", replayed, run, err)
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
	var rawStorylineSnapshot struct {
		Available []map[string]json.RawMessage `json:"available"`
	}
	if err = json.Unmarshal(persisted.GenerationContext.StorylineSnapshot, &rawStorylineSnapshot); err != nil || len(rawStorylineSnapshot.Available) == 0 {
		t.Fatalf("decode raw persisted storyline snapshot: value=%s err=%v", persisted.GenerationContext.StorylineSnapshot, err)
	}
	if _, ok := rawStorylineSnapshot.Available[0]["id"]; !ok {
		t.Fatalf("persisted storyline snapshot must use lowercase id: %s", persisted.GenerationContext.StorylineSnapshot)
	}
	if _, ok := rawStorylineSnapshot.Available[0]["ID"]; ok {
		t.Fatalf("persisted storyline snapshot contains incompatible ID field: %s", persisted.GenerationContext.StorylineSnapshot)
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

func TestPostgresPreflightCreateConsumeUsesFrozenBaseSnapshot(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	f := newFixture(t, ctx, db)
	seedPreflightPlan(t, ctx, db, f)
	planID := mustExistingPlanID(t, ctx, db, f.project)
	revisionID := uuid.New()
	base := NormalizedCandidate{
		ChapterNo: 1, Title: "One", Summary: "one", ChapterPurpose: "plot_advance",
		StorylineRefs:     []NormalizedReference{{ID: f.storylines[0], ProjectID: f.project, Label: "story", Relation: "primary", Version: 1}, {ID: f.storylines[1], ProjectID: f.project, Label: "story two", Relation: "secondary", Position: 1, Version: 1}},
		MaterialRefs:      []NormalizedReference{{ID: f.materials[0], ProjectID: f.project, Label: "material", Relation: "material_ref", Version: 1}, {ID: f.materials[1], ProjectID: f.project, Label: "material two", Relation: "material_ref", Position: 1, Version: 1}},
		ForeshadowingRefs: []NormalizedReference{{ID: f.foreshadowings[0], ProjectID: f.project, Label: "foreshadowing", Relation: "foreshadowing_ref", Version: 1}, {ID: f.foreshadowings[1], ProjectID: f.project, Label: "foreshadowing two", Relation: "foreshadowing_ref", Position: 1, Version: 1}},
	}
	baseSnapshot, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO chapter_plan_revisions(id,chapter_plan_id,project_id,revision_no,snapshot,change_type,created_by) VALUES($1,$2,$3,1,$4,'legacy_backfill','cf15-r04')`, revisionID, planID, f.project, baseSnapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "UPDATE chapter_plans SET current_revision_id=$2,title='current scalar must not replace revision' WHERE id=$1", planID, revisionID); err != nil {
		t.Fatal(err)
	}

	connectionID, workflowID := uuid.New(), uuid.New()
	if _, err = db.Exec(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config) VALUES($1,$2,'n8n','http://localhost:5678','api_key',30,'{}')", connectionID, "r04-connection-"+connectionID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version) VALUES($1,$2,$3,'[\"chapter_planning\"]','{}','v1','v1')", workflowID, "r04-workflow-"+workflowID.String(), connectionID); err != nil {
		t.Fatal(err)
	}
	bindings := &mutablePreflightBindingReader{binding: workflowbinding.ProjectWorkflowBinding{ID: uuid.New(), ProjectID: f.project, Stage: workflowbinding.StageChapterPlanning, WorkflowConfigurationID: workflowID, Version: 4}}
	workflows := &mutablePreflightWorkflowReader{workflow: globalconfig.Workflow{Common: globalconfig.Common{ID: workflowID, Enabled: true, Version: 5}, ConnectionID: connectionID, WorkflowType: "n8n", ApplicableStages: []string{"chapter_planning"}}}
	connections := &mutablePreflightConnectionReader{connection: globalconfig.Connection{Common: globalconfig.Common{ID: connectionID, Enabled: true, Version: 6}}}
	runtime := workflowrun.NewService(workflowrun.NewPostgresRepository(db), project.NewPostgresRepository(db), bindings, workflows, connections)
	service, err := NewPostgresService(project.NewPostgresRepository(db), db, "test-hmac-secret-1234567890")
	if err != nil {
		t.Fatal(err)
	}
	service.ConfigureChapterPlanningRuntime(bindings, workflows, connections, runtime)
	for _, tc := range []struct {
		name, want string
		mutate     func(*NormalizedCandidate)
	}{
		{"no_change", "no_change", func(c *NormalizedCandidate) {
			c.StorylineRefs[0], c.StorylineRefs[1] = c.StorylineRefs[1], c.StorylineRefs[0]
			c.MaterialRefs[0], c.MaterialRefs[1] = c.MaterialRefs[1], c.MaterialRefs[0]
			c.ForeshadowingRefs[0], c.ForeshadowingRefs[1] = c.ForeshadowingRefs[1], c.ForeshadowingRefs[0]
			c.StorylineRefs = append(c.StorylineRefs, c.StorylineRefs[0])
			c.MaterialRefs = append(c.MaterialRefs, c.MaterialRefs[0])
			c.ForeshadowingRefs = append(c.ForeshadowingRefs, c.ForeshadowingRefs[0])
		}},
		{"replace", "replace", func(c *NormalizedCandidate) { c.Summary = "changed" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preflight, err := service.Preflight(ctx, f.project, postgresPreflightRequest())
			if err != nil || !preflight.Passed {
				t.Fatalf("Preflight result=%+v err=%v", preflight, err)
			}
			if len(preflight.Snapshot.BaseChapterPlans) != 1 || preflight.Snapshot.BaseChapterPlans[0].ID != planID || preflight.Snapshot.BaseChapterPlans[0].RevisionID == nil || *preflight.Snapshot.BaseChapterPlans[0].RevisionID != revisionID || preflight.Snapshot.BaseChapterPlans[0].Version != 1 || !isSameSnapshot(preflight.Snapshot.BaseChapterPlans[0].Snapshot, baseSnapshot) {
				t.Fatalf("base snapshot was not the current revision: %+v", preflight.Snapshot.BaseChapterPlans)
			}
			run, err := service.CreateChapterPlanningRun(ctx, f.project, "preflight-postgres-test", preflight.Token, "r04-real-chain-"+tc.name)
			if err != nil {
				t.Fatal(err)
			}
			var persisted struct {
				GenerationContext       GenerationContextSnapshot `json:"generationContext"`
				GenerationContextDigest string                    `json:"generationContextDigest"`
			}
			if err = db.QueryRow(ctx, "SELECT input_payload FROM workflow_run_records WHERE id=$1", run.ID).Scan(&run.InputPayload); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(run.InputPayload, &persisted); err != nil {
				t.Fatal(err)
			}
			if digest, err := digestGenerationContext(persisted.GenerationContext); err != nil || digest != persisted.GenerationContextDigest || digest != persisted.GenerationContext.InputDigest {
				t.Fatalf("persisted input digest=%q recomputed=%q err=%v", persisted.GenerationContext.InputDigest, digest, err)
			}
			candidate := base
			tc.mutate(&candidate)
			output := NormalizedChapterPlanOutput{ProjectID: f.project, GenerationMode: "full", Target: preflight.Target, SourceWorkflowRunID: run.ID, Candidates: []NormalizedCandidate{candidate, NormalizedCandidate{ChapterNo: 2, Title: "Two", Summary: "two", ChapterPurpose: "transition", StorylineRefs: base.StorylineRefs}}, Metadata: OutputMetadata{InputDigest: persisted.GenerationContext.InputDigest, GeneratedAt: time.Now().UTC().Format(time.RFC3339), SafeProviderSummary: "safe"}}
			raw, _ := json.Marshal(output)
			if _, err := db.Exec(ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload=$2,started_at=NOW(),finished_at=NOW() WHERE id=$1", run.ID, raw); err != nil {
				t.Fatal(err)
			}
			stored, err := workflowrun.NewPostgresRepository(db).GetByID(ctx, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if err = NewRuntimeConsumer(NewResultIngestor(db), NewConsumptionRepository(db)).ConsumeSucceededRun(ctx, stored); err != nil {
				t.Fatal(err)
			}
			var got, newType string
			if err = db.QueryRow(ctx, "SELECT diff_type FROM chapter_plan_candidates WHERE batch_id=(SELECT candidate_batch_id FROM chapter_plan_result_consumptions WHERE workflow_run_id=$1) AND chapter_no=1", run.ID).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("diff=%s want=%s", got, tc.want)
			}
			if err = db.QueryRow(ctx, "SELECT diff_type FROM chapter_plan_candidates WHERE batch_id=(SELECT candidate_batch_id FROM chapter_plan_result_consumptions WHERE workflow_run_id=$1) AND chapter_no=2", run.ID).Scan(&newType); err != nil || newType != "new" {
				t.Fatalf("new diff=%s err=%v", newType, err)
			}
		})
	}
}

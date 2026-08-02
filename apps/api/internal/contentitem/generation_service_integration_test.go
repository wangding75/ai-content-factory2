package contentitem

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type generationRunSpy struct {
	run                                                      workflowrun.WorkflowRun
	getCalls, createRunCalls, runtimeDispatchCalls, n8nCalls int
	events                                                   []workflowrun.Event
	mu                                                       sync.Mutex
}

func (s *generationRunSpy) CreateRunIdempotentForScope(context.Context, string, uuid.UUID, string, string, workflowrun.CreateRunPreparation) (workflowrun.WorkflowRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createRunCalls++
	return workflowrun.WorkflowRun{}, errors.New("unexpected create")
}
func (s *generationRunSpy) CreateRunForPreflightToken(context.Context, uuid.UUID, string, string, workflowrun.CreateRunPreparation) (workflowrun.WorkflowRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createRunCalls++
	return workflowrun.WorkflowRun{}, errors.New("unexpected create")
}
func (s *generationRunSpy) CreateRunForPreflightTokenIdempotent(context.Context, uuid.UUID, string, string, string, workflowrun.CreateRunTxPreparation) (workflowrun.WorkflowRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createRunCalls++
	return workflowrun.WorkflowRun{}, errors.New("unexpected create")
}
func (s *generationRunSpy) ListRuns(context.Context, workflowrun.ListRunsQuery) (workflowrun.RunList, error) {
	return workflowrun.RunList{}, nil
}
func (s *generationRunSpy) ListRunEvents(context.Context, uuid.UUID) ([]workflowrun.Event, error) {
	return nil, nil
}
func (s *generationRunSpy) GetRun(context.Context, uuid.UUID) (workflowrun.WorkflowRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCalls++
	return s.run, nil
}
func (s *generationRunSpy) AddEvent(_ context.Context, event workflowrun.Event) (workflowrun.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return event, nil
}
func (s *generationRunSpy) counts() (int, int, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getCalls, s.createRunCalls, s.runtimeDispatchCalls, s.n8nCalls
}

func generationFixture(t *testing.T) (*PostgresRepository, context.Context, fx, CreateResult, *generationRunSpy) {
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	repo := NewPostgresRepository(db)
	item := create(t, ctx, repo, f)
	connection, config, runID := uuid.New(), uuid.New(), uuid.New()
	_, e := db.Exec(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,integration_status,enabled,last_verified_version) VALUES($1,$2,'n8n','http://fixture','api_key',5,'{}','verified',true,1)", connection, "fixture-"+connection.String())
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version,integration_status,enabled,last_verified_version) VALUES($1,$2,$3,'[\"content_generation\"]','{}','v1','v1','verified',true,1)", config, "fixture-"+config.String(), connection)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { cleanupGenerationFixture(t, db, f, config, connection) })
	run := insertSucceededGenerationRun(t, ctx, db, item, config, runID)
	return repo, ctx, f, item, &generationRunSpy{run: run}
}

func generationCreateFixture(t *testing.T) (*PostgresRepository, context.Context, fx, CreateResult, *GenerationService, *workflowrun.Service, *generationRunSpy) {
	t.Helper()
	repo, ctx, f, item, spy := generationFixture(t)
	bindingID := uuid.New()
	if _, e := repo.db.Exec(ctx, "INSERT INTO project_workflow_bindings(id,project_id,stage,workflow_configuration_id) VALUES($1,$2,'content_generation',$3)", bindingID, item.Detail.Item.ProjectID, spy.run.WorkflowConfigurationID); e != nil {
		t.Fatal(e)
	}
	configs, e := globalconfig.NewService(repo.db, "generation-test-key")
	if e != nil {
		t.Fatal(e)
	}
	runs := workflowrun.NewService(workflowrun.NewPostgresRepository(repo.db), project.NewPostgresRepository(repo.db), workflowbinding.NewPostgresRepository(repo.db), configs, configs)
	svc := NewGenerationService(repo, workflowbinding.NewPostgresRepository(repo.db), configs, runs, "generation-test-secret")
	return repo, ctx, f, item, svc, runs, spy
}

func generationPreflight(t *testing.T, ctx context.Context, svc *GenerationService, item CreateResult) GenerationPreflightResult {
	t.Helper()
	result, e := svc.Preflight(ctx, item.Detail.Item.ID, GenerationPreflightRequest{ExpectedCurrentVersionID: item.Detail.CurrentVersion.ID, ExpectedCurrentVersion: item.Detail.CurrentVersion.Version, ContextOptions: GenerationContextOptions{}, ActorID: "actor"})
	if e != nil || !result.Passed || result.Token == "" {
		t.Fatalf("preflight=%+v err=%v", result, e)
	}
	return result
}

func cleanupGenerationFixture(t *testing.T, db *pgxpool.Pool, f fx, config, connection uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Errorf("generation fixture cleanup begin: %v", e)
		return
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "UPDATE content_items AS item SET current_version_id=candidate.source_content_version_id FROM content_versions AS candidate WHERE item.current_version_id=candidate.id AND candidate.source='workflow_generated' AND (item.project_id=$1 OR item.project_id=$2)", f.project, f.other); e == nil {
		_, e = tx.Exec(ctx, "DELETE FROM content_versions WHERE source='workflow_generated' AND content_item_id IN (SELECT id FROM content_items WHERE project_id=$1 OR project_id=$2)", f.project, f.other)
	}
	if e == nil {
		_, e = tx.Exec(ctx, "DELETE FROM projects WHERE id=$1 OR id=$2", f.project, f.other)
	}
	if e == nil {
		_, e = tx.Exec(ctx, "DELETE FROM workflow_configurations WHERE id=$1", config)
	}
	if e == nil {
		_, e = tx.Exec(ctx, "DELETE FROM workflow_connections WHERE id=$1", connection)
	}
	if e == nil {
		e = tx.Commit(ctx)
	}
	if e != nil {
		t.Errorf("generation fixture cleanup: %v", e)
	}
}

func insertSucceededGenerationRun(t *testing.T, ctx context.Context, db *pgxpool.Pool, item CreateResult, config, runID uuid.UUID) workflowrun.WorkflowRun {
	t.Helper()
	kind := "content_item"
	now := time.Now().UTC()
	run := workflowrun.WorkflowRun{ID: runID, RunNumber: "WR-" + runID.String()[:8], ProjectID: item.Detail.Item.ProjectID, Stage: "content_generation", WorkflowConfigurationID: config, TriggerSource: "manual", Status: workflowrun.StatusSucceeded, SubjectType: &kind, SubjectID: &item.Detail.Item.ID, ConfigurationSnapshot: []byte(`{}`), InputPayload: []byte(`{"sourceContentVersionId":"` + item.Detail.CurrentVersion.ID.String() + `","sourceContentVersionVersion":` + strconv.Itoa(item.Detail.CurrentVersion.Version) + `}`), OutputPayload: []byte(`{"title":"candidate","content":"generated text","summary":"generated summary","wordCount":14}`), StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2}
	_, e := db.Exec(ctx, "INSERT INTO workflow_run_records(id,run_number,project_id,stage,subject_type,subject_id,workflow_configuration_id,trigger_source,status,configuration_snapshot,input_payload,output_payload,started_at,finished_at,version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)", run.ID, run.RunNumber, run.ProjectID, run.Stage, kind, item.Detail.Item.ID, config, run.TriggerSource, run.Status, run.ConfigurationSnapshot, run.InputPayload, run.OutputPayload, now, now, run.Version)
	if e != nil {
		t.Fatal(e)
	}
	return run
}
func insertGenerationEvent(t *testing.T, ctx context.Context, db *pgxpool.Pool, runID uuid.UUID, eventType string) {
	t.Helper()
	if _, e := db.Exec(ctx, "INSERT INTO workflow_run_events(id,run_id,event_type,status,payload,created_at) VALUES($1,$2,$3,'succeeded','{}',NOW())", uuid.New(), runID, eventType); e != nil {
		t.Fatal(e)
	}
}

func generationCandidate(t *testing.T, ctx context.Context, repo *PostgresRepository, item CreateResult, runID uuid.UUID, versionNo int) ContentVersion {
	t.Helper()
	tx, e := repo.db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	candidate := ContentVersion{ID: uuid.New(), ContentItemID: item.Detail.Item.ID, VersionNo: versionNo, SourceContentVersionID: &item.Detail.CurrentVersion.ID, SourceContentVersionVersion: &item.Detail.CurrentVersion.Version, SourceWorkflowRunID: &runID, Title: "candidate-" + runID.String()[:8], Content: "generated text", WordCount: 2, Source: ContentVersionSourceWorkflowGenerated, Status: ContentVersionStatusEditableDraft, Version: 1}
	out, e := repo.CreateContentVersion(ctx, tx, candidate)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	return out
}

func concurrent[T any](t *testing.T, call func(int) (T, error)) ([]T, []error) {
	t.Helper()
	start := make(chan struct{})
	var wg sync.WaitGroup
	out := make([]T, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); <-start; out[i], errs[i] = call(i) }(i)
	}
	close(start)
	wg.Wait()
	return out, errs
}

func TestGenerationCreateRunAtomicSuccessReplayAndTokenConsumption(t *testing.T) {
	repo, ctx, _, item, svc, runs, _ := generationCreateFixture(t)
	preflight := generationPreflight(t, ctx, svc, item)
	created, e := svc.CreateRun(ctx, item.Detail.Item.ID, "actor", preflight.Token, "create-success")
	if e != nil {
		t.Fatal(e)
	}
	if created.Status != workflowrun.StatusQueued || created.Stage != "content_generation" || created.SubjectType == nil || *created.SubjectType != "content_item" || created.SubjectID == nil || *created.SubjectID != item.Detail.Item.ID || !json.Valid(created.ConfigurationSnapshot) {
		t.Fatalf("created=%+v", created)
	}
	events, e := runs.ListRunEvents(ctx, created.ID)
	if e != nil || len(events) != 1 || events[0].EventType != "queued" {
		t.Fatalf("events=%+v err=%v", events, e)
	}
	replay, e := svc.CreateRun(ctx, item.Detail.Item.ID, "actor", preflight.Token, "create-success")
	if e != nil || replay.ID != created.ID {
		t.Fatalf("replay=%+v err=%v", replay, e)
	}
	if _, e = svc.CreateRun(ctx, item.Detail.Item.ID, "other", preflight.Token, "create-success"); !errors.Is(e, workflowrun.ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict=%v", e)
	}
	if _, e = svc.CreateRun(ctx, item.Detail.Item.ID, "actor", preflight.Token, "create-other-key"); !errors.Is(e, ErrGenerationTokenConsumed) {
		t.Fatalf("token reuse=%v", e)
	}
	if count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_records WHERE subject_id=$1 AND status='queued'", item.Detail.Item.ID) != 1 {
		t.Fatal("unexpected queued run count")
	}
}

func TestGenerationCreateRunDriftRollsBackAllWrites(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(context.Context, *PostgresRepository, CreateResult, *generationRunSpy) error
	}{
		{name: "current version id", mutate: func(ctx context.Context, repo *PostgresRepository, item CreateResult, _ *generationRunSpy) error {
			versionID := uuid.New()
			if _, e := repo.db.Exec(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,word_count,source,status) VALUES($1,$2,2,'changed','changed',7,'manual_created','editable_draft')", versionID, item.Detail.Item.ID); e != nil {
				return e
			}
			_, e := repo.db.Exec(ctx, "UPDATE content_items SET current_version_id=$1,version=version+1 WHERE id=$2", versionID, item.Detail.Item.ID)
			return e
		}},
		{name: "current version version", mutate: func(ctx context.Context, repo *PostgresRepository, item CreateResult, _ *generationRunSpy) error {
			_, e := repo.db.Exec(ctx, "UPDATE content_versions SET version=version+1 WHERE id=$1", item.Detail.CurrentVersion.ID)
			return e
		}},
		{name: "generation context", mutate: func(ctx context.Context, repo *PostgresRepository, item CreateResult, _ *generationRunSpy) error {
			_, e := repo.db.Exec(ctx, "UPDATE chapter_plans SET chapter_goal='changed' WHERE id=$1", item.Detail.Item.ChapterPlanID)
			return e
		}},
		{name: "binding", mutate: func(ctx context.Context, repo *PostgresRepository, item CreateResult, _ *generationRunSpy) error {
			_, e := repo.db.Exec(ctx, "UPDATE project_workflow_bindings SET version=version+1 WHERE project_id=$1 AND stage='content_generation'", item.Detail.Item.ProjectID)
			return e
		}},
		{name: "configuration", mutate: func(ctx context.Context, repo *PostgresRepository, _ CreateResult, spy *generationRunSpy) error {
			_, e := repo.db.Exec(ctx, "UPDATE workflow_configurations SET integration_status='stale',version=version+1 WHERE id=$1", spy.run.WorkflowConfigurationID)
			return e
		}},
		{name: "connection", mutate: func(ctx context.Context, repo *PostgresRepository, _ CreateResult, spy *generationRunSpy) error {
			_, e := repo.db.Exec(ctx, "UPDATE workflow_connections SET integration_status='stale',version=version+1 WHERE id=(SELECT connection_id FROM workflow_configurations WHERE id=$1)", spy.run.WorkflowConfigurationID)
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, ctx, _, item, svc, runs, spy := generationCreateFixture(t)
			preflight := generationPreflight(t, ctx, svc, item)
			beforeRuns := count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_records WHERE project_id=$1", item.Detail.Item.ProjectID)
			beforeEvents := count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_events e JOIN workflow_run_records r ON r.id=e.run_id WHERE r.project_id=$1", item.Detail.Item.ProjectID)
			if e := tc.mutate(ctx, repo, item, spy); e != nil {
				t.Fatal(e)
			}
			_, e := svc.CreateRun(ctx, item.Detail.Item.ID, "actor", preflight.Token, "drift-"+tc.name)
			if !errors.Is(e, ErrGenerationInputChanged) {
				t.Fatalf("error=%v", e)
			}
			if got := count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_records WHERE project_id=$1", item.Detail.Item.ProjectID); got != beforeRuns {
				t.Fatalf("runs=%d before=%d", got, beforeRuns)
			}
			if got := count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_events e JOIN workflow_run_records r ON r.id=e.run_id WHERE r.project_id=$1", item.Detail.Item.ProjectID); got != beforeEvents {
				t.Fatalf("events=%d before=%d", got, beforeEvents)
			}
			claims, e := chapterplan.VerifyPreflightToken([]byte("generation-test-secret"), preflight.Token, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			used, e := workflowrun.NewPostgresRepository(repo.db).PreflightTokenUsed(ctx, claims.Nonce)
			if e != nil || used {
				t.Fatalf("token used=%v err=%v", used, e)
			}
			if count(t, ctx, repo.db, "SELECT count(*) FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2", "createContentGenerationRun:"+item.Detail.Item.ProjectID.String(), "drift-"+tc.name) != 0 {
				t.Fatal("success idempotency record persisted")
			}
			_ = runs
		})
	}
}

func TestGenerationCreateRunConcurrentActiveGuard(t *testing.T) {
	repo, ctx, _, item, svc, _, _ := generationCreateFixture(t)
	first := generationPreflight(t, ctx, svc, item)
	second := generationPreflight(t, ctx, svc, item)
	tokens := []string{first.Token, second.Token}
	_, errs := concurrent(t, func(i int) (workflowrun.WorkflowRun, error) {
		return svc.CreateRun(context.Background(), item.Detail.Item.ID, "actor", tokens[i], "concurrent-"+string(rune('a'+i)))
	})
	success := 0
	for _, e := range errs {
		if e == nil {
			success++
		}
	}
	if success != 1 || count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_records WHERE subject_id=$1 AND status IN ('queued','running')", item.Detail.Item.ID) != 1 {
		t.Fatalf("success=%d errors=%v", success, errs)
	}
}

func TestGenerationRetryConsumptionConcurrent(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	insertGenerationEvent(t, ctx, repo.db, spy.run.ID, workflowrun.EventTypeResultConsumptionFailed)
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	out, errs := concurrent(t, func(i int) (ContentGenerationResult, error) {
		return svc.RetryConsumption(ctx, spy.run.ID, RetryConsumptionRequest{ExpectedRunVersion: 2, IdempotencyKey: "retry-concurrent-" + string(rune('a'+i))})
	})
	success := 0
	var candidateID uuid.UUID
	for i := range out {
		if errs[i] == nil {
			success++
			if candidateID == uuid.Nil {
				candidateID = out[i].CandidateVersion.ID
			} else if candidateID != out[i].CandidateVersion.ID {
				t.Fatalf("two successful candidates: %s and %s", candidateID, out[i].CandidateVersion.ID)
			}
		} else if !errors.Is(errs[i], workflowrun.ErrVersionConflict) {
			t.Fatalf("unexpected concurrent retry error: %v", errs[i])
		}
	}
	if success < 1 || count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE content_item_id=$1 AND source_workflow_run_id=$2", item.Detail.Item.ID, spy.run.ID) != 1 {
		t.Fatalf("success=%d candidateCount=%d", success, count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE content_item_id=$1 AND source_workflow_run_id=$2", item.Detail.Item.ID, spy.run.ID))
	}
	var current uuid.UUID
	if e := repo.db.QueryRow(ctx, "SELECT current_version_id FROM content_items WHERE id=$1", item.Detail.Item.ID).Scan(&current); e != nil || current != item.Detail.CurrentVersion.ID {
		t.Fatalf("current=%s err=%v", current, e)
	}
	if count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", spy.run.ID) != 1 {
		t.Fatal("contradictory result-consumed events")
	}
}

func TestGenerationRetryConsumptionIdempotencyConflict(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	insertGenerationEvent(t, ctx, repo.db, spy.run.ID, workflowrun.EventTypeResultConsumptionFailed)
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	first, e := svc.RetryConsumption(ctx, spy.run.ID, RetryConsumptionRequest{ExpectedRunVersion: 2, IdempotencyKey: "retry-conflict"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = svc.RetryConsumption(ctx, spy.run.ID, RetryConsumptionRequest{ExpectedRunVersion: 3, IdempotencyKey: "retry-conflict"})
	if !errors.Is(e, workflowrun.ErrIdempotencyConflict) {
		t.Fatalf("conflict=%v", e)
	}
	if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE content_item_id=$1 AND source_workflow_run_id=$2", item.Detail.Item.ID, spy.run.ID) != 1 || first.CandidateVersion.SourceWorkflowRunID == nil {
		t.Fatal("idempotency conflict duplicated consumption")
	}
}

func TestGenerationRetryConsumptionIsolation(t *testing.T) {
	repo, ctx, _, _, spy := generationFixture(t)
	insertGenerationEvent(t, ctx, repo.db, spy.run.ID, workflowrun.EventTypeResultConsumptionFailed)
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	if _, e := svc.RetryConsumption(ctx, spy.run.ID, RetryConsumptionRequest{ExpectedRunVersion: 2, IdempotencyKey: "retry-isolation"}); e != nil {
		t.Fatal(e)
	}
	get, create, runtime, n8n := spy.counts()
	if get != 0 || create != 0 || runtime != 0 || n8n != 0 {
		t.Fatalf("get=%d createRun=%d runtimeDispatch=%d n8n=%d", get, create, runtime, n8n)
	}
}

func TestGenerationResultConsumptionRetryCompletesSameFailedRuntimeRun(t *testing.T) {
	repo, ctx, _, item, svc, runs, spy := generationCreateFixture(t)
	runs.SetContentSucceededConsumer(svc)
	externalID := "external-generation-original"
	if _, e := repo.db.Exec(ctx, "UPDATE workflow_run_records SET status='failed',failure_phase='result_consumption',failure_code='result_consumption_failed',safe_error_message='safe',error_code='result_consumption_failed',error_message='safe',error_details='{}',retryability='result_consumption_retry',external_execution_id=$2 WHERE id=$1", spy.run.ID, externalID); e != nil {
		t.Fatal(e)
	}
	if _, e := repo.db.Exec(ctx, "INSERT INTO workflow_run_result_consumptions(workflow_run_id,status,failure_code,safe_error_message) VALUES($1,'failed','result_consumption_failed','safe')", spy.run.ID); e != nil {
		t.Fatal(e)
	}
	insertGenerationEvent(t, ctx, repo.db, spy.run.ID, workflowrun.EventTypeResultConsumptionFailed)
	result, e := svc.RetryConsumption(ctx, spy.run.ID, RetryConsumptionRequest{ExpectedRunVersion: spy.run.Version, IdempotencyKey: "same-runtime-consume"})
	if e != nil {
		t.Fatal(e)
	}
	if result.WorkflowRun.ID != spy.run.ID || result.WorkflowRun.Status != workflowrun.StatusSucceeded || result.WorkflowRun.Version != spy.run.Version+1 || result.WorkflowRun.ExternalExecutionID == nil || *result.WorkflowRun.ExternalExecutionID != externalID {
		t.Fatalf("run=%+v", result.WorkflowRun)
	}
	if count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_records WHERE id=$1", spy.run.ID) != 1 || count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", spy.run.ID) != 1 || count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='succeeded'", spy.run.ID) != 1 {
		t.Fatal("same-run consumption was not unique")
	}
	if item.Detail.Item.CurrentVersionID != item.Detail.CurrentVersion.ID {
		t.Fatal("candidate unexpectedly became current")
	}
}

func TestGenerationRetryConsumptionEligibilityMatrix(t *testing.T) {
	for _, tc := range []struct {
		name, eventType string
		mutate          func(context.Context, *pgxpool.Pool, *generationRunSpy) error
		want            error
	}{
		{name: "missing failure event", want: ErrRunNotConsumable},
		{name: "output validation failed", eventType: workflowrun.EventTypeOutputValidationFailed, want: ErrRunNotConsumable},
		{name: "run not succeeded", eventType: workflowrun.EventTypeResultConsumptionFailed, mutate: func(ctx context.Context, db *pgxpool.Pool, spy *generationRunSpy) error {
			_, e := db.Exec(ctx, "UPDATE workflow_run_records SET status='running',output_payload=NULL,finished_at=NULL,cancelled_at=NULL WHERE id=$1", spy.run.ID)
			return e
		}, want: ErrRunNotConsumable},
		{name: "wrong stage", eventType: workflowrun.EventTypeResultConsumptionFailed, mutate: func(ctx context.Context, db *pgxpool.Pool, spy *generationRunSpy) error {
			_, e := db.Exec(ctx, "UPDATE workflow_run_records SET stage='review' WHERE id=$1", spy.run.ID)
			return e
		}, want: ErrRunNotConsumable},
		{name: "invalid persisted output", eventType: workflowrun.EventTypeResultConsumptionFailed, mutate: func(ctx context.Context, db *pgxpool.Pool, spy *generationRunSpy) error {
			_, e := db.Exec(ctx, "UPDATE workflow_run_records SET output_payload='{}' WHERE id=$1", spy.run.ID)
			return e
		}, want: ErrValidation},
		{name: "eligible", eventType: workflowrun.EventTypeResultConsumptionFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, ctx, _, _, spy := generationFixture(t)
			if tc.eventType != "" {
				insertGenerationEvent(t, ctx, repo.db, spy.run.ID, tc.eventType)
			}
			if tc.mutate != nil {
				if e := tc.mutate(ctx, repo.db, spy); e != nil {
					t.Fatal(e)
				}
			}
			svc := NewGenerationService(repo, nil, nil, spy, "x")
			out, e := svc.RetryConsumption(ctx, spy.run.ID, RetryConsumptionRequest{ExpectedRunVersion: 2, IdempotencyKey: "eligibility"})
			if tc.want != nil {
				if !errors.Is(e, tc.want) {
					t.Fatalf("error=%v want=%v", e, tc.want)
				}
				if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", spy.run.ID) != 0 {
					t.Fatal("candidate created")
				}
				return
			}
			if e != nil || out.CandidateVersion.ID == uuid.Nil {
				t.Fatalf("out=%+v err=%v", out, e)
			}
			if count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", spy.run.ID) != 1 {
				t.Fatal("missing result_consumed")
			}
		})
	}
}

func TestGenerationRetryConsumptionReturnsExistingCandidate(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	candidate := generationCandidate(t, ctx, repo, item, spy.run.ID, 2)
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	out, e := svc.RetryConsumption(ctx, spy.run.ID, RetryConsumptionRequest{ExpectedRunVersion: 2, IdempotencyKey: "existing"})
	if e != nil || out.CandidateVersion.ID != candidate.ID {
		t.Fatalf("out=%+v err=%v", out, e)
	}
	if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", spy.run.ID) != 1 || count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", spy.run.ID) != 0 {
		t.Fatal("existing candidate was consumed again")
	}
}

func TestGenerationSetCurrentConcurrentCAS(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	candidateA := generationCandidate(t, ctx, repo, item, spy.run.ID, 2)
	runB := insertSucceededGenerationRun(t, ctx, repo.db, item, spy.run.WorkflowConfigurationID, uuid.New())
	candidateB := generationCandidate(t, ctx, repo, item, runB.ID, 3)
	out, errs := concurrent(t, func(i int) (Detail, error) {
		candidate := candidateA.ID
		if i == 1 {
			candidate = candidateB.ID
		}
		return svc.SetCurrent(ctx, item.Detail.Item.ID, SetCurrentRequest{CandidateVersionID: candidate, ExpectedCurrentVersionID: item.Detail.CurrentVersion.ID, ExpectedCurrentVersion: item.Detail.CurrentVersion.Version, IdempotencyKey: "set-concurrent-" + string(rune('a'+i))})
	})
	success := 0
	var winner uuid.UUID
	for i := range out {
		if errs[i] == nil {
			success++
			winner = out[i].CurrentVersion.ID
		} else if !errors.Is(errs[i], ErrVersionConflict) && !errors.Is(errs[i], ErrCandidateStale) {
			t.Fatalf("unexpected concurrent set error: %v", errs[i])
		}
	}
	if success != 1 || winner == uuid.Nil {
		t.Fatalf("success=%d winner=%s", success, winner)
	}
	detail, e := repo.GetByID(ctx, item.Detail.Item.ID)
	if e != nil || detail.Item.Version != item.Detail.Item.Version+1 || detail.Item.CurrentVersionID != winner || detail.Item.ReviewedAt != nil || detail.Item.Status != "draft" {
		t.Fatalf("detail=%+v err=%v", detail, e)
	}
	if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE content_item_id=$1", item.Detail.Item.ID) != 3 {
		t.Fatal("candidate or history disappeared")
	}
}

func TestGenerationSetCurrentRowsAffectedZero(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	candidateA := generationCandidate(t, ctx, repo, item, spy.run.ID, 2)
	runB := insertSucceededGenerationRun(t, ctx, repo.db, item, spy.run.WorkflowConfigurationID, uuid.New())
	candidateB := generationCandidate(t, ctx, repo, item, runB.ID, 3)
	tx, e := repo.db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	tag, e := tx.Exec(ctx, "UPDATE content_items SET current_version_id=$1,version=version+1 WHERE id=$2 AND current_version_id=$3 AND version=$4", candidateA.ID, item.Detail.Item.ID, item.Detail.CurrentVersion.ID, item.Detail.Item.Version)
	if e != nil || tag.RowsAffected() != 1 {
		t.Fatalf("winner tag=%d err=%v", tag.RowsAffected(), e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	tx, e = repo.db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	tag, e = tx.Exec(ctx, "UPDATE content_items SET current_version_id=$1,version=version+1 WHERE id=$2 AND current_version_id=$3 AND version=$4", candidateB.ID, item.Detail.Item.ID, item.Detail.CurrentVersion.ID, item.Detail.Item.Version)
	if e != nil || tag.RowsAffected() != 0 {
		t.Fatalf("expected zero rows, tag=%d err=%v", tag.RowsAffected(), e)
	}
	if e = setCurrentContentItemPointer(ctx, tx, item.Detail.Item.ID, candidateB.ID, item.Detail.CurrentVersion.ID, item.Detail.Item.Version); !errors.Is(e, ErrVersionConflict) {
		t.Fatalf("zero-row CAS converted to %v", e)
	}
	if e = tx.Rollback(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestGenerationSetCurrentIdempotencyConflict(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	candidateA := generationCandidate(t, ctx, repo, item, spy.run.ID, 2)
	runB := insertSucceededGenerationRun(t, ctx, repo.db, item, spy.run.WorkflowConfigurationID, uuid.New())
	candidateB := generationCandidate(t, ctx, repo, item, runB.ID, 3)
	first, e := svc.SetCurrent(ctx, item.Detail.Item.ID, SetCurrentRequest{CandidateVersionID: candidateA.ID, ExpectedCurrentVersionID: item.Detail.CurrentVersion.ID, ExpectedCurrentVersion: item.Detail.CurrentVersion.Version, IdempotencyKey: "set-conflict"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = svc.SetCurrent(ctx, item.Detail.Item.ID, SetCurrentRequest{CandidateVersionID: candidateB.ID, ExpectedCurrentVersionID: item.Detail.CurrentVersion.ID, ExpectedCurrentVersion: item.Detail.CurrentVersion.Version, IdempotencyKey: "set-conflict"})
	if !errors.Is(e, workflowrun.ErrIdempotencyConflict) {
		t.Fatalf("conflict=%v", e)
	}
	detail, e := repo.GetByID(ctx, item.Detail.Item.ID)
	if e != nil || detail.Item.CurrentVersionID != first.CurrentVersion.ID || detail.Item.Version != item.Detail.Item.Version+1 {
		t.Fatalf("detail=%+v err=%v", detail, e)
	}
}

func TestGenerationStaleCandidateAllowsRegenerationWithNextVersionNumber(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	stale := generationCandidate(t, ctx, repo, item, spy.run.ID, 2)
	if _, e := repo.db.Exec(ctx, "UPDATE content_versions SET version=version+1 WHERE id=$1", item.Detail.CurrentVersion.ID); e != nil {
		t.Fatal(e)
	}
	latest, e := repo.GetByID(ctx, item.Detail.Item.ID)
	if e != nil {
		t.Fatal(e)
	}
	runs := &generationSummaryRuns{list: workflowrun.RunList{Items: []workflowrun.WorkflowRun{spy.run}}}
	svc := NewGenerationService(repo, generationSummaryBindings{err: workflowbinding.ErrNotFound}, generationSummaryConfigs{}, runs, "x")
	summary, e := svc.Summary(ctx, item.Detail.Item.ID)
	if e != nil || summary.State != "candidate_ready" || summary.CandidateCanBecomeCurrent || !summary.CanGenerate || summary.LatestCandidate == nil || summary.LatestCandidate.ID != stale.ID {
		t.Fatalf("summary=%+v err=%v", summary, e)
	}
	if _, e = svc.SetCurrent(ctx, item.Detail.Item.ID, SetCurrentRequest{CandidateVersionID: stale.ID, ExpectedCurrentVersionID: latest.CurrentVersion.ID, ExpectedCurrentVersion: latest.CurrentVersion.Version, IdempotencyKey: "stale"}); !errors.Is(e, ErrCandidateStale) {
		t.Fatalf("set stale=%v", e)
	}
	latestItem := item
	latestItem.Detail = latest
	runB := insertSucceededGenerationRun(t, ctx, repo.db, latestItem, spy.run.WorkflowConfigurationID, uuid.New())
	if e = svc.ConsumeSucceededRun(ctx, runB); e != nil {
		t.Fatal(e)
	}
	candidate, e := repo.GetContentVersionBySourceWorkflowRunID(ctx, runB.ID)
	if e != nil || candidate.VersionNo != 3 {
		t.Fatalf("candidate=%+v err=%v", candidate, e)
	}
}

func TestGenerationSetCurrentIdempotencyReplayReturnsFirstResult(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	candidateA := generationCandidate(t, ctx, repo, item, spy.run.ID, 2)
	first, e := svc.SetCurrent(ctx, item.Detail.Item.ID, SetCurrentRequest{CandidateVersionID: candidateA.ID, ExpectedCurrentVersionID: item.Detail.CurrentVersion.ID, ExpectedCurrentVersion: item.Detail.CurrentVersion.Version, IdempotencyKey: "key-a"})
	if e != nil {
		t.Fatal(e)
	}
	currentItem := item
	currentItem.Detail = first
	runB := insertSucceededGenerationRun(t, ctx, repo.db, currentItem, spy.run.WorkflowConfigurationID, uuid.New())
	candidateB := generationCandidate(t, ctx, repo, currentItem, runB.ID, 3)
	second, e := svc.SetCurrent(ctx, item.Detail.Item.ID, SetCurrentRequest{CandidateVersionID: candidateB.ID, ExpectedCurrentVersionID: first.CurrentVersion.ID, ExpectedCurrentVersion: first.CurrentVersion.Version, IdempotencyKey: "key-b"})
	if e != nil {
		t.Fatal(e)
	}
	replay, e := svc.SetCurrent(ctx, item.Detail.Item.ID, SetCurrentRequest{CandidateVersionID: candidateA.ID, ExpectedCurrentVersionID: item.Detail.CurrentVersion.ID, ExpectedCurrentVersion: item.Detail.CurrentVersion.Version, IdempotencyKey: "key-a"})
	if e != nil || replay.CurrentVersion.ID != first.CurrentVersion.ID || replay.CurrentVersion.ID == second.CurrentVersion.ID {
		t.Fatalf("replay=%+v first=%+v second=%+v err=%v", replay, first, second, e)
	}
}

func TestOutputValidationConsumptionFailureLeavesNoCandidate(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	if _, e := repo.db.Exec(ctx, "UPDATE workflow_run_records SET output_payload='{"+`"title":"","content":"bad","wordCount":1`+"}' WHERE id=$1", spy.run.ID); e != nil {
		t.Fatal(e)
	}
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	if e := svc.ConsumeSucceededRun(ctx, spy.run); !errors.Is(e, ErrValidation) {
		t.Fatalf("consume=%v", e)
	}
	if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", spy.run.ID) != 0 {
		t.Fatal("candidate persisted")
	}
	var current uuid.UUID
	if e := repo.db.QueryRow(ctx, "SELECT current_version_id FROM content_items WHERE id=$1", item.Detail.Item.ID).Scan(&current); e != nil || current != item.Detail.CurrentVersion.ID {
		t.Fatalf("current=%s err=%v", current, e)
	}
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.events) != 1 || spy.events[0].EventType != "output_validation_failed" {
		t.Fatalf("events=%+v", spy.events)
	}
}

func TestGenerationConsumptionRejectsChangedSourceVersion(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	if _, e := repo.db.Exec(ctx, "UPDATE workflow_run_records SET input_payload=$1 WHERE id=$2", []byte(`{"sourceContentVersionId":"`+item.Detail.CurrentVersion.ID.String()+`","sourceContentVersionVersion":99}`), spy.run.ID); e != nil {
		t.Fatal(e)
	}
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	if e := svc.ConsumeSucceededRun(ctx, spy.run); !errors.Is(e, ErrGenerationSourceInvalid) {
		t.Fatalf("consume=%v", e)
	}
	if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", spy.run.ID) != 0 {
		t.Fatal("candidate persisted")
	}
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.events) != 1 || spy.events[0].EventType != workflowrun.EventTypeResultConsumptionFailed {
		t.Fatalf("events=%+v", spy.events)
	}
}

func insertGenerationSource(t *testing.T, ctx context.Context, db *pgxpool.Pool, projectID uuid.UUID) (uuid.UUID, uuid.UUID) {
	t.Helper()
	planID, itemID, versionID := uuid.New(), uuid.New(), uuid.New()
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SET CONSTRAINTS ALL DEFERRED"); e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(ctx, "INSERT INTO chapter_plans(id,project_id,chapter_no,title,summary,status,source,created_by,confirmed_at) VALUES($1,$2,99,'source','source','confirmed','mock_generated','test',NOW())", planID, projectID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(ctx, "INSERT INTO content_items(id,project_id,chapter_plan_id,title,status,current_version_id) VALUES($1,$2,$3,'source','draft',$4)", itemID, projectID, planID, versionID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = tx.Exec(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,word_count,source,status) VALUES($1,$2,1,'source','source',6,'manual_created','editable_draft')", versionID, itemID)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	return itemID, versionID
}

func TestGenerationSourceContentVersionValidationMatrix(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source func(*testing.T, context.Context, *pgxpool.Pool, fx, CreateResult) uuid.UUID
	}{
		{name: "not found", source: func(*testing.T, context.Context, *pgxpool.Pool, fx, CreateResult) uuid.UUID { return uuid.New() }},
		{name: "other content item", source: func(t *testing.T, ctx context.Context, db *pgxpool.Pool, _ fx, item CreateResult) uuid.UUID {
			_, id := insertGenerationSource(t, ctx, db, item.Detail.Item.ProjectID)
			return id
		}},
		{name: "other project", source: func(t *testing.T, ctx context.Context, db *pgxpool.Pool, f fx, item CreateResult) uuid.UUID {
			_, id := insertGenerationSource(t, ctx, db, f.other)
			return id
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, ctx, f, item, spy := generationFixture(t)
			sourceID := tc.source(t, ctx, repo.db, f, item)
			if _, e := repo.db.Exec(ctx, "UPDATE workflow_run_records SET input_payload=$1 WHERE id=$2", []byte(`{"sourceContentVersionId":"`+sourceID.String()+`","sourceContentVersionVersion":1}`), spy.run.ID); e != nil {
				t.Fatal(e)
			}
			svc := NewGenerationService(repo, nil, nil, spy, "x")
			if e := svc.ConsumeSucceededRun(ctx, spy.run); !errors.Is(e, ErrGenerationSourceInvalid) {
				t.Fatalf("error=%v", e)
			}
			if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", spy.run.ID) != 0 {
				t.Fatal("candidate created")
			}
		})
	}
}

func TestGenerationSourceContentVersionDatabaseErrorPropagates(t *testing.T) {
	repo, ctx, _, _, spy := generationFixture(t)
	databaseErr := errors.New("source query failed")
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	svc.begin = func(ctx context.Context) (pgx.Tx, error) {
		tx, e := repo.db.Begin(ctx)
		if e != nil {
			return nil, e
		}
		return &generationFailureTx{Tx: tx, sourceErr: databaseErr}, nil
	}
	e := svc.ConsumeSucceededRun(ctx, spy.run)
	if !errors.Is(e, databaseErr) || errors.Is(e, ErrGenerationSourceInvalid) {
		t.Fatalf("error=%v", e)
	}
	if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", spy.run.ID) != 0 {
		t.Fatal("candidate created")
	}
}

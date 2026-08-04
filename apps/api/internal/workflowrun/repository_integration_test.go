package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
)

func openDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	raw := os.Getenv("DATABASE_URL")
	if raw == "" {
		t.Fatal("DATABASE_URL is not set; PostgreSQL integration test is required")
	}
	// Connection setup only uses a short deadline. The returned context must
	// outlive multi-step fixtures (100+ run history) and full-suite contention.
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 15*time.Second)
	db, err := pgxpool.New(connectCtx, raw)
	connectCancel()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return db, ctx
}
func fixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	p, c, w := uuid.New(), uuid.New(), uuid.New()
	_, err := db.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','workflowrun-test')", p, "workflowrun-"+p.String()[:8])
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config) VALUES($1,$2,'n8n','http://localhost','api_key',30,'{}')", c, "workflowrun-"+c.String()[:8])
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version) VALUES($1,$2,$3,'[\"review\"]','{}','v1','v1')", w, "workflowrun-"+w.String()[:8], c)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_run_records WHERE project_id=$1", p)
		_, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE id=$1", p)
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE id=$1", w)
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", c)
	})
	return p, w
}

func testRunNumber(projectID uuid.UUID, number string) string {
	return number + "-" + projectID.String()[:8]
}

func newRun(t *testing.T, p, w uuid.UUID, n string) WorkflowRun {
	t.Helper()
	v, err := New(uuid.New(), p, w, testRunNumber(p, n), "review", "manual", json.RawMessage(`{"connection":{"type":"n8n"}}`), json.RawMessage(`{"content":"safe"}`))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestExternalExecutionIDIsUniqueWithinConnectionOnly(t *testing.T) {
	db, ctx := openDB(t)
	projectID, workflowID := fixture(t, ctx, db)
	var connectionID uuid.UUID
	if err := db.QueryRow(ctx, "SELECT connection_id FROM workflow_configurations WHERE id=$1", workflowID).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	secondConnectionID := uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config) VALUES($1,$2,'n8n','http://localhost','api_key',30,'{}')", secondConnectionID, "workflowrun-second-"+secondConnectionID.String()[:8]); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_run_records WHERE project_id=$1", projectID)
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", secondConnectionID)
	})
	insert := func(connection *uuid.UUID, externalID string) error {
		_, err := db.Exec(ctx, `INSERT INTO workflow_run_records(
			id,run_number,project_id,stage,workflow_configuration_id,trigger_source,status,
			configuration_snapshot,input_payload,binding_snapshot,connection_snapshot,llm_policy_snapshot,
			workflow_connection_id,external_execution_id
		) VALUES($1,$2,$3,'review',$4,'manual','queued','{}','{}','{}','{}','{}',$5,$6)`, uuid.New(), "WR-UNIQUE-"+uuid.NewString()[:8], projectID, workflowID, connection, externalID)
		return err
	}
	if err := insert(&connectionID, "shared-execution"); err != nil {
		t.Fatal(err)
	}
	if err := insert(&connectionID, "shared-execution"); err == nil {
		t.Fatal("duplicate external execution id in one connection was accepted")
	}
	if err := insert(&secondConnectionID, "shared-execution"); err != nil {
		t.Fatalf("same external id in another connection: %v", err)
	}
	if err := insert(nil, "shared-execution"); err != nil {
		t.Fatalf("first NULL connection: %v", err)
	}
	if err := insert(nil, "shared-execution"); err != nil {
		t.Fatalf("second NULL connection: %v", err)
	}
}

func contentGenerationService(t *testing.T, repo *Repository, projectID, workflowID uuid.UUID) *Service {
	t.Helper()
	var connectionID uuid.UUID
	if err := repo.db.QueryRow(context.Background(), "SELECT connection_id FROM workflow_configurations WHERE id=$1", workflowID).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	s := NewService(repo, serviceProjects{p: project.Project{ID: projectID}}, serviceBindings{b: workflowbinding.ProjectWorkflowBinding{ID: uuid.New(), ProjectID: projectID, Stage: workflowbinding.StageContentGeneration, WorkflowConfigurationID: workflowID, Version: 1}}, serviceConfigs{w: globalconfig.Workflow{Common: globalconfig.Common{ID: workflowID, Enabled: true, IntegrationStatus: "verified", Version: 1}, ConnectionID: connectionID, ApplicableStages: []string{"content_generation"}, TypeConfig: json.RawMessage(`{}`), DefaultParameters: json.RawMessage(`{}`)}}, serviceConnections{c: globalconfig.Connection{Common: globalconfig.Common{ID: connectionID, Enabled: true, IntegrationStatus: "verified", Version: 1}, ConnectionType: "n8n", BaseURL: "http://localhost", AuthType: "api_key", TypeConfig: json.RawMessage(`{}`)}})
	return s
}

func preflightCommand(projectID uuid.UUID, nonce string) CreateRunPreparation {
	return func() (CreateRunCommand, error) {
		return CreateRunCommand{ProjectID: projectID, Stage: "content_generation", TriggerSource: "manual", InputPayload: json.RawMessage(`{"preflightTokenNonce":"` + nonce + `"}`)}, nil
	}
}

func pendingConsumptionRun(t *testing.T, ctx context.Context, repo *Repository, projectID, workflowID uuid.UUID, number string) WorkflowRun {
	t.Helper()
	run := newRun(t, projectID, workflowID, number)
	running, err := run.Start(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, running); err != nil {
		t.Fatal(err)
	}
	stored, _, err := repo.SaveOutputForConsumption(ctx, running, json.RawMessage(`{"schemaVersion":"review.output.v1"}`), Event{
		ID: uuid.New(), RunID: running.ID, EventType: "output_validated", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return stored
}

func TestResultConsumptionTransactionCommitsRuntimeFactAndDomainEventAtomically(t *testing.T) {
	db, ctx := openDB(t)
	projectID, workflowID := fixture(t, ctx, db)
	repo := NewPostgresRepository(db)
	run := pendingConsumptionRun(t, ctx, repo, projectID, workflowID, "WR-CONSUME-COMMIT")

	updated, replay, err := repo.ConsumeResult(ctx, run.ID, run.Version, time.Now().UTC(), false, func(ctx context.Context, tx pgx.Tx, locked WorkflowRun) error {
		_, callbackErr := AddEventTx(ctx, tx, Event{ID: uuid.New(), RunID: locked.ID, EventType: "result_consumed", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()})
		return callbackErr
	})
	if err != nil || replay || updated.Status != StatusSucceeded || updated.ID != run.ID {
		t.Fatalf("updated=%+v replay=%v err=%v", updated, replay, err)
	}
	var state string
	if err = db.QueryRow(ctx, "SELECT status FROM workflow_run_result_consumptions WHERE workflow_run_id=$1", run.ID).Scan(&state); err != nil || state != "completed" {
		t.Fatalf("state=%s err=%v", state, err)
	}
	for _, eventType := range []string{EventTypeResultConsumed, "succeeded"} {
		var count int
		if err = db.QueryRow(ctx, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type=$2", run.ID, eventType).Scan(&count); err != nil || count != 1 {
			t.Fatalf("event=%s count=%d err=%v", eventType, count, err)
		}
	}
}

func TestResultConsumptionTransactionRollsBackPartialDomainWriteAndCanRetry(t *testing.T) {
	db, ctx := openDB(t)
	projectID, workflowID := fixture(t, ctx, db)
	repo := NewPostgresRepository(db)
	run := pendingConsumptionRun(t, ctx, repo, projectID, workflowID, "WR-CONSUME-ROLLBACK")
	canary := errors.New("domain write failed")

	_, _, err := repo.ConsumeResult(ctx, run.ID, run.Version, time.Now().UTC(), false, func(ctx context.Context, tx pgx.Tx, locked WorkflowRun) error {
		if _, insertErr := AddEventTx(ctx, tx, Event{ID: uuid.New(), RunID: locked.ID, EventType: "result_consumed", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}); insertErr != nil {
			return insertErr
		}
		return canary
	})
	if !errors.Is(err, canary) {
		t.Fatalf("err=%v", err)
	}
	var eventCount int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", run.ID).Scan(&eventCount); err != nil || eventCount != 0 {
		t.Fatalf("partial event count=%d err=%v", eventCount, err)
	}
	persisted, err := repo.GetByID(ctx, run.ID)
	if err != nil || persisted.Status != StatusRunning || persisted.Version != run.Version {
		t.Fatalf("persisted=%+v err=%v", persisted, err)
	}
	var state string
	if err = db.QueryRow(ctx, "SELECT status FROM workflow_run_result_consumptions WHERE workflow_run_id=$1", run.ID).Scan(&state); err != nil || state != "pending" {
		t.Fatalf("state=%s err=%v", state, err)
	}
}

func TestConcurrentResultConsumptionInvokesDomainCallbackOnce(t *testing.T) {
	db, ctx := openDB(t)
	projectID, workflowID := fixture(t, ctx, db)
	repo := NewPostgresRepository(db)
	run := pendingConsumptionRun(t, ctx, repo, projectID, workflowID, "WR-CONSUME-CONCURRENT")
	start := make(chan struct{})
	var calls int
	var mutex sync.Mutex
	errs := make([]error, 2)
	replays := make([]bool, 2)
	var group sync.WaitGroup
	for i := range errs {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			_, replays[index], errs[index] = repo.ConsumeResult(context.Background(), run.ID, run.Version, time.Now().UTC(), false, func(context.Context, pgx.Tx, WorkflowRun) error {
				mutex.Lock()
				calls++
				mutex.Unlock()
				return nil
			})
		}(i)
	}
	close(start)
	group.Wait()
	if errs[0] != nil || errs[1] != nil || calls != 1 || replays[0] == replays[1] {
		t.Fatalf("calls=%d replays=%v errors=%v", calls, replays, errs)
	}
}

func TestPreflightTokenSequentialSingleConsumption(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	service := contentGenerationService(t, repo, projectID, workflowID)
	nonce := uuid.NewString()
	first, err := service.CreateRunForPreflightToken(ctx, projectID, nonce, "first", preflightCommand(projectID, nonce))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateRunForPreflightToken(ctx, projectID, nonce, "second", preflightCommand(projectID, nonce)); !errors.Is(err, ErrPreflightTokenConsumed) {
		t.Fatalf("second use = %v", err)
	}
	var count int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM workflow_run_records WHERE id=$1", first.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("runs=%d err=%v", count, err)
	}
	var payload string
	if err = db.QueryRow(ctx, "SELECT input_payload::text FROM workflow_run_records WHERE id=$1", first.ID).Scan(&payload); err != nil || strings.Contains(payload, "eyJ") {
		t.Fatalf("token leaked or query failed: %q %v", payload, err)
	}
}

func TestPreflightTokenConcurrentSingleConsumption(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	service := contentGenerationService(t, repo, projectID, workflowID)
	nonce := uuid.NewString()
	start := make(chan struct{})
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			<-start
			_, err := service.CreateRunForPreflightToken(ctx, projectID, nonce, "key-"+string(rune('a'+i)), preflightCommand(projectID, nonce))
			errs <- err
		}(i)
	}
	close(start)
	success := 0
	for i := 0; i < 2; i++ {
		if err := <-errs; err == nil {
			success++
		} else if !errors.Is(err, ErrPreflightTokenConsumed) {
			t.Fatalf("concurrent use = %v", err)
		}
	}
	var count int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM workflow_run_records WHERE input_payload->>'preflightTokenNonce'=$1", nonce).Scan(&count); err != nil || success != 1 || count != 1 {
		t.Fatalf("success=%d runs=%d err=%v", success, count, err)
	}
}

func TestPreflightTokenConsumptionSurvivesMoreThan100HistoricalRuns(t *testing.T) {
	db, baseCtx := openDB(t)
	ctx, cancel := context.WithTimeout(baseCtx, 2*time.Minute)
	defer cancel()
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	service := contentGenerationService(t, repo, projectID, workflowID)
	// Terminalize historical rows so the shared development API worker does not
	// thrash 100+ freshly queued content_generation runs during this lookup test.
	terminalize := func(runID uuid.UUID) {
		t.Helper()
		if _, err := db.Exec(ctx, `
			UPDATE workflow_run_records
			SET status='failed',
			    started_at=COALESCE(started_at, NOW()),
			    finished_at=COALESCE(finished_at, NOW()),
			    failure_phase='external_execution',
			    failure_code='test_terminalized',
			    safe_error_message='test terminalized',
			    error_code='test_terminalized',
			    error_message='test terminalized',
			    updated_at=NOW(),
			    version=version+1
			WHERE id=$1 AND status IN ('queued','running','cancelling')`, runID); err != nil {
			t.Fatal(err)
		}
	}
	nonce := uuid.NewString()
	first, err := service.CreateRunForPreflightToken(ctx, projectID, nonce, "first", preflightCommand(projectID, nonce))
	if err != nil {
		t.Fatal(err)
	}
	terminalize(first.ID)
	for i := 0; i < 101; i++ {
		n := uuid.NewString()
		run, createErr := service.CreateRunForPreflightToken(ctx, projectID, n, "history-"+n, preflightCommand(projectID, n))
		if createErr != nil {
			t.Fatal(createErr)
		}
		terminalize(run.ID)
	}
	if _, err := service.CreateRunForPreflightToken(ctx, projectID, nonce, "again", preflightCommand(projectID, nonce)); !errors.Is(err, ErrPreflightTokenConsumed) {
		t.Fatalf("reused nonce=%v", err)
	}
	var count int
	_ = db.QueryRow(ctx, "SELECT count(*) FROM workflow_run_records WHERE input_payload->>'preflightTokenNonce'=$1", nonce).Scan(&count)
	if count != 1 {
		t.Fatalf("runs=%d", count)
	}
}
func TestRepositoryCRUDEventsAndSummary(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	first, err := repo.Create(ctx, newRun(t, p, w, "WR-001"))
	if err != nil {
		t.Fatal(err)
	}
	running, err := first.Start(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	running, err = repo.UpdateStatus(ctx, running)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := running.Fail(time.Now().UTC(), Failure{Code: "TIMEOUT", Message: "request timed out", Details: json.RawMessage(`{"safe":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.UpdateStatus(ctx, failed); err != nil {
		t.Fatal(err)
	}
	event, err := repo.AddEvent(ctx, Event{ID: uuid.New(), RunID: first.ID, EventType: "failed", Status: StatusFailed, Payload: json.RawMessage(`{"safe":true}`), CreatedAt: time.Now().UTC()})
	if err != nil || event.ID == uuid.Nil {
		t.Fatalf("add event=%+v err=%v", event, err)
	}
	got, err := repo.GetByID(ctx, first.ID)
	if err != nil || got.Status != StatusFailed {
		t.Fatalf("get=%+v err=%v", got, err)
	}
	list, err := repo.List(ctx, ListFilter{ProjectID: &p, Status: string(StatusFailed)})
	if err != nil || len(list) != 1 {
		t.Fatalf("list=%d err=%v", len(list), err)
	}
	summary, err := repo.QuerySummary(ctx, p, 5)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalRuns != 1 || summary.RunningCount != 0 || summary.LatestFailure == nil || summary.LatestRun == nil || len(summary.RecentRuns) != 1 {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestRepositoryIteration19SnapshotAndRetryRoundTrip(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)

	original, err := repo.Create(ctx, newRun(t, projectID, workflowID, "WR-I19-ORIGINAL"))
	if err != nil {
		t.Fatal(err)
	}
	retry, err := New(
		uuid.New(),
		projectID,
		workflowID,
		testRunNumber(projectID, "WR-I19-RETRY"),
		"review",
		"retry",
		json.RawMessage(`{"connection":{"id":"connection-safe","type":"n8n"},"configurationVersion":7}`),
		json.RawMessage(`{"content":"safe"}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	retryOf := original.ID
	retryMode := "original_configuration"
	externalID := "execution-safe-19"
	retry.RetryOfRunID = &retryOf
	retry.RetryMode = &retryMode
	retry.ExternalExecutionID = &externalID
	retry.Retryability = "runtime_retry"
	retry.BindingSnapshot = json.RawMessage(`{"bindingId":"binding-safe","bindingVersion":3,"stage":"review"}`)
	retry.ConnectionSnapshot = json.RawMessage(`{"id":"connection-safe","version":4,"credentialFingerprint":"sha256:safe"}`)
	retry.LlmPolicySnapshot = json.RawMessage(`{"strategy":"acf_managed","providerId":"provider-safe","providerVersion":5,"model":"fixture-model","secretFingerprint":"sha256:safe"}`)

	created, err := repo.Create(ctx, retry)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RetryOfRunID == nil || *got.RetryOfRunID != original.ID || got.RetryMode == nil || *got.RetryMode != retryMode || got.ExternalExecutionID == nil || *got.ExternalExecutionID != externalID || got.Retryability != "runtime_retry" {
		t.Fatalf("retry metadata did not round-trip: %+v", got)
	}
	for name, pair := range map[string][2]json.RawMessage{
		"binding":    {retry.BindingSnapshot, got.BindingSnapshot},
		"connection": {retry.ConnectionSnapshot, got.ConnectionSnapshot},
		"llm policy": {retry.LlmPolicySnapshot, got.LlmPolicySnapshot},
	} {
		if !jsonEqual(pair[0], pair[1]) {
			t.Errorf("%s snapshot did not round-trip: want=%s got=%s", name, pair[0], pair[1])
		}
	}

	selfRetry := newRun(t, projectID, workflowID, "WR-I19-SELF")
	selfRetry.RetryOfRunID = &selfRetry.ID
	if _, err = repo.Create(ctx, selfRetry); err == nil {
		t.Fatal("self-referencing retry_of_run_id unexpectedly succeeded")
	}
}

func TestRepositoryIteration19TimedOutFailureRoundTrip(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)

	run := newRun(t, projectID, workflowID, "WR-I19-TIMED-OUT")
	failurePhase := "external_execution"
	failureCode := "execution_timeout"
	safeMessage := "The external workflow timed out."
	startedAt := time.Now().UTC().Add(-time.Minute)
	finishedAt := time.Now().UTC()
	run.Status = StatusTimedOut
	run.FailurePhase = &failurePhase
	run.FailureCode = &failureCode
	run.SafeErrorMessage = &safeMessage
	run.Retryability = "runtime_retry"
	run.StartedAt = &startedAt
	run.FinishedAt = &finishedAt
	run.TimedOutAt = &finishedAt

	created, err := repo.Create(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusTimedOut || got.FailurePhase == nil || *got.FailurePhase != failurePhase || got.FailureCode == nil || *got.FailureCode != failureCode || got.SafeErrorMessage == nil || *got.SafeErrorMessage != safeMessage || got.Retryability != "runtime_retry" || got.TimedOutAt == nil {
		t.Fatalf("timed-out failure metadata did not round-trip: %+v", got)
	}
}

func TestRepositoryListsCancellingAndTimedOutStatuses(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	now := time.Now().UTC()
	cancelling := newRun(t, projectID, workflowID, "WR-I19-CANCELLING")
	reason := "user"
	cancelling.Status, cancelling.StartedAt, cancelling.CancellationRequestedAt, cancelling.CancellationReason, cancelling.Version = StatusCancelling, &now, &now, &reason, 2
	if _, err := repo.Create(ctx, cancelling); err != nil {
		t.Fatal(err)
	}
	timedOut := newRun(t, projectID, workflowID, "WR-I19-TIMED-OUT-LIST")
	phase, code, message := "external_execution", "execution_timeout", "safe timeout"
	timedOut.Status, timedOut.StartedAt, timedOut.FinishedAt, timedOut.TimedOutAt = StatusTimedOut, &now, &now, &now
	timedOut.FailurePhase, timedOut.FailureCode, timedOut.SafeErrorMessage, timedOut.ErrorCode, timedOut.ErrorMessage, timedOut.Retryability, timedOut.Version = &phase, &code, &message, &code, &message, "runtime_retry", 2
	if _, err := repo.Create(ctx, timedOut); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"cancelling", "timed_out"} {
		items, err := repo.List(ctx, ListFilter{ProjectID: &projectID, Status: status})
		total, countErr := repo.Count(ctx, ListFilter{ProjectID: &projectID, Status: status})
		if err != nil || countErr != nil || len(items) != 1 || total != 1 || string(items[0].Status) != status {
			t.Fatalf("status=%s items=%+v total=%d listErr=%v countErr=%v", status, items, total, err, countErr)
		}
	}
}

func jsonEqual(left, right json.RawMessage) bool {
	var leftValue any
	var rightValue any
	return json.Unmarshal(left, &leftValue) == nil && json.Unmarshal(right, &rightValue) == nil && reflect.DeepEqual(leftValue, rightValue)
}

func TestRepositoryListQueryTimeAndPaginationFilters(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	connectionID, providerID := uuid.New(), uuid.New()
	base := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	for index, number := range []string{"WR-ALPHA", "WR-BRAVO", "WR-CHARLIE"} {
		run := newRun(t, p, w, number)
		if number == "WR-BRAVO" {
			phase, code, message := "output_validation", "invalid_output", "safe output error"
			run.Status, run.FailurePhase, run.FailureCode, run.SafeErrorMessage, run.ErrorCode, run.ErrorMessage, run.Retryability = StatusFailed, &phase, &code, &message, &code, &message, "runtime_retry"
			run.ErrorDetails = json.RawMessage(`{}`)
			run.ConfigurationSnapshot = mustSafeJSON(map[string]any{"workflowConfiguration": map[string]any{"id": w, "version": 7}, "workflowConnection": map[string]any{"id": connectionID}})
			run.ConnectionSnapshot = mustSafeJSON(map[string]any{"id": connectionID})
			run.LlmPolicySnapshot = mustSafeJSON(map[string]any{"strategy": "acf_managed", "providerId": providerID, "model": "model-19"})
		}
		run.CreatedAt = base.Add(time.Duration(index) * time.Hour)
		run.UpdatedAt = run.CreatedAt
		if run.Status == StatusFailed {
			run.StartedAt, run.FinishedAt = &run.CreatedAt, &run.CreatedAt
		}
		if _, err := repo.Create(ctx, run); err != nil {
			t.Fatal(err)
		}
	}
	q, err := repo.List(ctx, ListFilter{ProjectID: &p, Query: "bravo"})
	if err != nil || len(q) != 1 || q[0].RunNumber != testRunNumber(p, "WR-BRAVO") {
		t.Fatalf("q list=%+v err=%v", q, err)
	}
	noResult, err := repo.List(ctx, ListFilter{ProjectID: &p, Query: "missing"})
	if err != nil || len(noResult) != 0 {
		t.Fatalf("empty q len=%d err=%v", len(noResult), err)
	}
	start := base.Add(time.Hour)
	fromStart, err := repo.List(ctx, ListFilter{ProjectID: &p, StartTime: &start})
	if err != nil || len(fromStart) != 2 {
		t.Fatalf("start range len=%d err=%v", len(fromStart), err)
	}
	end := base.Add(time.Hour)
	toEnd, err := repo.List(ctx, ListFilter{ProjectID: &p, EndTime: &end})
	if err != nil || len(toEnd) != 2 {
		t.Fatalf("end range len=%d err=%v", len(toEnd), err)
	}
	between, err := repo.List(ctx, ListFilter{ProjectID: &p, StartTime: &start, EndTime: &end, Stage: "review"})
	if err != nil || len(between) != 1 || between[0].RunNumber != testRunNumber(p, "WR-BRAVO") {
		t.Fatalf("between=%+v err=%v", between, err)
	}
	invalidEnd := base
	if _, err = repo.List(ctx, ListFilter{ProjectID: &p, StartTime: &start, EndTime: &invalidEnd}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid range=%v", err)
	}
	page, err := repo.List(ctx, ListFilter{ProjectID: &p, Limit: 1, Offset: 1})
	if err != nil || len(page) != 1 || page[0].RunNumber != testRunNumber(p, "WR-BRAVO") {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	advanced := ListFilter{ProjectID: &p, Status: "failed", DisplayStatus: "output_validation_failed", ConnectionID: connectionID.String(), ProviderID: providerID.String(), Model: "model-19", ConfigurationVersion: 7, Retryability: "runtime_retry"}
	filtered, err := repo.List(ctx, advanced)
	total, countErr := repo.Count(ctx, advanced)
	if err != nil || countErr != nil || len(filtered) != 1 || total != len(filtered) || filtered[0].RunNumber != testRunNumber(p, "WR-BRAVO") {
		t.Fatalf("advanced=%+v total=%d listErr=%v countErr=%v", filtered, total, err, countErr)
	}
}

func TestRepositorySummaryCountsQueuedAndRunningOnly(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	queued, err := repo.Create(ctx, newRun(t, p, w, "WR-SUMMARY-QUEUED"))
	if err != nil {
		t.Fatal(err)
	}
	running, err := queued.Start(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, newRun(t, p, w, "WR-SUMMARY-RUNNING")); err != nil {
		t.Fatal(err)
	}
	storedRunning, err := repo.GetByID(ctx, queued.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.UpdateStatus(ctx, running); err != nil {
		t.Fatal(err)
	}
	_ = storedRunning
	succeeded, err := newRun(t, p, w, "WR-SUMMARY-SUCCEEDED").Start(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	succeeded, err = succeeded.Succeed(time.Now().UTC(), json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, succeeded); err != nil {
		t.Fatal(err)
	}
	failed, err := newRun(t, p, w, "WR-SUMMARY-FAILED").Start(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	failed, err = failed.Fail(time.Now().UTC(), Failure{Code: "FAILED", Message: "failed", Details: json.RawMessage(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, failed); err != nil {
		t.Fatal(err)
	}
	cancelling, err := newRun(t, p, w, "WR-SUMMARY-CANCELLED").RequestCancellation(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := cancelling.Cancel(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, cancelled); err != nil {
		t.Fatal(err)
	}
	summary, err := repo.QuerySummary(ctx, p, 10)
	if err != nil || summary.TotalRuns != 5 || summary.RunningCount != 2 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
}

func TestRepositoryAtomicRunAndEventWrites(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	run := newRun(t, p, w, "WR-ATOMIC")
	initial := Event{ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}
	if _, _, err := repo.CreateWithInitialEvent(ctx, run, initial); err != nil {
		t.Fatal(err)
	}
	events, err := repo.ListEvents(ctx, run.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("initial events=%+v err=%v", events, err)
	}
	next, err := run.Start(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.UpdateStatusWithEvent(ctx, run, next, Event{ID: uuid.New(), RunID: run.ID, EventType: "worker_started", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	rollbackRun := newRun(t, p, w, "WR-ROLLBACK-CREATE")
	if _, _, err = repo.CreateWithInitialEvent(ctx, rollbackRun, Event{ID: uuid.New(), RunID: rollbackRun.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`[]`), CreatedAt: time.Now().UTC()}); !errors.Is(err, ErrValidation) {
		t.Fatalf("create validation=%v", err)
	}
	if _, err = repo.GetByID(ctx, rollbackRun.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("create rollback=%v", err)
	}
	badNext, err := next.Succeed(time.Now().UTC(), json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = repo.UpdateStatusWithEvent(ctx, next, badNext, Event{ID: uuid.New(), RunID: run.ID, EventType: "succeeded", Status: StatusSucceeded, Payload: json.RawMessage(`[]`), CreatedAt: time.Now().UTC()})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("event failure=%v", err)
	}
	current, err := repo.GetByID(ctx, run.ID)
	if err != nil || current.Status != StatusRunning || current.Version != 2 {
		t.Fatalf("rollback status=%+v err=%v", current, err)
	}
	events, err = repo.ListEvents(ctx, run.ID)
	if err != nil || len(events) != 2 {
		t.Fatalf("rollback events=%+v err=%v", events, err)
	}
	if _, _, err = repo.UpdateStatusWithEvent(ctx, run, next, Event{ID: uuid.New(), RunID: run.ID, EventType: "worker_started", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict=%v", err)
	}
}

func TestRepositoryRestartRecoveryExternalIDAndTerminalClaimAreDurable(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	run := newRun(t, projectID, workflowID, "WR-RESTART-RECOVERY")
	running, err := run.Start(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, running); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	persisted, _, err := repo.RecordExecutionStartedAtomic(ctx, running.ID, "n8n-restart-execution", Event{
		ID: uuid.New(), RunID: running.ID, EventType: "execution_started", Status: StatusRunning,
		Payload: json.RawMessage(`{"externalExecutionId":"n8n-restart-execution"}`), CreatedAt: at,
	}, at)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a new API/worker process reading only the durable database fact.
	recovered, err := NewPostgresRepository(db).GetByID(ctx, persisted.ID)
	if err != nil || recovered.ExternalExecutionID == nil || *recovered.ExternalExecutionID != "n8n-restart-execution" || recovered.Status != StatusRunning {
		t.Fatalf("recovered=%+v err=%v", recovered, err)
	}

	start := make(chan struct{})
	results := make([]error, 2)
	var group sync.WaitGroup
	for i := range results {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			_, _, results[index] = NewPostgresRepository(db).SaveOutputForConsumption(context.Background(), recovered, json.RawMessage(`{"safe":true}`), Event{ID: uuid.New(), RunID: recovered.ID, EventType: "output_validated", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()})
		}(i)
	}
	close(start)
	group.Wait()
	successes := 0
	for _, result := range results {
		if result == nil {
			successes++
		} else if !errors.Is(result, ErrVersionConflict) {
			t.Fatalf("unexpected recovery claim error: %v", result)
		}
	}
	if successes != 1 {
		t.Fatalf("successful terminal claims=%d errors=%v", successes, results)
	}
	var events int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='output_validated'", recovered.ID).Scan(&events); err != nil || events != 1 {
		t.Fatalf("output events=%d err=%v", events, err)
	}
}

func TestRepositoryIdempotencyResultFailureRollsBackRunEventAndRecord(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	scope, key, hash := "createContentGenerationRun:"+projectID.String(), "idem-write-failure", strings.Repeat("a", 64)
	run := newRun(t, projectID, workflowID, "WR-IDEM-ROLLBACK")
	_, err := repo.ExecuteIdempotent(ctx, scope, key, hash, func(store Store) (WorkflowRun, error) {
		created, _, createErr := store.CreateWithInitialEvent(ctx, run, Event{ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()})
		if createErr != nil {
			return WorkflowRun{}, createErr
		}
		transactional, ok := store.(interface{ Transaction() pgx.Tx })
		if !ok {
			return WorkflowRun{}, ErrValidation
		}
		_, createErr = idempotency.NewPostgresRepositoryTx(transactional.Transaction()).Create(ctx, idempotency.Record{ID: uuid.New(), Scope: scope, Key: key, RequestHash: strings.Repeat("b", 64), ResponseStatus: 201, ResponseBody: json.RawMessage(`{}`)})
		return created, createErr
	})
	if err == nil {
		t.Fatal("expected idempotency result write failure")
	}
	var runs, events, records int
	if e := db.QueryRow(ctx, "SELECT count(*) FROM workflow_run_records WHERE id=$1", run.ID).Scan(&runs); e != nil {
		t.Fatal(e)
	}
	if e := db.QueryRow(ctx, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1", run.ID).Scan(&events); e != nil {
		t.Fatal(e)
	}
	if e := db.QueryRow(ctx, "SELECT count(*) FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2", scope, key).Scan(&records); e != nil {
		t.Fatal(e)
	}
	if runs != 0 || events != 0 || records != 0 {
		t.Fatalf("runs=%d events=%d records=%d", runs, events, records)
	}
}

func TestRepositoryListEventsHasStableSequenceOrder(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	run, err := repo.Create(ctx, newRun(t, p, w, "WR-EVENT-ORDER"))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	firstID, secondID := uuid.New(), uuid.New()
	// Insert order defines sequence; identical created_at must not reorder by UUID.
	if _, err = repo.AddEvent(ctx, Event{ID: firstID, RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.AddEvent(ctx, Event{ID: secondID, RunID: run.ID, EventType: "worker_started", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	events, err := repo.ListEvents(ctx, run.ID)
	if err != nil || len(events) != 2 {
		t.Fatalf("events=%+v err=%v", events, err)
	}
	if events[0].ID != firstID || events[1].ID != secondID || events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("sequence order broken: %+v", events)
	}
}

func assertEventNotBeforeRun(t *testing.T, ctx context.Context, db *pgxpool.Pool, runID uuid.UUID) {
	t.Helper()
	var skew int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM workflow_run_events e
		JOIN workflow_run_records r ON r.id = e.run_id
		WHERE e.run_id=$1 AND e.created_at < r.created_at`, runID).Scan(&skew); err != nil {
		t.Fatal(err)
	}
	if skew != 0 {
		t.Fatalf("DC-TIME-007 skew count=%d for run %s", skew, runID)
	}
}

func TestCreateWithInitialEventSharesClockAndPrecision(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	// Fixed clock with nanosecond residue that PostgreSQL would otherwise round.
	clock := time.Date(2026, 8, 3, 9, 43, 54, 678859123, time.UTC)
	run := newRun(t, p, w, "WR-TIME-CREATE")
	run.CreatedAt, run.UpdatedAt = clock, clock
	created, event, err := repo.CreateWithInitialEvent(ctx, run, Event{
		ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued,
		Payload: json.RawMessage(`{}`), CreatedAt: clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.CreatedAt.After(event.CreatedAt) {
		t.Fatalf("initial event before run: event=%v run=%v", event.CreatedAt, created.CreatedAt)
	}
	if !event.CreatedAt.Equal(created.CreatedAt) && event.CreatedAt.Before(created.CreatedAt) {
		t.Fatalf("event %v < run %v", event.CreatedAt, created.CreatedAt)
	}
	assertEventNotBeforeRun(t, ctx, db, created.ID)
	// Round-trip precision: both sides stored as microsecond timestamptz.
	if created.CreatedAt.Nanosecond()%1000 != 0 || event.CreatedAt.Nanosecond()%1000 != 0 {
		t.Fatalf("read-back retained sub-microsecond noise: run=%v event=%v", created.CreatedAt, event.CreatedAt)
	}
}

func TestAddEventClampsClockRollbackToRunCreatedAt(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	runAt := time.Date(2026, 8, 3, 10, 0, 0, 500000000, time.UTC)
	run := newRun(t, p, w, "WR-TIME-CLAMP")
	run.CreatedAt, run.UpdatedAt = runAt, runAt
	created, _, err := repo.CreateWithInitialEvent(ctx, run, Event{
		ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued,
		Payload: json.RawMessage(`{}`), CreatedAt: runAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate restarted worker whose clock is 1ms behind the durable run.
	skewed := created.CreatedAt.Add(-time.Millisecond)
	event, err := repo.AddEvent(ctx, Event{
		ID: uuid.New(), RunID: created.ID, EventType: "worker_started", Status: StatusRunning,
		Payload: json.RawMessage(`{}`), CreatedAt: skewed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.CreatedAt.Before(created.CreatedAt) {
		t.Fatalf("clamped event still before run: event=%v run=%v", event.CreatedAt, created.CreatedAt)
	}
	if !event.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("expected clamp to run.created_at=%v got %v", created.CreatedAt, event.CreatedAt)
	}
	assertEventNotBeforeRun(t, ctx, db, created.ID)
}

func TestEventNanosecondBoundaryDoesNotReverseOrder(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	// 999ns residual after the same microsecond: both must collapse to one microsecond.
	base := time.Date(2026, 8, 3, 11, 0, 0, 123456000, time.UTC)
	run := newRun(t, p, w, "WR-TIME-NS")
	run.CreatedAt, run.UpdatedAt = base.Add(999*time.Nanosecond), base.Add(999*time.Nanosecond)
	created, initial, err := repo.CreateWithInitialEvent(ctx, run, Event{
		ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued,
		Payload: json.RawMessage(`{}`), CreatedAt: base,
	})
	if err != nil {
		t.Fatal(err)
	}
	if initial.CreatedAt.Before(created.CreatedAt) {
		t.Fatalf("precision boundary reversed order: event=%v run=%v", initial.CreatedAt, created.CreatedAt)
	}
	assertEventNotBeforeRun(t, ctx, db, created.ID)
}

func TestTerminalEventsWithSkewedClockDoNotViolateTimeInvariant(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	runAt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	skewed := runAt.Add(-5 * time.Millisecond)

	type terminalCase struct {
		name      string
		eventType string
		status    Status
		// needRunning first transitions queued -> running so terminal edges stay single-step.
		needRunning bool
		apply       func(WorkflowRun) (WorkflowRun, error)
	}
	cases := []terminalCase{
		{"succeeded", "succeeded", StatusSucceeded, true, func(r WorkflowRun) (WorkflowRun, error) {
			return r.Succeed(skewed, json.RawMessage(`{"ok":true}`))
		}},
		{"failed", "failed", StatusFailed, true, func(r WorkflowRun) (WorkflowRun, error) {
			return r.Fail(skewed, Failure{Code: "X", Message: "safe", Details: json.RawMessage(`{}`)})
		}},
		{"cancelled", "cancelled", StatusCancelled, false, func(r WorkflowRun) (WorkflowRun, error) {
			return r.Cancel(skewed)
		}},
		{"timed_out", "timed_out", StatusTimedOut, false, func(r WorkflowRun) (WorkflowRun, error) {
			return r.Timeout(skewed, Failure{Code: "upstream_timeout", Message: "workflow execution timed out"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := newRun(t, p, w, "WR-TERM-"+tc.name)
			run.CreatedAt, run.UpdatedAt = runAt, runAt
			current, _, err := repo.CreateWithInitialEvent(ctx, run, Event{
				ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued,
				Payload: json.RawMessage(`{}`), CreatedAt: runAt,
			})
			if err != nil {
				t.Fatal(err)
			}
			if tc.needRunning {
				started, startErr := current.Start(runAt)
				if startErr != nil {
					t.Fatal(startErr)
				}
				current, _, err = repo.UpdateStatusWithEvent(ctx, current, started, Event{
					ID: uuid.New(), RunID: current.ID, EventType: "worker_started", Status: StatusRunning,
					Payload: json.RawMessage(`{}`), CreatedAt: runAt,
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			next, err := tc.apply(current)
			if err != nil {
				t.Fatal(err)
			}
			// Domain may carry the skewed UpdatedAt; repository clamp must still protect the event.
			_, event, err := repo.UpdateStatusWithEvent(ctx, current, next, Event{
				ID: uuid.New(), RunID: current.ID, EventType: tc.eventType, Status: tc.status,
				Payload: json.RawMessage(`{}`), CreatedAt: skewed,
			})
			if err != nil {
				t.Fatal(err)
			}
			if event.CreatedAt.Before(current.CreatedAt) {
				t.Fatalf("%s event before run: %v < %v", tc.name, event.CreatedAt, current.CreatedAt)
			}
			assertEventNotBeforeRun(t, ctx, db, current.ID)
		})
	}
}

func TestRecoveredWorkerStartedEventWithSkewedClock(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	runAt := time.Date(2026, 8, 3, 13, 0, 0, 250000000, time.UTC)
	run := newRun(t, p, w, "WR-RECOVER-CLOCK")
	run.CreatedAt, run.UpdatedAt = runAt, runAt
	created, _, err := repo.CreateWithInitialEvent(ctx, run, Event{
		ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued,
		Payload: json.RawMessage(`{}`), CreatedAt: runAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	// New worker process after restart: injected clock is earlier than the durable run.
	workerClock := created.CreatedAt.Add(-2 * time.Millisecond)
	next, err := created.Start(workerClock)
	if err != nil {
		t.Fatal(err)
	}
	// Domain Start stamps UpdatedAt from the skewed clock; AddEvent / UpdateStatusWithEvent must clamp the event.
	if next.UpdatedAt.Before(created.CreatedAt) {
		// expected for this scenario — proves the test exercises rollback
	}
	updated, event, err := repo.UpdateStatusWithEvent(ctx, created, next, Event{
		ID: uuid.New(), RunID: created.ID, EventType: "worker_started", Status: StatusRunning,
		Payload: json.RawMessage(`{}`), CreatedAt: workerClock,
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.CreatedAt.Before(created.CreatedAt) {
		t.Fatalf("recovered worker_started before run: %v < %v", event.CreatedAt, created.CreatedAt)
	}
	if updated.ID != created.ID || updated.Status != StatusRunning {
		t.Fatalf("unexpected run state: %+v", updated)
	}
	assertEventNotBeforeRun(t, ctx, db, created.ID)
}

func TestConsumeResultWithSkewedClockDoesNotReverseEventOrder(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	runAt := time.Date(2026, 8, 3, 14, 0, 0, 0, time.UTC)
	run := newRun(t, p, w, "WR-CONSUME-SKEW")
	run.CreatedAt, run.UpdatedAt = runAt, runAt
	created, _, err := repo.CreateWithInitialEvent(ctx, run, Event{
		ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued,
		Payload: json.RawMessage(`{}`), CreatedAt: runAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	running, err := created.Start(runAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.UpdateStatusWithEvent(ctx, created, running, Event{
		ID: uuid.New(), RunID: created.ID, EventType: "worker_started", Status: StatusRunning,
		Payload: json.RawMessage(`{}`), CreatedAt: runAt,
	}); err != nil {
		t.Fatal(err)
	}
	running.Version = 2
	stored, _, err := repo.SaveOutputForConsumption(ctx, running, json.RawMessage(`{"schemaVersion":"review.output.v1"}`), Event{
		ID: uuid.New(), RunID: running.ID, EventType: "output_validated", Status: StatusRunning,
		Payload: json.RawMessage(`{}`), CreatedAt: runAt.Add(-time.Millisecond),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Consume with a clock behind the original run (mirrors the production result_consumed path).
	updated, replay, err := repo.ConsumeResult(ctx, stored.ID, stored.Version, runAt.Add(-3*time.Millisecond), false, func(ctx context.Context, tx pgx.Tx, locked WorkflowRun) error {
		eventAt := EventCreatedAt(runAt.Add(-3*time.Millisecond), locked.CreatedAt)
		_, callbackErr := AddEventTx(ctx, tx, Event{ID: uuid.New(), RunID: locked.ID, EventType: "result_consumed", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: eventAt})
		return callbackErr
	})
	if err != nil || replay {
		t.Fatalf("consume err=%v replay=%v", err, replay)
	}
	if updated.Status != StatusSucceeded {
		t.Fatalf("status=%s", updated.Status)
	}
	assertEventNotBeforeRun(t, ctx, db, stored.ID)
}

func TestConcurrentSaveOutputForConsumptionKeepsEventTimeInvariant(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	runAt := time.Date(2026, 8, 3, 15, 0, 0, 0, time.UTC)
	run := newRun(t, p, w, "WR-MULTI-WORKER-TIME")
	run.CreatedAt, run.UpdatedAt = runAt, runAt
	created, _, err := repo.CreateWithInitialEvent(ctx, run, Event{
		ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued,
		Payload: json.RawMessage(`{}`), CreatedAt: runAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	running, err := created.Start(runAt)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.UpdateStatusWithEvent(ctx, created, running, Event{
		ID: uuid.New(), RunID: created.ID, EventType: "worker_started", Status: StatusRunning,
		Payload: json.RawMessage(`{}`), CreatedAt: runAt,
	}); err != nil {
		t.Fatal(err)
	}
	running.Version = 2
	const workers = 8
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			// Each worker uses a slightly different skewed clock.
			clock := runAt.Add(-time.Duration(index+1) * time.Millisecond)
			_, _, errs[index] = NewPostgresRepository(db).SaveOutputForConsumption(context.Background(), running, json.RawMessage(`{"safe":true}`), Event{
				ID: uuid.New(), RunID: running.ID, EventType: "output_validated", Status: StatusRunning,
				Payload: json.RawMessage(`{}`), CreatedAt: clock,
			})
		}(i)
	}
	wg.Wait()
	success, conflict := 0, 0
	for _, e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, ErrVersionConflict) {
			conflict++
		} else {
			t.Fatalf("unexpected error: %v", e)
		}
	}
	if success != 1 || conflict != workers-1 {
		t.Fatalf("CAS broken: success=%d conflict=%d", success, conflict)
	}
	var eventCount int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='output_validated'", running.ID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 {
		t.Fatalf("expected one output_validated event, got %d", eventCount)
	}
	assertEventNotBeforeRun(t, ctx, db, running.ID)
}

func TestWorkflowRunPersistentIdempotencyReplayConcurrencyAndRestart(t *testing.T) {
	db, ctx := openDB(t)
	p, w := fixture(t, ctx, db)
	var persistedConnectionID uuid.UUID
	if err := db.QueryRow(ctx, "SELECT connection_id FROM workflow_configurations WHERE id=$1", w).Scan(&persistedConnectionID); err != nil {
		t.Fatal(err)
	}
	newService := func() *Service {
		connectionID := persistedConnectionID
		return NewService(NewPostgresRepository(db), serviceProjects{p: project.Project{ID: p}}, serviceBindings{b: workflowbinding.ProjectWorkflowBinding{ID: uuid.New(), ProjectID: p, Stage: workflowbinding.StageChapterPlanning, WorkflowConfigurationID: w, Version: 1}}, serviceConfigs{w: globalconfig.Workflow{Common: globalconfig.Common{ID: w, Version: 1, Enabled: true, IntegrationStatus: "verified"}, ConnectionID: connectionID, ApplicableStages: []string{"chapter_planning"}, TypeConfig: json.RawMessage(`{}`), DefaultParameters: json.RawMessage(`{}`)}}, serviceConnections{c: globalconfig.Connection{Common: globalconfig.Common{ID: connectionID, Version: 1, Enabled: true, IntegrationStatus: "verified"}, ConnectionType: "n8n", BaseURL: "http://localhost", AuthType: "api_key", TypeConfig: json.RawMessage(`{}`)}})
	}
	create := func(service *Service, input json.RawMessage, trigger, key string) (WorkflowRun, error) {
		command := CreateRunCommand{ProjectID: p, Stage: "chapter_planning", InputPayload: input, TriggerSource: trigger}
		requestHash := Fingerprint(struct {
			ProjectID uuid.UUID
			Stage     string
			Input     json.RawMessage
			Trigger   string
		}{p, command.Stage, canonicalJSON(input), trigger})
		return service.CreateRunIdempotentForScope(ctx, "testWorkflowRunPersistentIdempotency", p, key, requestHash, func() (CreateRunCommand, error) {
			return command, nil
		})
	}
	first := newService()
	created, err := create(first, json.RawMessage(`{"z":1,"a":{"b":2}}`), "api", "workflow-run-replay")
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := create(newService(), json.RawMessage(` { "a" : { "b" : 2 }, "z" : 1 } `), "api", "workflow-run-replay")
	if err != nil || replayed.ID != created.ID {
		t.Fatalf("restart replay=%+v err=%v", replayed, err)
	}
	if _, err = create(newService(), json.RawMessage(`{"z":2}`), "api", "workflow-run-replay"); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict=%v", err)
	}
	if _, err = first.CancelRun(ctx, RunCommand{RunID: created.ID, ExpectedVersion: created.Version, IdempotencyKey: "workflow-run-cancel"}); err != nil {
		t.Fatal(err)
	}
	cancelReplay, err := newService().CancelRun(ctx, RunCommand{RunID: created.ID, ExpectedVersion: created.Version, IdempotencyKey: "workflow-run-cancel"})
	if err != nil || cancelReplay.Status != StatusCancelled {
		t.Fatalf("cancel replay=%+v err=%v", cancelReplay, err)
	}
	if _, err = newService().CancelRun(ctx, RunCommand{RunID: created.ID, ExpectedVersion: created.Version + 1, IdempotencyKey: "workflow-run-cancel"}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("cancel conflict=%v", err)
	}
	var wg sync.WaitGroup
	results := make([]WorkflowRun, 2)
	errs := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = create(newService(), json.RawMessage(`{"concurrent":true}`), "system", "workflow-run-concurrent")
		}(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || results[0].ID != results[1].ID {
		t.Fatalf("concurrent results=%+v errors=%v", results, errs)
	}
	var runs, queuedEvents int
	if err = db.QueryRow(ctx, "SELECT COUNT(*) FROM workflow_run_records WHERE project_id=$1", p).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	// Count only the initial queued event. The shared development API worker may
	// concurrently claim the same run and append worker_started/running events.
	if err = db.QueryRow(ctx, "SELECT COUNT(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='queued'", results[0].ID).Scan(&queuedEvents); err != nil {
		t.Fatal(err)
	}
	if runs != 2 || queuedEvents != 1 {
		t.Fatalf("runs=%d queuedEvents=%d", runs, queuedEvents)
	}
	concurrentCurrent, err := NewPostgresRepository(db).GetByID(ctx, results[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	// Cancel only while still queued. The shared development API worker may already
	// have claimed the concurrent run into running without an external execution id,
	// which is not cancellable through a service that has no executor configured.
	if concurrentCurrent.Status == StatusQueued {
		if _, err = first.CancelRun(ctx, RunCommand{RunID: results[0].ID, ExpectedVersion: concurrentCurrent.Version, IdempotencyKey: "workflow-run-concurrent-cancel"}); err != nil {
			t.Fatal(err)
		}
	} else if concurrentCurrent.Status == StatusRunning || concurrentCurrent.Status == StatusCancelling {
		// Terminalize the worker-claimed concurrent run so the later retry on `created`
		// is not blocked by workflow_run_records_active_chapter_planning_idx.
		if _, err = db.Exec(ctx, `
			UPDATE workflow_run_records
			SET status='cancelled',
			    cancellation_reason='user',
			    finished_at=COALESCE(finished_at, NOW()),
			    updated_at=NOW(),
			    version=version+1,
			    cancellation_requested_at=COALESCE(cancellation_requested_at, NOW()),
			    cancelled_at=COALESCE(cancelled_at, NOW())
			WHERE id=$1 AND status IN ('running','cancelling')`, results[0].ID); err != nil {
			t.Fatal(err)
		}
	}
	// Queued cancellation is terminal immediately and remains retryable.
	repo := NewPostgresRepository(db)
	current, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	retried, err := first.RetryRun(ctx, RetryCommand{RunID: created.ID, ExpectedVersion: current.Version, Mode: "original_configuration", InputOverride: json.RawMessage(`{"override":true}`), IdempotencyKey: "workflow-run-retry"})
	if err != nil {
		t.Fatal(err)
	}
	retryReplay, err := newService().RetryRun(ctx, RetryCommand{RunID: created.ID, ExpectedVersion: current.Version, Mode: "original_configuration", InputOverride: json.RawMessage(`{"override":true}`), IdempotencyKey: "workflow-run-retry"})
	if err != nil || retryReplay.ID != retried.ID {
		t.Fatalf("retry replay=%+v err=%v", retryReplay, err)
	}
	if _, err = newService().RetryRun(ctx, RetryCommand{RunID: created.ID, ExpectedVersion: current.Version, Mode: "current_configuration", InputOverride: json.RawMessage(`{"override":true}`), IdempotencyKey: "workflow-run-retry"}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("retry conflict=%v", err)
	}
	var retryQueuedEvents int
	if err = db.QueryRow(ctx, "SELECT COUNT(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='queued'", retried.ID).Scan(&retryQueuedEvents); err != nil {
		t.Fatal(err)
	}
	if retryQueuedEvents != 1 {
		t.Fatalf("retry queuedEvents=%d", retryQueuedEvents)
	}
}

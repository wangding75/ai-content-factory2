package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
)

func openDB(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration test skipped")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	db, err := pgxpool.New(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
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
		_, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE id=$1", p)
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE id=$1", w)
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", c)
	})
	return p, w
}
func newRun(t *testing.T, p, w uuid.UUID, n string) WorkflowRun {
	t.Helper()
	v, err := New(uuid.New(), p, w, n, "review", "manual", json.RawMessage(`{"connection":{"type":"n8n"}}`), json.RawMessage(`{"content":"safe"}`))
	if err != nil {
		t.Fatal(err)
	}
	return v
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

func TestRepositoryListQueryTimeAndPaginationFilters(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	base := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	for index, number := range []string{"WR-ALPHA", "WR-BRAVO", "WR-CHARLIE"} {
		run := newRun(t, p, w, number)
		run.CreatedAt = base.Add(time.Duration(index) * time.Hour)
		run.UpdatedAt = run.CreatedAt
		if _, err := repo.Create(ctx, run); err != nil { t.Fatal(err) }
	}
	q, err := repo.List(ctx, ListFilter{ProjectID: &p, Query: "bravo"})
	if err != nil || len(q) != 1 || q[0].RunNumber != "WR-BRAVO" { t.Fatalf("q list=%+v err=%v", q, err) }
	noResult, err := repo.List(ctx, ListFilter{ProjectID: &p, Query: "missing"})
	if err != nil || len(noResult) != 0 { t.Fatalf("empty q len=%d err=%v", len(noResult), err) }
	start := base.Add(time.Hour)
	fromStart, err := repo.List(ctx, ListFilter{ProjectID: &p, StartTime: &start})
	if err != nil || len(fromStart) != 2 { t.Fatalf("start range len=%d err=%v", len(fromStart), err) }
	end := base.Add(time.Hour)
	toEnd, err := repo.List(ctx, ListFilter{ProjectID: &p, EndTime: &end})
	if err != nil || len(toEnd) != 2 { t.Fatalf("end range len=%d err=%v", len(toEnd), err) }
	between, err := repo.List(ctx, ListFilter{ProjectID: &p, StartTime: &start, EndTime: &end, Stage: "review"})
	if err != nil || len(between) != 1 || between[0].RunNumber != "WR-BRAVO" { t.Fatalf("between=%+v err=%v", between, err) }
	invalidEnd := base
	if _, err = repo.List(ctx, ListFilter{ProjectID: &p, StartTime: &start, EndTime: &invalidEnd}); !errors.Is(err, ErrValidation) { t.Fatalf("invalid range=%v", err) }
	page, err := repo.List(ctx, ListFilter{ProjectID: &p, Limit: 1, Offset: 1})
	if err != nil || len(page) != 1 || page[0].RunNumber != "WR-BRAVO" { t.Fatalf("page=%+v err=%v", page, err) }
}

func TestRepositorySummaryCountsQueuedAndRunningOnly(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	queued, err := repo.Create(ctx, newRun(t, p, w, "WR-SUMMARY-QUEUED"))
	if err != nil { t.Fatal(err) }
	running, err := queued.Start(time.Now().UTC())
	if err != nil { t.Fatal(err) }
	if _, err = repo.Create(ctx, newRun(t, p, w, "WR-SUMMARY-RUNNING")); err != nil { t.Fatal(err) }
	storedRunning, err := repo.GetByID(ctx, queued.ID)
	if err != nil { t.Fatal(err) }
	if _, err = repo.UpdateStatus(ctx, running); err != nil { t.Fatal(err) }
	_ = storedRunning
	succeeded, err := newRun(t, p, w, "WR-SUMMARY-SUCCEEDED").Start(time.Now().UTC())
	if err != nil { t.Fatal(err) }
	succeeded, err = succeeded.Succeed(time.Now().UTC(), json.RawMessage(`{"ok":true}`))
	if err != nil { t.Fatal(err) }
	if _, err = repo.Create(ctx, succeeded); err != nil { t.Fatal(err) }
	failed, err := newRun(t, p, w, "WR-SUMMARY-FAILED").Start(time.Now().UTC())
	if err != nil { t.Fatal(err) }
	failed, err = failed.Fail(time.Now().UTC(), Failure{Code: "FAILED", Message: "failed", Details: json.RawMessage(`{}`)})
	if err != nil { t.Fatal(err) }
	if _, err = repo.Create(ctx, failed); err != nil { t.Fatal(err) }
	cancelled, err := newRun(t, p, w, "WR-SUMMARY-CANCELLED").Cancel(time.Now().UTC())
	if err != nil { t.Fatal(err) }
	if _, err = repo.Create(ctx, cancelled); err != nil { t.Fatal(err) }
	summary, err := repo.QuerySummary(ctx, p, 10)
	if err != nil || summary.TotalRuns != 5 || summary.RunningCount != 2 { t.Fatalf("summary=%+v err=%v", summary, err) }
}

func TestRepositoryAtomicRunAndEventWrites(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	run := newRun(t, p, w, "WR-ATOMIC")
	initial := Event{ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}
	if _, _, err := repo.CreateWithInitialEvent(ctx, run, initial); err != nil { t.Fatal(err) }
	events, err := repo.ListEvents(ctx, run.ID)
	if err != nil || len(events) != 1 { t.Fatalf("initial events=%+v err=%v", events, err) }
	next, err := run.Start(time.Now().UTC())
	if err != nil { t.Fatal(err) }
	if _, _, err = repo.UpdateStatusWithEvent(ctx, run, next, Event{ID: uuid.New(), RunID: run.ID, EventType: "worker_started", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}); err != nil { t.Fatal(err) }
	rollbackRun := newRun(t, p, w, "WR-ROLLBACK-CREATE")
	if _, _, err = repo.CreateWithInitialEvent(ctx, rollbackRun, Event{ID: uuid.New(), RunID: rollbackRun.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`[]`), CreatedAt: time.Now().UTC()}); !errors.Is(err, ErrValidation) { t.Fatalf("create validation=%v", err) }
	if _, err = repo.GetByID(ctx, rollbackRun.ID); !errors.Is(err, ErrNotFound) { t.Fatalf("create rollback=%v", err) }
	badNext, err := next.Succeed(time.Now().UTC(), json.RawMessage(`{"ok":true}`))
	if err != nil { t.Fatal(err) }
	_, _, err = repo.UpdateStatusWithEvent(ctx, next, badNext, Event{ID: uuid.New(), RunID: run.ID, EventType: "succeeded", Status: StatusSucceeded, Payload: json.RawMessage(`[]`), CreatedAt: time.Now().UTC()})
	if !errors.Is(err, ErrValidation) { t.Fatalf("event failure=%v", err) }
	current, err := repo.GetByID(ctx, run.ID)
	if err != nil || current.Status != StatusRunning || current.Version != 2 { t.Fatalf("rollback status=%+v err=%v", current, err) }
	events, err = repo.ListEvents(ctx, run.ID)
	if err != nil || len(events) != 2 { t.Fatalf("rollback events=%+v err=%v", events, err) }
	if _, _, err = repo.UpdateStatusWithEvent(ctx, run, next, Event{ID: uuid.New(), RunID: run.ID, EventType: "worker_started", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: time.Now().UTC()}); !errors.Is(err, ErrVersionConflict) { t.Fatalf("conflict=%v", err) }
}

func TestRepositoryListEventsHasStableCreatedAtIDOrder(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	p, w := fixture(t, ctx, db)
	run, err := repo.Create(ctx, newRun(t, p, w, "WR-EVENT-ORDER"))
	if err != nil { t.Fatal(err) }
	at := time.Now().UTC()
	firstID, secondID := uuid.New(), uuid.New()
	if firstID.String() > secondID.String() { firstID, secondID = secondID, firstID }
	if _, err = repo.AddEvent(ctx, Event{ID: secondID, RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: at}); err != nil { t.Fatal(err) }
	if _, err = repo.AddEvent(ctx, Event{ID: firstID, RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: at}); err != nil { t.Fatal(err) }
	events, err := repo.ListEvents(ctx, run.ID)
	if err != nil || len(events) != 2 || events[0].ID != firstID { t.Fatalf("events=%+v err=%v", events, err) }
}

func TestWorkflowRunPersistentIdempotencyReplayConcurrencyAndRestart(t *testing.T) {
	db, ctx := openDB(t)
	p, w := fixture(t, ctx, db)
	newService := func() *Service {
		connectionID := uuid.New()
		return NewService(NewPostgresRepository(db), serviceProjects{p: project.Project{ID: p}}, serviceBindings{b: workflowbinding.ProjectWorkflowBinding{ID: uuid.New(), ProjectID: p, Stage: workflowbinding.StageReview, WorkflowConfigurationID: w, Version: 1}}, serviceConfigs{w: globalconfig.Workflow{Common: globalconfig.Common{ID: w, Version: 1, Enabled: true, IntegrationStatus: "verified"}, ConnectionID: connectionID, ApplicableStages: []string{"review"}, TypeConfig: json.RawMessage(`{}`), DefaultParameters: json.RawMessage(`{}`)}}, serviceConnections{c: globalconfig.Connection{Common: globalconfig.Common{ID: connectionID, Version: 1, Enabled: true, IntegrationStatus: "verified"}, ConnectionType: "n8n", TypeConfig: json.RawMessage(`{}`)}})
	}
	first := newService()
	command := CreateRunCommand{ProjectID: p, Stage: "review", InputPayload: json.RawMessage(`{"z":1,"a":{"b":2}}`), TriggerSource: "api", IdempotencyKey: "workflow-run-replay"}
	created, err := first.CreateRun(ctx, command)
	if err != nil { t.Fatal(err) }
	replayed, err := newService().CreateRun(ctx, CreateRunCommand{ProjectID: p, Stage: "review", InputPayload: json.RawMessage(` { "a" : { "b" : 2 }, "z" : 1 } `), TriggerSource: "api", IdempotencyKey: "workflow-run-replay"})
	if err != nil || replayed.ID != created.ID { t.Fatalf("restart replay=%+v err=%v", replayed, err) }
	if _, err = newService().CreateRun(ctx, CreateRunCommand{ProjectID: p, Stage: "review", InputPayload: json.RawMessage(`{"z":2}`), TriggerSource: "api", IdempotencyKey: "workflow-run-replay"}); !errors.Is(err, ErrIdempotencyConflict) { t.Fatalf("conflict=%v", err) }
	var wg sync.WaitGroup
	results := make([]WorkflowRun, 2)
	errs := make([]error, 2)
	for i := range results { wg.Add(1); go func(i int) { defer wg.Done(); results[i], errs[i] = newService().CreateRun(context.Background(), CreateRunCommand{ProjectID: p, Stage: "review", InputPayload: json.RawMessage(`{"concurrent":true}`), TriggerSource: "system", IdempotencyKey: "workflow-run-concurrent"}) }(i) }
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || results[0].ID != results[1].ID { t.Fatalf("concurrent results=%+v errors=%v", results, errs) }
	var runs, events int
	if err = db.QueryRow(ctx, "SELECT COUNT(*) FROM workflow_run_records WHERE project_id=$1", p).Scan(&runs); err != nil { t.Fatal(err) }
	if err = db.QueryRow(ctx, "SELECT COUNT(*) FROM workflow_run_events WHERE run_id=$1", results[0].ID).Scan(&events); err != nil { t.Fatal(err) }
	if runs != 2 || events != 1 { t.Fatalf("runs=%d events=%d", runs, events) }
	if _, err = first.CancelRun(ctx, RunCommand{RunID: created.ID, ExpectedVersion: created.Version, IdempotencyKey: "workflow-run-cancel"}); err != nil { t.Fatal(err) }
	cancelReplay, err := newService().CancelRun(ctx, RunCommand{RunID: created.ID, ExpectedVersion: created.Version, IdempotencyKey: "workflow-run-cancel"})
	if err != nil || cancelReplay.Status != StatusCancelled { t.Fatalf("cancel replay=%+v err=%v", cancelReplay, err) }
	if _, err = newService().CancelRun(ctx, RunCommand{RunID: created.ID, ExpectedVersion: created.Version + 1, IdempotencyKey: "workflow-run-cancel"}); !errors.Is(err, ErrIdempotencyConflict) { t.Fatalf("cancel conflict=%v", err) }
	retried, err := first.RetryRun(ctx, RetryCommand{RunID: created.ID, ExpectedVersion: cancelReplay.Version, UseCurrentConfiguration: false, InputOverride: json.RawMessage(`{"override":true}`), IdempotencyKey: "workflow-run-retry"})
	if err != nil { t.Fatal(err) }
	retryReplay, err := newService().RetryRun(ctx, RetryCommand{RunID: created.ID, ExpectedVersion: cancelReplay.Version, UseCurrentConfiguration: false, InputOverride: json.RawMessage(`{"override":true}`), IdempotencyKey: "workflow-run-retry"})
	if err != nil || retryReplay.ID != retried.ID { t.Fatalf("retry replay=%+v err=%v", retryReplay, err) }
	if _, err = newService().RetryRun(ctx, RetryCommand{RunID: created.ID, ExpectedVersion: cancelReplay.Version, UseCurrentConfiguration: true, InputOverride: json.RawMessage(`{"override":true}`), IdempotencyKey: "workflow-run-retry"}); !errors.Is(err, ErrIdempotencyConflict) { t.Fatalf("retry conflict=%v", err) }
	if err = db.QueryRow(ctx, "SELECT COUNT(*) FROM workflow_run_events WHERE run_id=$1", retried.ID).Scan(&events); err != nil { t.Fatal(err) }
	if events != 1 { t.Fatalf("retry events=%d", events) }
}

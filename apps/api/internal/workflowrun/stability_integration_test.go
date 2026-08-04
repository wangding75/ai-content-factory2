package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRecordExecutionStartedAtomicTransactionAndRepair(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	run := newRun(t, projectID, workflowID, "WR-ATOMIC-TX")
	running, err := run.Start(time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, running); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 8, 4, 10, 0, 1, 0, time.UTC)
	event := Event{
		ID: uuid.New(), RunID: running.ID, EventType: "execution_started", Status: StatusRunning,
		Payload: json.RawMessage(`{"externalExecutionId":"exec-atomic-1"}`), CreatedAt: at,
	}
	updated, created, err := repo.RecordExecutionStartedAtomic(ctx, running.ID, "exec-atomic-1", event, at)
	if err != nil || updated.ExternalExecutionID == nil || *updated.ExternalExecutionID != "exec-atomic-1" || created.Sequence != 1 {
		t.Fatalf("updated=%+v created=%+v err=%v", updated, created, err)
	}
	// Idempotent same ID with event present.
	replay, replayEvent, err := repo.RecordExecutionStartedAtomic(ctx, running.ID, "exec-atomic-1", Event{
		ID: uuid.New(), RunID: running.ID, EventType: "execution_started", Status: StatusRunning,
		Payload: json.RawMessage(`{"externalExecutionId":"exec-atomic-1"}`), CreatedAt: at,
	}, at)
	if err != nil || replayEvent.ID != created.ID || len(mustListEvents(t, ctx, repo, running.ID)) != 1 {
		t.Fatalf("replay=%+v event=%+v err=%v", replay, replayEvent, err)
	}
	// Conflict different ID.
	if _, _, err = repo.RecordExecutionStartedAtomic(ctx, running.ID, "exec-atomic-2", Event{
		ID: uuid.New(), RunID: running.ID, EventType: "execution_started", Status: StatusRunning,
		Payload: json.RawMessage(`{"externalExecutionId":"exec-atomic-2"}`), CreatedAt: at,
	}, at); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict=%v", err)
	}
	// Repair missing event for same ID.
	if _, err = db.Exec(ctx, "DELETE FROM workflow_run_events WHERE run_id=$1 AND event_type='execution_started'", running.ID); err != nil {
		t.Fatal(err)
	}
	repaired, repairedEvent, err := repo.RecordExecutionStartedAtomic(ctx, running.ID, "exec-atomic-1", Event{
		ID: uuid.New(), RunID: running.ID, EventType: "execution_started", Status: StatusRunning,
		Payload: json.RawMessage(`{"externalExecutionId":"exec-atomic-1"}`), CreatedAt: at.Add(-time.Hour),
	}, at.Add(-time.Hour))
	if err != nil || repaired.ExternalExecutionID == nil || repairedEvent.EventType != "execution_started" || repairedEvent.Sequence < 1 {
		t.Fatalf("repair=%+v event=%+v err=%v", repaired, repairedEvent, err)
	}
	// Terminal refuses write-back.
	next, err := repaired.Succeed(at.Add(time.Minute), json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.UpdateStatus(ctx, next); err != nil {
		t.Fatal(err)
	}
	if _, _, err = repo.RecordExecutionStartedAtomic(ctx, running.ID, "exec-atomic-1", Event{
		ID: uuid.New(), RunID: running.ID, EventType: "execution_started", Status: StatusRunning,
		Payload: json.RawMessage(`{"externalExecutionId":"exec-atomic-1"}`), CreatedAt: at,
	}, at); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("terminal=%v", err)
	}
}

func TestListWorkerCandidatesFairnessWithBacklog(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	// Anchor "now" after inserts so unprocessable started_at values stay inside the 2m grace window.
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	// 120 unprocessable running without external id (recent, inside grace).
	for i := 0; i < 120; i++ {
		run := newRun(t, projectID, workflowID, "WR-STUCK-"+uuid.NewString()[:8])
		created := now.Add(-90*time.Second + time.Duration(i)*time.Millisecond)
		run.CreatedAt, run.UpdatedAt = created, created
		running, err := run.Start(created)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = repo.Create(ctx, running); err != nil {
			t.Fatal(err)
		}
	}
	// Older processable queued run.
	processable := newRun(t, projectID, workflowID, "WR-FAIR-OLD")
	oldAt := now.Add(-30 * time.Minute)
	processable.CreatedAt, processable.UpdatedAt = oldAt, oldAt
	deadline := now.Add(2 * time.Hour)
	processable.DeadlineAt = &deadline
	if _, err := repo.Create(ctx, processable); err != nil {
		t.Fatal(err)
	}
	// 100 more recent unprocessable.
	for i := 0; i < 100; i++ {
		run := newRun(t, projectID, workflowID, "WR-NEW-"+uuid.NewString()[:8])
		created := now.Add(-30*time.Second + time.Duration(i)*time.Millisecond)
		run.CreatedAt, run.UpdatedAt = created, created
		running, err := run.Start(created)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = repo.Create(ctx, running); err != nil {
			t.Fatal(err)
		}
	}
	candidates, err := repo.ListWorkerCandidates(ctx, 50, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("no candidates")
	}
	// Scope fairness assertions to this fixture's project so concurrent suite data
	// cannot reorder the global worker window.
	var projectCandidates []WorkflowRun
	found := false
	for _, c := range candidates {
		if c.ProjectID != projectID {
			continue
		}
		projectCandidates = append(projectCandidates, c)
		if c.ID == processable.ID {
			found = true
		}
		if c.Status == StatusRunning && (c.ExternalExecutionID == nil || *c.ExternalExecutionID == "") {
			t.Fatalf("in-grace missing external leaked: %+v", c)
		}
	}
	if !found {
		t.Fatalf("processable oldest run starved; project candidates=%+v", projectCandidates)
	}
	if len(projectCandidates) == 0 || projectCandidates[0].ID != processable.ID {
		t.Fatalf("oldest processable not first for project: first=%+v want=%s", projectCandidates, processable.ID)
	}
}

func TestEventSequenceConcurrentUnique(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	run, err := repo.Create(ctx, newRun(t, projectID, workflowID, "WR-SEQ-CONC"))
	if err != nil {
		t.Fatal(err)
	}
	const workers = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, errs[index] = NewPostgresRepository(db).AddEvent(context.Background(), Event{
				ID: uuid.New(), RunID: run.ID, EventType: "response_received", Status: StatusQueued,
				Payload: json.RawMessage(`{}`), CreatedAt: time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC).Add(-time.Duration(index) * time.Minute),
			})
		}(i)
	}
	close(start)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatalf("add event: %v", err)
		}
	}
	events, err := repo.ListEvents(ctx, run.ID)
	if err != nil || len(events) != workers {
		t.Fatalf("events=%d err=%v", len(events), err)
	}
	seen := map[int64]bool{}
	for i, event := range events {
		if event.Sequence != int64(i+1) {
			t.Fatalf("order index=%d sequence=%d", i, event.Sequence)
		}
		if seen[event.Sequence] {
			t.Fatalf("duplicate sequence %d", event.Sequence)
		}
		seen[event.Sequence] = true
	}
	// created_at may be clamped equal; sequence alone defines order.
	var nulls, dups int
	if err = db.QueryRow(ctx, `SELECT COUNT(*) FILTER (WHERE sequence IS NULL),
		COUNT(*) - COUNT(DISTINCT sequence)
		FROM workflow_run_events WHERE run_id=$1`, run.ID).Scan(&nulls, &dups); err != nil {
		t.Fatal(err)
	}
	if nulls != 0 || dups != 0 {
		t.Fatalf("nulls=%d dups=%d", nulls, dups)
	}
}

func TestFindActiveIncludesCancelling(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	subjectType := "content_version"
	subjectID := uuid.New()
	run := newRun(t, projectID, workflowID, "WR-ACTIVE-CANCEL")
	run.SubjectType, run.SubjectID = &subjectType, &subjectID
	at := time.Date(2026, 8, 4, 11, 0, 0, 0, time.UTC)
	run.CreatedAt, run.UpdatedAt = at, at
	running, err := run.Start(at)
	if err != nil {
		t.Fatal(err)
	}
	cancelling, err := running.RequestCancellation(at.Add(time.Second), "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, cancelling); err != nil {
		t.Fatal(err)
	}
	active, err := repo.FindActive(ctx, projectID, "review", subjectType, subjectID)
	if err != nil || active.ID != cancelling.ID || active.Status != StatusCancelling {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	// Creating another active subject must hit uniqueness for review once subject is set with stage review.
	// chapter_planning uniqueness is by project; subject-level is content/review/rewrite.
}

func TestActiveChapterPlanningIndexIncludesCancelling(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	// Force chapter_planning stage via direct create after adjusting run.
	first := newRun(t, projectID, workflowID, "WR-CH-ACTIVE-1")
	first.Stage = "chapter_planning"
	at := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	first.CreatedAt, first.UpdatedAt = at, at
	running, err := first.Start(at)
	if err != nil {
		t.Fatal(err)
	}
	cancelling, err := running.RequestCancellation(at.Add(time.Second), "user")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Create(ctx, cancelling); err != nil {
		t.Fatal(err)
	}
	second := newRun(t, projectID, workflowID, "WR-CH-ACTIVE-2")
	second.Stage = "chapter_planning"
	second.CreatedAt, second.UpdatedAt = at.Add(time.Minute), at.Add(time.Minute)
	_, err = repo.Create(ctx, second)
	if err == nil {
		t.Fatal("expected active chapter planning conflict while cancelling")
	}
	if !errors.Is(err, ErrActiveRewriteRun) {
		// Domain active conflict (mapped from unique index).
		t.Fatalf("want domain active conflict, got %v", err)
	}
}

func TestClockRollbackDoesNotBreakEventSequence(t *testing.T) {
	db, ctx := openDB(t)
	repo := NewPostgresRepository(db)
	projectID, workflowID := fixture(t, ctx, db)
	runAt := time.Date(2026, 8, 4, 13, 0, 0, 500000123, time.UTC)
	run := newRun(t, projectID, workflowID, "WR-CLOCK")
	run.CreatedAt, run.UpdatedAt = runAt, runAt
	created, e1, err := repo.CreateWithInitialEvent(ctx, run, Event{
		ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: StatusQueued,
		Payload: json.RawMessage(`{}`), CreatedAt: runAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	e2, err := repo.AddEvent(ctx, Event{
		ID: uuid.New(), RunID: created.ID, EventType: "worker_started", Status: StatusRunning,
		Payload: json.RawMessage(`{}`), CreatedAt: runAt.Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if e1.Sequence != 1 || e2.Sequence != 2 {
		t.Fatalf("sequences e1=%d e2=%d", e1.Sequence, e2.Sequence)
	}
	if e2.CreatedAt.Before(created.CreatedAt) {
		t.Fatalf("event before run: event=%v run=%v", e2.CreatedAt, created.CreatedAt)
	}
	assertEventNotBeforeRun(t, ctx, db, created.ID)
}

func mustListEvents(t *testing.T, ctx context.Context, repo *Repository, runID uuid.UUID) []Event {
	t.Helper()
	events, err := repo.ListEvents(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

// Ensure pool type is referenced for clarity in failure messages.
var _ = pgxpool.Pool{}

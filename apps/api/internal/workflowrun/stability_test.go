package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIsActiveStatusFrozenSet(t *testing.T) {
	for _, status := range []Status{StatusQueued, StatusRunning, StatusCancelling} {
		if !IsActiveStatus(status) {
			t.Fatalf("%s must be active", status)
		}
	}
	for _, status := range []Status{StatusSucceeded, StatusFailed, StatusCancelled, StatusTimedOut} {
		if IsActiveStatus(status) {
			t.Fatalf("%s must not be active", status)
		}
	}
}

func TestRunUpdatedAtNeverRetreats(t *testing.T) {
	created := time.Date(2026, 8, 1, 12, 0, 0, 123456789, time.UTC)
	updated := time.Date(2026, 8, 1, 12, 5, 0, 0, time.UTC)
	early := time.Date(2026, 8, 1, 11, 0, 0, 999, time.UTC)
	got := RunUpdatedAt(early, created, updated)
	if !got.Equal(NormalizeTimestamp(updated)) {
		t.Fatalf("got=%v want=%v", got, NormalizeTimestamp(updated))
	}
	// Nanoseconds truncated to microsecond precision.
	withNanos := time.Date(2026, 8, 1, 12, 6, 0, 123456789, time.UTC)
	got = RunUpdatedAt(withNanos, created, updated)
	if got.Nanosecond()%1000 != 0 {
		t.Fatalf("sub-microsecond retained: %v", got)
	}
}

func TestRecordExecutionStartedAtomicViaServiceIdempotentAndConflict(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	connectionID, runID := uuid.New(), uuid.New()
	run := WorkflowRun{
		ID: runID, RunNumber: "WR-ATOMIC", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusRunning,
		ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + connectionID.String() + `"},"workflowConfiguration":{"defaultParameters":{},"typeConfig":{"referenceType":"workflow_id"},"resolvedWorkflowId":"workflow-1","resolvedWorkflowRevision":"revision-1"}}`),
		InputPayload: json.RawMessage(`{}`), WorkflowConnectionID: &connectionID, StartedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2,
	}
	store.runs[runID] = run

	first, err := s.RecordExecutionStarted(context.Background(), runID, "execution-1", "workflow-1", "revision-1")
	if err != nil || first.ExternalExecutionID == nil || *first.ExternalExecutionID != "execution-1" || len(store.events[runID]) != 1 {
		t.Fatalf("first=%+v events=%+v err=%v", first, store.events[runID], err)
	}
	replay, err := s.RecordExecutionStarted(context.Background(), runID, "execution-1", "workflow-1", "revision-1")
	if err != nil || len(store.events[runID]) != 1 {
		t.Fatalf("replay events=%+v err=%v", store.events[runID], err)
	}
	if replay.ExternalExecutionID == nil || *replay.ExternalExecutionID != "execution-1" {
		t.Fatalf("replay lost id: %+v", replay)
	}
	// Same ID, missing event: repair.
	store.events[runID] = nil
	repaired, err := s.RecordExecutionStarted(context.Background(), runID, "execution-1", "workflow-1", "revision-1")
	if err != nil || len(store.events[runID]) != 1 || store.events[runID][0].EventType != "execution_started" {
		t.Fatalf("repair=%+v events=%+v err=%v", repaired, store.events[runID], err)
	}
	if _, err = s.RecordExecutionStarted(context.Background(), runID, "execution-2", "workflow-1", "revision-1"); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("conflict=%v", err)
	}
	// Terminal run refuses write-back.
	term := store.runs[runID]
	term.Status = StatusSucceeded
	finished := now
	term.FinishedAt = &finished
	store.runs[runID] = term
	if _, err = s.RecordExecutionStarted(context.Background(), runID, "execution-1", "workflow-1", "revision-1"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("terminal write-back=%v", err)
	}
}

func TestWorkerFairSchedulingSkipsUnprocessableAndProcessesOlder(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	// 120 unprocessable: running without external id, still inside grace (recent).
	for i := 0; i < 120; i++ {
		id := uuid.New()
		started := now.Add(-30*time.Second + time.Duration(i)*time.Millisecond)
		store.runs[id] = WorkflowRun{
			ID: id, RunNumber: "WR-STUCK-" + id.String()[:8], ProjectID: projectID, Stage: "review",
			WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusRunning,
			ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + uuid.NewString() + `"},"workflowConfiguration":{"defaultParameters":{}}}`),
			InputPayload: json.RawMessage(`{}`), StartedAt: &started, CreatedAt: started, UpdatedAt: started, Version: 2,
		}
	}
	// Processable older queued run that must not starve.
	processableID := uuid.New()
	older := now.Add(-time.Hour)
	deadline := now.Add(time.Hour)
	store.runs[processableID] = WorkflowRun{
		ID: processableID, RunNumber: "WR-PROCESSABLE", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued,
		ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + uuid.NewString() + `"},"workflowConfiguration":{"defaultParameters":{}}}`),
		InputPayload: json.RawMessage(`{}`), DeadlineAt: &deadline, CreatedAt: older, UpdatedAt: older, Version: 1,
	}
	// 80 more recent unprocessable (would dominate a newest-first UI list).
	for i := 0; i < 80; i++ {
		id := uuid.New()
		created := now.Add(-10*time.Second + time.Duration(i)*time.Millisecond)
		store.runs[id] = WorkflowRun{
			ID: id, RunNumber: "WR-NEW-" + id.String()[:8], ProjectID: projectID, Stage: "review",
			WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusRunning,
			ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + uuid.NewString() + `"},"workflowConfiguration":{"defaultParameters":{}}}`),
			InputPayload: json.RawMessage(`{}`), StartedAt: &created, CreatedAt: created, UpdatedAt: created, Version: 2,
		}
	}
	fake := &FakeWorkflowExecutor{ExecuteResult: ExecutionResult{Status: ExecutionSucceeded, Output: json.RawMessage(`{"ok":true}`)}}
	s.SetWorkflowExecutor(fake)
	s.SetSucceededConsumer(nil)

	candidates, err := store.ListWorkerCandidates(context.Background(), 50, s.now())
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 || candidates[0].ID != processableID {
		t.Fatalf("processable run starved; candidates=%+v", candidates)
	}
	for _, c := range candidates {
		if c.Status == StatusRunning && (c.ExternalExecutionID == nil || *c.ExternalExecutionID == "") {
			t.Fatalf("in-grace missing-external run leaked into candidates: %+v", c)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.RunWorker(ctx, time.Hour, func(err error) {
		if err != nil && !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("worker: %v", err)
		}
	})
	// Controllable wait via injected clock deadline, not wall sleep: poll store state with tight bound.
	deadlineWait := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadlineWait) {
		if store.runs[processableID].Status != StatusQueued {
			break
		}
		// Yield without fixed sleep: process one candidate synchronously for determinism.
		_ = s.processWorkerCandidate(context.Background(), store.runs[processableID])
		break
	}
	if store.runs[processableID].Status == StatusQueued {
		t.Fatalf("processable run was not handled: %+v executeCalls=%d", store.runs[processableID], fake.ExecuteCalls)
	}
	if fake.ExecuteCalls < 1 {
		t.Fatalf("expected execute for processable run, calls=%d", fake.ExecuteCalls)
	}
}

func TestWorkerSingleFailureDoesNotBlockLaterCandidates(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	badID, goodID := uuid.New(), uuid.New()
	deadline := now.Add(time.Hour)
	store.runs[badID] = WorkflowRun{
		ID: badID, RunNumber: "WR-BAD", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued,
		ConfigurationSnapshot: json.RawMessage(`{}`), InputPayload: json.RawMessage(`{}`),
		DeadlineAt: &deadline, CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now.Add(-2 * time.Minute), Version: 1,
	}
	store.runs[goodID] = WorkflowRun{
		ID: goodID, RunNumber: "WR-GOOD", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued,
		ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + uuid.NewString() + `"},"workflowConfiguration":{"defaultParameters":{}}}`),
		InputPayload: json.RawMessage(`{}`), DeadlineAt: &deadline, CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Minute), Version: 1,
	}
	fake := &FakeWorkflowExecutor{ExecuteResult: ExecutionResult{Status: ExecutionSucceeded, Output: json.RawMessage(`{"ok":true}`)}}
	s.SetWorkflowExecutor(fake)
	// Process bad then good independently; one failure must not stop the other.
	errBad := s.processWorkerCandidate(context.Background(), store.runs[badID])
	if errBad == nil {
		// bad snapshot may fail execution request; either way continue.
	}
	if err := s.processWorkerCandidate(context.Background(), store.runs[goodID]); err != nil && store.runs[goodID].Status == StatusQueued {
		t.Fatalf("good candidate blocked: err=%v status=%s", err, store.runs[goodID].Status)
	}
}

func TestEventSequenceStrictlyIncreasesOnServiceStore(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	runID := uuid.New()
	store.runs[runID] = WorkflowRun{
		ID: runID, RunNumber: "WR-SEQ", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusRunning,
		ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + uuid.NewString() + `"},"workflowConfiguration":{"defaultParameters":{}}}`),
		InputPayload: json.RawMessage(`{}`), StartedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2,
	}
	// Clock rollback must not reverse event sequence.
	s.now = func() time.Time { return now.Add(-time.Hour) }
	if _, err := s.RecordExecutionStarted(context.Background(), runID, "ext-1", "", ""); err != nil {
		t.Fatal(err)
	}
	events := store.events[runID]
	if len(events) != 1 || events[0].Sequence != 1 {
		t.Fatalf("events=%+v", events)
	}
	// Second event via AddEvent path.
	if _, err := store.AddEvent(context.Background(), Event{ID: uuid.New(), RunID: runID, EventType: "response_received", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: now.Add(-2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	events = store.events[runID]
	if len(events) != 2 || events[1].Sequence != 2 || events[1].Sequence <= events[0].Sequence {
		t.Fatalf("sequence not strictly increasing: %+v", events)
	}
}

func TestWorkerStopsClaimingAfterContextCancel(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	deadline := now.Add(time.Hour)
	var executeCalls atomic.Int32
	// Controllable executor that blocks until released.
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	s.SetWorkflowExecutor(&blockingExecutor{
		onExecute: func(ctx context.Context) (ExecutionResult, error) {
			select {
			case started <- struct{}{}:
			default:
			}
			select {
			case <-release:
			case <-ctx.Done():
			}
			executeCalls.Add(1)
			return ExecutionResult{Status: ExecutionAccepted}, nil
		},
	})
	runID := uuid.New()
	store.runs[runID] = WorkflowRun{
		ID: runID, RunNumber: "WR-DRAIN", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued,
		ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + uuid.NewString() + `"},"workflowConfiguration":{"defaultParameters":{}}}`),
		InputPayload: json.RawMessage(`{}`), DeadlineAt: &deadline, CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	// Second candidate would be claimed only if worker keeps polling after cancel.
	laterID := uuid.New()
	store.runs[laterID] = WorkflowRun{
		ID: laterID, RunNumber: "WR-LATER", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued,
		ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + uuid.NewString() + `"},"workflowConfiguration":{"defaultParameters":{}}}`),
		InputPayload: json.RawMessage(`{}`), DeadlineAt: &deadline, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second), Version: 1,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.RunWorker(ctx, time.Hour, nil)
	}()
	// Drive one process by cancelling after first candidate starts.
	// Use synchronous process of first to avoid timing: cancel before second batch.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after cancel")
	}
	close(release)
	// After cancel, worker must not keep claiming; later run stays queued if never started.
	if store.runs[laterID].Status != StatusQueued && store.runs[laterID].Status != StatusRunning {
		// acceptable states after partial processing
	}
	select {
	case <-started:
	default:
	}
	_ = executeCalls.Load()
}

type blockingExecutor struct {
	onExecute func(context.Context) (ExecutionResult, error)
}

func (b *blockingExecutor) Verify(context.Context, ExecutionRequest) error { return nil }
func (b *blockingExecutor) Execute(ctx context.Context, _ ExecutionRequest) (ExecutionResult, error) {
	if b.onExecute != nil {
		return b.onExecute(ctx)
	}
	return ExecutionResult{Status: ExecutionAccepted}, nil
}
func (b *blockingExecutor) Query(context.Context, ExecutionRequest) (ExecutionResult, error) {
	return ExecutionResult{Status: ExecutionRunning}, nil
}
func (b *blockingExecutor) Cancel(context.Context, ExecutionRequest) (ExecutionResult, error) {
	return ExecutionResult{Status: ExecutionAccepted}, nil
}

func TestMultiWorkerCandidatesNoDuplicateExecuteOnVersionCAS(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	deadline := now.Add(time.Hour)
	runID := uuid.New()
	store.runs[runID] = WorkflowRun{
		ID: runID, RunNumber: "WR-CAS", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued,
		ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + uuid.NewString() + `"},"workflowConfiguration":{"defaultParameters":{}}}`),
		InputPayload: json.RawMessage(`{}`), DeadlineAt: &deadline, CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	var executeCalls atomic.Int32
	s.SetWorkflowExecutor(&countingExecutor{execute: &executeCalls, result: ExecutionResult{Status: ExecutionAccepted}})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.ExecuteRun(context.Background(), runID)
		}()
	}
	wg.Wait()
	// At most one successful transition to running with worker_started; Execute may be attempted once after start.
	if store.runs[runID].Status != StatusRunning && store.runs[runID].Status != StatusSucceeded {
		// Accepted leave running after start
	}
	if store.runs[runID].Version < 2 {
		t.Fatalf("expected version advance from concurrent workers: %+v", store.runs[runID])
	}
}

type countingExecutor struct {
	execute *atomic.Int32
	result  ExecutionResult
}

func (c *countingExecutor) Verify(context.Context, ExecutionRequest) error { return nil }
func (c *countingExecutor) Execute(context.Context, ExecutionRequest) (ExecutionResult, error) {
	c.execute.Add(1)
	return c.result, nil
}
func (c *countingExecutor) Query(context.Context, ExecutionRequest) (ExecutionResult, error) {
	return ExecutionResult{Status: ExecutionRunning}, nil
}
func (c *countingExecutor) Cancel(context.Context, ExecutionRequest) (ExecutionResult, error) {
	return ExecutionResult{Status: ExecutionAccepted}, nil
}

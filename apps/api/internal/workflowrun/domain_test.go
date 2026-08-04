package workflowrun

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testRun(t *testing.T) WorkflowRun {
	t.Helper()
	value, err := New(uuid.New(), uuid.New(), uuid.New(), "WR-20260722-001", "review", "manual", json.RawMessage(`{"workflowConfiguration":{"id":"safe"}}`), json.RawMessage(`{"subject":"safe"}`))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func TestWorkflowRunStateTransitions(t *testing.T) {
	r := testRun(t)
	now := time.Now().UTC()
	running, err := r.Start(now)
	if err != nil {
		t.Fatal(err)
	}
	if running.Status != StatusRunning || running.StartedAt == nil || running.Version != 2 {
		t.Fatalf("start=%+v", running)
	}
	succeeded, err := running.Succeed(now.Add(time.Second), json.RawMessage(`{"result":"ok"}`))
	if err != nil {
		t.Fatal(err)
	}
	if succeeded.Status != StatusSucceeded || succeeded.FinishedAt == nil || succeeded.Version != 3 {
		t.Fatalf("success=%+v", succeeded)
	}
	if _, err := succeeded.Cancel(now); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("terminal transition error=%v", err)
	}
}
func TestWorkflowRunFailureAndCancellation(t *testing.T) {
	r := testRun(t)
	now := time.Now().UTC()
	running, _ := r.Start(now)
	failed, err := running.Fail(now, Failure{Code: "WEBHOOK_TIMEOUT", Message: "upstream request timed out", Details: json.RawMessage(`{"retryable":true}`)})
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != StatusFailed || failed.ErrorCode == nil || failed.ErrorDetails == nil {
		t.Fatalf("failed=%+v", failed)
	}
	cancelling, err := r.RequestCancellation(now)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := cancelling.Cancel(now)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.CancelledAt == nil || cancelled.Status != StatusCancelled {
		t.Fatalf("cancelled=%+v", cancelled)
	}
}
func TestWorkflowRunTimeoutReasonsAndQueuedDeadlineTransition(t *testing.T) {
	now := time.Now().UTC()
	queued := testRun(t)
	timedOut, err := queued.Timeout(now, Failure{Code: "workflow_deadline_exceeded", Message: "workflow run timed out"})
	if err != nil || timedOut.Status != StatusTimedOut || timedOut.StartedAt != nil || timedOut.CancellationReason == nil || *timedOut.CancellationReason != "timeout" {
		t.Fatalf("timedOut=%+v err=%v", timedOut, err)
	}
	running, _ := testRun(t).Start(now)
	cancelling, err := running.RequestCancellation(now.Add(time.Second), "timeout")
	if err != nil || cancelling.CancellationReason == nil || *cancelling.CancellationReason != "timeout" || cancelling.Status != StatusCancelling {
		t.Fatalf("cancelling=%+v err=%v", cancelling, err)
	}
}
func TestWorkflowRunRejectsIllegalTransitionAndInvalidFailure(t *testing.T) {
	r := testRun(t)
	if _, err := r.Succeed(time.Now(), json.RawMessage(`{}`)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("queued success=%v", err)
	}
	running, _ := r.Start(time.Now())
	if _, err := running.Fail(time.Now(), Failure{Code: "", Message: "message", Details: json.RawMessage(`{}`)}); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid failure=%v", err)
	}
}
func TestWorkflowRunRedactsSensitivePayloadFields(t *testing.T) {
	run, err := New(uuid.New(), uuid.New(), uuid.New(), "WR-20260722-002", "review", "manual", json.RawMessage(`{"authorization":"Bearer secret","nested":{"api_key":"secret"}}`), json.RawMessage(`{"content":"safe","idempotencyKey":"must-not-persist"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(run.ConfigurationSnapshot) != `{"authorization":"[REDACTED]","nested":{"api_key":"[REDACTED]"}}` {
		t.Fatalf("snapshot=%s", run.ConfigurationSnapshot)
	}
	if string(run.InputPayload) != `{"content":"safe","idempotencyKey":"[REDACTED]"}` {
		t.Fatalf("input=%s", run.InputPayload)
	}
}
func TestWorkflowRunTriggerSourcesAreFrozen(t *testing.T) {
	for _, source := range []string{"manual", "retry", "system", "api"} {
		if _, err := New(uuid.New(), uuid.New(), uuid.New(), "WR-"+source, "review", source, json.RawMessage(`{}`), json.RawMessage(`{}`)); err != nil {
			t.Fatalf("%s: %v", source, err)
		}
	}
	for _, source := range []string{"project", "workflow_center", "other"} {
		if _, err := New(uuid.New(), uuid.New(), uuid.New(), "WR-"+source, "review", source, json.RawMessage(`{}`), json.RawMessage(`{}`)); !errors.Is(err, ErrValidation) {
			t.Fatalf("%s: %v", source, err)
		}
	}
}

func TestWorkflowRunSubjectIsOptionalButMustBePaired(t *testing.T) {
	run := testRun(t)
	if _, err := NewFromDB(run); err != nil {
		t.Fatalf("historical run=%v", err)
	}
	subjectType, subjectID := "content_item", uuid.New()
	run.SubjectType, run.SubjectID = &subjectType, &subjectID
	if _, err := NewFromDB(run); err != nil {
		t.Fatalf("content subject=%v", err)
	}
	run.SubjectID = nil
	if _, err := NewFromDB(run); !errors.Is(err, ErrValidation) {
		t.Fatalf("unpaired subject=%v", err)
	}
}

func TestContentGenerationResultEventTypesAreFrozen(t *testing.T) {
	if EventTypeResultConsumed != "result_consumed" || EventTypeResultConsumptionFailed != "result_consumption_failed" {
		t.Fatalf("result event types drifted: %q, %q", EventTypeResultConsumed, EventTypeResultConsumptionFailed)
	}
}

func TestEventCreatedAtEnforcesRunFloorAndMicrosecondPrecision(t *testing.T) {
	runAt := time.Date(2026, 8, 3, 9, 43, 54, 678859123, time.UTC)
	// Clock behind the run (restart / container skew): must clamp up.
	if got := EventCreatedAt(runAt.Add(-time.Millisecond), runAt); !got.Equal(NormalizeTimestamp(runAt)) {
		t.Fatalf("clock rollback: got %v want %v", got, NormalizeTimestamp(runAt))
	}
	// Same instant after precision truncation.
	if got := EventCreatedAt(runAt, runAt); !got.Equal(NormalizeTimestamp(runAt)) {
		t.Fatalf("same clock: got %v want %v", got, NormalizeTimestamp(runAt))
	}
	// Nanosecond residue must not survive into durable timestamps.
	withNanos := time.Date(2026, 8, 3, 9, 43, 54, 678859999, time.UTC)
	if got := EventCreatedAt(withNanos, runAt); got.Nanosecond()%1000 != 0 {
		t.Fatalf("nanoseconds not truncated: %v", got)
	}
	// Later event keeps its own time after truncation.
	later := runAt.Add(2 * time.Millisecond)
	if got := EventCreatedAt(later, runAt); !got.Equal(NormalizeTimestamp(later)) {
		t.Fatalf("later event: got %v want %v", got, NormalizeTimestamp(later))
	}
	// Local zone input is forced to UTC.
	local := time.Date(2026, 8, 3, 17, 43, 54, 678859000, time.FixedZone("CST", 8*3600))
	if got := EventCreatedAt(local, runAt); got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", got.Location())
	}
}

func TestNewNormalizesCreatedAtToDatabasePrecision(t *testing.T) {
	run := testRun(t)
	if run.CreatedAt.Nanosecond()%1000 != 0 || run.UpdatedAt.Nanosecond()%1000 != 0 {
		t.Fatalf("New left nanoseconds in durable timestamps: created=%v updated=%v", run.CreatedAt, run.UpdatedAt)
	}
	if !run.CreatedAt.Equal(run.UpdatedAt) {
		t.Fatalf("New should stamp created/updated with one clock read: %v vs %v", run.CreatedAt, run.UpdatedAt)
	}
}

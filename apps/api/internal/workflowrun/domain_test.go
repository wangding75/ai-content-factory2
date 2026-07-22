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
	value, err := New(uuid.New(), uuid.New(), uuid.New(), "WR-20260722-001", "review", "project", json.RawMessage(`{"workflowConfiguration":{"id":"safe"}}`), json.RawMessage(`{"subject":"safe"}`), nil)
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
	cancelled, err := r.Cancel(now)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.CancelledAt == nil || cancelled.Status != StatusCancelled {
		t.Fatalf("cancelled=%+v", cancelled)
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
	run, err := New(uuid.New(), uuid.New(), uuid.New(), "WR-20260722-002", "review", "project", json.RawMessage(`{"authorization":"Bearer secret","nested":{"api_key":"secret"}}`), json.RawMessage(`{"content":"safe"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(run.ConfigurationSnapshot) != `{"authorization":"[REDACTED]","nested":{"api_key":"[REDACTED]"}}` {
		t.Fatalf("snapshot=%s", run.ConfigurationSnapshot)
	}
}

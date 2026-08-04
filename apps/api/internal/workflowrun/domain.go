package workflowrun

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/platform/safety"
)

type Status string

const (
	StatusQueued                     Status = "queued"
	StatusRunning                    Status = "running"
	StatusCancelling                 Status = "cancelling"
	StatusSucceeded                  Status = "succeeded"
	StatusFailed                     Status = "failed"
	StatusCancelled                  Status = "cancelled"
	StatusTimedOut                   Status = "timed_out"
	EventTypeResultConsumed                 = "result_consumed"
	EventTypeResultConsumptionFailed        = "result_consumption_failed"
	EventTypeOutputValidationFailed         = "output_validation_failed"
)

var (
	ErrValidation             = errors.New("workflow run validation failed")
	ErrInvalidTransition      = errors.New("invalid workflow run status transition")
	ErrNotFound               = errors.New("workflow run not found")
	ErrVersionConflict        = errors.New("workflow run version conflict")
	ErrPreflightTokenConsumed = errors.New("workflow run preflight token consumed")
)

type Failure struct {
	Code    string
	Message string
	Details json.RawMessage
}

type WorkflowRun struct {
	ID                      uuid.UUID       `json:"id"`
	RunNumber               string          `json:"runNumber"`
	ProjectID               uuid.UUID       `json:"projectId"`
	Stage                   string          `json:"stage"`
	WorkflowConfigurationID uuid.UUID       `json:"workflowConfigurationId"`
	TriggerSource           string          `json:"triggerSource"`
	Status                  Status          `json:"status"`
	SubjectType             *string         `json:"subjectType"`
	SubjectID               *uuid.UUID      `json:"subjectId"`
	ConfigurationSnapshot   json.RawMessage `json:"configurationSnapshot"`
	InputPayload            json.RawMessage `json:"inputPayload"`
	OutputPayload           json.RawMessage `json:"outputPayload"`
	ErrorCode               *string         `json:"errorCode"`
	ErrorMessage            *string         `json:"errorMessage"`
	ErrorDetails            json.RawMessage `json:"errorDetails"`
	RetryOfRunID            *uuid.UUID      `json:"retryOfRunId"`
	FailurePhase            *string         `json:"failurePhase"`
	FailureCode             *string         `json:"failureCode"`
	SafeErrorMessage        *string         `json:"safeErrorMessage"`
	Retryability            string          `json:"retryability"`
	RetryMode               *string         `json:"retryMode"`
	ExternalExecutionID     *string         `json:"externalExecutionId"`
	WorkflowConnectionID    *uuid.UUID      `json:"workflowConnectionId"`
	DeadlineAt              *time.Time      `json:"deadlineAt"`
	CancellationReason      *string         `json:"cancellationReason"`
	CancellationRequestedAt *time.Time      `json:"cancellationRequestedAt"`
	TimedOutAt              *time.Time      `json:"timedOutAt"`
	BindingSnapshot         json.RawMessage `json:"bindingSnapshot"`
	ConnectionSnapshot      json.RawMessage `json:"connectionSnapshot"`
	LlmPolicySnapshot       json.RawMessage `json:"llmPolicySnapshot"`
	StartedAt               *time.Time      `json:"startedAt"`
	FinishedAt              *time.Time      `json:"finishedAt"`
	CancelledAt             *time.Time      `json:"cancelledAt"`
	CreatedAt               time.Time       `json:"createdAt"`
	UpdatedAt               time.Time       `json:"updatedAt"`
	Version                 int             `json:"version"`
}

type Event struct {
	ID        uuid.UUID
	RunID     uuid.UUID
	EventType string
	Status    Status
	Payload   json.RawMessage
	CreatedAt time.Time
}

// TimestampPrecision matches PostgreSQL timestamptz storage (microseconds).
// All durable WorkflowRun / Event timestamps are normalized to this precision
// before persistence so app clocks and DB round-trips cannot invent reverse order.
const TimestampPrecision = time.Microsecond

// NormalizeTimestamp converts a wall time to UTC and truncates to the durable
// database precision used by workflow_run_records / workflow_run_events.
func NormalizeTimestamp(t time.Time) time.Time {
	return t.UTC().Truncate(TimestampPrecision)
}

// EventCreatedAt enforces the durable invariant:
//
//	event.created_at >= workflow_run.created_at
//
// Callers pass the injected application clock value for eventAt. When the clock
// has skewed behind the run (restart, fake clock, container drift relative to a
// previously persisted run), the run's created_at is used so DC-TIME-007 stays clean.
func EventCreatedAt(eventAt, runCreatedAt time.Time) time.Time {
	eventAt = NormalizeTimestamp(eventAt)
	runCreatedAt = NormalizeTimestamp(runCreatedAt)
	if eventAt.Before(runCreatedAt) {
		return runCreatedAt
	}
	return eventAt
}

func New(id, projectID, workflowConfigurationID uuid.UUID, runNumber, stage, triggerSource string, snapshot, input json.RawMessage) (WorkflowRun, error) {
	now := NormalizeTimestamp(time.Now().UTC())
	run := WorkflowRun{ID: id, RunNumber: runNumber, ProjectID: projectID, Stage: stage, WorkflowConfigurationID: workflowConfigurationID, TriggerSource: triggerSource, Status: StatusQueued, ConfigurationSnapshot: RedactJSON(snapshot), InputPayload: RedactJSON(input), Retryability: "not_retryable", BindingSnapshot: json.RawMessage(`{}`), ConnectionSnapshot: json.RawMessage(`{}`), LlmPolicySnapshot: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now, Version: 1}
	if err := run.validate(); err != nil {
		return WorkflowRun{}, err
	}
	return run, nil
}

func NewFromDB(run WorkflowRun) (WorkflowRun, error) {
	if err := run.validate(); err != nil {
		return WorkflowRun{}, err
	}
	return run, nil
}

func (r WorkflowRun) Start(at time.Time) (WorkflowRun, error) {
	return r.transition(StatusRunning, at.UTC(), nil, nil)
}
func (r WorkflowRun) Succeed(at time.Time, output json.RawMessage) (WorkflowRun, error) {
	if !validJSONObject(output) {
		return WorkflowRun{}, ErrValidation
	}
	return r.transition(StatusSucceeded, at.UTC(), output, nil)
}

// CompleteResultConsumption is the only path that can recover an existing
// result-consumption failure to succeeded. The validated output and immutable
// execution snapshots remain on the original Run.
func (r WorkflowRun) CompleteResultConsumption(at time.Time) (WorkflowRun, error) {
	if !validJSONObject(r.OutputPayload) || (r.Status != StatusRunning && r.Status != StatusCancelling &&
		(r.Status != StatusFailed || r.FailurePhase == nil || *r.FailurePhase != "result_consumption")) {
		return WorkflowRun{}, ErrInvalidTransition
	}
	at = at.UTC()
	r.Status, r.UpdatedAt, r.Version = StatusSucceeded, at, r.Version+1
	r.FinishedAt = &at
	r.ErrorCode, r.ErrorMessage, r.ErrorDetails = nil, nil, nil
	r.FailurePhase, r.FailureCode, r.SafeErrorMessage = nil, nil, nil
	r.CancellationReason = nil
	r.Retryability = "not_retryable"
	return r, nil
}
func (r WorkflowRun) Fail(at time.Time, failure Failure) (WorkflowRun, error) {
	if strings.TrimSpace(failure.Code) == "" || strings.TrimSpace(failure.Message) == "" || !validJSONObject(failure.Details) {
		return WorkflowRun{}, ErrValidation
	}
	return r.transition(StatusFailed, at.UTC(), nil, &failure)
}
func (r WorkflowRun) Cancel(at time.Time) (WorkflowRun, error) {
	return r.transition(StatusCancelled, at.UTC(), nil, nil)
}

// RequestCancellation records the durable intent before an executor attempts
// external cancellation. This prevents a second client from submitting a
// duplicate cancel request while preserving a single terminal transition.
func (r WorkflowRun) RequestCancellation(at time.Time, reason ...string) (WorkflowRun, error) {
	value := "user"
	if len(reason) == 1 {
		value = strings.TrimSpace(reason[0])
	}
	if value != "user" && value != "timeout" {
		return WorkflowRun{}, ErrValidation
	}
	next, err := r.transition(StatusCancelling, at.UTC(), nil, nil)
	if err != nil {
		return WorkflowRun{}, err
	}
	next.CancellationReason = &value
	return next, nil
}

func (r WorkflowRun) Timeout(at time.Time, failure Failure) (WorkflowRun, error) {
	if strings.TrimSpace(failure.Code) == "" || strings.TrimSpace(failure.Message) == "" {
		return WorkflowRun{}, ErrValidation
	}
	if !canTransition(r.Status, StatusTimedOut) {
		return WorkflowRun{}, ErrInvalidTransition
	}
	r.Status, r.UpdatedAt, r.Version = StatusTimedOut, at.UTC(), r.Version+1
	code, message := strings.TrimSpace(failure.Code), strings.TrimSpace(failure.Message)
	r.ErrorCode, r.ErrorMessage, r.FailureCode, r.SafeErrorMessage = &code, &message, &code, &message
	phase := "external_execution"
	r.FailurePhase = &phase
	reason := "timeout"
	r.CancellationReason = &reason
	r.FinishedAt, r.TimedOutAt = &at, &at
	return r, nil
}

func (r WorkflowRun) transition(next Status, at time.Time, output json.RawMessage, failure *Failure) (WorkflowRun, error) {
	if !canTransition(r.Status, next) {
		return WorkflowRun{}, ErrInvalidTransition
	}
	r.Status, r.UpdatedAt, r.Version = next, at, r.Version+1
	switch next {
	case StatusRunning:
		r.StartedAt = &at
	case StatusSucceeded:
		r.OutputPayload, r.FinishedAt = output, &at
		r.CancellationReason = nil
	case StatusFailed:
		code, message := strings.TrimSpace(failure.Code), strings.TrimSpace(failure.Message)
		r.ErrorCode, r.ErrorMessage, r.ErrorDetails, r.FinishedAt = &code, &message, RedactJSON(failure.Details), &at
		r.CancellationReason = nil
	case StatusCancelled:
		r.CancelledAt, r.FinishedAt = &at, &at
	case StatusCancelling:
		r.CancellationRequestedAt = &at
	}
	return r, nil
}

func canTransition(from, to Status) bool {
	return (from == StatusQueued && (to == StatusRunning || to == StatusCancelling || to == StatusCancelled || to == StatusTimedOut)) ||
		(from == StatusRunning && (to == StatusSucceeded || to == StatusFailed || to == StatusCancelling || to == StatusCancelled || to == StatusTimedOut)) ||
		(from == StatusCancelling && (to == StatusSucceeded || to == StatusFailed || to == StatusCancelled || to == StatusTimedOut))
}

func (r WorkflowRun) validate() error {
	if r.ID == uuid.Nil || r.ProjectID == uuid.Nil || r.WorkflowConfigurationID == uuid.Nil || strings.TrimSpace(r.RunNumber) == "" || strings.TrimSpace(r.Stage) == "" || !validTriggerSource(r.TriggerSource) || r.Version < 1 || !validJSONObject(r.ConfigurationSnapshot) || !validJSONObject(r.InputPayload) || len(r.BindingSnapshot) > 0 && !validJSONObject(r.BindingSnapshot) || len(r.ConnectionSnapshot) > 0 && !validJSONObject(r.ConnectionSnapshot) || len(r.LlmPolicySnapshot) > 0 && !validJSONObject(r.LlmPolicySnapshot) {
		return ErrValidation
	}
	if (r.SubjectType == nil) != (r.SubjectID == nil) || (r.SubjectType != nil && (strings.TrimSpace(*r.SubjectType) == "" || *r.SubjectID == uuid.Nil)) {
		return ErrValidation
	}
	if r.OutputPayload != nil && !validJSONObject(r.OutputPayload) || r.ErrorDetails != nil && !validJSONObject(r.ErrorDetails) {
		return ErrValidation
	}
	if r.Status == StatusFailed || r.Status == StatusTimedOut {
		failureCode := r.FailureCode
		if failureCode == nil {
			failureCode = r.ErrorCode
		}
		safeMessage := r.SafeErrorMessage
		if safeMessage == nil {
			safeMessage = r.ErrorMessage
		}
		if failureCode == nil || safeMessage == nil || strings.TrimSpace(*failureCode) == "" || strings.TrimSpace(*safeMessage) == "" {
			return ErrValidation
		}
	}
	if r.Status != StatusQueued && r.Status != StatusRunning && r.Status != StatusCancelling && r.Status != StatusSucceeded && r.Status != StatusFailed && r.Status != StatusCancelled && r.Status != StatusTimedOut {
		return ErrValidation
	}
	if r.Retryability != "" && r.Retryability != "runtime_retry" && r.Retryability != "result_consumption_retry" && r.Retryability != "not_retryable" {
		return ErrValidation
	}
	if r.RetryMode != nil && *r.RetryMode != "current_configuration" && *r.RetryMode != "original_configuration" {
		return ErrValidation
	}
	if r.WorkflowConnectionID != nil && *r.WorkflowConnectionID == uuid.Nil || r.DeadlineAt != nil && r.DeadlineAt.Before(r.CreatedAt) {
		return ErrValidation
	}
	if r.CancellationReason != nil && *r.CancellationReason != "user" && *r.CancellationReason != "timeout" {
		return ErrValidation
	}
	return nil
}

func validTriggerSource(value string) bool {
	return value == "manual" || value == "retry" || value == "system" || value == "api"
}

func validJSONObject(value json.RawMessage) bool {
	return len(value) > 0 && json.Valid(value) && strings.HasPrefix(strings.TrimSpace(string(value)), "{")
}

// RedactJSON removes secret-bearing values before data becomes part of a durable run record.
func RedactJSON(value json.RawMessage) json.RawMessage {
	if !validJSONObject(value) {
		return value
	}
	return safety.RedactJSON(value)
}

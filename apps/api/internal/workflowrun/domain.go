package workflowrun

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

var (
	ErrValidation        = errors.New("workflow run validation failed")
	ErrInvalidTransition = errors.New("invalid workflow run status transition")
	ErrNotFound          = errors.New("workflow run not found")
	ErrVersionConflict   = errors.New("workflow run version conflict")
)

type Failure struct {
	Code    string
	Message string
	Details json.RawMessage
}

type WorkflowRun struct {
	ID                      uuid.UUID
	RunNumber               string
	ProjectID               uuid.UUID
	Stage                   string
	WorkflowConfigurationID uuid.UUID
	TriggerSource           string
	Status                  Status
	ConfigurationSnapshot   json.RawMessage
	InputPayload            json.RawMessage
	OutputPayload           json.RawMessage
	ErrorCode               *string
	ErrorMessage            *string
	ErrorDetails            json.RawMessage
	IdempotencyKey          *string
	RetryOfRunID            *uuid.UUID
	StartedAt               *time.Time
	FinishedAt              *time.Time
	CancelledAt             *time.Time
	CreatedAt               time.Time
	UpdatedAt               time.Time
	Version                 int
}

type Event struct {
	ID        uuid.UUID
	RunID     uuid.UUID
	EventType string
	Status    Status
	Payload   json.RawMessage
	CreatedAt time.Time
}

func New(id, projectID, workflowConfigurationID uuid.UUID, runNumber, stage, triggerSource string, snapshot, input json.RawMessage, idempotencyKey *string) (WorkflowRun, error) {
	now := time.Now().UTC()
	run := WorkflowRun{ID: id, RunNumber: runNumber, ProjectID: projectID, Stage: stage, WorkflowConfigurationID: workflowConfigurationID, TriggerSource: triggerSource, Status: StatusQueued, ConfigurationSnapshot: RedactJSON(snapshot), InputPayload: RedactJSON(input), IdempotencyKey: trimOptional(idempotencyKey), CreatedAt: now, UpdatedAt: now, Version: 1}
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
func (r WorkflowRun) Fail(at time.Time, failure Failure) (WorkflowRun, error) {
	if strings.TrimSpace(failure.Code) == "" || strings.TrimSpace(failure.Message) == "" || !validJSONObject(failure.Details) {
		return WorkflowRun{}, ErrValidation
	}
	return r.transition(StatusFailed, at.UTC(), nil, &failure)
}
func (r WorkflowRun) Cancel(at time.Time) (WorkflowRun, error) {
	return r.transition(StatusCancelled, at.UTC(), nil, nil)
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
	case StatusFailed:
		code, message := strings.TrimSpace(failure.Code), strings.TrimSpace(failure.Message)
		r.ErrorCode, r.ErrorMessage, r.ErrorDetails, r.FinishedAt = &code, &message, RedactJSON(failure.Details), &at
	case StatusCancelled:
		r.CancelledAt = &at
	}
	return r, nil
}

func canTransition(from, to Status) bool {
	return (from == StatusQueued && (to == StatusRunning || to == StatusCancelled)) || (from == StatusRunning && (to == StatusSucceeded || to == StatusFailed || to == StatusCancelled))
}

func (r WorkflowRun) validate() error {
	if r.ID == uuid.Nil || r.ProjectID == uuid.Nil || r.WorkflowConfigurationID == uuid.Nil || strings.TrimSpace(r.RunNumber) == "" || strings.TrimSpace(r.Stage) == "" || strings.TrimSpace(r.TriggerSource) == "" || r.Version < 1 || !validJSONObject(r.ConfigurationSnapshot) || !validJSONObject(r.InputPayload) {
		return ErrValidation
	}
	if r.OutputPayload != nil && !validJSONObject(r.OutputPayload) || r.ErrorDetails != nil && !validJSONObject(r.ErrorDetails) {
		return ErrValidation
	}
	if r.Status == StatusFailed && (r.ErrorCode == nil || r.ErrorMessage == nil || strings.TrimSpace(*r.ErrorCode) == "" || strings.TrimSpace(*r.ErrorMessage) == "") {
		return ErrValidation
	}
	if r.Status != StatusQueued && r.Status != StatusRunning && r.Status != StatusSucceeded && r.Status != StatusFailed && r.Status != StatusCancelled {
		return ErrValidation
	}
	return nil
}

func validJSONObject(value json.RawMessage) bool {
	return len(value) > 0 && json.Valid(value) && strings.HasPrefix(strings.TrimSpace(string(value)), "{")
}
func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// RedactJSON removes secret-bearing values before data becomes part of a durable run record.
func RedactJSON(value json.RawMessage) json.RawMessage {
	if !validJSONObject(value) {
		return value
	}
	var payload any
	if err := json.Unmarshal(value, &payload); err != nil {
		return value
	}
	redactValue(payload)
	redacted, err := json.Marshal(payload)
	if err != nil {
		return value
	}
	return redacted
}
func redactValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if isSensitiveKey(key) {
				typed[key] = "[REDACTED]"
			} else {
				redactValue(child)
			}
		}
	case []any:
		for _, child := range typed {
			redactValue(child)
		}
	}
}
func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
	return strings.Contains(normalized, "secret") || strings.Contains(normalized, "credential") || strings.Contains(normalized, "password") || strings.Contains(normalized, "apikey") || strings.Contains(normalized, "authorization") || strings.Contains(normalized, "token")
}

package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrExecutorUnavailable    = errors.New("workflow executor unavailable")
	ErrExecutionTimeout       = errors.New("workflow execution timed out")
	ErrExecutionNotFound      = errors.New("workflow execution not found")
	ErrInvalidExecutionResult = errors.New("invalid workflow execution result")
	// Permanent output-contract failures (converged to failed, never infinite Query).
	ErrExecutionOutputMissing        = errors.New("execution output missing")
	ErrExecutionOutputMultipleItems  = errors.New("execution output multiple items")
	ErrExecutionOutputInvalidShape   = errors.New("execution output invalid shape")
	ErrExecutionResultTooLarge       = errors.New("execution result too large")
)

// PermanentExecutionOutputCodes are durable business failures for successful
// upstream executions whose payload violates the frozen single-item contract.
var PermanentExecutionOutputCodes = map[string]struct{}{
	"execution_output_missing":         {},
	"execution_output_multiple_items":  {},
	"execution_output_invalid_shape":   {},
	"execution_result_too_large":       {},
}

// IsPermanentExecutionOutputError reports whether err is a frozen output-contract failure.
func IsPermanentExecutionOutputError(err error) bool {
	return errors.Is(err, ErrExecutionOutputMissing) ||
		errors.Is(err, ErrExecutionOutputMultipleItems) ||
		errors.Is(err, ErrExecutionOutputInvalidShape) ||
		errors.Is(err, ErrExecutionResultTooLarge)
}

// PermanentExecutionOutputCode maps a permanent output error to a stable code.
func PermanentExecutionOutputCode(err error) string {
	switch {
	case errors.Is(err, ErrExecutionOutputMissing):
		return "execution_output_missing"
	case errors.Is(err, ErrExecutionOutputMultipleItems):
		return "execution_output_multiple_items"
	case errors.Is(err, ErrExecutionOutputInvalidShape):
		return "execution_output_invalid_shape"
	case errors.Is(err, ErrExecutionResultTooLarge):
		return "execution_result_too_large"
	default:
		return ""
	}
}

type ExecutionStatus string

const (
	ExecutionAccepted  ExecutionStatus = "accepted"
	ExecutionRunning   ExecutionStatus = "running"
	ExecutionSucceeded ExecutionStatus = "succeeded"
	ExecutionFailed    ExecutionStatus = "failed"
	ExecutionCancelled ExecutionStatus = "cancelled"
)

type ExecutionRequest struct {
	RunID, ProjectID, WorkflowConfigurationID, WorkflowConnectionID uuid.UUID
	Stage                                                           string
	ConfigurationSnapshot, Input, Parameters                        json.RawMessage
	Metadata                                                        map[string]string
	Timeout                                                         time.Duration
	CorrelationID                                                   string
	ExternalExecutionID                                             string
}

type ExecutionResult struct {
	Status                  ExecutionStatus
	ExternalExecutionID     string
	Output                  json.RawMessage
	ErrorCode, ErrorMessage string
	Metadata                map[string]string
}

// WorkflowExecutor isolates WorkflowRun from any particular execution platform.
// Implementations do not persist runs or events; the application service owns both.
type WorkflowExecutor interface {
	Verify(context.Context, ExecutionRequest) error
	Execute(context.Context, ExecutionRequest) (ExecutionResult, error)
	Cancel(context.Context, ExecutionRequest) (ExecutionResult, error)
	Query(context.Context, ExecutionRequest) (ExecutionResult, error)
}

type FakeWorkflowExecutor struct {
	VerifyError, ExecuteError, CancelError, QueryError error
	ExecuteResult, CancelResult, QueryResult           ExecutionResult
	VerifyCalls, ExecuteCalls, CancelCalls, QueryCalls int
	LastRequest                                        ExecutionRequest
}

func (f *FakeWorkflowExecutor) Verify(_ context.Context, request ExecutionRequest) error {
	f.VerifyCalls++
	f.LastRequest = request
	return f.VerifyError
}
func (f *FakeWorkflowExecutor) Execute(_ context.Context, request ExecutionRequest) (ExecutionResult, error) {
	f.ExecuteCalls++
	f.LastRequest = request
	return f.ExecuteResult, f.ExecuteError
}
func (f *FakeWorkflowExecutor) Cancel(_ context.Context, request ExecutionRequest) (ExecutionResult, error) {
	f.CancelCalls++
	f.LastRequest = request
	return f.CancelResult, f.CancelError
}
func (f *FakeWorkflowExecutor) Query(_ context.Context, request ExecutionRequest) (ExecutionResult, error) {
	f.QueryCalls++
	f.LastRequest = request
	return f.QueryResult, f.QueryError
}

type UnavailableWorkflowExecutor struct{}

func (UnavailableWorkflowExecutor) Verify(context.Context, ExecutionRequest) error {
	return ErrExecutorUnavailable
}
func (UnavailableWorkflowExecutor) Execute(context.Context, ExecutionRequest) (ExecutionResult, error) {
	return ExecutionResult{}, ErrExecutorUnavailable
}
func (UnavailableWorkflowExecutor) Cancel(context.Context, ExecutionRequest) (ExecutionResult, error) {
	return ExecutionResult{}, ErrExecutorUnavailable
}
func (UnavailableWorkflowExecutor) Query(context.Context, ExecutionRequest) (ExecutionResult, error) {
	return ExecutionResult{}, ErrExecutorUnavailable
}

func validExecutionResult(result ExecutionResult) bool {
	if result.Status != ExecutionAccepted && result.Status != ExecutionRunning && result.Status != ExecutionSucceeded && result.Status != ExecutionFailed && result.Status != ExecutionCancelled {
		return false
	}
	if result.Status == ExecutionSucceeded {
		return validJSONObject(result.Output)
	}
	if result.Status == ExecutionFailed {
		return strings.TrimSpace(result.ErrorCode) != "" && strings.TrimSpace(result.ErrorMessage) != ""
	}
	return true
}

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
	ErrExecutorUnavailable = errors.New("workflow executor unavailable")
	ErrExecutionTimeout = errors.New("workflow execution timed out")
	ErrInvalidExecutionResult = errors.New("invalid workflow execution result")
)

type ExecutionStatus string

const (
	ExecutionAccepted ExecutionStatus = "accepted"
	ExecutionRunning ExecutionStatus = "running"
	ExecutionSucceeded ExecutionStatus = "succeeded"
	ExecutionFailed ExecutionStatus = "failed"
	ExecutionCancelled ExecutionStatus = "cancelled"
)

type ExecutionRequest struct {
	RunID, ProjectID, WorkflowConfigurationID, WorkflowConnectionID uuid.UUID
	Stage string
	ConfigurationSnapshot, Input, Parameters json.RawMessage
	Metadata map[string]string
	Timeout time.Duration
	CorrelationID string
}

type ExecutionResult struct {
	Status ExecutionStatus
	ExternalExecutionID string
	Output json.RawMessage
	ErrorCode, ErrorMessage string
	Metadata map[string]string
}

// WorkflowExecutor isolates WorkflowRun from any particular execution platform.
// Implementations do not persist runs or events; the application service owns both.
type WorkflowExecutor interface {
	Verify(context.Context, ExecutionRequest) error
	Execute(context.Context, ExecutionRequest) (ExecutionResult, error)
	Cancel(context.Context, ExecutionRequest) (ExecutionResult, error)
}

type FakeWorkflowExecutor struct {
	VerifyError, ExecuteError, CancelError error
	ExecuteResult, CancelResult ExecutionResult
	VerifyCalls, ExecuteCalls, CancelCalls int
	LastRequest ExecutionRequest
}

func (f *FakeWorkflowExecutor) Verify(_ context.Context, request ExecutionRequest) error { f.VerifyCalls++; f.LastRequest = request; return f.VerifyError }
func (f *FakeWorkflowExecutor) Execute(_ context.Context, request ExecutionRequest) (ExecutionResult, error) { f.ExecuteCalls++; f.LastRequest = request; return f.ExecuteResult, f.ExecuteError }
func (f *FakeWorkflowExecutor) Cancel(_ context.Context, request ExecutionRequest) (ExecutionResult, error) { f.CancelCalls++; f.LastRequest = request; return f.CancelResult, f.CancelError }

type UnavailableWorkflowExecutor struct{}
func (UnavailableWorkflowExecutor) Verify(context.Context, ExecutionRequest) error { return ErrExecutorUnavailable }
func (UnavailableWorkflowExecutor) Execute(context.Context, ExecutionRequest) (ExecutionResult, error) { return ExecutionResult{}, ErrExecutorUnavailable }
func (UnavailableWorkflowExecutor) Cancel(context.Context, ExecutionRequest) (ExecutionResult, error) { return ExecutionResult{}, ErrExecutorUnavailable }

func validExecutionResult(result ExecutionResult) bool {
	if result.Status != ExecutionAccepted && result.Status != ExecutionRunning && result.Status != ExecutionSucceeded && result.Status != ExecutionFailed && result.Status != ExecutionCancelled { return false }
	if result.Status == ExecutionSucceeded { return validJSONObject(result.Output) }
	if result.Status == ExecutionFailed { return strings.TrimSpace(result.ErrorCode) != "" && strings.TrimSpace(result.ErrorMessage) != "" }
	return true
}

package workflowrun

import (
	"context"

	"github.com/google/uuid"
)

// ChapterPlanningRuntime is the narrow runtime boundary required by chapter
// planning. Protected stages must use the explicit idempotent scope operation
// instead of the generic CreateRun entry point.
type ChapterPlanningRuntime interface {
	CreateRunIdempotentForScope(context.Context, string, uuid.UUID, string, string, CreateRunPreparation) (WorkflowRun, error)
	RetryResultConsumption(context.Context, uuid.UUID, int) (WorkflowRun, error)
}

// RuntimeBridge is the only runtime boundary used by the four domain stages.
// It deliberately exposes existing WorkflowRun operations only: execution and
// result consumption remain separate concerns owned by the runtime and domain.
type RuntimeBridge interface {
	ChapterPlanningRuntime
	CreateRun(context.Context, CreateRunCommand) (WorkflowRun, error)
	CreateRunForPreflightToken(context.Context, uuid.UUID, string, string, CreateRunPreparation) (WorkflowRun, error)
	CreateRunForPreflightTokenIdempotent(context.Context, uuid.UUID, string, string, string, CreateRunTxPreparation) (WorkflowRun, error)
	CreateRunForPreflightTokenIdempotentForScope(context.Context, string, uuid.UUID, string, string, string, CreateRunTxPreparation) (WorkflowRun, error)
	CreateRunForPreflightTokenIdempotentForScopeWithReplay(context.Context, string, uuid.UUID, string, string, string, CreateRunTxPreparation) (WorkflowRun, bool, error)
	ListRuns(context.Context, ListRunsQuery) (RunList, error)
	ListRunEvents(context.Context, uuid.UUID) ([]Event, error)
	GetRun(context.Context, uuid.UUID) (WorkflowRun, error)
	AddEvent(context.Context, Event) (Event, error)
	RetryResultConsumption(context.Context, uuid.UUID, int) (WorkflowRun, error)
}

type bridge struct{ runtime *Service }

func NewRuntimeBridge(runtime *Service) RuntimeBridge { return bridge{runtime: runtime} }

func (b bridge) CreateRun(ctx context.Context, command CreateRunCommand) (WorkflowRun, error) {
	return b.runtime.CreateRun(ctx, command)
}
func (b bridge) CreateRunIdempotentForScope(ctx context.Context, operation string, projectID uuid.UUID, key, requestHash string, prepare CreateRunPreparation) (WorkflowRun, error) {
	return b.runtime.CreateRunIdempotentForScope(ctx, operation, projectID, key, requestHash, prepare)
}
func (b bridge) CreateRunForPreflightToken(ctx context.Context, projectID uuid.UUID, nonce, requestHash string, prepare CreateRunPreparation) (WorkflowRun, error) {
	return b.runtime.CreateRunForPreflightToken(ctx, projectID, nonce, requestHash, prepare)
}
func (b bridge) CreateRunForPreflightTokenIdempotent(ctx context.Context, projectID uuid.UUID, key, requestHash, nonce string, prepare CreateRunTxPreparation) (WorkflowRun, error) {
	return b.runtime.CreateRunForPreflightTokenIdempotent(ctx, projectID, key, requestHash, nonce, prepare)
}
func (b bridge) CreateRunForPreflightTokenIdempotentForScope(ctx context.Context, operation string, projectID uuid.UUID, key, requestHash, nonce string, prepare CreateRunTxPreparation) (WorkflowRun, error) {
	return b.runtime.CreateRunForPreflightTokenIdempotentForScope(ctx, operation, projectID, key, requestHash, nonce, prepare)
}
func (b bridge) CreateRunForPreflightTokenIdempotentForScopeWithReplay(ctx context.Context, operation string, projectID uuid.UUID, key, requestHash, nonce string, prepare CreateRunTxPreparation) (WorkflowRun, bool, error) {
	return b.runtime.CreateRunForPreflightTokenIdempotentForScopeWithReplay(ctx, operation, projectID, key, requestHash, nonce, prepare)
}
func (b bridge) ListRuns(ctx context.Context, query ListRunsQuery) (RunList, error) {
	return b.runtime.ListRuns(ctx, query)
}
func (b bridge) ListRunEvents(ctx context.Context, id uuid.UUID) ([]Event, error) {
	return b.runtime.ListRunEvents(ctx, id)
}
func (b bridge) GetRun(ctx context.Context, id uuid.UUID) (WorkflowRun, error) {
	return b.runtime.GetRun(ctx, id)
}
func (b bridge) AddEvent(ctx context.Context, event Event) (Event, error) {
	return b.runtime.AddEvent(ctx, event)
}
func (b bridge) RetryResultConsumption(ctx context.Context, id uuid.UUID, expectedVersion int) (WorkflowRun, error) {
	return b.runtime.RetryResultConsumption(ctx, id, expectedVersion)
}

var _ RuntimeBridge = bridge{}
var _ ChapterPlanningRuntime = bridge{}

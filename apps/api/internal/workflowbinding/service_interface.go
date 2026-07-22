package workflowbinding

import (
	"context"

	"github.com/google/uuid"
)

// BindingService is the interface the HTTP handlers depend on.  It exposes the
// idempotent PUT / DELETE entry points (which return the cached HTTP status
// code alongside the result) and the read-only GET.
type BindingService interface {
	ListStages(ctx context.Context, projectID uuid.UUID) ([]StageRead, error)
	PutWithIdempotency(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, req PutRequest, key string) (PutResult, int, error)
	DeleteWithIdempotency(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, req DeleteRequest, key string) (UnbindResult, int, error)
}

// Compile-time check that the concrete Service satisfies the interface.
var _ BindingService = (*Service)(nil)

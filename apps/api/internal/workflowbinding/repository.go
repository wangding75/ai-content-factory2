package workflowbinding

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists ProjectWorkflowBinding records and abstracts the
// optimistic-locking contract from the domain Service.  It embeds the shared
// queryer interface so callers can pass a *Repository directly to the shared
// idempotency / audit helpers (which accept pgx.Tx-backed queryers).
type Repository struct{ queryer }

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func queryRows(db queryer, ctx context.Context, q string, args ...any) (pgx.Rows, error) {
	return db.Query(ctx, q, args...)
}

func NewPostgresRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{queryer: pool}
}

func NewPostgresRepositoryTx(tx pgx.Tx) *Repository { return &Repository{queryer: tx} }

const bindingColumns = "id, project_id, stage, workflow_configuration_id, version, created_at, updated_at"

func scanBinding(row pgx.Row) (ProjectWorkflowBinding, error) {
	var b ProjectWorkflowBinding
	if err := row.Scan(&b.ID, &b.ProjectID, &b.Stage, &b.WorkflowConfigurationID, &b.Version, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return ProjectWorkflowBinding{}, err
	}
	validated, err := NewFromDB(b.ID, b.ProjectID, b.WorkflowConfigurationID, b.Stage, b.Version, b.CreatedAt, b.UpdatedAt)
	if err != nil {
		return ProjectWorkflowBinding{}, fmt.Errorf("rebuild binding from db: %w", err)
	}
	return validated, nil
}

// ListByProject returns every binding for a project in the fixed stage order:
// chapter_planning, content_generation, review, rewrite.
func (r *Repository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]ProjectWorkflowBinding, error) {
	rows, err := queryRows(r.queryer, ctx, "SELECT "+bindingColumns+" FROM project_workflow_bindings WHERE project_id=$1 ORDER BY CASE stage WHEN 'chapter_planning' THEN 0 WHEN 'content_generation' THEN 1 WHEN 'review' THEN 2 WHEN 'rewrite' THEN 3 END, id ASC", projectID)
	if err != nil {
		return nil, fmt.Errorf("list project workflow bindings: %w", err)
	}
	defer rows.Close()
	out := []ProjectWorkflowBinding{}
	for rows.Next() {
		b, err := scanBinding(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project workflow binding: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project workflow bindings: %w", err)
	}
	return out, nil
}

// GetByProjectAndStage returns the binding for a single project and stage.
// It returns ErrNotFound when no binding exists.
func (r *Repository) GetByProjectAndStage(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage) (ProjectWorkflowBinding, error) {
	b, err := scanBinding(r.queryer.QueryRow(ctx, "SELECT "+bindingColumns+" FROM project_workflow_bindings WHERE project_id=$1 AND stage=$2", projectID, stage.String()))
	if err != nil {
		if errorsIsNoRows(err) {
			return ProjectWorkflowBinding{}, ErrNotFound
		}
		return ProjectWorkflowBinding{}, fmt.Errorf("get project workflow binding: %w", err)
	}
	return b, nil
}

// Create inserts a new binding.  The UNIQUE(project_id, stage) constraint maps
// to ErrBindingAlreadyExists so concurrent first-time binds resolve to 409.
func (r *Repository) Create(ctx context.Context, b ProjectWorkflowBinding) (ProjectWorkflowBinding, error) {
	created, err := scanBinding(r.queryer.QueryRow(ctx, "INSERT INTO project_workflow_bindings (id, project_id, stage, workflow_configuration_id, version, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "+bindingColumns, b.ID, b.ProjectID, b.Stage.String(), b.WorkflowConfigurationID, b.Version, b.CreatedAt, b.UpdatedAt))
	if err != nil {
		if isUniqueViolation(err) {
			return ProjectWorkflowBinding{}, ErrBindingAlreadyExists
		}
		return ProjectWorkflowBinding{}, fmt.Errorf("create project workflow binding: %w", err)
	}
	return created, nil
}

// Replace atomically updates workflow_configuration_id and bumps the version
// in a single conditional statement keyed by (project_id, stage, expected
// version).  When the condition is not met, a probe in the same transaction
// distinguishes a missing binding (ErrNotFound) from a stale version
// (ErrVersionConflict carrying expected/current/projectId/stage context).
func (r *Repository) Replace(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, expectedVersion int, newWorkflowConfigurationID uuid.UUID) (ProjectWorkflowBinding, error) {
	updated, err := scanBinding(r.queryer.QueryRow(ctx, "UPDATE project_workflow_bindings SET workflow_configuration_id=$1, version=version+1, updated_at=NOW() WHERE project_id=$2 AND stage=$3 AND version=$4 RETURNING "+bindingColumns, newWorkflowConfigurationID, projectID, stage.String(), expectedVersion))
	if err == nil {
		return updated, nil
	}
	if !errorsIsNoRows(err) {
		return ProjectWorkflowBinding{}, fmt.Errorf("replace project workflow binding: %w", err)
	}
	return ProjectWorkflowBinding{}, r.probeConflict(ctx, projectID, stage, expectedVersion)
}

// Delete atomically removes the binding only when it exists with the expected
// version, returning the deleted record for audit.  It uses a single
// conditional DELETE ... RETURNING and probes missing-vs-conflict on miss.
// It never reads-then-deletes by id without a version guard.
func (r *Repository) Delete(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, expectedVersion int) (ProjectWorkflowBinding, error) {
	removed, err := scanBinding(r.queryer.QueryRow(ctx, "DELETE FROM project_workflow_bindings WHERE project_id=$1 AND stage=$2 AND version=$3 RETURNING "+bindingColumns, projectID, stage.String(), expectedVersion))
	if err == nil {
		return removed, nil
	}
	if !errorsIsNoRows(err) {
		return ProjectWorkflowBinding{}, fmt.Errorf("delete project workflow binding: %w", err)
	}
	return ProjectWorkflowBinding{}, r.probeConflict(ctx, projectID, stage, expectedVersion)
}

// probeConflict re-reads the current binding to classify a conditional miss.
// It returns ErrNotFound when the binding is absent and a VersionConflictError
// (wrapping ErrVersionConflict) when it exists with a different version.
func (r *Repository) probeConflict(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, expectedVersion int) error {
	current, err := r.GetByProjectAndStage(ctx, projectID, stage)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return fmt.Errorf("probe current binding: %w", err)
	}
	return &VersionConflictError{
		ProjectID:       projectID,
		Stage:           stage,
		ExpectedVersion: expectedVersion,
		CurrentVersion:  current.Version,
	}
}

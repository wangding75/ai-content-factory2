package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}
type Repository struct{ db queryer }

func NewPostgresRepository(pool *pgxpool.Pool) *Repository { return &Repository{db: pool} }
func NewPostgresRepositoryTx(tx pgx.Tx) *Repository        { return &Repository{db: tx} }

const runColumns = "id, run_number, project_id, stage, workflow_configuration_id, trigger_source, status, configuration_snapshot, input_payload, output_payload, error_code, error_message, error_details, idempotency_key, retry_of_run_id, started_at, finished_at, cancelled_at, created_at, updated_at, version"

type ListFilter struct {
	ProjectID                                                        *uuid.UUID
	Stage, WorkflowConfigurationID, Status, TriggerSource, RunNumber string
	Limit, Offset                                                    int
}
type Summary struct {
	TotalRuns, RunningCount  int
	LatestFailure, LatestRun *WorkflowRun
	RecentRuns               []WorkflowRun
}

func scanRun(row pgx.Row) (WorkflowRun, error) {
	var r WorkflowRun
	if err := row.Scan(&r.ID, &r.RunNumber, &r.ProjectID, &r.Stage, &r.WorkflowConfigurationID, &r.TriggerSource, &r.Status, &r.ConfigurationSnapshot, &r.InputPayload, &r.OutputPayload, &r.ErrorCode, &r.ErrorMessage, &r.ErrorDetails, &r.IdempotencyKey, &r.RetryOfRunID, &r.StartedAt, &r.FinishedAt, &r.CancelledAt, &r.CreatedAt, &r.UpdatedAt, &r.Version); err != nil {
		return WorkflowRun{}, err
	}
	return NewFromDB(r)
}
func scanEvent(row pgx.Row) (Event, error) {
	var e Event
	if err := row.Scan(&e.ID, &e.RunID, &e.EventType, &e.Status, &e.Payload, &e.CreatedAt); err != nil {
		return Event{}, err
	}
	if e.ID == uuid.Nil || e.RunID == uuid.Nil || e.EventType == "" || !validJSONObject(e.Payload) {
		return Event{}, ErrValidation
	}
	return e, nil
}

func (r *Repository) Create(ctx context.Context, value WorkflowRun) (WorkflowRun, error) {
	created, err := scanRun(r.db.QueryRow(ctx, "INSERT INTO workflow_run_records ("+runColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21) RETURNING "+runColumns, value.ID, value.RunNumber, value.ProjectID, value.Stage, value.WorkflowConfigurationID, value.TriggerSource, value.Status, value.ConfigurationSnapshot, value.InputPayload, nullableJSON(value.OutputPayload), value.ErrorCode, value.ErrorMessage, nullableJSON(value.ErrorDetails), value.IdempotencyKey, value.RetryOfRunID, value.StartedAt, value.FinishedAt, value.CancelledAt, value.CreatedAt, value.UpdatedAt, value.Version))
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("create workflow run: %w", err)
	}
	return created, nil
}
func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (WorkflowRun, error) {
	value, err := scanRun(r.db.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_run_records WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, ErrNotFound
	}
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("get workflow run: %w", err)
	}
	return value, nil
}
func (r *Repository) List(ctx context.Context, f ListFilter) ([]WorkflowRun, error) {
	q, args := "SELECT "+runColumns+" FROM workflow_run_records WHERE TRUE", []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		q += fmt.Sprintf(" AND "+clause, len(args))
	}
	if f.ProjectID != nil {
		add("project_id=$%d", *f.ProjectID)
	}
	if f.Stage != "" {
		add("stage=$%d", f.Stage)
	}
	if f.WorkflowConfigurationID != "" {
		add("workflow_configuration_id=$%d", f.WorkflowConfigurationID)
	}
	if f.Status != "" {
		add("status=$%d", f.Status)
	}
	if f.TriggerSource != "" {
		add("trigger_source=$%d", f.TriggerSource)
	}
	if f.RunNumber != "" {
		add("run_number ILIKE '%' || $%d || '%'", f.RunNumber)
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	args = append(args, limit)
	q += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", len(args))
	if f.Offset > 0 {
		args = append(args, f.Offset)
		q += fmt.Sprintf(" OFFSET $%d", len(args))
	}
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list workflow runs: %w", err)
	}
	defer rows.Close()
	out := []WorkflowRun{}
	for rows.Next() {
		v, e := scanRun(rows)
		if e != nil {
			return nil, fmt.Errorf("scan workflow run: %w", e)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow runs: %w", err)
	}
	return out, nil
}
func (r *Repository) UpdateStatus(ctx context.Context, value WorkflowRun) (WorkflowRun, error) {
	if _, err := NewFromDB(value); err != nil {
		return WorkflowRun{}, err
	}
	if value.Version < 2 {
		return WorkflowRun{}, ErrValidation
	}
	updated, err := scanRun(r.db.QueryRow(ctx, "UPDATE workflow_run_records SET status=$1, output_payload=$2, error_code=$3, error_message=$4, error_details=$5, started_at=$6, finished_at=$7, cancelled_at=$8, updated_at=$9, version=$10 WHERE id=$11 AND version=$12 RETURNING "+runColumns, value.Status, nullableJSON(value.OutputPayload), value.ErrorCode, value.ErrorMessage, nullableJSON(value.ErrorDetails), value.StartedAt, value.FinishedAt, value.CancelledAt, value.UpdatedAt, value.Version, value.ID, value.Version-1))
	if errors.Is(err, pgx.ErrNoRows) {
		existing, e := r.GetByID(ctx, value.ID)
		if errors.Is(e, ErrNotFound) {
			return WorkflowRun{}, ErrNotFound
		}
		if e != nil {
			return WorkflowRun{}, e
		}
		_ = existing
		return WorkflowRun{}, ErrVersionConflict
	}
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("update workflow run status: %w", err)
	}
	return updated, nil
}
func (r *Repository) AddEvent(ctx context.Context, value Event) (Event, error) {
	if value.ID == uuid.Nil || value.RunID == uuid.Nil || value.EventType == "" || !validJSONObject(value.Payload) {
		return Event{}, ErrValidation
	}
	value.Payload = RedactJSON(value.Payload)
	created, err := scanEvent(r.db.QueryRow(ctx, "INSERT INTO workflow_run_events (id,run_id,event_type,status,payload,created_at) VALUES ($1,$2,$3,$4,$5,$6) RETURNING id,run_id,event_type,status,payload,created_at", value.ID, value.RunID, value.EventType, value.Status, value.Payload, value.CreatedAt))
	if err != nil {
		return Event{}, fmt.Errorf("add workflow run event: %w", err)
	}
	return created, nil
}
func (r *Repository) QuerySummary(ctx context.Context, projectID uuid.UUID, recentLimit int) (Summary, error) {
	if recentLimit <= 0 || recentLimit > 20 {
		recentLimit = 5
	}
	var s Summary
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*), COUNT(*) FILTER (WHERE status='running') FROM workflow_run_records WHERE project_id=$1", projectID).Scan(&s.TotalRuns, &s.RunningCount); err != nil {
		return Summary{}, fmt.Errorf("query workflow run totals: %w", err)
	}
	latest, err := scanRun(r.db.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_run_records WHERE project_id=$1 ORDER BY created_at DESC,id DESC LIMIT 1", projectID))
	if err == nil {
		s.LatestRun = &latest
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Summary{}, fmt.Errorf("query latest workflow run: %w", err)
	}
	failure, err := scanRun(r.db.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_run_records WHERE project_id=$1 AND status='failed' ORDER BY finished_at DESC,id DESC LIMIT 1", projectID))
	if err == nil {
		s.LatestFailure = &failure
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Summary{}, fmt.Errorf("query latest failed workflow run: %w", err)
	}
	recent, err := r.List(ctx, ListFilter{ProjectID: &projectID, Limit: recentLimit})
	if err != nil {
		return Summary{}, err
	}
	s.RecentRuns = recent
	return s, nil
}
func nullableJSON(value json.RawMessage) any {
	if value == nil {
		return nil
	}
	return value
}

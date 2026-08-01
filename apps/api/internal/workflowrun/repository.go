package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
)

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}
type Repository struct {
	db   queryer
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *Repository { return &Repository{db: pool, pool: pool} }
func NewPostgresRepositoryTx(tx pgx.Tx) *Repository        { return &Repository{db: tx} }
func (r *Repository) Transaction() pgx.Tx {
	tx, _ := r.db.(pgx.Tx)
	return tx
}

const runColumns = "id, run_number, project_id, stage, subject_type, subject_id, workflow_configuration_id, trigger_source, status, configuration_snapshot, input_payload, output_payload, error_code, error_message, error_details, retry_of_run_id, failure_phase, failure_code, safe_error_message, retryability, retry_mode, external_execution_id, cancellation_requested_at, timed_out_at, binding_snapshot, connection_snapshot, llm_policy_snapshot, started_at, finished_at, cancelled_at, created_at, updated_at, version"

type ListFilter struct {
	ProjectID                                                        *uuid.UUID
	Stage, WorkflowConfigurationID, Status, TriggerSource, RunNumber string
	SubjectType                                                      *string
	SubjectID                                                        *uuid.UUID
	Query                                                            string
	StartTime, EndTime                                               *time.Time
	Limit, Offset                                                    int
}
type Summary struct {
	TotalRuns, ActiveRuns, RecentFailedRuns int
	LastRunAt                               *time.Time
	RecentRuns                              []WorkflowRun
	// Compatibility fields remain for the repository contract completed in CF-14-02A.
	RunningCount             int
	LatestFailure, LatestRun *WorkflowRun
}

func scanRun(row pgx.Row) (WorkflowRun, error) {
	var r WorkflowRun
	if err := row.Scan(&r.ID, &r.RunNumber, &r.ProjectID, &r.Stage, &r.SubjectType, &r.SubjectID, &r.WorkflowConfigurationID, &r.TriggerSource, &r.Status, &r.ConfigurationSnapshot, &r.InputPayload, &r.OutputPayload, &r.ErrorCode, &r.ErrorMessage, &r.ErrorDetails, &r.RetryOfRunID, &r.FailurePhase, &r.FailureCode, &r.SafeErrorMessage, &r.Retryability, &r.RetryMode, &r.ExternalExecutionID, &r.CancellationRequestedAt, &r.TimedOutAt, &r.BindingSnapshot, &r.ConnectionSnapshot, &r.LlmPolicySnapshot, &r.StartedAt, &r.FinishedAt, &r.CancelledAt, &r.CreatedAt, &r.UpdatedAt, &r.Version); err != nil {
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
	validated, err := NewFromDB(value)
	if err != nil {
		return WorkflowRun{}, err
	}
	value = normalizedPersistenceRun(validated)
	created, err := scanRun(r.db.QueryRow(ctx, "INSERT INTO workflow_run_records ("+runColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33) RETURNING "+runColumns, value.ID, value.RunNumber, value.ProjectID, value.Stage, value.SubjectType, value.SubjectID, value.WorkflowConfigurationID, value.TriggerSource, value.Status, value.ConfigurationSnapshot, value.InputPayload, nullableJSON(value.OutputPayload), value.ErrorCode, value.ErrorMessage, nullableJSON(value.ErrorDetails), value.RetryOfRunID, value.FailurePhase, value.FailureCode, value.SafeErrorMessage, value.Retryability, value.RetryMode, value.ExternalExecutionID, value.CancellationRequestedAt, value.TimedOutAt, value.BindingSnapshot, value.ConnectionSnapshot, value.LlmPolicySnapshot, value.StartedAt, value.FinishedAt, value.CancelledAt, value.CreatedAt, value.UpdatedAt, value.Version))
	if err != nil {
		var postgresError *pgconn.PgError
		if errors.As(err, &postgresError) && postgresError.ConstraintName == "workflow_run_records_active_rewrite_subject_idx" {
			return WorkflowRun{}, ErrActiveRewriteRun
		}
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
func (r *Repository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (WorkflowRun, error) {
	value, err := scanRun(r.db.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_run_records WHERE id=$1 FOR UPDATE", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, ErrNotFound
	}
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("lock workflow run: %w", err)
	}
	return value, nil
}
func (r *Repository) PreflightTokenUsed(ctx context.Context, nonce string) (bool, error) {
	var used bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_records WHERE input_payload->>'preflightTokenNonce'=$1)", nonce).Scan(&used)
	if err != nil {
		return false, fmt.Errorf("find workflow run preflight token: %w", err)
	}
	return used, nil
}
func (r *Repository) HasContentGenerationCandidate(ctx context.Context, runID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM content_versions WHERE source_workflow_run_id=$1)", runID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("find content generation candidate: %w", err)
	}
	return exists, nil
}
func (r *Repository) HasReviewReport(ctx context.Context, runID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM review_reports WHERE workflow_run_id=$1)", runID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("find review report: %w", err)
	}
	return exists, nil
}
func (r *Repository) HasRewriteCandidate(ctx context.Context, runID uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM content_versions WHERE source_workflow_run_id=$1 AND source='workflow_rewrite')", runID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("find rewrite candidate: %w", err)
	}
	return exists, nil
}

type rewriteRetryInput struct {
	SchemaVersion               string    `json:"schemaVersion"`
	ProjectID                   uuid.UUID `json:"projectId"`
	ContentItemID               uuid.UUID `json:"contentItemId"`
	SourceContentVersionID      uuid.UUID `json:"sourceContentVersionId"`
	SourceContentVersionVersion int       `json:"sourceContentVersionVersion"`
	ReviewReportID              uuid.UUID `json:"reviewReportId"`
	SelectedIssues              []struct {
		ReviewIssueID  uuid.UUID `json:"reviewIssueId"`
		ReviewReportID uuid.UUID `json:"reviewReportId"`
		Position       int       `json:"position"`
	} `json:"selectedIssues"`
}

func parseRewriteRetryInput(run WorkflowRun) (rewriteRetryInput, error) {
	var input rewriteRetryInput
	if json.Unmarshal(run.InputPayload, &input) != nil || input.SchemaVersion != "rewrite.input.v1" ||
		input.ProjectID != run.ProjectID || input.ContentItemID == uuid.Nil ||
		input.SourceContentVersionID == uuid.Nil || input.SourceContentVersionVersion < 1 ||
		run.SubjectType == nil || *run.SubjectType != "review_report" || run.SubjectID == nil ||
		input.ReviewReportID != *run.SubjectID || len(input.SelectedIssues) < 1 || len(input.SelectedIssues) > 50 {
		return rewriteRetryInput{}, ErrNotRetryable
	}
	seen := map[uuid.UUID]bool{}
	for _, issue := range input.SelectedIssues {
		if issue.ReviewIssueID == uuid.Nil || issue.ReviewReportID != input.ReviewReportID || seen[issue.ReviewIssueID] {
			return rewriteRetryInput{}, ErrNotRetryable
		}
		seen[issue.ReviewIssueID] = true
	}
	return input, nil
}

func (r *Repository) LockRewriteRetryScope(ctx context.Context, run WorkflowRun) error {
	input, err := parseRewriteRetryInput(run)
	if err != nil {
		return err
	}
	if _, err = r.db.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "content-item:"+input.ContentItemID.String()); err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "rewrite-active:"+run.ProjectID.String()+":"+input.ReviewReportID.String())
	return err
}

func (r *Repository) ValidateRewriteRetryRelations(ctx context.Context, run WorkflowRun) error {
	input, err := parseRewriteRetryInput(run)
	if err != nil {
		return err
	}
	var sourceItem uuid.UUID
	var sourceVersion int
	if err = r.db.QueryRow(ctx, "SELECT content_item_id,version FROM content_versions WHERE id=$1 FOR UPDATE", input.SourceContentVersionID).Scan(&sourceItem, &sourceVersion); err != nil {
		return ErrNotRetryable
	}
	if sourceItem != input.ContentItemID || sourceVersion != input.SourceContentVersionVersion {
		return ErrNotRetryable
	}
	var itemProject uuid.UUID
	if err = r.db.QueryRow(ctx, "SELECT project_id FROM content_items WHERE id=$1 FOR UPDATE", input.ContentItemID).Scan(&itemProject); err != nil || itemProject != run.ProjectID {
		return ErrNotRetryable
	}
	var reportProject, reportItem, reportSource uuid.UUID
	var reportStatus, provider string
	var schemaVersion *string
	if err = r.db.QueryRow(ctx, "SELECT project_id,content_item_id,content_version_id,status,provider_key,schema_version FROM review_reports WHERE id=$1 FOR UPDATE", input.ReviewReportID).Scan(&reportProject, &reportItem, &reportSource, &reportStatus, &provider, &schemaVersion); err != nil {
		return ErrNotRetryable
	}
	if reportProject != run.ProjectID || reportItem != input.ContentItemID ||
		reportSource != input.SourceContentVersionID || reportStatus != "completed" ||
		provider != "runtime" || schemaVersion == nil || *schemaVersion != "review.output.v1" {
		return ErrNotRetryable
	}
	issueIDs := make([]uuid.UUID, len(input.SelectedIssues))
	for i := range input.SelectedIssues {
		issueIDs[i] = input.SelectedIssues[i].ReviewIssueID
	}
	rows, err := r.db.Query(ctx, "SELECT id FROM review_findings WHERE review_id=$1 AND id=ANY($2) ORDER BY sort_order ASC,id ASC FOR UPDATE", input.ReviewReportID, issueIDs)
	if err != nil {
		return err
	}
	count := 0
	for rows.Next() {
		var issueID uuid.UUID
		if err = rows.Scan(&issueID); err != nil {
			rows.Close()
			return err
		}
		count++
	}
	rows.Close()
	if count != len(issueIDs) {
		return ErrNotRetryable
	}
	var active bool
	if err = r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_records WHERE project_id=$1 AND stage='rewrite' AND subject_type='review_report' AND subject_id=$2 AND status IN ('queued','running'))", run.ProjectID, input.ReviewReportID).Scan(&active); err != nil {
		return err
	}
	if active {
		return ErrActiveRewriteRun
	}
	return nil
}
func (r *Repository) FindActive(ctx context.Context, projectID uuid.UUID, stage string, subjectType string, subjectID uuid.UUID) (WorkflowRun, error) {
	value, err := scanRun(r.db.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_run_records WHERE project_id=$1 AND stage=$2 AND subject_type=$3 AND subject_id=$4 AND status IN ('queued','running') ORDER BY created_at DESC,id DESC LIMIT 1", projectID, stage, subjectType, subjectID))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, ErrNotFound
	}
	if err != nil {
		return WorkflowRun{}, fmt.Errorf("find active workflow run: %w", err)
	}
	return value, nil
}
func (r *Repository) List(ctx context.Context, f ListFilter) ([]WorkflowRun, error) {
	if f.StartTime != nil && f.EndTime != nil && f.StartTime.After(*f.EndTime) {
		return nil, ErrValidation
	}
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
		add("run_number=$%d", f.RunNumber)
	}
	if f.SubjectType != nil {
		add("subject_type=$%d", *f.SubjectType)
	}
	if f.SubjectID != nil {
		add("subject_id=$%d", *f.SubjectID)
	}
	if f.Query != "" {
		add("run_number ILIKE '%%' || $%d || '%%'", f.Query)
	}
	if f.StartTime != nil {
		add("created_at >= $%d", f.StartTime.UTC())
	}
	if f.EndTime != nil {
		add("created_at <= $%d", f.EndTime.UTC())
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
func (r *Repository) Count(ctx context.Context, f ListFilter) (int, error) {
	if f.StartTime != nil && f.EndTime != nil && f.StartTime.After(*f.EndTime) {
		return 0, ErrValidation
	}
	q, args := "SELECT COUNT(*) FROM workflow_run_records WHERE TRUE", []any{}
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
		add("run_number=$%d", f.RunNumber)
	}
	if f.SubjectType != nil {
		add("subject_type=$%d", *f.SubjectType)
	}
	if f.SubjectID != nil {
		add("subject_id=$%d", *f.SubjectID)
	}
	if f.Query != "" {
		add("run_number ILIKE '%%' || $%d || '%%'", f.Query)
	}
	if f.StartTime != nil {
		add("created_at >= $%d", f.StartTime.UTC())
	}
	if f.EndTime != nil {
		add("created_at <= $%d", f.EndTime.UTC())
	}
	var total int
	if err := r.db.QueryRow(ctx, q, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count workflow runs: %w", err)
	}
	return total, nil
}
func (r *Repository) UpdateStatus(ctx context.Context, value WorkflowRun) (WorkflowRun, error) {
	if _, err := NewFromDB(value); err != nil {
		return WorkflowRun{}, err
	}
	if value.Version < 2 {
		return WorkflowRun{}, ErrValidation
	}
	updated, err := scanRun(r.db.QueryRow(ctx, "UPDATE workflow_run_records SET status=$1, output_payload=$2, error_code=$3, error_message=$4, error_details=$5, failure_phase=$6, failure_code=$7, safe_error_message=$8, retryability=$9, retry_mode=$10, external_execution_id=$11, cancellation_requested_at=$12, timed_out_at=$13, started_at=$14, finished_at=$15, cancelled_at=$16, updated_at=$17, version=$18 WHERE id=$19 AND version=$20 RETURNING "+runColumns, value.Status, nullableJSON(value.OutputPayload), value.ErrorCode, value.ErrorMessage, nullableJSON(value.ErrorDetails), value.FailurePhase, value.FailureCode, value.SafeErrorMessage, normalizedPersistenceRun(value).Retryability, value.RetryMode, value.ExternalExecutionID, value.CancellationRequestedAt, value.TimedOutAt, value.StartedAt, value.FinishedAt, value.CancelledAt, value.UpdatedAt, value.Version, value.ID, value.Version-1))
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
func (r *Repository) ListEvents(ctx context.Context, runID uuid.UUID) ([]Event, error) {
	rows, err := r.db.Query(ctx, "SELECT id,run_id,event_type,status,payload,created_at FROM workflow_run_events WHERE run_id=$1 ORDER BY created_at ASC,id ASC", runID)
	if err != nil {
		return nil, fmt.Errorf("list workflow run events: %w", err)
	}
	defer rows.Close()
	events := []Event{}
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan workflow run event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflow run events: %w", err)
	}
	return events, nil
}
func (r *Repository) CreateWithInitialEvent(ctx context.Context, value WorkflowRun, event Event) (WorkflowRun, Event, error) {
	if event.RunID != value.ID || event.Status != StatusQueued {
		return WorkflowRun{}, Event{}, ErrValidation
	}
	if r.pool == nil {
		created, err := r.Create(ctx, value)
		if err != nil {
			return WorkflowRun{}, Event{}, err
		}
		createdEvent, err := r.AddEvent(ctx, event)
		return created, createdEvent, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return WorkflowRun{}, Event{}, fmt.Errorf("begin workflow run creation transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	txRepo := NewPostgresRepositoryTx(tx)
	created, err := txRepo.Create(ctx, value)
	if err != nil {
		return WorkflowRun{}, Event{}, err
	}
	createdEvent, err := txRepo.AddEvent(ctx, event)
	if err != nil {
		return WorkflowRun{}, Event{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return WorkflowRun{}, Event{}, fmt.Errorf("commit workflow run creation transaction: %w", err)
	}
	return created, createdEvent, nil
}
func (r *Repository) UpdateStatusWithEvent(ctx context.Context, current, next WorkflowRun, event Event) (WorkflowRun, Event, error) {
	if current.ID != next.ID || next.Version != current.Version+1 || !canTransition(current.Status, next.Status) || event.RunID != current.ID || event.Status != next.Status {
		return WorkflowRun{}, Event{}, ErrInvalidTransition
	}
	if _, err := NewFromDB(next); err != nil {
		return WorkflowRun{}, Event{}, err
	}
	if r.pool == nil {
		updated, err := r.UpdateStatus(ctx, next)
		if err != nil {
			return WorkflowRun{}, Event{}, err
		}
		createdEvent, err := r.AddEvent(ctx, event)
		return updated, createdEvent, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return WorkflowRun{}, Event{}, fmt.Errorf("begin workflow run status transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	txRepo := NewPostgresRepositoryTx(tx)
	updated, err := txRepo.UpdateStatus(ctx, next)
	if err != nil {
		return WorkflowRun{}, Event{}, err
	}
	createdEvent, err := txRepo.AddEvent(ctx, event)
	if err != nil {
		return WorkflowRun{}, Event{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return WorkflowRun{}, Event{}, fmt.Errorf("commit workflow run status transaction: %w", err)
	}
	return updated, createdEvent, nil
}

// ExecuteIdempotent serializes an operation/key pair and writes the business
// result and its replay record in one database transaction.  The shared
// idempotency table is therefore the sole durable source of replay state.
func (r *Repository) ExecuteIdempotent(ctx context.Context, scope, key, requestHash string, fn func(Store) (WorkflowRun, error)) (WorkflowRun, error) {
	run, _, err := r.ExecuteIdempotentWithReplay(ctx, scope, key, requestHash, fn)
	return run, err
}

func (r *Repository) ExecuteIdempotentWithReplay(ctx context.Context, scope, key, requestHash string, fn func(Store) (WorkflowRun, error)) (WorkflowRun, bool, error) {
	if r.pool == nil || scope == "" || key == "" || requestHash == "" {
		return WorkflowRun{}, false, ErrValidation
	}
	var tx pgx.Tx
	var err error
	if strings.HasPrefix(scope, "createContentGenerationRun:") || strings.HasPrefix(scope, "createContentRewriteRun:") {
		tx, err = r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	} else {
		tx, err = r.pool.Begin(ctx)
	}
	if err != nil {
		return WorkflowRun{}, false, fmt.Errorf("begin workflow run idempotency transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "idempotency:"+scope+":"+key); err != nil {
		return WorkflowRun{}, false, fmt.Errorf("lock workflow run idempotency request: %w", err)
	}
	idem := idempotency.NewPostgresRepositoryTx(tx)
	if strings.HasPrefix(scope, "consumeContentGenerationPreflightToken:") {
		used, usedErr := NewPostgresRepositoryTx(tx).PreflightTokenUsed(ctx, key)
		if usedErr != nil {
			return WorkflowRun{}, false, usedErr
		}
		if used {
			return WorkflowRun{}, false, ErrPreflightTokenConsumed
		}
	}
	if record, getErr := idem.GetForUpdate(ctx, scope, key); getErr == nil {
		if record.RequestHash != requestHash {
			return WorkflowRun{}, false, ErrIdempotencyConflict
		}
		var replay WorkflowRun
		if err := json.Unmarshal(record.ResponseBody, &replay); err != nil {
			return WorkflowRun{}, false, fmt.Errorf("decode workflow run idempotency replay: %w", err)
		}
		if replay.ID == uuid.Nil {
			return WorkflowRun{}, false, fmt.Errorf("decode workflow run idempotency replay: %w", ErrValidation)
		}
		return replay, true, nil
	} else if !errors.Is(getErr, idempotency.ErrNotFound) {
		return WorkflowRun{}, false, getErr
	}
	created, err := fn(NewPostgresRepositoryTx(tx))
	if err != nil {
		return WorkflowRun{}, false, err
	}
	body, err := json.Marshal(created)
	if err != nil {
		return WorkflowRun{}, false, fmt.Errorf("encode workflow run idempotency replay: %w", err)
	}
	status := 200
	if strings.Contains(scope, "createWorkflowRun") || strings.Contains(scope, "createContentGenerationRun") || strings.Contains(scope, "createContentReviewRun") || strings.Contains(scope, "createContentRewriteRun") || strings.Contains(scope, "retryWorkflowRun") {
		status = 201
	}
	if _, err = idem.Create(ctx, idempotency.Record{ID: uuid.New(), Scope: scope, Key: key, RequestHash: requestHash, ResponseStatus: status, ResponseBody: RedactJSON(body)}); err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			return WorkflowRun{}, false, ErrIdempotencyConflict
		}
		return WorkflowRun{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return WorkflowRun{}, false, fmt.Errorf("commit workflow run idempotency transaction: %w", err)
	}
	return created, false, nil
}
func (r *Repository) QuerySummary(ctx context.Context, projectID uuid.UUID, recentLimit int) (Summary, error) {
	if recentLimit <= 0 || recentLimit > 3 {
		recentLimit = 3
	}
	var s Summary
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*), COUNT(*) FILTER (WHERE status IN ('queued','running')), COUNT(*) FILTER (WHERE status='failed' AND created_at >= NOW() - INTERVAL '7 days'), MAX(created_at) FROM workflow_run_records WHERE project_id=$1", projectID).Scan(&s.TotalRuns, &s.ActiveRuns, &s.RecentFailedRuns, &s.LastRunAt); err != nil {
		return Summary{}, fmt.Errorf("query workflow run totals: %w", err)
	}
	s.RunningCount = s.ActiveRuns
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

func normalizedPersistenceRun(value WorkflowRun) WorkflowRun {
	if value.Retryability == "" {
		value.Retryability = "not_retryable"
	}
	if len(value.BindingSnapshot) == 0 {
		value.BindingSnapshot = json.RawMessage(`{}`)
	}
	if len(value.ConnectionSnapshot) == 0 {
		value.ConnectionSnapshot = json.RawMessage(`{}`)
	}
	if len(value.LlmPolicySnapshot) == 0 {
		value.LlmPolicySnapshot = json.RawMessage(`{}`)
	}
	value.BindingSnapshot = RedactJSON(value.BindingSnapshot)
	value.ConnectionSnapshot = RedactJSON(value.ConnectionSnapshot)
	value.LlmPolicySnapshot = RedactJSON(value.LlmPolicySnapshot)
	return value
}

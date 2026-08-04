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

const runColumns = "id, run_number, project_id, stage, subject_type, subject_id, workflow_configuration_id, trigger_source, status, configuration_snapshot, input_payload, output_payload, error_code, error_message, error_details, retry_of_run_id, failure_phase, failure_code, safe_error_message, retryability, retry_mode, external_execution_id, workflow_connection_id, deadline_at, cancellation_reason, cancellation_requested_at, timed_out_at, binding_snapshot, connection_snapshot, llm_policy_snapshot, started_at, finished_at, cancelled_at, created_at, updated_at, version"

func prefixedRunColumns(alias string) string {
	columns := strings.Split(runColumns, ", ")
	for i := range columns {
		columns[i] = alias + "." + columns[i]
	}
	return strings.Join(columns, ", ")
}

type ListFilter struct {
	ProjectID                                                                                                                      *uuid.UUID
	Stage, WorkflowConfigurationID, Status, DisplayStatus, ConnectionID, ProviderID, Model, Retryability, TriggerSource, RunNumber string
	ConfigurationVersion                                                                                                           int
	SubjectType                                                                                                                    *string
	SubjectID                                                                                                                      *uuid.UUID
	Query                                                                                                                          string
	StartTime, EndTime                                                                                                             *time.Time
	Limit, Offset                                                                                                                  int
}

func workflowRunFilterSQL(f ListFilter) (string, []any) {
	q, args := "", []any{}
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
	if f.DisplayStatus != "" {
		add("CASE WHEN status='failed' AND failure_phase IN ('output_validation','result_consumption') THEN failure_phase || '_failed' ELSE status::text END=$%d", f.DisplayStatus)
	}
	if f.ConnectionID != "" {
		add("COALESCE(NULLIF(connection_snapshot->>'id',''), configuration_snapshot->'workflowConnection'->>'id')=$%d", f.ConnectionID)
	}
	if f.ProviderID != "" {
		add("llm_policy_snapshot->>'providerId'=$%d", f.ProviderID)
	}
	if f.Model != "" {
		add("llm_policy_snapshot->>'model'=$%d", f.Model)
	}
	if f.ConfigurationVersion > 0 {
		add("(configuration_snapshot->'workflowConfiguration'->>'version')::integer=$%d", f.ConfigurationVersion)
	}
	if f.Retryability != "" {
		add("retryability=$%d", f.Retryability)
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
	return q, args
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
	if err := row.Scan(&r.ID, &r.RunNumber, &r.ProjectID, &r.Stage, &r.SubjectType, &r.SubjectID, &r.WorkflowConfigurationID, &r.TriggerSource, &r.Status, &r.ConfigurationSnapshot, &r.InputPayload, &r.OutputPayload, &r.ErrorCode, &r.ErrorMessage, &r.ErrorDetails, &r.RetryOfRunID, &r.FailurePhase, &r.FailureCode, &r.SafeErrorMessage, &r.Retryability, &r.RetryMode, &r.ExternalExecutionID, &r.WorkflowConnectionID, &r.DeadlineAt, &r.CancellationReason, &r.CancellationRequestedAt, &r.TimedOutAt, &r.BindingSnapshot, &r.ConnectionSnapshot, &r.LlmPolicySnapshot, &r.StartedAt, &r.FinishedAt, &r.CancelledAt, &r.CreatedAt, &r.UpdatedAt, &r.Version); err != nil {
		return WorkflowRun{}, err
	}
	return NewFromDB(r)
}
func scanEvent(row pgx.Row) (Event, error) {
	var e Event
	if err := row.Scan(&e.ID, &e.RunID, &e.EventType, &e.Status, &e.Payload, &e.CreatedAt, &e.Sequence); err != nil {
		return Event{}, err
	}
	if e.ID == uuid.Nil || e.RunID == uuid.Nil || e.EventType == "" || e.Sequence < 1 || !validJSONObject(e.Payload) {
		return Event{}, ErrValidation
	}
	return e, nil
}

func mapActiveConstraint(err error) error {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "23505" {
		return err
	}
	switch postgresError.ConstraintName {
	case "workflow_run_records_active_rewrite_subject_idx":
		return ErrActiveRewriteRun
	case "workflow_run_records_active_content_generation_subject_idx",
		"workflow_run_records_active_review_subject_idx",
		"workflow_run_records_active_chapter_planning_idx":
		return ErrActiveRewriteRun
	default:
		return err
	}
}

// allocateEventSequence must run inside a transaction that already holds the run
// row (or will not race with other writers). Sequences are unique and strictly
// increasing per run; gaps are allowed.
func (r *Repository) allocateEventSequence(ctx context.Context, runID uuid.UUID) (int64, error) {
	var sequence int64
	err := r.db.QueryRow(ctx, `UPDATE workflow_run_records
		SET next_event_sequence = next_event_sequence + 1
		WHERE id = $1
		RETURNING next_event_sequence - 1`, runID).Scan(&sequence)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("allocate workflow run event sequence: %w", err)
	}
	if sequence < 1 {
		return 0, ErrValidation
	}
	return sequence, nil
}

// AddEventTx inserts an event inside an existing transaction with a durable sequence.
// Domain consumers must use this instead of raw INSERT into workflow_run_events.
func AddEventTx(ctx context.Context, tx pgx.Tx, value Event) (Event, error) {
	if tx == nil {
		return Event{}, ErrValidation
	}
	return NewPostgresRepositoryTx(tx).insertEvent(ctx, value)
}

func (r *Repository) insertEvent(ctx context.Context, value Event) (Event, error) {
	if value.ID == uuid.Nil || value.RunID == uuid.Nil || value.EventType == "" || !validJSONObject(value.Payload) {
		return Event{}, ErrValidation
	}
	value.Payload = RedactJSON(value.Payload)
	value.CreatedAt = NormalizeTimestamp(value.CreatedAt)
	if value.Sequence < 1 {
		sequence, err := r.allocateEventSequence(ctx, value.RunID)
		if err != nil {
			return Event{}, err
		}
		value.Sequence = sequence
	}
	created, err := scanEvent(r.db.QueryRow(ctx, `INSERT INTO workflow_run_events (id,run_id,event_type,status,payload,created_at,sequence)
		SELECT $1,$2,$3,$4,$5, GREATEST($6::timestamptz, r.created_at), $7
		FROM workflow_run_records r WHERE r.id=$2
		RETURNING id,run_id,event_type,status,payload,created_at,sequence`, value.ID, value.RunID, value.EventType, value.Status, value.Payload, value.CreatedAt, value.Sequence))
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("add workflow run event: %w", err)
	}
	return created, nil
}

func (r *Repository) Create(ctx context.Context, value WorkflowRun) (WorkflowRun, error) {
	validated, err := NewFromDB(value)
	if err != nil {
		return WorkflowRun{}, err
	}
	value = normalizedPersistenceRun(validated)
	value.CreatedAt = NormalizeTimestamp(value.CreatedAt)
	value.UpdatedAt = NormalizeTimestamp(value.UpdatedAt)
	created, err := scanRun(r.db.QueryRow(ctx, "INSERT INTO workflow_run_records ("+runColumns+") VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33,$34,$35,$36) RETURNING "+runColumns, value.ID, value.RunNumber, value.ProjectID, value.Stage, value.SubjectType, value.SubjectID, value.WorkflowConfigurationID, value.TriggerSource, value.Status, value.ConfigurationSnapshot, value.InputPayload, nullableJSON(value.OutputPayload), value.ErrorCode, value.ErrorMessage, nullableJSON(value.ErrorDetails), value.RetryOfRunID, value.FailurePhase, value.FailureCode, value.SafeErrorMessage, value.Retryability, value.RetryMode, value.ExternalExecutionID, value.WorkflowConnectionID, value.DeadlineAt, value.CancellationReason, value.CancellationRequestedAt, value.TimedOutAt, value.BindingSnapshot, value.ConnectionSnapshot, value.LlmPolicySnapshot, value.StartedAt, value.FinishedAt, value.CancelledAt, value.CreatedAt, value.UpdatedAt, value.Version))
	if err != nil {
		if mapped := mapActiveConstraint(err); !errors.Is(mapped, err) {
			return WorkflowRun{}, mapped
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
	if err = r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_records WHERE project_id=$1 AND stage='rewrite' AND subject_type='review_report' AND subject_id=$2 AND status IN ('queued','running','cancelling'))", run.ProjectID, input.ReviewReportID).Scan(&active); err != nil {
		return err
	}
	if active {
		return ErrActiveRewriteRun
	}
	return nil
}
func (r *Repository) FindActive(ctx context.Context, projectID uuid.UUID, stage string, subjectType string, subjectID uuid.UUID) (WorkflowRun, error) {
	value, err := scanRun(r.db.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_run_records WHERE project_id=$1 AND stage=$2 AND subject_type=$3 AND subject_id=$4 AND status IN ('queued','running','cancelling') ORDER BY created_at DESC,id DESC LIMIT 1", projectID, stage, subjectType, subjectID))
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
	where, args := workflowRunFilterSQL(f)
	q := "SELECT " + runColumns + " FROM workflow_run_records WHERE TRUE" + where
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
func (r *Repository) ListRecoverableResultConsumptions(ctx context.Context, limit int, now time.Time) ([]WorkflowRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, "SELECT "+prefixedRunColumns("r")+` FROM workflow_run_records r
		JOIN workflow_run_result_consumptions c ON c.workflow_run_id=r.id
		WHERE r.output_payload IS NOT NULL
		  AND ((r.status='running' AND c.status='pending') OR
		       (r.status='failed' AND r.failure_phase='result_consumption' AND c.status='failed') OR
		       (c.status='in_progress' AND (c.lease_until IS NULL OR c.lease_until <= $1)
		        AND (r.status='running' OR (r.status='failed' AND r.failure_phase='result_consumption'))))
		ORDER BY c.updated_at ASC, r.id ASC LIMIT $2`, now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list recoverable result consumptions: %w", err)
	}
	defer rows.Close()
	out := make([]WorkflowRun, 0)
	for rows.Next() {
		value, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan recoverable result consumption: %w", scanErr)
		}
		out = append(out, value)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recoverable result consumptions: %w", err)
	}
	return out, nil
}

// ListWorkerCandidates returns the oldest processable Active Runs for workers.
// It never reuses the UI List path (newest-first, status-filtered limit 100).
// Running rows without an external execution id only appear after the grace
// window so they cannot permanently occupy the candidate set.
func (r *Repository) ListWorkerCandidates(ctx context.Context, limit int, now time.Time) ([]WorkflowRun, error) {
	if limit <= 0 || limit > workerCandidateBatchLimit {
		limit = workerCandidateBatchLimit
	}
	now = NormalizeTimestamp(now)
	graceBefore := now.Add(-missingExternalIDGracePeriod)
	rows, err := r.db.Query(ctx, "SELECT "+runColumns+` FROM workflow_run_records
		WHERE status IN ('queued', 'cancelling')
		   OR (
		        status = 'running'
		        AND output_payload IS NULL
		        AND (
		            (external_execution_id IS NOT NULL AND btrim(external_execution_id) <> '')
		            OR (
		                (external_execution_id IS NULL OR btrim(external_execution_id) = '')
		                AND COALESCE(started_at, updated_at) <= $1
		            )
		        )
		   )
		ORDER BY created_at ASC, id ASC
		LIMIT $2`, graceBefore, limit)
	if err != nil {
		return nil, fmt.Errorf("list worker candidates: %w", err)
	}
	defer rows.Close()
	out := make([]WorkflowRun, 0, limit)
	for rows.Next() {
		value, scanErr := scanRun(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan worker candidate: %w", scanErr)
		}
		out = append(out, value)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worker candidates: %w", err)
	}
	return out, nil
}
func (r *Repository) Count(ctx context.Context, f ListFilter) (int, error) {
	if f.StartTime != nil && f.EndTime != nil && f.StartTime.After(*f.EndTime) {
		return 0, ErrValidation
	}
	where, args := workflowRunFilterSQL(f)
	q := "SELECT COUNT(*) FROM workflow_run_records WHERE TRUE" + where
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
	value.UpdatedAt = NormalizeTimestamp(value.UpdatedAt)
	// Persist updated_at with a lower bound so a skewed service clock cannot reverse durable time.
	updated, err := scanRun(r.db.QueryRow(ctx, "UPDATE workflow_run_records SET status=$1, output_payload=$2, error_code=$3, error_message=$4, error_details=$5, failure_phase=$6, failure_code=$7, safe_error_message=$8, retryability=$9, retry_mode=$10, external_execution_id=$11, cancellation_reason=$12, cancellation_requested_at=$13, timed_out_at=$14, started_at=$15, finished_at=$16, cancelled_at=$17, updated_at=GREATEST($18::timestamptz, created_at, updated_at), version=$19 WHERE id=$20 AND version=$21 RETURNING "+runColumns, value.Status, nullableJSON(value.OutputPayload), value.ErrorCode, value.ErrorMessage, nullableJSON(value.ErrorDetails), value.FailurePhase, value.FailureCode, value.SafeErrorMessage, normalizedPersistenceRun(value).Retryability, value.RetryMode, value.ExternalExecutionID, value.CancellationReason, value.CancellationRequestedAt, value.TimedOutAt, value.StartedAt, value.FinishedAt, value.CancelledAt, value.UpdatedAt, value.Version, value.ID, value.Version-1))
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

// RecordExecutionStartedAtomic persists external_execution_id and the
// execution_started event in one transaction. Same-ID retries are idempotent and
// repair a missing event; a different ID is a stable conflict; terminal Runs refuse write-back.
func (r *Repository) RecordExecutionStartedAtomic(ctx context.Context, runID uuid.UUID, externalID string, event Event, at time.Time) (WorkflowRun, Event, error) {
	externalID = strings.TrimSpace(externalID)
	if r.pool == nil || runID == uuid.Nil || externalID == "" || event.RunID != runID || event.EventType != "execution_started" {
		return WorkflowRun{}, Event{}, ErrValidation
	}
	at = NormalizeTimestamp(at)
	event.CreatedAt = at
	event.Payload = RedactJSON(event.Payload)
	if !validJSONObject(event.Payload) {
		event.Payload = mustSafeJSON(map[string]any{"externalExecutionId": externalID})
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return WorkflowRun{}, Event{}, fmt.Errorf("begin execution started transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	txRepo := NewPostgresRepositoryTx(tx)
	run, err := txRepo.GetByIDForUpdate(ctx, runID)
	if err != nil {
		return WorkflowRun{}, Event{}, err
	}
	if run.Status != StatusRunning && run.Status != StatusCancelling {
		return WorkflowRun{}, Event{}, ErrInvalidTransition
	}
	event.Status = run.Status
	event.CreatedAt = EventCreatedAt(at, run.CreatedAt)
	if run.ExternalExecutionID != nil {
		if *run.ExternalExecutionID != externalID {
			return WorkflowRun{}, Event{}, ErrVersionConflict
		}
		var existingID uuid.UUID
		lookupErr := tx.QueryRow(ctx, "SELECT id FROM workflow_run_events WHERE run_id=$1 AND event_type='execution_started' LIMIT 1", runID).Scan(&existingID)
		if lookupErr == nil {
			createdEvent, listErr := txRepo.GetEventByID(ctx, existingID)
			if listErr != nil {
				return WorkflowRun{}, Event{}, listErr
			}
			if err = tx.Commit(ctx); err != nil {
				return WorkflowRun{}, Event{}, err
			}
			return run, createdEvent, nil
		}
		if !errors.Is(lookupErr, pgx.ErrNoRows) {
			return WorkflowRun{}, Event{}, lookupErr
		}
		// Same external id, missing event: repair inside the locked transaction.
		createdEvent, insertErr := txRepo.insertEvent(ctx, event)
		if insertErr != nil {
			return WorkflowRun{}, Event{}, insertErr
		}
		if err = tx.Commit(ctx); err != nil {
			return WorkflowRun{}, Event{}, err
		}
		return run, createdEvent, nil
	}
	updatedAt := RunUpdatedAt(at, run.CreatedAt, run.UpdatedAt)
	updated, err := scanRun(tx.QueryRow(ctx, `UPDATE workflow_run_records
		SET external_execution_id=$1,
		    updated_at=GREATEST($2::timestamptz, created_at, updated_at),
		    version=version+1
		WHERE id=$3 AND version=$4 AND external_execution_id IS NULL AND status IN ('running','cancelling')
		RETURNING `+runColumns, externalID, updatedAt, runID, run.Version))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, Event{}, ErrVersionConflict
	}
	if err != nil {
		return WorkflowRun{}, Event{}, fmt.Errorf("save external execution id: %w", err)
	}
	createdEvent, err := txRepo.insertEvent(ctx, event)
	if err != nil {
		return WorkflowRun{}, Event{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return WorkflowRun{}, Event{}, fmt.Errorf("commit execution started transaction: %w", err)
	}
	return updated, createdEvent, nil
}

func (r *Repository) GetEventByID(ctx context.Context, id uuid.UUID) (Event, error) {
	event, err := scanEvent(r.db.QueryRow(ctx, "SELECT id,run_id,event_type,status,payload,created_at,sequence FROM workflow_run_events WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("get workflow run event: %w", err)
	}
	return event, nil
}

// SaveOutputForConsumption durably records a successful external result before
// any domain write.  The output and its event share one transaction so a
// restarted worker can consume it without touching the executor again.
func (r *Repository) SaveOutputForConsumption(ctx context.Context, current WorkflowRun, output json.RawMessage, event Event) (WorkflowRun, Event, error) {
	if (current.Status != StatusRunning && current.Status != StatusCancelling) || !validJSONObject(output) || event.RunID != current.ID || event.Status != current.Status {
		return WorkflowRun{}, Event{}, ErrValidation
	}
	if r.pool == nil {
		return WorkflowRun{}, Event{}, ErrValidation
	}
	event.CreatedAt = EventCreatedAt(event.CreatedAt, current.CreatedAt)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return WorkflowRun{}, Event{}, err
	}
	defer tx.Rollback(ctx)
	updated, err := scanRun(tx.QueryRow(ctx, "UPDATE workflow_run_records SET output_payload=$1,updated_at=GREATEST($2::timestamptz, created_at, updated_at),version=version+1 WHERE id=$3 AND version=$4 AND status IN ('running','cancelling') AND output_payload IS NULL RETURNING "+runColumns, RedactJSON(output), event.CreatedAt, current.ID, current.Version))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, Event{}, ErrVersionConflict
	}
	if err != nil {
		return WorkflowRun{}, Event{}, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO workflow_run_result_consumptions(workflow_run_id,status) VALUES($1,'pending') ON CONFLICT (workflow_run_id) DO NOTHING", current.ID); err != nil {
		return WorkflowRun{}, Event{}, err
	}
	created, err := NewPostgresRepositoryTx(tx).AddEvent(ctx, event)
	if err != nil {
		return WorkflowRun{}, Event{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return WorkflowRun{}, Event{}, err
	}
	return updated, created, nil
}

// ConsumeResult owns the transaction that spans the stage domain write,
// durable consumption fact, Runtime terminal state and terminal events.
func (r *Repository) ConsumeResult(ctx context.Context, runID uuid.UUID, expectedVersion int, at time.Time, retried bool, consume func(context.Context, pgx.Tx, WorkflowRun) error) (WorkflowRun, bool, error) {
	if r.pool == nil || runID == uuid.Nil || consume == nil {
		return WorkflowRun{}, false, ErrValidation
	}
	// The consumption row is the serialization point. Read committed lets a
	// waiter observe the committed completed fact after acquiring FOR UPDATE;
	// Serializable would instead surface a transient 40001 to an otherwise
	// idempotent concurrent retry.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return WorkflowRun{}, false, fmt.Errorf("begin result consumption: %w", err)
	}
	defer tx.Rollback(ctx)
	var state string
	if err = tx.QueryRow(ctx, "SELECT status FROM workflow_run_result_consumptions WHERE workflow_run_id=$1 FOR UPDATE", runID).Scan(&state); errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, false, ErrNotFound
	} else if err != nil {
		return WorkflowRun{}, false, err
	}
	run, err := NewPostgresRepositoryTx(tx).GetByIDForUpdate(ctx, runID)
	if err != nil {
		return WorkflowRun{}, false, err
	}
	// Clamp application clock to the durable run timestamp so terminal events
	// never violate DC-TIME-007 when the worker clock is behind the original run.
	at = EventCreatedAt(at, run.CreatedAt)
	if run.Version != expectedVersion && state != "completed" {
		return WorkflowRun{}, false, ErrVersionConflict
	}
	if state == "completed" {
		if run.Status != StatusSucceeded {
			return WorkflowRun{}, false, ErrVersionConflict
		}
		return run, true, tx.Commit(ctx)
	}
	if !validJSONObject(run.OutputPayload) || (run.Status != StatusRunning && (run.Status != StatusFailed || run.FailurePhase == nil || *run.FailurePhase != "result_consumption")) {
		return WorkflowRun{}, false, ErrInvalidTransition
	}
	if _, err = tx.Exec(ctx, "UPDATE workflow_run_result_consumptions SET status='in_progress',lease_until=$2,attempt_count=attempt_count+1,failure_code=NULL,safe_error_message=NULL,updated_at=$2 WHERE workflow_run_id=$1", runID, at); err != nil {
		return WorkflowRun{}, false, err
	}
	if err = consume(ctx, tx, run); err != nil {
		return WorkflowRun{}, false, err
	}
	next, err := run.CompleteResultConsumption(at)
	if err != nil {
		return WorkflowRun{}, false, err
	}
	updated, err := scanRun(tx.QueryRow(ctx, "UPDATE workflow_run_records SET status='succeeded',error_code=NULL,error_message=NULL,error_details=NULL,failure_phase=NULL,failure_code=NULL,safe_error_message=NULL,retryability='not_retryable',finished_at=$2,updated_at=GREATEST($2::timestamptz, created_at, updated_at),version=$3 WHERE id=$1 AND version=$4 RETURNING "+runColumns, runID, at, next.Version, run.Version))
	if err != nil {
		return WorkflowRun{}, false, err
	}
	if _, err = tx.Exec(ctx, "UPDATE workflow_run_result_consumptions SET status='completed',lease_until=NULL,completed_at=$2,updated_at=$2 WHERE workflow_run_id=$1", runID, at); err != nil {
		return WorkflowRun{}, false, err
	}
	txRepo := NewPostgresRepositoryTx(tx)
	if retried {
		if _, err = txRepo.insertEvent(ctx, Event{ID: uuid.New(), RunID: runID, EventType: "result_consumption_retried", Status: StatusSucceeded, Payload: json.RawMessage(`{"retried":true}`), CreatedAt: at}); err != nil {
			return WorkflowRun{}, false, err
		}
	}
	var succeededExists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='succeeded')", runID).Scan(&succeededExists); err != nil {
		return WorkflowRun{}, false, err
	}
	if !succeededExists {
		if _, err = txRepo.insertEvent(ctx, Event{ID: uuid.New(), RunID: runID, EventType: "succeeded", Status: StatusSucceeded, Payload: json.RawMessage(`{}`), CreatedAt: at}); err != nil {
			return WorkflowRun{}, false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return WorkflowRun{}, false, fmt.Errorf("commit result consumption: %w", err)
	}
	return updated, false, nil
}

func (r *Repository) MarkResultConsumptionFailure(ctx context.Context, runID uuid.UUID, phase, code, message string, at time.Time) (WorkflowRun, error) {
	if r.pool == nil || runID == uuid.Nil || (phase != "output_validation" && phase != "result_consumption") {
		return WorkflowRun{}, ErrValidation
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return WorkflowRun{}, err
	}
	defer tx.Rollback(ctx)
	run, err := NewPostgresRepositoryTx(tx).GetByIDForUpdate(ctx, runID)
	if err != nil {
		return WorkflowRun{}, err
	}
	if run.Status == StatusSucceeded {
		return run, nil
	}
	at = EventCreatedAt(at, run.CreatedAt)
	retryability := "not_retryable"
	if phase == "result_consumption" {
		retryability = "result_consumption_retry"
	}
	updated, err := scanRun(tx.QueryRow(ctx, "UPDATE workflow_run_records SET status='failed',error_code=$2,error_message=$3,error_details='{}',failure_phase=$4,failure_code=$2,safe_error_message=$3,retryability=$5,finished_at=$6,updated_at=GREATEST($6::timestamptz, created_at, updated_at),version=version+1 WHERE id=$1 AND status IN ('running','failed') RETURNING "+runColumns, runID, code, message, phase, retryability, at))
	if err != nil {
		return WorkflowRun{}, err
	}
	if _, err = tx.Exec(ctx, "UPDATE workflow_run_result_consumptions SET status='failed',lease_until=NULL,attempt_count=attempt_count+1,failure_code=$2,safe_error_message=$3,updated_at=$4 WHERE workflow_run_id=$1", runID, code, message, at); err != nil {
		return WorkflowRun{}, err
	}
	var failureExists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type=$2)", runID, code).Scan(&failureExists); err != nil {
		return WorkflowRun{}, err
	}
	if !failureExists {
		if _, err = NewPostgresRepositoryTx(tx).insertEvent(ctx, Event{ID: uuid.New(), RunID: runID, EventType: code, Status: StatusFailed, Payload: json.RawMessage(`{}`), CreatedAt: at}); err != nil {
			return WorkflowRun{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return WorkflowRun{}, err
	}
	return updated, nil
}
func (r *Repository) AddEvent(ctx context.Context, value Event) (Event, error) {
	// Standalone AddEvent must allocate sequence and insert under one transaction
	// when a pool is available so concurrent writers cannot share a sequence.
	if r.pool != nil {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return Event{}, fmt.Errorf("begin workflow run event transaction: %w", err)
		}
		defer tx.Rollback(ctx)
		var locked uuid.UUID
		if err = tx.QueryRow(ctx, "SELECT id FROM workflow_run_records WHERE id=$1 FOR UPDATE", value.RunID).Scan(&locked); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Event{}, ErrNotFound
			}
			return Event{}, err
		}
		created, err := NewPostgresRepositoryTx(tx).insertEvent(ctx, value)
		if err != nil {
			return Event{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Event{}, err
		}
		return created, nil
	}
	return r.insertEvent(ctx, value)
}
func (r *Repository) ListEvents(ctx context.Context, runID uuid.UUID) ([]Event, error) {
	rows, err := r.db.Query(ctx, "SELECT id,run_id,event_type,status,payload,created_at,sequence FROM workflow_run_events WHERE run_id=$1 ORDER BY sequence ASC", runID)
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
	if err := r.db.QueryRow(ctx, "SELECT COUNT(*), COUNT(*) FILTER (WHERE status IN ('queued','running','cancelling')), COUNT(*) FILTER (WHERE status='failed' AND created_at >= NOW() - INTERVAL '7 days'), MAX(created_at) FROM workflow_run_records WHERE project_id=$1", projectID).Scan(&s.TotalRuns, &s.ActiveRuns, &s.RecentFailedRuns, &s.LastRunAt); err != nil {
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

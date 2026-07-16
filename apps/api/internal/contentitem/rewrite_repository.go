package contentitem

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const rewriteRunColumns = "id,project_id,content_item_id,content_version_id,target_content_version_id,source_review_report_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,error_code,error_summary,started_at,finished_at,created_at,updated_at"
const contentVersionItemVersionNoUniqueConstraint = "content_versions_item_version_no_unique"

var (
	ErrWorkflowRunNotFound = errors.New("workflow run not found")

	_ ContentVersionRepository = (*PostgresRepository)(nil)
	_ WorkflowRunRepository    = (*PostgresRepository)(nil)
)

func scanRewriteRun(row pgx.Row) (value WorkflowRun, err error) {
	err = row.Scan(
		&value.ID, &value.ProjectID, &value.ContentItemID, &value.ContentVersionID,
		&value.TargetContentVersionID, &value.SourceReviewReportID,
		&value.ProviderKey, &value.WorkflowKey, &value.SubjectType, &value.SubjectID,
		&value.Status, &value.IdempotencyKey, &value.RequestFingerprint,
		&value.InputJSON, &value.OutputJSON, &value.ErrorCode, &value.ErrorSummary,
		&value.StartedAt, &value.FinishedAt, &value.CreatedAt, &value.UpdatedAt,
	)
	return
}

func (r *PostgresRepository) GetContentVersion(ctx context.Context, versionID uuid.UUID) (ContentVersion, error) {
	value, err := scanVersion(r.db.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=$1", versionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ContentVersion{}, ErrContentVersionNotFound
	}
	if err != nil {
		return ContentVersion{}, rewriteDatabaseError(err)
	}
	return value, nil
}

func rewriteContentVersionCreateError(err error, value ContentVersion) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == contentVersionItemVersionNoUniqueConstraint && value.Source == ContentVersionSourceMockRewrite && value.VersionNo == 2 {
		return ErrRewriteAlreadyExists
	}
	return rewriteDatabaseError(err)
}

func (r *PostgresRepository) ListContentVersions(ctx context.Context, itemID uuid.UUID, options ContentVersionPageOptions) (out ContentVersionPage, err error) {
	if options.Limit < 1 || options.Limit > 100 || options.Offset < 0 {
		return out, ErrInvalidContentVersion
	}
	if err = r.db.QueryRow(ctx, "SELECT count(*) FROM content_versions WHERE content_item_id=$1", itemID).Scan(&out.Total); err != nil {
		return out, rewriteDatabaseError(err)
	}
	rows, err := r.db.Query(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE content_item_id=$1 ORDER BY version_no DESC,id DESC LIMIT $2 OFFSET $3", itemID, options.Limit, options.Offset)
	if err != nil {
		return out, rewriteDatabaseError(err)
	}
	defer rows.Close()
	for rows.Next() {
		value, scanErr := scanVersion(rows)
		if scanErr != nil {
			return out, rewriteDatabaseError(scanErr)
		}
		out.Items = append(out.Items, value)
	}
	if err = rows.Err(); err != nil {
		return out, rewriteDatabaseError(err)
	}
	out.Limit, out.Offset = options.Limit, options.Offset
	return out, nil
}

func (r *PostgresRepository) CountContentVersions(ctx context.Context, itemID uuid.UUID) (int, error) {
	var total int
	if err := r.db.QueryRow(ctx, "SELECT count(*) FROM content_versions WHERE content_item_id=$1", itemID).Scan(&total); err != nil {
		return 0, rewriteDatabaseError(err)
	}
	return total, nil
}

func (r *PostgresRepository) CreateContentVersion(ctx context.Context, tx pgx.Tx, value ContentVersion) (ContentVersion, error) {
	if err := value.ValidateRewriteShape(); err != nil {
		return ContentVersion{}, err
	}
	created, err := scanVersion(tx.QueryRow(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,summary,word_count,source,status,generation_parameters,version,frozen_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,COALESCE($10,'{}'::jsonb),$11,$12,COALESCE($13,NOW()),COALESCE($14,NOW())) RETURNING "+versionColumns, value.ID, value.ContentItemID, value.VersionNo, value.Title, value.Content, value.Summary, value.WordCount, value.Source, value.Status, value.GenerationParameters, value.Version, value.FrozenAt, nullableTime(value.CreatedAt), nullableTime(value.UpdatedAt)))
	if err != nil {
		return ContentVersion{}, rewriteContentVersionCreateError(err, value)
	}
	return created, nil
}

func (r *PostgresRepository) GetContentVersionByNumber(ctx context.Context, itemID uuid.UUID, versionNo int) (ContentVersion, error) {
	value, err := scanVersion(r.db.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE content_item_id=$1 AND version_no=$2", itemID, versionNo))
	if errors.Is(err, pgx.ErrNoRows) {
		return ContentVersion{}, ErrContentVersionNotFound
	}
	if err != nil {
		return ContentVersion{}, rewriteDatabaseError(err)
	}
	return value, nil
}

func (r *PostgresRepository) GetWorkflowRun(ctx context.Context, runID uuid.UUID) (WorkflowRun, error) {
	value, err := scanRewriteRun(r.db.QueryRow(ctx, "SELECT "+rewriteRunColumns+" FROM workflow_runs WHERE id=$1", runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, ErrWorkflowRunNotFound
	}
	if err != nil {
		return WorkflowRun{}, rewriteDatabaseError(err)
	}
	return value, nil
}

func (r *PostgresRepository) CreateMockRewriteRunning(ctx context.Context, tx pgx.Tx, value WorkflowRun) (WorkflowRun, error) {
	if err := value.ValidateRewriteShape(); err != nil {
		return WorkflowRun{}, err
	}
	created, err := scanRewriteRun(tx.QueryRow(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,target_content_version_id,source_review_report_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,error_code,error_summary,started_at,finished_at,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,COALESCE($14,'{}'::jsonb),COALESCE($15,'{}'::jsonb),$16,$17,COALESCE($18,NOW()),$19,COALESCE($20,NOW()),COALESCE($21,NOW())) RETURNING "+rewriteRunColumns, value.ID, value.ProjectID, value.ContentItemID, value.ContentVersionID, value.TargetContentVersionID, value.SourceReviewReportID, value.ProviderKey, value.WorkflowKey, value.SubjectType, value.SubjectID, value.Status, value.IdempotencyKey, value.RequestFingerprint, value.InputJSON, value.OutputJSON, value.ErrorCode, value.ErrorSummary, nullableTime(value.StartedAt), value.FinishedAt, nullableTime(value.CreatedAt), nullableTime(value.UpdatedAt)))
	if err != nil {
		return WorkflowRun{}, rewriteDatabaseError(err)
	}
	return created, nil
}

func (r *PostgresRepository) MarkMockRewriteSucceeded(ctx context.Context, tx pgx.Tx, runID, targetVersionID uuid.UUID, output []byte, finishedAt time.Time) (WorkflowRun, error) {
	value, err := scanRewriteRun(tx.QueryRow(ctx, "UPDATE workflow_runs SET status=$2,target_content_version_id=$3,output_json=COALESCE($4,'{}'::jsonb),error_code=NULL,error_summary=NULL,finished_at=$5,updated_at=NOW() WHERE id=$1 AND workflow_key=$6 AND status=$7 RETURNING "+rewriteRunColumns, runID, WorkflowRunStatusSucceeded, targetVersionID, output, finishedAt, WorkflowKeyMockRewrite, WorkflowRunStatusRunning))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, ErrWorkflowRunNotFound
	}
	if err != nil {
		return WorkflowRun{}, rewriteDatabaseError(err)
	}
	return value, nil
}

func (r *PostgresRepository) MarkMockRewriteFailed(ctx context.Context, tx pgx.Tx, runID uuid.UUID, code, summary string, finishedAt time.Time) (WorkflowRun, error) {
	if !safeFailure(code, summary) {
		return WorkflowRun{}, ErrInvalidMockRewriteRun
	}
	value, err := scanRewriteRun(tx.QueryRow(ctx, "UPDATE workflow_runs SET status=$2,target_content_version_id=NULL,output_json='{}'::jsonb,error_code=$3,error_summary=$4,finished_at=$5,updated_at=NOW() WHERE id=$1 AND workflow_key=$6 AND status=$7 RETURNING "+rewriteRunColumns, runID, WorkflowRunStatusFailed, code, summary, finishedAt, WorkflowKeyMockRewrite, WorkflowRunStatusRunning))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, ErrWorkflowRunNotFound
	}
	if err != nil {
		return WorkflowRun{}, rewriteDatabaseError(err)
	}
	return value, nil
}

func (r *PostgresRepository) FindMockRewriteByIdempotencyKey(ctx context.Context, itemID uuid.UUID, key string) (WorkflowRun, error) {
	return r.findMockRewrite(ctx, "idempotency_key", itemID, key)
}

func (r *PostgresRepository) FindMockRewriteByFingerprint(ctx context.Context, itemID uuid.UUID, fingerprint string) (WorkflowRun, error) {
	return r.findMockRewrite(ctx, "request_fingerprint", itemID, fingerprint)
}

func (r *PostgresRepository) findMockRewrite(ctx context.Context, column string, itemID uuid.UUID, value string) (WorkflowRun, error) {
	query := "SELECT " + rewriteRunColumns + " FROM workflow_runs WHERE content_item_id=$1 AND workflow_key=$2 AND " + column + "=$3"
	run, err := scanRewriteRun(r.db.QueryRow(ctx, query, itemID, WorkflowKeyMockRewrite, value))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, ErrWorkflowRunNotFound
	}
	if err != nil {
		return WorkflowRun{}, rewriteDatabaseError(err)
	}
	return run, nil
}

func (r *PostgresRepository) LatestMockRewrite(ctx context.Context, itemID uuid.UUID) (WorkflowRun, error) {
	value, err := scanRewriteRun(r.db.QueryRow(ctx, "SELECT "+rewriteRunColumns+" FROM workflow_runs WHERE content_item_id=$1 AND workflow_key=$2 ORDER BY started_at DESC,id DESC LIMIT 1", itemID, WorkflowKeyMockRewrite))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkflowRun{}, ErrWorkflowRunNotFound
	}
	if err != nil {
		return WorkflowRun{}, rewriteDatabaseError(err)
	}
	return value, nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func rewriteDatabaseError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return ErrInternal
	}
	switch pgErr.Code {
	case "23505":
		if pgErr.ConstraintName == "workflow_runs_scope_idempotency_key_unique" {
			return ErrIdempotencyConflict
		}
		return ErrVersionConflict
	case "23503":
		return ErrCrossProjectRelation
	case "23514", "22001", "22P02":
		return ErrInvalidMockRewriteRun
	default:
		return ErrInternal
	}
}

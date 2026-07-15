package contentitem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const reportColumns = "id,project_id,content_item_id,content_version_id,workflow_run_id,provider_key,status,conclusion,score,summary,created_at,completed_at"
const findingColumns = "id,review_id,category,severity,title,description,location_json,sort_order,created_at"
const recommendationColumns = "id,review_id,priority,title,description,sort_order,created_at"

func scanReport(r pgx.Row) (v ReviewReport, err error) {
	err = r.Scan(&v.ID, &v.ProjectID, &v.ContentItemID, &v.ContentVersionID, &v.WorkflowRunID, &v.ProviderKey, &v.Status, &v.Conclusion, &v.Score, &v.Summary, &v.CreatedAt, &v.CompletedAt)
	return
}
func scanFinding(r pgx.Row) (v ReviewFinding, err error) {
	err = r.Scan(&v.ID, &v.ReviewID, &v.Category, &v.Severity, &v.Title, &v.Description, &v.LocationJSON, &v.SortOrder, &v.CreatedAt)
	return
}
func scanRecommendation(r pgx.Row) (v ReviewRecommendation, err error) {
	err = r.Scan(&v.ID, &v.ReviewID, &v.Priority, &v.Title, &v.Description, &v.SortOrder, &v.CreatedAt)
	return
}

func reviewRun(ctx context.Context, tx pgx.Tx, projectID, itemID uuid.UUID, key string) (WorkflowRun, error) {
	return scanRun(tx.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_runs WHERE project_id=$1 AND content_item_id=$2 AND workflow_key='content_mock_review' AND idempotency_key=$3", projectID, itemID, key))
}

func validateReviewResult(x ReviewResult) error {
	if (x.Conclusion != "pass" && x.Conclusion != "revise") || x.Score < 0 || x.Score > 100 || len(x.Summary) > 5000 {
		return ErrInvalidReviewResult
	}
	for _, f := range x.Findings {
		if !oneOf(f.Category, "pacing", "foreshadowing", "character_consistency", "world_consistency") || !oneOf(f.Severity, "low", "medium", "high") || strings.TrimSpace(f.Title) == "" || len(f.Title) > 200 || len(f.Description) > 5000 || f.SortOrder < 0 || !jsonObjectOrEmpty(f.LocationJSON) {
			return ErrInvalidReviewResult
		}
	}
	for _, rec := range x.Recommendations {
		if !oneOf(rec.Priority, "low", "medium", "high") || strings.TrimSpace(rec.Title) == "" || len(rec.Title) > 200 || len(rec.Description) > 5000 || rec.SortOrder < 0 {
			return ErrInvalidReviewResult
		}
	}
	return nil
}
func oneOf(v string, choices ...string) bool {
	for _, x := range choices {
		if v == x {
			return true
		}
	}
	return false
}
func jsonObjectOrEmpty(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	var v map[string]json.RawMessage
	return json.Unmarshal(raw, &v) == nil
}

// PersistMockReview is the D2 atomic boundary: it persists only a caller-provided,
// deterministic result and leaves no running run or partial review tree on error.
func (r *PostgresRepository) PersistMockReview(ctx context.Context, req ReviewRequest) (out ReviewOutcome, err error) {
	if err = validateReviewResult(req.Result); err != nil {
		return out, err
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	d, err := r.detail(ctx, tx, req.ContentItemID)
	if err != nil {
		return out, err
	}
	version, err := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=$1", req.ContentVersionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrContentVersionNotFound
	}
	if err != nil {
		return out, fmt.Errorf("review version read: %w", err)
	}
	if version.ContentItemID != d.Item.ID {
		return out, ErrCrossProjectRelation
	}
	if version.ID != d.Item.CurrentVersionID {
		return out, ErrContentVersionLocked
	}
	prior, priorErr := reviewRun(ctx, tx, d.Item.ProjectID, req.ContentItemID, req.IdempotencyKey)
	if priorErr == nil {
		if prior.RequestFingerprint != req.Fingerprint {
			return out, ErrIdempotencyConflict
		}
		out, err = r.reviewOutcomeByRun(ctx, tx, prior.ID, d.Item.ProjectID, req.ContentItemID)
		if err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(priorErr, pgx.ErrNoRows) {
		return out, fmt.Errorf("review idempotency read: %w", priorErr)
	}
	if d.Item.Status == "reviewed" || version.Status == "frozen" {
		return out, ErrContentVersionReviewed
	}
	if d.Item.Status != "draft" || version.Status != "editable_draft" {
		return out, ErrContentVersionLocked
	}
	if version.Version != req.ExpectedVersion {
		return out, ErrVersionConflict
	}
	runID, reviewID := uuid.New(), uuid.New()
	_, err = tx.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json) VALUES($1,$2,$3,$4,'mock','content_mock_review','content_item',$3,'running',$5,$6,COALESCE($7,'{}'::jsonb),COALESCE($8,'{}'::jsonb))", runID, d.Item.ProjectID, d.Item.ID, version.ID, req.IdempotencyKey, req.Fingerprint, req.Result.InputJSON, req.Result.OutputJSON)
	if err != nil {
		return out, fmt.Errorf("review workflow create: %w", err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO review_reports(id,project_id,content_item_id,content_version_id,workflow_run_id,provider_key,status,conclusion,score,summary) VALUES($1,$2,$3,$4,$5,'mock','completed',$6,$7,$8)", reviewID, d.Item.ProjectID, d.Item.ID, version.ID, runID, req.Result.Conclusion, req.Result.Score, req.Result.Summary)
	if err != nil {
		return out, fmt.Errorf("review report create: %w", err)
	}
	for _, f := range req.Result.Findings {
		_, err = tx.Exec(ctx, "INSERT INTO review_findings(id,review_id,category,severity,title,description,location_json,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", uuid.New(), reviewID, f.Category, f.Severity, f.Title, f.Description, nullableJSON(f.LocationJSON), f.SortOrder)
		if err != nil {
			return out, ErrInvalidReviewResult
		}
	}
	for _, rec := range req.Result.Recommendations {
		_, err = tx.Exec(ctx, "INSERT INTO review_recommendations(id,review_id,priority,title,description,sort_order) VALUES($1,$2,$3,$4,$5,$6)", uuid.New(), reviewID, rec.Priority, rec.Title, rec.Description, rec.SortOrder)
		if err != nil {
			return out, ErrInvalidReviewResult
		}
	}
	_, err = tx.Exec(ctx, "UPDATE content_versions SET status='frozen',frozen_at=NOW(),version=version+1,updated_at=NOW() WHERE id=$1", version.ID)
	if err != nil {
		return out, fmt.Errorf("content version freeze: %w", err)
	}
	_, err = tx.Exec(ctx, "UPDATE content_items SET status='reviewed',reviewed_at=NOW(),version=version+1,updated_at=NOW() WHERE id=$1", d.Item.ID)
	if err != nil {
		return out, fmt.Errorf("content item review: %w", err)
	}
	_, err = tx.Exec(ctx, "UPDATE workflow_runs SET status='succeeded',finished_at=NOW(),updated_at=NOW() WHERE id=$1", runID)
	if err != nil {
		return out, fmt.Errorf("review workflow complete: %w", err)
	}
	out, err = r.reviewOutcomeByRun(ctx, tx, runID, d.Item.ProjectID, d.Item.ID)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func nullableJSON(v []byte) any {
	if len(v) == 0 {
		return nil
	}
	return v
}

// GetReviewContentVersion is a narrow read used by the Application layer to
// reject a missing or non-current review target before generator execution.
func (r *PostgresRepository) GetReviewContentVersion(ctx context.Context, itemID, versionID uuid.UUID) (ContentVersion, error) {
	d, err := r.detail(ctx, r.db, itemID)
	if err != nil {
		return ContentVersion{}, err
	}
	v, err := scanVersion(r.db.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=$1", versionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ContentVersion{}, ErrContentVersionNotFound
	}
	if err != nil {
		return ContentVersion{}, fmt.Errorf("review version read: %w", err)
	}
	if v.ContentItemID != d.Item.ID {
		return ContentVersion{}, ErrContentVersionNotFound
	}
	return v, nil
}

func (r *PostgresRepository) RecordMockReviewFailure(ctx context.Context, req ReviewFailureRequest) (out WorkflowRun, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	d, err := r.detail(ctx, tx, req.ContentItemID)
	if err != nil {
		return out, err
	}
	v, err := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=$1", req.ContentVersionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrContentVersionNotFound
	}
	if err != nil {
		return out, err
	}
	if v.ContentItemID != d.Item.ID {
		return out, ErrCrossProjectRelation
	}
	prior, e := reviewRun(ctx, tx, d.Item.ProjectID, req.ContentItemID, req.IdempotencyKey)
	if e == nil {
		if prior.RequestFingerprint != req.Fingerprint {
			return out, ErrIdempotencyConflict
		}
		return prior, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	if d.Item.Status != "draft" || v.Status != "editable_draft" {
		return out, ErrContentVersionLocked
	}
	if !safeFailure(req.ErrorCode, req.ErrorSummary) {
		return out, ErrInvalidReviewResult
	}
	id := uuid.New()
	_, err = tx.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,error_code,error_summary,finished_at) VALUES($1,$2,$3,$4,'mock','content_mock_review','content_item',$3,'failed',$5,$6,COALESCE($7,'{}'::jsonb),'{}'::jsonb,$8,$9,NOW())", id, d.Item.ProjectID, d.Item.ID, v.ID, req.IdempotencyKey, req.Fingerprint, req.InputJSON, req.ErrorCode, req.ErrorSummary)
	if err != nil {
		return out, fmt.Errorf("review workflow failure create: %w", err)
	}
	out, err = scanRun(tx.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_runs WHERE id=$1", id))
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (r *PostgresRepository) ListReviews(ctx context.Context, itemID uuid.UUID, limit, offset int) (out ReviewList, err error) {
	if limit < 1 || limit > 100 || offset < 0 {
		return out, ErrInvalidReviewResult
	}
	detail, err := r.detail(ctx, r.db, itemID)
	if err != nil {
		return out, err
	}
	err = r.db.QueryRow(ctx, "SELECT count(*) FROM review_reports WHERE project_id=$1 AND content_item_id=$2", detail.Item.ProjectID, itemID).Scan(&out.Total)
	if err != nil {
		return out, err
	}
	rows, err := r.db.Query(ctx, "SELECT "+reportColumns+" FROM review_reports WHERE project_id=$1 AND content_item_id=$2 ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4", detail.Item.ProjectID, itemID, limit, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		x, e := scanReport(rows)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	out.Limit, out.Offset = limit, offset
	return out, nil
}

func (r *PostgresRepository) GetReview(ctx context.Context, reviewID uuid.UUID) (out ReviewDetail, err error) {
	row := r.db.QueryRow(ctx, "SELECT "+qualifiedColumns("rr", reportColumns)+","+qualifiedColumns("cv", versionColumns)+","+qualifiedColumns("wr", runColumns)+" FROM review_reports rr JOIN content_versions cv ON cv.id=rr.content_version_id JOIN workflow_runs wr ON wr.id=rr.workflow_run_id WHERE rr.id=$1", reviewID)
	err = row.Scan(&out.Review.ID, &out.Review.ProjectID, &out.Review.ContentItemID, &out.Review.ContentVersionID, &out.Review.WorkflowRunID, &out.Review.ProviderKey, &out.Review.Status, &out.Review.Conclusion, &out.Review.Score, &out.Review.Summary, &out.Review.CreatedAt, &out.Review.CompletedAt, &out.ContentVersion.ID, &out.ContentVersion.ContentItemID, &out.ContentVersion.VersionNo, &out.ContentVersion.Title, &out.ContentVersion.Content, &out.ContentVersion.Summary, &out.ContentVersion.WordCount, &out.ContentVersion.Source, &out.ContentVersion.Status, &out.ContentVersion.GenerationParameters, &out.ContentVersion.Version, &out.ContentVersion.FrozenAt, &out.ContentVersion.CreatedAt, &out.ContentVersion.UpdatedAt, &out.WorkflowRun.ID, &out.WorkflowRun.ProjectID, &out.WorkflowRun.ContentItemID, &out.WorkflowRun.ContentVersionID, &out.WorkflowRun.ProviderKey, &out.WorkflowRun.WorkflowKey, &out.WorkflowRun.SubjectType, &out.WorkflowRun.SubjectID, &out.WorkflowRun.Status, &out.WorkflowRun.IdempotencyKey, &out.WorkflowRun.RequestFingerprint, &out.WorkflowRun.InputJSON, &out.WorkflowRun.OutputJSON, &out.WorkflowRun.ErrorCode, &out.WorkflowRun.ErrorSummary, &out.WorkflowRun.StartedAt, &out.WorkflowRun.FinishedAt, &out.WorkflowRun.CreatedAt, &out.WorkflowRun.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrReviewNotFound
	}
	if err != nil {
		return out, fmt.Errorf("review detail read: %w", err)
	}
	if out.Review.ContentVersionID != out.ContentVersion.ID || out.Review.WorkflowRunID != out.WorkflowRun.ID || out.WorkflowRun.ContentVersionID != out.ContentVersion.ID {
		return out, ErrCrossProjectRelation
	}
	findings, err := r.db.Query(ctx, "SELECT "+findingColumns+" FROM review_findings WHERE review_id=$1 ORDER BY sort_order,id", reviewID)
	if err != nil {
		return out, err
	}
	defer findings.Close()
	for findings.Next() {
		x, e := scanFinding(findings)
		if e != nil {
			return out, e
		}
		out.Findings = append(out.Findings, x)
	}
	if err = findings.Err(); err != nil {
		return out, err
	}
	recs, err := r.db.Query(ctx, "SELECT "+recommendationColumns+" FROM review_recommendations WHERE review_id=$1 ORDER BY sort_order,id", reviewID)
	if err != nil {
		return out, err
	}
	defer recs.Close()
	for recs.Next() {
		x, e := scanRecommendation(recs)
		if e != nil {
			return out, e
		}
		out.Recommendations = append(out.Recommendations, x)
	}
	if err = recs.Err(); err != nil {
		return out, err
	}
	return out, nil
}

func qualifiedColumns(alias, columns string) string {
	parts := strings.Split(columns, ",")
	for i := range parts {
		parts[i] = alias + "." + parts[i]
	}
	return strings.Join(parts, ",")
}

func (r *PostgresRepository) reviewOutcomeByRun(ctx context.Context, tx pgx.Tx, runID, projectID, itemID uuid.UUID) (out ReviewOutcome, err error) {
	out.Detail, err = r.detail(ctx, tx, itemID)
	if err != nil {
		return out, err
	}
	out.WorkflowRun, err = scanRun(tx.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_runs WHERE id=$1", runID))
	if err != nil {
		return out, err
	}
	out.Review, err = scanReport(tx.QueryRow(ctx, "SELECT "+reportColumns+" FROM review_reports WHERE workflow_run_id=$1", runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrInvalidReviewResult
	}
	if err != nil {
		return out, err
	}
	rows, err := tx.Query(ctx, "SELECT "+findingColumns+" FROM review_findings WHERE review_id=$1 ORDER BY sort_order,id", out.Review.ID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		x, e := scanFinding(rows)
		if e != nil {
			return out, e
		}
		out.Findings = append(out.Findings, x)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, "SELECT "+recommendationColumns+" FROM review_recommendations WHERE review_id=$1 ORDER BY sort_order,id", out.Review.ID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		x, e := scanRecommendation(rows)
		if e != nil {
			return out, e
		}
		out.Recommendations = append(out.Recommendations, x)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	return out, nil
}

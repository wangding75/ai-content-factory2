// Package contentitem persists the Iteration 06 body domain.
package contentitem

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrChapterPlanNotFound     = errors.New("chapter plan not found")
	ErrChapterPlanNotConfirmed = errors.New("chapter plan not confirmed")
	ErrContentItemNotFound     = errors.New("content item not found")
	ErrContentVersionNotFound  = errors.New("content version not found")
	ErrContentVersionLocked    = errors.New("content version locked")
	ErrContentVersionReviewed  = errors.New("content version already reviewed")
	ErrVersionConflict         = errors.New("content version conflict")
	ErrIdempotencyConflict     = errors.New("idempotency key reused with different payload")
	ErrCrossProjectRelation    = errors.New("cross-project relation conflict")
	ErrReviewNotFound          = errors.New("review not found")
	ErrInvalidReviewResult     = errors.New("invalid review result")
	ErrInvalidContentVersion   = errors.New("invalid content version")
	ErrInvalidMockRewriteRun   = errors.New("invalid mock rewrite workflow run")
)

type ContentItem struct {
	ID, ProjectID, ChapterPlanID, CurrentVersionID uuid.UUID
	Title, Status                                  string
	Version                                        int
	ReviewedAt                                     *time.Time
	CreatedAt, UpdatedAt                           time.Time
}
type ContentVersion struct {
	ID, ContentItemID              uuid.UUID
	VersionNo, Version, WordCount  int
	Title, Content, Source, Status string
	Summary                        *string
	GenerationParameters           []byte
	FrozenAt                       *time.Time
	CreatedAt, UpdatedAt           time.Time
}
type WorkflowRun struct {
	ID, ProjectID, ContentItemID, ContentVersionID uuid.UUID
	TargetContentVersionID, SourceReviewReportID   *uuid.UUID
	ProviderKey, WorkflowKey, SubjectType          string
	SubjectID                                      uuid.UUID
	Status, IdempotencyKey, RequestFingerprint     string
	InputJSON, OutputJSON                          []byte
	ErrorCode, ErrorSummary                        *string
	StartedAt                                      time.Time
	FinishedAt                                     *time.Time
	CreatedAt, UpdatedAt                           time.Time
}
type Detail struct {
	Item           ContentItem
	CurrentVersion ContentVersion
}
type CreateResult struct {
	Detail  Detail
	Created bool
}

// OptionalString distinguishes omitted (Set=false), null (Set=true, Value=nil), and empty text.
type OptionalString struct {
	Set   bool
	Value *string
}
type OptionalInt struct {
	Set   bool
	Value *int
}
type DraftPatch struct {
	Title, Content, Summary OptionalString
	WordCount               OptionalInt
}
type GenerationResult struct {
	Title, Content, Summary string
	SummarySet              bool
	WordCount               int
	Parameters              []byte
	InputJSON, OutputJSON   []byte
}
type GenerationRequest struct {
	ContentItemID               uuid.UUID
	ExpectedVersion             int
	IdempotencyKey, Fingerprint string
	Result                      GenerationResult
}
type GenerationOutcome struct {
	Detail      Detail
	WorkflowRun WorkflowRun
}
type FailureRequest struct {
	ContentItemID               uuid.UUID
	IdempotencyKey, Fingerprint string
	InputJSON                   []byte
	ErrorCode, ErrorSummary     string
}

// ReviewReport and its children are immutable output supplied by the future
// Application layer; this repository deliberately never generates review text.
type ReviewReport struct {
	ID, ProjectID, ContentItemID, ContentVersionID, WorkflowRunID uuid.UUID
	ProviderKey, Status, Conclusion, Summary                      string
	Score                                                         int
	CreatedAt, CompletedAt                                        time.Time
}
type ReviewFinding struct {
	ID, ReviewID                           uuid.UUID
	Category, Severity, Title, Description string
	LocationJSON                           []byte
	SortOrder                              int
	CreatedAt                              time.Time
}
type ReviewRecommendation struct {
	ID, ReviewID                 uuid.UUID
	Priority, Title, Description string
	SortOrder                    int
	CreatedAt                    time.Time
}
type ReviewResult struct {
	Conclusion, Summary   string
	Score                 int
	Findings              []ReviewFinding
	Recommendations       []ReviewRecommendation
	InputJSON, OutputJSON []byte
}
type ReviewRequest struct {
	ContentItemID, ContentVersionID uuid.UUID
	ExpectedVersion                 int
	IdempotencyKey, Fingerprint     string
	Result                          ReviewResult
}
type ReviewFailureRequest struct {
	ContentItemID, ContentVersionID uuid.UUID
	IdempotencyKey, Fingerprint     string
	InputJSON                       []byte
	ErrorCode, ErrorSummary         string
}
type ReviewOutcome struct {
	Detail          Detail
	Review          ReviewReport
	Findings        []ReviewFinding
	Recommendations []ReviewRecommendation
	WorkflowRun     WorkflowRun
}
type ReviewList struct {
	Items                []ReviewReport
	Total, Limit, Offset int
}
type ReviewDetail struct {
	Review          ReviewReport
	ContentVersion  ContentVersion
	Findings        []ReviewFinding
	Recommendations []ReviewRecommendation
	WorkflowRun     WorkflowRun
}

type Repository interface {
	CreateOrGet(context.Context, uuid.UUID) (CreateResult, error)
	GetByID(context.Context, uuid.UUID) (Detail, error)
	GetByChapterPlanID(context.Context, uuid.UUID) (Detail, error)
	SaveDraft(context.Context, uuid.UUID, int, DraftPatch) (Detail, error)
	PersistMockGeneration(context.Context, GenerationRequest) (GenerationOutcome, error)
	RecordMockGenerationFailure(context.Context, FailureRequest) (WorkflowRun, error)
	PersistMockReview(context.Context, ReviewRequest) (ReviewOutcome, error)
	RecordMockReviewFailure(context.Context, ReviewFailureRequest) (WorkflowRun, error)
	ListReviews(context.Context, uuid.UUID, int, int) (ReviewList, error)
	GetReview(context.Context, uuid.UUID) (ReviewDetail, error)
}

type PostgresRepository struct{ db *pgxpool.Pool }

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository { return &PostgresRepository{db: db} }

const itemColumns = "id,project_id,chapter_plan_id,title,status,current_version_id,version,reviewed_at,created_at,updated_at"
const versionColumns = "id,content_item_id,version_no,title,content,summary,word_count,source,status,generation_parameters,version,frozen_at,created_at,updated_at"
const runColumns = "id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,error_code,error_summary,started_at,finished_at,created_at,updated_at"

func scanItem(r pgx.Row) (v ContentItem, err error) {
	err = r.Scan(&v.ID, &v.ProjectID, &v.ChapterPlanID, &v.Title, &v.Status, &v.CurrentVersionID, &v.Version, &v.ReviewedAt, &v.CreatedAt, &v.UpdatedAt)
	return
}
func scanVersion(r pgx.Row) (v ContentVersion, err error) {
	err = r.Scan(&v.ID, &v.ContentItemID, &v.VersionNo, &v.Title, &v.Content, &v.Summary, &v.WordCount, &v.Source, &v.Status, &v.GenerationParameters, &v.Version, &v.FrozenAt, &v.CreatedAt, &v.UpdatedAt)
	return
}
func scanRun(r pgx.Row) (v WorkflowRun, err error) {
	err = r.Scan(&v.ID, &v.ProjectID, &v.ContentItemID, &v.ContentVersionID, &v.ProviderKey, &v.WorkflowKey, &v.SubjectType, &v.SubjectID, &v.Status, &v.IdempotencyKey, &v.RequestFingerprint, &v.InputJSON, &v.OutputJSON, &v.ErrorCode, &v.ErrorSummary, &v.StartedAt, &v.FinishedAt, &v.CreatedAt, &v.UpdatedAt)
	return
}

func (r *PostgresRepository) detail(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, itemID uuid.UUID) (Detail, error) {
	i, e := scanItem(q.QueryRow(ctx, "SELECT "+itemColumns+" FROM content_items WHERE id=$1", itemID))
	if errors.Is(e, pgx.ErrNoRows) {
		return Detail{}, ErrContentItemNotFound
	}
	if e != nil {
		return Detail{}, fmt.Errorf("content item read: %w", e)
	}
	v, e := scanVersion(q.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=$1 AND content_item_id=$2", i.CurrentVersionID, i.ID))
	if errors.Is(e, pgx.ErrNoRows) {
		return Detail{}, ErrContentVersionNotFound
	}
	if e != nil {
		return Detail{}, fmt.Errorf("content version read: %w", e)
	}
	return Detail{Item: i, CurrentVersion: v}, nil
}
func (r *PostgresRepository) GetByID(ctx context.Context, itemID uuid.UUID) (Detail, error) {
	return r.detail(ctx, r.db, itemID)
}
func (r *PostgresRepository) GetByChapterPlanID(ctx context.Context, planID uuid.UUID) (Detail, error) {
	var id uuid.UUID
	e := r.db.QueryRow(ctx, "SELECT id FROM content_items WHERE chapter_plan_id=$1", planID).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return Detail{}, ErrContentItemNotFound
	}
	if e != nil {
		return Detail{}, fmt.Errorf("content item lookup: %w", e)
	}
	return r.GetByID(ctx, id)
}

func (r *PostgresRepository) CreateOrGet(ctx context.Context, planID uuid.UUID) (out CreateResult, err error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var planProject uuid.UUID
	var title, status string
	err = tx.QueryRow(ctx, "SELECT project_id,title,status FROM chapter_plans WHERE id=$1 FOR UPDATE", planID).Scan(&planProject, &title, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrChapterPlanNotFound
	}
	if err != nil {
		return out, fmt.Errorf("chapter plan read: %w", err)
	}
	if status != "confirmed" {
		return out, ErrChapterPlanNotConfirmed
	}
	var existing uuid.UUID
	err = tx.QueryRow(ctx, "SELECT id FROM content_items WHERE chapter_plan_id=$1", planID).Scan(&existing)
	if err == nil {
		out.Detail, err = r.detail(ctx, tx, existing)
		if err != nil {
			return out, err
		}
		return out, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return out, fmt.Errorf("content item lookup: %w", err)
	}
	itemID, versionID := uuid.New(), uuid.New()
	_, err = tx.Exec(ctx, "INSERT INTO content_items(id,project_id,chapter_plan_id,title,status,current_version_id) VALUES($1,$2,$3,$4,'draft',$5)", itemID, planProject, planID, title, versionID)
	if err != nil {
		return out, fmt.Errorf("content item create: %w", err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,summary,word_count,source,status) VALUES($1,$2,1,$3,'',NULL,0,'manual_created','editable_draft')", versionID, itemID, title)
	if err != nil {
		return out, fmt.Errorf("content version create: %w", err)
	}
	out.Detail, err = r.detail(ctx, tx, itemID)
	if err != nil {
		return out, err
	}
	out.Created = true
	err = tx.Commit(ctx)
	return out, err
}

func optionalString(v OptionalString, current *string) *string {
	if !v.Set {
		return current
	}
	return v.Value
}
func optionalInt(v OptionalInt, current int) int {
	if !v.Set {
		return current
	}
	if v.Value == nil {
		return current
	}
	return *v.Value
}
func (r *PostgresRepository) SaveDraft(ctx context.Context, itemID uuid.UUID, expected int, p DraftPatch) (Detail, error) {
	tx, e := r.db.Begin(ctx)
	if e != nil {
		return Detail{}, e
	}
	defer tx.Rollback(ctx)
	var projectID uuid.UUID
	e = tx.QueryRow(ctx, "SELECT project_id FROM content_items WHERE id=$1 FOR UPDATE", itemID).Scan(&projectID)
	if errors.Is(e, pgx.ErrNoRows) {
		return Detail{}, ErrContentItemNotFound
	}
	if e != nil {
		return Detail{}, e
	}
	d, e := r.detail(ctx, tx, itemID)
	if e != nil {
		return Detail{}, e
	}
	if d.CurrentVersion.Status != "editable_draft" {
		return Detail{}, ErrContentVersionLocked
	}
	if d.CurrentVersion.Version != expected {
		return Detail{}, ErrVersionConflict
	}
	title := optionalString(p.Title, &d.CurrentVersion.Title)
	content := optionalString(p.Content, &d.CurrentVersion.Content)
	summary := optionalString(p.Summary, d.CurrentVersion.Summary)
	words := optionalInt(p.WordCount, d.CurrentVersion.WordCount)
	if title == nil {
		return Detail{}, ErrVersionConflict
	}
	_, e = tx.Exec(ctx, "UPDATE content_versions SET title=$2,content=$3,summary=$4,word_count=$5,version=version+1,updated_at=NOW() WHERE id=$1", d.CurrentVersion.ID, *title, *content, summary, words)
	if e != nil {
		return Detail{}, fmt.Errorf("content draft save: %w", e)
	}
	_, e = tx.Exec(ctx, "UPDATE content_items SET title=$2,updated_at=NOW() WHERE id=$1", itemID, *title)
	if e != nil {
		return Detail{}, e
	}
	d, e = r.detail(ctx, tx, itemID)
	if e != nil {
		return Detail{}, e
	}
	e = tx.Commit(ctx)
	return d, e
}

func (r *PostgresRepository) PersistMockGeneration(ctx context.Context, req GenerationRequest) (GenerationOutcome, error) {
	tx, e := r.db.Begin(ctx)
	if e != nil {
		return GenerationOutcome{}, e
	}
	defer tx.Rollback(ctx)
	d, e := r.detail(ctx, tx, req.ContentItemID)
	if e != nil {
		return GenerationOutcome{}, e
	}
	var prior WorkflowRun
	prior, e = scanRun(tx.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_runs WHERE project_id=$1 AND content_item_id=$2 AND workflow_key='content_mock_generate' AND idempotency_key=$3", d.Item.ProjectID, req.ContentItemID, req.IdempotencyKey))
	if e == nil {
		if prior.RequestFingerprint != req.Fingerprint {
			return GenerationOutcome{}, ErrIdempotencyConflict
		}
		d, e = r.detail(ctx, tx, req.ContentItemID)
		if e != nil {
			return GenerationOutcome{}, e
		}
		return GenerationOutcome{d, prior}, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return GenerationOutcome{}, e
	}
	if d.CurrentVersion.Status != "editable_draft" {
		return GenerationOutcome{}, ErrContentVersionLocked
	}
	if d.CurrentVersion.Version != req.ExpectedVersion {
		return GenerationOutcome{}, ErrVersionConflict
	}
	runID := uuid.New()
	_, e = tx.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json) VALUES($1,$2,$3,$4,'mock','content_mock_generate','content_item',$3,'running',$5,$6,COALESCE($7,'{}'::jsonb),COALESCE($8,'{}'::jsonb))", runID, d.Item.ProjectID, req.ContentItemID, d.CurrentVersion.ID, req.IdempotencyKey, req.Fingerprint, req.Result.InputJSON, req.Result.OutputJSON)
	if e != nil {
		return GenerationOutcome{}, fmt.Errorf("workflow run create: %w", e)
	}
	summary := d.CurrentVersion.Summary
	if req.Result.SummarySet {
		summary = &req.Result.Summary
	}
	title := req.Result.Title
	if title == "" {
		title = d.CurrentVersion.Title
	}
	_, e = tx.Exec(ctx, "UPDATE content_versions SET title=$2,content=$3,summary=$4,word_count=$5,source='mock_generated',generation_parameters=COALESCE($6,'{}'::jsonb),version=version+1,updated_at=NOW() WHERE id=$1", d.CurrentVersion.ID, title, req.Result.Content, summary, req.Result.WordCount, req.Result.Parameters)
	if e != nil {
		return GenerationOutcome{}, fmt.Errorf("generated content save: %w", e)
	}
	_, e = tx.Exec(ctx, "UPDATE content_items SET title=$2,updated_at=NOW() WHERE id=$1", d.Item.ID, title)
	if e != nil {
		return GenerationOutcome{}, e
	}
	_, e = tx.Exec(ctx, "UPDATE workflow_runs SET status='succeeded',finished_at=NOW(),updated_at=NOW() WHERE id=$1", runID)
	if e != nil {
		return GenerationOutcome{}, e
	}
	d, e = r.detail(ctx, tx, req.ContentItemID)
	if e != nil {
		return GenerationOutcome{}, e
	}
	run, e := scanRun(tx.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_runs WHERE id=$1", runID))
	if e != nil {
		return GenerationOutcome{}, e
	}
	e = tx.Commit(ctx)
	return GenerationOutcome{d, run}, e
}

func (r *PostgresRepository) RecordMockGenerationFailure(ctx context.Context, req FailureRequest) (WorkflowRun, error) {
	tx, e := r.db.Begin(ctx)
	if e != nil {
		return WorkflowRun{}, e
	}
	defer tx.Rollback(ctx)
	d, e := r.detail(ctx, tx, req.ContentItemID)
	if e != nil {
		return WorkflowRun{}, e
	}
	prior, e := scanRun(tx.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_runs WHERE project_id=$1 AND content_item_id=$2 AND workflow_key='content_mock_generate' AND idempotency_key=$3", d.Item.ProjectID, req.ContentItemID, req.IdempotencyKey))
	if e == nil {
		if prior.RequestFingerprint != req.Fingerprint {
			return WorkflowRun{}, ErrIdempotencyConflict
		}
		return prior, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return WorkflowRun{}, e
	}
	if !safeFailure(req.ErrorCode, req.ErrorSummary) {
		return WorkflowRun{}, ErrVersionConflict
	}
	id := uuid.New()
	_, e = tx.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,error_code,error_summary,finished_at) VALUES($1,$2,$3,$4,'mock','content_mock_generate','content_item',$3,'failed',$5,$6,COALESCE($7,'{}'::jsonb),'{}'::jsonb,$8,$9,NOW())", id, d.Item.ProjectID, req.ContentItemID, d.CurrentVersion.ID, req.IdempotencyKey, req.Fingerprint, req.InputJSON, req.ErrorCode, req.ErrorSummary)
	if e != nil {
		return WorkflowRun{}, fmt.Errorf("workflow failure create: %w", e)
	}
	out, e := scanRun(tx.QueryRow(ctx, "SELECT "+runColumns+" FROM workflow_runs WHERE id=$1", id))
	if e != nil {
		return WorkflowRun{}, e
	}
	e = tx.Commit(ctx)
	return out, e
}

// safeFailure is deliberately conservative: this persistence boundary accepts only
// caller-sanitised operational errors, never database diagnostics, stacks, or prompts.
func safeFailure(code, summary string) bool {
	if strings.TrimSpace(code) == "" || len(code) > 120 || len(summary) > 5000 {
		return false
	}
	lower := strings.ToLower(summary)
	for _, forbidden := range []string{"sql", "postgres", "stack", "prompt", "traceback", "\n", "\r"} {
		if strings.Contains(lower, forbidden) {
			return false
		}
	}
	return true
}

package chapterplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrBatchNotFound     = errors.New("candidate batch not found")
	ErrCandidateNotFound = errors.New("candidate not found")
	ErrRevisionNotFound  = errors.New("chapter plan revision not found")
)

const candidateBatchCols = "id,project_id,source_workflow_run_id,generation_mode,range_start,range_end,requested_chapter_count,input_digest,input_snapshot,storyline_selection_snapshot,context_options,additional_instructions,workflow_binding_snapshot,candidate_count,pending_count,stale_count,adopted_count,discarded_count,status,version,completed_at,abandoned_at,abandon_reason,created_by,updated_by,created_at,updated_at"

func scanBatch(r pgx.Row) (CandidateBatch, error) {
	var b CandidateBatch
	var rStart, rEnd *int
	var addInst, abReason *string
	var inputSnap, stSnap, ctxOpts, bindSnap []byte

	err := r.Scan(
		&b.ID, &b.ProjectID, &b.SourceWorkflowRunID, &b.GenerationMode,
		&rStart, &rEnd, &b.RequestedChapterCount, &b.InputDigest,
		&inputSnap, &stSnap, &ctxOpts, &addInst, &bindSnap,
		&b.CandidateCount, &b.PendingCount, &b.StaleCount, &b.AdoptedCount, &b.DiscardedCount,
		&b.Status, &b.Version, &b.CompletedAt, &b.AbandonedAt, &abReason,
		&b.CreatedBy, &b.UpdatedBy, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return b, err
	}

	b.RangeStart = rStart
	b.RangeEnd = rEnd
	b.AdditionalInstructions = addInst
	b.AbandonReason = abReason
	b.InputSnapshot = inputSnap
	b.StorylineSnapshot = stSnap
	b.ContextOptions = ctxOpts
	b.WorkflowBindingSnapshot = bindSnap

	if rStart != nil {
		b.Target.StartChapterNo = *rStart
	} else {
		b.Target.StartChapterNo = 1
	}
	if rEnd != nil {
		b.Target.EndChapterNo = *rEnd
	} else {
		b.Target.EndChapterNo = b.Target.StartChapterNo + b.RequestedChapterCount - 1
	}
	b.Target.RequestedChapterCount = b.RequestedChapterCount

	parseInputSummary(&b)
	return b, nil
}

func parseInputSummary(b *CandidateBatch) {
	summary := InputSummary{
		GenerationMode: b.GenerationMode,
		Target:         b.Target,
	}

	if len(b.StorylineSnapshot) > 0 && json.Valid(b.StorylineSnapshot) && string(b.StorylineSnapshot) != "{}" {
		summary.StorylineSelection = b.StorylineSnapshot
	} else {
		summary.StorylineSelection = json.RawMessage(`{"mode":"auto_balanced"}`)
	}

	if len(b.ContextOptions) > 0 && json.Valid(b.ContextOptions) && string(b.ContextOptions) != "{}" {
		summary.ContextOptions = b.ContextOptions
	} else {
		summary.ContextOptions = json.RawMessage(`{"includeProjectMaterials":true,"includeUnpaidForeshadowings":true,"includePriorChapterSummaries":true,"coreSettingsOnly":false}`)
	}

	b.InputSummary = summary
}

const candidateCols = "id,batch_id,project_id,chapter_no,sort_order,base_chapter_plan_id,base_revision_id,base_chapter_version,base_snapshot,generated_snapshot,current_snapshot,diff_type,status,adopted_chapter_plan_id,adopted_revision_id,adopted_at,discarded_at,discard_reason,created_by,updated_by,last_edited_by,last_edited_at,version,created_at,updated_at"

func scanCandidate(r pgx.Row) (Candidate, error) {
	var c Candidate
	var baseSnap []byte

	err := r.Scan(
		&c.ID, &c.BatchID, &c.ProjectID, &c.ChapterNo, &c.SortOrder,
		&c.BaseChapterPlanID, &c.BaseRevisionID, &c.BaseChapterPlanVersion,
		&baseSnap, &c.GeneratedSnapshot, &c.CurrentSnapshot,
		&c.DiffType, &c.Status,
		&c.AdoptedChapterPlanID, &c.AdoptedRevisionID, &c.AdoptedAt, &c.DiscardedAt, &c.DiscardReason,
		&c.CreatedBy, &c.UpdatedBy, &c.LastEditedBy, &c.LastEditedAt,
		&c.Version, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return c, err
	}
	c.BaseSnapshot = baseSnap
	c.BaseChapterVersion = c.BaseChapterPlanVersion
	return c, nil
}

const revisionCols = "id,chapter_plan_id,project_id,revision_no,snapshot,change_type,source_candidate_id,source_candidate_batch_id,source_workflow_run_id,created_by,created_at"

func scanRevision(r pgx.Row) (Revision, error) {
	var rev Revision
	err := r.Scan(
		&rev.ID, &rev.ChapterPlanID, &rev.ProjectID, &rev.RevisionNo,
		&rev.Snapshot, &rev.ChangeType,
		&rev.SourceCandidateID, &rev.SourceCandidateBatchID, &rev.SourceWorkflowRunID,
		&rev.CreatedBy, &rev.CreatedAt,
	)
	return rev, err
}

func (r *Repository) ListCandidateBatches(ctx context.Context, projectID uuid.UUID, f BatchFilter) (BatchListResult, error) {
	where := []string{"project_id = $1"}
	args := []any{projectID}
	argID := 2

	if f.Status != nil && *f.Status != "" {
		where = append(where, "status = $"+strconv.Itoa(argID))
		args = append(args, *f.Status)
		argID++
	}
	if f.GenerationMode != nil && *f.GenerationMode != "" {
		where = append(where, "generation_mode = $"+strconv.Itoa(argID))
		args = append(args, *f.GenerationMode)
		argID++
	}
	if f.SourceWorkflowRunID != nil {
		where = append(where, "source_workflow_run_id = $"+strconv.Itoa(argID))
		args = append(args, *f.SourceWorkflowRunID)
		argID++
	}
	if f.CreatedAtFrom != nil {
		where = append(where, "created_at >= $"+strconv.Itoa(argID))
		args = append(args, *f.CreatedAtFrom)
		argID++
	}
	if f.CreatedAtTo != nil {
		where = append(where, "created_at <= $"+strconv.Itoa(argID))
		args = append(args, *f.CreatedAtTo)
		argID++
	}

	whereClause := strings.Join(where, " AND ")

	countQuery := "SELECT COUNT(*) FROM chapter_plan_candidate_batches WHERE " + whereClause
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return BatchListResult{}, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf("SELECT %s FROM chapter_plan_candidate_batches WHERE %s ORDER BY created_at DESC, id DESC LIMIT %d OFFSET %d",
		candidateBatchCols, whereClause, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return BatchListResult{}, err
	}
	defer rows.Close()

	items := make([]CandidateBatch, 0)
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return BatchListResult{}, err
		}
		items = append(items, b)
	}

	return BatchListResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, rows.Err()
}

func (r *Repository) GetCandidateBatchByID(ctx context.Context, batchID uuid.UUID) (CandidateBatch, error) {
	query := fmt.Sprintf("SELECT %s FROM chapter_plan_candidate_batches WHERE id = $1", candidateBatchCols)
	b, err := scanBatch(r.db.QueryRow(ctx, query, batchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateBatch{}, ErrBatchNotFound
	}
	return b, err
}

func (r *Repository) ListCandidates(ctx context.Context, batchID uuid.UUID, f CandidateFilter) (CandidateListResult, error) {
	where := []string{"batch_id = $1"}
	args := []any{batchID}
	argID := 2

	if f.Status != nil && *f.Status != "" {
		where = append(where, "status = $"+strconv.Itoa(argID))
		args = append(args, *f.Status)
		argID++
	}
	if f.DiffType != nil && *f.DiffType != "" {
		where = append(where, "diff_type = $"+strconv.Itoa(argID))
		args = append(args, *f.DiffType)
		argID++
	}
	if f.StorylineID != nil {
		where = append(where, fmt.Sprintf("current_snapshot->'storylineRefs' @> '[{\"id\":\"%s\"}]'::jsonb", f.StorylineID.String()))
	}
	if f.Q != nil && strings.TrimSpace(*f.Q) != "" {
		qPattern := "%" + strings.TrimSpace(*f.Q) + "%"
		where = append(where, fmt.Sprintf("(current_snapshot->>'title' ILIKE $%d OR current_snapshot->>'summary' ILIKE $%d OR current_snapshot->>'chapterPurpose' ILIKE $%d OR current_snapshot->'generationBasis'->>'contextSummary' ILIKE $%d)", argID, argID, argID, argID))
		args = append(args, qPattern)
		argID++
	}

	whereClause := strings.Join(where, " AND ")

	countQuery := "SELECT COUNT(*) FROM chapter_plan_candidates WHERE " + whereClause
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return CandidateListResult{}, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf("SELECT %s FROM chapter_plan_candidates WHERE %s ORDER BY chapter_no ASC, id ASC LIMIT %d OFFSET %d",
		candidateCols, whereClause, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return CandidateListResult{}, err
	}
	defer rows.Close()

	items := make([]Candidate, 0)
	for rows.Next() {
		c, err := scanCandidate(rows)
		if err != nil {
			return CandidateListResult{}, err
		}
		items = append(items, c)
	}

	return CandidateListResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, rows.Err()
}

func (r *Repository) GetCandidateByID(ctx context.Context, candidateID uuid.UUID) (Candidate, error) {
	query := fmt.Sprintf("SELECT %s FROM chapter_plan_candidates WHERE id = $1", candidateCols)
	c, err := scanCandidate(r.db.QueryRow(ctx, query, candidateID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Candidate{}, ErrCandidateNotFound
	}
	return c, err
}

func (r *Repository) ListRevisions(ctx context.Context, chapterPlanID uuid.UUID, limit, offset int) (RevisionListResult, error) {
	countQuery := "SELECT COUNT(*) FROM chapter_plan_revisions WHERE chapter_plan_id = $1"
	var total int
	if err := r.db.QueryRow(ctx, countQuery, chapterPlanID).Scan(&total); err != nil {
		return RevisionListResult{}, err
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf("SELECT %s FROM chapter_plan_revisions WHERE chapter_plan_id = $1 ORDER BY revision_no DESC LIMIT %d OFFSET %d",
		revisionCols, limit, offset)

	rows, err := r.db.Query(ctx, query, chapterPlanID)
	if err != nil {
		return RevisionListResult{}, err
	}
	defer rows.Close()

	items := make([]Revision, 0)
	for rows.Next() {
		rev, err := scanRevision(rows)
		if err != nil {
			return RevisionListResult{}, err
		}
		items = append(items, rev)
	}

	return RevisionListResult{
		Items:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, rows.Err()
}

func (r *Repository) GetChapterPlanningSummary(ctx context.Context, projectID uuid.UUID) (Summary, error) {
	var summary Summary

	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plans WHERE project_id = $1", projectID).Scan(&summary.CurrentChapterCount)
	if err != nil {
		return summary, err
	}

	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plans WHERE project_id = $1 AND status = 'pending_confirmation'", projectID).Scan(&summary.PendingConfirmationCount)
	if err != nil {
		return summary, err
	}

	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plans WHERE project_id = $1 AND status = 'confirmed'", projectID).Scan(&summary.ConfirmedChapterCount)
	if err != nil {
		return summary, err
	}

	var batchCounts CandidateBatchCounts
	_ = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_candidate_batches WHERE project_id = $1 AND status = 'ready'", projectID).Scan(&batchCounts.Ready)
	_ = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_candidate_batches WHERE project_id = $1 AND status = 'partially_adopted'", projectID).Scan(&batchCounts.PartiallyAdopted)
	_ = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_candidate_batches WHERE project_id = $1 AND status = 'adopted'", projectID).Scan(&batchCounts.Adopted)
	_ = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_candidate_batches WHERE project_id = $1 AND status = 'abandoned'", projectID).Scan(&batchCounts.Abandoned)
	summary.CandidateBatchCounts = batchCounts

	var activeRunPayload []byte
	activeRunQuery := `SELECT json_build_object(
		'id', id,
		'runNumber', run_number,
		'projectId', project_id,
		'stage', stage,
		'workflowConfigurationId', workflow_configuration_id,
		'triggerSource', trigger_source,
		'status', status,
		'configurationSnapshot', configuration_snapshot,
		'inputPayload', input_payload,
		'outputPayload', output_payload,
		'errorCode', error_code,
		'errorMessage', error_message,
		'errorDetails', error_details,
		'startedAt', started_at,
		'finishedAt', finished_at,
		'cancelledAt', cancelled_at,
		'createdAt', created_at,
		'updatedAt', updated_at,
		'version', version
	)::jsonb FROM workflow_run_records WHERE project_id = $1 AND stage = 'chapter_planning' AND status IN ('queued', 'running') ORDER BY created_at DESC LIMIT 1`

	err = r.db.QueryRow(ctx, activeRunQuery, projectID).Scan(&activeRunPayload)
	if errors.Is(err, pgx.ErrNoRows) || len(activeRunPayload) == 0 {
		summary.ActiveRun = json.RawMessage("null")
	} else if err == nil {
		summary.ActiveRun = activeRunPayload
	}

	return summary, nil
}

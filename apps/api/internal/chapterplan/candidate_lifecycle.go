package chapterplan

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrBatchAlreadyFinalized    = errors.New("batch is already finalized")
	ErrIdempotencyKeyReused     = errors.New("idempotency key reused with different payload")
	ErrRevisionSequenceConflict = errors.New("revision sequence conflict")

	hmacSecretKey = []byte("cf15-chapter-planning-idempotency-secret-v1")
)

type AdoptCandidateCommand struct {
	CandidateID                uuid.UUID `json:"candidateId"`
	ExpectedCandidateVersion   int       `json:"expectedCandidateVersion"`
	ExpectedChapterPlanVersion *int      `json:"expectedChapterPlanVersion"`
	IdempotencyKey             string    `json:"idempotencyKey"`
	ActorID                    string    `json:"actorId"`
}

type AdoptCandidateResult struct {
	Outcome     string         `json:"outcome"` // "adopted" or "no_change"
	Candidate   Candidate      `json:"candidate"`
	ChapterPlan *Plan          `json:"chapterPlan"`
	Revision    *Revision      `json:"revision"`
	Batch       CandidateBatch `json:"batch"`
}

type DiscardCandidateCommand struct {
	CandidateID              uuid.UUID `json:"candidateId"`
	ExpectedCandidateVersion int       `json:"expectedCandidateVersion"`
	Reason                   *string   `json:"reason"`
	IdempotencyKey           string    `json:"idempotencyKey"`
	ActorID                  string    `json:"actorId"`
}

type BulkAdoptCandidateItemCommand struct {
	CandidateID                uuid.UUID `json:"candidateId"`
	ExpectedCandidateVersion   int       `json:"expectedCandidateVersion"`
	ExpectedChapterPlanVersion *int      `json:"expectedChapterPlanVersion"`
}

type BulkAdoptCommand struct {
	BatchID              uuid.UUID                       `json:"batchId"`
	ExpectedBatchVersion int                             `json:"expectedBatchVersion"`
	Candidates           []BulkAdoptCandidateItemCommand `json:"candidates"`
	IdempotencyKey       string                          `json:"idempotencyKey"`
	ActorID              string                          `json:"actorId"`
}

type BulkAdoptItemResult struct {
	CandidateID uuid.UUID      `json:"candidateId"`
	Outcome     string         `json:"outcome"` // "adopted", "no_change", "stale", "conflict", "failed"
	Candidate   *Candidate     `json:"candidate,omitempty"`
	ChapterPlan *Plan          `json:"chapterPlan,omitempty"`
	Revision    *Revision      `json:"revision,omitempty"`
	Error       map[string]any `json:"error,omitempty"`
}

type BulkAdoptResult struct {
	Items []BulkAdoptItemResult `json:"items"`
	Batch CandidateBatch        `json:"batch"`
}

type AbandonBatchCommand struct {
	BatchID                            uuid.UUID `json:"batchId"`
	ExpectedBatchVersion               int       `json:"expectedBatchVersion"`
	Reason                             *string   `json:"reason"`
	AcknowledgeAdoptedChaptersRemain   bool      `json:"acknowledgeAdoptedChaptersRemain"`
	IdempotencyKey                     string    `json:"idempotencyKey"`
	ActorID                            string    `json:"actorId"`
}

func deriveKeyFingerprint(rawKey string) string {
	mac := hmac.New(sha256.New, hmacSecretKey)
	mac.Write([]byte(rawKey))
	return hex.EncodeToString(mac.Sum(nil))
}

func hashPayload(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func acquireAdvisoryLock(ctx context.Context, tx pgx.Tx, scope, key string) error {
	sum := sha256.Sum256([]byte(scope + ":" + key))
	lockKey := int64(uint64(sum[0])<<56 | uint64(sum[1])<<48 | uint64(sum[2])<<40 | uint64(sum[3])<<32 | uint64(sum[4])<<24 | uint64(sum[5])<<16 | uint64(sum[6])<<8 | uint64(sum[7]))
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey)
	return err
}

func checkIdempotency(ctx context.Context, tx pgx.Tx, scope, key, reqHash string) ([]byte, int, bool, error) {
	var dbHash string
	var status int
	var respBody []byte

	err := tx.QueryRow(ctx, `
		SELECT request_hash, response_status, response_body
		FROM idempotency_records
		WHERE scope = $1 AND idempotency_key = $2
	`, scope, key).Scan(&dbHash, &status, &respBody)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}

	if dbHash != reqHash {
		return nil, 0, true, ErrIdempotencyKeyReused
	}

	return respBody, status, true, nil
}

func recordIdempotency(ctx context.Context, tx pgx.Tx, scope, key, reqHash string, status int, respObj any) error {
	respBody, err := json.Marshal(respObj)
	if err != nil {
		return err
	}
	id := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO idempotency_records (id, scope, idempotency_key, request_hash, response_status, response_body, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, id, scope, key, reqHash, status, respBody)
	return err
}

func recordAuditLog(ctx context.Context, tx pgx.Tx, actor, action, subjectType string, subjectID uuid.UUID, payload map[string]any) error {
	cleanPayload := make(map[string]any)
	for k, v := range payload {
		if k == "token" || k == "idempotencyKey" || k == "rawKey" {
			continue
		}
		cleanPayload[k] = v
	}
	pBytes, _ := json.Marshal(cleanPayload)
	id := uuid.New()
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_logs (id, actor_id, action, subject_type, subject_id, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`, id, actor, action, subjectType, subjectID.String(), pBytes)
	return err
}

func replaceChapterPlanRelations(ctx context.Context, tx pgx.Tx, planID, projectID uuid.UUID, candidateSnap []byte) error {
	var snap candidateSnapshotStruct
	if err := json.Unmarshal(candidateSnap, &snap); err != nil {
		return fmt.Errorf("unmarshal candidate snapshot: %w", err)
	}

	if _, err := tx.Exec(ctx, "DELETE FROM chapter_plan_storylines WHERE chapter_plan_id = $1", planID); err != nil {
		return fmt.Errorf("delete chapter_plan_storylines: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM chapter_plan_materials WHERE chapter_plan_id = $1", planID); err != nil {
		return fmt.Errorf("delete chapter_plan_materials: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM chapter_plan_foreshadowings WHERE chapter_plan_id = $1", planID); err != nil {
		return fmt.Errorf("delete chapter_plan_foreshadowings: %w", err)
	}

	hasPrimary := false
	for i, ref := range snap.StorylineRefs {
		rel := ref.Relation
		if rel != "primary" && rel != "secondary" {
			if !hasPrimary {
				rel = "primary"
			} else {
				rel = "secondary"
			}
		}
		if rel == "primary" {
			hasPrimary = true
		}
		pos := ref.Position
		if pos < 0 {
			pos = i
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO chapter_plan_storylines (chapter_plan_id, project_id, storyline_id, relation, position)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (chapter_plan_id, storyline_id) DO UPDATE SET relation = EXCLUDED.relation, position = EXCLUDED.position
		`, planID, projectID, ref.ID, rel, pos)
		if err != nil {
			return fmt.Errorf("insert storyline relation: %w", err)
		}
	}

	for i, ref := range snap.MaterialRefs {
		pos := ref.Position
		if pos < 0 {
			pos = i
		}
		_, _ = tx.Exec(ctx, `
			INSERT INTO project_material_usages (id, project_id, material_id, usage_type, created_by)
			VALUES ($1, $2, $3, 'reference', 'system')
			ON CONFLICT (project_id, material_id) DO NOTHING
		`, uuid.New(), projectID, ref.ID)

		_, err := tx.Exec(ctx, `
			INSERT INTO chapter_plan_materials (chapter_plan_id, project_id, material_id, position)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (chapter_plan_id, material_id) DO UPDATE SET position = EXCLUDED.position
		`, planID, projectID, ref.ID, pos)
		if err != nil {
			return fmt.Errorf("insert material relation: %w", err)
		}
	}

	for i, ref := range snap.ForeshadowingRefs {
		pos := ref.Position
		if pos < 0 {
			pos = i
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO chapter_plan_foreshadowings (chapter_plan_id, project_id, foreshadowing_id, position)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (chapter_plan_id, foreshadowing_id) DO UPDATE SET position = EXCLUDED.position
		`, planID, projectID, ref.ID, pos)
		if err != nil {
			return fmt.Errorf("insert foreshadowing relation: %w", err)
		}
	}

	return nil
}

type candidateSnapshotStruct struct {
	ChapterNo         int                   `json:"chapterNo"`
	Title             string                `json:"title"`
	Summary           string                `json:"summary"`
	ChapterPurpose    string                `json:"chapterPurpose"`
	StorylineRefs     []NormalizedReference `json:"storylineRefs"`
	MaterialRefs      []NormalizedReference `json:"materialRefs"`
	ForeshadowingRefs []NormalizedReference `json:"foreshadowingRefs"`
	GenerationBasis   GenerationBasis       `json:"generationBasis"`
}

func (r *Repository) AdoptCandidate(ctx context.Context, cmd AdoptCandidateCommand) (AdoptCandidateResult, error) {
	if len(cmd.IdempotencyKey) == 0 {
		return AdoptCandidateResult{}, fmt.Errorf("%w: idempotency key is required", ErrInvalidCandidateState)
	}
	keyFp := r.computeHMACKeyFingerprint(cmd.IdempotencyKey)
	scope := fmt.Sprintf("chapter_plan_candidate:%s", cmd.CandidateID)
	reqHash := hashPayload(cmd)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return AdoptCandidateResult{}, err
	}
	defer tx.Rollback(ctx)

	if err := acquireAdvisoryLock(ctx, tx, scope, keyFp); err != nil {
		return AdoptCandidateResult{}, err
	}

	storedBody, _, found, err := checkIdempotency(ctx, tx, scope, keyFp, reqHash)
	if err != nil {
		return AdoptCandidateResult{}, err
	}
	if found {
		var res AdoptCandidateResult
		if err := json.Unmarshal(storedBody, &res); err != nil {
			return AdoptCandidateResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return AdoptCandidateResult{}, err
		}
		return res, nil
	}

	cand, err := scanCandidate(tx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM chapter_plan_candidates WHERE id = $1 FOR UPDATE", candidateCols), cmd.CandidateID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AdoptCandidateResult{}, ErrCandidateNotFound
	}
	if err != nil {
		return AdoptCandidateResult{}, err
	}

	if cand.Status == "adopted" || cand.Status == "discarded" {
		return AdoptCandidateResult{}, ErrInvalidCandidateState
	}
	if cand.Version != cmd.ExpectedCandidateVersion {
		return AdoptCandidateResult{}, ErrVersionConflict
	}

	batch, err := scanBatch(tx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM chapter_plan_candidate_batches WHERE id = $1 FOR UPDATE", candidateBatchCols), cand.BatchID))
	if err != nil {
		return AdoptCandidateResult{}, err
	}

	targetPlan, targetRevID, err := r.findTargetChapterPlan(ctx, tx, cand.ProjectID, cand.ChapterNo)
	if err != nil {
		return AdoptCandidateResult{}, err
	}

	if cand.Status == "stale" || cand.DiffType == "stale_conflict" {
		return AdoptCandidateResult{}, ErrStaleCandidate
	}
	if targetPlan != nil && cand.BaseRevisionID != nil && targetRevID != nil && *cand.BaseRevisionID != *targetRevID {
		return AdoptCandidateResult{}, ErrStaleCandidate
	}

	if targetPlan != nil {
		if cmd.ExpectedChapterPlanVersion == nil || *cmd.ExpectedChapterPlanVersion != targetPlan.Version {
			return AdoptCandidateResult{}, ErrVersionConflict
		}
	} else {
		if cmd.ExpectedChapterPlanVersion != nil && *cmd.ExpectedChapterPlanVersion != 0 {
			return AdoptCandidateResult{}, ErrVersionConflict
		}
	}

	currSnap := cand.CurrentSnapshot
	var targetSnap []byte
	if targetPlan != nil {
		targetSnap = r.getPlanSnapshotJSON(ctx, tx, targetPlan)
	}

	if targetPlan != nil && len(targetSnap) > 0 && isSameSnapshot(currSnap, targetSnap) {
		batchSummary := batch
		res := AdoptCandidateResult{
			Outcome:     "no_change",
			Candidate:   cand,
			ChapterPlan: targetPlan,
			Revision:    nil,
			Batch:       batchSummary,
		}
		if err := recordIdempotency(ctx, tx, scope, keyFp, reqHash, 200, res); err != nil {
			return AdoptCandidateResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return AdoptCandidateResult{}, err
		}
		return res, nil
	}

	actor := cmd.ActorID
	if actor == "" {
		actor = "system"
	}

	var snapObj candidateSnapshotStruct
	_ = json.Unmarshal(currSnap, &snapObj)

	title := snapObj.Title
	if title == "" {
		title = fmt.Sprintf("Chapter %d", cand.ChapterNo)
	}
	summaryText := snapObj.Summary

	var planID uuid.UUID
	var revNo int
	var committedPlan Plan

	if targetPlan == nil {
		// 1. Create base ChapterPlan with current_revision_id = NULL
		planID = uuid.New()
		revNo = 1
		createPlanQuery := `INSERT INTO chapter_plans (
			id, project_id, chapter_no, title, summary,
			status, source, current_revision_id, source_candidate_id, source_candidate_batch_id,
			source_workflow_run_id, created_by, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			'pending_confirmation', 'candidate_adopted', NULL, $6, $7,
			$8, $9, 1, NOW(), NOW()
		)`
		_, err := tx.Exec(ctx, createPlanQuery, planID, cand.ProjectID, cand.ChapterNo, title, summaryText, cand.ID, cand.BatchID, batch.SourceWorkflowRunID, actor)
		if err != nil {
			return AdoptCandidateResult{}, classifyAdoptErr(err)
		}
	} else {
		planID = targetPlan.ID
		var maxRev int
		_ = tx.QueryRow(ctx, "SELECT COALESCE(MAX(revision_no), 0) FROM chapter_plan_revisions WHERE chapter_plan_id = $1", planID).Scan(&maxRev)
		revNo = maxRev + 1
	}

	// 2. Create Revision
	revID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO chapter_plan_revisions (
			id, chapter_plan_id, project_id, revision_no, snapshot,
			change_type, source_candidate_id, source_candidate_batch_id, source_workflow_run_id,
			created_by, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			'candidate_adopt', $6, $7, $8,
			$9, NOW()
		)
	`, revID, planID, cand.ProjectID, revNo, currSnap, cand.ID, cand.BatchID, batch.SourceWorkflowRunID, actor)
	if err != nil {
		return AdoptCandidateResult{}, classifyAdoptErr(err)
	}

	// 3. Replace ChapterPlan relationship tables
	if err := replaceChapterPlanRelations(ctx, tx, planID, cand.ProjectID, currSnap); err != nil {
		return AdoptCandidateResult{}, err
	}

	// 4. Update ChapterPlan current_revision_id and scalar fields
	if targetPlan == nil {
		upQuery := `UPDATE chapter_plans
			SET current_revision_id = $2, updated_at = NOW()
			WHERE id = $1 RETURNING ` + cols
		committedPlan, err = scan(tx.QueryRow(ctx, upQuery, planID, revID))
		if err != nil {
			return AdoptCandidateResult{}, err
		}
	} else {
		upQuery := `UPDATE chapter_plans
			SET title = $3, summary = $4, status = 'pending_confirmation', source = 'candidate_adopted',
			    current_revision_id = $5, source_candidate_id = $6, source_candidate_batch_id = $7,
			    source_workflow_run_id = $8, version = version + 1, updated_at = NOW()
			WHERE id = $1 AND version = $2 RETURNING ` + cols
		committedPlan, err = scan(tx.QueryRow(ctx, upQuery, planID, targetPlan.Version, title, summaryText, revID, cand.ID, cand.BatchID, batch.SourceWorkflowRunID))
		if errors.Is(err, pgx.ErrNoRows) {
			return AdoptCandidateResult{}, ErrVersionConflict
		}
		if err != nil {
			return AdoptCandidateResult{}, classifyAdoptErr(err)
		}
	}
	_ = r.loadRefs(ctx, &committedPlan)

	// 5. Update Candidate
	now := time.Now()
	upCandQuery := `UPDATE chapter_plan_candidates
		SET status = 'adopted', adopted_chapter_plan_id = $2, adopted_revision_id = $3, adopted_at = $4, version = version + 1, updated_at = NOW()
		WHERE id = $1 RETURNING ` + candidateCols
	updatedCand, err := scanCandidate(tx.QueryRow(ctx, upCandQuery, cand.ID, committedPlan.ID, revID, now))
	if err != nil {
		return AdoptCandidateResult{}, err
	}

	// 6. Recalculate Batch
	batch, err = r.recalculateBatchInTx(ctx, tx, batch.ID)
	if err != nil {
		return AdoptCandidateResult{}, err
	}

	rev, err := scanRevision(tx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM chapter_plan_revisions WHERE id = $1", revisionCols), revID))
	if err != nil {
		return AdoptCandidateResult{}, err
	}

	res := AdoptCandidateResult{
		Outcome:     "adopted",
		Candidate:   updatedCand,
		ChapterPlan: &committedPlan,
		Revision:    &rev,
		Batch:       batch,
	}

	if err := recordAuditLog(ctx, tx, actor, "candidate.adopted", "chapter_plan_candidate", cand.ID, map[string]any{"candidate_id": cand.ID, "chapter_plan_id": committedPlan.ID, "revision_id": rev.ID}); err != nil {
		return AdoptCandidateResult{}, err
	}

	if err := recordIdempotency(ctx, tx, scope, keyFp, reqHash, 200, res); err != nil {
		return AdoptCandidateResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return AdoptCandidateResult{}, err
	}

	return res, nil
}

func classifyAdoptErr(err error) error {
	var x *pgconn.PgError
	if errors.As(err, &x) {
		if x.Code == "23505" {
			if x.ConstraintName == "chapter_plans_project_chapter_no_unique" {
				return ErrChapterNoConflict
			}
			if x.ConstraintName == "chapter_plan_revisions_chapter_plan_no_unique" {
				return ErrRevisionSequenceConflict
			}
		}
	}
	return err
}

func (r *Repository) recalculateBatchInTx(ctx context.Context, tx pgx.Tx, batchID uuid.UUID) (CandidateBatch, error) {
	var pendingCount, staleCount, adoptedCount, discardedCount int
	err := tx.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status = 'stale'),
			COUNT(*) FILTER (WHERE status = 'adopted'),
			COUNT(*) FILTER (WHERE status = 'discarded')
		FROM chapter_plan_candidates
		WHERE batch_id = $1
	`, batchID).Scan(&pendingCount, &staleCount, &adoptedCount, &discardedCount)

	if err != nil {
		return CandidateBatch{}, err
	}

	newStatus := "ready"
	var currentStatus string
	_ = tx.QueryRow(ctx, "SELECT status FROM chapter_plan_candidate_batches WHERE id = $1", batchID).Scan(&currentStatus)
	if currentStatus == "abandoned" {
		newStatus = "abandoned"
	} else if adoptedCount > 0 && (pendingCount > 0 || staleCount > 0) {
		newStatus = "partially_adopted"
	} else if (pendingCount == 0 && staleCount == 0) && adoptedCount > 0 {
		newStatus = "adopted"
	}

	var completedAt *time.Time
	if pendingCount == 0 && staleCount == 0 {
		now := time.Now()
		completedAt = &now
	}

	q := `UPDATE chapter_plan_candidate_batches
		SET pending_count = $2, stale_count = $3, adopted_count = $4, discarded_count = $5,
		    status = $6, completed_at = COALESCE(completed_at, $7), version = version + 1, updated_at = NOW()
		WHERE id = $1 RETURNING ` + candidateBatchCols

	return scanBatch(tx.QueryRow(ctx, q, batchID, pendingCount, staleCount, adoptedCount, discardedCount, newStatus, completedAt))
}

func (r *Repository) BulkAdoptCandidates(ctx context.Context, cmd BulkAdoptCommand) (BulkAdoptResult, error) {
	if len(cmd.IdempotencyKey) == 0 {
		return BulkAdoptResult{}, fmt.Errorf("%w: idempotency key is required", ErrInvalidCandidateState)
	}
	keyFp := r.computeHMACKeyFingerprint(cmd.IdempotencyKey)
	scope := fmt.Sprintf("chapter_plan_bulk_adopt:%s", cmd.BatchID)
	reqHash := hashPayload(cmd)

	conn, err := r.db.Acquire(ctx)
	if err != nil {
		return BulkAdoptResult{}, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	lockID := advisoryLockID(scope, keyFp)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		return BulkAdoptResult{}, fmt.Errorf("acquire session lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockID)
	}()

	// Query existing idempotency record using dedicated conn
	var storedBody []byte
	var storedStatus int
	var storedHash string
	err = conn.QueryRow(ctx, `
		SELECT response_body, response_status, request_hash
		FROM idempotency_records
		WHERE scope = $1 AND idempotency_key = $2
	`, scope, keyFp).Scan(&storedBody, &storedStatus, &storedHash)

	if err == nil {
		if storedHash != reqHash {
			return BulkAdoptResult{}, ErrIdempotencyConflict
		}
		var res BulkAdoptResult
		if err := json.Unmarshal(storedBody, &res); err != nil {
			return BulkAdoptResult{}, err
		}
		return res, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return BulkAdoptResult{}, err
	}

	batch, err := r.GetCandidateBatchByID(ctx, cmd.BatchID)
	if errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
		return BulkAdoptResult{}, ErrBatchNotFound
	}
	if err != nil {
		return BulkAdoptResult{}, err
	}

	if batch.Status == "abandoned" {
		return BulkAdoptResult{}, ErrBatchAlreadyFinalized
	}
	if batch.Version != cmd.ExpectedBatchVersion {
		return BulkAdoptResult{}, ErrVersionConflict
	}

	itemResults := make([]BulkAdoptItemResult, 0, len(cmd.Candidates))

	for i, item := range cmd.Candidates {
		itemKey := fmt.Sprintf("%s:item:%s:%d", keyFp, item.CandidateID, i)
		itemRes, err := r.AdoptCandidate(ctx, AdoptCandidateCommand{
			CandidateID:                item.CandidateID,
			ExpectedCandidateVersion:   item.ExpectedCandidateVersion,
			ExpectedChapterPlanVersion: item.ExpectedChapterPlanVersion,
			IdempotencyKey:             itemKey,
			ActorID:                    cmd.ActorID,
		})

		if err == nil {
			itemResults = append(itemResults, BulkAdoptItemResult{
				CandidateID: item.CandidateID,
				Outcome:     itemRes.Outcome,
				Candidate:   &itemRes.Candidate,
				ChapterPlan: itemRes.ChapterPlan,
				Revision:    itemRes.Revision,
			})
		} else {
			cand, _ := r.GetCandidateByID(ctx, item.CandidateID)
			var candPtr *Candidate
			if cand.ID != uuid.Nil {
				candPtr = &cand
			}

			outcome := "failed"
			errCode := "failed"
			if errors.Is(err, ErrStaleCandidate) {
				outcome = "stale"
				errCode = "stale_candidate"
			} else if errors.Is(err, ErrVersionConflict) || errors.Is(err, ErrChapterNoConflict) {
				outcome = "conflict"
				errCode = "version_conflict"
			}

			itemResults = append(itemResults, BulkAdoptItemResult{
				CandidateID: item.CandidateID,
				Outcome:     outcome,
				Candidate:   candPtr,
				Error: map[string]any{
					"code":    errCode,
					"message": err.Error(),
				},
			})
		}
	}

	finalBatch, err := r.GetCandidateBatchByID(ctx, cmd.BatchID)
	if err != nil {
		return BulkAdoptResult{}, fmt.Errorf("query final batch: %w", err)
	}
	result := BulkAdoptResult{
		Items: itemResults,
		Batch: finalBatch,
	}

	txFinal, err := r.db.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("begin final tx: %w", err)
	}
	defer txFinal.Rollback(ctx)

	actor := cmd.ActorID
	if actor == "" {
		actor = "system"
	}

	if err := recordAuditLog(ctx, txFinal, actor, "candidate_batch.bulk_adopted", "chapter_plan_candidate_batch", cmd.BatchID, map[string]any{"batch_id": cmd.BatchID, "item_count": len(itemResults)}); err != nil {
		return result, fmt.Errorf("record audit: %w", err)
	}

	if err := recordIdempotency(ctx, txFinal, scope, keyFp, reqHash, 200, result); err != nil {
		return result, fmt.Errorf("record idempotency: %w", err)
	}

	if err := txFinal.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit final tx: %w", err)
	}

	return result, nil
}

func (r *Repository) DiscardCandidate(ctx context.Context, cmd DiscardCandidateCommand) (Candidate, error) {
	if len(cmd.IdempotencyKey) == 0 {
		return Candidate{}, fmt.Errorf("%w: idempotency key is required", ErrInvalidCandidateState)
	}
	keyFp := r.computeHMACKeyFingerprint(cmd.IdempotencyKey)
	scope := fmt.Sprintf("chapter_plan_candidate:%s", cmd.CandidateID)
	reqHash := hashPayload(cmd)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Candidate{}, err
	}
	defer tx.Rollback(ctx)

	if err := acquireAdvisoryLock(ctx, tx, scope, keyFp); err != nil {
		return Candidate{}, err
	}

	storedBody, _, found, err := checkIdempotency(ctx, tx, scope, keyFp, reqHash)
	if err != nil {
		return Candidate{}, err
	}
	if found {
		var res Candidate
		if err := json.Unmarshal(storedBody, &res); err != nil {
			return Candidate{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Candidate{}, err
		}
		return res, nil
	}

	cand, err := scanCandidate(tx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM chapter_plan_candidates WHERE id = $1 FOR UPDATE", candidateCols), cmd.CandidateID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Candidate{}, ErrCandidateNotFound
	}
	if err != nil {
		return Candidate{}, err
	}

	if cand.Status == "adopted" || cand.Status == "discarded" {
		return Candidate{}, ErrInvalidCandidateState
	}
	if cand.Version != cmd.ExpectedCandidateVersion {
		return Candidate{}, ErrVersionConflict
	}

	now := time.Now()
	actor := cmd.ActorID
	if actor == "" {
		actor = "system"
	}

	upQuery := `UPDATE chapter_plan_candidates
		SET status = 'discarded', discarded_at = $2, discard_reason = $3, version = version + 1, updated_by = $4, updated_at = NOW()
		WHERE id = $1 AND version = $5 RETURNING ` + candidateCols

	updatedCand, err := scanCandidate(tx.QueryRow(ctx, upQuery, cand.ID, now, cmd.Reason, actor, cmd.ExpectedCandidateVersion))
	if errors.Is(err, pgx.ErrNoRows) {
		return Candidate{}, ErrVersionConflict
	}
	if err != nil {
		return Candidate{}, err
	}

	_, err = r.recalculateBatchInTx(ctx, tx, cand.BatchID)
	if err != nil {
		return Candidate{}, err
	}

	if err := recordAuditLog(ctx, tx, actor, "candidate.discarded", "chapter_plan_candidate", cand.ID, map[string]any{"candidate_id": cand.ID, "reason": cmd.Reason}); err != nil {
		return Candidate{}, err
	}

	if err := recordIdempotency(ctx, tx, scope, keyFp, reqHash, 200, updatedCand); err != nil {
		return Candidate{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Candidate{}, err
	}
	return updatedCand, nil
}

func (r *Repository) AbandonBatch(ctx context.Context, cmd AbandonBatchCommand) (CandidateBatch, error) {
	if len(cmd.IdempotencyKey) == 0 {
		return CandidateBatch{}, fmt.Errorf("%w: idempotency key is required", ErrInvalidCandidateState)
	}
	keyFp := r.computeHMACKeyFingerprint(cmd.IdempotencyKey)
	scope := fmt.Sprintf("chapter_plan_batch:%s", cmd.BatchID)
	reqHash := hashPayload(cmd)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CandidateBatch{}, err
	}
	defer tx.Rollback(ctx)

	if err := acquireAdvisoryLock(ctx, tx, scope, keyFp); err != nil {
		return CandidateBatch{}, err
	}

	storedBody, _, found, err := checkIdempotency(ctx, tx, scope, keyFp, reqHash)
	if err != nil {
		return CandidateBatch{}, err
	}
	if found {
		var res CandidateBatch
		if err := json.Unmarshal(storedBody, &res); err != nil {
			return CandidateBatch{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return CandidateBatch{}, err
		}
		return res, nil
	}

	batch, err := scanBatch(tx.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM chapter_plan_candidate_batches WHERE id = $1 FOR UPDATE", candidateBatchCols), cmd.BatchID))
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateBatch{}, ErrBatchNotFound
	}
	if err != nil {
		return CandidateBatch{}, err
	}

	if batch.Status == "abandoned" || batch.Status == "adopted" {
		return CandidateBatch{}, ErrBatchAlreadyFinalized
	}
	if batch.Version != cmd.ExpectedBatchVersion {
		return CandidateBatch{}, ErrVersionConflict
	}

	actor := cmd.ActorID
	if actor == "" {
		actor = "system"
	}
	now := time.Now()

	_, err = tx.Exec(ctx, `
		UPDATE chapter_plan_candidates
		SET status = 'discarded', discarded_at = $2, discard_reason = $3, version = version + 1, updated_by = $4, updated_at = NOW()
		WHERE batch_id = $1 AND status IN ('pending', 'stale')
	`, batch.ID, now, cmd.Reason, actor)
	if err != nil {
		return CandidateBatch{}, err
	}

	upBatchQuery := `UPDATE chapter_plan_candidate_batches
		SET status = 'abandoned', abandoned_at = $2, abandon_reason = $3, version = version + 1, updated_by = $4, updated_at = NOW()
		WHERE id = $1 AND version = $5 RETURNING ` + candidateBatchCols

	abandonedBatch, err := scanBatch(tx.QueryRow(ctx, upBatchQuery, batch.ID, now, cmd.Reason, actor, cmd.ExpectedBatchVersion))
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateBatch{}, ErrVersionConflict
	}
	if err != nil {
		return CandidateBatch{}, err
	}

	finalBatch, err := r.recalculateBatchInTx(ctx, tx, batch.ID)
	if err == nil {
		abandonedBatch = finalBatch
	}

	if err := recordAuditLog(ctx, tx, actor, "candidate_batch.abandoned", "chapter_plan_candidate_batch", batch.ID, map[string]any{"batch_id": batch.ID, "reason": cmd.Reason}); err != nil {
		return CandidateBatch{}, err
	}

	if err := recordIdempotency(ctx, tx, scope, keyFp, reqHash, 200, abandonedBatch); err != nil {
		return CandidateBatch{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CandidateBatch{}, err
	}
	return abandonedBatch, nil
}

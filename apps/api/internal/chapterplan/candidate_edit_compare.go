package chapterplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrInvalidCandidateState = errors.New("invalid candidate state for mutation")
	ErrStaleCandidate        = errors.New("candidate baseline is stale")
)

type DiffEntry struct {
	Path       string  `json:"path"`
	ChangeType string  `json:"changeType"`
	Before     *string `json:"before"`
	After      *string `json:"after"`
}

type CandidateDiff struct {
	BaseRevisionID   *uuid.UUID  `json:"baseRevisionId"`
	CandidateVersion int         `json:"candidateVersion"`
	Entries          []DiffEntry `json:"entries"`
	Stale            bool        `json:"stale"`
}

type CandidateComparison struct {
	Candidate      Candidate     `json:"candidate"`
	CurrentChapter *Plan         `json:"currentChapter"`
	Diff           CandidateDiff `json:"diff"`
}

type UpdateCandidateCommand struct {
	CandidateID              uuid.UUID       `json:"candidateId"`
	ExpectedCandidateVersion int             `json:"expectedCandidateVersion"`
	CurrentSnapshot          json.RawMessage `json:"currentSnapshot"`
	IdempotencyKey           string          `json:"idempotencyKey"`
	ActorID                  string          `json:"actorId"`
}

type RecompareCandidateCommand struct {
	CandidateID              uuid.UUID `json:"candidateId"`
	ExpectedCandidateVersion int       `json:"expectedCandidateVersion"`
	IdempotencyKey           string    `json:"idempotencyKey"`
	ActorID                  string    `json:"actorId"`
}

func (r *Repository) UpdateCandidate(ctx context.Context, cmd UpdateCandidateCommand) (Candidate, error) {
	if len(cmd.IdempotencyKey) == 0 {
		return Candidate{}, fmt.Errorf("%w: idempotency key is required", ErrInvalidCandidateState)
	}
	keyFp := r.computeHMACKeyFingerprint(cmd.IdempotencyKey)
	scope := fmt.Sprintf("chapter-plan-candidate-update:%s", cmd.CandidateID)
	reqHash, err := hashPayload(cmd)
	if err != nil {
		return Candidate{}, err
	}

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

	query := fmt.Sprintf("SELECT %s FROM chapter_plan_candidates WHERE id = $1 FOR UPDATE", candidateCols)
	cand, err := scanCandidate(tx.QueryRow(ctx, query, cmd.CandidateID))
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

	targetPlan, targetRevID, err := r.findTargetChapterPlan(ctx, tx, cand.ProjectID, cand.ChapterNo)
	if err != nil {
		return Candidate{}, err
	}

	diffType, isStale := ComputeDiffTypeAndStale(cmd.CurrentSnapshot, cand.BaseSnapshot, cand.BaseRevisionID, targetPlan, targetRevID)

	newStatus := cand.Status
	if isStale || cand.Status == "stale" {
		newStatus = "stale"
		diffType = "stale_conflict"
	} else {
		newStatus = "pending"
	}

	actor := cmd.ActorID
	if actor == "" {
		actor = "system"
	}

	updateQuery := `
		UPDATE chapter_plan_candidates
		SET current_snapshot = $2,
		    diff_type = $3,
		    status = $4,
		    version = version + 1,
		    last_edited_by = $5,
		    last_edited_at = NOW(),
		    updated_by = $5,
		    updated_at = NOW()
		WHERE id = $1 AND version = $6
		RETURNING ` + candidateCols

	updatedCand, err := scanCandidate(tx.QueryRow(ctx, updateQuery, cmd.CandidateID, cmd.CurrentSnapshot, diffType, newStatus, actor, cmd.ExpectedCandidateVersion))
	if errors.Is(err, pgx.ErrNoRows) {
		return Candidate{}, ErrVersionConflict
	}
	if err != nil {
		return Candidate{}, err
	}

	if err := recordAuditLog(ctx, tx, actor, "candidate.updated", "chapter_plan_candidate", cand.ID, map[string]any{"candidate_id": cand.ID, "version": updatedCand.Version}); err != nil {
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

func (r *Repository) CompareCandidate(ctx context.Context, candidateID uuid.UUID) (CandidateComparison, error) {
	cand, err := r.GetCandidateByID(ctx, candidateID)
	if err != nil {
		return CandidateComparison{}, err
	}

	var targetPlan *Plan
	plan, err := r.GetChapterPlanByProjectAndNumber(ctx, cand.ProjectID, cand.ChapterNo)
	if err == nil {
		targetPlan = &plan
	}

	diff := CalculateCandidateDiff(cand, targetPlan)
	var currentChapterPtr *Plan
	if targetPlan != nil {
		currentChapterPtr = targetPlan
	}

	return CandidateComparison{
		Candidate:      cand,
		CurrentChapter: currentChapterPtr,
		Diff:           diff,
	}, nil
}

func (r *Repository) RecompareCandidate(ctx context.Context, cmd RecompareCandidateCommand) (CandidateComparison, error) {
	if len(cmd.IdempotencyKey) == 0 {
		return CandidateComparison{}, fmt.Errorf("%w: idempotency key is required", ErrInvalidCandidateState)
	}
	keyFp := r.computeHMACKeyFingerprint(cmd.IdempotencyKey)
	scope := fmt.Sprintf("chapter_plan_candidate:%s", cmd.CandidateID)
	reqHash, err := hashPayload(map[string]any{
		"op":           "recompare",
		"candidate_id": cmd.CandidateID,
		"expected_ver": cmd.ExpectedCandidateVersion,
	})
	if err != nil {
		return CandidateComparison{}, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CandidateComparison{}, err
	}
	defer tx.Rollback(ctx)

	if err := acquireAdvisoryLock(ctx, tx, scope, keyFp); err != nil {
		return CandidateComparison{}, err
	}

	storedBody, _, found, err := checkIdempotency(ctx, tx, scope, keyFp, reqHash)
	if err != nil {
		return CandidateComparison{}, err
	}
	if found {
		var res CandidateComparison
		if err := json.Unmarshal(storedBody, &res); err != nil {
			return CandidateComparison{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return CandidateComparison{}, err
		}
		return res, nil
	}

	query := fmt.Sprintf("SELECT %s FROM chapter_plan_candidates WHERE id = $1 FOR UPDATE", candidateCols)
	cand, err := scanCandidate(tx.QueryRow(ctx, query, cmd.CandidateID))
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateComparison{}, ErrCandidateNotFound
	}
	if err != nil {
		return CandidateComparison{}, err
	}

	if cand.Status == "adopted" || cand.Status == "discarded" {
		return CandidateComparison{}, ErrInvalidCandidateState
	}
	if cand.Version != cmd.ExpectedCandidateVersion {
		return CandidateComparison{}, ErrVersionConflict
	}

	targetPlan, targetRevID, err := r.findTargetChapterPlan(ctx, tx, cand.ProjectID, cand.ChapterNo)
	if err != nil {
		return CandidateComparison{}, err
	}

	diffType, isStale := ComputeDiffTypeAndStale(cand.CurrentSnapshot, cand.BaseSnapshot, cand.BaseRevisionID, targetPlan, targetRevID)
	newStatus := "pending"
	if isStale || cand.Status == "stale" {
		newStatus = "stale"
		diffType = "stale_conflict"
	}

	actor := cmd.ActorID
	if actor == "" {
		actor = "system"
	}

	// Recompare MUST NOT update base_chapter_plan_id, base_revision_id, base_chapter_version, base_snapshot, or generated_snapshot.
	updateQuery := `
		UPDATE chapter_plan_candidates
		SET diff_type = $2,
		    status = $3,
		    version = version + 1,
		    updated_by = $4,
		    updated_at = NOW()
		WHERE id = $1 AND version = $5
		RETURNING ` + candidateCols

	updatedCand, err := scanCandidate(tx.QueryRow(ctx, updateQuery, cand.ID, diffType, newStatus, actor, cmd.ExpectedCandidateVersion))
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateComparison{}, ErrVersionConflict
	}
	if err != nil {
		return CandidateComparison{}, err
	}

	diff := CalculateCandidateDiff(updatedCand, targetPlan)
	var currentChapterPtr *Plan
	if targetPlan != nil {
		currentChapterPtr = targetPlan
	}

	res := CandidateComparison{
		Candidate:      updatedCand,
		CurrentChapter: currentChapterPtr,
		Diff:           diff,
	}

	if err := recordAuditLog(ctx, tx, actor, "candidate.recompared", "chapter_plan_candidate", cand.ID, map[string]any{"candidate_id": cand.ID, "version": updatedCand.Version}); err != nil {
		return CandidateComparison{}, err
	}

	if err := recordIdempotency(ctx, tx, scope, keyFp, reqHash, 200, res); err != nil {
		return CandidateComparison{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CandidateComparison{}, err
	}

	return res, nil
}

func (r *Repository) GetChapterPlanByProjectAndNumber(ctx context.Context, projectID uuid.UUID, chapterNo int) (Plan, error) {
	p, err := scan(r.db.QueryRow(ctx, "SELECT "+cols+" FROM chapter_plans WHERE project_id = $1 AND chapter_no = $2", projectID, chapterNo))
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, ErrNotFound
	}
	if err == nil {
		err = r.loadRefs(ctx, &p)
	}
	return p, err
}

func (r *Repository) findTargetChapterPlan(ctx context.Context, tx pgx.Tx, projectID uuid.UUID, chapterNo int) (*Plan, *uuid.UUID, error) {
	var p Plan
	var currentRevID *uuid.UUID
	row := tx.QueryRow(ctx, "SELECT "+cols+", current_revision_id FROM chapter_plans WHERE project_id = $1 AND chapter_no = $2", projectID, chapterNo)
	err := row.Scan(
		&p.ID, &p.ProjectID, &p.RunID, &p.ChapterNo, &p.Title, &p.Summary, &p.Goal, &p.Notes,
		&p.Status, &p.Source, &p.CreatedBy, &p.ConfirmedAt, &p.Version, &p.CreatedAt, &p.UpdatedAt,
		&currentRevID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return &p, currentRevID, nil
}

func (r *Repository) getPlanSnapshotJSON(ctx context.Context, tx pgx.Tx, plan *Plan) []byte {
	if plan == nil {
		return nil
	}
	var revSnap []byte
	err := tx.QueryRow(ctx, "SELECT snapshot FROM chapter_plan_revisions WHERE chapter_plan_id = $1 ORDER BY revision_no DESC LIMIT 1", plan.ID).Scan(&revSnap)
	if err == nil && len(revSnap) > 0 {
		return revSnap
	}
	snapMap := map[string]any{
		"chapterNo":         plan.ChapterNo,
		"title":             plan.Title,
		"summary":           plan.Summary,
		"chapterPurpose":    "other",
		"storylineRefs":     plan.Storylines,
		"materialRefs":      plan.Materials,
		"foreshadowingRefs": plan.Foreshadowings,
		"generationBasis": map[string]any{
			"contextSummary": "",
		},
	}
	b, _ := json.Marshal(snapMap)
	return b
}

func ComputeDiffTypeAndStale(currentSnap, baseSnap []byte, baseRevID *uuid.UUID, targetPlan *Plan, targetRevID *uuid.UUID) (diffType string, isStale bool) {
	if targetPlan == nil {
		return "new", false
	}
	if baseRevID != nil && targetRevID != nil && *baseRevID != *targetRevID {
		return "stale_conflict", true
	}
	if baseRevID != nil && targetRevID == nil {
		return "stale_conflict", true
	}

	if len(baseSnap) > 0 && isSameSnapshot(currentSnap, baseSnap) {
		return "no_change", false
	}
	return "replace", false
}

func CalculateCandidateDiff(c Candidate, targetPlan *Plan) CandidateDiff {
	entries := make([]DiffEntry, 0)
	var currentMap map[string]any
	var baseMap map[string]any

	_ = json.Unmarshal(c.CurrentSnapshot, &currentMap)
	if len(c.BaseSnapshot) > 0 {
		_ = json.Unmarshal(c.BaseSnapshot, &baseMap)
	}

	fields := []string{"title", "summary", "chapterPurpose", "storylineRefs", "materialRefs", "foreshadowingRefs"}
	for _, f := range fields {
		currVal, hasCurr := currentMap[f]
		baseVal, hasBase := baseMap[f]

		var beforeStr, afterStr *string
		if hasBase && baseVal != nil {
			s := fmt.Sprintf("%v", baseVal)
			beforeStr = &s
		}
		if hasCurr && currVal != nil {
			s := fmt.Sprintf("%v", currVal)
			afterStr = &s
		}

		changeType := "unchanged"
		if !hasBase && hasCurr {
			changeType = "added"
		} else if hasBase && !hasCurr {
			changeType = "removed"
		} else if !reflect.DeepEqual(currVal, baseVal) {
			changeType = "changed"
		}

		entries = append(entries, DiffEntry{
			Path:       f,
			ChangeType: changeType,
			Before:     beforeStr,
			After:      afterStr,
		})
	}

	isStale := c.Status == "stale" || c.DiffType == "stale_conflict"
	return CandidateDiff{
		BaseRevisionID:   c.BaseRevisionID,
		CandidateVersion: c.Version,
		Entries:          entries,
		Stale:            isStale,
	}
}

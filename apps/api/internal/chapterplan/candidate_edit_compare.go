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
	Candidate      Candidate      `json:"candidate"`
	CurrentChapter *Plan          `json:"currentChapter"`
	Diff           CandidateDiff  `json:"diff"`
}

type UpdateCandidateCommand struct {
	CandidateID             uuid.UUID       `json:"candidateId"`
	ExpectedCandidateVersion int             `json:"expectedCandidateVersion"`
	CurrentSnapshot         json.RawMessage `json:"currentSnapshot"`
	ActorID                 string          `json:"actorId"`
}

type RecompareCandidateCommand struct {
	CandidateID             uuid.UUID `json:"candidateId"`
	ExpectedCandidateVersion int       `json:"expectedCandidateVersion"`
	ActorID                 string    `json:"actorId"`
}

func (r *Repository) UpdateCandidate(ctx context.Context, cmd UpdateCandidateCommand) (Candidate, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Candidate{}, err
	}
	defer tx.Rollback(ctx)

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

	// Read current target chapter plan (if exists)
	targetPlan, targetRevID, err := r.findTargetChapterPlan(ctx, tx, cand.ProjectID, cand.ChapterNo)
	if err != nil {
		return Candidate{}, err
	}

	diffType, isStale := ComputeDiffTypeAndStale(cmd.CurrentSnapshot, cand.BaseSnapshot, cand.BaseRevisionID, targetPlan, targetRevID)

	newStatus := cand.Status
	if isStale {
		newStatus = "stale"
		diffType = "stale_conflict"
	} else if cand.Status == "stale" {
		// keep stale unless recompared
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
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return CandidateComparison{}, err
	}
	defer tx.Rollback(ctx)

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

	var basePlanID *uuid.UUID
	var baseRevID *uuid.UUID
	var baseVer *int
	var baseSnap []byte

	if targetPlan != nil {
		basePlanID = &targetPlan.ID
		baseRevID = targetRevID
		baseVer = &targetPlan.Version
		baseSnap = r.getPlanSnapshotJSON(ctx, tx, targetPlan)
	}

	diffType, _ := ComputeDiffTypeAndStale(cand.CurrentSnapshot, baseSnap, baseRevID, targetPlan, targetRevID)
	newStatus := "pending"

	actor := cmd.ActorID
	if actor == "" {
		actor = "system"
	}

	updateQuery := `
		UPDATE chapter_plan_candidates
		SET base_chapter_plan_id = $2,
		    base_revision_id = $3,
		    base_chapter_version = $4,
		    base_snapshot = $5,
		    diff_type = $6,
		    status = $7,
		    version = version + 1,
		    updated_by = $8,
		    updated_at = NOW()
		WHERE id = $1 AND version = $9
		RETURNING ` + candidateCols

	updatedCand, err := scanCandidate(tx.QueryRow(ctx, updateQuery, cand.ID, basePlanID, baseRevID, baseVer, baseSnap, diffType, newStatus, actor, cmd.ExpectedCandidateVersion))
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateComparison{}, ErrVersionConflict
	}
	if err != nil {
		return CandidateComparison{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return CandidateComparison{}, err
	}

	diff := CalculateCandidateDiff(updatedCand, targetPlan)
	var currentChapterPtr *Plan
	if targetPlan != nil {
		currentChapterPtr = targetPlan
	}

	return CandidateComparison{
		Candidate:      updatedCand,
		CurrentChapter: currentChapterPtr,
		Diff:           diff,
	}, nil
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
	// Fallback to building snapshot object
	snapMap := map[string]any{
		"chapterNo":      plan.ChapterNo,
		"title":          plan.Title,
		"summary":        plan.Summary,
		"chapterPurpose": "other",
		"storylineRefs":  plan.Storylines,
		"materialRefs":   plan.Materials,
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

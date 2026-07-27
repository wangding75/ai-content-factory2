package chapterplan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrOutputValidationFailed = errors.New("normalized output validation failed")
	ErrRunAlreadyConsumed     = errors.New("run already consumed")
	ErrIngestionTransaction   = errors.New("result ingestion transaction failed")

	digestPattern = regexp.MustCompile("^[a-f0-9]{64}$")
)

type RunReference struct {
	RunID     uuid.UUID `json:"runId"`
	ProjectID uuid.UUID `json:"projectId"`
}

type BaseChapterPlanContext struct {
	ID         uuid.UUID       `json:"id"`
	ChapterNo  int             `json:"chapterNo"`
	Version    int             `json:"version"`
	RevisionID *uuid.UUID      `json:"revisionId"`
	Snapshot   json.RawMessage `json:"snapshot"`
}

type GenerationContextSnapshot struct {
	InputDigest             string                   `json:"inputDigest"`
	InputSnapshot           json.RawMessage          `json:"inputSnapshot"`
	StorylineSnapshot       json.RawMessage          `json:"storylineSnapshot"`
	ContextOptions          json.RawMessage          `json:"contextOptions"`
	AdditionalInstructions  *string                  `json:"additionalInstructions"`
	WorkflowBindingSnapshot json.RawMessage          `json:"workflowBindingSnapshot"`
	BaseChapterPlans        []BaseChapterPlanContext `json:"baseChapterPlans"`
}

type NormalizedReference struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"projectId"`
	Label     string    `json:"label"`
	Relation  string    `json:"relation"`
	Position  int       `json:"position"`
	Version   int       `json:"version"`
}

type GenerationBasis struct {
	ContextSummary         string  `json:"contextSummary"`
	AdditionalInstructions *string `json:"additionalInstructions"`
}

type NormalizedCandidate struct {
	ChapterNo         int                   `json:"chapterNo"`
	Title             string                `json:"title"`
	Summary           string                `json:"summary"`
	ChapterPurpose    string                `json:"chapterPurpose"`
	StorylineRefs     []NormalizedReference `json:"storylineRefs"`
	MaterialRefs      []NormalizedReference `json:"materialRefs"`
	ForeshadowingRefs []NormalizedReference `json:"foreshadowingRefs"`
	GenerationBasis   GenerationBasis       `json:"generationBasis"`
}

type OutputMetadata struct {
	InputDigest         string `json:"inputDigest"`
	GeneratedAt         string `json:"generatedAt"`
	SafeProviderSummary string `json:"safeProviderSummary"`
}

type NormalizedChapterPlanOutput struct {
	ProjectID           uuid.UUID             `json:"projectId"`
	GenerationMode      string                `json:"generationMode"`
	Target              BatchTarget           `json:"target"`
	SourceWorkflowRunID uuid.UUID             `json:"sourceWorkflowRunId"`
	Candidates          []NormalizedCandidate `json:"candidates"`
	Metadata            OutputMetadata        `json:"metadata"`
}

type IngestInput struct {
	Run              RunReference                `json:"run"`
	Context          GenerationContextSnapshot   `json:"context"`
	NormalizedOutput NormalizedChapterPlanOutput `json:"normalizedOutput"`
}

type Ingestor interface {
	Ingest(ctx context.Context, input IngestInput) (CandidateBatch, error)
}

type ResultIngestor struct {
	pool *pgxpool.Pool
}

func NewResultIngestor(pool *pgxpool.Pool) *ResultIngestor {
	return &ResultIngestor{pool: pool}
}

func ValidateNormalizedOutput(input IngestInput) error {
	out := input.NormalizedOutput
	run := input.Run
	ctx := input.Context

	if out.ProjectID == uuid.Nil || run.ProjectID == uuid.Nil || out.ProjectID != run.ProjectID {
		return fmt.Errorf("%w: output projectId %s does not match run projectId %s", ErrOutputValidationFailed, out.ProjectID, run.ProjectID)
	}
	if out.SourceWorkflowRunID == uuid.Nil || run.RunID == uuid.Nil || out.SourceWorkflowRunID != run.RunID {
		return fmt.Errorf("%w: output sourceWorkflowRunId %s does not match runId %s", ErrOutputValidationFailed, out.SourceWorkflowRunID, run.RunID)
	}

	if !digestPattern.MatchString(out.Metadata.InputDigest) {
		return fmt.Errorf("%w: invalid metadata inputDigest format %s", ErrOutputValidationFailed, out.Metadata.InputDigest)
	}
	if out.Metadata.InputDigest != ctx.InputDigest {
		return fmt.Errorf("%w: metadata inputDigest %s does not match context inputDigest %s", ErrOutputValidationFailed, out.Metadata.InputDigest, ctx.InputDigest)
	}
	var frozenInput struct {
		GenerationMode string      `json:"generationMode"`
		Target         BatchTarget `json:"target"`
	}
	if err := json.Unmarshal(ctx.InputSnapshot, &frozenInput); err != nil || frozenInput.GenerationMode == "" {
		return fmt.Errorf("%w: frozen generation input is invalid", ErrOutputValidationFailed)
	}

	if _, err := time.Parse(time.RFC3339, out.Metadata.GeneratedAt); err != nil {
		if _, err2 := time.Parse("2006-01-02T15:04:05Z07:00", out.Metadata.GeneratedAt); err2 != nil {
			return fmt.Errorf("%w: invalid generatedAt format %s", ErrOutputValidationFailed, out.Metadata.GeneratedAt)
		}
	}

	if len(out.Metadata.SafeProviderSummary) < 1 || len(out.Metadata.SafeProviderSummary) > 300 {
		return fmt.Errorf("%w: safeProviderSummary length must be between 1 and 300", ErrOutputValidationFailed)
	}

	mode := out.GenerationMode
	if mode != "full" && mode != "append" && mode != "range" {
		return fmt.Errorf("%w: invalid generationMode %s", ErrOutputValidationFailed, mode)
	}
	if mode != frozenInput.GenerationMode {
		return fmt.Errorf("%w: output generationMode does not match frozen mode", ErrOutputValidationFailed)
	}

	tgt := out.Target
	if tgt.StartChapterNo < 1 || tgt.StartChapterNo > 100 || tgt.EndChapterNo < tgt.StartChapterNo || tgt.EndChapterNo > 100 || tgt.RequestedChapterCount < 1 || tgt.RequestedChapterCount > 100 {
		return fmt.Errorf("%w: invalid target numbers start=%d end=%d count=%d", ErrOutputValidationFailed, tgt.StartChapterNo, tgt.EndChapterNo, tgt.RequestedChapterCount)
	}
	if tgt != frozenInput.Target {
		return fmt.Errorf("%w: output target does not match frozen target", ErrOutputValidationFailed)
	}

	if len(out.Candidates) == 0 || len(out.Candidates) > 100 {
		return fmt.Errorf("%w: candidates array length %d out of bounds [1, 100]", ErrOutputValidationFailed, len(out.Candidates))
	}
	if len(out.Candidates) != tgt.RequestedChapterCount {
		return fmt.Errorf("%w: candidates count %d does not match requested count %d", ErrOutputValidationFailed, len(out.Candidates), tgt.RequestedChapterCount)
	}
	if (tgt.EndChapterNo - tgt.StartChapterNo + 1) != tgt.RequestedChapterCount {
		return fmt.Errorf("%w: target range [%d, %d] size does not match requested count %d", ErrOutputValidationFailed, tgt.StartChapterNo, tgt.EndChapterNo, tgt.RequestedChapterCount)
	}

	seenChapters := make(map[int]bool)
	for i, c := range out.Candidates {
		if c.ChapterNo < tgt.StartChapterNo || c.ChapterNo > tgt.EndChapterNo {
			return fmt.Errorf("%w: candidate %d chapterNo %d out of range [%d, %d]", ErrOutputValidationFailed, i, c.ChapterNo, tgt.StartChapterNo, tgt.EndChapterNo)
		}
		if seenChapters[c.ChapterNo] {
			return fmt.Errorf("%w: duplicate chapterNo %d in candidates", ErrOutputValidationFailed, c.ChapterNo)
		}
		seenChapters[c.ChapterNo] = true

		if stringsTrimEmpty(c.Title) || len(c.Title) > 120 {
			return fmt.Errorf("%w: candidate %d title length must be between 1 and 120", ErrOutputValidationFailed, i)
		}
		if len(c.Summary) > 5000 {
			return fmt.Errorf("%w: candidate %d summary length exceeds 5000", ErrOutputValidationFailed, i)
		}
		if !validPurpose(c.ChapterPurpose) {
			return fmt.Errorf("%w: candidate %d invalid chapterPurpose %s", ErrOutputValidationFailed, i, c.ChapterPurpose)
		}

		if len(c.StorylineRefs) < 1 {
			return fmt.Errorf("%w: candidate %d storylineRefs must contain at least 1 item", ErrOutputValidationFailed, i)
		}

		allRefs := append([]NormalizedReference{}, c.StorylineRefs...)
		allRefs = append(allRefs, c.MaterialRefs...)
		allRefs = append(allRefs, c.ForeshadowingRefs...)

		for _, ref := range allRefs {
			if ref.ID == uuid.Nil {
				return fmt.Errorf("%w: candidate %d reference ID is nil", ErrOutputValidationFailed, i)
			}
			if ref.ProjectID != run.ProjectID {
				return fmt.Errorf("%w: candidate %d reference projectId mismatch", ErrOutputValidationFailed, i)
			}
			if len(ref.Label) < 1 || len(ref.Label) > 160 {
				return fmt.Errorf("%w: candidate %d reference label length invalid", ErrOutputValidationFailed, i)
			}
			if len(ref.Relation) > 40 {
				return fmt.Errorf("%w: candidate %d reference relation length invalid", ErrOutputValidationFailed, i)
			}
			if ref.Position < 0 || ref.Version < 1 {
				return fmt.Errorf("%w: candidate %d reference position/version invalid", ErrOutputValidationFailed, i)
			}
		}

		if len(c.GenerationBasis.ContextSummary) > 5000 {
			return fmt.Errorf("%w: candidate %d contextSummary length exceeds 5000", ErrOutputValidationFailed, i)
		}
		if c.GenerationBasis.AdditionalInstructions != nil && len(*c.GenerationBasis.AdditionalInstructions) > 2000 {
			return fmt.Errorf("%w: candidate %d additionalInstructions length exceeds 2000", ErrOutputValidationFailed, i)
		}
	}

	return nil
}

func stringsTrimEmpty(s string) bool {
	return len(bytes.TrimSpace([]byte(s))) == 0
}

func validPurpose(p string) bool {
	switch p {
	case "information_reveal", "plot_advance", "conflict_escalation", "transition", "atmosphere", "other":
		return true
	default:
		return false
	}
}

func (ing *ResultIngestor) Ingest(ctx context.Context, input IngestInput) (CandidateBatch, error) {
	tx, err := ing.pool.Begin(ctx)
	if err != nil {
		return CandidateBatch{}, fmt.Errorf("%w: begin tx failed: %v", ErrIngestionTransaction, err)
	}
	defer tx.Rollback(ctx)

	// Acquire advisory transaction lock on RunID to serialize concurrent ingestion requests
	runLockID := advisoryLockID("workflow_run", input.Run.RunID.String())
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", runLockID); err != nil {
		return CandidateBatch{}, fmt.Errorf("%w: acquire run lock failed: %v", ErrIngestionTransaction, err)
	}

	// Verify workflow run status and project ID in DB
	var projID uuid.UUID
	var runStage, runStatus string
	err = tx.QueryRow(ctx, "SELECT project_id, stage, status FROM workflow_run_records WHERE id = $1 FOR UPDATE", input.Run.RunID).Scan(&projID, &runStage, &runStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return CandidateBatch{}, fmt.Errorf("%w: workflow run not found", ErrOutputValidationFailed)
	}
	if err != nil {
		return CandidateBatch{}, fmt.Errorf("%w: query workflow run failed: %v", ErrIngestionTransaction, err)
	}
	if projID != input.Run.ProjectID {
		return CandidateBatch{}, fmt.Errorf("%w: workflow run project mismatch", ErrOutputValidationFailed)
	}
	if runStatus != "succeeded" {
		return CandidateBatch{}, fmt.Errorf("%w: workflow run is not succeeded", ErrOutputValidationFailed)
	}
	if runStage != "chapter_planning" {
		return CandidateBatch{}, fmt.Errorf("%w: workflow run stage mismatch", ErrOutputValidationFailed)
	}

	// Check if already consumed by a batch
	queryExisting := fmt.Sprintf("SELECT %s FROM chapter_plan_candidate_batches WHERE source_workflow_run_id = $1 FOR UPDATE", candidateBatchCols)
	existingBatch, err := scanBatch(tx.QueryRow(ctx, queryExisting, input.Run.RunID))
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return CandidateBatch{}, fmt.Errorf("%w: commit replay failed: %v", ErrIngestionTransaction, err)
		}
		return existingBatch, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return CandidateBatch{}, fmt.Errorf("%w: query existing batch failed: %v", ErrIngestionTransaction, err)
	}

	if err := ValidateNormalizedOutput(input); err != nil {
		return CandidateBatch{}, err
	}

	batchID := uuid.New()
	out := input.NormalizedOutput
	cSnapshot := input.Context

	inputSnapJSON := cSnapshot.InputSnapshot
	if len(inputSnapJSON) == 0 {
		inputSnapJSON = []byte("{}")
	}
	stSnapJSON := cSnapshot.StorylineSnapshot
	if len(stSnapJSON) == 0 {
		stSnapJSON = []byte("{}")
	}
	ctxOptsJSON := cSnapshot.ContextOptions
	if len(ctxOptsJSON) == 0 {
		ctxOptsJSON = []byte("{}")
	}
	bindSnapJSON := cSnapshot.WorkflowBindingSnapshot
	if len(bindSnapJSON) == 0 {
		bindSnapJSON = []byte("{}")
	}

	candCount := len(out.Candidates)

	_, err = tx.Exec(ctx, `
		INSERT INTO chapter_plan_candidate_batches (
			id, project_id, source_workflow_run_id, generation_mode, range_start, range_end,
			requested_chapter_count, input_digest, input_snapshot, storyline_selection_snapshot,
			context_options, additional_instructions, workflow_binding_snapshot, status,
			candidate_count, pending_count, stale_count, adopted_count, discarded_count,
			created_by, updated_by, version
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, 'ready',
			$14, $14, 0, 0, 0,
			'system', 'system', 1
		)
	`,
		batchID, input.Run.ProjectID, input.Run.RunID, out.GenerationMode, out.Target.StartChapterNo, out.Target.EndChapterNo,
		out.Target.RequestedChapterCount, out.Metadata.InputDigest, inputSnapJSON, stSnapJSON,
		ctxOptsJSON, cSnapshot.AdditionalInstructions, bindSnapJSON,
		candCount,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			// Concurrent insertion constraint hit -> query existing batch
			existingBatch, queryErr := scanBatch(ing.pool.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM chapter_plan_candidate_batches WHERE source_workflow_run_id = $1", candidateBatchCols), input.Run.RunID))
			if queryErr == nil {
				return existingBatch, nil
			}
		}
		return CandidateBatch{}, fmt.Errorf("%w: insert batch failed: %v", ErrIngestionTransaction, err)
	}

	baseMap := make(map[int]BaseChapterPlanContext)
	for _, bcp := range cSnapshot.BaseChapterPlans {
		baseMap[bcp.ChapterNo] = bcp
	}

	candidatesSorted := make([]NormalizedCandidate, len(out.Candidates))
	copy(candidatesSorted, out.Candidates)
	sort.SliceStable(candidatesSorted, func(i, j int) bool {
		return candidatesSorted[i].ChapterNo < candidatesSorted[j].ChapterNo
	})

	for i, c := range candidatesSorted {
		candID := uuid.New()
		candSnapJSON, err := json.Marshal(c)
		if err != nil {
			return CandidateBatch{}, fmt.Errorf("%w: marshal candidate snapshot failed: %v", ErrIngestionTransaction, err)
		}

		diffType := "new"
		var basePlanID *uuid.UUID
		var baseRevID *uuid.UUID
		var baseVer *int
		var baseSnap []byte

		if bcp, ok := baseMap[c.ChapterNo]; ok {
			basePlanID = &bcp.ID
			baseRevID = bcp.RevisionID
			baseVer = &bcp.Version
			baseSnap = bcp.Snapshot

			if isSameSnapshot(candSnapJSON, bcp.Snapshot) {
				diffType = "no_change"
			} else {
				diffType = "replace"
			}
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO chapter_plan_candidates (
				id, batch_id, project_id, chapter_no, sort_order,
				base_chapter_plan_id, base_revision_id, base_chapter_version, base_snapshot,
				generated_snapshot, current_snapshot, diff_type, status,
				created_by, updated_by, version
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9,
				$10, $10, $11, 'pending',
				'system', 'system', 1
			)
		`,
			candID, batchID, input.Run.ProjectID, c.ChapterNo, i,
			basePlanID, baseRevID, baseVer, baseSnap,
			candSnapJSON, diffType,
		)
		if err != nil {
			return CandidateBatch{}, fmt.Errorf("%w: insert candidate failed: %v", ErrIngestionTransaction, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return CandidateBatch{}, fmt.Errorf("%w: commit tx failed: %v", ErrIngestionTransaction, err)
	}

	return scanBatch(ing.pool.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM chapter_plan_candidate_batches WHERE id = $1", candidateBatchCols), batchID))
}

func isSameSnapshot(candJSON, baseJSON []byte) bool {
	if len(candJSON) == 0 || len(baseJSON) == 0 {
		return false
	}
	var o1, o2 map[string]any
	if json.Unmarshal(candJSON, &o1) != nil || json.Unmarshal(baseJSON, &o2) != nil {
		return false
	}
	return reflect.DeepEqual(o1, o2)
}

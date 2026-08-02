package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

// chapterPlanApplication is the narrow HTTP-facing application contract. It deliberately
// exposes no repository or workflow details to handlers.
type chapterPlanApplication interface {
	List(context.Context, uuid.UUID) ([]chapterplan.Plan, error)
	Get(context.Context, uuid.UUID) (chapterplan.Plan, error)
	GenerateMock(context.Context, uuid.UUID, chapterplan.MockGenerateCommand) (chapterplan.MockGenerateResult, error)
	Update(context.Context, uuid.UUID, chapterplan.UpdateCommand) (chapterplan.Plan, error)
	Delete(context.Context, uuid.UUID, int) error
	Confirm(context.Context, uuid.UUID, []chapterplan.Selection) ([]chapterplan.Plan, error)

	ListCandidateBatches(context.Context, uuid.UUID, chapterplan.BatchFilter) (chapterplan.BatchListResult, error)
	GetCandidateBatchByID(context.Context, uuid.UUID) (chapterplan.CandidateBatch, error)
	ListCandidates(context.Context, uuid.UUID, chapterplan.CandidateFilter) (chapterplan.CandidateListResult, error)
	GetCandidateByID(context.Context, uuid.UUID) (chapterplan.Candidate, error)
	ListRevisions(context.Context, uuid.UUID, int, int) (chapterplan.RevisionListResult, error)
	GetChapterPlanningSummary(context.Context, uuid.UUID) (chapterplan.Summary, error)
	UpdateCandidate(context.Context, chapterplan.UpdateCandidateCommand) (chapterplan.Candidate, error)
	CompareCandidate(context.Context, uuid.UUID) (chapterplan.CandidateComparison, error)
	RecompareCandidate(context.Context, chapterplan.RecompareCandidateCommand) (chapterplan.CandidateComparison, error)
	AdoptCandidate(context.Context, chapterplan.AdoptCandidateCommand) (chapterplan.AdoptCandidateResult, error)
	BulkAdoptCandidates(context.Context, chapterplan.BulkAdoptCommand) (chapterplan.BulkAdoptResult, error)
	DiscardCandidate(context.Context, chapterplan.DiscardCandidateCommand) (chapterplan.Candidate, error)
	AbandonBatch(context.Context, chapterplan.AbandonBatchCommand) (chapterplan.CandidateBatch, error)
}

type chapterPlanRunApplication interface {
	Preflight(context.Context, uuid.UUID, chapterplan.PreflightRequest) (chapterplan.PreflightResult, error)
	CreateChapterPlanningRun(context.Context, uuid.UUID, string, string, string) (workflowrun.WorkflowRun, error)
}

type chapterPlanStorylineRefResponse struct {
	StorylineID uuid.UUID `json:"storyline_id"`
	Relation    string    `json:"relation"`
}
type chapterPlanResponse struct {
	ID                     uuid.UUID                         `json:"id"`
	ProjectID              uuid.UUID                         `json:"project_id"`
	ChapterNo              int                               `json:"chapter_no"`
	Title                  string                            `json:"title"`
	Summary                string                            `json:"summary"`
	Status                 string                            `json:"status"`
	Source                 string                            `json:"source"`
	StorylineRefsJSON      []chapterPlanStorylineRefResponse `json:"storyline_refs_json"`
	MaterialRefsJSON       []uuid.UUID                       `json:"material_refs_json"`
	ForeshadowingRefsJSON  []uuid.UUID                       `json:"foreshadowing_refs_json"`
	ChapterGoal            *string                           `json:"chapter_goal"`
	CreationNotes          *string                           `json:"creation_notes"`
	ConfirmedAt            *string                           `json:"confirmed_at"`
	CurrentRevisionID      *uuid.UUID                        `json:"currentRevisionId"`
	SourceCandidateID      *uuid.UUID                        `json:"sourceCandidateId"`
	SourceCandidateBatchID *uuid.UUID                        `json:"sourceCandidateBatchId"`
	SourceWorkflowRunID    *uuid.UUID                        `json:"sourceWorkflowRunId"`
	Version                int                               `json:"version"`
	CreatedAt              string                            `json:"created_at"`
	UpdatedAt              string                            `json:"updated_at"`
}
type mockGenerationRunResponse struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"project_id"`
	ProviderKey string    `json:"provider_key"`
	WorkflowKey string    `json:"workflow_key"`
	Status      string    `json:"status"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
}
type mockGenerateChapterPlansRequest struct {
	TargetStorylineID            *string         `json:"target_storyline_id"`
	StartChapterNo               *int            `json:"start_chapter_no"`
	EndChapterNo                 *int            `json:"end_chapter_no"`
	ChapterCount                 *int            `json:"chapter_count"`
	IncludeMainStoryline         *bool           `json:"include_main_storyline"`
	IncludeChildStorylines       *bool           `json:"include_child_storylines"`
	IncludeProjectMaterials      *bool           `json:"include_project_materials"`
	IncludeUnpaidForeshadowings  *bool           `json:"include_unpaid_foreshadowings"`
	IncludePriorChapterSummaries *bool           `json:"include_prior_chapter_summaries"`
	SummaryLength                *string         `json:"summary_length"`
	ChapterPace                  *string         `json:"chapter_pace"`
	GenerationNotes              json.RawMessage `json:"generation_notes"`
}

func registerChapterPlanRoutes(mux *http.ServeMux, service chapterPlanApplication, registerLegacyMock bool) {
	if runs, ok := service.(chapterPlanRunApplication); ok {
		registerChapterPlanRunRoutes(mux, runs)
	}
	mux.HandleFunc("GET /api/v1/projects/{projectId}/chapter-plans", listChapterPlansHandler(service))
	mux.HandleFunc("GET /api/v1/chapter-plans/{chapterPlanId}", getChapterPlanHandler(service))
	mux.HandleFunc("PATCH /api/v1/chapter-plans/{chapterPlanId}", updateChapterPlanHandler(service))
	mux.HandleFunc("DELETE /api/v1/chapter-plans/{chapterPlanId}", deleteChapterPlanHandler(service))
	mux.HandleFunc("POST /api/v1/projects/{projectId}/chapter-plans/confirm", confirmChapterPlansHandler(service))
	if registerLegacyMock {
		mux.HandleFunc("POST /api/v1/projects/{projectId}/chapter-plans/mock-generate", generateMockChapterPlansHandler(service))
	}
	mux.HandleFunc("GET /api/v1/projects/{projectId}/chapter-plan-candidate-batches", listCandidateBatchesHandler(service))
	mux.HandleFunc("GET /api/v1/chapter-plan-candidate-batches/{batchId}", getCandidateBatchHandler(service))
	mux.HandleFunc("GET /api/v1/chapter-plan-candidate-batches/{batchId}/candidates", listCandidatesHandler(service))
	mux.HandleFunc("GET /api/v1/chapter-plan-candidates/{candidateId}", getCandidateHandler(service))
	mux.HandleFunc("GET /api/v1/chapter-plans/{chapterPlanId}/revisions", listRevisionsHandler(service))
	mux.HandleFunc("GET /api/v1/projects/{projectId}/chapter-planning-summary", getChapterPlanningSummaryHandler(service))
	mux.HandleFunc("PATCH /api/v1/chapter-plan-candidates/{candidateId}", updateCandidateHandler(service))
	mux.HandleFunc("GET /api/v1/chapter-plan-candidates/{candidateId}/compare", compareCandidateHandler(service))
	mux.HandleFunc("POST /api/v1/chapter-plan-candidates/{candidateId}/recompare", recompareCandidateHandler(service))
	mux.HandleFunc("POST /api/v1/chapter-plan-candidates/{candidateId}/adopt", adoptCandidateHandler(service))
	mux.HandleFunc("POST /api/v1/chapter-plan-candidate-batches/{batchId}/adoptions", bulkAdoptCandidatesHandler(service))
	mux.HandleFunc("POST /api/v1/chapter-plan-candidates/{candidateId}/discard", discardCandidateHandler(service))
	mux.HandleFunc("POST /api/v1/chapter-plan-candidate-batches/{batchId}/abandon", abandonBatchHandler(service))
}

func listChapterPlansHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		status := r.URL.Query().Get("status")
		if status != "" && status != "pending_confirmation" && status != "confirmed" {
			writeError(w, r, 400, "validation_error", "invalid status", map[string]any{})
			return
		}
		limit, offset, ok := listPagination(r)
		if !ok {
			writeError(w, r, 400, "validation_error", "invalid pagination", map[string]any{})
			return
		}
		items, err := service.List(r.Context(), id)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		items = append([]chapterplan.Plan(nil), items...)
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].ChapterNo != items[j].ChapterNo {
				return items[i].ChapterNo < items[j].ChapterNo
			}
			return items[i].ID.String() < items[j].ID.String()
		})
		filtered := make([]chapterplan.Plan, 0, len(items))
		for _, item := range items {
			if status == "" || item.Status == status {
				filtered = append(filtered, item)
			}
		}
		total := len(filtered)
		start := offset
		if start > total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}
		response := make([]chapterPlanResponse, 0, end-start)
		for _, item := range filtered[start:end] {
			response = append(response, chapterPlanResponseFrom(item))
		}
		writeJSON(w, r, http.StatusOK, map[string]any{"items": response, "total": total, "limit": limit, "offset": offset})
	}
}

func getChapterPlanHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("chapterPlanId"))
		if err != nil {
			writeError(w, r, 400, "invalid_uuid", "chapterPlanId must be a UUID", map[string]any{})
			return
		}
		value, err := service.Get(r.Context(), id)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, chapterPlanResponseFrom(value))
	}
}

func generateMockChapterPlansHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, ok := projectID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		command, err := decodeMockGenerateChapterPlans(r)
		if err != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		result, err := service.GenerateMock(r.Context(), projectID, command)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		items := make([]chapterPlanResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, chapterPlanResponseFrom(item))
		}
		writeJSON(w, r, http.StatusCreated, map[string]any{"run": mockGenerationRunResponse{ID: result.Run.ID, ProjectID: result.Run.ProjectID, ProviderKey: "mock", WorkflowKey: "chapter_plan_mock_generate", Status: "succeeded", CreatedAt: formatChapterPlanTime(result.Run.CreatedAt), UpdatedAt: formatChapterPlanTime(result.Run.UpdatedAt)}, "items": items})
	}
}

func decodeMockGenerateChapterPlans(r *http.Request) (chapterplan.MockGenerateCommand, error) {
	var body mockGenerateChapterPlansRequest
	if err := decodeBody(r, &body); err != nil || body.TargetStorylineID == nil || body.StartChapterNo == nil || body.EndChapterNo == nil || body.ChapterCount == nil || body.IncludeMainStoryline == nil || body.IncludeChildStorylines == nil || body.IncludeProjectMaterials == nil || body.IncludeUnpaidForeshadowings == nil || body.IncludePriorChapterSummaries == nil || body.SummaryLength == nil || body.ChapterPace == nil || body.GenerationNotes == nil {
		return chapterplan.MockGenerateCommand{}, errors.New("invalid request")
	}
	target, err := uuid.Parse(*body.TargetStorylineID)
	if err != nil {
		return chapterplan.MockGenerateCommand{}, err
	}
	var notes *string
	if string(body.GenerationNotes) != "null" {
		var note string
		if err := json.Unmarshal(body.GenerationNotes, &note); err != nil {
			return chapterplan.MockGenerateCommand{}, err
		}
		notes = &note
	}
	return chapterplan.MockGenerateCommand{TargetStorylineID: target, StartChapterNo: *body.StartChapterNo, EndChapterNo: *body.EndChapterNo, ChapterCount: *body.ChapterCount, IncludeMainStoryline: *body.IncludeMainStoryline, IncludeChildStorylines: *body.IncludeChildStorylines, IncludeProjectMaterials: *body.IncludeProjectMaterials, IncludeUnpaidForeshadowings: *body.IncludeUnpaidForeshadowings, IncludePriorChapterSummaries: *body.IncludePriorChapterSummaries, SummaryLength: *body.SummaryLength, ChapterPace: *body.ChapterPace, GenerationNotes: notes, ActorID: requestActorID(r)}, nil
}

func chapterPlanResponseFrom(value chapterplan.Plan) chapterPlanResponse {
	storylines := make([]chapterPlanStorylineRefResponse, 0, len(value.Storylines))
	for _, ref := range value.Storylines {
		storylines = append(storylines, chapterPlanStorylineRefResponse{StorylineID: ref.ID, Relation: ref.Relation})
	}
	var confirmedAt *string
	if value.ConfirmedAt != nil {
		formatted := formatChapterPlanTime(*value.ConfirmedAt)
		confirmedAt = &formatted
	}
	source := chapterPlanSourceResponse(value.Source)
	return chapterPlanResponse{
		ID:                     value.ID,
		ProjectID:              value.ProjectID,
		ChapterNo:              value.ChapterNo,
		Title:                  value.Title,
		Summary:                value.Summary,
		Status:                 value.Status,
		Source:                 source,
		StorylineRefsJSON:      storylines,
		MaterialRefsJSON:       nonNilUUIDs(value.Materials),
		ForeshadowingRefsJSON:  nonNilUUIDs(value.Foreshadowings),
		ChapterGoal:            value.Goal,
		CreationNotes:          value.Notes,
		ConfirmedAt:            confirmedAt,
		CurrentRevisionID:      value.CurrentRevisionID,
		SourceCandidateID:      value.SourceCandidateID,
		SourceCandidateBatchID: value.SourceCandidateBatchID,
		SourceWorkflowRunID:    value.SourceWorkflowRunID,
		Version:                value.Version,
		CreatedAt:              formatChapterPlanTime(value.CreatedAt),
		UpdatedAt:              formatChapterPlanTime(value.UpdatedAt),
	}
}

type candidateComparisonResponse struct {
	Candidate      chapterplan.Candidate     `json:"candidate"`
	CurrentChapter *chapterPlanResponse      `json:"currentChapter"`
	Diff           chapterplan.CandidateDiff `json:"diff"`
}

func candidateComparisonResponseFrom(value chapterplan.CandidateComparison) candidateComparisonResponse {
	var currentChapter *chapterPlanResponse
	if value.CurrentChapter != nil {
		response := chapterPlanResponseFrom(*value.CurrentChapter)
		currentChapter = &response
	}
	return candidateComparisonResponse{
		Candidate:      value.Candidate,
		CurrentChapter: currentChapter,
		Diff:           value.Diff,
	}
}

type adoptCandidateResponse struct {
	Outcome     string                     `json:"outcome"`
	Candidate   chapterplan.Candidate      `json:"candidate"`
	ChapterPlan *chapterPlanResponse       `json:"chapterPlan"`
	Revision    *chapterplan.Revision      `json:"revision"`
	Batch       chapterplan.CandidateBatch `json:"batch"`
}

func adoptCandidateResponseFrom(value chapterplan.AdoptCandidateResult) adoptCandidateResponse {
	var plan *chapterPlanResponse
	if value.ChapterPlan != nil {
		response := chapterPlanResponseFrom(*value.ChapterPlan)
		plan = &response
	}
	return adoptCandidateResponse{
		Outcome:     value.Outcome,
		Candidate:   value.Candidate,
		ChapterPlan: plan,
		Revision:    value.Revision,
		Batch:       value.Batch,
	}
}

type bulkAdoptItemResponse struct {
	CandidateID uuid.UUID              `json:"candidateId"`
	Outcome     string                 `json:"outcome"`
	Candidate   *chapterplan.Candidate `json:"candidate,omitempty"`
	ChapterPlan *chapterPlanResponse   `json:"chapterPlan,omitempty"`
	Revision    *chapterplan.Revision  `json:"revision,omitempty"`
	Error       map[string]any         `json:"error,omitempty"`
}

type bulkAdoptResponse struct {
	Items []bulkAdoptItemResponse    `json:"items"`
	Batch chapterplan.CandidateBatch `json:"batch"`
}

func bulkAdoptResponseFrom(value chapterplan.BulkAdoptResult) bulkAdoptResponse {
	items := make([]bulkAdoptItemResponse, 0, len(value.Items))
	for _, item := range value.Items {
		var plan *chapterPlanResponse
		if item.ChapterPlan != nil {
			response := chapterPlanResponseFrom(*item.ChapterPlan)
			plan = &response
		}
		items = append(items, bulkAdoptItemResponse{
			CandidateID: item.CandidateID,
			Outcome:     item.Outcome,
			Candidate:   item.Candidate,
			ChapterPlan: plan,
			Revision:    item.Revision,
			Error:       item.Error,
		})
	}
	return bulkAdoptResponse{Items: items, Batch: value.Batch}
}

func chapterPlanSourceResponse(source string) string {
	switch source {
	case "mock_generated", "candidate_adopted":
		return source
	case "manual", "legacy_manual":
		return "legacy_manual"
	default:
		return "legacy_manual"
	}
}
func nonNilUUIDs(values []uuid.UUID) []uuid.UUID {
	if values == nil {
		return []uuid.UUID{}
	}
	return values
}
func formatChapterPlanTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func chapterPlanningDetails(retryAction, safeReason string) map[string]any {
	return map[string]any{
		"retryAction": retryAction,
		"safeReason":  safeReason,
	}
}

func chapterPlanServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, chapterplan.ErrProjectNotFound):
		writeError(w, r, 404, "project_not_found", "project not found", chapterPlanningDetails("check_project_id", "The specified project was not found"))
	case errors.Is(err, chapterplan.ErrChapterPlanNotFound), errors.Is(err, chapterplan.ErrNotFound):
		writeError(w, r, 404, "chapter_plan_not_found", "chapter plan not found", chapterPlanningDetails("refresh_list", "The specified chapter plan was not found"))
	case errors.Is(err, chapterplan.ErrBatchNotFound):
		writeError(w, r, 404, "candidate_batch_not_found", "candidate batch not found", chapterPlanningDetails("refresh_list", "The specified candidate batch was not found"))
	case errors.Is(err, chapterplan.ErrCandidateNotFound):
		writeError(w, r, 404, "candidate_not_found", "candidate not found", chapterPlanningDetails("refresh_list", "The specified candidate was not found"))
	case errors.Is(err, chapterplan.ErrRevisionNotFound):
		writeError(w, r, 404, "revision_not_found", "revision not found", chapterPlanningDetails("refresh_list", "The specified revision was not found"))
	case errors.Is(err, chapterplan.ErrStorylineReferenceInvalid):
		writeError(w, r, 404, "storyline_not_found", "storyline not found", chapterPlanningDetails("check_references", "One or more referenced storylines do not exist"))
	case errors.Is(err, chapterplan.ErrMaterialReferenceInvalid):
		writeError(w, r, 404, "material_not_found", "material not found", chapterPlanningDetails("check_references", "One or more referenced materials do not exist"))
	case errors.Is(err, chapterplan.ErrForeshadowingReferenceInvalid):
		writeError(w, r, 404, "foreshadowing_not_found", "foreshadowing not found", chapterPlanningDetails("check_references", "One or more referenced foreshadowings do not exist"))
	case errors.Is(err, chapterplan.ErrChapterNoConflict):
		writeError(w, r, 409, "chapter_no_conflict", "chapter number conflict", chapterPlanningDetails("change_chapter_no", "Another chapter plan already uses this chapter number"))
	case errors.Is(err, chapterplan.ErrInvalidCandidateState):
		writeError(w, r, 409, "invalid_candidate_state", "candidate state is invalid for mutation", chapterPlanningDetails("re-fetch_summary", "Candidate status does not allow this operation"))
	case errors.Is(err, chapterplan.ErrStaleCandidate):
		writeError(w, r, 409, "stale_candidate", "candidate baseline is stale", chapterPlanningDetails("recompare_and_review", "Chapter plan baseline has been updated since candidate generation"))
	case errors.Is(err, chapterplan.ErrBatchAlreadyFinalized):
		writeError(w, r, 409, "batch_already_finalized", "batch is already finalized", chapterPlanningDetails("re-fetch_summary", "Batch has already been adopted or abandoned"))
	case errors.Is(err, chapterplan.ErrIdempotencyKeyReused), errors.Is(err, chapterplan.ErrIdempotencyConflict):
		writeError(w, r, 409, "idempotency_key_reused_with_different_payload", "idempotency key reused with different payload", chapterPlanningDetails("use_new_idempotency_key", "The idempotency key was previously used with a different payload"))
	case errors.Is(err, chapterplan.ErrRevisionSequenceConflict):
		writeError(w, r, 409, "revision_sequence_conflict", "revision sequence conflict", chapterPlanningDetails("refresh_and_retry", "Revision sequence mismatch"))
	case errors.Is(err, chapterplan.ErrOutputValidationFailed):
		writeError(w, r, 422, "output_validation_failed", "runtime output validation failed", chapterPlanningDetails("retry_run", "The runtime output does not match the frozen generation context."))
	case errors.Is(err, chapterplan.ErrIngestionTransaction):
		writeError(w, r, 500, "result_consumption_failed", "result consumption failed", chapterPlanningDetails("retry_run", "The generated result could not be stored safely."))
	case errors.Is(err, chapterplan.ErrInvalidState), errors.Is(err, chapterplan.ErrVersionConflict):
		writeError(w, r, 409, "version_conflict", "chapter plan version conflict", chapterPlanningDetails("refresh_and_retry", "Target resource version changed since last fetch"))
	case errors.Is(err, chapterplan.ErrValidation), errors.Is(err, chapterplan.ErrProjectMismatch), errors.Is(err, chapterplan.ErrInvalidReference):
		writeError(w, r, 400, "validation_error", "invalid chapter plan request", chapterPlanningDetails("fix_payload", "Request payload validation failed"))
	default:
		writeError(w, r, 500, "internal_error", "internal server error", chapterPlanningDetails("retry_later", "An internal error occurred"))
	}
}

func listCandidateBatchesHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		var f chapterplan.BatchFilter
		status := r.URL.Query().Get("status")
		if status != "" {
			f.Status = &status
		}
		genMode := r.URL.Query().Get("generationMode")
		if genMode == "" {
			genMode = r.URL.Query().Get("generation_mode")
		}
		if genMode != "" {
			f.GenerationMode = &genMode
		}
		runIDStr := r.URL.Query().Get("sourceWorkflowRunId")
		if runIDStr == "" {
			runIDStr = r.URL.Query().Get("source_workflow_run_id")
		}
		if runIDStr != "" {
			runID, err := uuid.Parse(runIDStr)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "invalid_uuid", "sourceWorkflowRunId must be a UUID", map[string]any{})
				return
			}
			f.SourceWorkflowRunID = &runID
		}
		if fromStr := r.URL.Query().Get("createdAtFrom"); fromStr != "" {
			if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
				f.CreatedAtFrom = &t
			}
		}
		if toStr := r.URL.Query().Get("createdAtTo"); toStr != "" {
			if t, err := time.Parse(time.RFC3339, toStr); err == nil {
				f.CreatedAtTo = &t
			}
		}
		limit, offset, ok := listPagination(r)
		if !ok {
			writeError(w, r, 400, "validation_error", "invalid pagination", map[string]any{})
			return
		}
		f.Limit = limit
		f.Offset = offset

		res, err := service.ListCandidateBatches(r.Context(), id, f)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, res)
	}
}

func getCandidateBatchHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		batchID, err := uuid.Parse(r.PathValue("batchId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "batchId must be a UUID", map[string]any{})
			return
		}
		res, err := service.GetCandidateBatchByID(r.Context(), batchID)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, res)
	}
}

func listCandidatesHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		batchID, err := uuid.Parse(r.PathValue("batchId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "batchId must be a UUID", map[string]any{})
			return
		}
		var f chapterplan.CandidateFilter
		status := r.URL.Query().Get("status")
		if status != "" {
			f.Status = &status
		}
		diffType := r.URL.Query().Get("diffType")
		if diffType == "" {
			diffType = r.URL.Query().Get("diff_type")
		}
		if diffType != "" {
			f.DiffType = &diffType
		}
		stIDStr := r.URL.Query().Get("storylineId")
		if stIDStr == "" {
			stIDStr = r.URL.Query().Get("storyline_id")
		}
		if stIDStr != "" {
			stID, err := uuid.Parse(stIDStr)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "invalid_uuid", "storylineId must be a UUID", map[string]any{})
				return
			}
			f.StorylineID = &stID
		}
		if q := r.URL.Query().Get("q"); q != "" {
			f.Q = &q
		}
		limit, offset, ok := listPagination(r)
		if !ok {
			writeError(w, r, 400, "validation_error", "invalid pagination", map[string]any{})
			return
		}
		f.Limit = limit
		f.Offset = offset

		res, err := service.ListCandidates(r.Context(), batchID, f)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, res)
	}
}

func getCandidateHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		candidateID, err := uuid.Parse(r.PathValue("candidateId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "candidateId must be a UUID", map[string]any{})
			return
		}
		res, err := service.GetCandidateByID(r.Context(), candidateID)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, res)
	}
}

func listRevisionsHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chapterPlanID, err := uuid.Parse(r.PathValue("chapterPlanId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "chapterPlanId must be a UUID", map[string]any{})
			return
		}
		limit, offset, ok := listPagination(r)
		if !ok {
			writeError(w, r, 400, "validation_error", "invalid pagination", map[string]any{})
			return
		}
		res, err := service.ListRevisions(r.Context(), chapterPlanID, limit, offset)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, res)
	}
}

func getChapterPlanningSummaryHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		res, err := service.GetChapterPlanningSummary(r.Context(), id)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, res)
	}
}

type updateCandidateRequest struct {
	ExpectedCandidateVersion int             `json:"expectedCandidateVersion"`
	CurrentSnapshot          json.RawMessage `json:"currentSnapshot"`
}

type recompareCandidateRequest struct {
	ExpectedCandidateVersion int `json:"expectedCandidateVersion"`
}

func updateCandidateHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		candidateID, err := uuid.Parse(r.PathValue("candidateId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "candidateId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Idempotency-Key header is required", map[string]any{})
			return
		}
		var req updateCandidateRequest
		if err := decodeBody(r, &req); err != nil || req.ExpectedCandidateVersion <= 0 || len(req.CurrentSnapshot) == 0 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "expectedCandidateVersion and currentSnapshot are required", map[string]any{})
			return
		}
		res, err := service.UpdateCandidate(r.Context(), chapterplan.UpdateCandidateCommand{
			CandidateID:              candidateID,
			ExpectedCandidateVersion: req.ExpectedCandidateVersion,
			CurrentSnapshot:          req.CurrentSnapshot,
			IdempotencyKey:           key,
			ActorID:                  requestActorID(r),
		})
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, res)
	}
}

func compareCandidateHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		candidateID, err := uuid.Parse(r.PathValue("candidateId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "candidateId must be a UUID", map[string]any{})
			return
		}
		res, err := service.CompareCandidate(r.Context(), candidateID)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, candidateComparisonResponseFrom(res))
	}
}

func recompareCandidateHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		candidateID, err := uuid.Parse(r.PathValue("candidateId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "candidateId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Idempotency-Key header is required", map[string]any{})
			return
		}
		var req recompareCandidateRequest
		if err := decodeBody(r, &req); err != nil || req.ExpectedCandidateVersion <= 0 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "expectedCandidateVersion is required", map[string]any{})
			return
		}
		res, err := service.RecompareCandidate(r.Context(), chapterplan.RecompareCandidateCommand{
			CandidateID:              candidateID,
			ExpectedCandidateVersion: req.ExpectedCandidateVersion,
			IdempotencyKey:           key,
			ActorID:                  requestActorID(r),
		})
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, candidateComparisonResponseFrom(res))
	}
}

type adoptCandidateRequest struct {
	ExpectedCandidateVersion   int  `json:"expectedCandidateVersion"`
	ExpectedChapterPlanVersion *int `json:"expectedChapterPlanVersion"`
}

type discardCandidateRequest struct {
	ExpectedCandidateVersion int     `json:"expectedCandidateVersion"`
	Reason                   *string `json:"reason"`
}

type bulkAdoptCandidateItemRequest struct {
	CandidateID                uuid.UUID `json:"candidateId"`
	ExpectedCandidateVersion   int       `json:"expectedCandidateVersion"`
	ExpectedChapterPlanVersion *int      `json:"expectedChapterPlanVersion"`
}

type bulkAdoptRequest struct {
	ExpectedBatchVersion int                             `json:"expectedBatchVersion"`
	Candidates           []bulkAdoptCandidateItemRequest `json:"candidates"`
}

type abandonBatchRequest struct {
	ExpectedBatchVersion             int     `json:"expectedBatchVersion"`
	Reason                           *string `json:"reason"`
	AcknowledgeAdoptedChaptersRemain bool    `json:"acknowledgeAdoptedChaptersRemain"`
}

func adoptCandidateHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		candidateID, err := uuid.Parse(r.PathValue("candidateId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "candidateId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Idempotency-Key header is required", map[string]any{})
			return
		}
		var req adoptCandidateRequest
		if err := decodeBody(r, &req); err != nil || req.ExpectedCandidateVersion <= 0 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "expectedCandidateVersion is required", map[string]any{})
			return
		}
		res, err := service.AdoptCandidate(r.Context(), chapterplan.AdoptCandidateCommand{
			CandidateID:                candidateID,
			ExpectedCandidateVersion:   req.ExpectedCandidateVersion,
			ExpectedChapterPlanVersion: req.ExpectedChapterPlanVersion,
			IdempotencyKey:             key,
			ActorID:                    requestActorID(r),
		})
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, adoptCandidateResponseFrom(res))
	}
}

func bulkAdoptCandidatesHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		batchID, err := uuid.Parse(r.PathValue("batchId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "batchId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Idempotency-Key header is required", map[string]any{})
			return
		}
		var req bulkAdoptRequest
		if err := decodeBody(r, &req); err != nil || req.ExpectedBatchVersion <= 0 || len(req.Candidates) == 0 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "expectedBatchVersion and candidates are required", map[string]any{})
			return
		}
		items := make([]chapterplan.BulkAdoptCandidateItemCommand, len(req.Candidates))
		for i, c := range req.Candidates {
			items[i] = chapterplan.BulkAdoptCandidateItemCommand{
				CandidateID:                c.CandidateID,
				ExpectedCandidateVersion:   c.ExpectedCandidateVersion,
				ExpectedChapterPlanVersion: c.ExpectedChapterPlanVersion,
			}
		}
		res, err := service.BulkAdoptCandidates(r.Context(), chapterplan.BulkAdoptCommand{
			BatchID:              batchID,
			ExpectedBatchVersion: req.ExpectedBatchVersion,
			Candidates:           items,
			IdempotencyKey:       key,
			ActorID:              requestActorID(r),
		})
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, bulkAdoptResponseFrom(chapterplan.SanitizeBulkAdoptResult(res)))
	}
}

func discardCandidateHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		candidateID, err := uuid.Parse(r.PathValue("candidateId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "candidateId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Idempotency-Key header is required", map[string]any{})
			return
		}
		var req discardCandidateRequest
		if err := decodeBody(r, &req); err != nil || req.ExpectedCandidateVersion <= 0 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "expectedCandidateVersion is required", map[string]any{})
			return
		}
		res, err := service.DiscardCandidate(r.Context(), chapterplan.DiscardCandidateCommand{
			CandidateID:              candidateID,
			ExpectedCandidateVersion: req.ExpectedCandidateVersion,
			Reason:                   req.Reason,
			IdempotencyKey:           key,
			ActorID:                  requestActorID(r),
		})
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, res)
	}
}

func abandonBatchHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		batchID, err := uuid.Parse(r.PathValue("batchId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "batchId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeError(w, r, http.StatusBadRequest, "validation_error", "Idempotency-Key header is required", map[string]any{})
			return
		}
		var req abandonBatchRequest
		if err := decodeBody(r, &req); err != nil || req.ExpectedBatchVersion <= 0 || !req.AcknowledgeAdoptedChaptersRemain {
			writeError(w, r, http.StatusBadRequest, "validation_error", "expectedBatchVersion and acknowledgeAdoptedChaptersRemain=true are required", map[string]any{})
			return
		}
		res, err := service.AbandonBatch(r.Context(), chapterplan.AbandonBatchCommand{
			BatchID:                          batchID,
			ExpectedBatchVersion:             req.ExpectedBatchVersion,
			Reason:                           req.Reason,
			AcknowledgeAdoptedChaptersRemain: req.AcknowledgeAdoptedChaptersRemain,
			IdempotencyKey:                   key,
			ActorID:                          requestActorID(r),
		})
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, res)
	}
}

type updateChapterPlanRequest struct {
	ExpectedVersion       json.RawMessage `json:"expected_version"`
	ChapterNo             json.RawMessage `json:"chapter_no"`
	Title                 json.RawMessage `json:"title"`
	Summary               json.RawMessage `json:"summary"`
	StorylineRefsJSON     json.RawMessage `json:"storyline_refs_json"`
	MaterialRefsJSON      json.RawMessage `json:"material_refs_json"`
	ForeshadowingRefsJSON json.RawMessage `json:"foreshadowing_refs_json"`
	ChapterGoal           json.RawMessage `json:"chapter_goal"`
	CreationNotes         json.RawMessage `json:"creation_notes"`
}
type confirmChapterPlansRequest struct {
	Selections json.RawMessage `json:"selections"`
}
type confirmChapterPlanSelectionRequest struct {
	ChapterPlanID   json.RawMessage `json:"chapter_plan_id"`
	ExpectedVersion json.RawMessage `json:"expected_version"`
}

func updateChapterPlanHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("chapterPlanId"))
		if err != nil {
			writeError(w, r, 400, "invalid_uuid", "chapterPlanId must be a UUID", map[string]any{})
			return
		}
		command, err := decodeUpdateChapterPlan(r)
		if err != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		value, err := service.Update(r.Context(), id, command)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		writeJSON(w, r, 200, chapterPlanResponseFrom(value))
	}
}
func deleteChapterPlanHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("chapterPlanId"))
		if err != nil {
			writeError(w, r, 400, "invalid_uuid", "chapterPlanId must be a UUID", map[string]any{})
			return
		}
		expected, err := positiveQueryInt(r, "expected_version")
		if err != nil {
			writeError(w, r, 400, "validation_error", "invalid expected_version", map[string]any{})
			return
		}
		if err = service.Delete(r.Context(), id, expected); err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
func confirmChapterPlansHandler(service chapterPlanApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		selections, err := decodeConfirmChapterPlans(r)
		if err != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		items, err := service.Confirm(r.Context(), id, selections)
		if err != nil {
			chapterPlanServiceError(w, r, err)
			return
		}
		response := make([]chapterPlanResponse, 0, len(items))
		for _, item := range items {
			response = append(response, chapterPlanResponseFrom(item))
		}
		writeJSON(w, r, 200, map[string]any{"items": response, "total": len(response), "limit": len(response), "offset": 0})
	}
}
func positiveQueryInt(r *http.Request, key string) (int, error) {
	v := r.URL.Query().Get(key)
	n, e := strconv.Atoi(v)
	if v == "" || e != nil || n < 1 {
		return 0, errors.New("invalid positive integer")
	}
	return n, nil
}
func decodeUpdateChapterPlan(r *http.Request) (chapterplan.UpdateCommand, error) {
	var body updateChapterPlanRequest
	if err := decodeBody(r, &body); err != nil || body.ExpectedVersion == nil {
		return chapterplan.UpdateCommand{}, errors.New("invalid request")
	}
	expected, err := rawPositiveInt(body.ExpectedVersion)
	if err != nil {
		return chapterplan.UpdateCommand{}, err
	}
	c := chapterplan.UpdateCommand{ExpectedVersion: expected}
	if c.ChapterNo, err = rawOptionalInt(body.ChapterNo); err != nil {
		return c, err
	}
	if c.Title, err = rawOptionalString(body.Title); err != nil {
		return c, err
	}
	if c.Summary, err = rawOptionalString(body.Summary); err != nil {
		return c, err
	}
	if c.Title != nil && (len(*c.Title) == 0 || len(*c.Title) > 120) || c.Summary != nil && len(*c.Summary) > 5000 {
		return c, errors.New("invalid text length")
	}
	if c.Storylines.Value, c.Storylines.Set, err = rawStorylines(body.StorylineRefsJSON); err != nil {
		return c, err
	}
	if c.Materials.Value, c.Materials.Set, err = rawUUIDs(body.MaterialRefsJSON); err != nil {
		return c, err
	}
	if c.Foreshadowings.Value, c.Foreshadowings.Set, err = rawUUIDs(body.ForeshadowingRefsJSON); err != nil {
		return c, err
	}
	if c.Goal.Value, c.Goal.Set, err = rawNullableString(body.ChapterGoal); err != nil {
		return c, err
	}
	if c.Notes.Value, c.Notes.Set, err = rawNullableString(body.CreationNotes); err != nil {
		return c, err
	}
	if c.Goal.Value != nil && len(*c.Goal.Value) > 2000 || c.Notes.Value != nil && len(*c.Notes.Value) > 2000 {
		return c, errors.New("invalid text length")
	}
	if c.ChapterNo == nil && c.Title == nil && c.Summary == nil && !c.Storylines.Set && !c.Materials.Set && !c.Foreshadowings.Set && !c.Goal.Set && !c.Notes.Set {
		return c, errors.New("no updates")
	}
	return c, nil
}
func decodeRawStrict(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func rawPositiveInt(raw json.RawMessage) (int, error) {
	var n int
	if len(raw) == 0 || json.Unmarshal(raw, &n) != nil || n < 1 {
		return 0, errors.New("invalid integer")
	}
	return n, nil
}
func rawOptionalInt(raw json.RawMessage) (*int, error) {
	if raw == nil {
		return nil, nil
	}
	n, e := rawPositiveInt(raw)
	if e != nil {
		return nil, e
	}
	return &n, nil
}
func rawOptionalString(raw json.RawMessage) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	var v string
	if json.Unmarshal(raw, &v) != nil {
		return nil, errors.New("invalid string")
	}
	return &v, nil
}
func rawNullableString(raw json.RawMessage) (*string, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, true, nil
	}
	v, e := rawOptionalString(raw)
	return v, true, e
}
func rawStorylines(raw json.RawMessage) ([]chapterplan.StorylineRef, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	var v []struct {
		ID       string `json:"storyline_id"`
		Relation string `json:"relation"`
	}
	if decodeRawStrict(raw, &v) != nil || len(v) < 1 {
		return nil, false, errors.New("invalid storylines")
	}
	out := make([]chapterplan.StorylineRef, 0, len(v))
	for _, x := range v {
		id, e := uuid.Parse(x.ID)
		if e != nil || (x.Relation != "primary" && x.Relation != "secondary") {
			return nil, false, errors.New("invalid storyline")
		}
		out = append(out, chapterplan.StorylineRef{ID: id, Relation: x.Relation})
	}
	return out, true, nil
}
func rawUUIDs(raw json.RawMessage) ([]uuid.UUID, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	var values []string
	if json.Unmarshal(raw, &values) != nil {
		return nil, false, errors.New("invalid UUID list")
	}
	out := make([]uuid.UUID, 0, len(values))
	seen := map[uuid.UUID]struct{}{}
	for _, value := range values {
		id, e := uuid.Parse(value)
		if e != nil {
			return nil, false, e
		}
		if _, ok := seen[id]; ok {
			return nil, false, errors.New("duplicate UUID")
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, true, nil
}
func decodeConfirmChapterPlans(r *http.Request) ([]chapterplan.Selection, error) {
	var body confirmChapterPlansRequest
	if err := decodeBody(r, &body); err != nil || body.Selections == nil {
		return nil, errors.New("invalid request")
	}
	var raw []confirmChapterPlanSelectionRequest
	if decodeRawStrict(body.Selections, &raw) != nil || len(raw) == 0 {
		return nil, errors.New("invalid selections")
	}
	out := make([]chapterplan.Selection, 0, len(raw))
	seen := map[uuid.UUID]struct{}{}
	for _, value := range raw {
		if value.ChapterPlanID == nil || value.ExpectedVersion == nil {
			return nil, errors.New("missing selection")
		}
		var idText string
		if json.Unmarshal(value.ChapterPlanID, &idText) != nil {
			return nil, errors.New("invalid chapter plan ID")
		}
		id, e := uuid.Parse(idText)
		if e != nil {
			return nil, e
		}
		expected, e := rawPositiveInt(value.ExpectedVersion)
		if e != nil {
			return nil, e
		}
		if _, ok := seen[id]; ok {
			return nil, errors.New("duplicate selection")
		}
		seen[id] = struct{}{}
		out = append(out, chapterplan.Selection{ID: id, ExpectedVersion: expected})
	}
	return out, nil
}

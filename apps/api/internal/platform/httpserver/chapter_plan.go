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
}

type chapterPlanStorylineRefResponse struct {
	StorylineID uuid.UUID `json:"storyline_id"`
	Relation    string    `json:"relation"`
}
type chapterPlanResponse struct {
	ID                    uuid.UUID                         `json:"id"`
	ProjectID             uuid.UUID                         `json:"project_id"`
	ChapterNo             int                               `json:"chapter_no"`
	Title                 string                            `json:"title"`
	Summary               string                            `json:"summary"`
	Status                string                            `json:"status"`
	Source                string                            `json:"source"`
	StorylineRefsJSON     []chapterPlanStorylineRefResponse `json:"storyline_refs_json"`
	MaterialRefsJSON      []uuid.UUID                       `json:"material_refs_json"`
	ForeshadowingRefsJSON []uuid.UUID                       `json:"foreshadowing_refs_json"`
	ChapterGoal           *string                           `json:"chapter_goal"`
	CreationNotes         *string                           `json:"creation_notes"`
	ConfirmedAt           *string                           `json:"confirmed_at"`
	Version               int                               `json:"version"`
	CreatedAt             string                            `json:"created_at"`
	UpdatedAt             string                            `json:"updated_at"`
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

func registerChapterPlanRoutes(mux *http.ServeMux, service chapterPlanApplication) {
	mux.HandleFunc("GET /api/v1/projects/{projectId}/chapter-plans", listChapterPlansHandler(service))
	mux.HandleFunc("POST /api/v1/projects/{projectId}/chapter-plans/mock-generate", generateMockChapterPlansHandler(service))
	mux.HandleFunc("GET /api/v1/chapter-plans/{chapterPlanId}", getChapterPlanHandler(service))
	mux.HandleFunc("PATCH /api/v1/chapter-plans/{chapterPlanId}", updateChapterPlanHandler(service))
	mux.HandleFunc("DELETE /api/v1/chapter-plans/{chapterPlanId}", deleteChapterPlanHandler(service))
	mux.HandleFunc("POST /api/v1/projects/{projectId}/chapter-plans/confirm", confirmChapterPlansHandler(service))
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
	return chapterplan.MockGenerateCommand{TargetStorylineID: target, StartChapterNo: *body.StartChapterNo, EndChapterNo: *body.EndChapterNo, ChapterCount: *body.ChapterCount, IncludeMainStoryline: *body.IncludeMainStoryline, IncludeChildStorylines: *body.IncludeChildStorylines, IncludeProjectMaterials: *body.IncludeProjectMaterials, IncludeUnpaidForeshadowings: *body.IncludeUnpaidForeshadowings, IncludePriorChapterSummaries: *body.IncludePriorChapterSummaries, SummaryLength: *body.SummaryLength, ChapterPace: *body.ChapterPace, GenerationNotes: notes, ActorID: "system"}, nil
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
	return chapterPlanResponse{ID: value.ID, ProjectID: value.ProjectID, ChapterNo: value.ChapterNo, Title: value.Title, Summary: value.Summary, Status: value.Status, Source: value.Source, StorylineRefsJSON: storylines, MaterialRefsJSON: nonNilUUIDs(value.Materials), ForeshadowingRefsJSON: nonNilUUIDs(value.Foreshadowings), ChapterGoal: value.Goal, CreationNotes: value.Notes, ConfirmedAt: confirmedAt, Version: value.Version, CreatedAt: formatChapterPlanTime(value.CreatedAt), UpdatedAt: formatChapterPlanTime(value.UpdatedAt)}
}
func nonNilUUIDs(values []uuid.UUID) []uuid.UUID {
	if values == nil {
		return []uuid.UUID{}
	}
	return values
}
func formatChapterPlanTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func chapterPlanServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, chapterplan.ErrProjectNotFound):
		writeError(w, r, 404, "project_not_found", "project not found", map[string]any{})
	case errors.Is(err, chapterplan.ErrChapterPlanNotFound), errors.Is(err, chapterplan.ErrNotFound):
		writeError(w, r, 404, "chapter_plan_not_found", "chapter plan not found", map[string]any{})
	case errors.Is(err, chapterplan.ErrStorylineReferenceInvalid):
		writeError(w, r, 404, "storyline_not_found", "storyline not found", map[string]any{})
	case errors.Is(err, chapterplan.ErrMaterialReferenceInvalid):
		writeError(w, r, 404, "material_not_found", "material not found", map[string]any{})
	case errors.Is(err, chapterplan.ErrForeshadowingReferenceInvalid):
		writeError(w, r, 404, "foreshadowing_not_found", "foreshadowing not found", map[string]any{})
	case errors.Is(err, chapterplan.ErrChapterNoConflict):
		writeError(w, r, 409, "chapter_no_conflict", "chapter number conflict", map[string]any{})
	case errors.Is(err, chapterplan.ErrInvalidState), errors.Is(err, chapterplan.ErrVersionConflict):
		writeError(w, r, 409, "version_conflict", "chapter plan version conflict", map[string]any{})
	case errors.Is(err, chapterplan.ErrValidation), errors.Is(err, chapterplan.ErrProjectMismatch), errors.Is(err, chapterplan.ErrInvalidReference):
		writeError(w, r, 400, "validation_error", "invalid chapter plan request", map[string]any{})
	default:
		writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
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

package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/foreshadowing"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type foreshadowingApplication interface {
	List(context.Context, uuid.UUID) (foreshadowing.ListResult, error)
	Create(context.Context, uuid.UUID, foreshadowing.CreateCommand) (foreshadowing.Foreshadowing, error)
	Update(context.Context, uuid.UUID, foreshadowing.UpdateCommand) (foreshadowing.Foreshadowing, error)
}

type foreshadowingResponse struct {
	ID                   uuid.UUID  `json:"id"`
	ProjectID            uuid.UUID  `json:"project_id"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	Priority             string     `json:"priority"`
	PlantedPlotLineID    *uuid.UUID `json:"planted_plot_line_id"`
	PayoffPlotLineID     *uuid.UUID `json:"payoff_plot_line_id"`
	PlannedPlantChapter  *int       `json:"planned_plant_chapter"`
	PlannedPayoffChapter *int       `json:"planned_payoff_chapter"`
	Status               string     `json:"status"`
	Version              int        `json:"version"`
	CreatedAt            string     `json:"created_at"`
	UpdatedAt            string     `json:"updated_at"`
}
type foreshadowingCreateRequest struct {
	Title         *string         `json:"title"`
	Description   *string         `json:"description"`
	Priority      *string         `json:"priority"`
	Planted       json.RawMessage `json:"planted_plot_line_id"`
	Payoff        json.RawMessage `json:"payoff_plot_line_id"`
	PlantChapter  json.RawMessage `json:"planned_plant_chapter"`
	PayoffChapter json.RawMessage `json:"planned_payoff_chapter"`
	Status        *string         `json:"status"`
}
type foreshadowingUpdateRequest struct {
	ExpectedVersion *int            `json:"expected_version"`
	Title           *string         `json:"title"`
	Description     *string         `json:"description"`
	Priority        *string         `json:"priority"`
	Planted         json.RawMessage `json:"planted_plot_line_id"`
	Payoff          json.RawMessage `json:"payoff_plot_line_id"`
	PlantChapter    json.RawMessage `json:"planned_plant_chapter"`
	PayoffChapter   json.RawMessage `json:"planned_payoff_chapter"`
	Status          *string         `json:"status"`
}

func registerForeshadowingRoutes(mux *http.ServeMux, service foreshadowingApplication) {
	mux.HandleFunc("GET /api/v1/projects/{projectId}/foreshadowings", listForeshadowingsHandler(service))
	mux.HandleFunc("POST /api/v1/projects/{projectId}/foreshadowings", createForeshadowingHandler(service))
	mux.HandleFunc("PATCH /api/v1/foreshadowings/{foreshadowingId}", updateForeshadowingHandler(service))
}
func listForeshadowingsHandler(service foreshadowingApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		limit, offset, ok := listPagination(r)
		if !ok {
			writeError(w, r, 400, "validation_error", "invalid pagination", map[string]any{})
			return
		}
		result, err := service.List(r.Context(), id)
		if err != nil {
			foreshadowingServiceError(w, r, err)
			return
		}
		total := len(result.Items)
		start := offset
		if start > total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}
		items := make([]foreshadowingResponse, 0, end-start)
		for _, item := range result.Items[start:end] {
			items = append(items, foreshadowingResponseFrom(item))
		}
		writeJSON(w, r, 200, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
	}
}
func listPagination(r *http.Request) (int, int, bool) {
	limit, offset := 20, 0
	var err error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, false
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return 0, 0, false
		}
	}
	return limit, offset, true
}
func createForeshadowingHandler(service foreshadowingApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, ok := projectID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		command, err := decodeCreateForeshadowing(r)
		if err != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		value, err := service.Create(r.Context(), projectID, command)
		if err != nil {
			foreshadowingServiceError(w, r, err)
			return
		}
		writeJSON(w, r, 201, foreshadowingResponseFrom(value))
	}
}
func updateForeshadowingHandler(service foreshadowingApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := foreshadowingID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "foreshadowingId must be a UUID", map[string]any{})
			return
		}
		command, err := decodeUpdateForeshadowing(r)
		if err != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		value, err := service.Update(r.Context(), id, command)
		if err != nil {
			foreshadowingServiceError(w, r, err)
			return
		}
		writeJSON(w, r, 200, foreshadowingResponseFrom(value))
	}
}
func decodeCreateForeshadowing(r *http.Request) (foreshadowing.CreateCommand, error) {
	var b foreshadowingCreateRequest
	if err := decodeBody(r, &b); err != nil || b.Title == nil || b.Description == nil || b.Priority == nil || b.Status == nil {
		return foreshadowing.CreateCommand{}, errors.New("invalid")
	}
	planted, ok, err := nullableUUID(b.Planted)
	if err != nil || !ok {
		return foreshadowing.CreateCommand{}, errors.New("invalid")
	}
	payoff, ok, err := nullableUUID(b.Payoff)
	if err != nil || !ok {
		return foreshadowing.CreateCommand{}, errors.New("invalid")
	}
	plant, ok, err := decodeNullableInt(b.PlantChapter)
	if err != nil || !ok {
		return foreshadowing.CreateCommand{}, errors.New("invalid")
	}
	pay, ok, err := decodeNullableInt(b.PayoffChapter)
	if err != nil || !ok {
		return foreshadowing.CreateCommand{}, errors.New("invalid")
	}
	return foreshadowing.CreateCommand{Title: *b.Title, Description: *b.Description, Priority: *b.Priority, Status: *b.Status, PlantedPlotLineID: planted, PayoffPlotLineID: payoff, PlannedPlantChapter: plant, PlannedPayoffChapter: pay, ActorID: "system"}, nil
}
func decodeUpdateForeshadowing(r *http.Request) (foreshadowing.UpdateCommand, error) {
	var b foreshadowingUpdateRequest
	if err := decodeBody(r, &b); err != nil || b.ExpectedVersion == nil || *b.ExpectedVersion < 1 {
		return foreshadowing.UpdateCommand{}, errors.New("invalid")
	}
	planted, plantSet, err := nullableUUID(b.Planted)
	if err != nil {
		return foreshadowing.UpdateCommand{}, err
	}
	payoff, payoffSet, err := nullableUUID(b.Payoff)
	if err != nil {
		return foreshadowing.UpdateCommand{}, err
	}
	plant, plantChapterSet, err := decodeNullableInt(b.PlantChapter)
	if err != nil {
		return foreshadowing.UpdateCommand{}, err
	}
	pay, payChapterSet, err := decodeNullableInt(b.PayoffChapter)
	if err != nil {
		return foreshadowing.UpdateCommand{}, err
	}
	if b.Title == nil && b.Description == nil && b.Priority == nil && b.Status == nil && !plantSet && !payoffSet && !plantChapterSet && !payChapterSet {
		return foreshadowing.UpdateCommand{}, errors.New("empty")
	}
	return foreshadowing.UpdateCommand{ExpectedVersion: *b.ExpectedVersion, Title: b.Title, Description: b.Description, Priority: b.Priority, Status: b.Status, PlantedPlotLineID: foreshadowing.OptionalUUID{Set: plantSet, Value: planted}, PayoffPlotLineID: foreshadowing.OptionalUUID{Set: payoffSet, Value: payoff}, PlannedPlantChapter: foreshadowing.OptionalInt{Set: plantChapterSet, Value: plant}, PlannedPayoffChapter: foreshadowing.OptionalInt{Set: payChapterSet, Value: pay}, ActorID: "system"}, nil
}
func nullableUUID(raw json.RawMessage) (*uuid.UUID, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, true, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, false, err
	}
	id, err := uuid.Parse(text)
	if err != nil {
		return nil, false, err
	}
	return &id, true, nil
}
func foreshadowingID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("foreshadowingId"))
	return id, err == nil
}
func foreshadowingResponseFrom(v foreshadowing.Foreshadowing) foreshadowingResponse {
	return foreshadowingResponse{ID: v.ID, ProjectID: v.ProjectID, Title: v.Title, Description: v.Description, Priority: v.Priority, PlantedPlotLineID: v.PlantedPlotLineID, PayoffPlotLineID: v.PayoffPlotLineID, PlannedPlantChapter: v.PlannedPlantChapter, PlannedPayoffChapter: v.PlannedPayoffChapter, Status: v.Status, Version: v.Version, CreatedAt: v.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: v.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}
func foreshadowingServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, project.ErrNotFound):
		writeError(w, r, 404, "project_not_found", "project not found", map[string]any{})
	case errors.Is(err, foreshadowing.ErrNotFound):
		writeError(w, r, 404, "foreshadowing_not_found", "foreshadowing not found", map[string]any{})
	case errors.Is(err, foreshadowing.ErrStorylineNotFound), errors.Is(err, foreshadowing.ErrInvalidReference):
		writeError(w, r, 404, "storyline_not_found", "storyline not found", map[string]any{})
	case errors.Is(err, foreshadowing.ErrVersionConflict):
		writeError(w, r, 409, "version_conflict", "foreshadowing version conflict", map[string]any{})
	case errors.Is(err, foreshadowing.ErrValidation), errors.Is(err, foreshadowing.ErrInvalidPriority), errors.Is(err, foreshadowing.ErrInvalidStatus), errors.Is(err, foreshadowing.ErrInvalidTransition), errors.Is(err, foreshadowing.ErrChapterRange), errors.Is(err, foreshadowing.ErrProjectMismatch):
		writeError(w, r, 400, "validation_error", "invalid foreshadowing request", map[string]any{})
	default:
		writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
	}
}

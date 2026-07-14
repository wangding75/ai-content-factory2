package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
)

type storylineApplication interface {
	GetTree(context.Context, uuid.UUID) (storyline.GetTreeResult, error)
	CreateRoot(context.Context, uuid.UUID, storyline.CreateRootCommand) (storyline.PlotLine, error)
	CreateChildForParent(context.Context, uuid.UUID, storyline.CreateChildCommand) (storyline.PlotLine, error)
	Update(context.Context, uuid.UUID, storyline.UpdateCommand) (storyline.PlotLine, error)
}

type storylineResponse struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"project_id"`
	ParentID     *uuid.UUID `json:"parent_id"`
	Type         string     `json:"type"`
	Relation     string     `json:"relation"`
	Name         string     `json:"name"`
	Summary      string     `json:"summary"`
	StartChapter *int       `json:"start_chapter"`
	EndChapter   *int       `json:"end_chapter"`
	Status       string     `json:"status"`
	SortOrder    int        `json:"sort_order"`
	Version      int        `json:"version"`
	CreatedAt    string     `json:"created_at"`
	UpdatedAt    string     `json:"updated_at"`
}

type storylineTreeNodeResponse struct {
	storylineResponse
	Children []*storylineTreeNodeResponse `json:"children"`
}

type createStorylineRequest struct {
	Name         *string         `json:"name"`
	Summary      *string         `json:"summary"`
	StartChapter json.RawMessage `json:"start_chapter"`
	EndChapter   json.RawMessage `json:"end_chapter"`
	Status       *string         `json:"status"`
	SortOrder    *int            `json:"sort_order"`
}

type updateStorylineRequest struct {
	ExpectedVersion *int            `json:"expected_version"`
	Name            *string         `json:"name"`
	Summary         *string         `json:"summary"`
	StartChapter    json.RawMessage `json:"start_chapter"`
	EndChapter      json.RawMessage `json:"end_chapter"`
	Status          *string         `json:"status"`
	SortOrder       *int            `json:"sort_order"`
}

func registerStorylineRoutes(mux *http.ServeMux, service storylineApplication) {
	mux.HandleFunc("GET /api/v1/projects/{projectId}/storylines", getStorylineTreeHandler(service))
	mux.HandleFunc("POST /api/v1/projects/{projectId}/storylines", createRootStorylineHandler(service))
	mux.HandleFunc("POST /api/v1/storylines/{storylineId}/children", createChildStorylineHandler(service))
	mux.HandleFunc("PATCH /api/v1/storylines/{storylineId}", updateStorylineHandler(service))
}

func getStorylineTreeHandler(service storylineApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, ok := projectID(r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		result, err := service.GetTree(r.Context(), projectID)
		if err != nil {
			storylineServiceError(w, r, err)
			return
		}
		items := make([]*storylineTreeNodeResponse, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, storylineTreeResponse(item))
		}
		writeJSON(w, r, http.StatusOK, map[string]any{"items": items})
	}
}

func createRootStorylineHandler(service storylineApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, ok := projectID(r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		command, err := decodeCreateStoryline(r)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{})
			return
		}
		created, err := service.CreateRoot(r.Context(), projectID, command)
		if err != nil {
			storylineServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusCreated, storylineResponseFrom(created))
	}
}

func createChildStorylineHandler(service storylineApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parentID, ok := storylineID(r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "storylineId must be a UUID", map[string]any{})
			return
		}
		command, err := decodeCreateStoryline(r)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{})
			return
		}
		// The frozen child request has no project_id; the Application derives the project from the parent.
		created, err := service.CreateChildForParent(r.Context(), parentID, command)
		if err != nil {
			storylineServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusCreated, storylineResponseFrom(created))
	}
}

func updateStorylineHandler(service storylineApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := storylineID(r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "invalid_uuid", "storylineId must be a UUID", map[string]any{})
			return
		}
		command, err := decodeUpdateStoryline(r)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{})
			return
		}
		updated, err := service.Update(r.Context(), id, command)
		if err != nil {
			storylineServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, storylineResponseFrom(updated))
	}
}

func decodeCreateStoryline(r *http.Request) (storyline.CreateRootCommand, error) {
	var body createStorylineRequest
	if err := decodeBody(r, &body); err != nil || body.Name == nil || body.Summary == nil || body.Status == nil || body.SortOrder == nil {
		return storyline.CreateRootCommand{}, errors.New("invalid request")
	}
	start, provided, err := decodeNullableInt(body.StartChapter)
	if err != nil || !provided {
		return storyline.CreateRootCommand{}, errors.New("invalid start_chapter")
	}
	end, provided, err := decodeNullableInt(body.EndChapter)
	if err != nil || !provided {
		return storyline.CreateRootCommand{}, errors.New("invalid end_chapter")
	}
	return storyline.CreateRootCommand{Name: *body.Name, Summary: *body.Summary, StartChapter: start, EndChapter: end, Status: *body.Status, SortOrder: *body.SortOrder, ActorID: "system"}, nil
}

func decodeUpdateStoryline(r *http.Request) (storyline.UpdateCommand, error) {
	var body updateStorylineRequest
	if err := decodeBody(r, &body); err != nil || body.ExpectedVersion == nil {
		return storyline.UpdateCommand{}, errors.New("invalid request")
	}
	start, startSet, err := decodeNullableInt(body.StartChapter)
	if err != nil {
		return storyline.UpdateCommand{}, err
	}
	end, endSet, err := decodeNullableInt(body.EndChapter)
	if err != nil {
		return storyline.UpdateCommand{}, err
	}
	if body.Name == nil && body.Summary == nil && body.Status == nil && body.SortOrder == nil && !startSet && !endSet {
		return storyline.UpdateCommand{}, errors.New("missing update field")
	}
	return storyline.UpdateCommand{ExpectedVersion: *body.ExpectedVersion, Name: body.Name, Summary: body.Summary, Status: body.Status, StartChapter: storyline.OptionalInt{Set: startSet, Value: start}, EndChapter: storyline.OptionalInt{Set: endSet, Value: end}, SortOrder: body.SortOrder, ActorID: "system"}, nil
}

func decodeNullableInt(raw json.RawMessage) (*int, bool, error) {
	if raw == nil {
		return nil, false, nil
	}
	if string(raw) == "null" {
		return nil, true, nil
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false, err
	}
	return &value, true, nil
}

func storylineID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("storylineId"))
	return id, err == nil
}

func storylineResponseFrom(value storyline.PlotLine) storylineResponse {
	return storylineResponse{ID: value.ID, ProjectID: value.ProjectID, ParentID: value.ParentID, Type: value.Type, Relation: value.Relation, Name: value.Name, Summary: value.Summary, StartChapter: value.StartChapter, EndChapter: value.EndChapter, Status: value.Status, SortOrder: value.SortOrder, Version: value.Version, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: value.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}

func storylineTreeResponse(value *storyline.StorylineTreeNode) *storylineTreeNodeResponse {
	children := make([]*storylineTreeNodeResponse, 0, len(value.Children))
	for _, child := range value.Children {
		children = append(children, storylineTreeResponse(child))
	}
	return &storylineTreeNodeResponse{storylineResponse: storylineResponseFrom(value.PlotLine), Children: children}
}

func storylineServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, project.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "project_not_found", "project not found", map[string]any{})
	case errors.Is(err, storyline.ErrNotFound), errors.Is(err, storyline.ErrParentNotFound):
		writeError(w, r, http.StatusNotFound, "storyline_not_found", "storyline not found", map[string]any{})
	case errors.Is(err, storyline.ErrVersionConflict):
		writeError(w, r, http.StatusConflict, "version_conflict", "storyline version conflict", map[string]any{})
	case errors.Is(err, storyline.ErrValidation), errors.Is(err, storyline.ErrInvalidTypeOrRelation), errors.Is(err, storyline.ErrChapterRange), errors.Is(err, storyline.ErrChildOutOfRange), errors.Is(err, storyline.ErrDescendantOutOfRange), errors.Is(err, storyline.ErrProjectMismatch), errors.Is(err, storyline.ErrMissingParent), errors.Is(err, storyline.ErrCycle), errors.Is(err, storyline.ErrDuplicateStoryline):
		writeError(w, r, http.StatusBadRequest, "validation_error", "invalid storyline request", map[string]any{})
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error", map[string]any{})
	}
}

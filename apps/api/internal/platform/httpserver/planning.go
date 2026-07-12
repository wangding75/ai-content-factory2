package httpserver

import (
	"errors"
	"net/http"

	"github.com/local/ai-content-factory/apps/api/internal/planning"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

func getProjectPlanningHandler(service *planning.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "INVALID_PROJECT_ID", "projectId must be a UUID", map[string]any{})
			return
		}
		value, err := service.GetProjectPlanning(r.Context(), id)
		if err != nil {
			planningServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, value)
	}
}

func putProjectPlanningHandler(service *planning.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, http.StatusBadRequest, "INVALID_PROJECT_ID", "projectId must be a UUID", map[string]any{})
			return
		}
		var request planning.SaveRequest
		if err := decodeBody(r, &request); err != nil {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", map[string]any{})
			return
		}
		value, err := service.PutProjectPlanning(r.Context(), id, request, "system")
		if err != nil {
			planningServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, value)
	}
}

func planningServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, project.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "project not found", map[string]any{})
	case errors.Is(err, planning.ErrVersionConflict):
		writeError(w, r, http.StatusConflict, "VERSION_CONFLICT", "project planning version conflict", map[string]any{})
	case errors.Is(err, planning.ErrValidation), errors.Is(err, planning.ErrInvalidJSON):
		writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid project planning", map[string]any{})
	default:
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", map[string]any{})
	}
}

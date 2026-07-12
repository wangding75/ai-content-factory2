package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type projectMaterialService interface {
	List(context.Context, uuid.UUID, material.ListOptions) (material.ProjectMaterialList, error)
	CreateAndBindMaterial(context.Context, uuid.UUID, material.CreateProjectMaterialRequest, string, string) (material.ProjectMaterialItem, error)
	BindExistingMaterial(context.Context, uuid.UUID, uuid.UUID, material.ProjectMaterialUsageRequest, string, string) (material.ProjectMaterialItem, error)
	UpdateProjectMaterialUsage(context.Context, uuid.UUID, uuid.UUID, material.UpdateProjectMaterialUsageRequest, string) (material.ProjectMaterialItem, error)
	UnbindProjectMaterial(context.Context, uuid.UUID, uuid.UUID, int, string) (material.UnbindProjectMaterialResult, error)
}

func listProjectMaterialsHandler(s projectMaterialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("projectId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_PROJECT_ID", "projectId must be a UUID", map[string]any{})
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		typ := r.URL.Query().Get("type")
		sort := r.URL.Query().Get("sort")
		if len(q) > 120 || typ != "" && !validMaterialType(typ) || sort != "" && !validMaterialSort(sort) {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid project material query", map[string]any{})
			return
		}
		limit, offset := 20, 0
		if value := r.URL.Query().Get("limit"); value != "" {
			limit, err = strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 100 {
				writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid pagination", map[string]any{})
				return
			}
		}
		if value := r.URL.Query().Get("offset"); value != "" {
			offset, err = strconv.Atoi(value)
			if err != nil || offset < 0 {
				writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid pagination", map[string]any{})
				return
			}
		}
		result, err := s.List(r.Context(), id, material.ListOptions{Query: q, Type: typ, Sort: sort, Limit: limit, Offset: offset})
		if errors.Is(err, project.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "project not found", map[string]any{})
			return
		}
		if errors.Is(err, material.ErrInvalidSort) || errors.Is(err, material.ErrValidation) {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid project material query", map[string]any{})
			return
		}
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", map[string]any{})
			return
		}
		writeJSON(w, r, http.StatusOK, result)
	}
}

func createProjectMaterialHandler(s projectMaterialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("projectId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_PROJECT_ID", "projectId must be a UUID", map[string]any{})
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if strings.TrimSpace(key) == "" || len(key) > 128 {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Idempotency-Key is required", map[string]any{})
			return
		}
		var request material.CreateProjectMaterialRequest
		if err := decodeBody(r, &request); err != nil {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", map[string]any{})
			return
		}
		item, err := s.CreateAndBindMaterial(r.Context(), id, request, key, "system")
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "project not found", map[string]any{})
		case errors.Is(err, material.ErrIdempotencyReused):
			writeError(w, r, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "idempotency key reused", map[string]any{})
		case errors.Is(err, material.ErrAlreadyBound):
			writeError(w, r, http.StatusConflict, "MATERIAL_ALREADY_BOUND", "material already bound", map[string]any{})
		case errors.Is(err, material.ErrValidation):
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid project material request", map[string]any{})
		case err != nil:
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", map[string]any{})
		default:
			writeJSON(w, r, http.StatusCreated, item)
		}
	}
}

func bindProjectMaterialHandler(s projectMaterialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := uuid.Parse(r.PathValue("projectId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_PROJECT_ID", "projectId must be a UUID", map[string]any{})
			return
		}
		materialID, err := uuid.Parse(r.PathValue("materialId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_MATERIAL_ID", "materialId must be a UUID", map[string]any{})
			return
		}
		key := r.Header.Get("Idempotency-Key")
		if strings.TrimSpace(key) == "" || len(key) > 128 {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "Idempotency-Key is required", map[string]any{})
			return
		}
		var request material.ProjectMaterialUsageRequest
		if err := decodeBody(r, &request); err != nil {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", map[string]any{})
			return
		}
		item, err := s.BindExistingMaterial(r.Context(), projectID, materialID, request, key, "system")
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "project not found", map[string]any{})
		case errors.Is(err, material.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "MATERIAL_NOT_FOUND", "material not found", map[string]any{})
		case errors.Is(err, material.ErrIdempotencyReused):
			writeError(w, r, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "idempotency key reused", map[string]any{})
		case errors.Is(err, material.ErrAlreadyBound):
			writeError(w, r, http.StatusConflict, "MATERIAL_ALREADY_BOUND", "material already bound", map[string]any{})
		case errors.Is(err, material.ErrValidation):
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid project material usage", map[string]any{})
		case err != nil:
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", map[string]any{})
		default:
			writeJSON(w, r, http.StatusCreated, item)
		}
	}
}

func updateProjectMaterialUsageHandler(s projectMaterialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := uuid.Parse(r.PathValue("projectId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_PROJECT_ID", "projectId must be a UUID", map[string]any{})
			return
		}
		materialID, err := uuid.Parse(r.PathValue("materialId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_MATERIAL_ID", "materialId must be a UUID", map[string]any{})
			return
		}
		var request material.UpdateProjectMaterialUsageRequest
		if err := decodeBody(r, &request); err != nil {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body", map[string]any{})
			return
		}
		item, err := s.UpdateProjectMaterialUsage(r.Context(), projectID, materialID, request, "system")
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "project not found", map[string]any{})
		case errors.Is(err, material.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "MATERIAL_NOT_FOUND", "material not found", map[string]any{})
		case errors.Is(err, material.ErrUsageNotFound):
			writeError(w, r, http.StatusNotFound, "MATERIAL_BINDING_NOT_FOUND", "material binding not found", map[string]any{})
		case errors.Is(err, material.ErrVersionConflict):
			writeError(w, r, http.StatusConflict, "VERSION_CONFLICT", "usage version conflict", map[string]any{})
		case errors.Is(err, material.ErrValidation):
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid project material usage", map[string]any{})
		case err != nil:
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", map[string]any{})
		default:
			writeJSON(w, r, http.StatusOK, item)
		}
	}
}

func unbindProjectMaterialHandler(s projectMaterialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		projectID, err := uuid.Parse(r.PathValue("projectId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_PROJECT_ID", "projectId must be a UUID", map[string]any{})
			return
		}
		materialID, err := uuid.Parse(r.PathValue("materialId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "INVALID_MATERIAL_ID", "materialId must be a UUID", map[string]any{})
			return
		}
		value := r.URL.Query().Get("expected_version")
		expectedVersion, err := strconv.Atoi(value)
		if value == "" || err != nil || expectedVersion < 1 {
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid expected_version", map[string]any{})
			return
		}
		result, err := s.UnbindProjectMaterial(r.Context(), projectID, materialID, expectedVersion, "system")
		switch {
		case errors.Is(err, project.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "PROJECT_NOT_FOUND", "project not found", map[string]any{})
		case errors.Is(err, material.ErrNotFound):
			writeError(w, r, http.StatusNotFound, "MATERIAL_NOT_FOUND", "material not found", map[string]any{})
		case errors.Is(err, material.ErrVersionConflict):
			writeError(w, r, http.StatusConflict, "VERSION_CONFLICT", "usage version conflict", map[string]any{})
		case errors.Is(err, material.ErrValidation):
			writeError(w, r, http.StatusBadRequest, "VALIDATION_ERROR", "invalid expected_version", map[string]any{})
		case err != nil:
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", map[string]any{})
		default:
			writeJSON(w, r, http.StatusOK, result)
		}
	}
}

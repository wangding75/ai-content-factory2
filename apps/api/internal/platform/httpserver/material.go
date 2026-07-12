package httpserver

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"net/http"
	"strconv"
	"strings"
)

func materialID(r *http.Request) (uuid.UUID, bool) {
	x, e := uuid.Parse(r.PathValue("materialId"))
	return x, e == nil
}
func materialError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, material.ErrNotFound):
		writeError(w, r, 404, "MATERIAL_NOT_FOUND", "material not found", map[string]any{})
	case errors.Is(e, material.ErrVersionConflict):
		writeError(w, r, 409, "VERSION_CONFLICT", "material version conflict", map[string]any{})
	case errors.Is(e, material.ErrIdempotencyReused):
		writeError(w, r, 409, "IDEMPOTENCY_KEY_REUSED", "idempotency key reused", map[string]any{})
	case errors.Is(e, material.ErrValidation), errors.Is(e, material.ErrInvalidSort):
		writeError(w, r, 400, "VALIDATION_ERROR", "invalid material request", map[string]any{})
	default:
		writeError(w, r, 500, "INTERNAL_ERROR", "internal server error", map[string]any{})
	}
}

type materialService interface {
	ListMaterials(context.Context, material.ListOptions) ([]material.Material, int, error)
	CreateMaterial(context.Context, material.CreateRequest, string, string) (material.Material, error)
	GetMaterial(context.Context, uuid.UUID) (material.Detail, error)
	UpdateMaterial(context.Context, uuid.UUID, material.UpdateRequest, string) (material.Material, error)
}

func listMaterialsHandler(s materialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		typ := r.URL.Query().Get("type")
		sort := r.URL.Query().Get("sort")
		if q != "" && len(q) > 120 || typ != "" && !validMaterialType(typ) || sort != "" && !validMaterialSort(sort) {
			writeError(w, r, 400, "VALIDATION_ERROR", "invalid material query", map[string]any{})
			return
		}
		limit, offset := 20, 0
		var e error
		if x := r.URL.Query().Get("limit"); x != "" {
			limit, e = strconv.Atoi(x)
			if e != nil || limit < 1 || limit > 100 {
				writeError(w, r, 400, "VALIDATION_ERROR", "invalid pagination", map[string]any{})
				return
			}
		}
		if x := r.URL.Query().Get("offset"); x != "" {
			offset, e = strconv.Atoi(x)
			if e != nil || offset < 0 {
				writeError(w, r, 400, "VALIDATION_ERROR", "invalid pagination", map[string]any{})
				return
			}
		}
		items, total, e := s.ListMaterials(r.Context(), material.ListOptions{Query: q, Type: typ, Sort: sort, Limit: limit, Offset: offset})
		if e != nil {
			materialError(w, r, e)
			return
		}
		writeJSON(w, r, 200, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
	}
}
func createMaterialHandler(s materialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if strings.TrimSpace(key) == "" || len(key) > 128 {
			writeError(w, r, 400, "VALIDATION_ERROR", "Idempotency-Key is required", map[string]any{})
			return
		}
		var body material.CreateRequest
		if e := decodeBody(r, &body); e != nil {
			writeError(w, r, 400, "VALIDATION_ERROR", "invalid request body", map[string]any{})
			return
		}
		v, e := s.CreateMaterial(r.Context(), body, key, "system")
		if e != nil {
			materialError(w, r, e)
			return
		}
		writeJSON(w, r, 201, v)
	}
}
func getMaterialHandler(s materialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := materialID(r)
		if !ok {
			writeError(w, r, 400, "INVALID_MATERIAL_ID", "materialId must be a UUID", map[string]any{})
			return
		}
		v, e := s.GetMaterial(r.Context(), id)
		if e != nil {
			materialError(w, r, e)
			return
		}
		writeJSON(w, r, 200, v)
	}
}
func updateMaterialHandler(s materialService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := materialID(r)
		if !ok {
			writeError(w, r, 400, "INVALID_MATERIAL_ID", "materialId must be a UUID", map[string]any{})
			return
		}
		var body material.UpdateRequest
		if e := decodeBody(r, &body); e != nil {
			writeError(w, r, 400, "VALIDATION_ERROR", "invalid request body", map[string]any{})
			return
		}
		v, e := s.UpdateMaterial(r.Context(), id, body, "system")
		if e != nil {
			materialError(w, r, e)
			return
		}
		writeJSON(w, r, 200, v)
	}
}
func validMaterialType(x string) bool {
	return x == "character" || x == "worldview" || x == "location" || x == "organization" || x == "item" || x == "reference"
}
func validMaterialSort(x string) bool {
	return x == "updated_at_desc" || x == "updated_at_asc" || x == "name_asc" || x == "name_desc"
}

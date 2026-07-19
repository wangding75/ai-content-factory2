package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"github.com/local/ai-content-factory/apps/api/internal/planning"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type Server struct{ httpServer *http.Server }
type envelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}
type errorEnvelope struct {
	Error     apiError `json:"error"`
	RequestID string   `json:"request_id"`
}
type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func New(address string, projects *project.Service, services ...any) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler)
	mux.HandleFunc("GET /api/v1/meta", metaHandler)
	mux.HandleFunc("GET /api/v1/project-types", listProjectTypesHandler)
	mux.HandleFunc("GET /api/v1/projects", listProjectsHandler(projects))
	mux.HandleFunc("POST /api/v1/projects", createProjectHandler(projects))
	mux.HandleFunc("GET /api/v1/projects/{projectId}", getProjectHandler(projects))
	mux.HandleFunc("PATCH /api/v1/projects/{projectId}", updateProjectHandler(projects))
	mux.HandleFunc("GET /api/v1/projects/{projectId}/workspace", workspaceHandler(projects))
	for _, service := range services {
		switch value := service.(type) {
		case *planning.Service:
			if value != nil {
				mux.HandleFunc("GET /api/v1/projects/{projectId}/planning", getProjectPlanningHandler(value))
				mux.HandleFunc("PUT /api/v1/projects/{projectId}/planning", putProjectPlanningHandler(value))
			}
		case *material.Service:
			if value != nil {
				mux.HandleFunc("GET /api/v1/materials", listMaterialsHandler(value))
				mux.HandleFunc("POST /api/v1/materials", createMaterialHandler(value))
				mux.HandleFunc("GET /api/v1/materials/{materialId}", getMaterialHandler(value))
				mux.HandleFunc("PATCH /api/v1/materials/{materialId}", updateMaterialHandler(value))
			}
		case storylineApplication:
			if value != nil {
				registerStorylineRoutes(mux, value)
			}
		case foreshadowingApplication:
			if value != nil {
				registerForeshadowingRoutes(mux, value)
			}
		case chapterPlanApplication:
			if value != nil {
				registerChapterPlanRoutes(mux, value)
			}
		case contentItemApplication:
			if value != nil {
				registerContentItemRoutes(mux, value)
			}
		case *contentitem.Iteration07Application:
			if value != nil {
				registerIteration07Routes(mux, value)
			}
		case *contentitem.GlobalLiteService:
			if value != nil {
				registerIteration08Routes(mux, value)
			}
		case *material.ProjectMaterialService:
			if value != nil {
				mux.HandleFunc("GET /api/v1/projects/{projectId}/materials", listProjectMaterialsHandler(value))
				mux.HandleFunc("POST /api/v1/projects/{projectId}/materials", createProjectMaterialHandler(value))
				mux.HandleFunc("POST /api/v1/projects/{projectId}/materials/{materialId}/binding", bindProjectMaterialHandler(value))
				mux.HandleFunc("DELETE /api/v1/projects/{projectId}/materials/{materialId}/binding", unbindProjectMaterialHandler(value))
				mux.HandleFunc("PATCH /api/v1/projects/{projectId}/materials/{materialId}/usage", updateProjectMaterialUsageHandler(value))
			}
		case *globalconfig.Service:
			if value != nil {
				registerGlobalConfigurationRoutes(mux, value)
			}
		}
	}
	return &Server{httpServer: &http.Server{Addr: address, Handler: withRequestID(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}}
}
func (s *Server) ListenAndServe() error { return s.httpServer.ListenAndServe() }
func (s *Server) Shutdown() error       { return s.httpServer.Close() }
func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{"status": "ok", "service": "api"})
}
func readyHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{"status": "ready", "checks": map[string]string{"api": "ok"}})
}
func metaHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{"product": "AI Content Factory 2.0", "scope": "P0", "content_packs": []string{"novel"}, "workflow_provider": "mock", "real_ai": "disabled", "external_workflow": "disabled", "publishing": "disabled"})
}
func writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Data: data, RequestID: requestIDFrom(r)})
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: apiError{code, message, details}, RequestID: requestIDFrom(r)})
}
func projectID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("projectId"))
	return id, err == nil
}
func serviceError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, project.ErrNotFound) {
		writeError(w, r, 404, "project_not_found", "project not found", map[string]any{})
		return
	}
	writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
}
func listProjectsHandler(s *project.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := 20, 0
		var err error
		if v := r.URL.Query().Get("limit"); v != "" {
			limit, err = strconv.Atoi(v)
			if err != nil || limit < 1 || limit > 100 {
				writeError(w, r, 400, "validation_error", "invalid limit", map[string]any{})
				return
			}
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			offset, err = strconv.Atoi(v)
			if err != nil || offset < 0 {
				writeError(w, r, 400, "validation_error", "invalid offset", map[string]any{})
				return
			}
		}
		status := r.URL.Query().Get("status")
		if status != "" && status != "planning" && status != "producing" && status != "archived" {
			writeError(w, r, 400, "validation_error", "invalid status", map[string]any{})
			return
		}
		q := r.URL.Query().Get("q")
		if len(q) > 120 {
			writeError(w, r, 400, "validation_error", "invalid q", map[string]any{})
			return
		}
		items, total, err := s.List(r.Context(), project.ListOptions{Status: status, Query: q, Limit: limit, Offset: offset})
		if err != nil {
			serviceError(w, r, err)
			return
		}
		writeJSON(w, r, 200, map[string]any{"items": items, "total": total, "limit": limit, "offset": offset})
	}
}

func listProjectTypesHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{"items": project.ProjectTypes()})
}

type createRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

func createProjectHandler(s *project.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body createRequest
		if err := decodeBody(r, &body); err != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		p, err := s.Create(r.Context(), body.Name, body.Type, body.Description, "system")
		if errors.Is(err, project.ErrValidation) {
			writeError(w, r, 400, "validation_error", "invalid project", map[string]any{})
			return
		}
		if err != nil {
			serviceError(w, r, err)
			return
		}
		writeJSON(w, r, 201, p)
	}
}
func getProjectHandler(s *project.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		p, err := s.Get(r.Context(), id)
		if err != nil {
			serviceError(w, r, err)
			return
		}
		writeJSON(w, r, 200, p)
	}
}

type updateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

func updateProjectHandler(s *project.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		var body updateRequest
		if err := decodeBody(r, &body); err != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		p, err := s.Update(r.Context(), id, body.Name, body.Description)
		if errors.Is(err, project.ErrValidation) {
			writeError(w, r, 400, "validation_error", "invalid project update", map[string]any{})
			return
		}
		if err != nil {
			serviceError(w, r, err)
			return
		}
		writeJSON(w, r, 200, p)
	}
}
func workspaceHandler(s *project.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		data, err := s.Workspace(r.Context(), id)
		if err != nil {
			serviceError(w, r, err)
			return
		}
		writeJSON(w, r, 200, data)
	}
}
func decodeBody(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

var _ = strings.TrimSpace

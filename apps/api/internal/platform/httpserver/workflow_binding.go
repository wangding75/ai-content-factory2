package httpserver

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
)

// registerWorkflowBindingRoutes mounts the Iteration 13 project workflow binding
// GET / PUT / DELETE routes on the server mux.  All responses use the shared
// writeJSON / writeError helpers so the envelope is a single {data, request_id}
// (or {error, request_id}) object and the X-Request-ID header matches the body
// request_id exactly.
func registerWorkflowBindingRoutes(m *http.ServeMux, svc workflowbinding.BindingService) {
	h := &workflowBindingHandler{svc: svc}
	m.HandleFunc("GET /api/v1/projects/{projectId}/workflow-bindings", h.list)
	m.HandleFunc("PUT /api/v1/projects/{projectId}/workflow-bindings/{stage}", h.put)
	m.HandleFunc("DELETE /api/v1/projects/{projectId}/workflow-bindings/{stage}", h.delete)
}

type workflowBindingHandler struct {
	svc workflowbinding.BindingService
}

func (h *workflowBindingHandler) list(w http.ResponseWriter, r *http.Request) {
	projectID, ok := workflowBindingPathUUID(w, r, "projectId")
	if !ok {
		return
	}
	items, err := h.svc.ListStages(r.Context(), projectID)
	if err != nil {
		workflowBindingError(w, r, err)
		return
	}
	stages := make([]workflowbinding.WorkflowBindingStageDTO, 0, len(items))
	for _, item := range items {
		stages = append(stages, workflowbinding.StageDTO(item))
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": stages})
}

func (h *workflowBindingHandler) put(w http.ResponseWriter, r *http.Request) {
	projectID, ok := workflowBindingPathUUID(w, r, "projectId")
	if !ok {
		return
	}
	stage, ok := workflowBindingPathStage(w, r)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		writeError(w, r, http.StatusBadRequest, "validation_error", "Idempotency-Key is required", map[string]any{"fields": map[string]string{"Idempotency-Key": "required_or_too_long"}})
		return
	}
	var body workflowbinding.PutRequest
	if err := decodeBody(r, &body); err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{"fields": map[string]string{"body": "invalid_json_or_unknown_field"}})
		return
	}
	result, status, err := h.svc.PutWithIdempotency(r.Context(), projectID, stage, body, key)
	if err != nil {
		workflowBindingError(w, r, err)
		return
	}
	dto := workflowbinding.StageDTO(workflowbinding.StageRead{Stage: result.Stage, Bound: true, Binding: &result.Binding, WorkflowConfigurationSummary: &result.Summary})
	writeJSON(w, r, status, dto)
}

func (h *workflowBindingHandler) delete(w http.ResponseWriter, r *http.Request) {
	projectID, ok := workflowBindingPathUUID(w, r, "projectId")
	if !ok {
		return
	}
	stage, ok := workflowBindingPathStage(w, r)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		writeError(w, r, http.StatusBadRequest, "validation_error", "Idempotency-Key is required", map[string]any{"fields": map[string]string{"Idempotency-Key": "required_or_too_long"}})
		return
	}
	rawVersion := r.URL.Query().Get("expected_version")
	if rawVersion == "" {
		writeError(w, r, http.StatusBadRequest, "validation_error", "expected_version is required", map[string]any{"fields": map[string]string{"expected_version": "required"}})
		return
	}
	version, err := strconv.Atoi(rawVersion)
	if err != nil || version < 1 {
		writeError(w, r, http.StatusBadRequest, "validation_error", "invalid expected_version", map[string]any{"fields": map[string]string{"expected_version": "invalid_integer"}})
		return
	}
	result, _, err := h.svc.DeleteWithIdempotency(r.Context(), projectID, stage, workflowbinding.DeleteRequest{ExpectedVersion: version}, key)
	if err != nil {
		workflowBindingError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, workflowbinding.UnbindDTO(result))
}

func workflowBindingPathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(r.PathValue(name)))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", "projectId must be a UUID", map[string]any{"fields": map[string]string{name: "invalid_uuid"}})
		return uuid.Nil, false
	}
	return id, true
}

func workflowBindingPathStage(w http.ResponseWriter, r *http.Request) (workflowbinding.WorkflowBindingStage, bool) {
	stage, err := workflowbinding.ParseStage(r.PathValue("stage"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", "invalid stage", map[string]any{"fields": map[string]string{"stage": "invalid"}})
		return "", false
	}
	return stage, true
}

// workflowBindingError maps domain errors to the frozen Iteration 13 response
// codes.  Version conflicts surface expectedVersion / currentVersion /
// projectId / stage in the 409 details.
func workflowBindingError(w http.ResponseWriter, r *http.Request, err error) {
	var conflict *workflowbinding.VersionConflictError
	switch {
	case errors.As(err, &conflict):
		writeError(w, r, http.StatusConflict, "version_conflict", "workflow binding version conflict", map[string]any{
			"expectedVersion": conflict.ExpectedVersion,
			"currentVersion":  conflict.CurrentVersion,
			"projectId":       conflict.ProjectID,
			"stage":           conflict.Stage.String(),
		})
	case errors.Is(err, workflowbinding.ErrProjectNotFound):
		writeError(w, r, http.StatusNotFound, "project_not_found", "project not found", map[string]any{})
	case errors.Is(err, workflowbinding.ErrConfigurationNotFound):
		writeError(w, r, http.StatusNotFound, "configuration_not_found", "workflow configuration not found", map[string]any{})
	case errors.Is(err, workflowbinding.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "workflow_binding_not_found", "workflow binding not found", map[string]any{})
	case errors.Is(err, workflowbinding.ErrBindingAlreadyExists):
		writeError(w, r, http.StatusConflict, "binding_already_exists", "binding already exists", map[string]any{})
	case errors.Is(err, workflowbinding.ErrIdempotencyReused):
		writeError(w, r, http.StatusConflict, "idempotency_key_reused_with_different_payload", "idempotency key reused", map[string]any{})
	case errors.Is(err, workflowbinding.ErrDisabledWorkflow):
		writeError(w, r, http.StatusUnprocessableEntity, "disabled_workflow", "workflow is not enabled", map[string]any{})
	case errors.Is(err, workflowbinding.ErrNotApplicable):
		writeError(w, r, http.StatusUnprocessableEntity, "workflow_not_applicable_to_stage", "workflow not applicable to stage", map[string]any{})
	case errors.Is(err, workflowbinding.ErrValidation):
		writeError(w, r, http.StatusBadRequest, "validation_error", "invalid request", map[string]any{})
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error", map[string]any{})
	}
}

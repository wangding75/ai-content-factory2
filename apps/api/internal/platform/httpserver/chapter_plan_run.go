package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type chapterPlanPreflightRequest struct {
	GenerationMode     string                              `json:"generationMode"`
	Target             chapterplan.GenerationTargetRequest `json:"target"`
	StorylineSelection struct {
		StorylineIDs []uuid.UUID `json:"storylineIds"`
	} `json:"storylineSelection"`
	ContextOptions         json.RawMessage `json:"contextOptions"`
	AdditionalInstructions *string         `json:"additionalInstructions"`
}
type createChapterPlanRunRequest struct {
	PreflightToken string `json:"preflightToken"`
}

func registerChapterPlanRunRoutes(mux *http.ServeMux, app chapterPlanRunApplication) {
	mux.HandleFunc("POST /api/v1/projects/{projectId}/chapter-plan-runs/preflight", chapterPlanPreflightHandler(app))
	mux.HandleFunc("POST /api/v1/projects/{projectId}/chapter-plan-runs", createChapterPlanRunHandler(app))
}
func chapterPlanPreflightHandler(app chapterPlanRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		var body chapterPlanPreflightRequest
		if decodeBody(r, &body) != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		result, err := app.Preflight(r.Context(), id, chapterplan.PreflightRequest{GenerationMode: body.GenerationMode, Target: body.Target, StorylineIDs: body.StorylineSelection.StorylineIDs, ContextOptions: body.ContextOptions, AdditionalInstructions: body.AdditionalInstructions, ActorID: "system"})
		if err != nil {
			chapterPlanRunError(w, r, err)
			return
		}
		writeJSON(w, r, 200, map[string]any{"result": "passed", "status": "passed", "preflightToken": result.Token, "expiresAt": result.ExpiresAt, "inputDigest": result.InputDigest, "inputSummary": map[string]any{"generationMode": body.GenerationMode, "target": result.Target, "storylineSelection": body.StorylineSelection, "contextOptions": json.RawMessage(body.ContextOptions)}, "executionConfigurationSummary": map[string]any{"stage": "chapter_planning", "workflowBindingId": result.BindingID, "workflowBindingVersion": result.BindingVersion}, "checks": []any{}, "blockers": []any{}, "warnings": []any{}})
	}
}
func createChapterPlanRunHandler(app chapterPlanRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeError(w, r, 400, "validation_error", "Idempotency-Key header is required", map[string]any{})
			return
		}
		var body createChapterPlanRunRequest
		if decodeBody(r, &body) != nil || strings.TrimSpace(body.PreflightToken) == "" {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		run, err := app.CreateChapterPlanningRun(r.Context(), id, "system", body.PreflightToken, key)
		if err != nil {
			chapterPlanRunError(w, r, err)
			return
		}
		writeJSON(w, r, 201, iteration14WorkflowRunResponse(run))
	}
}
func chapterPlanRunError(w http.ResponseWriter, r *http.Request, err error) {
	details := map[string]any{"retryAction": "rerun_preflight", "safeReason": "The chapter-planning input must be checked again."}
	switch {
	case errors.Is(err, chapterplan.ErrPreflightTokenExpired):
		writeError(w, r, 422, "preflight_token_expired", "preflight token has expired", details)
	case errors.Is(err, chapterplan.ErrPreflightTokenInvalid):
		writeError(w, r, 422, "preflight_token_invalid", "preflight token is invalid", details)
	case errors.Is(err, chapterplan.ErrPreflightInputChanged):
		writeError(w, r, 409, "preflight_input_changed", "preflight input has changed", details)
	case errors.Is(err, chapterplan.ErrWorkflowNotConfigured):
		writeError(w, r, 422, "workflow_not_configured", "workflow is not configured", details)
	case errors.Is(err, workflowrun.ErrIdempotencyConflict):
		writeError(w, r, 409, "idempotency_key_reused_with_different_payload", "idempotency key was reused", details)
	default:
		writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
	}
}

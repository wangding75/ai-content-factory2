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
		Mode         string      `json:"mode"`
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
	mux.HandleFunc("POST /api/v1/workflow-runs/{workflowRunId}/chapter-planning-result-consumption-retries", chapterPlanResultConsumptionRetryHandler(app))
}

func chapterPlanResultConsumptionRetryHandler(app chapterPlanRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID, err := uuid.Parse(r.PathValue("workflowRunId"))
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		var body struct {
			ExpectedRunVersion int `json:"expectedRunVersion"`
		}
		if err != nil || key == "" || len(key) > 128 || decodeBody(r, &body) != nil || body.ExpectedRunVersion < 1 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid result consumption retry request", map[string]any{})
			return
		}
		run, err := app.RetryChapterPlanningResultConsumption(r.Context(), runID, body.ExpectedRunVersion, key)
		if err != nil {
			switch {
			case errors.Is(err, workflowrun.ErrVersionConflict):
				writeError(w, r, http.StatusConflict, "workflow_run_version_conflict", "workflow run version changed", map[string]any{})
			case errors.Is(err, workflowrun.ErrNotFound):
				writeError(w, r, http.StatusNotFound, "workflow_run_not_found", "workflow run was not found", map[string]any{})
			case errors.Is(err, workflowrun.ErrNotRetryable):
				writeError(w, r, http.StatusConflict, "result_consumption_failed", "workflow run result is not retryable", map[string]any{})
			default:
				writeError(w, r, http.StatusInternalServerError, "result_consumption_failed", "chapter planning result could not be stored safely", map[string]any{})
			}
			return
		}
		writeJSON(w, r, http.StatusOK, iteration14WorkflowRunResponse(run))
	}
}
func chapterPlanPreflightHandler(app chapterPlanRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, actorOK := actorIDFromRequest(r)
		if !actorOK || strings.TrimSpace(actorID) == "" {
			writeError(w, r, 422, "preflight_token_invalid", "request actor is unavailable", map[string]any{})
			return
		}
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
		result, err := app.Preflight(r.Context(), id, chapterplan.PreflightRequest{GenerationMode: body.GenerationMode, Target: body.Target, StorylineSelectionMode: body.StorylineSelection.Mode, StorylineIDs: body.StorylineSelection.StorylineIDs, ContextOptions: body.ContextOptions, AdditionalInstructions: body.AdditionalInstructions, ActorID: actorID})
		if err != nil {
			chapterPlanRunError(w, r, err)
			return
		}
		blockers := make([]any, 0, len(result.Blockers))
		for _, blocker := range result.Blockers {
			item := map[string]any{"code": blocker.Code, "message": blocker.Message, "safeReason": blocker.SafeReason, "retryAction": blocker.RetryAction}
			if blocker.RepairTarget != nil { item["repairTarget"] = blocker.RepairTarget }
			blockers = append(blockers, item)
		}
		var inputSummary any
		if completePreflightInputSummary(body, result) {
			selection := map[string]any{"mode": body.StorylineSelection.Mode}
			if body.StorylineSelection.Mode == "specified" {
				selection["storylineIds"] = body.StorylineSelection.StorylineIDs
			}
			inputSummary = map[string]any{"generationMode": body.GenerationMode, "target": result.Target, "storylineSelection": selection, "contextOptions": json.RawMessage(body.ContextOptions)}
		}
		var executionConfigurationSummary any
		if result.BindingID != uuid.Nil && result.BindingVersion > 0 {
			executionConfigurationSummary = map[string]any{"stage": "chapter_planning", "workflowBindingId": result.BindingID, "workflowBindingVersion": result.BindingVersion}
		}
		response := map[string]any{"result": "blocked", "status": "blocked", "inputDigest": result.InputDigest, "inputSummary": inputSummary, "executionConfigurationSummary": executionConfigurationSummary, "checks": []any{}, "blockers": blockers, "warnings": []any{}}
		if result.Passed {
			response["result"], response["status"], response["preflightToken"], response["expiresAt"] = "passed", "passed", result.Token, result.ExpiresAt
		}
		writeJSON(w, r, 200, response)
	}
}

func completePreflightInputSummary(body chapterPlanPreflightRequest, result chapterplan.PreflightResult) bool {
	if result.Target.StartChapterNo < 1 || result.Target.EndChapterNo < result.Target.StartChapterNo || result.Target.RequestedChapterCount < 1 || !json.Valid(body.ContextOptions) {
		return false
	}
	if body.StorylineSelection.Mode == "auto_balanced" {
		return true
	}
	return body.StorylineSelection.Mode == "specified" && len(body.StorylineSelection.StorylineIDs) > 0
}
func createChapterPlanRunHandler(app chapterPlanRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, actorOK := actorIDFromRequest(r)
		if !actorOK || strings.TrimSpace(actorID) == "" {
			writeError(w, r, 422, "preflight_token_invalid", "request actor is unavailable", map[string]any{})
			return
		}
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
		run, err := app.CreateChapterPlanningRun(r.Context(), id, actorID, body.PreflightToken, key)
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

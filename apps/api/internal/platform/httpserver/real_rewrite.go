package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type rewritePreflightRequest struct {
	SelectedIssueIDs     []uuid.UUID               `json:"selectedIssueIds"`
	OptionalInstructions *string                   `json:"optionalInstructions"`
	RewriteOptions       contentitem.RewriteOptions `json:"rewriteOptions"`
}

type rewriteCreateRequest struct {
	PreflightToken string `json:"preflightToken"`
}

type realRewriteApplication interface {
	Availability(context.Context, uuid.UUID) (contentitem.RewriteAvailability, error)
	Preflight(context.Context, uuid.UUID, contentitem.RewritePreflightRequest) (contentitem.RewritePreflightResult, error)
	CreateRun(context.Context, uuid.UUID, string, string, string) (workflowrun.WorkflowRun, bool, error)
}

func registerRealRewriteRoutes(mux *http.ServeMux, service realRewriteApplication) {
	mux.HandleFunc("GET /api/v1/reviews/{reviewId}/rewrite-availability", realRewriteAvailabilityHandler(service))
	mux.HandleFunc("POST /api/v1/reviews/{reviewId}/rewrites/preflight", realRewritePreflightHandler(service))
	mux.HandleFunc("POST /api/v1/reviews/{reviewId}/rewrites", realRewriteCreateHandler(service))
}

func realRewriteReviewID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("reviewId"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", "reviewId must be a UUID", map[string]any{})
		return uuid.Nil, false
	}
	return id, true
}

func realRewriteAvailabilityHandler(service realRewriteApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviewID, ok := realRewriteReviewID(w, r)
		if !ok { return }
		result, err := service.Availability(r.Context(), reviewID)
		if err != nil {
			realRewriteError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, result)
	}
}

func realRewritePreflightHandler(service realRewriteApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviewID, ok := realRewriteReviewID(w, r)
		if !ok { return }
		var body rewritePreflightRequest
		if decodeBody(r, &body) != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid request body", map[string]any{})
			return
		}
		result, err := service.Preflight(r.Context(), reviewID, contentitem.RewritePreflightRequest{
			SelectedIssueIDs: body.SelectedIssueIDs, OptionalInstructions: body.OptionalInstructions,
			RewriteOptions: body.RewriteOptions, ActorID: requestActorID(r),
		})
		if err != nil {
			realRewriteError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, result)
	}
}

func realRewriteCreateHandler(service realRewriteApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviewID, ok := realRewriteReviewID(w, r)
		if !ok { return }
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		var body rewriteCreateRequest
		if key == "" || len(key) > 128 || decodeBody(r, &body) != nil || strings.TrimSpace(body.PreflightToken) == "" || len(body.PreflightToken) > 4096 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid request body or Idempotency-Key", map[string]any{})
			return
		}
		run, replay, err := service.CreateRun(r.Context(), reviewID, requestActorID(r), body.PreflightToken, key)
		if err != nil {
			realRewriteError(w, r, err)
			return
		}
		status := http.StatusCreated
		if replay { status = http.StatusOK }
		writeJSON(w, r, status, realRewriteWorkflowRunResponse(run))
	}
}

func realRewriteWorkflowRunResponse(run workflowrun.WorkflowRun) workflowRunDTO {
	response := iteration14WorkflowRunResponse(run)
	var snapshot struct {
		ProjectID uuid.UUID `json:"projectId"`
		Stage string `json:"stage"`
		Binding struct {
			ID uuid.UUID `json:"id"`
			Version int `json:"version"`
		} `json:"binding"`
		WorkflowConfiguration struct {
			ID uuid.UUID `json:"id"`
			Version int `json:"version"`
			InputContractVersion string `json:"inputContractVersion"`
			OutputContractVersion string `json:"outputContractVersion"`
		} `json:"workflowConfiguration"`
		WorkflowConnection struct {
			ID uuid.UUID `json:"id"`
			Version int `json:"version"`
			Type string `json:"type"`
		} `json:"workflowConnection"`
		CreatedAt time.Time `json:"createdAt"`
	}
	if json.Unmarshal(run.ConfigurationSnapshot, &snapshot) == nil {
		if safe, err := json.Marshal(snapshot); err == nil { response.ConfigurationSnapshot = safe }
	}
	return response
}

func realRewriteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, contentitem.ErrReviewNotFound):
		writeError(w, r, http.StatusNotFound, "review_not_found", "requested review was not found", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewIssueNotFound):
		writeError(w, r, http.StatusNotFound, "review_issue_not_found", "requested review issue was not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "workflow_run_not_found", "requested workflow run was not found", map[string]any{})
	case errors.Is(err, contentitem.ErrRewriteNotConfigured):
		writeError(w, r, http.StatusUnprocessableEntity, "rewrite_not_configured", "重写工作流尚未配置", map[string]any{})
	case errors.Is(err, contentitem.ErrRewriteNotAvailable):
		writeError(w, r, http.StatusUnprocessableEntity, "rewrite_not_available", "当前审核报告不可创建重写任务", map[string]any{})
	case errors.Is(err, contentitem.ErrRewritePreflightExpired):
		writeError(w, r, http.StatusUnprocessableEntity, "rewrite_preflight_expired", "重写预检令牌已过期", map[string]any{})
	case errors.Is(err, contentitem.ErrRewritePreflightConsumed):
		writeError(w, r, http.StatusConflict, "rewrite_preflight_consumed", "重写预检令牌已使用", map[string]any{})
	case errors.Is(err, contentitem.ErrRewritePreflightStale), errors.Is(err, contentitem.ErrRewriteTokenInvalid):
		writeError(w, r, http.StatusConflict, "rewrite_preflight_stale", "重写预检输入已变化", map[string]any{})
	case errors.Is(err, contentitem.ErrRewriteActiveRun):
		writeError(w, r, http.StatusConflict, "active_rewrite_run_conflict", "该审核报告已有活跃重写任务", map[string]any{})
	case errors.Is(err, workflowrun.ErrIdempotencyConflict), errors.Is(err, contentitem.ErrIdempotencyConflict):
		writeError(w, r, http.StatusConflict, "idempotency_conflict", "幂等键已用于不同请求", map[string]any{})
	case errors.Is(err, workflowrun.ErrVersionConflict):
		writeError(w, r, http.StatusConflict, "workflow_run_version_conflict", "工作流运行版本冲突", map[string]any{})
	case errors.Is(err, contentitem.ErrValidation):
		writeError(w, r, http.StatusUnprocessableEntity, "rewrite_not_available", "重写请求无效", map[string]any{})
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error", map[string]any{})
	}
}

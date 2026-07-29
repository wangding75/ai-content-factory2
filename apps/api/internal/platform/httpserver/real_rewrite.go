package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
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
	Summary(context.Context, uuid.UUID) (contentitem.RewriteSummary, error)
	History(context.Context, uuid.UUID, int, int) (contentitem.RewriteHistory, error)
	Result(context.Context, uuid.UUID) (contentitem.RewriteResult, error)
	RetryResultConsumption(context.Context, uuid.UUID, contentitem.RetryRewriteConsumptionRequest) (contentitem.RewriteResult, error)
}

func registerRealRewriteRoutes(mux *http.ServeMux, service realRewriteApplication) {
	mux.HandleFunc("GET /api/v1/reviews/{reviewId}/rewrite-availability", realRewriteAvailabilityHandler(service))
	mux.HandleFunc("POST /api/v1/reviews/{reviewId}/rewrites/preflight", realRewritePreflightHandler(service))
	mux.HandleFunc("POST /api/v1/reviews/{reviewId}/rewrites", realRewriteCreateHandler(service))
	mux.HandleFunc("GET /api/v1/reviews/{reviewId}/rewrite-summary", realRewriteSummaryHandler(service))
	mux.HandleFunc("GET /api/v1/content-items/{contentItemId}/rewrite-history", realRewriteHistoryHandler(service))
	mux.HandleFunc("GET /api/v1/workflow-runs/{workflowRunId}/rewrite-result", realRewriteResultHandler(service))
	mux.HandleFunc("POST /api/v1/workflow-runs/{workflowRunId}/rewrite-result-consumption-retries", realRewriteConsumptionRetryHandler(service))
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

func realRewriteSummaryHandler(service realRewriteApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviewID, ok := realRewriteReviewID(w, r)
		if !ok { return }
		result, err := service.Summary(r.Context(), reviewID)
		if err != nil {
			realRewriteError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, realRewriteSummaryResponse(result))
	}
}

func realRewriteHistoryHandler(service realRewriteApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemID, err := uuid.Parse(r.PathValue("contentItemId"))
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "validation_error", "contentItemId must be a UUID", map[string]any{})
			return
		}
		limit, offset := 20, 0
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			limit, err = strconv.Atoi(raw)
			if err != nil || limit < 1 || limit > 100 {
				writeError(w, r, http.StatusBadRequest, "validation_error", "limit must be between 1 and 100", map[string]any{})
				return
			}
		}
		if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
			offset, err = strconv.Atoi(raw)
			if err != nil || offset < 0 {
				writeError(w, r, http.StatusBadRequest, "validation_error", "offset must be at least 0", map[string]any{})
				return
			}
		}
		result, err := service.History(r.Context(), itemID, limit, offset)
		if err != nil {
			realRewriteError(w, r, err)
			return
		}
		items := make([]any, 0, len(result.Items))
		for _, item := range result.Items {
			var candidate any
			if item.CandidateVersion != nil {
				candidate = realRewriteCandidateResponse(*item.CandidateVersion, false)
			}
			items = append(items, map[string]any{
				"reviewReportSnapshot": item.ReviewReportSnapshot,
				"sourceContentVersionSummary": item.SourceContentVersionSummary,
				"workflowRun": realRewriteQueryRunResponse(item.WorkflowRun),
				"state": item.State, "candidateVersion": candidate,
				"candidateIsCurrent": item.CandidateIsCurrent, "latestError": item.LatestError,
			})
		}
		writeJSON(w, r, http.StatusOK, map[string]any{
			"items": items, "total": result.Total, "limit": result.Limit, "offset": result.Offset,
		})
	}
}

func realRewriteWorkflowRunID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("workflowRunId"))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "validation_error", "workflowRunId must be a UUID", map[string]any{})
		return uuid.Nil, false
	}
	return id, true
}

func realRewriteResultHandler(service realRewriteApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID, ok := realRewriteWorkflowRunID(w, r)
		if !ok { return }
		result, err := service.Result(r.Context(), runID)
		if err != nil {
			realRewriteError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, realRewriteResultResponse(result))
	}
}

func realRewriteConsumptionRetryHandler(service realRewriteApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID, ok := realRewriteWorkflowRunID(w, r)
		if !ok { return }
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		var body struct {
			ExpectedRunVersion int `json:"expectedRunVersion"`
		}
		if key == "" || len(key) > 128 || decodeBody(r, &body) != nil || body.ExpectedRunVersion < 1 {
			writeError(w, r, http.StatusBadRequest, "validation_error", "invalid request body or Idempotency-Key", map[string]any{})
			return
		}
		result, err := service.RetryResultConsumption(r.Context(), runID, contentitem.RetryRewriteConsumptionRequest{
			ExpectedRunVersion: body.ExpectedRunVersion, IdempotencyKey: key,
		})
		if err != nil {
			realRewriteError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, realRewriteResultResponse(result))
	}
}

func realRewriteQueryRunResponse(run workflowrun.WorkflowRun) workflowRunDTO {
	response := iteration14WorkflowRunResponse(run)
	response.InputPayload = json.RawMessage(`{}`)
	response.OutputPayload = nil
	response.ErrorCode = nil
	response.ErrorMessage = nil
	response.ErrorDetails = nil
	response.ConfigurationSnapshot = json.RawMessage(`{}`)
	return response
}

func realRewriteCandidateResponse(value contentitem.ContentVersion, includeContent bool) map[string]any {
	content := ""
	summary := any(nil)
	if includeContent {
		content = value.Content
		summary = value.Summary
	}
	return map[string]any{
		"id": value.ID, "content_item_id": value.ContentItemID, "version_no": value.VersionNo,
		"version": value.Version, "status": value.Status, "source": value.Source,
		"source_content_version_id": value.SourceContentVersionID,
		"source_content_version_version": value.SourceContentVersionVersion,
		"source_workflow_run_id": value.SourceWorkflowRunID,
		"title": value.Title, "content": content, "summary": summary, "word_count": value.WordCount,
		"frozen_at": value.FrozenAt, "created_at": value.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": value.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func realRewriteSummaryResponse(value contentitem.RewriteSummary) map[string]any {
	var activeRun, latestRun, selectedIssues, candidate any
	if value.ActiveRun != nil {
		activeRun = realRewriteQueryRunResponse(*value.ActiveRun)
	}
	if value.LatestRun != nil {
		latestRun = realRewriteQueryRunResponse(*value.LatestRun)
	}
	if value.SelectedIssueSummary != nil {
		selectedIssues = value.SelectedIssueSummary
	}
	if value.CandidateVersion != nil {
		candidate = realRewriteCandidateResponse(*value.CandidateVersion, false)
	}
	return map[string]any{
		"reviewReportId": value.ReviewReportID, "contentItemId": value.ContentItemID,
		"state": value.State, "canStartRewrite": value.CanStartRewrite,
		"activeRun": activeRun, "latestRun": latestRun,
		"sourceContentVersionSummary": value.SourceContentVersionSummary,
		"selectedIssueSummary": selectedIssues, "candidateVersion": candidate,
		"candidateIsCurrent": value.CandidateIsCurrent, "canSetCurrent": value.CanSetCurrent,
		"latestError": value.LatestError, "configurationSummary": value.ConfigurationSummary,
	}
}

func realRewriteResultResponse(value contentitem.RewriteResult) map[string]any {
	return map[string]any{
		"reviewReportSnapshot": value.ReviewReportSnapshot,
		"sourceContentVersionSummary": value.SourceContentVersionSummary,
		"selectedIssueSummary": value.SelectedIssueSummary,
		"workflowRun": realRewriteQueryRunResponse(value.WorkflowRun),
		"output": value.Output,
		"candidateVersion": realRewriteCandidateResponse(value.CandidateVersion, true),
		"candidateIsCurrent": value.CandidateIsCurrent, "canSetCurrent": value.CanSetCurrent,
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
	case errors.Is(err, contentitem.ErrRewriteCandidateNotFound):
		writeError(w, r, http.StatusNotFound, "rewrite_candidate_not_found", "requested rewrite candidate was not found", map[string]any{})
	case errors.Is(err, contentitem.ErrRewriteCandidateNotReady), errors.Is(err, contentitem.ErrRewriteRunNotConsumable):
		writeError(w, r, http.StatusConflict, "rewrite_candidate_not_ready", "rewrite candidate is not ready", map[string]any{})
	case errors.Is(err, contentitem.ErrRewriteOutputInvalid):
		writeError(w, r, http.StatusConflict, "rewrite_output_validation_failed", "rewrite output validation failed", map[string]any{})
	case errors.Is(err, contentitem.ErrRewriteResultConsumption):
		writeError(w, r, http.StatusInternalServerError, "rewrite_result_consumption_failed", "rewrite result could not be stored safely", map[string]any{})
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

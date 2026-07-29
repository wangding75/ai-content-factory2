package httpserver

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type reviewPreflightRequest struct {
	SourceContentVersionVersion int             `json:"sourceContentVersionVersion"`
	OptionalInstructions        json.RawMessage `json:"optionalInstructions"`
}

type reviewCreateRequest struct {
	PreflightToken string `json:"preflightToken"`
}

type reviewConsumptionRetryRequest struct {
	ExpectedRunVersion int `json:"expectedRunVersion"`
}

type reviewIssueUpdateRequest struct {
	Disposition    string `json:"disposition"`
	ExpectedVersion int   `json:"expectedVersion"`
}

func registerRealReviewRoutes(mux *http.ServeMux, service *contentitem.RealReviewService) {
	mux.HandleFunc("POST /api/v1/content-versions/{contentVersionId}/review-runs/preflight", realReviewPreflightHandler(service))
	mux.HandleFunc("POST /api/v1/content-versions/{contentVersionId}/review-runs", realReviewCreateHandler(service))
	mux.HandleFunc("GET /api/v1/content-items/{contentItemId}/review-summary", realReviewSummaryHandler(service))
	mux.HandleFunc("PATCH /api/v1/reviews/{reviewId}/issues/{issueId}", realReviewIssueHandler(service))
	mux.HandleFunc("POST /api/v1/workflow-runs/{workflowRunId}/review-result-consumption-retries", realReviewConsumptionRetryHandler(service))
}

func realReviewPreflightHandler(service *contentitem.RealReviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		versionID, err := uuid.Parse(r.PathValue("contentVersionId"))
		if err != nil {
			writeError(w, r, 400, "invalid_uuid", "contentVersionId must be a UUID", map[string]any{})
			return
		}
		var body reviewPreflightRequest
		if decodeBody(r, &body) != nil || body.SourceContentVersionVersion < 1 || len(body.OptionalInstructions) == 0 {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		var instructions *string
		if string(body.OptionalInstructions) != "null" {
			var value string
			if json.Unmarshal(body.OptionalInstructions, &value) != nil {
				writeError(w, r, 400, "validation_error", "invalid optionalInstructions", map[string]any{})
				return
			}
			instructions = &value
		}
		result, err := service.Preflight(r.Context(), versionID, contentitem.ReviewPreflightRequest{
			SourceContentVersionVersion: body.SourceContentVersionVersion,
			OptionalInstructions: instructions, ActorID: requestActorID(r),
		})
		if err != nil {
			realReviewError(w, r, err)
			return
		}
		writeJSON(w, r, 200, result)
	}
}

func realReviewCreateHandler(service *contentitem.RealReviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		versionID, err := uuid.Parse(r.PathValue("contentVersionId"))
		if err != nil {
			writeError(w, r, 400, "invalid_uuid", "contentVersionId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		var body reviewCreateRequest
		if key == "" || len(key) > 128 || decodeBody(r, &body) != nil || strings.TrimSpace(body.PreflightToken) == "" {
			writeError(w, r, 400, "validation_error", "invalid request body or Idempotency-Key", map[string]any{})
			return
		}
		run, replay, err := service.CreateRunWithReplay(r.Context(), versionID, requestActorID(r), body.PreflightToken, key)
		if err != nil {
			realReviewError(w, r, err)
			return
		}
		status := http.StatusCreated
		if replay {
			status = http.StatusOK
		}
		writeJSON(w, r, status, iteration14WorkflowRunResponse(run))
	}
}

func realReviewSummaryHandler(service *contentitem.RealReviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemID, err := uuid.Parse(r.PathValue("contentItemId"))
		if err != nil {
			writeError(w, r, 400, "invalid_uuid", "contentItemId must be a UUID", map[string]any{})
			return
		}
		summary, err := service.Summary(r.Context(), itemID)
		if err != nil {
			realReviewError(w, r, err)
			return
		}
		writeJSON(w, r, 200, summary)
	}
}

func realReviewIssueHandler(service *contentitem.RealReviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviewID, reviewErr := uuid.Parse(r.PathValue("reviewId"))
		issueID, issueErr := uuid.Parse(r.PathValue("issueId"))
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		var body reviewIssueUpdateRequest
		if reviewErr != nil || issueErr != nil {
			writeError(w, r, 400, "invalid_uuid", "reviewId and issueId must be UUIDs", map[string]any{})
			return
		}
		if key == "" || len(key) > 128 || decodeBody(r, &body) != nil || body.ExpectedVersion < 1 {
			writeError(w, r, 400, "validation_error", "invalid request body or Idempotency-Key", map[string]any{})
			return
		}
		issue, err := service.UpdateIssue(r.Context(), reviewID, issueID, contentitem.ReviewIssueUpdateRequest{
			Disposition: body.Disposition, ExpectedVersion: body.ExpectedVersion,
			IdempotencyKey: key, ActorID: requestActorID(r),
		})
		if err != nil {
			realReviewError(w, r, err)
			return
		}
		writeJSON(w, r, 200, issue)
	}
}

func realReviewConsumptionRetryHandler(service *contentitem.RealReviewService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID, err := uuid.Parse(r.PathValue("workflowRunId"))
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		var body reviewConsumptionRetryRequest
		if err != nil {
			writeError(w, r, 400, "invalid_uuid", "workflowRunId must be a UUID", map[string]any{})
			return
		}
		if key == "" || len(key) > 128 || decodeBody(r, &body) != nil || body.ExpectedRunVersion < 1 {
			writeError(w, r, 400, "validation_error", "invalid request body or Idempotency-Key", map[string]any{})
			return
		}
		result, err := service.RetryResultConsumption(r.Context(), runID, contentitem.ReviewResultConsumptionRetryRequest{
			ExpectedRunVersion: body.ExpectedRunVersion, IdempotencyKey: key,
		})
		if err != nil {
			realReviewError(w, r, err)
			return
		}
		writeJSON(w, r, 200, result)
	}
}

func realReviewError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, contentitem.ErrContentVersionNotFound):
		writeError(w, r, 404, "content_version_not_found", "requested review resource was not found", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewNotFound):
		writeError(w, r, 404, "review_not_found", "requested review was not found", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewIssueNotFound):
		writeError(w, r, 404, "review_issue_not_found", "requested review issue was not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotFound):
		writeError(w, r, 404, "workflow_run_not_found", "requested workflow run was not found", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewNotConfigured):
		writeError(w, r, 422, "review_not_configured", "审核工作流尚未配置", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewNotReviewable):
		writeError(w, r, 422, "content_version_not_reviewable", "正文版本不可审核", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewActiveRun):
		writeError(w, r, 409, "active_review_conflict", "该正文版本已有活跃审核任务", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewTokenInvalid):
		writeError(w, r, 422, "preflight_token_invalid", "预检令牌无效", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewTokenExpired):
		writeError(w, r, 422, "preflight_token_expired", "预检令牌已过期", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewTokenConsumed):
		writeError(w, r, 422, "preflight_token_consumed", "预检令牌已使用", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewPreflightChanged):
		writeError(w, r, 409, "preflight_input_changed", "审核预检输入已变化", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewIssueVersion):
		writeError(w, r, 409, "review_issue_version_conflict", "审核问题版本冲突", map[string]any{})
	case errors.Is(err, workflowrun.ErrVersionConflict):
		writeError(w, r, 409, "workflow_run_version_conflict", "工作流运行版本冲突", map[string]any{})
	case errors.Is(err, workflowrun.ErrIdempotencyConflict), errors.Is(err, contentitem.ErrIdempotencyConflict):
		writeError(w, r, 409, "idempotency_key_reused_with_different_payload", "幂等键已用于不同请求", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewResultNotRetryable):
		writeError(w, r, 409, "review_result_not_retryable", "审核结果当前不可重试", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewOutputInvalid):
		writeError(w, r, 409, "output_validation_failed", "审核输出未通过结构校验", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewResultConsumption):
		writeError(w, r, 500, "result_consumption_failed", "审核结果未能安全保存", map[string]any{})
	case errors.Is(err, contentitem.ErrValidation), errors.Is(err, contentitem.ErrInvalidReviewParameters), errors.Is(err, contentitem.ErrInvalidPagination):
		writeError(w, r, 400, "validation_error", "请求无效", map[string]any{})
	default:
		writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
	}
}

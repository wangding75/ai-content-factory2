package httpserver

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func registerIteration07Routes(m *http.ServeMux, a *contentitem.Iteration07Application) {
	m.HandleFunc("POST /api/v1/content-items/{contentItemId}/rewrites/mock", mockRewriteHandler(a))
	m.HandleFunc("GET /api/v1/content-workflow-runs/{workflowRunId}", getWorkflowRunHandler(a))
	m.HandleFunc("GET /api/v1/content-items/{contentItemId}/versions", listVersionsHandler(a))
	m.HandleFunc("GET /api/v1/content-versions/{versionId}", getVersionHandler(a))
	m.HandleFunc("GET /api/v1/projects/{projectId}/works", listWorksHandler(a))
	m.HandleFunc("GET /api/v1/works/{workId}", getWorkHandler(a))
}
func t(v time.Time) string { return v.UTC().Format(time.RFC3339Nano) }
func tp(v *time.Time) any {
	if v == nil {
		return nil
	}
	return t(*v)
}
func version(v contentitem.ContentVersion) map[string]any {
	return map[string]any{"id": v.ID, "content_item_id": v.ContentItemID, "version_no": v.VersionNo, "version": v.Version, "status": v.Status, "source": v.Source, "title": v.Title, "content": v.Content, "summary": v.Summary, "word_count": v.WordCount, "frozen_at": tp(v.FrozenAt), "created_at": t(v.CreatedAt), "updated_at": t(v.UpdatedAt)}
}
func item(v contentitem.ContentItem) map[string]any {
	return map[string]any{"id": v.ID, "chapter_plan_id": v.ChapterPlanID, "title": v.Title, "status": v.Status, "current_version_id": v.CurrentVersionID, "reviewed_at": tp(v.ReviewedAt), "created_at": t(v.CreatedAt), "updated_at": t(v.UpdatedAt)}
}
func itemSummary(v contentitem.ContentItem) map[string]any {
	return map[string]any{"id": v.ID, "title": v.Title, "status": v.Status, "current_version_id": v.CurrentVersionID}
}
func reviewSummary(v *contentitem.ReviewReport) any {
	if v == nil {
		return nil
	}
	return map[string]any{"id": v.ID, "status": v.Status, "conclusion": v.Conclusion, "score": v.Score, "summary": v.Summary, "created_at": t(v.CreatedAt)}
}
func reviewFull(v contentitem.ReviewReport) map[string]any {
	return map[string]any{"id": v.ID, "content_item_id": v.ContentItemID, "content_version_id": v.ContentVersionID, "provider_key": v.ProviderKey, "status": v.Status, "conclusion": v.Conclusion, "score": v.Score, "summary": v.Summary, "created_at": t(v.CreatedAt)}
}
func runSummary(v *contentitem.WorkflowRun) any {
	if v == nil {
		return nil
	}
	return map[string]any{"id": v.ID, "provider_key": v.ProviderKey, "workflow_key": v.WorkflowKey, "status": v.Status, "started_at": t(v.StartedAt), "finished_at": tp(v.FinishedAt)}
}
func versionSource(v *contentitem.ContentVersion) any {
	if v == nil {
		return nil
	}
	return map[string]any{"id": v.ID, "version_no": v.VersionNo, "title": v.Title, "source": v.Source, "frozen_at": tp(v.FrozenAt)}
}
func versionList(v contentitem.VersionRead) map[string]any {
	return map[string]any{"id": v.Version.ID, "content_item_id": v.Version.ContentItemID, "version_no": v.Version.VersionNo, "status": v.Version.Status, "source": v.Version.Source, "title": v.Version.Title, "word_count": v.Version.WordCount, "created_at": t(v.Version.CreatedAt), "frozen_at": tp(v.Version.FrozenAt), "is_current": v.IsCurrent, "source_content_version": versionSource(v.Lineage.SourceContentVersion), "source_review_report": reviewSummary(v.Lineage.SourceReviewReport), "source_workflow_run": runSummary(v.Lineage.RewriteWorkflowRun)}
}
func runDetail(v contentitem.WorkflowRun) map[string]any {
	var input map[string]any
	_ = json.Unmarshal(v.InputJSON, &input)
	focus, _ := input["Focus"].([]any)
	if focus == nil {
		focus, _ = input["focus"].([]any)
	}
	if focus == nil {
		focus = []any{}
	}
	in := map[string]any{"source_version_no": 1, "review_report_id": v.SourceReviewReportID, "parameters": map[string]any{"rewrite_focus": focus, "preserve_ending": input["PreserveEnding"], "instructions": nil}}
	var out any
	if v.Status == "succeeded" {
		var x map[string]any
		_ = json.Unmarshal(v.OutputJSON, &x)
		out = map[string]any{"target_version_no": 2, "target_content_version_id": v.TargetContentVersionID, "word_count": x["word_count"]}
	}
	var er any
	if v.ErrorCode != nil {
		er = map[string]any{"code": *v.ErrorCode, "message": *v.ErrorSummary}
	}
	return map[string]any{"id": v.ID, "project_id": v.ProjectID, "provider_key": v.ProviderKey, "workflow_key": v.WorkflowKey, "status": v.Status, "source_content_item_id": v.ContentItemID, "source_content_version_id": v.ContentVersionID, "target_content_version_id": v.TargetContentVersionID, "source_review_report_id": v.SourceReviewReportID, "input_summary": in, "output_summary": out, "error": er, "idempotency_key": v.IdempotencyKey, "request_fingerprint": v.RequestFingerprint, "started_at": t(v.StartedAt), "finished_at": tp(v.FinishedAt)}
}

type rewriteRequest struct {
	Source     string `json:"source_content_version_id"`
	Review     string `json:"review_report_id"`
	Expected   int    `json:"expected_version"`
	Parameters struct {
		Focus        []string `json:"rewrite_focus"`
		Preserve     bool     `json:"preserve_ending"`
		Instructions *string  `json:"instructions"`
	} `json:"parameters"`
}

func mockRewriteHandler(a *contentitem.Iteration07Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, e := uuid.Parse(r.PathValue("contentItemId"))
		if e != nil {
			writeError(w, r, 400, "invalid_uuid", "contentItemId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" || len(key) > 128 {
			writeError(w, r, 400, "idempotency_key_required", "idempotency key required", map[string]any{})
			return
		}
		var b rewriteRequest
		if e = decodeBody(r, &b); e != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		source, e1 := uuid.Parse(b.Source)
		review, e2 := uuid.Parse(b.Review)
		if e1 != nil || e2 != nil || b.Expected < 1 {
			writeError(w, r, 422, "invalid_rewrite_parameters", "invalid rewrite parameters", map[string]any{})
			return
		}
		out, e := a.Rewrite.Rewrite(r.Context(), contentitem.MockRewriteCommand{ContentItemID: id, SourceContentVersionID: source, ReviewReportID: review, ExpectedVersion: b.Expected, IdempotencyKey: key, BusinessInputID: key, Parameters: contentitem.MockRewriteParameters{RewriteFocus: b.Parameters.Focus, PreserveEnding: b.Parameters.Preserve, Instructions: b.Parameters.Instructions}})
		if e != nil {
			iteration07Error(w, r, e)
			return
		}
		d, e := a.Query.GetItem(r.Context(), id)
		if e != nil {
			iteration07Error(w, r, e)
			return
		}
		rr, e := a.Query.GetRun(r.Context(), out.WorkflowRun.ID)
		if e != nil {
			iteration07Error(w, r, e)
			return
		}
		rv, e := a.Query.GetReview(r.Context(), review)
		if e != nil {
			iteration07Error(w, r, e)
			return
		}
		writeJSON(w, r, 201, map[string]any{"content_item": item(d), "source_content_version": map[string]any{"id": out.SourceContentVersion.ID, "version_no": out.SourceContentVersion.VersionNo, "version": out.SourceContentVersion.Version, "title": out.SourceContentVersion.Title, "word_count": out.SourceContentVersion.WordCount, "source": out.SourceContentVersion.Source, "frozen_at": tp(out.SourceContentVersion.FrozenAt)}, "source_review_report": reviewFull(rv), "target_content_version": version(*out.TargetContentVersion), "workflow_run": runDetail(rr)})
	}
}
func pagination(r *http.Request) (int, int, bool) {
	l, o := 20, 0
	if x := r.URL.Query().Get("limit"); x != "" {
		n, e := strconv.Atoi(x)
		if e != nil || n < 1 || n > 100 {
			return 0, 0, false
		}
		l = n
	}
	if x := r.URL.Query().Get("offset"); x != "" {
		n, e := strconv.Atoi(x)
		if e != nil || n < 0 {
			return 0, 0, false
		}
		o = n
	}
	return l, o, true
}
func listVersionsHandler(a *contentitem.Iteration07Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, e := uuid.Parse(r.PathValue("contentItemId"))
		if e != nil {
			writeError(w, r, 400, "invalid_uuid", "contentItemId must be a UUID", map[string]any{})
			return
		}
		l, o, ok := pagination(r)
		if !ok {
			writeError(w, r, 400, "invalid_pagination", "invalid pagination", map[string]any{})
			return
		}
		x, e := a.Query.ListVersions(r.Context(), id, l, o)
		if e != nil {
			iteration07Error(w, r, e)
			return
		}
		z := make([]map[string]any, len(x.Items))
		for i := range x.Items {
			z[i] = versionList(x.Items[i])
		}
		writeJSON(w, r, 200, map[string]any{"items": z, "total": x.Total, "limit": x.Limit, "offset": x.Offset})
	}
}
func getVersionHandler(a *contentitem.Iteration07Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, e := uuid.Parse(r.PathValue("versionId"))
		if e != nil {
			writeError(w, r, 400, "invalid_uuid", "versionId must be a UUID", map[string]any{})
			return
		}
		x, e := a.Query.GetVersion(r.Context(), id)
		if e != nil {
			iteration07Error(w, r, e)
			return
		}
		writeJSON(w, r, 200, map[string]any{"content_version": version(x.Version), "content_item": itemSummary(x.Item), "source_content_version": versionSource(x.Lineage.SourceContentVersion), "source_review_report": reviewSummary(x.Lineage.SourceReviewReport), "rewrite_workflow_run": runSummary(x.Lineage.RewriteWorkflowRun), "is_current": x.IsCurrent})
	}
}
func getWorkflowRunHandler(a *contentitem.Iteration07Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, e := uuid.Parse(r.PathValue("workflowRunId"))
		if e != nil {
			writeError(w, r, 400, "invalid_uuid", "workflowRunId must be a UUID", map[string]any{})
			return
		}
		x, e := a.Query.GetRun(r.Context(), id)
		if e != nil {
			iteration07Error(w, r, e)
			return
		}
		writeJSON(w, r, 200, runDetail(x))
	}
}
func work(v contentitem.WorkRead) map[string]any {
	nav := map[string]any{"content_item_id": v.Item.ID, "current_version_id": v.Item.CurrentVersionID, "latest_review_report_id": nil, "latest_workflow_run_id": nil, "rewrite_source_version_id": nil, "rewrite_review_report_id": nil, "rewrite_target_version_id": nil}
	if v.LatestReview != nil {
		nav["latest_review_report_id"] = v.LatestReview.ID
	}
	if v.LatestRun != nil {
		nav["latest_workflow_run_id"] = v.LatestRun.ID
	}
	if v.RewriteRun != nil {
		nav["rewrite_source_version_id"] = v.RewriteRun.ContentVersionID
		nav["rewrite_review_report_id"] = v.RewriteRun.SourceReviewReportID
		nav["rewrite_target_version_id"] = v.RewriteRun.TargetContentVersionID
	}
	return map[string]any{"work_id": v.WorkID, "project_id": v.ProjectID, "chapter_plan": map[string]any{"id": v.ChapterID, "chapter_no": v.ChapterNo, "title": v.ChapterTitle, "status": v.ChapterStatus}, "content_item": itemSummary(v.Item), "current_version": versionList(v.Current), "version_count": v.VersionCount, "latest_review": reviewSummary(v.LatestReview), "latest_workflow_run": runSummary(v.LatestRun), "navigation": nav}
}
func listWorksHandler(a *contentitem.Iteration07Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, e := uuid.Parse(r.PathValue("projectId"))
		if e != nil {
			writeError(w, r, 400, "invalid_uuid", "projectId must be a UUID", map[string]any{})
			return
		}
		l, o, ok := pagination(r)
		if !ok {
			writeError(w, r, 400, "invalid_pagination", "invalid pagination", map[string]any{})
			return
		}
		x, total, e := a.Query.ListWorks(r.Context(), id, l, o)
		if e != nil {
			iteration07Error(w, r, e)
			return
		}
		z := make([]map[string]any, len(x))
		for i := range x {
			z[i] = work(x[i])
		}
		writeJSON(w, r, 200, map[string]any{"items": z, "total": total, "limit": l, "offset": o})
	}
}
func getWorkHandler(a *contentitem.Iteration07Application) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, e := uuid.Parse(r.PathValue("workId"))
		if e != nil {
			writeError(w, r, 400, "invalid_uuid", "workId must be a UUID", map[string]any{})
			return
		}
		x, e := a.Query.GetWork(r.Context(), id)
		if e != nil {
			if errors.Is(e, contentitem.ErrContentItemNotFound) {
				writeError(w, r, 404, "work_not_found", "work not found", map[string]any{})
				return
			}
			iteration07Error(w, r, e)
			return
		}
		h, e := a.Query.ListVersions(r.Context(), id, 100, 0)
		if e != nil {
			iteration07Error(w, r, e)
			return
		}
		vs := make([]map[string]any, len(h.Items))
		for i := range h.Items {
			vs[i] = versionList(h.Items[i])
		}
		writeJSON(w, r, 200, map[string]any{"work": work(x), "project": map[string]any{"id": x.ProjectID, "name": x.ProjectName, "status": x.ProjectStatus}, "version_history": vs})
	}
}
func iteration07Error(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, contentitem.ErrProjectNotFound):
		writeError(w, r, 404, "project_not_found", "project not found", map[string]any{})
	case errors.Is(e, contentitem.ErrContentItemNotFound):
		writeError(w, r, 404, "content_item_not_found", "content item not found", map[string]any{})
	case errors.Is(e, contentitem.ErrContentVersionNotFound):
		writeError(w, r, 404, "content_version_not_found", "content version not found", map[string]any{})
	case errors.Is(e, contentitem.ErrWorkflowRunNotFound):
		writeError(w, r, 404, "workflow_run_not_found", "workflow run not found", map[string]any{})
	case errors.Is(e, contentitem.ErrReviewNotFound):
		writeError(w, r, 404, "review_report_not_found", "review report not found", map[string]any{})
	case errors.Is(e, contentitem.ErrContentVersionNotFrozen):
		writeError(w, r, 409, "content_version_not_frozen", "content version not frozen", map[string]any{})
	case errors.Is(e, contentitem.ErrReviewNotCompleted):
		writeError(w, r, 409, "review_not_completed", "review not completed", map[string]any{})
	case errors.Is(e, contentitem.ErrSourceVersionMismatch):
		writeError(w, r, 409, "source_version_mismatch", "source version mismatch", map[string]any{})
	case errors.Is(e, contentitem.ErrVersionConflict):
		writeError(w, r, 409, "version_conflict", "content version conflict", map[string]any{})
	case errors.Is(e, contentitem.ErrRewriteAlreadyExists):
		writeError(w, r, 409, "rewrite_already_exists", "mock rewrite already exists", map[string]any{})
	case errors.Is(e, contentitem.ErrIdempotencyConflict):
		writeError(w, r, 409, "idempotency_key_reused_with_different_payload", "idempotency key reused with different payload", map[string]any{})
	case errors.Is(e, contentitem.ErrCrossProjectRelation):
		writeError(w, r, 409, "cross_project_relation_conflict", "cross-project relation conflict", map[string]any{})
	case errors.Is(e, contentitem.ErrInvalidRewriteParameters):
		writeError(w, r, 422, "invalid_rewrite_parameters", "invalid rewrite parameters", map[string]any{})
	case errors.Is(e, contentitem.ErrMockRewriteFailed):
		writeError(w, r, 500, "mock_rewrite_failed", "mock rewrite failed", map[string]any{})
	default:
		writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
	}
}

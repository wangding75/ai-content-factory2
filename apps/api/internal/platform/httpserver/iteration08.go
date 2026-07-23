package httpserver

import (
	"errors"
	"net/http"

	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
)

func registerIteration08Routes(m *http.ServeMux, s *contentitem.GlobalLiteService) {
	m.HandleFunc("GET /api/v1/works", listGlobalWorksHandler(s))
	m.HandleFunc("GET /api/v1/workflows/builtin", listBuiltinWorkflowsHandler())
	m.HandleFunc("GET /api/v1/content-workflow-runs", listGlobalWorkflowRunsHandler(s))
	m.HandleFunc("GET /api/v1/capabilities", listCapabilitiesHandler())
	m.HandleFunc("GET /api/v1/integrations", listIntegrationsHandler())
}

func globalLiteError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, contentitem.ErrProjectNotFound) {
		writeError(w, r, 404, "project_not_found", "project not found", map[string]any{})
		return
	}
	writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
}
func listGlobalWorksHandler(s *contentitem.GlobalLiteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if scope := r.URL.Query().Get("scope"); scope != "" && scope != "global" {
			writeError(w, r, 400, "invalid_scope", "invalid scope", map[string]any{})
			return
		}
		limit, offset, ok := pagination(r)
		if !ok {
			writeError(w, r, 400, "invalid_pagination", "invalid pagination", map[string]any{})
			return
		}
		x, err := s.ListWorks(r.Context(), limit, offset)
		if err != nil {
			globalLiteError(w, r, err)
			return
		}
		items := make([]map[string]any, len(x.Items))
		for i := range x.Items {
			items[i] = map[string]any{"project": map[string]any{"id": x.Items[i].ProjectID, "name": x.Items[i].ProjectName, "status": x.Items[i].ProjectStatus}, "work": work(x.Items[i])}
		}
		writeJSON(w, r, 200, map[string]any{"items": items, "total": x.Total, "limit": x.Limit, "offset": x.Offset})
	}
}
func listBuiltinWorkflowsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, 200, map[string]any{"items": contentitem.BuiltinWorkflows()})
	}
}
func listCapabilitiesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, 200, map[string]any{"items": contentitem.Capabilities()})
	}
}
func listIntegrationsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, 200, map[string]any{"items": contentitem.Integrations()})
	}
}
func listGlobalWorkflowRunsHandler(s *contentitem.GlobalLiteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, ok := pagination(r)
		if !ok {
			writeError(w, r, 400, "invalid_pagination", "invalid pagination", map[string]any{})
			return
		}
		x, err := s.ListWorkflowRuns(r.Context(), limit, offset)
		if err != nil {
			globalLiteError(w, r, err)
			return
		}
		items := make([]map[string]any, len(x.Items))
		for i, v := range x.Items {
			var project any
			if v.ProjectID != nil {
				project = map[string]any{"id": v.ProjectID, "name": v.ProjectName, "status": v.ProjectStatus}
			}
			var failure any
			if v.Run.ErrorCode != nil && v.Run.ErrorSummary != nil {
				failure = map[string]any{"code": *v.Run.ErrorCode, "message": *v.Run.ErrorSummary}
			}
			items[i] = map[string]any{"id": v.Run.ID, "provider_key": v.Run.ProviderKey, "workflow_key": v.Run.WorkflowKey, "status": v.Run.Status, "project": project, "subject": map[string]any{"type": v.Run.SubjectType, "id": v.Run.SubjectID}, "error": failure, "started_at": t(v.Run.StartedAt), "finished_at": tp(v.Run.FinishedAt)}
		}
		writeJSON(w, r, 200, map[string]any{"items": items, "total": x.Total, "limit": x.Limit, "offset": x.Offset})
	}
}

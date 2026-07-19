package httpserver

import (
	"errors"
	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"net/http"
	"strconv"
	"strings"
)

func registerGlobalConfigurationRoutes(m *http.ServeMux, s *globalconfig.Service) {
	m.HandleFunc("GET /api/v1/llm-provider-types", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, 200, map[string]any{"items": []any{map[string]any{"providerType": "openai_compatible", "displayName": "OpenAI-compatible", "supportsSecret": true, "fieldSchemas": []any{}}}})
	})
	m.HandleFunc("GET /api/v1/workflow-connection-types", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, 200, map[string]any{"items": []any{map[string]any{"connectionType": "n8n", "displayName": "n8n", "authTypes": []string{"api_key"}, "fieldSchemas": []any{}}}})
	})
	m.HandleFunc("GET /api/v1/distribution-platform-types", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, 200, map[string]any{"items": []any{map[string]any{"platformType": "wechat_official_account", "displayName": "WeChat Official Account", "authTypes": []string{"api_key"}, "fieldSchemas": []any{}}, map[string]any{"platformType": "douyin", "displayName": "Douyin", "authTypes": []string{"oauth", "access_token"}, "fieldSchemas": []any{}}, map[string]any{"platformType": "youtube", "displayName": "YouTube", "authTypes": []string{"oauth", "api_key"}, "fieldSchemas": []any{}}, map[string]any{"platformType": "custom", "displayName": "Custom", "authTypes": []string{"api_key", "oauth", "access_token", "custom"}, "fieldSchemas": []any{}}}})
	})
	m.HandleFunc("GET /api/v1/llm-providers", func(w http.ResponseWriter, r *http.Request) {
		o, ok := configurationListOptions(w, r)
		if !ok {
			return
		}
		x, n, e := s.ListProviders(r.Context(), o)
		configurationResult(w, r, x, n, o, e)
	})
	m.HandleFunc("POST /api/v1/llm-providers", func(w http.ResponseWriter, r *http.Request) {
		var x globalconfig.ProviderCreate
		if !configurationBody(w, r, &x) {
			return
		}
		v, e := s.CreateProvider(r.Context(), x, configurationKey(r))
		configurationWrite(w, r, v, e)
	})
	m.HandleFunc("GET /api/v1/llm-providers/{providerId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := configurationID(w, r, "providerId")
		if !ok {
			return
		}
		v, e := s.GetProvider(r.Context(), id)
		configurationRead(w, r, v, e)
	})
	m.HandleFunc("PATCH /api/v1/llm-providers/{providerId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := configurationID(w, r, "providerId")
		if !ok {
			return
		}
		var x globalconfig.ProviderUpdate
		if !configurationBody(w, r, &x) {
			return
		}
		v, e := s.UpdateProvider(r.Context(), id, x)
		configurationRead(w, r, v, e)
	})
	m.HandleFunc("GET /api/v1/workflow-connections", func(w http.ResponseWriter, r *http.Request) {
		o, ok := configurationListOptions(w, r)
		if !ok {
			return
		}
		x, n, e := s.ListConnections(r.Context(), o)
		configurationResult(w, r, x, n, o, e)
	})
	m.HandleFunc("POST /api/v1/workflow-connections", func(w http.ResponseWriter, r *http.Request) {
		var x globalconfig.ConnectionCreate
		if !configurationBody(w, r, &x) {
			return
		}
		v, e := s.CreateConnection(r.Context(), x, configurationKey(r))
		configurationWrite(w, r, v, e)
	})
	m.HandleFunc("GET /api/v1/workflow-connections/{connectionId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := configurationID(w, r, "connectionId")
		if !ok {
			return
		}
		v, e := s.GetConnection(r.Context(), id)
		configurationRead(w, r, v, e)
	})
	m.HandleFunc("PATCH /api/v1/workflow-connections/{connectionId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := configurationID(w, r, "connectionId")
		if !ok {
			return
		}
		var x globalconfig.ConnectionUpdate
		if !configurationBody(w, r, &x) {
			return
		}
		v, e := s.UpdateConnection(r.Context(), id, x)
		configurationRead(w, r, v, e)
	})
	m.HandleFunc("GET /api/v1/workflow-configurations", func(w http.ResponseWriter, r *http.Request) {
		o, ok := configurationListOptions(w, r)
		if !ok {
			return
		}
		x, n, e := s.ListWorkflows(r.Context(), o)
		configurationResult(w, r, x, n, o, e)
	})
	m.HandleFunc("POST /api/v1/workflow-configurations", func(w http.ResponseWriter, r *http.Request) {
		var x globalconfig.WorkflowCreate
		if !configurationBody(w, r, &x) {
			return
		}
		v, e := s.CreateWorkflow(r.Context(), x, configurationKey(r))
		configurationWrite(w, r, v, e)
	})
	m.HandleFunc("GET /api/v1/workflow-configurations/{workflowId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := configurationID(w, r, "workflowId")
		if !ok {
			return
		}
		v, e := s.GetWorkflow(r.Context(), id)
		configurationRead(w, r, v, e)
	})
	m.HandleFunc("PATCH /api/v1/workflow-configurations/{workflowId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := configurationID(w, r, "workflowId")
		if !ok {
			return
		}
		var x globalconfig.WorkflowUpdate
		if !configurationBody(w, r, &x) {
			return
		}
		v, e := s.UpdateWorkflow(r.Context(), id, x)
		configurationRead(w, r, v, e)
	})
	m.HandleFunc("GET /api/v1/distribution-platforms", func(w http.ResponseWriter, r *http.Request) {
		o, ok := configurationListOptions(w, r)
		if !ok {
			return
		}
		x, n, e := s.ListPlatforms(r.Context(), o)
		configurationResult(w, r, x, n, o, e)
	})
	m.HandleFunc("POST /api/v1/distribution-platforms", func(w http.ResponseWriter, r *http.Request) {
		var x globalconfig.PlatformCreate
		if !configurationBody(w, r, &x) {
			return
		}
		v, e := s.CreatePlatform(r.Context(), x, configurationKey(r))
		configurationWrite(w, r, v, e)
	})
	m.HandleFunc("GET /api/v1/distribution-platforms/{platformId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := configurationID(w, r, "platformId")
		if !ok {
			return
		}
		v, e := s.GetPlatform(r.Context(), id)
		configurationRead(w, r, v, e)
	})
	m.HandleFunc("PATCH /api/v1/distribution-platforms/{platformId}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := configurationID(w, r, "platformId")
		if !ok {
			return
		}
		var x globalconfig.PlatformUpdate
		if !configurationBody(w, r, &x) {
			return
		}
		v, e := s.UpdatePlatform(r.Context(), id, x)
		configurationRead(w, r, v, e)
	})
}
func configurationBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if e := decodeBody(r, v); e != nil {
		writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
		return false
	}
	return true
}
func configurationID(w http.ResponseWriter, r *http.Request, n string) (uuid.UUID, bool) {
	id, e := uuid.Parse(r.PathValue(n))
	if e != nil {
		writeError(w, r, 400, "validation_error", "invalid configuration id", map[string]any{})
		return uuid.Nil, false
	}
	return id, true
}
func configurationKey(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("Idempotency-Key"))
}
func configurationListOptions(w http.ResponseWriter, r *http.Request) (globalconfig.ListOptions, bool) {
	o := globalconfig.ListOptions{Query: strings.TrimSpace(r.URL.Query().Get("q")), Type: r.URL.Query().Get("providerType"), ConnectionID: r.URL.Query().Get("connectionId"), Limit: 20}
	if o.Type == "" {
		o.Type = r.URL.Query().Get("connectionType")
	}
	if o.Type == "" {
		o.Type = r.URL.Query().Get("platformType")
	}
	for _, x := range []struct {
		n string
		p *int
	}{{"limit", &o.Limit}, {"offset", &o.Offset}} {
		if v := r.URL.Query().Get(x.n); v != "" {
			n, e := strconv.Atoi(v)
			if e != nil || n < 0 || (x.n == "limit" && (n < 1 || n > 100)) {
				writeError(w, r, 400, "validation_error", "invalid pagination", map[string]any{})
				return o, false
			}
			*x.p = n
		}
	}
	if len(o.Query) > 120 {
		writeError(w, r, 400, "validation_error", "invalid query", map[string]any{})
		return o, false
	}
	return o, true
}
func configurationResult(w http.ResponseWriter, r *http.Request, x any, n int, o globalconfig.ListOptions, e error) {
	if e != nil {
		configurationError(w, r, e)
		return
	}
	writeJSON(w, r, 200, map[string]any{"items": x, "total": n, "limit": o.Limit, "offset": o.Offset})
}
func configurationWrite(w http.ResponseWriter, r *http.Request, x any, e error) {
	if e != nil {
		configurationError(w, r, e)
		return
	}
	writeJSON(w, r, 201, x)
}
func configurationRead(w http.ResponseWriter, r *http.Request, x any, e error) {
	if e != nil {
		configurationError(w, r, e)
		return
	}
	writeJSON(w, r, 200, x)
}
func configurationError(w http.ResponseWriter, r *http.Request, e error) {
	switch {
	case errors.Is(e, globalconfig.ErrNotFound):
		writeError(w, r, 404, "configuration_not_found", "configuration not found", map[string]any{})
	case errors.Is(e, globalconfig.ErrVersionConflict):
		writeError(w, r, 409, "version_conflict", "configuration version conflict", map[string]any{})
	case errors.Is(e, globalconfig.ErrIdempotency):
		writeError(w, r, 409, "idempotency_key_reused_with_different_payload", "idempotency key reused", map[string]any{})
	case errors.Is(e, globalconfig.ErrValidation):
		writeError(w, r, 400, "validation_error", "invalid configuration", map[string]any{})
	default:
		writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
	}
}

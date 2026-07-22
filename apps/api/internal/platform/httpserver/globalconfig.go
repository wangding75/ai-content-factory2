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
		writeJSON(w, r, 200, map[string]any{"items": globalconfig.ProviderTypes()})
	})
	m.HandleFunc("GET /api/v1/workflow-connection-types", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, 200, map[string]any{"items": globalconfig.ConnectionTypes()})
	})
	m.HandleFunc("GET /api/v1/distribution-platform-types", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, 200, map[string]any{"items": globalconfig.PlatformTypes()})
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
		key, ok := configurationPatchKey(w, r)
		if !ok {
			return
		}
		v, e := s.UpdateProviderIdempotent(r.Context(), id, x, key)
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
		key, ok := configurationPatchKey(w, r)
		if !ok {
			return
		}
		v, e := s.UpdateConnectionIdempotent(r.Context(), id, x, key)
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
		key, ok := configurationPatchKey(w, r)
		if !ok {
			return
		}
		v, e := s.UpdateWorkflowIdempotent(r.Context(), id, x, key)
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
		key, ok := configurationPatchKey(w, r)
		if !ok {
			return
		}
		v, e := s.UpdatePlatformIdempotent(r.Context(), id, x, key)
		configurationRead(w, r, v, e)
	})
}
func configurationBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if e := decodeBody(r, v); e != nil {
		writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{"fields": map[string]string{"body": "invalid_json_or_unknown_field"}})
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
func configurationPatchKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := configurationKey(r)
	if key == "" || len(key) > 128 {
		writeError(w, r, 400, "validation_error", "invalid idempotency key", map[string]any{"fields": map[string]string{"Idempotency-Key": "required_or_too_long"}})
		return "", false
	}
	return key, true
}
func configurationListOptions(w http.ResponseWriter, r *http.Request) (globalconfig.ListOptions, bool) {
	o := globalconfig.ListOptions{Query: strings.TrimSpace(r.URL.Query().Get("q")), Type: r.URL.Query().Get("providerType"), ConnectionID: r.URL.Query().Get("connectionId"), IntegrationStatus: r.URL.Query().Get("integrationStatus"), Limit: 20}
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
	if len(o.Query) > 160 {
		writeError(w, r, 400, "validation_error", "invalid query", map[string]any{})
		return o, false
	}
	if o.ConnectionID != "" {
		if _, err := uuid.Parse(o.ConnectionID); err != nil {
			writeError(w, r, 400, "validation_error", "invalid query", map[string]any{"fields": map[string]string{"connectionId": "invalid_uuid"}})
			return o, false
		}
	}
	if o.IntegrationStatus != "" && !globalconfig.ValidIntegrationStatus(o.IntegrationStatus) {
		writeError(w, r, 400, "validation_error", "invalid query", map[string]any{"fields": map[string]string{"integrationStatus": "invalid_enum"}})
		return o, false
	}
	if raw, exists := r.URL.Query()["enabled"]; exists {
		value, err := strconv.ParseBool(raw[0])
		if err != nil {
			writeError(w, r, 400, "validation_error", "invalid query", map[string]any{"fields": map[string]string{"enabled": "invalid_boolean"}})
			return o, false
		}
		o.Enabled = &value
	}
	if o.Type != "" && !globalconfig.ValidType(r.URL.Path, o.Type) {
		writeError(w, r, 400, "validation_error", "invalid query", map[string]any{"fields": map[string]string{"type": "invalid_enum"}})
		return o, false
	}
	o.ApplicableStage = strings.TrimSpace(r.URL.Query().Get("applicableStage"))
	if o.ApplicableStage != "" && !validApplicableStage(o.ApplicableStage) {
		writeError(w, r, 400, "validation_error", "invalid query", map[string]any{"fields": map[string]string{"applicableStage": "invalid_enum"}})
		return o, false
	}
	return o, true
}

func validApplicableStage(v string) bool {
	switch v {
	case "chapter_planning", "content_generation", "review", "rewrite":
		return true
	}
	return false
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
	case errors.Is(e, globalconfig.ErrNameConflict):
		writeError(w, r, 409, "validation_error", "configuration name already exists", map[string]any{"fields": map[string]string{"name": "already_exists"}})
	case errors.Is(e, globalconfig.ErrValidation):
		writeError(w, r, 400, "validation_error", "invalid configuration", map[string]any{"fields": map[string]string{"body": "invalid"}})
	default:
		writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
	}
}

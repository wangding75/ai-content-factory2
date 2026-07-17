package httpserver

import (
	"net/http"
	"strings"
	"testing"

	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
)

func TestIteration08FrozenDescriptorHandlers(t *testing.T) {
	for name, test := range map[string]struct {
		h    http.HandlerFunc
		want string
	}{
		"builtin":      {listBuiltinWorkflowsHandler(), "content_mock_rewrite"},
		"capabilities": {listCapabilitiesHandler(), "not_configured"},
		"integrations": {listIntegrationsHandler(), "not_available"},
	} {
		t.Run(name, func(t *testing.T) {
			w := doRequest(test.h, http.MethodGet, "/api/v1/"+name, "")
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), test.want) || strings.Contains(strings.ToLower(w.Body.String()), "credential") {
				t.Fatalf("response=%s", w.Body.String())
			}
		})
	}
}

func TestIteration08MaterialScopeValidation(t *testing.T) {
	w := doRequest(listMaterialsHandler(&fakeMaterials{}), http.MethodGet, "/api/v1/materials?scope=project", "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("response=%s", w.Body.String())
	}
}

func TestIteration08RoutesRegistered(t *testing.T) {
	server := New(":0", nil, contentitem.NewGlobalLiteService(nil))
	for _, path := range []string{"/api/v1/workflows/builtin", "/api/v1/capabilities", "/api/v1/integrations", "/api/v1/works?limit=0", "/api/v1/workflow-runs?limit=0"} {
		w := doRequest(server.httpServer.Handler, http.MethodGet, path, "")
		if w.Code == http.StatusNotFound {
			t.Fatalf("route not registered: %s", path)
		}
	}
}

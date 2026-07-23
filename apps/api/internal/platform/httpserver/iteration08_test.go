package httpserver

import (
	"net/http"
	"strings"
	"testing"

	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

func TestIteration08FrozenDescriptorHandlers(t *testing.T) {
	for name, test := range map[string]struct {
		h    http.HandlerFunc
		want string
	}{
		"builtin":      {listBuiltinWorkflowsHandler(), "workflow_key"},
		"capabilities": {listCapabilitiesHandler(), "workflow_keys"},
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
	for _, path := range []string{"/api/v1/workflows/builtin", "/api/v1/capabilities", "/api/v1/integrations", "/api/v1/works?limit=0", "/api/v1/content-workflow-runs?limit=0"} {
		w := doRequest(server.httpServer.Handler, http.MethodGet, path, "")
		if w.Code == http.StatusNotFound {
			t.Fatalf("route not registered: %s", path)
		}
	}
}

func TestWorkflowRunRouteMigrationRegistersLegacyAndRuntimeHandlersTogether(t *testing.T) {
	run := workflowRunHTTPFixture()
	runtime := &fakeWorkflowRunApplication{run: run, listRuns: workflowrun.RunList{Items: []workflowrun.WorkflowRun{run}, Total: 1, Limit: 50}}
	server := New(":0", nil, contentitem.NewIteration07Application(nil, nil), contentitem.NewGlobalLiteService(nil), runtime)

	runtimeList := doRequest(server.httpServer.Handler, http.MethodGet, "/api/v1/workflow-runs?limit=50", "")
	if runtimeList.Code != http.StatusOK || !strings.Contains(runtimeList.Body.String(), `"runNumber"`) || strings.Contains(runtimeList.Body.String(), `"provider_key"`) {
		t.Fatalf("runtime list did not own /workflow-runs: %d %s", runtimeList.Code, runtimeList.Body.String())
	}
	runtimeDetail := doRequest(server.httpServer.Handler, http.MethodGet, "/api/v1/workflow-runs/"+run.ID.String(), "")
	if runtimeDetail.Code != http.StatusOK || !strings.Contains(runtimeDetail.Body.String(), `"triggerSource"`) || strings.Contains(runtimeDetail.Body.String(), `"workflow_key"`) {
		t.Fatalf("runtime detail did not own /workflow-runs/{runId}: %d %s", runtimeDetail.Code, runtimeDetail.Body.String())
	}
	legacyList := doRequest(server.httpServer.Handler, http.MethodGet, "/api/v1/content-workflow-runs?limit=0", "")
	if legacyList.Code == http.StatusNotFound {
		t.Fatalf("legacy list route is not registered")
	}
	legacyDetail := doRequest(server.httpServer.Handler, http.MethodGet, "/api/v1/content-workflow-runs/not-a-uuid", "")
	if legacyDetail.Code != http.StatusBadRequest || !strings.Contains(legacyDetail.Body.String(), "workflowRunId") {
		t.Fatalf("legacy detail did not retain its handler identity: %d %s", legacyDetail.Code, legacyDetail.Body.String())
	}
}

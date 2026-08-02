package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
)

func TestIteration07RewriteAlreadyExistsErrorMapping(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/content-items/x/rewrites/mock", nil)
	withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		iteration07Error(w, r, contentitem.ErrRewriteAlreadyExists)
	})).ServeHTTP(w, r)
	if w.Code != 409 || !strings.Contains(w.Body.String(), `"code":"rewrite_already_exists"`) || strings.Contains(w.Body.String(), "content_versions_item_version_no_unique") {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestIteration07RegistersReadRoutesWithoutMockRewrite(t *testing.T) {
	mux := http.NewServeMux()
	registerIteration07Routes(mux, contentitem.NewIteration07Application(nil, nil))

	for _, path := range []string{
		"/api/v1/content-workflow-runs/not-a-uuid",
		"/api/v1/content-items/not-a-uuid/versions",
		"/api/v1/content-versions/not-a-uuid",
		"/api/v1/projects/not-a-uuid/works",
		"/api/v1/works/not-a-uuid",
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("GET %s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/content-items/not-a-uuid/rewrites/mock", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("mock rewrite route status=%d body=%s", response.Code, response.Body.String())
	}
}

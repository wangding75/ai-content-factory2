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

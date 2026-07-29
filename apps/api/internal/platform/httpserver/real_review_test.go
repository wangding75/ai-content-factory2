package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
)

func TestRealReviewCommandHandlersRejectInvalidTransportInput(t *testing.T) {
	service := &contentitem.RealReviewService{}
	mux := http.NewServeMux()
	registerRealReviewRoutes(mux, service)
	versionID, reviewID, issueID, runID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	cases := []struct {
		name, method, path, body string
		headers                  map[string]string
	}{
		{"preflight requires optionalInstructions", "POST", "/api/v1/content-versions/" + versionID + "/review-runs/preflight", `{"sourceContentVersionVersion":1}`, nil},
		{"preflight rejects unknown field", "POST", "/api/v1/content-versions/" + versionID + "/review-runs/preflight", `{"sourceContentVersionVersion":1,"optionalInstructions":null,"stage":"review"}`, nil},
		{"create requires idempotency key", "POST", "/api/v1/content-versions/" + versionID + "/review-runs", `{"preflightToken":"token"}`, nil},
		{"issue requires idempotency key", "PATCH", "/api/v1/reviews/" + reviewID + "/issues/" + issueID, `{"disposition":"ignored","expectedVersion":1}`, nil},
		{"consumption retry requires key", "POST", "/api/v1/workflow-runs/" + runID + "/review-result-consumption-retries", `{"expectedRunVersion":2}`, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			for key, value := range test.headers {
				request.Header.Set(key, value)
			}
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"validation_error"`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRealReviewRoutesRejectInvalidUUIDsWithoutCallingService(t *testing.T) {
	service := &contentitem.RealReviewService{}
	mux := http.NewServeMux()
	registerRealReviewRoutes(mux, service)
	for _, request := range []*http.Request{
		httptest.NewRequest("GET", "/api/v1/content-items/not-a-uuid/review-summary", nil),
		httptest.NewRequest("POST", "/api/v1/content-versions/not-a-uuid/review-runs/preflight", strings.NewReader(`{}`)),
		httptest.NewRequest("PATCH", "/api/v1/reviews/not-a-uuid/issues/not-a-uuid", strings.NewReader(`{}`)),
		httptest.NewRequest("POST", "/api/v1/workflow-runs/not-a-uuid/review-result-consumption-retries", strings.NewReader(`{}`)),
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_uuid"`) {
			t.Fatalf("%s status=%d body=%s", request.URL.Path, response.Code, response.Body.String())
		}
	}
}

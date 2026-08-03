package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	withRequestID(http.HandlerFunc(healthHandler)).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	if recorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header")
	}
}

func TestMetaHandlerReportsActualDisabledTestRuntime(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	request := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
	recorder := httptest.NewRecorder()
	withRequestID(http.HandlerFunc(metaHandler)).ServeHTTP(recorder, request)
	var body struct { Data map[string]any `json:"data"` }
	if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &body) != nil || body.Data["workflow_provider"] != "mock" || body.Data["externalWorkflowEnabled"] != false || body.Data["workerEnabled"] != false || body.Data["environment"] != "test" {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type rewriteApplicationStub struct {
	availability contentitem.RewriteAvailability
	preflight    contentitem.RewritePreflightResult
	run          workflowrun.WorkflowRun
	replay       bool
	err          error
	request      contentitem.RewritePreflightRequest
}

func (s *rewriteApplicationStub) Availability(context.Context, uuid.UUID) (contentitem.RewriteAvailability, error) {
	return s.availability, s.err
}

func (s *rewriteApplicationStub) Preflight(_ context.Context, _ uuid.UUID, request contentitem.RewritePreflightRequest) (contentitem.RewritePreflightResult, error) {
	s.request = request
	return s.preflight, s.err
}

func (s *rewriteApplicationStub) CreateRun(context.Context, uuid.UUID, string, string, string) (workflowrun.WorkflowRun, bool, error) {
	return s.run, s.replay, s.err
}

func TestRealRewriteRoutesAndFrozenTransportContract(t *testing.T) {
	reviewID, issueID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	subjectType := "review_report"
	stub := &rewriteApplicationStub{
		availability: contentitem.RewriteAvailability{ReviewReportID: reviewID, ContentItemID: uuid.New(), Available: true},
		preflight: contentitem.RewritePreflightResult{Status: "passed"},
		run: workflowrun.WorkflowRun{
			ID: uuid.New(), RunNumber: "WR-REWRITE", ProjectID: uuid.New(), Stage: "rewrite",
			WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: workflowrun.StatusQueued,
			SubjectType: &subjectType, SubjectID: &reviewID, ConfigurationSnapshot: []byte(`{"projectId":"00000000-0000-4000-8000-000000000001","stage":"rewrite","binding":{"id":"00000000-0000-4000-8000-000000000002","version":1},"workflowConfiguration":{"id":"00000000-0000-4000-8000-000000000003","version":1,"inputContractVersion":"rewrite.input.v1","outputContractVersion":"rewrite.output.v1","typeConfig":{"referenceValue":"internal-webhook"}},"workflowConnection":{"id":"00000000-0000-4000-8000-000000000004","version":1,"type":"n8n","baseUrl":"http://internal.example"},"createdAt":"2026-01-01T00:00:00Z"}`),
			InputPayload: []byte(`{"schemaVersion":"rewrite.input.v1"}`), CreatedAt: now, UpdatedAt: now, Version: 1,
		},
	}
	mux := http.NewServeMux()
	registerRealRewriteRoutes(mux, stub)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/"+reviewID.String()+"/rewrite-availability", nil)
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"available":true`) {
		t.Fatalf("availability status=%d body=%s", response.Code, response.Body.String())
	}
	body := `{"selectedIssueIds":["`+issueID.String()+`"],"optionalInstructions":null,"rewriteOptions":{"strategy":"targeted_fix"}}`
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+reviewID.String()+"/rewrites/preflight", strings.NewReader(body))
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK || stub.request.ActorID != "system" ||
		len(stub.request.SelectedIssueIDs) != 1 || stub.request.SelectedIssueIDs[0] != issueID {
		t.Fatalf("preflight status=%d request=%+v body=%s", response.Code, stub.request, response.Body.String())
	}
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+reviewID.String()+"/rewrites", strings.NewReader(`{"preflightToken":"token"}`))
	request.Header.Set("Idempotency-Key", "rewrite-create")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"stage":"rewrite"`) ||
		strings.Contains(response.Body.String(), "internal.example") || strings.Contains(response.Body.String(), "internal-webhook") {
		t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	stub.replay = true
	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+reviewID.String()+"/rewrites", strings.NewReader(`{"preflightToken":"token"}`))
	request.Header.Set("Idempotency-Key", "rewrite-create")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK { t.Fatalf("replay status=%d body=%s", response.Code, response.Body.String()) }
}

func TestRealRewriteHandlersRejectUnknownMissingAndMalformedInput(t *testing.T) {
	stub := &rewriteApplicationStub{}
	mux := http.NewServeMux()
	registerRealRewriteRoutes(mux, stub)
	reviewID := uuid.NewString()
	cases := []struct {
		method string
		path   string
		body   string
		key    string
	}{
		{http.MethodGet, "/api/v1/reviews/not-a-uuid/rewrite-availability", "", ""},
		{http.MethodPost, "/api/v1/reviews/"+reviewID+"/rewrites/preflight", `{"selectedIssueIds":[],"optionalInstructions":null,"rewriteOptions":{"strategy":"targeted_fix"},"stage":"rewrite"}`, ""},
		{http.MethodPost, "/api/v1/reviews/"+reviewID+"/rewrites/preflight", `{"selectedIssueIds":["not-a-uuid"],"optionalInstructions":null,"rewriteOptions":{"strategy":"targeted_fix"}}`, ""},
		{http.MethodPost, "/api/v1/reviews/"+reviewID+"/rewrites", `{"preflightToken":"token"}`, ""},
		{http.MethodPost, "/api/v1/reviews/"+reviewID+"/rewrites", `{"preflightToken":"token","sourceContent":"client"}`, "key"},
	}
	for _, test := range cases {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		if test.key != "" { request.Header.Set("Idempotency-Key", test.key) }
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"validation_error"`) {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}

func TestRealRewriteErrorMappingIsPreciseAndSafe(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{contentitem.ErrReviewNotFound, 404, "review_not_found"},
		{contentitem.ErrReviewIssueNotFound, 404, "review_issue_not_found"},
		{contentitem.ErrRewriteNotAvailable, 422, "rewrite_not_available"},
		{contentitem.ErrRewriteNotConfigured, 422, "rewrite_not_configured"},
		{contentitem.ErrRewritePreflightExpired, 422, "rewrite_preflight_expired"},
		{contentitem.ErrRewritePreflightStale, 409, "rewrite_preflight_stale"},
		{contentitem.ErrRewritePreflightConsumed, 409, "rewrite_preflight_consumed"},
		{contentitem.ErrRewriteActiveRun, 409, "active_rewrite_run_conflict"},
		{workflowrun.ErrIdempotencyConflict, 409, "idempotency_conflict"},
		{errors.New("sql postgres webhook stack secret"), 500, "internal_error"},
	}
	for _, test := range cases {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/rewrite", nil)
		realRewriteError(response, request, test.err)
		lower := strings.ToLower(response.Body.String())
		if response.Code != test.status || !strings.Contains(lower, `"code":"`+test.code+`"`) {
			t.Fatalf("%s status=%d body=%s", test.code, response.Code, response.Body.String())
		}
		for _, forbidden := range []string{"sql", "postgres", "webhook", "stack", "secret"} {
			if strings.Contains(lower, forbidden) { t.Fatalf("%s leaked in %s", forbidden, response.Body.String()) }
		}
	}
}

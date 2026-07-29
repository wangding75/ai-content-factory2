package httpserver

import (
	"context"
	"encoding/json"
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
	summary      contentitem.RewriteSummary
	history      contentitem.RewriteHistory
	result       contentitem.RewriteResult
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

func (s *rewriteApplicationStub) Summary(context.Context, uuid.UUID) (contentitem.RewriteSummary, error) {
	return s.summary, s.err
}

func (s *rewriteApplicationStub) History(context.Context, uuid.UUID, int, int) (contentitem.RewriteHistory, error) {
	return s.history, s.err
}

func (s *rewriteApplicationStub) Result(context.Context, uuid.UUID) (contentitem.RewriteResult, error) {
	return s.result, s.err
}

func (s *rewriteApplicationStub) RetryResultConsumption(context.Context, uuid.UUID, contentitem.RetryRewriteConsumptionRequest) (contentitem.RewriteResult, error) {
	return s.result, s.err
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
		{contentitem.ErrRewriteCandidateNotFound, 404, "rewrite_candidate_not_found"},
		{contentitem.ErrRewriteCandidateNotReady, 409, "rewrite_candidate_not_ready"},
		{contentitem.ErrRewriteOutputInvalid, 409, "rewrite_output_validation_failed"},
		{contentitem.ErrRewriteResultConsumption, 500, "rewrite_result_consumption_failed"},
		{workflowrun.ErrVersionConflict, 409, "workflow_run_version_conflict"},
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

func TestRealRewriteSummaryHistoryResultAndConsumptionRetryRoutes(t *testing.T) {
	now := time.Now().UTC()
	reviewID, itemID, runID, sourceID, candidateID, issueID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	subjectType := "review_report"
	run := workflowrun.WorkflowRun{
		ID: runID, RunNumber: "WR-QUERY", ProjectID: uuid.New(), Stage: "rewrite",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: workflowrun.StatusSucceeded,
		SubjectType: &subjectType, SubjectID: &reviewID,
		ConfigurationSnapshot: json.RawMessage(`{"baseUrl":"http://internal.example","credential":"secret"}`),
		InputPayload: json.RawMessage(`{"sourceContent":"private full source","token":"secret"}`),
		OutputPayload: json.RawMessage(`{"content":"private runtime output"}`),
		ErrorDetails: json.RawMessage(`{"stack":"hidden"}`), CreatedAt: now, UpdatedAt: now, Version: 2,
	}
	source := contentitem.RewriteSourceVersionSummary{
		ID: sourceID, ContentItemID: itemID, VersionNo: 1, Version: 2,
		Title: "来源", WordCount: 4, ContentHash: strings.Repeat("a", 64),
	}
	report := contentitem.RewriteReportSnapshot{
		ReviewReportID: reviewID, SourceContentVersionID: sourceID,
		SourceContentVersionVersion: 2, SourceContentHash: strings.Repeat("a", 64),
		Conclusion: "needs_changes", Summary: "摘要", CompletedAt: now,
	}
	issues := contentitem.RewriteSelectedIssueSummary{
		Total: 1, Items: []contentitem.RewriteIssueSnapshot{{
			ReviewIssueID: issueID, ReviewReportID: reviewID, IssueKey: "ISSUE-1",
			Position: 1, Version: 1, CategoryKey: "logic", CategoryLabel: "逻辑",
			Severity: "warning", Title: "问题", Description: "描述",
			Disposition: "open",
		}},
	}
	candidate := contentitem.ContentVersion{
		ID: candidateID, ContentItemID: itemID, VersionNo: 2, Version: 1,
		Title: "候选", Content: "候选完整正文", WordCount: 6,
		Source: contentitem.ContentVersionSourceWorkflowRewrite,
		Status: contentitem.ContentVersionStatusEditableDraft,
		CreatedAt: now, UpdatedAt: now,
	}
	stub := &rewriteApplicationStub{
		summary: contentitem.RewriteSummary{
			ReviewReportID: reviewID, ContentItemID: itemID, State: "candidate_ready",
			LatestRun: &run, SourceContentVersionSummary: source,
			SelectedIssueSummary: &issues, CandidateVersion: &candidate, CanSetCurrent: true,
		},
		history: contentitem.RewriteHistory{
			Items: []contentitem.RewriteHistoryItem{{
				ReviewReportSnapshot: report, SourceContentVersionSummary: source,
				WorkflowRun: run, State: "candidate_ready", CandidateVersion: &candidate,
			}},
			Total: 1, Limit: 20, Offset: 0,
		},
		result: contentitem.RewriteResult{
			ReviewReportSnapshot: report, SourceContentVersionSummary: source,
			SelectedIssueSummary: issues, WorkflowRun: run,
			Output: contentitem.RewriteRuntimeOutputV1{
				SchemaVersion: "rewrite.output.v1", Title: "候选", Content: "候选完整正文",
				Summary: "摘要", AddressedIssues: []contentitem.RewriteIssueOutcomeV1{{
					ReviewIssueID: issueID, Summary: "已处理",
				}}, UnresolvedIssues: []contentitem.RewriteIssueOutcomeV1{}, Warnings: []string{},
			},
			CandidateVersion: candidate, CanSetCurrent: true,
		},
	}
	mux := http.NewServeMux()
	registerRealRewriteRoutes(mux, stub)
	cases := []struct {
		method, path, body, key string
		status                  int
		contains                string
		forbidden               []string
	}{
		{http.MethodGet, "/api/v1/reviews/" + reviewID.String() + "/rewrite-summary", "", "", 200, `"state":"candidate_ready"`, []string{"private full source", "private runtime output", "internal.example", "credential"}},
		{http.MethodGet, "/api/v1/content-items/" + itemID.String() + "/rewrite-history?limit=20&offset=0", "", "", 200, `"total":1`, []string{"候选完整正文", "private full source", "private runtime output", "hidden"}},
		{http.MethodGet, "/api/v1/workflow-runs/" + runID.String() + "/rewrite-result", "", "", 200, `"content":"候选完整正文"`, []string{"private full source", "private runtime output", "internal.example", "hidden"}},
		{http.MethodPost, "/api/v1/workflow-runs/" + runID.String() + "/rewrite-result-consumption-retries", `{"expectedRunVersion":2}`, "retry-key", 200, `"candidateVersion"`, []string{"private full source", "private runtime output", "internal.example", "hidden"}},
	}
	for _, test := range cases {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
		if test.key != "" {
			request.Header.Set("Idempotency-Key", test.key)
		}
		mux.ServeHTTP(response, request)
		if response.Code != test.status || !strings.Contains(response.Body.String(), test.contains) {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
		for _, forbidden := range test.forbidden {
			if strings.Contains(response.Body.String(), forbidden) {
				t.Fatalf("%s leaked %q: %s", test.path, forbidden, response.Body.String())
			}
		}
	}
}

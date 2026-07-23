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
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type fakeWorkflowRunApplication struct {
	createCommand workflowrun.CreateRunCommand
	listQuery workflowrun.ListRunsQuery
	cancelCommand workflowrun.RunCommand
	retryCommand workflowrun.RetryCommand
	createRun workflowrun.WorkflowRun
	listRuns workflowrun.RunList
	run workflowrun.WorkflowRun
	events []workflowrun.Event
	summary workflowrun.Summary
	createErr, listErr, getErr, eventsErr, cancelErr, retryErr, summaryErr error
}

func (f *fakeWorkflowRunApplication) CreateRun(_ context.Context, command workflowrun.CreateRunCommand) (workflowrun.WorkflowRun, error) { f.createCommand = command; return f.createRun, f.createErr }
func (f *fakeWorkflowRunApplication) ListRuns(_ context.Context, query workflowrun.ListRunsQuery) (workflowrun.RunList, error) { f.listQuery = query; return f.listRuns, f.listErr }
func (f *fakeWorkflowRunApplication) GetRun(context.Context, uuid.UUID) (workflowrun.WorkflowRun, error) { return f.run, f.getErr }
func (f *fakeWorkflowRunApplication) ListRunEvents(context.Context, uuid.UUID) ([]workflowrun.Event, error) { return f.events, f.eventsErr }
func (f *fakeWorkflowRunApplication) CancelRun(_ context.Context, command workflowrun.RunCommand) (workflowrun.WorkflowRun, error) { f.cancelCommand = command; return f.run, f.cancelErr }
func (f *fakeWorkflowRunApplication) RetryRun(_ context.Context, command workflowrun.RetryCommand) (workflowrun.WorkflowRun, error) { f.retryCommand = command; return f.run, f.retryErr }
func (f *fakeWorkflowRunApplication) GetProjectRunSummary(context.Context, uuid.UUID) (workflowrun.Summary, error) { return f.summary, f.summaryErr }

func workflowRunHTTPHandler(app workflowRunApplication) http.Handler {
	mux := http.NewServeMux()
	registerWorkflowRunRoutes(mux, app)
	return withRequestID(mux)
}

func workflowRunHTTPFixture() workflowrun.WorkflowRun {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	return workflowrun.WorkflowRun{ID: uuid.New(), RunNumber: "RUN-14-001", ProjectID: uuid.New(), WorkflowConfigurationID: uuid.New(), Stage: "review", TriggerSource: "manual", Status: workflowrun.StatusQueued, ConfigurationSnapshot: json.RawMessage(`{"token":"hidden","workflow":"safe"}`), InputPayload: json.RawMessage(`{"token":"hidden","topic":"safe"}`), CreatedAt: now, UpdatedAt: now, Version: 1}
}

func workflowRunHTTPRequest(handler http.Handler, method, path, body, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if key != "" { req.Header.Set("Idempotency-Key", key) }
	w := httptest.NewRecorder(); handler.ServeHTTP(w, req); return w
}

func TestWorkflowRunHTTPCreateMapsRequestAndRedactsResponse(t *testing.T) {
	run := workflowRunHTTPFixture()
	app := &fakeWorkflowRunApplication{createRun: run}
	handler := workflowRunHTTPHandler(app)
	w := workflowRunHTTPRequest(handler, http.MethodPost, "/api/v1/workflow-runs", `{"projectId":"`+run.ProjectID.String()+`","stage":"review","inputPayload":{"token":"value","title":"ok"}}`, " create-key ")
	if w.Code != http.StatusCreated { t.Fatalf("status = %d: %s", w.Code, w.Body.String()) }
	if app.createCommand.ProjectID != run.ProjectID || app.createCommand.TriggerSource != "manual" || app.createCommand.IdempotencyKey != "create-key" || !strings.Contains(string(app.createCommand.InputPayload), "title") { t.Fatalf("unexpected command: %#v", app.createCommand) }
	var body struct { Data struct { ConfigurationSnapshot map[string]any `json:"configurationSnapshot"`; InputPayload map[string]any `json:"inputPayload"` } `json:"data"`; RequestID string `json:"request_id"` }
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil { t.Fatal(err) }
	if body.RequestID == "" || body.Data.ConfigurationSnapshot["token"] != "[REDACTED]" || body.Data.InputPayload["token"] != "[REDACTED]" { t.Fatalf("unsafe or incomplete response: %s", w.Body.String()) }
	for _, tc := range []struct{ body, key string }{{`{}`, "key"}, {`{"projectId":"`+run.ProjectID.String()+`","stage":"review","inputPayload":{}}`, ""}} {
		w = workflowRunHTTPRequest(handler, http.MethodPost, "/api/v1/workflow-runs", tc.body, tc.key)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"request_id"`) { t.Fatalf("invalid create = %d: %s", w.Code, w.Body.String()) }
	}
}

func TestWorkflowRunHTTPListParsesFrozenFilters(t *testing.T) {
	run := workflowRunHTTPFixture(); app := &fakeWorkflowRunApplication{listRuns: workflowrun.RunList{Items: []workflowrun.WorkflowRun{run}, Total: 1, Limit: 3, Offset: 2}}
	path := "/api/v1/workflow-runs?projectId="+run.ProjectID.String()+"&stage=review&workflowConfigurationId="+run.WorkflowConfigurationID.String()+"&status=queued&triggerSource=manual&runNumber=RUN-14-001&q=14-0&startTime=2026-07-01T00:00:00Z&endTime=2026-07-23T00:00:00Z&limit=3&offset=2"
	w := workflowRunHTTPRequest(workflowRunHTTPHandler(app), http.MethodGet, path, "", "")
	if w.Code != http.StatusOK || app.listQuery.ProjectID == nil || app.listQuery.RunNumber != "RUN-14-001" || app.listQuery.Query != "14-0" || app.listQuery.StartTime == nil || app.listQuery.Limit != 3 || app.listQuery.Offset != 2 { t.Fatalf("list mapping failed: %d %#v", w.Code, app.listQuery) }
	w = workflowRunHTTPRequest(workflowRunHTTPHandler(app), http.MethodGet, "/api/v1/workflow-runs?startTime=2026-07-24T00:00:00Z&endTime=2026-07-23T00:00:00Z", "", "")
	if w.Code != http.StatusBadRequest { t.Fatalf("invalid time range = %d", w.Code) }
}

func TestWorkflowRunHTTPListAcceptsFrozenTriggerSources(t *testing.T) {
	app := &fakeWorkflowRunApplication{}
	handler := workflowRunHTTPHandler(app)
	for _, source := range []string{"manual", "system", "api", "retry"} {
		w := workflowRunHTTPRequest(handler, http.MethodGet, "/api/v1/workflow-runs?triggerSource="+source, "", "")
		if w.Code != http.StatusOK || app.listQuery.TriggerSource != source { t.Fatalf("%s = %d %#v", source, w.Code, app.listQuery) }
	}
	w := workflowRunHTTPRequest(handler, http.MethodGet, "/api/v1/workflow-runs", "", "")
	if w.Code != http.StatusOK || app.listQuery.TriggerSource != "" { t.Fatalf("empty filter = %d %#v", w.Code, app.listQuery) }
	for _, source := range []string{"project", "workflow_center", "other"} {
		w = workflowRunHTTPRequest(handler, http.MethodGet, "/api/v1/workflow-runs?triggerSource="+source, "", "")
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "validation_error") { t.Fatalf("%s = %d %s", source, w.Code, w.Body.String()) }
	}
}

func TestWorkflowRunHTTPCommandsAndErrors(t *testing.T) {
	run := workflowRunHTTPFixture(); app := &fakeWorkflowRunApplication{run: run}; handler := workflowRunHTTPHandler(app)
	w := workflowRunHTTPRequest(handler, http.MethodPost, "/api/v1/workflow-runs/"+run.ID.String()+"/cancel", `{"expectedVersion":1}`, "cancel-key")
	if w.Code != http.StatusOK || app.cancelCommand.RunID != run.ID || app.cancelCommand.ExpectedVersion != 1 || app.cancelCommand.IdempotencyKey != "cancel-key" { t.Fatalf("cancel mapping failed: %d %#v", w.Code, app.cancelCommand) }
	w = workflowRunHTTPRequest(handler, http.MethodPost, "/api/v1/workflow-runs/"+run.ID.String()+"/retries", `{"expectedVersion":1,"useCurrentConfiguration":true,"inputOverride":{"topic":"new"}}`, "retry-key")
	if w.Code != http.StatusCreated || !app.retryCommand.UseCurrentConfiguration || !strings.Contains(string(app.retryCommand.InputOverride), "new") { t.Fatalf("retry mapping failed: %d %#v", w.Code, app.retryCommand) }
	app.cancelErr = workflowrun.ErrVersionConflict
	w = workflowRunHTTPRequest(handler, http.MethodPost, "/api/v1/workflow-runs/"+run.ID.String()+"/cancel", `{"expectedVersion":1}`, "cancel-key-2")
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), `"code":"version_conflict"`) || strings.Contains(w.Body.String(), "workflow run version conflict") && strings.Contains(w.Body.String(), "stack") { t.Fatalf("version error = %d: %s", w.Code, w.Body.String()) }
	app.cancelErr = workflowrun.ErrIdempotencyConflict
	w = workflowRunHTTPRequest(handler, http.MethodPost, "/api/v1/workflow-runs/"+run.ID.String()+"/cancel", `{"expectedVersion":1}`, "cancel-key-3")
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "idempotency_key_reused_with_different_payload") { t.Fatalf("idempotency error = %d: %s", w.Code, w.Body.String()) }
}

func TestWorkflowRunHTTPDetailsEventsAndSummary(t *testing.T) {
	run := workflowRunHTTPFixture(); event := workflowrun.Event{ID: uuid.New(), RunID: run.ID, EventType: "queued", Status: workflowrun.StatusQueued, Payload: json.RawMessage(`{"cookie":"hidden","safe":true}`), CreatedAt: run.CreatedAt}
	app := &fakeWorkflowRunApplication{run: run, events: []workflowrun.Event{event}, summary: workflowrun.Summary{TotalRuns: 1, ActiveRuns: 1, RecentRuns: []workflowrun.WorkflowRun{run}, LastRunAt: &run.CreatedAt}}
	handler := workflowRunHTTPHandler(app)
	for _, path := range []string{"/api/v1/workflow-runs/"+run.ID.String(), "/api/v1/workflow-runs/"+run.ID.String()+"/events", "/api/v1/projects/"+run.ProjectID.String()+"/workflow-run-summary"} {
		w := workflowRunHTTPRequest(handler, http.MethodGet, path, "", "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"request_id"`) || strings.Contains(w.Body.String(), "hidden") { t.Fatalf("GET %s = %d: %s", path, w.Code, w.Body.String()) }
	}
	app.getErr = workflowrun.ErrNotFound
	w := workflowRunHTTPRequest(handler, http.MethodGet, "/api/v1/workflow-runs/"+run.ID.String(), "", "")
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "workflow_run_not_found") { t.Fatalf("not found = %d: %s", w.Code, w.Body.String()) }
	app.getErr = errors.New("token=unsafe internal database")
	w = workflowRunHTTPRequest(handler, http.MethodGet, "/api/v1/workflow-runs/"+run.ID.String(), "", "")
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "unsafe") { t.Fatalf("internal error leaked: %d: %s", w.Code, w.Body.String()) }
}

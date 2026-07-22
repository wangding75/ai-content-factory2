package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
)

// ── Fake BindingService for handler unit tests (no DB) ──────────────────────

type fakeBindingService struct {
	listFn    func(ctx context.Context, projectID uuid.UUID) ([]workflowbinding.StageRead, error)
	putFn     func(ctx context.Context, projectID uuid.UUID, stage workflowbinding.WorkflowBindingStage, req workflowbinding.PutRequest, key string) (workflowbinding.PutResult, int, error)
	deleteFn  func(ctx context.Context, projectID uuid.UUID, stage workflowbinding.WorkflowBindingStage, req workflowbinding.DeleteRequest, key string) (workflowbinding.UnbindResult, int, error)
	listCalls int
}

func (f *fakeBindingService) ListStages(ctx context.Context, projectID uuid.UUID) ([]workflowbinding.StageRead, error) {
	f.listCalls++
	if f.listFn != nil {
		return f.listFn(ctx, projectID)
	}
	return nil, nil
}

func (f *fakeBindingService) PutWithIdempotency(ctx context.Context, projectID uuid.UUID, stage workflowbinding.WorkflowBindingStage, req workflowbinding.PutRequest, key string) (workflowbinding.PutResult, int, error) {
	if f.putFn != nil {
		return f.putFn(ctx, projectID, stage, req, key)
	}
	return workflowbinding.PutResult{}, 0, errors.New("unexpected put")
}

func (f *fakeBindingService) DeleteWithIdempotency(ctx context.Context, projectID uuid.UUID, stage workflowbinding.WorkflowBindingStage, req workflowbinding.DeleteRequest, key string) (workflowbinding.UnbindResult, int, error) {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, projectID, stage, req, key)
	}
	return workflowbinding.UnbindResult{}, 0, errors.New("unexpected delete")
}

func workflowBindingTestHandler(svc workflowbinding.BindingService) http.Handler {
	mux := http.NewServeMux()
	registerWorkflowBindingRoutes(mux, svc)
	return withRequestID(mux)
}

func doWorkflowBindingRequest(handler http.Handler, method, path, body, idemKey string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if idemKey != "" {
		r.Header.Set("Idempotency-Key", idemKey)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func mustParseWorkflowBindingEnvelope(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshal envelope: %v\nbody: %s", err, w.Body.String())
	}
	reqIDRaw, ok := raw["request_id"]
	if !ok {
		t.Fatalf("missing request_id in response envelope: %s", w.Body.String())
	}
	var reqID string
	if err := json.Unmarshal(reqIDRaw, &reqID); err != nil || reqID == "" {
		t.Fatalf("invalid request_id in response envelope: %s", w.Body.String())
	}
	headerReqID := w.Header().Get("X-Request-ID")
	if headerReqID == "" {
		t.Fatalf("missing X-Request-ID header in response")
	}
	if headerReqID != reqID {
		t.Fatalf("X-Request-ID header %q != body request_id %q", headerReqID, reqID)
	}

	res := map[string]any{
		"request_id": reqID,
	}
	if dataRaw, ok := raw["data"]; ok {
		var data map[string]any
		if err := json.Unmarshal(dataRaw, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		res["data"] = data
	}
	if errRaw, ok := raw["error"]; ok {
		var errObj map[string]any
		if err := json.Unmarshal(errRaw, &errObj); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		res["error"] = errObj
	}
	return res
}

func TestWorkflowBindingRoutesRegistered(t *testing.T) {
	h := workflowBindingTestHandler(&fakeBindingService{})
	paths := []struct{
		method, path string
	}{
		{"GET", "/api/v1/projects/11111111-1111-4111-8111-111111111111/workflow-bindings"},
		{"PUT", "/api/v1/projects/11111111-1111-4111-8111-111111111111/workflow-bindings/chapter_planning"},
		{"DELETE", "/api/v1/projects/11111111-1111-4111-8111-111111111111/workflow-bindings/chapter_planning"},
	}
	for _, tc := range paths {
		w := doWorkflowBindingRequest(h, tc.method, tc.path, "", "key")
		// Routing itself must not 404 even if service fails.
		if w.Code == http.StatusNotFound {
			t.Fatalf("route not registered: %s %s", tc.method, tc.path)
		}
	}
}

func TestWorkflowBindingGetFourStagesFixedOrder(t *testing.T) {
	projectID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	svc := &fakeBindingService{
		listFn: func(ctx context.Context, id uuid.UUID) ([]workflowbinding.StageRead, error) {
			return []workflowbinding.StageRead{
				{Stage: workflowbinding.StageChapterPlanning},
				{Stage: workflowbinding.StageContentGeneration},
				{Stage: workflowbinding.StageReview},
				{Stage: workflowbinding.StageRewrite},
			}, nil
		},
	}
	h := workflowBindingTestHandler(svc)
	w := doWorkflowBindingRequest(h, http.MethodGet, "/api/v1/projects/"+projectID.String()+"/workflow-bindings", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	env := mustParseWorkflowBindingEnvelope(t, w)
	items, ok := env["data"].(map[string]any)["items"].([]any)
	if !ok || len(items) != 4 {
		t.Fatalf("expected 4 items, got %+v", env["data"])
	}
	expected := []string{"chapter_planning", "content_generation", "review", "rewrite"}
	for i, want := range expected {
		it := items[i].(map[string]any)
		if it["stage"] != want {
			t.Fatalf("items[%d].stage=%v want %s", i, it["stage"], want)
		}
		if it["bound"] != false {
			t.Fatalf("items[%d].bound=%v want false", i, it["bound"])
		}
		if it["binding"] != nil {
			t.Fatalf("items[%d].binding=%v want nil", i, it["binding"])
		}
		if it["workflowConfigurationSummary"] != nil {
			t.Fatalf("items[%d].workflowConfigurationSummary=%v want nil", i, it["workflowConfigurationSummary"])
		}
	}
}

func TestWorkflowBindingGetBoundShape(t *testing.T) {
	projectID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	wfID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	connID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	now := time.Now().UTC()
	svc := &fakeBindingService{
		listFn: func(ctx context.Context, id uuid.UUID) ([]workflowbinding.StageRead, error) {
			b := workflowbinding.ProjectWorkflowBinding{
				ID:                      uuid.MustParse("44444444-4444-4444-8444-444444444444"),
				ProjectID:               projectID,
				Stage:                   workflowbinding.StageChapterPlanning,
				WorkflowConfigurationID: wfID,
				Version:                 1,
				CreatedAt:               now,
				UpdatedAt:               now,
			}
			summary := workflowbinding.ReadWorkflowConfiguration{
				ID:                    wfID,
				Name:                  "planner",
				ConnectionID:          connID,
				ConnectionName:        "conn",
				ConnectionType:        "n8n",
				WorkflowType:          "n8n",
				ApplicableStages:      []string{"chapter_planning"},
				TypeConfig:            json.RawMessage(`{"referenceType":"workflow_id","referenceValue":"wf-1"}`),
				InputContractVersion:  "v1",
				OutputContractVersion: "v1",
				DefaultParameters:     json.RawMessage(`{"temperature":0.7}`),
				IntegrationStatus:     "not_connected",
				Enabled:               true,
				Version:               1,
				CreatedAt:             now,
				UpdatedAt:             now,
			}
			return []workflowbinding.StageRead{{Stage: workflowbinding.StageChapterPlanning, Bound: true, Binding: &b, WorkflowConfigurationSummary: &summary}}, nil
		},
	}
	h := workflowBindingTestHandler(svc)
	w := doWorkflowBindingRequest(h, http.MethodGet, "/api/v1/projects/"+projectID.String()+"/workflow-bindings", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	env := mustParseWorkflowBindingEnvelope(t, w)
	item := env["data"].(map[string]any)["items"].([]any)[0].(map[string]any)
	binding := item["binding"].(map[string]any)
	for _, want := range []string{"id", "projectId", "stage", "workflowConfigurationId", "version", "createdAt", "updatedAt"} {
		if _, ok := binding[want]; !ok {
			t.Fatalf("binding missing %s", want)
		}
	}
	if binding["stage"] != "chapter_planning" {
		t.Fatalf("binding.stage=%v", binding["stage"])
	}
	sum := item["workflowConfigurationSummary"].(map[string]any)
	if sum["typeConfig"] == nil || sum["defaultParameters"] == nil {
		t.Fatalf("summary objects missing: %v", sum)
	}
	tc := sum["typeConfig"].(map[string]any)
	if tc["referenceType"] != "workflow_id" {
		t.Fatalf("typeConfig not object: %v", tc)
	}
	dp := sum["defaultParameters"].(map[string]any)
	if dp["temperature"] != 0.7 {
		t.Fatalf("defaultParameters not object: %v", dp)
	}
}

func TestWorkflowBindingPutCreateReplaceNoop(t *testing.T) {
	projectID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	wfID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	bindingID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	now := time.Now().UTC()

	b := workflowbinding.ProjectWorkflowBinding{ID: bindingID, ProjectID: projectID, Stage: workflowbinding.StageChapterPlanning, WorkflowConfigurationID: wfID, Version: 1, CreatedAt: now, UpdatedAt: now}
	summary := workflowbinding.ReadWorkflowConfiguration{ID: wfID, Name: "planner", ConnectionID: uuid.New(), ConnectionName: "conn", ConnectionType: "n8n", WorkflowType: "n8n", ApplicableStages: []string{"chapter_planning"}, TypeConfig: json.RawMessage(`{}`), InputContractVersion: "v1", OutputContractVersion: "v1", DefaultParameters: json.RawMessage(`{}`), IntegrationStatus: "not_connected", Enabled: true, Version: 1, CreatedAt: now, UpdatedAt: now}

	twoPointOh := uuid.MustParse("55555555-5555-4555-8555-555555555555")

	svc := &fakeBindingService{
		putFn: func(ctx context.Context, pid uuid.UUID, stage workflowbinding.WorkflowBindingStage, req workflowbinding.PutRequest, key string) (workflowbinding.PutResult, int, error) {
			if req.ExpectedVersion == nil {
				return workflowbinding.PutResult{Stage: stage, Binding: b, Summary: summary, Created: true}, 201, nil
			}
			if *req.ExpectedVersion == 1 {
				return workflowbinding.PutResult{Stage: stage, Binding: b, Summary: summary, NoChange: true}, 200, nil
			}
			if *req.ExpectedVersion == 2 {
				b2 := b
				b2.WorkflowConfigurationID = twoPointOh
				b2.Version = 2
				s2 := summary
				s2.ID = twoPointOh
				return workflowbinding.PutResult{Stage: stage, Binding: b2, Summary: s2}, 200, nil
			}
			return workflowbinding.PutResult{}, 0, errors.New("unexpected put")
		},
	}
	h := workflowBindingTestHandler(svc)

	// Create -> 201
	w := doWorkflowBindingRequest(h, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning", `{"workflowConfigurationId":"`+wfID.String()+`"}`, "create-key")
	if w.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}

	// Replace -> 200
	w = doWorkflowBindingRequest(h, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning", `{"workflowConfigurationId":"`+twoPointOh.String()+`","expectedVersion":2}`, "replace-key")
	if w.Code != http.StatusOK {
		t.Fatalf("replace status=%d body=%s", w.Code, w.Body.String())
	}
	data := mustParseWorkflowBindingEnvelope(t, w)["data"].(map[string]any)
	if data["binding"].(map[string]any)["workflowConfigurationId"] != twoPointOh.String() {
		t.Fatalf("replace did not return new workflow id: %v", data["binding"])
	}

	// No-op -> 200
	w = doWorkflowBindingRequest(h, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning", `{"workflowConfigurationId":"`+wfID.String()+`","expectedVersion":1}`, "noop-key")
	if w.Code != http.StatusOK {
		t.Fatalf("noop status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestWorkflowBindingPutIdempotentReplay(t *testing.T) {
	projectID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	wfID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	bindingID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	now := time.Now().UTC()

	b := workflowbinding.ProjectWorkflowBinding{ID: bindingID, ProjectID: projectID, Stage: workflowbinding.StageChapterPlanning, WorkflowConfigurationID: wfID, Version: 1, CreatedAt: now, UpdatedAt: now}
	summary := workflowbinding.ReadWorkflowConfiguration{ID: wfID, Name: "planner", ConnectionID: uuid.New(), ConnectionName: "conn", ConnectionType: "n8n", WorkflowType: "n8n", ApplicableStages: []string{"chapter_planning"}, TypeConfig: json.RawMessage(`{}`), InputContractVersion: "v1", OutputContractVersion: "v1", DefaultParameters: json.RawMessage(`{}`), IntegrationStatus: "not_connected", Enabled: true, Version: 1, CreatedAt: now, UpdatedAt: now}

	called := 0
	svc := &fakeBindingService{
		putFn: func(ctx context.Context, pid uuid.UUID, stage workflowbinding.WorkflowBindingStage, req workflowbinding.PutRequest, key string) (workflowbinding.PutResult, int, error) {
			called++
			return workflowbinding.PutResult{Stage: stage, Binding: b, Summary: summary, Created: true}, 201, nil
		},
	}
	h := workflowBindingTestHandler(svc)
	body := `{"workflowConfigurationId":"` + wfID.String() + `"}`
	w1 := doWorkflowBindingRequest(h, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning", body, "replay-key")
	if w1.Code != 201 {
		t.Fatalf("first status=%d body=%s", w1.Code, w1.Body.String())
	}
	w2 := doWorkflowBindingRequest(h, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning", body, "replay-key")
	if w2.Code != 201 {
		t.Fatalf("replay status=%d body=%s", w2.Code, w2.Body.String())
	}
	if called != 2 {
		t.Fatalf("service called %d times, want 2", called)
	}
}

func TestWorkflowBindingDeleteAndReplay(t *testing.T) {
	projectID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	svc := &fakeBindingService{
		deleteFn: func(ctx context.Context, pid uuid.UUID, stage workflowbinding.WorkflowBindingStage, req workflowbinding.DeleteRequest, key string) (workflowbinding.UnbindResult, int, error) {
			return workflowbinding.UnbindResult{ProjectID: pid, Stage: stage, Unbound: true, WorkflowConfigurationRetained: true}, 200, nil
		},
	}
	h := workflowBindingTestHandler(svc)
	w := doWorkflowBindingRequest(h, http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning?expected_version=1", "", "delete-key")
	if w.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body.String())
	}
	env := mustParseWorkflowBindingEnvelope(t, w)
	data := env["data"].(map[string]any)
	if data["unbound"] != true || data["workflowConfigurationRetained"] != true {
		t.Fatalf("unexpected delete data: %v", data)
	}
	// Replay must still be 200.
	w2 := doWorkflowBindingRequest(h, http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning?expected_version=1", "", "delete-key")
	if w2.Code != http.StatusOK {
		t.Fatalf("delete replay status=%d body=%s", w2.Code, w2.Body.String())
	}
}

func TestWorkflowBindingValidationErrors(t *testing.T) {
	h := workflowBindingTestHandler(&fakeBindingService{})
	cases := []struct {
		name, method, path, body, key string
		status int
		code string
	}{
		{"missing idempotency key", http.MethodPut, "/api/v1/projects/11111111-1111-4111-8111-111111111111/workflow-bindings/chapter_planning", `{"workflowConfigurationId":"22222222-2222-4222-8222-222222222222"}`, "", http.StatusBadRequest, "validation_error"},
		{"delete missing expected_version", http.MethodDelete, "/api/v1/projects/11111111-1111-4111-8111-111111111111/workflow-bindings/chapter_planning", "", "key", http.StatusBadRequest, "validation_error"},
		{"delete expected_version zero", http.MethodDelete, "/api/v1/projects/11111111-1111-4111-8111-111111111111/workflow-bindings/chapter_planning?expected_version=0", "", "key", http.StatusBadRequest, "validation_error"},
		{"delete expected_version not int", http.MethodDelete, "/api/v1/projects/11111111-1111-4111-8111-111111111111/workflow-bindings/chapter_planning?expected_version=abc", "", "key", http.StatusBadRequest, "validation_error"},
		{"invalid project id", http.MethodGet, "/api/v1/projects/not-a-uuid/workflow-bindings", "", "", http.StatusBadRequest, "validation_error"},
		{"invalid stage", http.MethodPut, "/api/v1/projects/11111111-1111-4111-8111-111111111111/workflow-bindings/invalid_stage", `{"workflowConfigurationId":"22222222-2222-4222-8222-222222222222"}`, "key", http.StatusBadRequest, "validation_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doWorkflowBindingRequest(h, tc.method, tc.path, tc.body, tc.key)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			env := mustParseWorkflowBindingEnvelope(t, w)
			if env["error"].(map[string]any)["code"] != tc.code {
				t.Fatalf("code=%v want %s", env["error"].(map[string]any)["code"], tc.code)
			}
		})
	}
}

func TestWorkflowBindingErrorMapping(t *testing.T) {
	projectID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	wfID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	stage := workflowbinding.StageChapterPlanning

	cases := []struct {
		name   string
		setup  *fakeBindingService
		path   string
		method string
		body   string
		status int
		code   string
	}{
		{
			name: "project_not_found",
			setup: &fakeBindingService{listFn: func(ctx context.Context, id uuid.UUID) ([]workflowbinding.StageRead, error) { return nil, workflowbinding.ErrProjectNotFound }},
			path: "/api/v1/projects/" + projectID.String() + "/workflow-bindings",
			method: http.MethodGet, status: 404, code: "project_not_found",
		},
		{
			name: "configuration_not_found",
			setup: &fakeBindingService{putFn: func(ctx context.Context, pid uuid.UUID, s workflowbinding.WorkflowBindingStage, r workflowbinding.PutRequest, k string) (workflowbinding.PutResult, int, error) {
				return workflowbinding.PutResult{}, 0, workflowbinding.ErrConfigurationNotFound
			}},
			path: "/api/v1/projects/" + projectID.String() + "/workflow-bindings/chapter_planning",
			method: http.MethodPut, body: `{"workflowConfigurationId":"` + wfID.String() + `"}`, status: 404, code: "configuration_not_found",
		},
		{
			name: "workflow_binding_not_found",
			setup: &fakeBindingService{deleteFn: func(ctx context.Context, pid uuid.UUID, s workflowbinding.WorkflowBindingStage, r workflowbinding.DeleteRequest, k string) (workflowbinding.UnbindResult, int, error) {
				return workflowbinding.UnbindResult{}, 0, workflowbinding.ErrNotFound
			}},
			path: "/api/v1/projects/" + projectID.String() + "/workflow-bindings/chapter_planning?expected_version=1",
			method: http.MethodDelete, status: 404, code: "workflow_binding_not_found",
		},
		{
			name: "binding_already_exists",
			setup: &fakeBindingService{putFn: func(ctx context.Context, pid uuid.UUID, s workflowbinding.WorkflowBindingStage, r workflowbinding.PutRequest, k string) (workflowbinding.PutResult, int, error) {
				return workflowbinding.PutResult{}, 0, workflowbinding.ErrBindingAlreadyExists
			}},
			path: "/api/v1/projects/" + projectID.String() + "/workflow-bindings/chapter_planning",
			method: http.MethodPut, body: `{"workflowConfigurationId":"` + wfID.String() + `"}`, status: 409, code: "binding_already_exists",
		},
		{
			name: "idempotency_key_reused",
			setup: &fakeBindingService{putFn: func(ctx context.Context, pid uuid.UUID, s workflowbinding.WorkflowBindingStage, r workflowbinding.PutRequest, k string) (workflowbinding.PutResult, int, error) {
				return workflowbinding.PutResult{}, 0, workflowbinding.ErrIdempotencyReused
			}},
			path: "/api/v1/projects/" + projectID.String() + "/workflow-bindings/chapter_planning",
			method: http.MethodPut, body: `{"workflowConfigurationId":"` + wfID.String() + `"}`, status: 409, code: "idempotency_key_reused_with_different_payload",
		},
		{
			name: "disabled_workflow",
			setup: &fakeBindingService{putFn: func(ctx context.Context, pid uuid.UUID, s workflowbinding.WorkflowBindingStage, r workflowbinding.PutRequest, k string) (workflowbinding.PutResult, int, error) {
				return workflowbinding.PutResult{}, 0, workflowbinding.ErrDisabledWorkflow
			}},
			path: "/api/v1/projects/" + projectID.String() + "/workflow-bindings/chapter_planning",
			method: http.MethodPut, body: `{"workflowConfigurationId":"` + wfID.String() + `"}`, status: 422, code: "disabled_workflow",
		},
		{
			name: "workflow_not_applicable_to_stage",
			setup: &fakeBindingService{putFn: func(ctx context.Context, pid uuid.UUID, s workflowbinding.WorkflowBindingStage, r workflowbinding.PutRequest, k string) (workflowbinding.PutResult, int, error) {
				return workflowbinding.PutResult{}, 0, workflowbinding.ErrNotApplicable
			}},
			path: "/api/v1/projects/" + projectID.String() + "/workflow-bindings/chapter_planning",
			method: http.MethodPut, body: `{"workflowConfigurationId":"` + wfID.String() + `"}`, status: 422, code: "workflow_not_applicable_to_stage",
		},
		{
			name: "internal_error",
			setup: &fakeBindingService{listFn: func(ctx context.Context, id uuid.UUID) ([]workflowbinding.StageRead, error) { return nil, errors.New("boom") }},
			path: "/api/v1/projects/" + projectID.String() + "/workflow-bindings",
			method: http.MethodGet, status: 500, code: "internal_error",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := workflowBindingTestHandler(tc.setup)
			w := doWorkflowBindingRequest(h, tc.method, tc.path, tc.body, "err-key")
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			var rawEnv map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &rawEnv); err != nil {
				t.Fatalf("unmarshal error envelope: %v", err)
			}
			if _, ok := rawEnv["data"]; ok {
				t.Fatalf("error response must NOT contain data key: %s", w.Body.String())
			}
			if _, ok := rawEnv["error"]; !ok {
				t.Fatalf("error response MUST contain error key: %s", w.Body.String())
			}
			var errObj struct {
				Code string `json:"code"`
			}
			if err := json.Unmarshal(rawEnv["error"], &errObj); err != nil || errObj.Code != tc.code {
				t.Fatalf("code=%v want %s", errObj.Code, tc.code)
			}
		})
	}

	// Version conflict details.
	t.Run("version_conflict", func(t *testing.T) {
		svc := &fakeBindingService{
			putFn: func(ctx context.Context, pid uuid.UUID, s workflowbinding.WorkflowBindingStage, r workflowbinding.PutRequest, k string) (workflowbinding.PutResult, int, error) {
				return workflowbinding.PutResult{}, 0, &workflowbinding.VersionConflictError{ProjectID: pid, Stage: stage, ExpectedVersion: 1, CurrentVersion: 3}
			},
		}
		h := workflowBindingTestHandler(svc)
		w := doWorkflowBindingRequest(h, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning", `{"workflowConfigurationId":"`+wfID.String()+`","expectedVersion":1}`, "vc-key")
		if w.Code != 409 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var env struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
				Details struct {
					ExpectedVersion int    `json:"expectedVersion"`
					CurrentVersion  int    `json:"currentVersion"`
					ProjectID       string `json:"projectId"`
					Stage           string `json:"stage"`
				} `json:"details"`
			} `json:"error"`
			RequestID string `json:"request_id"`
		}
		dec := json.NewDecoder(w.Body)
		dec.UseNumber()
		if err := dec.Decode(&env); err != nil {
			t.Fatalf("decode version_conflict response: %v", err)
		}
		if env.Error.Code != "version_conflict" {
			t.Fatalf("code=%q, want version_conflict", env.Error.Code)
		}
		if env.Error.Details.ExpectedVersion != 1 {
			t.Fatalf("expectedVersion=%d, want 1", env.Error.Details.ExpectedVersion)
		}
		if env.Error.Details.CurrentVersion != 3 {
			t.Fatalf("currentVersion=%d, want 3", env.Error.Details.CurrentVersion)
		}
		if env.Error.Details.ProjectID != projectID.String() {
			t.Fatalf("projectId=%q, want %q", env.Error.Details.ProjectID, projectID.String())
		}
		if env.Error.Details.Stage != "chapter_planning" {
			t.Fatalf("stage=%q, want chapter_planning", env.Error.Details.Stage)
		}
	})
}

func TestWorkflowBindingSingleLayerEnvelope(t *testing.T) {
	projectID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	wfID := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	bindingID := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	now := time.Now().UTC()
	b := workflowbinding.ProjectWorkflowBinding{ID: bindingID, ProjectID: projectID, Stage: workflowbinding.StageChapterPlanning, WorkflowConfigurationID: wfID, Version: 1, CreatedAt: now, UpdatedAt: now}
	summary := workflowbinding.ReadWorkflowConfiguration{ID: wfID, Name: "planner", ConnectionID: uuid.New(), ConnectionName: "conn", ConnectionType: "n8n", WorkflowType: "n8n", ApplicableStages: []string{"chapter_planning"}, TypeConfig: json.RawMessage(`{}`), InputContractVersion: "v1", OutputContractVersion: "v1", DefaultParameters: json.RawMessage(`{}`), IntegrationStatus: "not_connected", Enabled: true, Version: 1, CreatedAt: now, UpdatedAt: now}

	h := workflowBindingTestHandler(&fakeBindingService{
		putFn: func(ctx context.Context, pid uuid.UUID, s workflowbinding.WorkflowBindingStage, r workflowbinding.PutRequest, k string) (workflowbinding.PutResult, int, error) {
			return workflowbinding.PutResult{Stage: s, Binding: b, Summary: summary, Created: true}, 201, nil
		},
	})
	w := doWorkflowBindingRequest(h, http.MethodPut, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning", `{"workflowConfigurationId":"`+wfID.String()+`"}`, "single-key")
	if w.Code != 201 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["data"]; !ok {
		t.Fatalf("missing data key: %v", raw)
	}
	data := raw["data"].(map[string]any)
	if _, ok := data["data"]; ok {
		t.Fatalf("data.data present: %v", raw)
	}
}

func TestWorkflowBindingSummaryLoadFailureIsInternalError(t *testing.T) {
	projectID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	h := workflowBindingTestHandler(&fakeBindingService{
		listFn: func(ctx context.Context, id uuid.UUID) ([]workflowbinding.StageRead, error) {
			return nil, errors.New("summary load failure")
		},
	})
	w := doWorkflowBindingRequest(h, http.MethodGet, "/api/v1/projects/"+projectID.String()+"/workflow-bindings", "", "")
	if w.Code != 500 {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	env := mustParseWorkflowBindingEnvelope(t, w)
	if env["error"].(map[string]any)["code"] != "internal_error" {
		t.Fatalf("expected internal_error: %v", env)
	}
	if strings.Contains(w.Body.String(), `"summary":null`) || strings.Contains(w.Body.String(), `"workflowConfigurationSummary":null`) {
		t.Fatalf("must not return null summary: %s", w.Body.String())
	}
}

// ── Integration-style handler test (real service + db) ──────────────────────

func workflowBindingIntegrationServer(t *testing.T, pool *pgxpool.Pool) http.Handler {
	t.Helper()
	svc, err := globalconfig.NewService(pool, "iteration-13-http-test-key")
	if err != nil {
		t.Fatal(err)
	}
	projects := project.NewPostgresRepository(pool)
	loop := workflowbinding.NewCloseLoop(pool, projects, svc)
	return New(":0", project.NewService(projects), svc, loop).httpServer.Handler
}

func TestWorkflowBindingHandlerIntegration(t *testing.T) {
	pool, ctx := workflowBindingIntegrationDatabase(t)
	projectID := uuid.New()
	connID := uuid.New()
	wfID := uuid.New()

	t.Cleanup(func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pStr := projectID.String()
		_, _ = pool.Exec(cleanCtx, "DELETE FROM project_workflow_bindings WHERE project_id = $1", projectID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM idempotency_records WHERE scope LIKE $1", "workflow_binding:"+pStr+"%")
		_, _ = pool.Exec(cleanCtx, "DELETE FROM audit_logs WHERE payload->>'projectId' = $1 OR payload->>'project_id' = $1", pStr)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM workflow_configurations WHERE id = $1", wfID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM workflow_connections WHERE id = $1", connID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM projects WHERE id = $1", projectID)
	})

	insertWorkflowBindingProject(t, ctx, pool, projectID)
	insertWorkflowBindingWorkflow(t, ctx, pool, wfID, connID, []string{"chapter_planning"})

	h := workflowBindingIntegrationServer(t, pool)
	call := func(method, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	// GET returns four stages, all unbound.
	w := call(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/workflow-bindings", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"stage":"chapter_planning"`) {
		t.Fatalf("get list: %d %s", w.Code, w.Body.String())
	}

	// PUT create -> 201
	body := `{"workflowConfigurationId":"` + wfID.String() + `"}`
	w = call(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning", body, "put-create")
	if w.Code != 201 || !strings.Contains(w.Body.String(), "workflowConfigurationSummary") {
		t.Fatalf("put create: %d %s", w.Code, w.Body.String())
	}

	// PUT replay -> 201
	w = call(http.MethodPut, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning", body, "put-create")
	if w.Code != 201 {
		t.Fatalf("put replay: %d %s", w.Code, w.Body.String())
	}

	// GET now bound
	w = call(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/workflow-bindings", "", "")
	if !strings.Contains(w.Body.String(), `"bound":true`) || !strings.Contains(w.Body.String(), `"typeConfig":{`) {
		t.Fatalf("get bound: %s", w.Body.String())
	}

	// DELETE -> 200
	w = call(http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning?expected_version=1", "", "delete-first")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"unbound":true`) {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}

	// DELETE replay -> 200
	w = call(http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/workflow-bindings/chapter_planning?expected_version=1", "", "delete-first")
	if w.Code != 200 {
		t.Fatalf("delete replay: %d %s", w.Code, w.Body.String())
	}
}

func TestWorkflowBindingHandlerIntegrationValidation(t *testing.T) {
	pool, ctx := workflowBindingIntegrationDatabase(t)
	projectID := uuid.New()
	t.Cleanup(func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pStr := projectID.String()
		_, _ = pool.Exec(cleanCtx, "DELETE FROM project_workflow_bindings WHERE project_id = $1", projectID)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM idempotency_records WHERE scope LIKE $1", "workflow_binding:"+pStr+"%")
		_, _ = pool.Exec(cleanCtx, "DELETE FROM audit_logs WHERE payload->>'projectId' = $1 OR payload->>'project_id' = $1", pStr)
		_, _ = pool.Exec(cleanCtx, "DELETE FROM projects WHERE id = $1", projectID)
	})

	insertWorkflowBindingProject(t, ctx, pool, projectID)
	h := workflowBindingIntegrationServer(t, pool)
	call := func(method, path, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	cases := []struct {
		name, method, path, body, key string
		status int
		code string
	}{
		{"missing idempotency key put", http.MethodPut, "/api/v1/projects/" + projectID.String() + "/workflow-bindings/chapter_planning", `{"workflowConfigurationId":"22222222-2222-4222-8222-222222222222"}`, "", 400, "validation_error"},
		{"missing expected_version", http.MethodDelete, "/api/v1/projects/" + projectID.String() + "/workflow-bindings/chapter_planning", "", "key", 400, "validation_error"},
		{"expected_version zero", http.MethodDelete, "/api/v1/projects/" + projectID.String() + "/workflow-bindings/chapter_planning?expected_version=0", "", "key", 400, "validation_error"},
		{"invalid project id", http.MethodGet, "/api/v1/projects/bad-uuid/workflow-bindings", "", "", 400, "validation_error"},
		{"invalid stage", http.MethodPut, "/api/v1/projects/" + projectID.String() + "/workflow-bindings/invalid_stage", `{"workflowConfigurationId":"22222222-2222-4222-8222-222222222222"}`, "key", 400, "validation_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := call(tc.method, tc.path, tc.body, tc.key)
			if w.Code != tc.status {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("missing %s: %s", tc.code, w.Body.String())
			}
		})
	}
}

func workflowBindingIntegrationDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		if os.Getenv("REQUIRE_POSTGRES_INTEGRATION") == "1" {
			t.Fatalf("TEST_DATABASE_URL must be set when REQUIRE_POSTGRES_INTEGRATION=1")
		}
		t.Skip("TEST_DATABASE_URL is not set; integration test skipped")
	}
	cfg, err := pgxpool.ParseConfig(u)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	if cfg.ConnConfig.Database != "ai_content_factory_http_test" {
		t.Fatalf("TEST_DATABASE_URL database=%q, want %q", cfg.ConnConfig.Database, "ai_content_factory_http_test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	var exists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'project_workflow_bindings')").Scan(&exists)
	if err != nil || !exists {
		t.Fatalf("table project_workflow_bindings missing in database ai_content_factory_http_test: %v", err)
	}

	return pool, ctx
}

func insertWorkflowBindingProject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `INSERT INTO projects (id, name, type, description, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'novel', '', 'planning', 'system', $3, $4)`, id, "test-project-"+id.String()[:8], now, now)
	if err != nil {
		t.Fatalf("insert project fixture: %v", err)
	}
}

func insertWorkflowBindingWorkflow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, wfID, connID uuid.UUID, stages []string) {
	t.Helper()
	now := time.Now().UTC()
	_, err := pool.Exec(ctx, `INSERT INTO workflow_connections (id, name, connection_type, base_url, auth_type, timeout_seconds, type_config, created_at, updated_at)
		VALUES ($1, $2, 'n8n', 'http://localhost', 'api_key', 30, '{}', $3, $4)`, connID, "test-conn-"+connID.String()[:8], now, now)
	if err != nil {
		t.Fatalf("insert connection fixture: %v", err)
	}
	raw, _ := json.Marshal(stages)
	_, err = pool.Exec(ctx, `INSERT INTO workflow_configurations (id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version, default_parameters, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, '{}', 'v1', 'v1', '{}', true, $5, $6)`, wfID, "test-wf-"+wfID.String()[:8], connID, raw, now, now)
	if err != nil {
		t.Fatalf("insert workflow fixture: %v", err)
	}
}

// bytesBuffer prevents go vet unused import if bytes is used above.
var _ = bytes.NewBuffer

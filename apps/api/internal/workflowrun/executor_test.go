package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestUnavailableWorkflowExecutor(t *testing.T) {
	x := UnavailableWorkflowExecutor{}
	if err := x.Verify(context.Background(), ExecutionRequest{}); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatal(err)
	}
	if _, err := x.Execute(context.Background(), ExecutionRequest{}); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatal(err)
	}
	if _, err := x.Cancel(context.Background(), ExecutionRequest{}); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatal(err)
	}
}

func TestFakeWorkflowExecutorAndServiceMapping(t *testing.T) {
	s, store, projectID := fixtureService(t)
	connectionID := uuid.New()
	runID := uuid.New()
	now := s.now()
	run := WorkflowRun{ID: runID, RunNumber: "WR-EXEC", ProjectID: projectID, Stage: "review", WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued, ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + connectionID.String() + `"},"workflowConfiguration":{"defaultParameters":{"token":"hidden"}}}`), InputPayload: json.RawMessage(`{"text":"ok"}`), CreatedAt: now, UpdatedAt: now, Version: 1}
	store.runs[runID] = run
	fake := &FakeWorkflowExecutor{ExecuteResult: ExecutionResult{Status: ExecutionSucceeded, Output: json.RawMessage(`{"result":"ok","access_token":"hidden"}`), Metadata: map[string]string{"token": "hidden"}}}
	s.SetWorkflowExecutor(fake)
	updated, err := s.ExecuteRun(context.Background(), runID)
	if err != nil || updated.Status != StatusSucceeded || fake.ExecuteCalls != 1 {
		t.Fatalf("run=%+v err=%v calls=%d", updated, err, fake.ExecuteCalls)
	}
	if len(store.events[runID]) != 3 || string(updated.OutputPayload) == "" || store.events[runID][1].EventType != "output_validated" {
		t.Fatalf("events=%+v output=%s", store.events[runID], updated.OutputPayload)
	}
	if string(fake.LastRequest.ConfigurationSnapshot) == "" || string(fake.LastRequest.Parameters) == "" {
		t.Fatal("missing request")
	}
}

func TestExecutionFailureIsDomainTransition(t *testing.T) {
	s, store, projectID := fixtureService(t)
	id, connectionID := uuid.New(), uuid.New()
	now := s.now()
	store.runs[id] = WorkflowRun{ID: id, RunNumber: "WR-FAIL", ProjectID: projectID, Stage: "review", WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued, ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + connectionID.String() + `"}}`), InputPayload: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now, Version: 1}
	s.SetWorkflowExecutor(&FakeWorkflowExecutor{ExecuteError: ErrExecutionTimeout})
	updated, err := s.ExecuteRun(context.Background(), id)
	if err != nil || updated.Status != StatusTimedOut || updated.ErrorCode == nil || *updated.ErrorCode != "upstream_timeout" || updated.TimedOutAt == nil {
		t.Fatalf("run=%+v err=%v", updated, err)
	}
}

func TestN8NWorkflowExecutorPostsRuntimeEnvelope(t *testing.T) {
	var received struct {
		RunID     string          `json:"runId"`
		ProjectID string          `json:"projectId"`
		Stage     string          `json:"stage"`
		Input     json.RawMessage `json:"input"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/webhook/chapter-planning" || r.Method != http.MethodPost {
			t.Fatalf("request=%s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-N8N-Execution-Id", "execution-42")
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	runID, projectID := uuid.New(), uuid.New()
	snapshot, _ := json.Marshal(map[string]any{
		"workflowConnection":    map[string]any{"type": "n8n", "baseUrl": server.URL, "timeoutSeconds": 5},
		"workflowConfiguration": map[string]any{"typeConfig": map[string]any{"referenceType": "webhook_path", "referenceValue": "chapter-planning"}},
	})
	result, err := NewN8NWorkflowExecutor(server.Client()).Execute(context.Background(), ExecutionRequest{
		RunID: runID, ProjectID: projectID, Stage: "chapter_planning",
		ConfigurationSnapshot: snapshot, Input: json.RawMessage(`{"generationContextDigest":"digest"}`),
	})
	if err != nil || result.Status != ExecutionSucceeded || result.ExternalExecutionID != "execution-42" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if received.RunID != runID.String() || received.ProjectID != projectID.String() || received.Stage != "chapter_planning" || string(received.Input) != `{"generationContextDigest":"digest"}` {
		t.Fatalf("received=%+v", received)
	}
}

func TestN8NWorkflowExecutorPersistsAcceptedExecutionForQueryRecovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-N8N-Execution-Id", "execution-running-42")
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}))
	defer server.Close()

	snapshot := json.RawMessage(`{"workflowConnection":{"type":"n8n","baseUrl":"` + server.URL + `","timeoutSeconds":5},"workflowConfiguration":{"typeConfig":{"referenceType":"webhook_path","referenceValue":"chapter-planning"}}}`)
	result, err := NewN8NWorkflowExecutor(server.Client()).Execute(context.Background(), ExecutionRequest{RunID: uuid.New(), ProjectID: uuid.New(), Stage: "chapter_planning", ConfigurationSnapshot: snapshot, Input: json.RawMessage(`{}`)})
	if err != nil || result.Status != ExecutionRunning || result.ExternalExecutionID != "execution-running-42" || len(result.Output) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestN8NWorkflowExecutorRejectsUnsafeWebhookReference(t *testing.T) {
	snapshot := json.RawMessage(`{"workflowConnection":{"type":"n8n","baseUrl":"https://example.test","timeoutSeconds":5},"workflowConfiguration":{"typeConfig":{"referenceType":"webhook_path","referenceValue":"../escape"}}}`)
	if _, _, err := n8nExecutionEndpoint(snapshot); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestN8NWorkflowExecutorQueryParsesTerminalAndRunningResponses(t *testing.T) {
	body := func(status string, output string) string {
		if output == "" { return `{"status":"` + status + `"}` }
		return `{"status":"` + status + `","data":{"resultData":{"lastNodeExecuted":"terminal","runData":{"terminal":[{"data":{"main":[[{"json":` + output + `}]]}}]}}}}`
	}
	for _, test := range []struct {
		name       string
		body       string
		wantStatus ExecutionStatus
		wantOutput string
	}{
		{name: "running", body: body("running", ""), wantStatus: ExecutionRunning},
		{name: "waiting", body: body("waiting", ""), wantStatus: ExecutionRunning},
		{name: "success", body: body("success", `{"result":"ok"}`), wantStatus: ExecutionSucceeded, wantOutput: `{"result":"ok"}`},
		{name: "failed", body: body("error", ""), wantStatus: ExecutionFailed},
		{name: "crashed", body: body("crashed", ""), wantStatus: ExecutionFailed},
		{name: "cancelled", body: body("canceled", ""), wantStatus: ExecutionCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/api/v1/executions/execution-42" || r.URL.Query().Get("includeData") != "true" {
					t.Fatalf("request=%s %s", r.Method, r.URL.Path)
				}
				_, _ = w.Write([]byte(test.body))
			}))
			defer server.Close()
			snapshot := json.RawMessage(`{"workflowConnection":{"type":"n8n","baseUrl":"` + server.URL + `","timeoutSeconds":5}}`)
			result, err := NewN8NWorkflowExecutor(server.Client()).Query(context.Background(), ExecutionRequest{ConfigurationSnapshot: snapshot, ExternalExecutionID: "execution-42"})
			if err != nil || result.Status != test.wantStatus || result.ExternalExecutionID != "execution-42" || (test.wantOutput != "" && string(result.Output) != test.wantOutput) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if result.Status == ExecutionFailed && (result.ErrorCode != "upstream_execution_failed" || result.ErrorMessage != "workflow execution failed") {
				t.Fatalf("unsafe failure=%+v", result)
			}
		})
	}
}

func TestN8NWorkflowExecutorQueryUsesDedicatedCredential(t *testing.T) {
	connectionID := uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-N8N-API-KEY"); got != "test-api-key" {
			t.Fatalf("api key=%q", got)
		}
		_, _ = w.Write([]byte(`{"status":"running"}`))
	}))
	defer server.Close()
	snapshot := json.RawMessage(`{"workflowConnection":{"type":"n8n","baseUrl":"` + server.URL + `","timeoutSeconds":5}}`)
	result, err := NewN8NWorkflowExecutor(server.Client(), func(_ context.Context, id uuid.UUID) (string, error) {
		if id != connectionID {
			t.Fatalf("connection id=%s", id)
		}
		return "test-api-key", nil
	}).Query(context.Background(), ExecutionRequest{ConfigurationSnapshot: snapshot, WorkflowConnectionID: connectionID, ExternalExecutionID: "execution-42"})
	if err != nil || result.Status != ExecutionRunning {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestN8NWorkflowExecutorQueryRejectsInvalidResponses(t *testing.T) {
	for _, body := range []string{
		`{"status":"success","data":{"resultData":{"runData":{}}}}`,
		`{"status":"unknown"}`,
		`not json`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			snapshot := json.RawMessage(`{"workflowConnection":{"type":"n8n","baseUrl":"` + server.URL + `","timeoutSeconds":5}}`)
			if _, err := NewN8NWorkflowExecutor(server.Client()).Query(context.Background(), ExecutionRequest{ConfigurationSnapshot: snapshot, ExternalExecutionID: "execution-42"}); !errors.Is(err, ErrInvalidExecutionResult) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestN8NWorkflowExecutorQueryRejectsMissingExternalExecutionID(t *testing.T) {
	snapshot := json.RawMessage(`{"workflowConnection":{"type":"n8n","baseUrl":"https://example.test","timeoutSeconds":5}}`)
	if _, err := NewN8NWorkflowExecutor(nil).Query(context.Background(), ExecutionRequest{ConfigurationSnapshot: snapshot}); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestN8NWorkflowExecutorCancelAcceptsCooperativeCancellation(t *testing.T) {
	result, err := NewN8NWorkflowExecutor(nil).Cancel(context.Background(), ExecutionRequest{ExternalExecutionID: "execution-42"})
	if err != nil || result.Status != ExecutionAccepted { t.Fatalf("result=%+v err=%v", result, err) }
}

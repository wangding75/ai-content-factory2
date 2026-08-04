package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/platform/config"
)

func successBody(main string) string {
	return `{"status":"success","data":{"resultData":{"lastNodeExecuted":"terminal","runData":{"terminal":[{"data":{"main":` + main + `}}]}}}}`
}

func TestParseN8NQueryResponseSingleItemContract(t *testing.T) {
	ok, err := parseN8NQueryResponse(json.RawMessage(successBody(`[[{"json":{"result":"ok"}}]]`)), "exec-1")
	if err != nil || ok.Status != ExecutionSucceeded || string(ok.Output) != `{"result":"ok"}` {
		t.Fatalf("ok=%+v err=%v", ok, err)
	}
}

func TestParseN8NQueryResponseRejectsZeroItems(t *testing.T) {
	result, err := parseN8NQueryResponse(json.RawMessage(successBody(`[[]]`)), "exec-1")
	if err != nil || result.Status != ExecutionFailed || result.ErrorCode != "execution_output_missing" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestParseN8NQueryResponseRejectsMultipleItems(t *testing.T) {
	result, err := parseN8NQueryResponse(json.RawMessage(successBody(`[[{"json":{"a":1}},{"json":{"b":2}}]]`)), "exec-1")
	if err != nil || result.Status != ExecutionFailed || result.ErrorCode != "execution_output_multiple_items" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestParseN8NQueryResponseRejectsMultipleBranches(t *testing.T) {
	result, err := parseN8NQueryResponse(json.RawMessage(successBody(`[[{"json":{"a":1}}],[{"json":{"b":2}}]]`)), "exec-1")
	if err != nil || result.Status != ExecutionFailed || result.ErrorCode != "execution_output_invalid_shape" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestParseN8NQueryResponseRejectsMissingMain(t *testing.T) {
	body := `{"status":"success","data":{"resultData":{"lastNodeExecuted":"terminal","runData":{"terminal":[{"data":{}}]}}}}`
	result, err := parseN8NQueryResponse(json.RawMessage(body), "exec-1")
	if err != nil || result.Status != ExecutionFailed || result.ErrorCode != "execution_output_missing" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestParseN8NQueryResponseRejectsMissingFinalNode(t *testing.T) {
	body := `{"status":"success","data":{"resultData":{"lastNodeExecuted":"","runData":{}}}}`
	result, err := parseN8NQueryResponse(json.RawMessage(body), "exec-1")
	if err != nil || result.Status != ExecutionFailed || result.ErrorCode != "execution_output_missing" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestParseN8NQueryResponseRejectsNonObjectJSON(t *testing.T) {
	for _, item := range []string{`null`, `[]`, `"text"`, `42`, `true`} {
		t.Run(item, func(t *testing.T) {
			result, err := parseN8NQueryResponse(json.RawMessage(successBody(`[[{"json":`+item+`}]]`)), "exec-1")
			if err != nil || result.Status != ExecutionFailed || result.ErrorCode != "execution_output_invalid_shape" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestN8NQueryResultSizeBoundary(t *testing.T) {
	// Cap must be within configured min/max (default min 64KiB).
	maxBytes := config.MinN8NExecutionResultMaxBytes
	payload := `{"x":"` + strings.Repeat("a", 100) + `"}`
	body := successBody(`[[{"json":` + payload + `}]]`)
	if len(body) > maxBytes {
		t.Fatalf("fixture too large for boundary test: %d", len(body))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()
	executor := NewN8NWorkflowExecutor(server.Client())
	executor.SetMaxResultBytes(maxBytes)
	snapshot := json.RawMessage(`{"workflowConnection":{"type":"n8n","baseUrl":"` + server.URL + `","timeoutSeconds":5}}`)
	result, err := executor.Query(context.Background(), ExecutionRequest{ConfigurationSnapshot: snapshot, ExternalExecutionID: "execution-42"})
	if err != nil || result.Status != ExecutionSucceeded {
		t.Fatalf("in-bound result=%+v err=%v", result, err)
	}

	// Oversized response body is permanent business failure, not transport error.
	largeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", maxBytes+1)))
	}))
	defer largeServer.Close()
	executor.SetMaxResultBytes(maxBytes)
	largeSnapshot := json.RawMessage(`{"workflowConnection":{"type":"n8n","baseUrl":"` + largeServer.URL + `","timeoutSeconds":5}}`)
	failed, err := executor.Query(context.Background(), ExecutionRequest{ConfigurationSnapshot: largeSnapshot, ExternalExecutionID: "execution-42"})
	if err != nil || failed.Status != ExecutionFailed || failed.ErrorCode != "execution_result_too_large" {
		t.Fatalf("oversize result=%+v err=%v", failed, err)
	}
}

func TestN8NQueryTemporaryHTTP5xxRemainsRecoverable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	snapshot := json.RawMessage(`{"workflowConnection":{"type":"n8n","baseUrl":"` + server.URL + `","timeoutSeconds":5}}`)
	if _, err := NewN8NWorkflowExecutor(server.Client()).Query(context.Background(), ExecutionRequest{ConfigurationSnapshot: snapshot, ExternalExecutionID: "execution-42"}); !errors.Is(err, ErrExecutorUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestPermanentOutputFailureAppliesFailedTerminalWithoutRetryLoop(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	connectionID := uuid.New()
	runID := uuid.New()
	external := "exec-permanent"
	run := WorkflowRun{
		ID: runID, RunNumber: "WR-PERM", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusRunning,
		ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + connectionID.String() + `"},"workflowConfiguration":{"defaultParameters":{}}}`),
		InputPayload: json.RawMessage(`{}`), ExternalExecutionID: &external, StartedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2,
	}
	store.runs[runID] = run
	fake := &FakeWorkflowExecutor{QueryResult: permanentOutputFailure("execution_output_multiple_items")}
	s.SetWorkflowExecutor(fake)
	updated, err := s.applyExecutionResult(context.Background(), run, fake.QueryResult)
	if err != nil || updated.Status != StatusFailed || updated.ErrorCode == nil || *updated.ErrorCode != "execution_output_multiple_items" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if store.runs[runID].Status != StatusFailed {
		t.Fatalf("run not terminal: %+v", store.runs[runID])
	}
}

func TestParseExecutionResultMaxBytesBounds(t *testing.T) {
	def, err := config.ParseN8NExecutionResultMaxBytes("")
	if err != nil || def != config.DefaultN8NExecutionResultMaxBytes {
		t.Fatalf("default=%d err=%v", def, err)
	}
	if _, err = config.ParseN8NExecutionResultMaxBytes("100"); err == nil {
		t.Fatal("expected min bound error")
	}
	if _, err = config.ParseN8NExecutionResultMaxBytes("999999999"); err == nil {
		t.Fatal("expected max bound error")
	}
	if _, err = config.ParseN8NExecutionResultMaxBytes("nope"); err == nil {
		t.Fatal("expected parse error")
	}
}

package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIteration14FrozenWorkflowRuntimeContract(t *testing.T) {
	contract, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "packages", "contracts", "openapi", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contract)
	operationIDs := map[string]struct{}{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "operationId: ") {
			continue
		}
		operationID := strings.TrimSpace(strings.TrimPrefix(line, "operationId: "))
		if _, exists := operationIDs[operationID]; exists {
			t.Fatalf("OpenAPI operationId must be unique: %s", operationID)
		}
		operationIDs[operationID] = struct{}{}
	}

	for _, path := range []string{
		"/api/v1/workflow-connections/{connectionId}/verify:",
		"/api/v1/workflow-connections/{connectionId}/enable:",
		"/api/v1/workflow-connections/{connectionId}/disable:",
		"/api/v1/workflow-configurations/{workflowConfigurationId}/verify:",
		"/api/v1/workflow-configurations/{workflowConfigurationId}/enable:",
		"/api/v1/workflow-configurations/{workflowConfigurationId}/disable:",
		"/api/v1/workflow-runs:",
		"/api/v1/workflow-runs/{runId}:",
		"/api/v1/workflow-runs/{runId}/events:",
		"/api/v1/workflow-runs/{runId}/retries:",
		"/api/v1/workflow-runs/{runId}/cancel:",
		"/api/v1/projects/{projectId}/workflow-run-summary:",
	} {
		if !strings.Contains(text, "  "+path) {
			t.Fatalf("frozen Iteration 14 path missing: %s", path)
		}
	}

	for _, operationID := range []string{
		"verifyWorkflowConnection", "enableWorkflowConnection", "disableWorkflowConnection",
		"verifyWorkflowConfiguration", "enableWorkflowConfiguration", "disableWorkflowConfiguration",
		"createWorkflowRun", "listWorkflowRuns", "getWorkflowRunDetail", "listWorkflowRunEvents",
		"retryWorkflowRun", "cancelWorkflowRun", "getProjectWorkflowRunSummary",
	} {
		if strings.Count(text, "operationId: "+operationID) != 1 {
			t.Fatalf("Iteration 14 operationId must exist exactly once: %s", operationID)
		}
	}

	for _, fragment := range []string{
		"CreateWorkflowRunRequest:", "Iteration14WorkflowRunEnvelope:", "Iteration14WorkflowRunListEnvelope:",
		"WorkflowRunEventListEnvelope:", "ProjectWorkflowRunSummaryEnvelope:", "WorkflowRunCommandRequest:",
		"Idempotency-Key", "expectedVersion", "Iteration14WorkflowRunStatus:",
		"enum: [queued, running, succeeded, failed, cancelled]",
		"inputPayload", "outputPayload", "errorCode", "errorMessage", "errorDetails", "configurationSnapshot",
		"error:", "code:", "message:", "details:", "request_id:",
		"WorkflowRunRetryRequest:", "useCurrentConfiguration:", "inputOverride:",
		"enum: [manual, retry, system, api]", "activeRuns:", "recentFailedRuns:", "lastRunAt:",
	} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("Iteration 14 schema or error wire fragment missing: %s", fragment)
		}
	}

	createStart := strings.Index(text, "    CreateWorkflowRunRequest:")
	commandStart := strings.Index(text, "    WorkflowRunCommandRequest:")
	if createStart < 0 || commandStart < 0 || createStart >= commandStart { t.Fatal("workflow run request schemas missing or unordered") }
	createSchema := text[createStart:commandStart]
	if strings.Contains(createSchema, "expectedVersion") || strings.Contains(createSchema, "expected_version") { t.Fatal("CreateWorkflowRunRequest must not contain expectedVersion") }
	if !strings.Contains(text, "Version of the WorkflowRun named by runId.") || !strings.Contains(text, "Version of the original WorkflowRun named by runId.") { t.Fatal("run command version targets must be explicit") }
	if strings.Contains(text, "requestId") || strings.Contains(text, "workflow_center") { t.Fatal("Iteration 14 must use request_id and frozen trigger sources") }
	for _, fragment := range []string{"inclusive lower bound for WorkflowRun.createdAt", "Exact displayed run number match", "contains match on runNumber only", "WorkflowRun status snapshot after this event is written"} {
		if !strings.Contains(text, fragment) { t.Fatalf("missing frozen Iteration 14 semantics: %s", fragment) }
	}
}

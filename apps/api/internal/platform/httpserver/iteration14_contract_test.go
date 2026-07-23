package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIteration14FrozenWorkflowRuntimeContract(t *testing.T) {
	contract, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "packages", "contracts", "openapi", "openapi.yaml"))
	if err != nil { t.Fatal(err) }
	text := string(contract)

	operationIDs := map[string]struct{}{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "operationId: ") { continue }
		operationID := strings.TrimSpace(strings.TrimPrefix(line, "operationId: "))
		if _, exists := operationIDs[operationID]; exists { t.Fatalf("OpenAPI operationId must be unique: %s", operationID) }
		operationIDs[operationID] = struct{}{}
	}

	pathBlock := func(path string) string {
		t.Helper()
		start := strings.Index(text, "  "+path+"\n")
		if start < 0 { t.Fatalf("OpenAPI path missing: %s", path) }
		rest := text[start+1:]
		if end := strings.Index(rest, "\n  /api/"); end >= 0 { return rest[:end] }
		return rest
	}
	assertRoute := func(path, operationID, schema string) {
		t.Helper()
		block := pathBlock(path)
		if strings.Count(block, "operationId: "+operationID) != 1 { t.Fatalf("%s must own operationId %s exactly once", path, operationID) }
		if !strings.Contains(block, "$ref: \"#/components/schemas/"+schema+"\"") { t.Fatalf("%s must reference %s", path, schema) }
	}

	assertRoute("/api/v1/workflow-runs:", "listWorkflowRuns", "Iteration14WorkflowRunListEnvelope")
	assertRoute("/api/v1/workflow-runs:", "createWorkflowRun", "CreateWorkflowRunRequest")
	assertRoute("/api/v1/workflow-runs/{runId}:", "getWorkflowRunDetail", "Iteration14WorkflowRunEnvelope")
	assertRoute("/api/v1/workflow-runs/{runId}/events:", "listWorkflowRunEvents", "WorkflowRunEventListEnvelope")
	assertRoute("/api/v1/workflow-runs/{runId}/retries:", "retryWorkflowRun", "WorkflowRunRetryRequest")
	assertRoute("/api/v1/workflow-runs/{runId}/cancel:", "cancelWorkflowRun", "WorkflowRunCommandRequest")
	assertRoute("/api/v1/projects/{projectId}/workflow-run-summary:", "getProjectWorkflowRunSummary", "ProjectWorkflowRunSummaryEnvelope")
	assertRoute("/api/v1/content-workflow-runs:", "listContentWorkflowRuns", "GlobalWorkflowRunListEnvelope")
	assertRoute("/api/v1/content-workflow-runs/{workflowRunId}:", "getContentWorkflowRun", "WorkflowRunDetailEnvelope")

	runtimeDetail := pathBlock("/api/v1/workflow-runs/{runId}:")
	for _, forbidden := range []string{"provider mock", "content_mock_rewrite", "ContentItem/v1/ReviewReport", "target v2", "idempotency key", "fingerprint", "rewrite run"} {
		if strings.Contains(strings.ToLower(runtimeDetail), strings.ToLower(forbidden)) { t.Fatalf("Runtime detail retains legacy rewrite description: %s", forbidden) }
	}
	for _, deferred := range []string{"verifyWorkflowConnection", "enableWorkflowConnection", "disableWorkflowConnection", "verifyWorkflowConfiguration", "enableWorkflowConfiguration", "disableWorkflowConfiguration"} {
		if _, exists := operationIDs[deferred]; exists { t.Fatalf("deferred operation remains active: %s", deferred) }
	}

	triggerStart := strings.Index(text, "    WorkflowRunTriggerSource:\n")
	if triggerStart < 0 { t.Fatal("WorkflowRunTriggerSource schema missing") }
	triggerRest := text[triggerStart+len("    WorkflowRunTriggerSource:\n"):]
	triggerEnd := strings.Index(triggerRest, "\n    Iteration14WorkflowRun:")
	if triggerEnd < 0 { t.Fatal("WorkflowRunTriggerSource schema is incomplete") }
	if got := triggerRest[:triggerEnd]; !strings.Contains(got, "enum: [manual, retry, system, api]") { t.Fatalf("unexpected triggerSource schema: %s", got) }
	if !strings.Contains(text, "request_id:") || strings.Contains(text, "requestId") { t.Fatal("ErrorEnvelope must use request_id only") }
	if !strings.Contains(text, "GlobalWorkflowRunListEnvelope:") || !strings.Contains(text, "WorkflowRunDetailEnvelope:") || !strings.Contains(text, "Iteration14WorkflowRunEnvelope:") { t.Fatal("legacy and Runtime WorkflowRun schemas must remain distinct") }

	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "docs", "development-inputs", "p1", "iterations", "iteration-14-workflow-run-runtime", "route-migration.md"))
	if err != nil { t.Fatal(err) }
	for _, fragment := range []string{"workflow_run_records", "workflow_run_events", "workflow_runs", "/api/v1/content-workflow-runs", "global-lite", "project-works"} {
		if !strings.Contains(string(migration), fragment) { t.Fatalf("route migration mapping missing %s", fragment) }
	}
}

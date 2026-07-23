package httpserver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIteration14FrozenWorkflowRuntimeContract(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	read := func(parts ...string) string {
		t.Helper()
		path := filepath.Join(append([]string{root}, parts...)...)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	contract := read("packages", "contracts", "openapi", "openapi.yaml")

	operationIDs := map[string]struct{}{}
	for _, line := range strings.Split(contract, "\n") {
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
	pathBlock := func(path string) string {
		t.Helper()
		start := strings.Index(contract, "  "+path+"\n")
		if start < 0 {
			t.Fatalf("OpenAPI path missing: %s", path)
		}
		rest := contract[start+1:]
		if end := strings.Index(rest, "\n  /api/"); end >= 0 {
			return rest[:end]
		}
		return rest
	}
	schemaBlock := func(name string) string {
		t.Helper()
		start := strings.Index(contract, "    "+name+":\n")
		if start < 0 {
			t.Fatalf("OpenAPI schema missing: %s", name)
		}
		lines := strings.Split(contract[start:], "\n")
		for index, line := range lines[1:] {
			if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
				return strings.Join(lines[:index+1], "\n")
			}
		}
		return strings.Join(lines, "\n")
	}
	assertRoute := func(path, operationID, schema string) {
		t.Helper()
		block := pathBlock(path)
		if strings.Count(block, "operationId: "+operationID) != 1 {
			t.Fatalf("%s must own operationId %s exactly once", path, operationID)
		}
		if !strings.Contains(block, "$ref: \"#/components/schemas/"+schema+"\"") {
			t.Fatalf("%s must reference %s", path, schema)
		}
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

	for _, deferred := range []string{"verifyWorkflowConnection", "enableWorkflowConnection", "disableWorkflowConnection", "verifyWorkflowConfiguration", "enableWorkflowConfiguration", "disableWorkflowConfiguration"} {
		if _, exists := operationIDs[deferred]; exists {
			t.Fatalf("deferred operation remains active: %s", deferred)
		}
	}
	for _, schema := range []string{"WorkflowConnection", "WorkflowConfiguration"} {
		block := schemaBlock(schema)
		for _, fragment := range []string{"integrationStatus:", "enabled:", "Reserved future", "not a project-binding or WorkflowRun-creation gate"} {
			if !strings.Contains(block, fragment) {
				t.Fatalf("%s must describe %q in its own schema block", schema, fragment)
			}
		}
	}
	createRun := pathBlock("/api/v1/workflow-runs:")
	for _, fragment := range []string{"queued run", "WorkflowConfiguration", "WorkflowConnection", "not prerequisites", "does not trigger external execution"} {
		if !strings.Contains(createRun, fragment) {
			t.Fatalf("create WorkflowRun description missing %q", fragment)
		}
	}
	trigger := schemaBlock("WorkflowRunTriggerSource")
	if !strings.Contains(trigger, "enum: [manual, retry, system, api]") {
		t.Fatalf("unexpected triggerSource schema: %s", trigger)
	}
	if !strings.Contains(contract, "request_id:") || strings.Contains(contract, "requestId") {
		t.Fatal("ErrorEnvelope must use request_id only")
	}

	docs := []string{"iteration-plan.md", "api-scope.yaml", "acceptance.md", "closed-loop.md", "data-model.md", "route-migration.md", "ui-scope.md"}
	for _, name := range docs {
		content := read("docs", "development-inputs", "p1", "iterations", "iteration-14-workflow-run-runtime", name)
		if !strings.Contains(content, "frozen_cf_14_01_r3") {
			t.Fatalf("%s must carry the R3 frozen state", name)
		}
	}
	scope := read("docs", "development-inputs", "p1", "iterations", "iteration-14-workflow-run-runtime", "ui-scope.md")
	for _, forbidden := range []string{"本迭代激活 n8n", "当前激活 n8n"} {
		if strings.Contains(scope, forbidden) {
			t.Fatalf("ui scope retains current integration claim: %s", forbidden)
		}
	}
	manifest := read("docs", "development-inputs", "p1", "iterations", "iteration-14-workflow-run-runtime", "ui-manifest.json")
	var manifestValue map[string]any
	if err := json.Unmarshal([]byte(manifest), &manifestValue); err != nil {
		t.Fatalf("ui manifest must be valid JSON: %v", err)
	}
	if manifestValue["contractFreeze"] != "CF-14-01-R3" || strings.Contains(manifest, "n8nAdapterOnly") {
		t.Fatal("ui manifest must freeze R3 without n8nAdapterOnly")
	}
	policy, ok := manifestValue["scopePolicy"].(map[string]any)
	if !ok || policy["verificationActions"] != false || policy["enableDisableActions"] != false {
		t.Fatal("ui manifest must declare verification and enable/disable actions out of scope")
	}

	migration := read("docs", "development-inputs", "p1", "iterations", "iteration-14-workflow-run-runtime", "route-migration.md")
	for _, fragment := range []string{"workflow_run_records", "workflow_run_events", "workflow_runs", "/api/v1/content-workflow-runs", "global-lite", "project-works"} {
		if !strings.Contains(migration, fragment) {
			t.Fatalf("route migration mapping missing %s", fragment)
		}
	}
}

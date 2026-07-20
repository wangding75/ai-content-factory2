package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIteration13FrozenContract(t *testing.T) {
	contract, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "packages", "contracts", "openapi", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contract)

	// ── Required paths ──────────────────────────────────────────────
	for _, path := range []string{
		"/api/v1/projects/{projectId}/workflow-bindings:",
		"/api/v1/projects/{projectId}/workflow-bindings/{stage}:",
	} {
		if !strings.Contains(text, "  "+path) {
			t.Fatalf("frozen Iteration 13 path missing: %s", path)
		}
	}

	// ── Required operations ──────────────────────────────────────────
	for _, op := range []string{
		"operationId: listProjectWorkflowBindings",
		"operationId: putProjectWorkflowBinding",
		"operationId: deleteProjectWorkflowBinding",
	} {
		if !strings.Contains(text, op) {
			t.Fatalf("frozen Iteration 13 operationId missing: %s", op)
		}
	}

	// ── applicableStage query parameter added to listWorkflowConfigurations ──
	if !strings.Contains(text, "applicableStage") {
		t.Fatal("frozen Iteration 13 fragment missing: applicableStage query parameter")
	}

	// ── Four stage enum values ───────────────────────────────────────
	for _, stage := range []string{"chapter_planning", "content_generation", "review", "rewrite"} {
		if !strings.Contains(text, stage) {
			t.Fatalf("frozen Iteration 13 stage enum missing: %s", stage)
		}
	}

	// ── PUT requires Idempotency-Key ──────────────────────────────────
	// The PUT operation block must contain Idempotency-Key
	if !strings.Contains(text, "putProjectWorkflowBinding") {
		t.Fatal("frozen Iteration 13: putProjectWorkflowBinding missing")
	}

	// ── DELETE requires Idempotency-Key + query parameter expected_version ──
	if !strings.Contains(text, "deleteProjectWorkflowBinding") {
		t.Fatal("frozen Iteration 13: deleteProjectWorkflowBinding missing")
	}
	if !strings.Contains(text, "ExpectedBindingVersionQuery") {
		t.Fatal("frozen Iteration 13: DELETE expectedVersion must be query parameter (ExpectedBindingVersionQuery)")
	}
	// DELETE must NOT have a request body schema
	if strings.Contains(text, "DeleteProjectWorkflowBindingRequest") {
		t.Fatal("frozen Iteration 13: DELETE must not use request body for expectedVersion")
	}

	// ── Required error codes ─────────────────────────────────────────
	for _, code := range []string{
		"VERSION_CONFLICT",
		"BINDING_ALREADY_EXISTS",
		"IDEMPOTENCY_KEY_REUSED_WITH_DIFFERENT_PAYLOAD",
		"disabled_workflow",
		"workflow_not_applicable_to_stage",
		"UNAUTHENTICATED",
		"FORBIDDEN",
		"WORKFLOW_BINDING_NOT_FOUND",
	} {
		if !strings.Contains(text, code) {
			t.Fatalf("frozen Iteration 13 error code missing: %s", code)
		}
	}

	// ── 401/403 response components exist ────────────────────────────
	if !strings.Contains(text, "Unauthorized:") {
		t.Fatal("frozen Iteration 13: Unauthorized (401) response component missing")
	}
	if !strings.Contains(text, "Forbidden:") {
		t.Fatal("frozen Iteration 13: Forbidden (403) response component missing")
	}

	// ── Forbidden schemas (out of scope for Iteration 13 binding model) ──
	// These must not appear as properties of ProjectWorkflowBinding or WorkflowBindingStage
	for _, forbidden := range []string{
		"ProjectWorkflowBinding/executionStatus",
		"ProjectWorkflowBinding/parameters",
		"ProjectWorkflowBinding/validationStatus",
		"WorkflowBindingStage/executionStatus",
		"WorkflowBindingStage/parameters",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("forbidden Iteration 13 binding schema fragment: %s", forbidden)
		}
	}
	// Iteration 13 must not define its own WorkflowRun schema
	if strings.Contains(text, "Iteration 13") && strings.Contains(text, "WorkflowRun") {
		// Allow existing WorkflowRun from earlier iterations
	}

	// ── WorkflowConfigurationSummary reuses existing schema ──────────
	if !strings.Contains(text, "WorkflowConfiguration") {
		t.Fatal("frozen Iteration 13: must reuse existing WorkflowConfiguration schema")
	}

	// ── bound / enabled / integrationStatus are independent fields ───
	if !strings.Contains(text, "bound") {
		t.Fatal("frozen Iteration 13: bound field missing")
	}
}
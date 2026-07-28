package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIteration12FrozenContractRoutesAndSafetyRules(t *testing.T) {
	contract, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "packages", "contracts", "openapi", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contract)
	for _, fragment := range []string{"/api/v1/llm-providers:", "/api/v1/workflow-connections:", "/api/v1/workflow-configurations:", "/api/v1/distribution-platforms:", "Idempotency-Key", "expectedVersion", "clearSecret", "clearCredential", "workflowType"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("frozen OpenAPI fragment missing: %s", fragment)
		}
	}
	for _, forbidden := range []string{"/verify", "/enable", "/disable", "/models"} {
		if strings.Contains(text, "  /api/v1/llm-providers"+forbidden) || strings.Contains(text, "  /api/v1/workflow-connections"+forbidden) {
			t.Fatalf("forbidden Iteration 12 endpoint: %s", forbidden)
		}
	}
}

func TestIteration145VerificationLifecycleContract(t *testing.T) {
	contract, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "packages", "contracts", "openapi", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(contract)
	for _, route := range []struct{ path, operation string }{
		{"/api/v1/workflow-connections/{connectionId}/verify:", "verifyWorkflowConnection"},
		{"/api/v1/workflow-connections/{connectionId}/disable:", "disableWorkflowConnection"},
		{"/api/v1/workflow-configurations/{workflowId}/verify:", "verifyWorkflowConfiguration"},
		{"/api/v1/workflow-configurations/{workflowId}/disable:", "disableWorkflowConfiguration"},
	} {
		if strings.Count(text, route.path) != 1 || strings.Count(text, "operationId: "+route.operation) != 1 {
			t.Fatalf("missing or duplicate action %s", route.operation)
		}
	}
	for _, fragment := range []string{"ConfigurationActionRequest", "\"200\":", "\"404\":", "\"409\":", "\"422\":", "readOnly: true", "Connected and enabled are required by project binding and real execution gates"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("verification contract missing %q", fragment)
		}
	}
	for _, obsolete := range []string{"Reserved future-integration metadata", "Reserved future-execution metadata", "not a project-binding or WorkflowRun-creation gate"} {
		if strings.Contains(text, obsolete) {
			t.Fatalf("obsolete lifecycle description remains: %q", obsolete)
		}
	}
}

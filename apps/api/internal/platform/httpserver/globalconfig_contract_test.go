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

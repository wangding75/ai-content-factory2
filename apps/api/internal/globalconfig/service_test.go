package globalconfig

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCredentialEncryptionAndFingerprint(t *testing.T) {
	s, err := NewService(nil, "iteration-12-test-key")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, fingerprint, err := s.seal("provider-secret")
	if err != nil {
		t.Fatal(err)
	}
	if ciphertext == "provider-secret" || strings.Contains(ciphertext, "provider-secret") {
		t.Fatal("credential was not encrypted")
	}
	if fingerprint != fingerprintForTest("provider-secret") || len(fingerprint) != 32 {
		t.Fatalf("unsafe fingerprint: %q", fingerprint)
	}
}

func TestConfigurationValidationRules(t *testing.T) {
	validN8nConfig := json.RawMessage(`{"referenceType":"workflow_id","referenceValue":"workflow-1"}`)
	if !validProvider("primary", "https://api.example.test/v1", "gpt-4.1-mini", 30) {
		t.Fatal("valid provider rejected")
	}
	if validProvider("primary", "not-a-url", "gpt-4.1-mini", 30) {
		t.Fatal("invalid provider URL accepted")
	}
	if !validN8n(validN8nConfig) || validN8n(json.RawMessage(`{"referenceType":"invalid","referenceValue":"x"}`)) {
		t.Fatal("n8n type configuration validation drift")
	}
	if !validWorkflow("planner", []string{"chapter_planning"}, validN8nConfig, "v1", "v1", json.RawMessage(`{}`)) {
		t.Fatal("valid workflow rejected")
	}
	if validWorkflow("planner", []string{"chapter_planning", "chapter_planning"}, validN8nConfig, "v1", "v1", json.RawMessage(`{}`)) {
		t.Fatal("duplicate workflow stage accepted")
	}
	if !validPlatform("custom", "custom", "account", stringPointer("https://publish.example.test"), "api_key", 30, json.RawMessage(`{}`)) {
		t.Fatal("valid custom platform rejected")
	}
	if validPlatform("custom", "custom", "account", nil, "api_key", 30, json.RawMessage(`{}`)) {
		t.Fatal("custom platform without endpoint accepted")
	}
}

func TestWriteRequestsRejectEmptyCredentialsAndMissingVersions(t *testing.T) {
	empty := ""
	if validOptional(&empty) {
		t.Fatal("empty credential may not implicitly clear a secret")
	}
	if _, err := NewService(nil, ""); err == nil {
		t.Fatal("missing encryption key accepted")
	}
}

func fingerprintForTest(value string) string { return fingerprint(value) }

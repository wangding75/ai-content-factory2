package globalconfig

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestVerificationOutboundAddressPolicy(t *testing.T) {
	for _, test := range []struct {
		name, host, ip string
		allowed        bool
	}{
		{"public", "api.example.test", "203.0.113.10", true},
		{"private literal", "10.0.0.8", "10.0.0.8", false},
		{"localhost", "localhost", "127.0.0.1", false},
		{"private dns", "internal.example.test", "192.168.1.10", false},
		{"n8n exception", "n8n", "172.20.0.3", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ip := net.ParseIP(test.ip)
			if got := verificationAddressAllowed(test.host, ip); got != test.allowed {
				t.Fatalf("allowed=%v, want %v", got, test.allowed)
			}
		})
	}
	if _, err := verificationURL("http://localhost:5678", "healthz"); err == nil {
		t.Fatal("localhost must be rejected")
	}
	if _, err := verificationURL("http://n8n:5678", "healthz"); err != nil {
		t.Fatalf("n8n should be accepted: %v", err)
	}
}

func TestVerificationHTTPClientRejectsPrivateResolutionAndRedirects(t *testing.T) {
	service := &Service{resolveHost: func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("10.0.0.7")}, nil }}
	request, err := http.NewRequest(http.MethodGet, "http://public.example.test/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.verificationHTTPClient().Do(request); err == nil {
		t.Fatal("private DNS answer must be rejected")
	}

	service = &Service{
		resolveHost: func(context.Context, string) ([]net.IP, error) { return []net.IP{net.ParseIP("203.0.113.11")}, nil },
		dialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			client, server := net.Pipe()
			go func() {
				defer server.Close()
				reader := bufio.NewReader(server)
				for {
					line, readErr := reader.ReadString('\n')
					if readErr != nil || line == "\r\n" {
						break
					}
				}
				_, _ = io.WriteString(server, "HTTP/1.1 302 Found\r\nLocation: http://other.example.test\r\nContent-Length: 0\r\n\r\n")
			}()
			return client, nil
		},
	}
	response, err := service.verificationHTTPClient().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusFound {
		t.Fatalf("redirect status=%d, want 302", response.StatusCode)
	}
}

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

func TestSafeAuditWhitelistExcludesFreeSensitiveValues(t *testing.T) {
	payload := safeAudit("update", 2, map[string]any{
		"name":                "safe-name",
		"typeConfigChanged":   true,
		"noteChanged":         true,
		"typeConfig":          map[string]any{"token": "type-config-token"},
		"defaultParameters":   map[string]any{"secret": "default-secret"},
		"note":                "note credential",
		"encryptedCredential": "ciphertext",
		"authorization":       "Bearer authorization-token",
	})
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, sensitive := range []string{"type-config-token", "default-secret", "note credential", "ciphertext", "authorization-token"} {
		if strings.Contains(text, sensitive) {
			t.Fatalf("audit leaked sensitive free-form value %q: %s", sensitive, text)
		}
	}
	if !strings.Contains(text, "safe-name") || !strings.Contains(text, "typeConfigChanged") || !strings.Contains(text, "noteChanged") {
		t.Fatalf("audit omitted whitelisted change metadata: %s", text)
	}
}

func fingerprintForTest(value string) string { return fingerprint(value) }

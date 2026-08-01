package safety

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSecretCanaryIsRemovedFromNestedOutputShapes(t *testing.T) {
	const canary = "I19-SECRET-CANARY"
	value := json.RawMessage(`{"dto":{"apiKey":"I19-SECRET-CANARY"},"error":{"details":[{"authorization":"Bearer I19-SECRET-CANARY"}]},"audit":{"credential":"I19-SECRET-CANARY"},"event":{"nested":{"cookie":"I19-SECRET-CANARY"}},"snapshot":{"safe":"kept","private_key":"I19-SECRET-CANARY"}}`)
	redacted := RedactJSON(value)
	if strings.Contains(string(redacted), canary) {
		t.Fatalf("canary leaked: %s", redacted)
	}
	for _, key := range []string{"dto", "error", "audit", "event", "snapshot", "kept"} {
		if !strings.Contains(string(redacted), key) {
			t.Fatalf("safe structure lost %q: %s", key, redacted)
		}
	}
	if got := Fingerprint(canary); got == canary || len(got) != 32 {
		t.Fatalf("fingerprint=%q", got)
	}
}

// Package safety centralizes sensitive-field redaction and non-reversible
// fingerprints for DTOs, errors, audits, events and immutable snapshots.
package safety

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

var sensitiveNames = map[string]struct{}{
	"secret": {}, "apikey": {}, "password": {}, "credential": {}, "credentials": {},
	"authorization": {}, "cookie": {}, "setcookie": {}, "token": {}, "accesstoken": {},
	"refreshtoken": {}, "clientsecret": {}, "privatekey": {}, "idempotencykey": {},
	"webhooksecret": {}, "authtoken": {}, "xapikey": {},
}

func RedactJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 || !json.Valid(value) {
		return json.RawMessage(`{}`)
	}
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return json.RawMessage(`{}`)
	}
	RedactValue(decoded)
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}

func RedactValue(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, sensitive := sensitiveNames[normalizedKey(key)]; sensitive {
				typed[key] = "[REDACTED]"
			} else {
				RedactValue(child)
			}
		}
	case []any:
		for _, child := range typed {
			RedactValue(child)
		}
	}
}

func Fingerprint(value string) string {
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])[:32]
}

func normalizedKey(value string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(value))
}

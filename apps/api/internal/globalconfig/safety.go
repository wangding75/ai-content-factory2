package globalconfig

import (
	"encoding/json"

	"github.com/local/ai-content-factory/apps/api/internal/platform/safety"
)

func sanitizeJSON(value json.RawMessage) json.RawMessage {
	return safety.RedactJSON(value)
}

func secureFingerprint(value string) string {
	return safety.Fingerprint(value)
}

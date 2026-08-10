package globalconfig

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/platform/safehttp"
)

// providerModelResponse and n8nWorkflowResponse intentionally model only the
// fields used by this service. Raw upstream bodies are never persisted or
// surfaced through an error.
type providerModelResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type n8nWorkflowResponse struct {
	ID        string `json:"id"`
	Active    bool   `json:"active"`
	VersionID string `json:"versionId"`
	Nodes     []struct {
		Type       string `json:"type"`
		Disabled   bool   `json:"disabled"`
		Parameters struct {
			Path string `json:"path"`
		} `json:"parameters"`
	} `json:"nodes"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

type workflowReferenceCheck struct {
	Exists      bool
	Active      bool
	Stages      []string
	WorkflowID  string
	Revision    string
	WebhookPath string
}

func (s *Service) unseal(ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", ErrVerification
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(raw) < gcm.NonceSize() {
		return "", ErrVerification
	}
	plain, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return "", ErrVerification
	}
	return string(plain), nil
}

func (s *Service) providerSecret(ctx context.Context, id uuid.UUID) (string, error) {
	var encrypted *string
	if err := s.pool.QueryRow(ctx, "SELECT encrypted_secret FROM llm_provider_configurations WHERE id=$1", id).Scan(&encrypted); err != nil {
		return "", notFound(err)
	}
	if encrypted == nil || *encrypted == "" {
		return "", ErrVerification
	}
	return s.unseal(*encrypted)
}

func (s *Service) connectionCredential(ctx context.Context, id uuid.UUID) (string, error) {
	var encrypted *string
	if err := s.pool.QueryRow(ctx, "SELECT encrypted_credential FROM workflow_connections WHERE id=$1", id).Scan(&encrypted); err != nil {
		return "", notFound(err)
	}
	if encrypted == nil || *encrypted == "" {
		// A connection created before Iteration 19 may rely on an unauthenticated
		// local n8n instance. The request remains safe; a protected instance will
		// return the stable authentication failure code.
		return "", nil
	}
	return s.unseal(*encrypted)
}

// RuntimeConnectionCredential confines decryption to the outbound runtime call.
// Callers must use the value immediately and must never persist or log it.
func (s *Service) RuntimeConnectionCredential(ctx context.Context, id uuid.UUID) (string, error) {
	return s.connectionCredential(ctx, id)
}

func (s *Service) integrationURL(base string, suffix string) (string, error) {
	policy := integrationOutboundPolicy()
	if s != nil {
		policy = s.credentialOutboundPolicy()
	}
	u, err := safehttp.NormalizeURL(base, policy)
	if err != nil {
		return "", err
	}
	rel, err := url.Parse(suffix)
	if err != nil || rel.IsAbs() || strings.Contains(suffix, "\\") {
		return "", &safehttp.Error{Code: safehttp.CodeUnsafeBaseURL, Message: "The integration URL is not allowed."}
	}
	basePath := strings.Trim(u.Path, "/")
	relPath := strings.Trim(rel.Path, "/")
	// OpenAI-compatible providers commonly accept either https://host or
	// https://host/v1 as their configured base URL. The request suffix is
	// canonicalized to v1/models, so do not duplicate the version segment.
	if (basePath == "v1" || strings.HasSuffix(basePath, "/v1")) && strings.HasPrefix(relPath, "v1/") {
		relPath = strings.TrimPrefix(relPath, "v1/")
	}
	switch {
	case basePath == "":
		u.Path = "/" + relPath
	case relPath == "":
		u.Path = "/" + basePath
	default:
		u.Path = "/" + basePath + "/" + relPath
	}
	u.RawQuery = rel.RawQuery
	return u.String(), nil
}

func (s *Service) integrationRequest(ctx context.Context, baseURL, suffix, credential, header string, timeout int, target any) error {
	return s.integrationRequestWithNotFoundCode(ctx, baseURL, suffix, credential, header, timeout, target, "workflow_reference_not_found")
}

func (s *Service) integrationRequestWithNotFoundCode(ctx context.Context, baseURL, suffix, credential, header string, timeout int, target any, notFoundCode string) error {
	endpoint, err := s.integrationURL(baseURL, suffix)
	if err != nil {
		return err
	}
	if timeout < 5 || timeout > 300 {
		return ErrVerification
	}
	requestContext, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestContext, http.MethodGet, endpoint, nil)
	if err != nil {
		return ErrVerification
	}
	if credential != "" {
		req.Header.Set(header, credential)
	}
	client := safehttp.New(s.outboundPolicy())
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if statusErr := integrationResponseError(response.StatusCode, notFoundCode); statusErr != nil {
		return statusErr
	}
	body, err := client.ReadBody(response)
	if err != nil {
		return err
	}
	if target != nil && json.Unmarshal(body, target) != nil {
		return &safehttp.Error{Code: safehttp.CodeUpstreamUnavailable, Message: "The integration response was invalid.", Retryable: false}
	}
	return nil
}

func integrationResponseError(status int, notFoundCode string) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &safehttp.Error{Code: "upstream_authentication_failed", Message: "The integration credentials were rejected.", Retryable: false}
	case status == http.StatusNotFound:
		return &safehttp.Error{Code: notFoundCode, Message: "The integration endpoint was not found.", Retryable: false}
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return &safehttp.Error{Code: safehttp.CodeUpstreamTimeout, Message: "The integration request timed out.", Retryable: true}
	case status == http.StatusTooManyRequests:
		return &safehttp.Error{Code: "upstream_rate_limited", Message: "The integration service is temporarily rate limited.", Retryable: true}
	case status >= 400 && status < 500:
		return &safehttp.Error{Code: "upstream_request_rejected", Message: "The integration service rejected the request.", Retryable: false}
	case status < 200 || status >= 300:
		return &safehttp.Error{Code: safehttp.CodeUpstreamUnavailable, Message: "The integration service returned an unsuccessful response.", Retryable: status >= 500}
	default:
		return nil
	}
}

func (s *Service) discoverModels(ctx context.Context, provider Provider) ([]string, error) {
	secret, err := s.providerSecret(ctx, provider.ID)
	if err != nil {
		return nil, err
	}
	var payload providerModelResponse
	if err = s.integrationRequestWithNotFoundCode(ctx, provider.BaseURL, "v1/models", "Bearer "+secret, "Authorization", provider.TimeoutSeconds, &payload, "llm_upstream_not_found"); err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		model := strings.TrimSpace(item.ID)
		if model == "" || len(model) > 200 {
			continue
		}
		if _, exists := seen[model]; !exists {
			seen[model] = struct{}{}
			models = append(models, model)
		}
	}
	if len(models) == 0 {
		return nil, &safehttp.Error{Code: "model_unavailable", Message: "No usable models were returned by the integration.", Retryable: false}
	}
	return models, nil
}

func validateProviderModels(defaultModel, optionalModel string, models []string) error {
	available := make(map[string]struct{}, len(models))
	for _, model := range models {
		available[model] = struct{}{}
	}
	if _, ok := available[strings.TrimSpace(defaultModel)]; !ok {
		return &safehttp.Error{Code: "model_unavailable", Message: "The configured default model is unavailable.", Retryable: false}
	}
	if candidate := strings.TrimSpace(optionalModel); candidate != "" {
		if _, ok := available[candidate]; !ok {
			return &safehttp.Error{Code: "model_unavailable", Message: "The requested model is unavailable.", Retryable: false}
		}
	}
	return nil
}

func (s *Service) n8nWorkflow(ctx context.Context, connection Connection, reference string) (workflowReferenceCheck, error) {
	credential, err := s.connectionCredential(ctx, connection.ID)
	if err != nil {
		return workflowReferenceCheck{}, err
	}
	var payload n8nWorkflowResponse
	if err = s.integrationRequest(ctx, connection.BaseURL, "api/v1/workflows/"+url.PathEscape(reference), credential, "X-N8N-API-KEY", connection.TimeoutSeconds, &payload); err != nil {
		return workflowReferenceCheck{}, err
	}
	check := workflowReferenceCheck{Exists: strings.TrimSpace(payload.ID) != "", Active: payload.Active}
	check.WorkflowID = strings.TrimSpace(payload.ID)
	check.Revision = strings.TrimSpace(payload.VersionID)
	for _, node := range payload.Nodes {
		if node.Type != "n8n-nodes-base.webhook" || node.Disabled || strings.TrimSpace(node.Parameters.Path) == "" {
			continue
		}
		if check.WebhookPath != "" {
			return workflowReferenceCheck{}, &safehttp.Error{Code: "workflow_reference_not_found", Message: "The workflow does not have one unambiguous production webhook.", Retryable: false}
		}
		check.WebhookPath = strings.TrimSpace(node.Parameters.Path)
	}
	if !check.Exists || check.WebhookPath == "" {
		return workflowReferenceCheck{}, &safehttp.Error{Code: "workflow_reference_not_found", Message: "The workflow does not have a production webhook.", Retryable: false}
	}
	if check.Revision == "" {
		digest := sha256.Sum256([]byte(check.WorkflowID + "\x00" + check.WebhookPath))
		check.Revision = hex.EncodeToString(digest[:])
	}
	for _, tag := range payload.Tags {
		name := strings.TrimSpace(tag.Name)
		if strings.HasPrefix(name, "acf-stage:") {
			check.Stages = append(check.Stages, strings.TrimPrefix(name, "acf-stage:"))
		}
	}
	return check, nil
}

func validationOutcome(err error, details json.RawMessage) ValidationOutcome {
	if err == nil {
		return ValidationOutcome{Success: true, Details: details}
	}
	code := safehttp.ErrorCode(err)
	message := "The integration could not be verified."
	var safeErr *safehttp.Error
	if errors.As(err, &safeErr) {
		message = safeErr.Message
	}
	if errors.Is(err, ErrVerification) {
		code = "configuration_verification_failed"
	}
	return ValidationOutcome{Code: code, Message: message, Details: details}
}

func (s *Service) updateModelCatalogue(ctx context.Context, tx pgx.Tx, providerID uuid.UUID, models []string) error {
	if providerID == uuid.Nil {
		return ErrValidation
	}
	seen := make([]string, 0, len(models))
	for _, key := range models {
		key = strings.TrimSpace(key)
		if key == "" || len(key) > 200 {
			continue
		}
		seen = append(seen, key)
		if _, err := tx.Exec(ctx, `INSERT INTO llm_provider_models(id,provider_id,model_key,source,availability,last_seen_at)
			VALUES($1,$2,$3,'discovered','available',NOW())
			ON CONFLICT(provider_id,model_key) DO UPDATE SET source='discovered',availability='available',last_seen_at=NOW(),updated_at=NOW()`, uuid.New(), providerID, key); err != nil {
			return err
		}
	}
	if len(seen) == 0 {
		return ErrVerification
	}
	if _, err := tx.Exec(ctx, "UPDATE llm_provider_models SET availability='unavailable',updated_at=NOW() WHERE provider_id=$1 AND NOT (model_key = ANY($2::text[]))", providerID, seen); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, "UPDATE llm_provider_configurations SET model_catalog_updated_at=NOW(),updated_at=NOW() WHERE id=$1", providerID)
	return err
}

func (s *Service) providerModelAvailable(ctx context.Context, providerID uuid.UUID, model string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM llm_provider_models WHERE provider_id=$1 AND model_key=$2 AND availability='available')", providerID, model).Scan(&exists)
	return exists, err
}

func (s *Service) providerModelAvailableTx(ctx context.Context, tx pgx.Tx, providerID uuid.UUID, model string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM llm_provider_models WHERE provider_id=$1 AND model_key=$2 AND availability='available')", providerID, model).Scan(&exists)
	return exists, err
}

func safeChecks(values ...map[string]any) json.RawMessage {
	body, err := json.Marshal(map[string]any{"checks": values})
	if err != nil {
		return json.RawMessage(`{"checks":[]}`)
	}
	return body
}

func check(name string, passed bool, code string) map[string]any {
	status := "passed"
	if !passed {
		status = "failed"
	}
	result := map[string]any{"code": name, "status": status}
	if code != "" {
		result["errorCode"] = code
	}
	return result
}

func invalidResponse(format string, args ...any) error {
	return &safehttp.Error{Code: safehttp.CodeUpstreamUnavailable, Message: fmt.Sprintf(format, args...), Retryable: false}
}

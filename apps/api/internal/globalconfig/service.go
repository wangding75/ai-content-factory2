// Package globalconfig implements the persisted, non-networked configuration catalogue.
package globalconfig

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
	"github.com/local/ai-content-factory/apps/api/internal/platform/safehttp"
)

var (
	ErrNotFound           = errors.New("configuration not found")
	ErrValidation         = errors.New("invalid configuration")
	ErrVersionConflict    = errors.New("configuration version conflict")
	ErrIdempotency        = errors.New("idempotency key reused with different payload")
	ErrNameConflict       = errors.New("configuration name already exists")
	ErrVerification       = errors.New("integration verification failed")
	ErrConnectionNotReady = errors.New("workflow connection is not connected")
	ErrNotExecutable      = errors.New("workflow configuration is not executable")
)

type Service struct {
	pool                  *pgxpool.Pool
	key                   []byte
	environment           string
	beforeIdempotencyLock func()
	resolveHost           func(context.Context, string) ([]net.IP, error)
	dialContext           func(context.Context, string, string) (net.Conn, error)
}

func NewService(pool *pgxpool.Pool, encryptionKey string) (*Service, error) {
	b := sha256.Sum256([]byte(encryptionKey))
	if strings.TrimSpace(encryptionKey) == "" {
		return nil, errors.New("CONFIGURATION_ENCRYPTION_KEY is required")
	}
	return &Service{pool: pool, key: b[:], environment: strings.TrimSpace(os.Getenv("APP_ENV"))}, nil
}

// SetEnvironment overrides APP_ENV for tests and controlled boot wiring.
func (s *Service) SetEnvironment(environment string) {
	if s == nil {
		return
	}
	s.environment = strings.TrimSpace(environment)
}

func (s *Service) runtimeEnvironment() string {
	if s != nil && strings.TrimSpace(s.environment) != "" {
		return s.environment
	}
	return strings.TrimSpace(os.Getenv("APP_ENV"))
}

type Common struct {
	ID                uuid.UUID       `json:"id"`
	Name              string          `json:"name"`
	IntegrationStatus string          `json:"integrationStatus"`
	ValidationStatus  string          `json:"validationStatus"`
	Enabled           bool            `json:"enabled"`
	Executable        bool            `json:"executable"`
	VerifiedVersion   *int            `json:"verifiedVersion"`
	ValidationDetails json.RawMessage `json:"validationDetails"`
	LastVerifiedAt    *time.Time      `json:"lastVerifiedAt"`
	LastErrorCode     *string         `json:"lastErrorCode"`
	LastErrorMessage  *string         `json:"lastErrorMessage"`
	Version           int             `json:"version"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}
type Provider struct {
	Common
	ProviderType      string  `json:"providerType"`
	BaseURL           string  `json:"baseUrl"`
	DefaultModel      string  `json:"defaultModel"`
	TimeoutSeconds    int     `json:"timeoutSeconds"`
	HasSecret         bool    `json:"hasSecret"`
	SecretFingerprint *string `json:"secretFingerprint"`
}
type Connection struct {
	Common
	ConnectionType        string          `json:"connectionType"`
	BaseURL               string          `json:"baseUrl"`
	AuthType              string          `json:"authType"`
	TimeoutSeconds        int             `json:"timeoutSeconds"`
	TypeConfig            json.RawMessage `json:"typeConfig"`
	HasCredential         bool            `json:"hasCredential"`
	CredentialFingerprint *string         `json:"credentialFingerprint"`
	encryptedCredential   *string
	credentialReadable    bool
	ineligibilityReasons  []IneligibilityReason
}
type Workflow struct {
	Common
	IneligibilityReasons  []IneligibilityReason `json:"ineligibilityReasons"`
	ConnectionID          uuid.UUID             `json:"connectionId"`
	ConnectionName        string                `json:"connectionName"`
	ConnectionType        string                `json:"connectionType"`
	WorkflowType          string                `json:"workflowType"`
	ApplicableStages      []string              `json:"applicableStages"`
	TypeConfig            json.RawMessage       `json:"typeConfig"`
	InputContractVersion  string                `json:"inputContractVersion"`
	OutputContractVersion string                `json:"outputContractVersion"`
	DefaultParameters     json.RawMessage       `json:"defaultParameters"`
	Note                  *string               `json:"note"`
	LlmStrategy           string                `json:"llmStrategy"`
	LlmProviderID         *uuid.UUID            `json:"llmProviderId"`
	LlmModel              *string               `json:"llmModel"`
	ResolvedWebhookPath   *string               `json:"-"`
	ResolvedWorkflowID    *string               `json:"-"`
	ResolvedRevision      *string               `json:"-"`
}
type WorkflowExecutionEligibility struct {
	Workflow   Workflow
	Connection Connection
	Executable bool
	Reasons    []IneligibilityReason
}
type ProviderModel struct {
	ID           uuid.UUID  `json:"id"`
	ProviderID   uuid.UUID  `json:"providerId"`
	ModelKey     string     `json:"modelName"`
	Source       string     `json:"source"`
	Availability string     `json:"availability"`
	LastSeenAt   *time.Time `json:"lastSeenAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}
type Platform struct {
	Common
	PlatformType          string          `json:"platformType"`
	AccountIdentifier     string          `json:"accountIdentifier"`
	EndpointURL           *string         `json:"endpointUrl"`
	AuthType              string          `json:"authType"`
	TimeoutSeconds        int             `json:"timeoutSeconds"`
	TypeConfig            json.RawMessage `json:"typeConfig"`
	Note                  *string         `json:"note"`
	HasCredential         bool            `json:"hasCredential"`
	CredentialFingerprint *string         `json:"credentialFingerprint"`
}
type ListOptions struct {
	Query, Type, ConnectionID, IntegrationStatus, ApplicableStage string
	ValidationStatus, LlmStrategy string
	Enabled                                                       *bool
	Executable                                                    *bool
	Limit, Offset                                                 int
}
type ProviderCreate struct {
	Name, ProviderType, BaseURL, DefaultModel string
	TimeoutSeconds                            int
	Secret                                    *string
}
type ProviderUpdate struct {
	ExpectedVersion             int
	Name, BaseURL, DefaultModel *string
	TimeoutSeconds              *int
	Secret                      *string
	ClearSecret                 *bool
}
type ConnectionCreate struct {
	Name, ConnectionType, BaseURL, AuthType string
	TimeoutSeconds                          int
	TypeConfig                              json.RawMessage
	Credential                              *string
}
type ConnectionUpdate struct {
	ExpectedVersion         int
	Name, BaseURL, AuthType *string
	TimeoutSeconds          *int
	TypeConfig              json.RawMessage
	Credential              *string
	ClearCredential         *bool
}
type WorkflowCreate struct {
	Name                                        string
	ConnectionID                                uuid.UUID
	ApplicableStages                            []string
	TypeConfig                                  json.RawMessage
	InputContractVersion, OutputContractVersion string
	DefaultParameters                           json.RawMessage
	Note                                        *string
	LlmStrategy                                 string
	LlmProviderID                               *uuid.UUID
	LlmModel                                    *string
}
type WorkflowUpdate struct {
	ExpectedVersion                             int
	Name                                        *string
	ConnectionID                                *uuid.UUID
	ApplicableStages                            *[]string
	TypeConfig                                  json.RawMessage
	InputContractVersion, OutputContractVersion *string
	DefaultParameters                           json.RawMessage
	Note                                        **string
	LlmStrategy                                 *string
	LlmProviderID                               **uuid.UUID
	LlmModel                                    **string
}
type PlatformCreate struct {
	Name, PlatformType, AccountIdentifier string
	EndpointURL                           *string
	AuthType                              string
	TimeoutSeconds                        int
	TypeConfig                            json.RawMessage
	Note                                  *string
	Credential                            *string
}
type PlatformUpdate struct {
	ExpectedVersion         int
	Name, AccountIdentifier *string
	EndpointURL             **string
	AuthType                *string
	TimeoutSeconds          *int
	TypeConfig              json.RawMessage
	Note                    **string
	Credential              *string
	ClearCredential         *bool
}

// Type catalogue is the single source for public schemas and server validation.
// It deliberately contains no credential fields or third-party-specific inventions.
type FieldSchema struct {
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Required    bool           `json:"required"`
	Constraints map[string]any `json:"constraints,omitempty"`
}
type ProviderType struct {
	ProviderType   string        `json:"providerType"`
	DisplayName    string        `json:"displayName"`
	SupportsSecret bool          `json:"supportsSecret"`
	FieldSchemas   []FieldSchema `json:"fieldSchemas"`
}
type ConnectionType struct {
	ConnectionType string        `json:"connectionType"`
	DisplayName    string        `json:"displayName"`
	AuthTypes      []string      `json:"authTypes"`
	FieldSchemas   []FieldSchema `json:"fieldSchemas"`
}
type PlatformType struct {
	PlatformType string        `json:"platformType"`
	DisplayName  string        `json:"displayName"`
	AuthTypes    []string      `json:"authTypes"`
	FieldSchemas []FieldSchema `json:"fieldSchemas"`
}

var providerTypes = []ProviderType{{ProviderType: "openai_compatible", DisplayName: "OpenAI-compatible", SupportsSecret: true, FieldSchemas: []FieldSchema{}}}
var connectionTypes = []ConnectionType{{ConnectionType: "n8n", DisplayName: "n8n", AuthTypes: []string{"api_key"}, FieldSchemas: []FieldSchema{{Name: "referenceType", Type: "string", Required: true, Constraints: map[string]any{"enum": []string{"workflow_id", "webhook_path"}}}, {Name: "referenceValue", Type: "string", Required: true, Constraints: map[string]any{"minLength": 1, "maxLength": 512}}}}}
var platformTypes = []PlatformType{{PlatformType: "wechat_official_account", DisplayName: "WeChat Official Account", AuthTypes: []string{"api_key"}, FieldSchemas: []FieldSchema{}}, {PlatformType: "douyin", DisplayName: "Douyin", AuthTypes: []string{"oauth", "access_token"}, FieldSchemas: []FieldSchema{}}, {PlatformType: "youtube", DisplayName: "YouTube", AuthTypes: []string{"oauth", "api_key"}, FieldSchemas: []FieldSchema{}}, {PlatformType: "custom", DisplayName: "Custom", AuthTypes: []string{"api_key", "oauth", "access_token", "custom"}, FieldSchemas: []FieldSchema{}}}

func ProviderTypes() []ProviderType     { return providerTypes }
func ConnectionTypes() []ConnectionType { return connectionTypes }
func PlatformTypes() []PlatformType     { return platformTypes }
func ValidIntegrationStatus(v string) bool {
	return validValidationStatus(ValidationStatus(v))
}
func ValidType(path, value string) bool {
	for _, x := range providerTypes {
		if strings.Contains(path, "llm-providers") && x.ProviderType == value {
			return true
		}
	}
	for _, x := range connectionTypes {
		if (strings.Contains(path, "workflow-connections") || strings.Contains(path, "workflow-configurations")) && x.ConnectionType == value {
			return true
		}
	}
	for _, x := range platformTypes {
		if strings.Contains(path, "distribution-platforms") && x.PlatformType == value {
			return true
		}
	}
	return false
}
func validPlatformAuth(platform, auth string) bool {
	for _, x := range platformTypes {
		if x.PlatformType == platform {
			for _, allowed := range x.AuthTypes {
				if auth == allowed {
					return true
				}
			}
		}
	}
	return false
}

func validURL(x string) bool {
	_, err := safehttp.NormalizeURL(x, integrationOutboundPolicy())
	return err == nil
}
func validN8n(x json.RawMessage) bool {
	var v struct {
		ReferenceType  string `json:"referenceType"`
		ReferenceValue string `json:"referenceValue"`
	}
	var raw map[string]json.RawMessage
	return json.Unmarshal(x, &v) == nil && json.Unmarshal(x, &raw) == nil && len(raw) == 2 && raw["referenceType"] != nil && raw["referenceValue"] != nil && (v.ReferenceType == "workflow_id" || v.ReferenceType == "webhook_path") && strings.TrimSpace(v.ReferenceValue) != "" && len(v.ReferenceValue) <= 512
}
func (s *Service) seal(value string) (string, string, error) {
	b, e := aes.NewCipher(s.key)
	if e != nil {
		return "", "", e
	}
	g, e := cipher.NewGCM(b)
	if e != nil {
		return "", "", e
	}
	n := make([]byte, g.NonceSize())
	if _, e = io.ReadFull(rand.Reader, n); e != nil {
		return "", "", e
	}
	return base64.StdEncoding.EncodeToString(append(n, g.Seal(nil, n, []byte(value), nil)...)), fingerprint(value), nil
}
func fingerprint(v string) string {
	return secureFingerprint(v)
}
func (s *Service) CreateProvider(ctx context.Context, r ProviderCreate, key string) (Provider, error) {
	if r.ProviderType != "openai_compatible" || !validProvider(r.Name, r.BaseURL, r.DefaultModel, r.TimeoutSeconds) || !validOptional(r.Secret) {
		return Provider{}, ErrValidation
	}
	body, err := s.idempotent(ctx, "llm-provider:create", key, r, 201, func(tx pgx.Tx) (json.RawMessage, error) {
		var out Provider
		enc, fp, e := s.secret(r.Secret)
		if e != nil {
			return nil, e
		}
		row := tx.QueryRow(ctx, "INSERT INTO llm_provider_configurations(id,name,provider_type,base_url,default_model,encrypted_secret,secret_fingerprint,timeout_seconds) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING "+providerColumns, uuid.New(), r.Name, r.ProviderType, r.BaseURL, r.DefaultModel, enc, fp, r.TimeoutSeconds)
		if e = scanProvider(row, &out); e != nil {
			return nil, e
		}
		if e = s.audit(ctx, tx, "create", "llm_provider", out.ID, safeAudit("create", out.Version, map[string]any{"name": out.Name})); e != nil {
			return nil, e
		}
		return json.Marshal(out)
	})
	var out Provider
	if err == nil {
		err = json.Unmarshal(body, &out)
	}
	return out, err
}
func (s *Service) GetProvider(ctx context.Context, id uuid.UUID) (Provider, error) {
	var x Provider
	e := scanProvider(s.pool.QueryRow(ctx, "SELECT "+providerColumns+" FROM llm_provider_configurations WHERE id=$1", id), &x)
	return x, notFound(e)
}
func (s *Service) ListProviders(ctx context.Context, o ListOptions) ([]Provider, int, error) {
	// SQL pushes static filters; executable is derived after finalizeCommon, so
	// candidates are fully filtered before total/offset/limit are applied.
	q, args := where(o, "provider_type", nil)
	rows, e := s.pool.Query(ctx, "SELECT "+providerColumns+" FROM llm_provider_configurations"+q+" ORDER BY updated_at DESC,id ASC", args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	filtered := make([]Provider, 0)
	for rows.Next() {
		var x Provider
		if e = scanProvider(rows, &x); e != nil {
			return nil, 0, e
		}
		if o.ValidationStatus != "" && x.ValidationStatus != o.ValidationStatus {
			continue
		}
		if o.Executable != nil && x.Executable != *o.Executable {
			continue
		}
		filtered = append(filtered, x)
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	return paginateSlice(filtered, o.Limit, o.Offset), len(filtered), nil
}
func (s *Service) ListProviderModels(ctx context.Context, providerID uuid.UUID) ([]ProviderModel, error) {
	rows, err := s.pool.Query(ctx, "SELECT id,provider_id,model_key,source,availability,last_seen_at,created_at,updated_at FROM llm_provider_models WHERE provider_id=$1 ORDER BY model_key ASC,id ASC", providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := []ProviderModel{}
	for rows.Next() {
		var model ProviderModel
		if err = rows.Scan(&model.ID, &model.ProviderID, &model.ModelKey, &model.Source, &model.Availability, &model.LastSeenAt, &model.CreatedAt, &model.UpdatedAt); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

// DiscoverProviderModels refreshes the safe catalogue from the saved
// OpenAI-compatible endpoint. The credential is decrypted only while building
// the outbound request and never appears in the response, audit, or error.
func (s *Service) DiscoverProviderModels(ctx context.Context, id uuid.UUID, expectedVersion int, key string) ([]ProviderModel, error) {
	provider, err := s.GetProvider(ctx, id)
	if err != nil {
		return nil, err
	}
	if provider.Version != expectedVersion {
		return nil, ErrVersionConflict
	}
	models, err := s.discoverModels(ctx, provider)
	if err != nil {
		return nil, err
	}
	body, err := s.idempotent(ctx, "llm-provider:discover:"+id.String(), key, struct {
		ExpectedVersion int      `json:"expectedVersion"`
		Models          []string `json:"models"`
	}{expectedVersion, models}, 200, func(tx pgx.Tx) (json.RawMessage, error) {
		var version int
		if err := tx.QueryRow(ctx, "SELECT version FROM llm_provider_configurations WHERE id=$1 FOR UPDATE", id).Scan(&version); err != nil {
			return nil, notFound(err)
		}
		if version != expectedVersion {
			return nil, ErrVersionConflict
		}
		if err := s.updateModelCatalogue(ctx, tx, id, models); err != nil {
			return nil, err
		}
		if err := s.audit(ctx, tx, "model_discover", "llm_provider", id, safeAudit("model_discover", version, map[string]any{})); err != nil {
			return nil, err
		}
		rows, err := tx.Query(ctx, "SELECT id,provider_id,model_key,source,availability,last_seen_at,created_at,updated_at FROM llm_provider_models WHERE provider_id=$1 ORDER BY model_key ASC,id ASC", id)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []ProviderModel{}
		for rows.Next() {
			var model ProviderModel
			if err = rows.Scan(&model.ID, &model.ProviderID, &model.ModelKey, &model.Source, &model.Availability, &model.LastSeenAt, &model.CreatedAt, &model.UpdatedAt); err != nil {
				return nil, err
			}
			out = append(out, model)
		}
		if err = rows.Err(); err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
	if err != nil {
		return nil, err
	}
	var out []ProviderModel
	if err = json.Unmarshal(body, &out); err != nil {
		return nil, ErrIdempotency
	}
	return out, nil
}

func (s *Service) VerifyProvider(ctx context.Context, id uuid.UUID, expectedVersion int, key string, optionalModel string) (Provider, error) {
	_, commandErr := s.RunValidationCommand(ctx, ValidationResourceProvider, id, expectedVersion, key, func(ctx context.Context) ValidationOutcome {
		provider, err := s.GetProvider(ctx, id)
		if err != nil {
			return validationOutcome(err, safeChecks(check("connection", false, "configuration_not_found")))
		}
		if model := strings.TrimSpace(optionalModel); model != "" && model != provider.DefaultModel {
			return validationOutcome(&safehttp.Error{Code: "model_unavailable"}, safeChecks(check("model", false, "model_unavailable")))
		}
		models, err := s.discoverModels(ctx, provider)
		if err != nil {
			return validationOutcome(err, safeChecks(check("connection", false, safehttp.ErrorCode(err))))
		}
		available := false
		for _, model := range models {
			if model == provider.DefaultModel {
				available = true
				break
			}
		}
		if !available {
			return validationOutcome(&safehttp.Error{Code: "model_unavailable"}, safeChecks(check("connection", true, ""), check("model", false, "model_unavailable")))
		}
		// Catalogue persistence is deliberately separated from the external call;
		// an optimistic transaction makes a raced edit win without stale writes.
		tx, txErr := s.pool.Begin(ctx)
		if txErr != nil {
			return validationOutcome(txErr, safeChecks(check("model", false, "upstream_unavailable")))
		}
		defer tx.Rollback(ctx)
		var version int
		if txErr = tx.QueryRow(ctx, "SELECT version FROM llm_provider_configurations WHERE id=$1 FOR UPDATE", id).Scan(&version); txErr == nil && version == expectedVersion {
			txErr = s.updateModelCatalogue(ctx, tx, id, models)
		}
		if txErr == nil && version != expectedVersion {
			txErr = ErrVersionConflict
		}
		if txErr == nil {
			txErr = tx.Commit(ctx)
		}
		if txErr != nil {
			return validationOutcome(txErr, safeChecks(check("model", false, "model_unavailable")))
		}
		return validationOutcome(nil, safeChecks(check("connection", true, ""), check("model", true, "")))
	})
	out, readErr := s.GetProvider(ctx, id)
	if readErr != nil {
		return Provider{}, readErr
	}
	return out, commandErr
}

func (s *Service) EnableProvider(ctx context.Context, id uuid.UUID, expectedVersion int, key string) (Provider, error) {
	if _, err := s.SetResourceEnabled(ctx, ValidationResourceProvider, id, expectedVersion, true, key); err != nil {
		return Provider{}, err
	}
	provider, err := s.GetProvider(ctx, id)
	if err != nil {
		return Provider{}, err
	}
	available, err := s.providerModelAvailable(ctx, id, provider.DefaultModel)
	if err != nil {
		return Provider{}, err
	}
	if !available {
		return Provider{}, ErrVerification
	}
	return provider, nil
}

func (s *Service) DisableProvider(ctx context.Context, id uuid.UUID, expectedVersion int, key string) (Provider, error) {
	if _, err := s.SetResourceEnabled(ctx, ValidationResourceProvider, id, expectedVersion, false, key); err != nil {
		return Provider{}, err
	}
	return s.GetProvider(ctx, id)
}
func (s *Service) UpsertProviderModel(ctx context.Context, model ProviderModel) (ProviderModel, error) {
	if model.ProviderID == uuid.Nil || strings.TrimSpace(model.ModelKey) == "" || len(model.ModelKey) > 200 ||
		(model.Source != "discovered" && model.Source != "manual") ||
		(model.Availability != "available" && model.Availability != "unavailable") {
		return ProviderModel{}, ErrValidation
	}
	if model.ID == uuid.Nil {
		model.ID = uuid.New()
	}
	var out ProviderModel
	err := s.pool.QueryRow(ctx, `INSERT INTO llm_provider_models(id,provider_id,model_key,source,availability,last_seen_at)
		VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(provider_id,model_key) DO UPDATE SET source=EXCLUDED.source,availability=EXCLUDED.availability,last_seen_at=EXCLUDED.last_seen_at,updated_at=NOW()
		RETURNING id,provider_id,model_key,source,availability,last_seen_at,created_at,updated_at`,
		model.ID, model.ProviderID, model.ModelKey, model.Source, model.Availability, model.LastSeenAt,
	).Scan(&out.ID, &out.ProviderID, &out.ModelKey, &out.Source, &out.Availability, &out.LastSeenAt, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}
func (s *Service) UpdateProvider(ctx context.Context, id uuid.UUID, r ProviderUpdate) (Provider, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Provider{}, err
	}
	defer tx.Rollback(ctx)
	out, err := s.updateProviderTx(ctx, tx, id, r)
	if err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}
func (s *Service) updateProviderTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, r ProviderUpdate) (Provider, error) {
	if r.ExpectedVersion < 1 || (r.Name == nil && r.BaseURL == nil && r.DefaultModel == nil && r.TimeoutSeconds == nil && r.Secret == nil && r.ClearSecret == nil) || !validOptional(r.Secret) || r.ClearSecret != nil && (!*r.ClearSecret || r.Secret != nil) {
		return Provider{}, ErrValidation
	}
	var cur Provider
	e := scanProvider(tx.QueryRow(ctx, "SELECT "+providerColumns+" FROM llm_provider_configurations WHERE id=$1", id), &cur)
	e = notFound(e)
	if e != nil {
		return Provider{}, e
	}
	if cur.Version != r.ExpectedVersion {
		return Provider{}, ErrVersionConflict
	}
	if r.Name != nil {
		cur.Name = *r.Name
	}
	if r.BaseURL != nil {
		cur.BaseURL = *r.BaseURL
	}
	if r.DefaultModel != nil {
		cur.DefaultModel = *r.DefaultModel
	}
	if r.TimeoutSeconds != nil {
		cur.TimeoutSeconds = *r.TimeoutSeconds
	}
	if !validProvider(cur.Name, cur.BaseURL, cur.DefaultModel, cur.TimeoutSeconds) {
		return Provider{}, ErrValidation
	}
	var enc any = nil
	fp := cur.SecretFingerprint
	if r.Secret != nil {
		var value string
		value, _, e = s.seal(*r.Secret)
		enc, fp = value, stringPointer(fingerprint(*r.Secret))
		if e != nil {
			return Provider{}, e
		}
	} else if r.ClearSecret != nil {
		enc = ""
		fp = nil
	}
	var out Provider
	e = scanProvider(tx.QueryRow(ctx, "UPDATE llm_provider_configurations SET name=$2,base_url=$3,default_model=$4,timeout_seconds=$5,encrypted_secret=CASE WHEN $6::text IS NULL THEN encrypted_secret ELSE NULLIF($6::text,'') END,secret_fingerprint=CASE WHEN $6::text IS NULL THEN secret_fingerprint ELSE $7 END,version=version+1,integration_status='stale',validation_details='{}'::jsonb,updated_at=NOW() WHERE id=$1 AND version=$8 RETURNING "+providerColumns, id, cur.Name, cur.BaseURL, cur.DefaultModel, cur.TimeoutSeconds, enc, fp, r.ExpectedVersion), &out)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, s.updateMissingOrConflict(ctx, tx, "llm_provider_configurations", id)
	}
	if e != nil {
		return out, e
	}
	if e = s.audit(ctx, tx, "update", "llm_provider", id, safeAudit("update", out.Version, map[string]any{"name": out.Name, "baseUrl": out.BaseURL, "defaultModel": out.DefaultModel, "timeoutSeconds": out.TimeoutSeconds, "secretChanged": r.Secret != nil, "secretCleared": r.ClearSecret != nil})); e != nil {
		return out, e
	}
	if r.Secret != nil || r.ClearSecret != nil {
		action := "secret_replace"
		if r.ClearSecret != nil {
			action = "secret_clear"
		}
		if e = s.audit(ctx, tx, action, "llm_provider", id, safeAudit(action, out.Version, map[string]any{})); e != nil {
			return out, e
		}
	}
	return out, nil
}

// UpdateProviderIdempotent performs a resource-scoped PATCH replay before any
// optimistic-lock check.  The PostgreSQL advisory lock coordinates all API instances.
func (s *Service) UpdateProviderIdempotent(ctx context.Context, id uuid.UUID, r ProviderUpdate, key string) (Provider, error) {
	body, err := s.idempotent(ctx, "llm-provider:update:"+id.String(), key, r, 200, func(tx pgx.Tx) (json.RawMessage, error) {
		out, err := s.updateProviderTx(ctx, tx, id, r)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
	var out Provider
	if err == nil {
		err = json.Unmarshal(body, &out)
	}
	return out, err
}
func (s *Service) CreateConnection(ctx context.Context, r ConnectionCreate, key string) (Connection, error) {
	if r.ConnectionType != "n8n" || r.AuthType != "api_key" || !validCommon(r.Name, r.BaseURL, r.TimeoutSeconds) || !validN8n(r.TypeConfig) || !validOptional(r.Credential) {
		return Connection{}, ErrValidation
	}
	body, err := s.idempotent(ctx, "workflow-connection:create", key, r, 201, func(tx pgx.Tx) (json.RawMessage, error) {
		var out Connection
		enc, fp, e := s.secret(r.Credential)
		if e != nil {
			return nil, e
		}
		e = scanConnection(tx.QueryRow(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,encrypted_credential,credential_fingerprint,timeout_seconds,type_config) VALUES($1,$2,'n8n',$3,'api_key',$4,$5,$6,$7) RETURNING "+connectionColumns, uuid.New(), r.Name, r.BaseURL, enc, fp, r.TimeoutSeconds, r.TypeConfig), &out)
		if e != nil {
			return nil, e
		}
		if e = s.audit(ctx, tx, "create", "workflow_connection", out.ID, safeAudit("create", out.Version, map[string]any{"name": out.Name})); e != nil {
			return nil, e
		}
		return json.Marshal(out)
	})
	var out Connection
	if err == nil {
		err = json.Unmarshal(body, &out)
	}
	return out, err
}
func (s *Service) GetConnection(ctx context.Context, id uuid.UUID) (Connection, error) {
	var x Connection
	e := scanConnection(s.pool.QueryRow(ctx, "SELECT "+connectionColumns+" FROM workflow_connections WHERE id=$1", id), &x)
	if e = notFound(e); e != nil {
		return x, e
	}
	return x, s.hydrateConnectionEligibility(ctx, &x)
}

func GetConnectionForShare(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Connection, error) {
	var x Connection
	e := scanConnection(tx.QueryRow(ctx, "SELECT "+connectionColumns+" FROM workflow_connections WHERE id=$1 FOR SHARE", id), &x)
	return x, notFound(e)
}
func (s *Service) ListConnections(ctx context.Context, o ListOptions) ([]Connection, int, error) {
	// Static filters in SQL; credential-aware executable is finalized then filtered before pagination.
	q, args := where(o, "connection_type", nil)
	rows, e := s.pool.Query(ctx, "SELECT "+connectionColumns+" FROM workflow_connections"+q+" ORDER BY updated_at DESC,id ASC", args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	filtered := make([]Connection, 0)
	for rows.Next() {
		var x Connection
		if e = scanConnection(rows, &x); e != nil {
			return nil, 0, e
		}
		if e = s.hydrateConnectionEligibility(ctx, &x); e != nil {
			return nil, 0, e
		}
		if o.ValidationStatus != "" && x.ValidationStatus != o.ValidationStatus {
			continue
		}
		if o.Executable != nil && x.Executable != *o.Executable {
			continue
		}
		filtered = append(filtered, x)
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	return paginateSlice(filtered, o.Limit, o.Offset), len(filtered), nil
}
func (s *Service) UpdateConnection(ctx context.Context, id uuid.UUID, r ConnectionUpdate) (Connection, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Connection{}, err
	}
	defer tx.Rollback(ctx)
	out, err := s.updateConnectionTx(ctx, tx, id, r)
	if err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}
func (s *Service) updateConnectionTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, r ConnectionUpdate) (Connection, error) {
	if r.ExpectedVersion < 1 || (r.Name == nil && r.BaseURL == nil && r.AuthType == nil && r.TimeoutSeconds == nil && r.TypeConfig == nil && r.Credential == nil && r.ClearCredential == nil) || !validOptional(r.Credential) || r.ClearCredential != nil && (!*r.ClearCredential || r.Credential != nil) {
		return Connection{}, ErrValidation
	}
	var cur Connection
	e := scanConnection(tx.QueryRow(ctx, "SELECT "+connectionColumns+" FROM workflow_connections WHERE id=$1", id), &cur)
	e = notFound(e)
	if e != nil {
		return Connection{}, e
	}
	if cur.Version != r.ExpectedVersion {
		return Connection{}, ErrVersionConflict
	}
	if r.Name != nil {
		cur.Name = *r.Name
	}
	if r.BaseURL != nil {
		cur.BaseURL = *r.BaseURL
	}
	if r.AuthType != nil {
		cur.AuthType = *r.AuthType
	}
	if r.TimeoutSeconds != nil {
		cur.TimeoutSeconds = *r.TimeoutSeconds
	}
	if r.TypeConfig != nil {
		cur.TypeConfig = r.TypeConfig
	}
	if cur.AuthType != "api_key" || !validCommon(cur.Name, cur.BaseURL, cur.TimeoutSeconds) || !validN8n(cur.TypeConfig) {
		return Connection{}, ErrValidation
	}
	var enc any = nil
	fp := cur.CredentialFingerprint
	if r.Credential != nil {
		var value string
		value, _, e = s.seal(*r.Credential)
		enc, fp = value, stringPointer(fingerprint(*r.Credential))
		if e != nil {
			return Connection{}, e
		}
	} else if r.ClearCredential != nil {
		enc = ""
		fp = nil
	}
	var out Connection
	e = scanConnection(tx.QueryRow(ctx, "UPDATE workflow_connections SET name=$2,base_url=$3,auth_type=$4,timeout_seconds=$5,type_config=$6,encrypted_credential=CASE WHEN $7::text IS NULL THEN encrypted_credential ELSE NULLIF($7::text,'') END,credential_fingerprint=CASE WHEN $7::text IS NULL THEN credential_fingerprint ELSE $8 END,version=version+1,integration_status='stale',validation_details='{}'::jsonb,updated_at=NOW() WHERE id=$1 AND version=$9 RETURNING "+connectionColumns, id, cur.Name, cur.BaseURL, cur.AuthType, cur.TimeoutSeconds, cur.TypeConfig, enc, fp, r.ExpectedVersion), &out)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, s.updateMissingOrConflict(ctx, tx, "workflow_connections", id)
	}
	if e != nil {
		return out, e
	}
	if e = s.audit(ctx, tx, "update", "workflow_connection", id, safeAudit("update", out.Version, map[string]any{"name": out.Name, "baseUrl": out.BaseURL, "authType": out.AuthType, "timeoutSeconds": out.TimeoutSeconds, "typeConfigChanged": r.TypeConfig != nil, "credentialChanged": r.Credential != nil, "credentialCleared": r.ClearCredential != nil})); e != nil {
		return out, e
	}
	if r.Credential != nil || r.ClearCredential != nil {
		action := "credential_replace"
		if r.ClearCredential != nil {
			action = "credential_clear"
		}
		if e = s.audit(ctx, tx, action, "workflow_connection", id, safeAudit(action, out.Version, map[string]any{})); e != nil {
			return out, e
		}
	}
	return out, nil
}
func (s *Service) UpdateConnectionIdempotent(ctx context.Context, id uuid.UUID, r ConnectionUpdate, key string) (Connection, error) {
	body, err := s.idempotent(ctx, "workflow-connection:update:"+id.String(), key, r, 200, func(tx pgx.Tx) (json.RawMessage, error) {
		out, err := s.updateConnectionTx(ctx, tx, id, r)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
	var out Connection
	if err == nil {
		err = json.Unmarshal(body, &out)
	}
	return out, err
}

// VerifyConnection retains the legacy n8n probe adapter while delegating the
// shared status, version, idempotency and audit semantics to the common
// validation command. Verification never enables the resource.
func (s *Service) VerifyConnection(ctx context.Context, id uuid.UUID, expectedVersion int, key string) (Connection, error) {
	_, commandErr := s.RunValidationCommand(ctx, ValidationResourceConnection, id, expectedVersion, key, func(ctx context.Context) ValidationOutcome {
		current, readErr := s.GetConnection(ctx, id)
		if readErr != nil {
			return validationOutcome(readErr, safeChecks(check("connection", false, "configuration_not_found")))
		}
		credential, credentialErr := s.connectionCredential(ctx, id)
		if credentialErr != nil {
			return validationOutcome(credentialErr, safeChecks(check("connection", false, "configuration_unverified")))
		}
		var instance any
		err := s.integrationRequest(ctx, current.BaseURL, "api/v1/workflows?limit=1", credential, "X-N8N-API-KEY", current.TimeoutSeconds, &instance)
		return validationOutcome(err, safeChecks(check("connection", err == nil, safehttp.ErrorCode(err))))
	})
	out, readErr := s.GetConnection(ctx, id)
	if readErr != nil {
		return Connection{}, readErr
	}
	return out, commandErr
}

func (s *Service) EnableConnection(ctx context.Context, id uuid.UUID, expectedVersion int, key string) (Connection, error) {
	if _, err := s.SetResourceEnabled(ctx, ValidationResourceConnection, id, expectedVersion, true, key); err != nil {
		return Connection{}, err
	}
	return s.GetConnection(ctx, id)
}

// DisableConnection is intentionally local: it neither deletes configurations
// nor contacts n8n.  Repeating the action at the current version is a no-op.
func (s *Service) DisableConnection(ctx context.Context, id uuid.UUID, expectedVersion int, key string) (Connection, error) {
	if _, err := s.SetResourceEnabled(ctx, ValidationResourceConnection, id, expectedVersion, false, key); err != nil {
		return Connection{}, err
	}
	return s.GetConnection(ctx, id)
}
func (s *Service) CreateWorkflow(ctx context.Context, r WorkflowCreate, key string) (Workflow, error) {
	r.LlmStrategy = normalizedLlmStrategy(r.LlmStrategy)
	if !validWorkflow(r.Name, r.ApplicableStages, r.TypeConfig, r.InputContractVersion, r.OutputContractVersion, r.DefaultParameters) || !validNote(r.Note) || !validLlmPolicy(r.LlmStrategy, r.LlmProviderID, r.LlmModel) {
		return Workflow{}, ErrValidation
	}
	if _, e := s.GetConnection(ctx, r.ConnectionID); e != nil {
		return Workflow{}, e
	}
	body, err := s.idempotent(ctx, "workflow-configuration:create", key, r, 201, func(tx pgx.Tx) (json.RawMessage, error) {
		id := uuid.New()
		_, e := tx.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version,default_parameters,note,llm_strategy,llm_provider_id,llm_model) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)", id, r.Name, r.ConnectionID, mustJSON(r.ApplicableStages), r.TypeConfig, r.InputContractVersion, r.OutputContractVersion, defaultJSON(r.DefaultParameters), r.Note, r.LlmStrategy, r.LlmProviderID, r.LlmModel)
		if e != nil {
			return nil, e
		}
		var out Workflow
		e = scanWorkflow(tx.QueryRow(ctx, "SELECT "+workflowColumns+" FROM workflow_configurations w JOIN workflow_connections c ON c.id=w.connection_id WHERE w.id=$1", id), &out)
		if e != nil {
			return nil, e
		}
		if e = s.audit(ctx, tx, "create", "workflow_configuration", id, safeAudit("create", out.Version, map[string]any{"name": out.Name})); e != nil {
			return nil, e
		}
		return json.Marshal(out)
	})
	var out Workflow
	if err == nil {
		err = json.Unmarshal(body, &out)
	}
	return out, err
}
func (s *Service) GetWorkflow(ctx context.Context, id uuid.UUID) (Workflow, error) {
	eligibility, err := s.EvaluateWorkflowExecutionEligibility(ctx, id, "")
	return eligibility.Workflow, err
}

func (s *Service) EvaluateWorkflowExecutionEligibility(ctx context.Context, id uuid.UUID, requiredStage string) (WorkflowExecutionEligibility, error) {
	var workflow Workflow
	err := scanWorkflow(s.pool.QueryRow(ctx, "SELECT "+workflowColumns+" FROM workflow_configurations w JOIN workflow_connections c ON c.id=w.connection_id WHERE w.id=$1", id), &workflow)
	if err != nil {
		return WorkflowExecutionEligibility{}, notFound(err)
	}
	connection, err := s.hydrateWorkflowEligibility(ctx, &workflow, requiredStage)
	if err != nil {
		return WorkflowExecutionEligibility{}, err
	}
	return EvaluateWorkflowExecutionEligibility(workflow, connection, requiredStage), nil
}

func EvaluateWorkflowExecutionEligibility(workflow Workflow, connection Connection, requiredStage string) WorkflowExecutionEligibility {
	reasons := append([]IneligibilityReason(nil), workflow.IneligibilityReasons...)
	if !workflow.Executable && len(reasons) == 0 {
		reasons = append(reasons, IneligibilityReason{Code: "workflow_configuration_not_executable", Message: "The workflow configuration cannot execute.", RepairAction: "workflow_configuration:view"})
	}
	if !connection.Executable {
		reasons = append(reasons, connection.ineligibilityReasons...)
		if len(connection.ineligibilityReasons) == 0 {
			reasons = append(reasons, IneligibilityReason{Code: "connection_not_executable", Message: "The workflow connection cannot execute.", RepairAction: "connection:view"})
		}
	}
	if strings.TrimSpace(requiredStage) != "" && !slices.Contains(workflow.ApplicableStages, requiredStage) {
		reasons = append(reasons, IneligibilityReason{Code: "workflow_stage_mismatch", Message: "The workflow does not support this stage.", RepairAction: "workflow_configuration:edit", RepairTarget: &RepairTarget{ConnectionID: &connection.ID, WorkflowConfigurationID: &workflow.ID}})
	}
	reasons = uniqueEligibilityReasons(reasons)
	return WorkflowExecutionEligibility{Workflow: workflow, Connection: connection, Executable: workflow.Executable && connection.Executable && len(reasons) == 0, Reasons: reasons}
}

func uniqueEligibilityReasons(reasons []IneligibilityReason) []IneligibilityReason {
	seen := make(map[string]struct{}, len(reasons))
	out := make([]IneligibilityReason, 0, len(reasons))
	for _, value := range reasons {
		if _, exists := seen[value.Code]; exists {
			continue
		}
		seen[value.Code] = struct{}{}
		out = append(out, value)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if reasonPriority(out[i].Code) == reasonPriority(out[j].Code) {
			return out[i].Code < out[j].Code
		}
		return reasonPriority(out[i].Code) < reasonPriority(out[j].Code)
	})
	return out
}

func GetWorkflowForShare(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Workflow, error) {
	var x Workflow
	e := scanWorkflow(tx.QueryRow(ctx, "SELECT "+workflowColumns+" FROM workflow_configurations w JOIN workflow_connections c ON c.id=w.connection_id WHERE w.id=$1 FOR SHARE OF w", id), &x)
	return x, notFound(e)
}
func (s *Service) ListWorkflows(ctx context.Context, o ListOptions) ([]Workflow, int, error) {
	whereClause, args := workflowListWhere(o)
	rows, e := s.pool.Query(ctx, "SELECT "+workflowColumns+" FROM workflow_configurations w JOIN workflow_connections c ON c.id=w.connection_id"+whereClause+" ORDER BY w.updated_at DESC,w.id ASC", args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	candidates := make([]Workflow, 0)
	for rows.Next() {
		var x Workflow
		if e = scanWorkflow(rows, &x); e != nil {
			return nil, 0, e
		}
		candidates = append(candidates, x)
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	// Batch-load connections/providers once for the filtered candidate set (bounded by SQL filters).
	if e = s.hydrateWorkflowsEligibilityBatch(ctx, candidates); e != nil {
		return nil, 0, e
	}
	filtered := make([]Workflow, 0, len(candidates))
	for _, x := range candidates {
		if o.ValidationStatus != "" && x.ValidationStatus != o.ValidationStatus {
			continue
		}
		if o.Executable != nil && x.Executable != *o.Executable {
			continue
		}
		if o.LlmStrategy != "" && x.LlmStrategy != o.LlmStrategy {
			continue
		}
		filtered = append(filtered, x)
	}
	return paginateSlice(filtered, o.Limit, o.Offset), len(filtered), nil
}

func workflowListWhere(o ListOptions) (string, []any) {
	where := ""
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		if where == "" {
			where = " WHERE " + fmt.Sprintf(clause, len(args))
			return
		}
		where += " AND " + fmt.Sprintf(clause, len(args))
	}
	if strings.TrimSpace(o.Query) != "" {
		add("w.name ILIKE $%d", "%"+strings.TrimSpace(o.Query)+"%")
	}
	if o.ConnectionID != "" {
		add("w.connection_id=$%d", o.ConnectionID)
	}
	if o.Type != "" {
		add("c.connection_type=$%d", o.Type)
	}
	if o.IntegrationStatus != "" {
		add("w.integration_status=$%d", o.IntegrationStatus)
	}
	if o.ValidationStatus != "" {
		add("w.integration_status=$%d", o.ValidationStatus)
	}
	if o.Enabled != nil {
		add("w.enabled=$%d", *o.Enabled)
	}
	if o.ApplicableStage != "" {
		add("w.applicable_stages::jsonb ? $%d", o.ApplicableStage)
	}
	if o.LlmStrategy != "" {
		add("COALESCE(NULLIF(w.llm_strategy,''),'none')=$%d", o.LlmStrategy)
	}
	return where, args
}

func (s *Service) hydrateWorkflowEligibility(ctx context.Context, workflow *Workflow, requiredStage string) (Connection, error) {
	connection, err := s.GetConnection(ctx, workflow.ConnectionID)
	if err != nil {
		return Connection{}, err
	}
	return connection, s.applyWorkflowEligibility(ctx, workflow, connection, requiredStage, nil)
}

// hydrateWorkflowsEligibilityBatch loads related connections and providers once
// for the candidate set so list filtering does not issue unbounded per-row queries.
func (s *Service) hydrateWorkflowsEligibilityBatch(ctx context.Context, workflows []Workflow) error {
	if len(workflows) == 0 {
		return nil
	}
	connectionIDs := make([]uuid.UUID, 0, len(workflows))
	providerIDs := make([]uuid.UUID, 0)
	seenConn := map[uuid.UUID]struct{}{}
	seenProv := map[uuid.UUID]struct{}{}
	for i := range workflows {
		if _, ok := seenConn[workflows[i].ConnectionID]; !ok && workflows[i].ConnectionID != uuid.Nil {
			seenConn[workflows[i].ConnectionID] = struct{}{}
			connectionIDs = append(connectionIDs, workflows[i].ConnectionID)
		}
		if workflows[i].LlmStrategy == "acf_managed" && workflows[i].LlmProviderID != nil {
			if _, ok := seenProv[*workflows[i].LlmProviderID]; !ok {
				seenProv[*workflows[i].LlmProviderID] = struct{}{}
				providerIDs = append(providerIDs, *workflows[i].LlmProviderID)
			}
		}
	}
	connections := map[uuid.UUID]Connection{}
	for _, id := range connectionIDs {
		connection, err := s.GetConnection(ctx, id)
		if err != nil {
			return err
		}
		connections[id] = connection
	}
	providers := map[uuid.UUID]Provider{}
	for _, id := range providerIDs {
		provider, err := s.GetProvider(ctx, id)
		if err != nil {
			// Missing provider is an eligibility fact, not a list failure.
			continue
		}
		providers[id] = provider
	}
	for i := range workflows {
		connection, ok := connections[workflows[i].ConnectionID]
		if !ok {
			return ErrNotFound
		}
		if err := s.applyWorkflowEligibility(ctx, &workflows[i], connection, "", providers); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) applyWorkflowEligibility(ctx context.Context, workflow *Workflow, connection Connection, requiredStage string, providers map[uuid.UUID]Provider) error {
	providerFact := EligibilityFact{Kind: "provider", Status: ValidationVerified, Enabled: true, Version: 1, VerifiedVersion: intPointer(1), ModelAvailable: true}
	strategyComplete := true
	if workflow.LlmStrategy == "acf_managed" {
		if workflow.LlmProviderID == nil || workflow.LlmModel == nil {
			strategyComplete = false
		} else {
			var provider Provider
			var ok bool
			if providers != nil {
				provider, ok = providers[*workflow.LlmProviderID]
			}
			if !ok {
				loaded, providerErr := s.GetProvider(ctx, *workflow.LlmProviderID)
				if providerErr != nil {
					strategyComplete = false
				} else {
					provider = loaded
					ok = true
				}
			}
			if ok {
				available, availabilityErr := s.providerModelAvailable(ctx, provider.ID, *workflow.LlmModel)
				if availabilityErr != nil {
					return availabilityErr
				}
				providerFact = EligibilityFact{Kind: "provider", ResourceID: &provider.ID, Status: ValidationStatus(provider.ValidationStatus), Enabled: provider.Enabled, Version: provider.Version, VerifiedVersion: provider.VerifiedVersion, ModelAvailable: available}
			} else {
				strategyComplete = false
			}
		}
	}
	stageMatches := len(workflow.ApplicableStages) > 0
	if strings.TrimSpace(requiredStage) != "" {
		stageMatches = slices.Contains(workflow.ApplicableStages, requiredStage)
	}
	workflowFact := EligibilityFact{Kind: "workflow_configuration", ResourceID: &workflow.ID, ConnectionID: &connection.ID, Status: ValidationStatus(workflow.ValidationStatus), Enabled: workflow.Enabled, Version: workflow.Version, VerifiedVersion: workflow.VerifiedVersion, StrategyComplete: strategyComplete, ReferenceExists: true, ReferenceActive: true, StageMatches: stageMatches, InputCompatible: validContractVersion(workflow.InputContractVersion), OutputCompatible: validContractVersion(workflow.OutputContractVersion)}
	connectionFact := EligibilityFact{Kind: "connection", ResourceID: &connection.ID, WorkflowConfigurationID: &workflow.ID, Status: ValidationStatus(connection.ValidationStatus), Enabled: connection.Enabled, Version: connection.Version, VerifiedVersion: connection.VerifiedVersion, CredentialRequired: connection.AuthType == "api_key", CredentialAvailable: connection.credentialReadable}
	workflow.Executable, workflow.IneligibilityReasons = EvaluateEligibility(workflowFact, connectionFact, providerFact)
	return nil
}

func (s *Service) hydrateConnectionEligibility(ctx context.Context, connection *Connection) error {
	credential := ""
	var err error
	if connection.encryptedCredential != nil && *connection.encryptedCredential != "" {
		credential, err = s.unseal(*connection.encryptedCredential)
	}
	connection.credentialReadable = err == nil && strings.TrimSpace(credential) != ""
	connection.Executable, connection.ineligibilityReasons = EvaluateEligibility(EligibilityFact{
		Kind: "connection", ResourceID: &connection.ID, Status: ValidationStatus(connection.ValidationStatus), Enabled: connection.Enabled,
		Version: connection.Version, VerifiedVersion: connection.VerifiedVersion, ModelAvailable: true,
		StrategyComplete: true, ReferenceExists: true, ReferenceActive: true, StageMatches: true,
		InputCompatible: true, OutputCompatible: true, CredentialRequired: connection.AuthType == "api_key", CredentialAvailable: connection.credentialReadable,
	})
	return nil
}

func (s *Service) workflowStrategyExecutable(ctx context.Context, workflow Workflow) bool {
	if workflow.LlmStrategy == "none" || workflow.LlmStrategy == "n8n_managed" {
		return true
	}
	if workflow.LlmStrategy != "acf_managed" || workflow.LlmProviderID == nil || workflow.LlmModel == nil {
		return false
	}
	provider, err := s.GetProvider(ctx, *workflow.LlmProviderID)
	if err != nil || !provider.Executable {
		return false
	}
	available, err := s.providerModelAvailable(ctx, provider.ID, *workflow.LlmModel)
	return err == nil && available
}

func (s *Service) workflowDependenciesExecutable(ctx context.Context, workflow Workflow) bool {
	connection, err := s.GetConnection(ctx, workflow.ConnectionID)
	return err == nil && connection.Executable && s.workflowStrategyExecutable(ctx, workflow)
}

func containsAll(actual, expected []string) bool {
	set := map[string]bool{}
	for _, value := range actual {
		set[value] = true
	}
	for _, value := range expected {
		if !set[value] {
			return false
		}
	}
	return len(expected) > 0
}

func validContractVersion(value string) bool {
	return strings.TrimSpace(value) != "" && len(value) <= 40
}
func intPointer(value int) *int { return &value }
func (s *Service) CreatePlatform(ctx context.Context, r PlatformCreate, key string) (Platform, error) {
	if !validPlatform(r.Name, r.PlatformType, r.AccountIdentifier, r.EndpointURL, r.AuthType, r.TimeoutSeconds, r.TypeConfig) || !validOptional(r.Credential) || !validNote(r.Note) {
		return Platform{}, ErrValidation
	}
	body, err := s.idempotent(ctx, "distribution-platform:create", key, r, 201, func(tx pgx.Tx) (json.RawMessage, error) {
		var out Platform
		enc, fp, e := s.secret(r.Credential)
		if e != nil {
			return nil, e
		}
		e = scanPlatform(tx.QueryRow(ctx, "INSERT INTO distribution_platform_configurations(id,name,platform_type,account_identifier,endpoint_url,auth_type,encrypted_credential,credential_fingerprint,timeout_seconds,type_config,note) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING "+platformColumns, uuid.New(), r.Name, r.PlatformType, r.AccountIdentifier, r.EndpointURL, r.AuthType, enc, fp, r.TimeoutSeconds, r.TypeConfig, r.Note), &out)
		if e != nil {
			return nil, e
		}
		if e = s.audit(ctx, tx, "create", "distribution_platform", out.ID, safeAudit("create", out.Version, map[string]any{"name": out.Name})); e != nil {
			return nil, e
		}
		return json.Marshal(out)
	})
	var out Platform
	if err == nil {
		err = json.Unmarshal(body, &out)
	}
	return out, err
}
func (s *Service) GetPlatform(ctx context.Context, id uuid.UUID) (Platform, error) {
	var x Platform
	e := scanPlatform(s.pool.QueryRow(ctx, "SELECT "+platformColumns+" FROM distribution_platform_configurations WHERE id=$1", id), &x)
	return x, notFound(e)
}
func (s *Service) ListPlatforms(ctx context.Context, o ListOptions) ([]Platform, int, error) {
	// Platform list must finalize derived validation/executable before filter and pagination.
	q, args := where(o, "platform_type", nil)
	rows, e := s.pool.Query(ctx, "SELECT "+platformColumns+" FROM distribution_platform_configurations"+q+" ORDER BY updated_at DESC,id ASC", args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	filtered := make([]Platform, 0)
	for rows.Next() {
		var x Platform
		if e = scanPlatform(rows, &x); e != nil {
			return nil, 0, e
		}
		if o.ValidationStatus != "" && x.ValidationStatus != o.ValidationStatus {
			continue
		}
		if o.Executable != nil && x.Executable != *o.Executable {
			continue
		}
		filtered = append(filtered, x)
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	return paginateSlice(filtered, o.Limit, o.Offset), len(filtered), nil
}
func (s *Service) UpdateWorkflow(ctx context.Context, id uuid.UUID, r WorkflowUpdate) (Workflow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Workflow{}, err
	}
	defer tx.Rollback(ctx)
	out, err := s.updateWorkflowTx(ctx, tx, id, r)
	if err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}
func (s *Service) updateWorkflowTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, r WorkflowUpdate) (Workflow, error) {
	if r.ExpectedVersion < 1 || (r.Name == nil && r.ConnectionID == nil && r.ApplicableStages == nil && r.TypeConfig == nil && r.InputContractVersion == nil && r.OutputContractVersion == nil && r.DefaultParameters == nil && r.Note == nil && r.LlmStrategy == nil && r.LlmProviderID == nil && r.LlmModel == nil) {
		return Workflow{}, ErrValidation
	}
	var cur Workflow
	e := scanWorkflow(tx.QueryRow(ctx, "SELECT "+workflowColumns+" FROM workflow_configurations w JOIN workflow_connections c ON c.id=w.connection_id WHERE w.id=$1", id), &cur)
	e = notFound(e)
	if e != nil {
		return Workflow{}, e
	}
	if cur.Version != r.ExpectedVersion {
		return Workflow{}, ErrVersionConflict
	}
	if r.Name != nil {
		cur.Name = *r.Name
	}
	if r.ConnectionID != nil {
		cur.ConnectionID = *r.ConnectionID
		if _, e = s.GetConnection(ctx, cur.ConnectionID); e != nil {
			return Workflow{}, e
		}
	}
	if r.ApplicableStages != nil {
		cur.ApplicableStages = *r.ApplicableStages
	}
	if r.TypeConfig != nil {
		cur.TypeConfig = r.TypeConfig
	}
	if r.InputContractVersion != nil {
		cur.InputContractVersion = *r.InputContractVersion
	}
	if r.OutputContractVersion != nil {
		cur.OutputContractVersion = *r.OutputContractVersion
	}
	if r.DefaultParameters != nil {
		cur.DefaultParameters = r.DefaultParameters
	}
	if r.Note != nil {
		cur.Note = *r.Note
	}
	if r.LlmStrategy != nil {
		cur.LlmStrategy = normalizedLlmStrategy(*r.LlmStrategy)
	}
	if r.LlmProviderID != nil {
		cur.LlmProviderID = *r.LlmProviderID
	}
	if r.LlmModel != nil {
		cur.LlmModel = *r.LlmModel
	}
	if !validWorkflow(cur.Name, cur.ApplicableStages, cur.TypeConfig, cur.InputContractVersion, cur.OutputContractVersion, cur.DefaultParameters) || !validNote(cur.Note) || !validLlmPolicy(cur.LlmStrategy, cur.LlmProviderID, cur.LlmModel) {
		return Workflow{}, ErrValidation
	}
	tag, e := tx.Exec(ctx, "UPDATE workflow_configurations SET name=$2,connection_id=$3,applicable_stages=$4,type_config=$5,input_contract_version=$6,output_contract_version=$7,default_parameters=$8,note=$9,llm_strategy=$10,llm_provider_id=$11,llm_model=$12,integration_status='stale',validation_details='{}'::jsonb,resolved_webhook_path=NULL,resolved_workflow_id=NULL,resolved_workflow_revision=NULL,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$13", id, cur.Name, cur.ConnectionID, mustJSON(cur.ApplicableStages), cur.TypeConfig, cur.InputContractVersion, cur.OutputContractVersion, cur.DefaultParameters, cur.Note, cur.LlmStrategy, cur.LlmProviderID, cur.LlmModel, r.ExpectedVersion)
	if e != nil {
		return Workflow{}, unique(e)
	}
	if tag.RowsAffected() != 1 {
		return Workflow{}, s.updateMissingOrConflict(ctx, tx, "workflow_configurations", id)
	}
	var out Workflow
	e = scanWorkflow(tx.QueryRow(ctx, "SELECT "+workflowColumns+" FROM workflow_configurations w JOIN workflow_connections c ON c.id=w.connection_id WHERE w.id=$1", id), &out)
	if e != nil {
		return Workflow{}, e
	}
	if e = s.audit(ctx, tx, "update", "workflow_configuration", id, safeAudit("update", out.Version, map[string]any{"name": out.Name, "connectionId": out.ConnectionID, "applicableStages": out.ApplicableStages, "inputContractVersion": out.InputContractVersion, "outputContractVersion": out.OutputContractVersion, "typeConfigChanged": r.TypeConfig != nil, "defaultParametersChanged": r.DefaultParameters != nil, "noteChanged": r.Note != nil})); e != nil {
		return Workflow{}, e
	}
	if r.ConnectionID != nil {
		if e = s.audit(ctx, tx, "connection_rebind", "workflow_configuration", id, safeAudit("connection_rebind", out.Version, map[string]any{"connectionId": out.ConnectionID})); e != nil {
			return Workflow{}, e
		}
	}
	return out, nil
}
func (s *Service) UpdateWorkflowIdempotent(ctx context.Context, id uuid.UUID, r WorkflowUpdate, key string) (Workflow, error) {
	body, err := s.idempotent(ctx, "workflow-configuration:update:"+id.String(), key, r, 200, func(tx pgx.Tx) (json.RawMessage, error) {
		out, err := s.updateWorkflowTx(ctx, tx, id, r)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
	var out Workflow
	if err == nil {
		err = json.Unmarshal(body, &out)
	}
	return out, err
}

// VerifyWorkflowConfiguration retains the legacy probe adapter and delegates
// common command semantics to RunValidationCommand. It never enables the
// workflow or creates a WorkflowRun, batch, or candidate.
func (s *Service) VerifyWorkflowConfiguration(ctx context.Context, id uuid.UUID, expectedVersion int, key string) (Workflow, error) {
	_, commandErr := s.RunValidationCommand(ctx, ValidationResourceWorkflow, id, expectedVersion, key, func(ctx context.Context) ValidationOutcome {
		workflow, err := s.GetWorkflow(ctx, id)
		if err != nil {
			return validationOutcome(err, safeChecks(check("connection", false, "configuration_not_found")))
		}
		connection, err := s.GetConnection(ctx, workflow.ConnectionID)
		if err != nil {
			return validationOutcome(err, safeChecks(check("connection", false, "configuration_unverified")))
		}
		checks := []map[string]any{}
		connectionOK := connection.Executable
		checks = append(checks, check("connection", connectionOK, "configuration_unverified"))
		var cfg struct {
			ReferenceType  string `json:"referenceType"`
			ReferenceValue string `json:"referenceValue"`
		}
		if json.Unmarshal(workflow.TypeConfig, &cfg) != nil || (cfg.ReferenceType != "workflow_id" && cfg.ReferenceType != "webhook_path") || strings.TrimSpace(cfg.ReferenceValue) == "" {
			return validationOutcome(&safehttp.Error{Code: "workflow_reference_not_found"}, safeChecks(checks...))
		}
		ref := workflowReferenceCheck{}
		var refErr error
		if cfg.ReferenceType == "workflow_id" {
			ref, refErr = s.n8nWorkflow(ctx, connection, cfg.ReferenceValue)
			if refErr == nil {
				resolvedConfig := workflow
				resolvedConfig.TypeConfig = mustJSON(map[string]string{"referenceType": "webhook_path", "referenceValue": ref.WebhookPath})
				refErr = s.probeWorkflow(ctx, connection, resolvedConfig)
				if refErr == nil {
					ref.Stages = workflow.ApplicableStages
				}
			}
		} else {
			refErr = s.probeWorkflow(ctx, connection, workflow)
			ref = workflowReferenceCheck{Exists: refErr == nil, Active: refErr == nil, Stages: workflow.ApplicableStages}
		}
		checks = append(checks, check("workflow_reference", refErr == nil && ref.Exists, "workflow_reference_not_found"))
		checks = append(checks, check("stage", refErr == nil && containsAll(ref.Stages, workflow.ApplicableStages), "workflow_stage_mismatch"))
		checks = append(checks, check("input_contract", validContractVersion(workflow.InputContractVersion), "input_contract_incompatible"))
		checks = append(checks, check("output_contract", validContractVersion(workflow.OutputContractVersion), "output_contract_incompatible"))
		strategyOK := s.workflowStrategyExecutable(ctx, workflow)
		checks = append(checks, check("llm_strategy", strategyOK, "llm_strategy_incomplete"))
		if refErr != nil {
			return validationOutcome(refErr, safeChecks(checks...))
		}
		if !connectionOK || !ref.Exists || !ref.Active || !containsAll(ref.Stages, workflow.ApplicableStages) || !validContractVersion(workflow.InputContractVersion) || !validContractVersion(workflow.OutputContractVersion) || !strategyOK {
			return validationOutcome(&safehttp.Error{Code: "configuration_verification_failed"}, safeChecks(checks...))
		}
		if cfg.ReferenceType == "workflow_id" {
			if _, persistErr := s.pool.Exec(ctx, `UPDATE workflow_configurations
				SET resolved_webhook_path=$1,resolved_workflow_id=$2,resolved_workflow_revision=$3,updated_at=NOW()
				WHERE id=$4 AND version=$5`, ref.WebhookPath, ref.WorkflowID, ref.Revision, id, expectedVersion); persistErr != nil {
				return validationOutcome(persistErr, safeChecks(checks...))
			}
		}
		return validationOutcome(nil, safeChecks(checks...))
	})
	out, readErr := s.GetWorkflow(ctx, id)
	if readErr != nil {
		return Workflow{}, readErr
	}
	return out, commandErr
}

func (s *Service) EnableWorkflowConfiguration(ctx context.Context, id uuid.UUID, expectedVersion int, key string) (Workflow, error) {
	workflow, err := s.GetWorkflow(ctx, id)
	if err != nil {
		return Workflow{}, err
	}
	if workflow.ValidationStatus != string(ValidationVerified) || workflow.VerifiedVersion == nil || *workflow.VerifiedVersion != workflow.Version || !s.workflowDependenciesExecutable(ctx, workflow) {
		return Workflow{}, ErrVerification
	}
	if _, err = s.SetResourceEnabled(ctx, ValidationResourceWorkflow, id, expectedVersion, true, key); err != nil {
		return Workflow{}, err
	}
	return s.GetWorkflow(ctx, id)
}

func (s *Service) DisableWorkflowConfiguration(ctx context.Context, id uuid.UUID, expectedVersion int, key string) (Workflow, error) {
	if _, err := s.SetResourceEnabled(ctx, ValidationResourceWorkflow, id, expectedVersion, false, key); err != nil {
		return Workflow{}, err
	}
	return s.GetWorkflow(ctx, id)
}
func (s *Service) UpdatePlatform(ctx context.Context, id uuid.UUID, r PlatformUpdate) (Platform, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Platform{}, err
	}
	defer tx.Rollback(ctx)
	out, err := s.updatePlatformTx(ctx, tx, id, r)
	if err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}
func (s *Service) updatePlatformTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, r PlatformUpdate) (Platform, error) {
	if r.ExpectedVersion < 1 || (r.Name == nil && r.AccountIdentifier == nil && r.EndpointURL == nil && r.AuthType == nil && r.TimeoutSeconds == nil && r.TypeConfig == nil && r.Note == nil && r.Credential == nil && r.ClearCredential == nil) || !validOptional(r.Credential) || (r.ClearCredential != nil && (!*r.ClearCredential || r.Credential != nil)) {
		return Platform{}, ErrValidation
	}
	var cur Platform
	e := scanPlatform(tx.QueryRow(ctx, "SELECT "+platformColumns+" FROM distribution_platform_configurations WHERE id=$1", id), &cur)
	e = notFound(e)
	if e != nil {
		return Platform{}, e
	}
	if cur.Version != r.ExpectedVersion {
		return Platform{}, ErrVersionConflict
	}
	if r.Name != nil {
		cur.Name = *r.Name
	}
	if r.AccountIdentifier != nil {
		cur.AccountIdentifier = *r.AccountIdentifier
	}
	if r.EndpointURL != nil {
		cur.EndpointURL = *r.EndpointURL
	}
	if r.AuthType != nil {
		cur.AuthType = *r.AuthType
	}
	if r.TimeoutSeconds != nil {
		cur.TimeoutSeconds = *r.TimeoutSeconds
	}
	if r.TypeConfig != nil {
		cur.TypeConfig = r.TypeConfig
	}
	if r.Note != nil {
		cur.Note = *r.Note
	}
	if !validPlatform(cur.Name, cur.PlatformType, cur.AccountIdentifier, cur.EndpointURL, cur.AuthType, cur.TimeoutSeconds, cur.TypeConfig) || !validOptional(r.Credential) || !validNote(cur.Note) {
		return Platform{}, ErrValidation
	}
	var enc any = nil
	fp := cur.CredentialFingerprint
	if r.Credential != nil {
		var value string
		value, _, e = s.seal(*r.Credential)
		enc, fp = value, stringPointer(fingerprint(*r.Credential))
		if e != nil {
			return Platform{}, e
		}
	} else if r.ClearCredential != nil && *r.ClearCredential {
		enc = ""
		fp = nil
	}
	var out Platform
	e = scanPlatform(tx.QueryRow(ctx, "UPDATE distribution_platform_configurations SET name=$2,account_identifier=$3,endpoint_url=$4,auth_type=$5,timeout_seconds=$6,type_config=$7,note=$8,encrypted_credential=CASE WHEN $9::text IS NULL THEN encrypted_credential ELSE NULLIF($9::text,'') END,credential_fingerprint=CASE WHEN $9::text IS NULL THEN credential_fingerprint ELSE $10 END,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$11 RETURNING "+platformColumns, id, cur.Name, cur.AccountIdentifier, cur.EndpointURL, cur.AuthType, cur.TimeoutSeconds, cur.TypeConfig, cur.Note, enc, fp, r.ExpectedVersion), &out)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, s.updateMissingOrConflict(ctx, tx, "distribution_platform_configurations", id)
	}
	if e != nil {
		return out, e
	}
	if e = s.audit(ctx, tx, "update", "distribution_platform", id, safeAudit("update", out.Version, map[string]any{"name": out.Name, "accountIdentifier": out.AccountIdentifier, "endpointUrl": out.EndpointURL, "authType": out.AuthType, "timeoutSeconds": out.TimeoutSeconds, "typeConfigChanged": r.TypeConfig != nil, "noteChanged": r.Note != nil, "credentialChanged": r.Credential != nil, "credentialCleared": r.ClearCredential != nil})); e != nil {
		return out, e
	}
	if r.Credential != nil || r.ClearCredential != nil {
		action := "credential_replace"
		if r.ClearCredential != nil {
			action = "credential_clear"
		}
		if e = s.audit(ctx, tx, action, "distribution_platform", id, safeAudit(action, out.Version, map[string]any{})); e != nil {
			return out, e
		}
	}
	return out, nil
}
func (s *Service) UpdatePlatformIdempotent(ctx context.Context, id uuid.UUID, r PlatformUpdate, key string) (Platform, error) {
	body, err := s.idempotent(ctx, "distribution-platform:update:"+id.String(), key, r, 200, func(tx pgx.Tx) (json.RawMessage, error) {
		out, err := s.updatePlatformTx(ctx, tx, id, r)
		if err != nil {
			return nil, err
		}
		return json.Marshal(out)
	})
	var out Platform
	if err == nil {
		err = json.Unmarshal(body, &out)
	}
	return out, err
}
func validCommon(n, u string, t int) bool {
	return strings.TrimSpace(n) != "" && len(n) <= 120 && validURL(u) && len(u) <= 512 && t >= 5 && t <= 300
}
func validProvider(n, u, m string, t int) bool {
	return validCommon(n, u, t) && strings.TrimSpace(m) != "" && len(m) <= 160
}
func validOptional(x *string) bool { return x == nil || (len(*x) > 0 && len(*x) <= 16384) }
func validWorkflow(n string, st []string, c json.RawMessage, i, o string, p json.RawMessage) bool {
	var params map[string]any
	if strings.TrimSpace(n) == "" || len(n) > 160 || !validN8n(c) || strings.TrimSpace(i) == "" || len(i) > 40 || strings.TrimSpace(o) == "" || len(o) > 40 || json.Unmarshal(defaultJSON(p), &params) != nil || len(st) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range st {
		if seen[x] || (x != "chapter_planning" && x != "content_generation" && x != "review" && x != "rewrite") {
			return false
		}
		seen[x] = true
	}
	return true
}
func normalizedLlmStrategy(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}
func validLlmPolicy(strategy string, providerID *uuid.UUID, model *string) bool {
	switch strategy {
	case "acf_managed":
		return providerID != nil && *providerID != uuid.Nil && model != nil && strings.TrimSpace(*model) != "" && len(*model) <= 200
	case "n8n_managed", "none":
		return providerID == nil && model == nil
	default:
		return false
	}
}
func validNote(v *string) bool { return v == nil || len(*v) <= 5000 }
func validPlatform(n, t, a string, e *string, auth string, timeout int, c json.RawMessage) bool {
	var config map[string]any
	if strings.TrimSpace(n) == "" || len(n) > 120 || strings.TrimSpace(a) == "" || len(a) > 240 || timeout < 5 || timeout > 300 || json.Unmarshal(c, &config) != nil {
		return false
	}
	if t != "wechat_official_account" && t != "douyin" && t != "youtube" && t != "custom" {
		return false
	}
	if !validPlatformAuth(t, auth) {
		return false
	}
	return (e == nil || (len(*e) <= 512 && validURL(*e))) && (t != "custom" || (e != nil && strings.TrimSpace(*e) != "" && validURL(*e)))
}
func (s *Service) secret(v *string) (any, *string, error) {
	if v == nil {
		return nil, nil, nil
	}
	e, f, x := s.seal(*v)
	return e, &f, x
}
func stringPointer(v string) *string { return &v }
func defaultJSON(v json.RawMessage) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage(`{}`)
	}
	return v
}
func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func (s *Service) probeConnection(ctx context.Context, connection Connection) error {
	endpoint, err := s.verificationURL(connection.BaseURL, "healthz")
	if err != nil {
		return err
	}
	return s.probe(ctx, endpoint, connection.TimeoutSeconds, nil, nil)
}

func (s *Service) probeWorkflow(ctx context.Context, connection Connection, workflow Workflow) error {
	var cfg struct{ ReferenceType, ReferenceValue string }
	if err := json.Unmarshal(workflow.TypeConfig, &cfg); err != nil || cfg.ReferenceValue == "" || len(workflow.ApplicableStages) == 0 {
		return ErrVerification
	}
	endpoint, err := s.verificationURL(connection.BaseURL, path.Join("webhook", cfg.ReferenceValue))
	if err != nil {
		return err
	}
	for _, stage := range workflow.ApplicableStages {
		requestID := uuid.NewString()
		body, _ := json.Marshal(map[string]string{"probeType": "acf_workflow_verification", "stage": stage, "contractVersion": workflow.InputContractVersion, "requestId": requestID})
		var response struct {
			Verified        bool   `json:"verified"`
			Stage           string `json:"stage"`
			ContractVersion string `json:"contractVersion"`
			RequestID       string `json:"requestId"`
		}
		if err = s.probe(ctx, endpoint, connection.TimeoutSeconds, body, &response); err != nil {
			return err
		}
		if !response.Verified || response.Stage != stage || response.ContractVersion != workflow.InputContractVersion || response.RequestID != requestID {
			return ErrVerification
		}
	}
	return nil
}

func (s *Service) probe(ctx context.Context, endpoint string, timeoutSeconds int, body []byte, response any) error {
	if timeoutSeconds < 5 || timeoutSeconds > 300 {
		return ErrVerification
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	method := http.MethodGet
	var reader io.Reader
	if body != nil {
		method, reader = http.MethodPost, strings.NewReader(string(body))
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return ErrVerification
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := s.verificationHTTPClient()
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return &safehttp.Error{Code: "upstream_unavailable", Message: "The integration service returned an unsuccessful response.", Retryable: res.StatusCode >= 500}
	}
	if response != nil {
		body, readErr := io.ReadAll(io.LimitReader(res.Body, 64*1024+1))
		if readErr != nil {
			return &safehttp.Error{Code: "upstream_unavailable", Message: "The integration response could not be read.", Retryable: true}
		}
		if len(body) > 64*1024 {
			return &safehttp.Error{Code: safehttp.CodeResponseTooLarge, Message: "The integration response is too large.", Retryable: false}
		}
		if json.Unmarshal(body, response) != nil {
			return &safehttp.Error{Code: "upstream_unavailable", Message: "The integration response was invalid.", Retryable: false}
		}
	}
	return nil
}

func (s *Service) verificationHTTPClient() *http.Client {
	// Credentialed integration client: no proxy, no redirects, shared policy.
	return safehttp.New(s.credentialOutboundPolicy()).HTTPClient()
}

// RuntimeHTTPClient is the single outbound client for n8n Execute/Query and any
// other runtime call that may attach connection credentials.
func (s *Service) RuntimeHTTPClient() *http.Client {
	return s.verificationHTTPClient()
}

// N8NRuntimeConfigured reports durable n8n runtime capability including
// credential presence and decryptability with the current encryption key.
// It does not perform outbound network probes against n8n.
func (s *Service) N8NRuntimeConfigured(ctx context.Context) bool {
	if s == nil || s.pool == nil {
		return false
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, encrypted_credential, credential_fingerprint, type_config
		FROM workflow_connections
		WHERE connection_type='n8n'
		  AND enabled=true
		  AND integration_status='verified'
		  AND last_verified_version=version
		ORDER BY updated_at DESC, id ASC`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var encrypted *string
		var fingerprint *string
		var typeConfig json.RawMessage
		if err = rows.Scan(&id, &encrypted, &fingerprint, &typeConfig); err != nil {
			return false
		}
		if fingerprint == nil || strings.TrimSpace(*fingerprint) == "" {
			continue
		}
		if encrypted == nil || strings.TrimSpace(*encrypted) == "" {
			continue
		}
		if !validN8n(typeConfig) {
			continue
		}
		plain, unsealErr := s.unseal(*encrypted)
		if unsealErr != nil || strings.TrimSpace(plain) == "" {
			// Controlled diagnostic only; never return decrypt detail to callers.
			log.Printf("n8n runtime credential unavailable connection_id=%s", id)
			continue
		}
		return true
	}
	_ = rows.Err()
	return false
}

// verificationURL validates only syntax. Every resolved address is checked in
// DialContext immediately before use, which prevents DNS rebinding bypasses.
func verificationURL(baseURL, suffix string) (string, error) {
	return verificationURLWithPolicy(baseURL, suffix, integrationOutboundPolicy())
}

func verificationURLWithPolicy(baseURL, suffix string, policy safehttp.Policy) (string, error) {
	u, err := safehttp.NormalizeURL(baseURL, policy)
	if err != nil {
		return "", ErrVerification
	}
	u.Path = path.Join(u.Path, suffix)
	u.RawQuery, u.Fragment = "", ""
	return u.String(), nil
}

func (s *Service) verificationURL(baseURL, suffix string) (string, error) {
	return verificationURLWithPolicy(baseURL, suffix, s.credentialOutboundPolicy())
}

func verificationAddressAllowed(host string, ip net.IP) bool {
	policy := integrationOutboundPolicy()
	policy.Resolver = func(context.Context, string) ([]net.IP, error) { return []net.IP{ip}, nil }
	u := &url.URL{Scheme: "https", Host: host}
	_, err := safehttp.ValidateDestination(context.Background(), u, policy)
	return err == nil
}

// integrationOutboundPolicy is the package-level credential policy used for URL
// normalization before a Service instance is available (e.g. validation helpers).
func integrationOutboundPolicy() safehttp.Policy {
	return safehttp.CredentialPolicy(strings.TrimSpace(os.Getenv("APP_ENV")))
}

func (s *Service) credentialOutboundPolicy() safehttp.Policy {
	policy := safehttp.CredentialPolicy(s.runtimeEnvironment())
	if s != nil {
		if s.resolveHost != nil {
			policy.Resolver = s.resolveHost
		}
		if s.dialContext != nil {
			policy.DialContext = s.dialContext
		}
	}
	return policy
}

// outboundPolicy is the shared credential-bearing outbound policy for Provider
// Verify, Model Discovery, Connection Verify and n8n runtime clients.
func (s *Service) outboundPolicy() safehttp.Policy {
	return s.credentialOutboundPolicy()
}

func (s *Service) idempotent(ctx context.Context, scope, key string, request any, responseStatus int, fn func(pgx.Tx) (json.RawMessage, error)) (json.RawMessage, error) {
	if strings.TrimSpace(key) == "" || len(key) > 128 {
		return nil, ErrValidation
	}
	b, _ := json.Marshal(request)
	h := sha256.Sum256(b)
	hash := hex.EncodeToString(h[:])
	if s.beforeIdempotencyLock != nil {
		s.beforeIdempotencyLock()
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", scope+":"+key); e != nil {
		return nil, fmt.Errorf("lock idempotency request: %w", e)
	}
	if r, e := idempotency.NewPostgresRepositoryTx(tx).Get(ctx, scope, key); e == nil {
		if r.RequestHash != hash {
			return nil, ErrIdempotency
		}
		return r.ResponseBody, nil
	} else if !errors.Is(e, idempotency.ErrNotFound) {
		return nil, e
	}
	body, e := fn(tx)
	if e != nil {
		return nil, unique(e)
	}
	_, e = idempotency.NewPostgresRepositoryTx(tx).Create(ctx, idempotency.Record{ID: uuid.New(), Scope: scope, Key: key, RequestHash: hash, ResponseStatus: responseStatus, ResponseBody: body})
	if e != nil {
		return nil, ErrIdempotency
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, e
	}
	return body, nil
}
func (s *Service) audit(ctx context.Context, tx pgx.Tx, action, subject string, id uuid.UUID, payload any) error {
	b, _ := json.Marshal(payload)
	return audit.NewRepository(tx).Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: "system", Action: subject + "." + action, SubjectType: subject, SubjectID: id.String(), Payload: b})
}
func safeAudit(operation string, version int, changes map[string]any) map[string]any {
	allowed := map[string]struct{}{
		"name": {}, "defaultModel": {}, "timeoutSeconds": {},
		"secretChanged": {}, "secretCleared": {}, "authType": {},
		"typeConfigChanged": {}, "credentialChanged": {}, "credentialCleared": {},
		"connectionId": {}, "applicableStages": {}, "inputContractVersion": {},
		"outputContractVersion": {}, "defaultParametersChanged": {}, "noteChanged": {},
		"accountIdentifier": {}, "validationStatus": {}, "errorCode": {},
	}
	safeChanges := make(map[string]any, len(changes))
	for key, value := range changes {
		if _, ok := allowed[key]; ok {
			safeChanges[key] = value
		}
	}
	if _, ok := changes["baseUrl"]; ok {
		safeChanges["baseUrlChanged"] = true
	}
	if _, ok := changes["endpointUrl"]; ok {
		safeChanges["endpointUrlChanged"] = true
	}
	return map[string]any{"operation": operation, "version": version, "changes": safeChanges}
}
func (s *Service) updateMissingOrConflict(ctx context.Context, tx pgx.Tx, table string, id uuid.UUID) error {
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+table+" WHERE id=$1)", id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	return ErrVersionConflict
}
func unique(e error) error {
	var p *pgconn.PgError
	if errors.As(e, &p) && p.Code == "23505" {
		return ErrNameConflict
	}
	return e
}
func notFound(e error) error {
	if errors.Is(e, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return e
}

type scanner interface{ Scan(...any) error }

const providerColumns = "id,name,provider_type,base_url,default_model,timeout_seconds,encrypted_secret IS NOT NULL,secret_fingerprint,integration_status,enabled,last_verified_version,validation_details,last_verified_at,last_error_code,last_error_message,version,created_at,updated_at"

func scanProvider(r scanner, x *Provider) error {
	err := r.Scan(&x.ID, &x.Name, &x.ProviderType, &x.BaseURL, &x.DefaultModel, &x.TimeoutSeconds, &x.HasSecret, &x.SecretFingerprint, &x.IntegrationStatus, &x.Enabled, &x.VerifiedVersion, &x.ValidationDetails, &x.LastVerifiedAt, &x.LastErrorCode, &x.LastErrorMessage, &x.Version, &x.CreatedAt, &x.UpdatedAt)
	finalizeCommon(&x.Common, "provider")
	return err
}

const connectionColumns = "id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,encrypted_credential IS NOT NULL,encrypted_credential,credential_fingerprint,integration_status,enabled,last_verified_version,validation_details,last_verified_at,last_error_code,last_error_message,version,created_at,updated_at"

func scanConnection(r scanner, x *Connection) error {
	err := r.Scan(&x.ID, &x.Name, &x.ConnectionType, &x.BaseURL, &x.AuthType, &x.TimeoutSeconds, &x.TypeConfig, &x.HasCredential, &x.encryptedCredential, &x.CredentialFingerprint, &x.IntegrationStatus, &x.Enabled, &x.VerifiedVersion, &x.ValidationDetails, &x.LastVerifiedAt, &x.LastErrorCode, &x.LastErrorMessage, &x.Version, &x.CreatedAt, &x.UpdatedAt)
	finalizeCommon(&x.Common, "connection")
	return err
}

const workflowColumns = "w.id,w.name,w.connection_id,c.name,c.connection_type,'n8n',w.applicable_stages,w.type_config,w.input_contract_version,w.output_contract_version,w.default_parameters,w.note,w.llm_strategy,w.llm_provider_id,w.llm_model,w.resolved_webhook_path,w.resolved_workflow_id,w.resolved_workflow_revision,w.integration_status,w.enabled,w.last_verified_version,w.validation_details,w.last_verified_at,w.last_error_code,w.last_error_message,w.version,w.created_at,w.updated_at"

func scanWorkflow(r scanner, x *Workflow) error {
	var raw json.RawMessage
	e := r.Scan(&x.ID, &x.Name, &x.ConnectionID, &x.ConnectionName, &x.ConnectionType, &x.WorkflowType, &raw, &x.TypeConfig, &x.InputContractVersion, &x.OutputContractVersion, &x.DefaultParameters, &x.Note, &x.LlmStrategy, &x.LlmProviderID, &x.LlmModel, &x.ResolvedWebhookPath, &x.ResolvedWorkflowID, &x.ResolvedRevision, &x.IntegrationStatus, &x.Enabled, &x.VerifiedVersion, &x.ValidationDetails, &x.LastVerifiedAt, &x.LastErrorCode, &x.LastErrorMessage, &x.Version, &x.CreatedAt, &x.UpdatedAt)
	if e == nil {
		e = json.Unmarshal(raw, &x.ApplicableStages)
	}
	x.IneligibilityReasons = finalizeCommon(&x.Common, "workflow_configuration")
	return e
}

func finalizeCommon(x *Common, kind string) []IneligibilityReason {
	x.ValidationStatus = x.IntegrationStatus
	var reasons []IneligibilityReason
	x.Executable, reasons = EvaluateEligibility(EligibilityFact{
		Kind: kind, ResourceID: &x.ID, Status: ValidationStatus(x.IntegrationStatus), Enabled: x.Enabled,
		Version: x.Version, VerifiedVersion: x.VerifiedVersion, ModelAvailable: true,
		StrategyComplete: true, ReferenceExists: true, ReferenceActive: true,
		StageMatches: true, InputCompatible: true, OutputCompatible: true,
	})
	return reasons
}

const platformColumns = "id,name,platform_type,account_identifier,endpoint_url,auth_type,timeout_seconds,type_config,note,encrypted_credential IS NOT NULL,credential_fingerprint,integration_status,enabled,last_verified_at,last_error_code,last_error_message,version,created_at,updated_at"

func scanPlatform(r scanner, x *Platform) error {
	err := r.Scan(&x.ID, &x.Name, &x.PlatformType, &x.AccountIdentifier, &x.EndpointURL, &x.AuthType, &x.TimeoutSeconds, &x.TypeConfig, &x.Note, &x.HasCredential, &x.CredentialFingerprint, &x.IntegrationStatus, &x.Enabled, &x.LastVerifiedAt, &x.LastErrorCode, &x.LastErrorMessage, &x.Version, &x.CreatedAt, &x.UpdatedAt)
	// Platform previously skipped finalize, leaving validationStatus/executable at zero values.
	finalizeCommon(&x.Common, "distribution_platform")
	return err
}

func paginateSlice[T any](items []T, limit, offset int) []T {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}
	if offset >= len(items) {
		return []T{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func where(o ListOptions, typeColumn string, _ any) (string, []any) {
	a := []any{}
	p := []string{}
	if strings.TrimSpace(o.Query) != "" {
		a = append(a, "%"+strings.TrimSpace(o.Query)+"%")
		p = append(p, fmt.Sprintf("name ILIKE $%d", len(a)))
	}
	if typeColumn != "" && o.Type != "" {
		a = append(a, o.Type)
		p = append(p, fmt.Sprintf("%s=$%d", typeColumn, len(a)))
	}
	if o.IntegrationStatus != "" {
		a = append(a, o.IntegrationStatus)
		p = append(p, fmt.Sprintf("integration_status=$%d", len(a)))
	}
	if o.Enabled != nil {
		a = append(a, *o.Enabled)
		p = append(p, fmt.Sprintf("enabled=$%d", len(a)))
	}
	if o.ValidationStatus != "" {
		a = append(a, o.ValidationStatus)
		p = append(p, fmt.Sprintf("integration_status=$%d", len(a)))
	}
	if len(p) == 0 {
		return "", a
	}
	return " WHERE " + strings.Join(p, " AND "), a
}

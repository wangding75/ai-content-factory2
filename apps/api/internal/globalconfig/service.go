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
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
)

var (
	ErrNotFound        = errors.New("configuration not found")
	ErrValidation      = errors.New("invalid configuration")
	ErrVersionConflict = errors.New("configuration version conflict")
	ErrIdempotency     = errors.New("idempotency key reused with different payload")
)

type Service struct {
	pool                  *pgxpool.Pool
	key                   []byte
	beforeIdempotencyLock func()
}

func NewService(pool *pgxpool.Pool, encryptionKey string) (*Service, error) {
	b := sha256.Sum256([]byte(encryptionKey))
	if strings.TrimSpace(encryptionKey) == "" {
		return nil, errors.New("CONFIGURATION_ENCRYPTION_KEY is required")
	}
	return &Service{pool: pool, key: b[:]}, nil
}

type Common struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name"`
	IntegrationStatus string     `json:"integrationStatus"`
	Enabled           bool       `json:"enabled"`
	LastVerifiedAt    *time.Time `json:"lastVerifiedAt"`
	LastErrorCode     *string    `json:"lastErrorCode"`
	LastErrorMessage  *string    `json:"lastErrorMessage"`
	Version           int        `json:"version"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
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
}
type Workflow struct {
	Common
	ConnectionID          uuid.UUID       `json:"connectionId"`
	ConnectionName        string          `json:"connectionName"`
	ConnectionType        string          `json:"connectionType"`
	WorkflowType          string          `json:"workflowType"`
	ApplicableStages      []string        `json:"applicableStages"`
	TypeConfig            json.RawMessage `json:"typeConfig"`
	InputContractVersion  string          `json:"inputContractVersion"`
	OutputContractVersion string          `json:"outputContractVersion"`
	DefaultParameters     json.RawMessage `json:"defaultParameters"`
	Note                  *string         `json:"note"`
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
	Query, Type, ConnectionID string
	Limit, Offset             int
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

func validURL(x string) bool {
	u, e := url.ParseRequestURI(x)
	return e == nil && u.Scheme != "" && u.Host != ""
}
func validN8n(x json.RawMessage) bool {
	var v struct {
		ReferenceType  string `json:"referenceType"`
		ReferenceValue string `json:"referenceValue"`
	}
	return json.Unmarshal(x, &v) == nil && (v.ReferenceType == "workflow_id" || v.ReferenceType == "webhook_path") && strings.TrimSpace(v.ReferenceValue) != "" && len(v.ReferenceValue) <= 512
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
	h := sha256.Sum256([]byte(v))
	return hex.EncodeToString(h[:])[:32]
}
func (s *Service) CreateProvider(ctx context.Context, r ProviderCreate, key string) (Provider, error) {
	if r.ProviderType != "openai_compatible" || !validProvider(r.Name, r.BaseURL, r.DefaultModel, r.TimeoutSeconds) || !validOptional(r.Secret) {
		return Provider{}, ErrValidation
	}
	body, err := s.idempotent(ctx, "llm-provider:create", key, r, func(tx pgx.Tx) (json.RawMessage, error) {
		var out Provider
		enc, fp, e := s.secret(r.Secret)
		if e != nil {
			return nil, e
		}
		row := tx.QueryRow(ctx, "INSERT INTO llm_provider_configurations(id,name,provider_type,base_url,default_model,encrypted_secret,secret_fingerprint,timeout_seconds) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,name,provider_type,base_url,default_model,timeout_seconds,encrypted_secret IS NOT NULL,secret_fingerprint,integration_status,enabled,last_verified_at,last_error_code,last_error_message,version,created_at,updated_at", uuid.New(), r.Name, r.ProviderType, r.BaseURL, r.DefaultModel, enc, fp, r.TimeoutSeconds)
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
	e := scanProvider(s.pool.QueryRow(ctx, "SELECT id,name,provider_type,base_url,default_model,timeout_seconds,encrypted_secret IS NOT NULL,secret_fingerprint,integration_status,enabled,last_verified_at,last_error_code,last_error_message,version,created_at,updated_at FROM llm_provider_configurations WHERE id=$1", id), &x)
	return x, notFound(e)
}
func (s *Service) ListProviders(ctx context.Context, o ListOptions) ([]Provider, int, error) {
	q, args := where(o, "", nil)
	var total int
	if e := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM llm_provider_configurations"+q, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	args = append(args, o.Limit, o.Offset)
	rows, e := s.pool.Query(ctx, "SELECT id,name,provider_type,base_url,default_model,timeout_seconds,encrypted_secret IS NOT NULL,secret_fingerprint,integration_status,enabled,last_verified_at,last_error_code,last_error_message,version,created_at,updated_at FROM llm_provider_configurations"+q+fmt.Sprintf(" ORDER BY updated_at DESC,id ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	xs := []Provider{}
	for rows.Next() {
		var x Provider
		if e = scanProvider(rows, &x); e != nil {
			return nil, 0, e
		}
		xs = append(xs, x)
	}
	return xs, total, rows.Err()
}
func (s *Service) UpdateProvider(ctx context.Context, id uuid.UUID, r ProviderUpdate) (Provider, error) {
	if r.ExpectedVersion < 1 || (r.Name == nil && r.BaseURL == nil && r.DefaultModel == nil && r.TimeoutSeconds == nil && r.Secret == nil && r.ClearSecret == nil) || !validOptional(r.Secret) || r.ClearSecret != nil && !*r.ClearSecret {
		return Provider{}, ErrValidation
	}
	cur, e := s.GetProvider(ctx, id)
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
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	e = scanProvider(tx.QueryRow(ctx, "UPDATE llm_provider_configurations SET name=$2,base_url=$3,default_model=$4,timeout_seconds=$5,encrypted_secret=CASE WHEN $6::text IS NULL THEN encrypted_secret ELSE NULLIF($6::text,'') END,secret_fingerprint=CASE WHEN $6::text IS NULL THEN secret_fingerprint ELSE $7 END,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$8 RETURNING id,name,provider_type,base_url,default_model,timeout_seconds,encrypted_secret IS NOT NULL,secret_fingerprint,integration_status,enabled,last_verified_at,last_error_code,last_error_message,version,created_at,updated_at", id, cur.Name, cur.BaseURL, cur.DefaultModel, cur.TimeoutSeconds, enc, fp, r.ExpectedVersion), &out)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, s.updateMissingOrConflict(ctx, tx, "llm_provider_configurations", id)
	}
	if e != nil {
		return out, e
	}
	if e = s.audit(ctx, tx, "update", "llm_provider", id, safeAudit("update", out.Version, map[string]any{"name": out.Name, "baseUrl": out.BaseURL, "defaultModel": out.DefaultModel, "timeoutSeconds": out.TimeoutSeconds, "secretChanged": r.Secret != nil, "secretCleared": r.ClearSecret != nil})); e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	return out, nil
}
func (s *Service) CreateConnection(ctx context.Context, r ConnectionCreate, key string) (Connection, error) {
	if r.ConnectionType != "n8n" || r.AuthType != "api_key" || !validCommon(r.Name, r.BaseURL, r.TimeoutSeconds) || !validN8n(r.TypeConfig) || !validOptional(r.Credential) {
		return Connection{}, ErrValidation
	}
	body, err := s.idempotent(ctx, "workflow-connection:create", key, r, func(tx pgx.Tx) (json.RawMessage, error) {
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
	return x, notFound(e)
}
func (s *Service) ListConnections(ctx context.Context, o ListOptions) ([]Connection, int, error) {
	q, args := where(o, "connection_type", nil)
	var n int
	if e := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM workflow_connections"+q, args...).Scan(&n); e != nil {
		return nil, 0, e
	}
	args = append(args, o.Limit, o.Offset)
	rows, e := s.pool.Query(ctx, "SELECT "+connectionColumns+" FROM workflow_connections"+q+fmt.Sprintf(" ORDER BY updated_at DESC,id ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		var x Connection
		if e = scanConnection(rows, &x); e != nil {
			return nil, 0, e
		}
		out = append(out, x)
	}
	return out, n, rows.Err()
}
func (s *Service) UpdateConnection(ctx context.Context, id uuid.UUID, r ConnectionUpdate) (Connection, error) {
	if r.ExpectedVersion < 1 || (r.Name == nil && r.BaseURL == nil && r.AuthType == nil && r.TimeoutSeconds == nil && r.TypeConfig == nil && r.Credential == nil && r.ClearCredential == nil) || !validOptional(r.Credential) || r.ClearCredential != nil && !*r.ClearCredential {
		return Connection{}, ErrValidation
	}
	cur, e := s.GetConnection(ctx, id)
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
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	e = scanConnection(tx.QueryRow(ctx, "UPDATE workflow_connections SET name=$2,base_url=$3,auth_type=$4,timeout_seconds=$5,type_config=$6,encrypted_credential=CASE WHEN $7::text IS NULL THEN encrypted_credential ELSE NULLIF($7::text,'') END,credential_fingerprint=CASE WHEN $7::text IS NULL THEN credential_fingerprint ELSE $8 END,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$9 RETURNING "+connectionColumns, id, cur.Name, cur.BaseURL, cur.AuthType, cur.TimeoutSeconds, cur.TypeConfig, enc, fp, r.ExpectedVersion), &out)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, s.updateMissingOrConflict(ctx, tx, "workflow_connections", id)
	}
	if e != nil {
		return out, e
	}
	if e = s.audit(ctx, tx, "update", "workflow_connection", id, safeAudit("update", out.Version, map[string]any{"name": out.Name, "baseUrl": out.BaseURL, "authType": out.AuthType, "timeoutSeconds": out.TimeoutSeconds, "typeConfigChanged": r.TypeConfig != nil, "credentialChanged": r.Credential != nil, "credentialCleared": r.ClearCredential != nil})); e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	return out, nil
}
func (s *Service) CreateWorkflow(ctx context.Context, r WorkflowCreate, key string) (Workflow, error) {
	if !validWorkflow(r.Name, r.ApplicableStages, r.TypeConfig, r.InputContractVersion, r.OutputContractVersion, r.DefaultParameters) {
		return Workflow{}, ErrValidation
	}
	if _, e := s.GetConnection(ctx, r.ConnectionID); e != nil {
		return Workflow{}, e
	}
	body, err := s.idempotent(ctx, "workflow-configuration:create", key, r, func(tx pgx.Tx) (json.RawMessage, error) {
		id := uuid.New()
		_, e := tx.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version,default_parameters,note) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)", id, r.Name, r.ConnectionID, mustJSON(r.ApplicableStages), r.TypeConfig, r.InputContractVersion, r.OutputContractVersion, defaultJSON(r.DefaultParameters), r.Note)
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
	var x Workflow
	e := scanWorkflow(s.pool.QueryRow(ctx, "SELECT "+workflowColumns+" FROM workflow_configurations w JOIN workflow_connections c ON c.id=w.connection_id WHERE w.id=$1", id), &x)
	return x, notFound(e)
}
func (s *Service) ListWorkflows(ctx context.Context, o ListOptions) ([]Workflow, int, error) {
	where := ""
	args := []any{}
	if strings.TrimSpace(o.Query) != "" {
		args = append(args, "%"+strings.TrimSpace(o.Query)+"%")
		where = " WHERE w.name ILIKE $1"
	}
	if o.ConnectionID != "" {
		args = append(args, o.ConnectionID)
		where += map[bool]string{true: " AND", false: " WHERE"}[where != ""] + fmt.Sprintf(" w.connection_id=$%d", len(args))
	}
	var n int
	if e := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM workflow_configurations w"+where, args...).Scan(&n); e != nil {
		return nil, 0, e
	}
	args = append(args, o.Limit, o.Offset)
	rows, e := s.pool.Query(ctx, "SELECT "+workflowColumns+" FROM workflow_configurations w JOIN workflow_connections c ON c.id=w.connection_id"+where+fmt.Sprintf(" ORDER BY w.updated_at DESC,w.id ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := []Workflow{}
	for rows.Next() {
		var x Workflow
		if e = scanWorkflow(rows, &x); e != nil {
			return nil, 0, e
		}
		out = append(out, x)
	}
	return out, n, rows.Err()
}
func (s *Service) CreatePlatform(ctx context.Context, r PlatformCreate, key string) (Platform, error) {
	if !validPlatform(r.Name, r.PlatformType, r.AccountIdentifier, r.EndpointURL, r.AuthType, r.TimeoutSeconds, r.TypeConfig) || !validOptional(r.Credential) {
		return Platform{}, ErrValidation
	}
	body, err := s.idempotent(ctx, "distribution-platform:create", key, r, func(tx pgx.Tx) (json.RawMessage, error) {
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
	q, args := where(o, "platform_type", nil)
	var n int
	if e := s.pool.QueryRow(ctx, "SELECT COUNT(*) FROM distribution_platform_configurations"+q, args...).Scan(&n); e != nil {
		return nil, 0, e
	}
	args = append(args, o.Limit, o.Offset)
	rows, e := s.pool.Query(ctx, "SELECT "+platformColumns+" FROM distribution_platform_configurations"+q+fmt.Sprintf(" ORDER BY updated_at DESC,id ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	out := []Platform{}
	for rows.Next() {
		var x Platform
		if e = scanPlatform(rows, &x); e != nil {
			return nil, 0, e
		}
		out = append(out, x)
	}
	return out, n, rows.Err()
}
func (s *Service) UpdateWorkflow(ctx context.Context, id uuid.UUID, r WorkflowUpdate) (Workflow, error) {
	if r.ExpectedVersion < 1 {
		return Workflow{}, ErrValidation
	}
	cur, e := s.GetWorkflow(ctx, id)
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
	if !validWorkflow(cur.Name, cur.ApplicableStages, cur.TypeConfig, cur.InputContractVersion, cur.OutputContractVersion, cur.DefaultParameters) {
		return Workflow{}, ErrValidation
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return Workflow{}, e
	}
	defer tx.Rollback(ctx)
	tag, e := tx.Exec(ctx, "UPDATE workflow_configurations SET name=$2,connection_id=$3,applicable_stages=$4,type_config=$5,input_contract_version=$6,output_contract_version=$7,default_parameters=$8,note=$9,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$10", id, cur.Name, cur.ConnectionID, mustJSON(cur.ApplicableStages), cur.TypeConfig, cur.InputContractVersion, cur.OutputContractVersion, cur.DefaultParameters, cur.Note, r.ExpectedVersion)
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
	if e = tx.Commit(ctx); e != nil {
		return Workflow{}, e
	}
	return out, nil
}
func (s *Service) UpdatePlatform(ctx context.Context, id uuid.UUID, r PlatformUpdate) (Platform, error) {
	if r.ExpectedVersion < 1 {
		return Platform{}, ErrValidation
	}
	cur, e := s.GetPlatform(ctx, id)
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
	if !validPlatform(cur.Name, cur.PlatformType, cur.AccountIdentifier, cur.EndpointURL, cur.AuthType, cur.TimeoutSeconds, cur.TypeConfig) || !validOptional(r.Credential) {
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
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
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
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	return out, nil
}
func validCommon(n, u string, t int) bool {
	return strings.TrimSpace(n) != "" && len(n) <= 120 && validURL(u) && len(u) <= 512 && t >= 5 && t <= 300
}
func validProvider(n, u, m string, t int) bool {
	return validCommon(n, u, t) && strings.TrimSpace(m) != "" && len(m) <= 160
}
func validOptional(x *string) bool { return x == nil || (len(*x) > 0 && len(*x) <= 16384) }
func validWorkflow(n string, st []string, c json.RawMessage, i, o string, p json.RawMessage) bool {
	if strings.TrimSpace(n) == "" || len(n) > 160 || !validN8n(c) || strings.TrimSpace(i) == "" || len(i) > 40 || strings.TrimSpace(o) == "" || len(o) > 40 || !json.Valid(defaultJSON(p)) || len(st) == 0 {
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
func validPlatform(n, t, a string, e *string, auth string, timeout int, c json.RawMessage) bool {
	if strings.TrimSpace(n) == "" || len(n) > 120 || strings.TrimSpace(a) == "" || len(a) > 240 || timeout < 5 || timeout > 300 || !json.Valid(c) {
		return false
	}
	if t != "wechat_official_account" && t != "douyin" && t != "youtube" && t != "custom" {
		return false
	}
	if auth != "api_key" && auth != "oauth" && auth != "access_token" && auth != "custom" {
		return false
	}
	return t != "custom" || (e != nil && validURL(*e))
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
func (s *Service) idempotent(ctx context.Context, scope, key string, request any, fn func(pgx.Tx) (json.RawMessage, error)) (json.RawMessage, error) {
	if strings.TrimSpace(key) == "" || len(key) > 128 {
		return nil, ErrValidation
	}
	b, _ := json.Marshal(request)
	h := sha256.Sum256(b)
	hash := hex.EncodeToString(h[:])
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback(ctx)
	if s.beforeIdempotencyLock != nil {
		s.beforeIdempotencyLock()
	}
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
	_, e = idempotency.NewPostgresRepositoryTx(tx).Create(ctx, idempotency.Record{ID: uuid.New(), Scope: scope, Key: key, RequestHash: hash, ResponseStatus: 201, ResponseBody: body})
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
		"name": {}, "baseUrl": {}, "defaultModel": {}, "timeoutSeconds": {},
		"secretChanged": {}, "secretCleared": {}, "authType": {},
		"typeConfigChanged": {}, "credentialChanged": {}, "credentialCleared": {},
		"connectionId": {}, "applicableStages": {}, "inputContractVersion": {},
		"outputContractVersion": {}, "defaultParametersChanged": {}, "noteChanged": {},
		"accountIdentifier": {}, "endpointUrl": {},
	}
	safeChanges := make(map[string]any, len(changes))
	for key, value := range changes {
		if _, ok := allowed[key]; ok {
			safeChanges[key] = value
		}
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
		return ErrValidation
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

func scanProvider(r scanner, x *Provider) error {
	return r.Scan(&x.ID, &x.Name, &x.ProviderType, &x.BaseURL, &x.DefaultModel, &x.TimeoutSeconds, &x.HasSecret, &x.SecretFingerprint, &x.IntegrationStatus, &x.Enabled, &x.LastVerifiedAt, &x.LastErrorCode, &x.LastErrorMessage, &x.Version, &x.CreatedAt, &x.UpdatedAt)
}

const connectionColumns = "id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,encrypted_credential IS NOT NULL,credential_fingerprint,integration_status,enabled,last_verified_at,last_error_code,last_error_message,version,created_at,updated_at"

func scanConnection(r scanner, x *Connection) error {
	return r.Scan(&x.ID, &x.Name, &x.ConnectionType, &x.BaseURL, &x.AuthType, &x.TimeoutSeconds, &x.TypeConfig, &x.HasCredential, &x.CredentialFingerprint, &x.IntegrationStatus, &x.Enabled, &x.LastVerifiedAt, &x.LastErrorCode, &x.LastErrorMessage, &x.Version, &x.CreatedAt, &x.UpdatedAt)
}

const workflowColumns = "w.id,w.name,w.connection_id,c.name,c.connection_type,'n8n',w.applicable_stages,w.type_config,w.input_contract_version,w.output_contract_version,w.default_parameters,w.note,w.integration_status,w.enabled,w.last_verified_at,w.last_error_code,w.last_error_message,w.version,w.created_at,w.updated_at"

func scanWorkflow(r scanner, x *Workflow) error {
	var raw json.RawMessage
	e := r.Scan(&x.ID, &x.Name, &x.ConnectionID, &x.ConnectionName, &x.ConnectionType, &x.WorkflowType, &raw, &x.TypeConfig, &x.InputContractVersion, &x.OutputContractVersion, &x.DefaultParameters, &x.Note, &x.IntegrationStatus, &x.Enabled, &x.LastVerifiedAt, &x.LastErrorCode, &x.LastErrorMessage, &x.Version, &x.CreatedAt, &x.UpdatedAt)
	if e == nil {
		e = json.Unmarshal(raw, &x.ApplicableStages)
	}
	return e
}

const platformColumns = "id,name,platform_type,account_identifier,endpoint_url,auth_type,timeout_seconds,type_config,note,encrypted_credential IS NOT NULL,credential_fingerprint,integration_status,enabled,last_verified_at,last_error_code,last_error_message,version,created_at,updated_at"

func scanPlatform(r scanner, x *Platform) error {
	return r.Scan(&x.ID, &x.Name, &x.PlatformType, &x.AccountIdentifier, &x.EndpointURL, &x.AuthType, &x.TimeoutSeconds, &x.TypeConfig, &x.Note, &x.HasCredential, &x.CredentialFingerprint, &x.IntegrationStatus, &x.Enabled, &x.LastVerifiedAt, &x.LastErrorCode, &x.LastErrorMessage, &x.Version, &x.CreatedAt, &x.UpdatedAt)
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
	if len(p) == 0 {
		return "", a
	}
	return " WHERE " + strings.Join(p, " AND "), a
}

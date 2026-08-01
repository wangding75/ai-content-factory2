package globalconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
)

type ValidationResource string

const (
	ValidationResourceProvider   ValidationResource = "llm_provider"
	ValidationResourceConnection ValidationResource = "workflow_connection"
	ValidationResourceWorkflow   ValidationResource = "workflow_configuration"
)

type ValidationAttempt struct {
	Resource        ValidationResource
	ID              uuid.UUID
	ExpectedVersion int
	StartedAt       time.Time
}

type ValidationOutcome struct {
	Success bool
	Code    string
	Message string
	Details json.RawMessage
}

type ValidationCommandResult struct {
	ID                   uuid.UUID             `json:"id"`
	ValidationStatus     ValidationStatus      `json:"validationStatus"`
	VerifiedVersion      *int                  `json:"verifiedVersion"`
	CheckedAt            time.Time             `json:"checkedAt"`
	Executable           bool                  `json:"executable"`
	IneligibilityReasons []IneligibilityReason `json:"ineligibilityReasons"`
	SafeError            *ValidationSafeError  `json:"safeError"`
}

type ValidationSafeError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type EnableCommandResult struct {
	ID                   uuid.UUID             `json:"id"`
	Enabled              bool                  `json:"enabled"`
	ValidationStatus     ValidationStatus      `json:"validationStatus"`
	Version              int                   `json:"version"`
	Executable           bool                  `json:"executable"`
	IneligibilityReasons []IneligibilityReason `json:"ineligibilityReasons"`
}

func (s *Service) SetResourceEnabled(ctx context.Context, resource ValidationResource, id uuid.UUID, expectedVersion int, enabled bool, key string) (EnableCommandResult, error) {
	table, subject, metadataErr := validationResourceMetadata(resource)
	if metadataErr != nil || id == uuid.Nil || expectedVersion < 1 {
		return EnableCommandResult{}, ErrValidation
	}
	action := "disable"
	if enabled {
		action = "enable"
	}
	request := struct {
		ExpectedVersion int  `json:"expectedVersion"`
		Enabled         bool `json:"enabled"`
	}{expectedVersion, enabled}
	body, err := s.idempotent(ctx, validationCommandPrefix(resource)+":"+action+":"+id.String(), key, request, 200, func(tx pgx.Tx) (json.RawMessage, error) {
		var currentEnabled bool
		var status string
		var version int
		var verifiedVersion *int
		if queryErr := tx.QueryRow(ctx, "SELECT enabled,integration_status,version,last_verified_version FROM "+table+" WHERE id=$1 FOR UPDATE", id).Scan(&currentEnabled, &status, &version, &verifiedVersion); queryErr != nil {
			return nil, notFound(queryErr)
		}
		if version != expectedVersion {
			return nil, ErrVersionConflict
		}
		if enabled && (status != string(ValidationVerified) || verifiedVersion == nil || *verifiedVersion != version) {
			return nil, ErrVerification
		}
		if enabled && resource == ValidationResourceProvider {
			var defaultModel string
			if err := tx.QueryRow(ctx, "SELECT default_model FROM llm_provider_configurations WHERE id=$1", id).Scan(&defaultModel); err != nil {
				return nil, err
			}
			available, err := s.providerModelAvailableTx(ctx, tx, id, defaultModel)
			if err != nil {
				return nil, err
			}
			if !available {
				return nil, ErrVerification
			}
		}
		if enabled && resource == ValidationResourceWorkflow {
			var connectionID uuid.UUID
			var strategy string
			var providerID *uuid.UUID
			var model *string
			if err := tx.QueryRow(ctx, "SELECT connection_id,llm_strategy,llm_provider_id,llm_model FROM workflow_configurations WHERE id=$1", id).Scan(&connectionID, &strategy, &providerID, &model); err != nil {
				return nil, err
			}
			var connectionStatus string
			var connectionEnabled bool
			var connectionVersion int
			var connectionVerified *int
			if err := tx.QueryRow(ctx, "SELECT integration_status,enabled,version,last_verified_version FROM workflow_connections WHERE id=$1 FOR SHARE", connectionID).Scan(&connectionStatus, &connectionEnabled, &connectionVersion, &connectionVerified); err != nil {
				return nil, notFound(err)
			}
			if connectionStatus != "verified" || !connectionEnabled || connectionVerified == nil || *connectionVerified != connectionVersion {
				return nil, ErrVerification
			}
			if strategy == "acf_managed" {
				if providerID == nil || model == nil {
					return nil, ErrVerification
				}
				var providerStatus string
				var providerEnabled bool
				var providerVersion int
				var providerVerified *int
				if err := tx.QueryRow(ctx, "SELECT integration_status,enabled,version,last_verified_version FROM llm_provider_configurations WHERE id=$1 FOR SHARE", *providerID).Scan(&providerStatus, &providerEnabled, &providerVersion, &providerVerified); err != nil {
					return nil, notFound(err)
				}
				available, err := s.providerModelAvailableTx(ctx, tx, *providerID, *model)
				if err != nil {
					return nil, err
				}
				if providerStatus != "verified" || !providerEnabled || providerVerified == nil || *providerVerified != providerVersion || !available {
					return nil, ErrVerification
				}
			} else if strategy != "n8n_managed" && strategy != "none" {
				return nil, ErrVerification
			}
		}
		if currentEnabled != enabled {
			tag, updateErr := tx.Exec(ctx, "UPDATE "+table+" SET enabled=$3,last_verified_version=CASE WHEN integration_status='verified' THEN version+1 ELSE last_verified_version END,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$2", id, expectedVersion, enabled)
			if updateErr != nil {
				return nil, updateErr
			}
			if tag.RowsAffected() != 1 {
				return nil, ErrVersionConflict
			}
			version++
			if status == string(ValidationVerified) {
				verifiedVersion = &version
			}
			if auditErr := s.audit(ctx, tx, action, subject, id, safeAudit(action, version, map[string]any{"validationStatus": status})); auditErr != nil {
				return nil, auditErr
			}
		}
		executable, reasons := EvaluateEligibility(EligibilityFact{Kind: string(resource), Status: ValidationStatus(status), Enabled: enabled, Version: version, VerifiedVersion: verifiedVersion, ModelAvailable: true, StrategyComplete: true, ReferenceExists: true, ReferenceActive: true, StageMatches: true, InputCompatible: true, OutputCompatible: true})
		return json.Marshal(EnableCommandResult{ID: id, Enabled: enabled, ValidationStatus: ValidationStatus(status), Version: version, Executable: executable, IneligibilityReasons: reasons})
	})
	if err != nil {
		return EnableCommandResult{}, err
	}
	var result EnableCommandResult
	if json.Unmarshal(body, &result) != nil {
		return EnableCommandResult{}, ErrIdempotency
	}
	return result, nil
}

// RunValidationCommand serializes an operation-scoped idempotency key without
// holding a database transaction during the external validator. Completion,
// audit and the replay record are committed atomically.
func (s *Service) RunValidationCommand(ctx context.Context, resource ValidationResource, id uuid.UUID, expectedVersion int, key string, validate func(context.Context) ValidationOutcome) (ValidationCommandResult, error) {
	table, subject, metadataErr := validationResourceMetadata(resource)
	key = strings.TrimSpace(key)
	if metadataErr != nil || id == uuid.Nil || expectedVersion < 1 || key == "" || len(key) > 128 || validate == nil {
		return ValidationCommandResult{}, ErrValidation
	}
	scope := validationCommandPrefix(resource) + ":verify:" + id.String()
	request, _ := json.Marshal(struct {
		ExpectedVersion int `json:"expectedVersion"`
	}{expectedVersion})
	digest := sha256.Sum256(request)
	requestHash := hex.EncodeToString(digest[:])
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return ValidationCommandResult{}, err
	}
	defer conn.Release()
	if _, err = conn.Exec(ctx, "SELECT pg_advisory_lock(hashtextextended($1,0))", scope+":"+key); err != nil {
		return ValidationCommandResult{}, err
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(hashtextextended($1,0))", scope+":"+key)

	var storedHash string
	var storedBody []byte
	err = conn.QueryRow(ctx, "SELECT request_hash,response_body FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2", scope, key).Scan(&storedHash, &storedBody)
	if err == nil {
		if storedHash != requestHash {
			return ValidationCommandResult{}, ErrIdempotency
		}
		var replay ValidationCommandResult
		if json.Unmarshal(storedBody, &replay) != nil {
			return ValidationCommandResult{}, ErrIdempotency
		}
		if replay.ValidationStatus == ValidationFailed {
			return replay, ErrVerification
		}
		return replay, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ValidationCommandResult{}, err
	}

	startedAt := time.Now().UTC()
	tag, err := conn.Exec(ctx, fmt.Sprintf("UPDATE %s SET integration_status='verifying',validation_details='{}'::jsonb,last_error_code=NULL,last_error_message=NULL,updated_at=$3 WHERE id=$1 AND version=$2 AND integration_status IN ('unverified','failed','stale','verified')", table), id, expectedVersion, startedAt)
	if err != nil {
		return ValidationCommandResult{}, err
	}
	if tag.RowsAffected() != 1 {
		return ValidationCommandResult{}, s.validationConflict(ctx, table, id, expectedVersion)
	}
	outcome := validate(ctx)
	status := ValidationFailed
	if outcome.Success {
		status = ValidationVerified
	}
	code, message := strings.TrimSpace(outcome.Code), strings.TrimSpace(outcome.Message)
	if outcome.Success {
		code, message = "", ""
	} else if code == "" || message == "" {
		code, message = "upstream_unavailable", "The integration could not be verified."
	}
	details := sanitizeJSON(outcome.Details)
	checkedAt := time.Now().UTC()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return ValidationCommandResult{}, err
	}
	defer tx.Rollback(ctx)
	tag, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET integration_status=$3::text,last_verified_version=CASE WHEN $3::text='verified' THEN version ELSE last_verified_version END,validation_details=$4,last_verified_at=$7,last_error_code=NULLIF($5,''),last_error_message=NULLIF($6,''),updated_at=$7 WHERE id=$1 AND version=$2 AND integration_status='verifying'`, table), id, expectedVersion, string(status), details, code, message, checkedAt)
	if err != nil {
		return ValidationCommandResult{}, err
	}
	if tag.RowsAffected() != 1 {
		return ValidationCommandResult{}, s.validationConflictTx(ctx, tx, table, id, expectedVersion)
	}
	var enabled bool
	var verifiedVersion *int
	if err = tx.QueryRow(ctx, "SELECT enabled,last_verified_version FROM "+table+" WHERE id=$1", id).Scan(&enabled, &verifiedVersion); err != nil {
		return ValidationCommandResult{}, err
	}
	executable, reasons := EvaluateEligibility(EligibilityFact{Kind: string(resource), Status: status, Enabled: enabled, Version: expectedVersion, VerifiedVersion: verifiedVersion, ModelAvailable: true, StrategyComplete: true, ReferenceExists: true, ReferenceActive: true, StageMatches: true, InputCompatible: true, OutputCompatible: true})
	result := ValidationCommandResult{ID: id, ValidationStatus: status, VerifiedVersion: verifiedVersion, CheckedAt: checkedAt, Executable: executable, IneligibilityReasons: reasons}
	if !outcome.Success {
		result.SafeError = &ValidationSafeError{Code: code, Message: message, Retryable: code == "upstream_timeout" || code == "upstream_unavailable"}
	}
	if err = s.audit(ctx, tx, "verify", subject, id, safeAudit("verify", expectedVersion, map[string]any{"validationStatus": status, "errorCode": code})); err != nil {
		return ValidationCommandResult{}, err
	}
	body, _ := json.Marshal(result)
	responseStatus := 200
	if !outcome.Success {
		responseStatus = 422
	}
	if _, err = idempotency.NewPostgresRepositoryTx(tx).Create(ctx, idempotency.Record{ID: uuid.New(), Scope: scope, Key: key, RequestHash: requestHash, ResponseStatus: responseStatus, ResponseBody: body}); err != nil {
		return ValidationCommandResult{}, ErrIdempotency
	}
	if err = tx.Commit(ctx); err != nil {
		return ValidationCommandResult{}, err
	}
	if !outcome.Success {
		return result, ErrVerification
	}
	return result, nil
}

func (s *Service) BeginValidation(ctx context.Context, resource ValidationResource, id uuid.UUID, expectedVersion int) (ValidationAttempt, error) {
	table, _, err := validationResourceMetadata(resource)
	if err != nil || id == uuid.Nil || expectedVersion < 1 {
		return ValidationAttempt{}, ErrValidation
	}
	startedAt := time.Now().UTC()
	tag, err := s.pool.Exec(ctx, fmt.Sprintf("UPDATE %s SET integration_status='verifying',validation_details='{}'::jsonb,last_error_code=NULL,last_error_message=NULL,updated_at=$3 WHERE id=$1 AND version=$2 AND integration_status IN ('unverified','failed','stale','verified')", table), id, expectedVersion, startedAt)
	if err != nil {
		return ValidationAttempt{}, err
	}
	if tag.RowsAffected() != 1 {
		return ValidationAttempt{}, s.validationConflict(ctx, table, id, expectedVersion)
	}
	return ValidationAttempt{Resource: resource, ID: id, ExpectedVersion: expectedVersion, StartedAt: startedAt}, nil
}

func (s *Service) CompleteValidation(ctx context.Context, attempt ValidationAttempt, outcome ValidationOutcome) error {
	table, subject, err := validationResourceMetadata(attempt.Resource)
	if err != nil || attempt.ID == uuid.Nil || attempt.ExpectedVersion < 1 {
		return ErrValidation
	}
	status := ValidationFailed
	if outcome.Success {
		status = ValidationVerified
	}
	details := sanitizeJSON(outcome.Details)
	if len(details) == 0 {
		details = json.RawMessage(`{}`)
	}
	code, message := strings.TrimSpace(outcome.Code), strings.TrimSpace(outcome.Message)
	if outcome.Success {
		code, message = "", ""
	} else if code == "" || message == "" {
		return ErrValidation
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET integration_status=$3::text,last_verified_version=CASE WHEN $3::text='verified' THEN version ELSE last_verified_version END,validation_details=$4,last_verified_at=NOW(),last_error_code=NULLIF($5,''),last_error_message=NULLIF($6,''),updated_at=NOW() WHERE id=$1 AND version=$2 AND integration_status='verifying'`, table), attempt.ID, attempt.ExpectedVersion, string(status), details, code, message)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return s.validationConflictTx(ctx, tx, table, attempt.ID, attempt.ExpectedVersion)
	}
	if err = s.audit(ctx, tx, "verify", subject, attempt.ID, safeAudit("verify", attempt.ExpectedVersion, map[string]any{"validationStatus": status, "errorCode": code})); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) validationConflict(ctx context.Context, table string, id uuid.UUID, expected int) error {
	var version int
	var status string
	err := s.pool.QueryRow(ctx, "SELECT version,integration_status FROM "+table+" WHERE id=$1", id).Scan(&version, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if version != expected || status == "verifying" || status == "verified" || status == "failed" {
		return ErrVersionConflict
	}
	return ErrInvalidValidationTransition
}

func (s *Service) validationConflictTx(ctx context.Context, tx pgx.Tx, table string, id uuid.UUID, expected int) error {
	var version int
	var status string
	err := tx.QueryRow(ctx, "SELECT version,integration_status FROM "+table+" WHERE id=$1", id).Scan(&version, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if version != expected || status != "verifying" {
		return ErrVersionConflict
	}
	return ErrInvalidValidationTransition
}

func validationResourceMetadata(resource ValidationResource) (table, subject string, err error) {
	switch resource {
	case ValidationResourceProvider:
		return "llm_provider_configurations", "llm_provider", nil
	case ValidationResourceConnection:
		return "workflow_connections", "workflow_connection", nil
	case ValidationResourceWorkflow:
		return "workflow_configurations", "workflow_configuration", nil
	default:
		return "", "", ErrValidation
	}
}

func validationCommandPrefix(resource ValidationResource) string {
	switch resource {
	case ValidationResourceProvider:
		return "llm-provider"
	case ValidationResourceConnection:
		return "workflow-connection"
	case ValidationResourceWorkflow:
		return "workflow-configuration"
	default:
		return "invalid"
	}
}

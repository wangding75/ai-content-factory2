package workflowbinding

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrNotFound              = errors.New("project workflow binding not found")
	ErrBindingAlreadyExists  = errors.New("project workflow binding already exists")
	ErrVersionConflict       = errors.New("project workflow binding version conflict")
	ErrProjectNotFound       = errors.New("project not found")
	ErrConfigurationNotFound = errors.New("workflow configuration not found")
	ErrDisabledWorkflow      = errors.New("workflow configuration is not enabled")
	ErrNotApplicable         = errors.New("workflow configuration is not applicable to stage")
	ErrIdempotencyReused     = errors.New("idempotency key reused with different payload")
)

// VersionConflictError carries the context required by the frozen 409
// version_conflict response details.  It wraps the ErrVersionConflict sentinel
// so errors.Is(err, ErrVersionConflict) keeps working while errors.As lets the
// HTTP layer surface expectedVersion / currentVersion / projectId / stage.
type VersionConflictError struct {
	ProjectID      uuid.UUID
	Stage          WorkflowBindingStage
	ExpectedVersion int
	CurrentVersion  int
	Missing         bool
}

func (e *VersionConflictError) Error() string { return ErrVersionConflict.Error() }
func (e *VersionConflictError) Unwrap() error { return ErrVersionConflict }

// isNotFound reports whether the error denotes a missing binding.
func isNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}

func errorsIsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// safeHashJSON marshals the value and returns its SHA-256 hex digest.  It is
// used to fingerprint idempotent requests so that a reused key with a different
// payload can be detected.
func safeHashJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

var _ = uuid.New

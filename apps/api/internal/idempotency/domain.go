package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound = errors.New("idempotency record not found")
	ErrConflict = errors.New("idempotency key conflict")
)

type Record struct {
	ID             uuid.UUID
	Scope          string
	Key            string
	RequestHash    string
	ResponseStatus int
	ResponseBody   json.RawMessage
	CreatedAt      time.Time
	ExpiresAt      *time.Time
}

type Repository interface {
	Get(context.Context, string, string) (Record, error)
	Create(context.Context, Record) (Record, error)
}

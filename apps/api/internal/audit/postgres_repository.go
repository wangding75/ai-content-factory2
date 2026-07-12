package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Entry struct {
	ID          uuid.UUID
	ActorID     string
	Action      string
	SubjectType string
	SubjectID   string
	Payload     json.RawMessage
	CreatedAt   time.Time
}

type Repository struct{ tx pgx.Tx }

func NewRepository(tx pgx.Tx) *Repository { return &Repository{tx: tx} }

func (r *Repository) Insert(ctx context.Context, entry Entry) error {
	if !json.Valid(entry.Payload) {
		return fmt.Errorf("invalid audit payload")
	}
	_, err := r.tx.Exec(ctx, "INSERT INTO audit_logs (id, actor_id, action, subject_type, subject_id, payload) VALUES ($1,$2,$3,$4,$5,$6)", entry.ID, entry.ActorID, entry.Action, entry.SubjectType, entry.SubjectID, entry.Payload)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

package chapterplan

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConsumptionStatus string

const (
	ConsumptionPending                 ConsumptionStatus = "pending"
	ConsumptionConsuming               ConsumptionStatus = "consuming"
	ConsumptionConsumed                ConsumptionStatus = "consumed"
	ConsumptionOutputValidationFailed  ConsumptionStatus = "output_validation_failed"
	ConsumptionResultConsumptionFailed ConsumptionStatus = "result_consumption_failed"
)

// ConsumptionRepository records the independent post-runtime outcome. It
// deliberately never changes workflow_run_records.status: a successful runtime
// remains successful even when normalized-output consumption fails.
type ConsumptionRepository struct{ pool *pgxpool.Pool }

func NewConsumptionRepository(pool *pgxpool.Pool) *ConsumptionRepository {
	return &ConsumptionRepository{pool: pool}
}

func (r *ConsumptionRepository) Set(ctx context.Context, runID, projectID uuid.UUID, status ConsumptionStatus, batchID *uuid.UUID, failureCode, safeReason, retryAction *string) error {
	var consumedAt *time.Time
	if status == ConsumptionConsumed {
		now := time.Now().UTC()
		consumedAt = &now
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO chapter_plan_result_consumptions (
			workflow_run_id, project_id, status, failure_code, safe_reason, retry_action, candidate_batch_id, consumed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (workflow_run_id) DO UPDATE SET
			status=EXCLUDED.status, failure_code=EXCLUDED.failure_code, safe_reason=EXCLUDED.safe_reason,
			retry_action=EXCLUDED.retry_action, candidate_batch_id=COALESCE(EXCLUDED.candidate_batch_id, chapter_plan_result_consumptions.candidate_batch_id),
			consumed_at=COALESCE(EXCLUDED.consumed_at, chapter_plan_result_consumptions.consumed_at),
			version=chapter_plan_result_consumptions.version+1, updated_at=NOW()
	`, runID, projectID, status, failureCode, safeReason, retryAction, batchID, consumedAt)
	return err
}

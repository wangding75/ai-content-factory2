package chapterplan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

// RuntimeConsumer is injected into Iteration 14 rather than duplicating its
// execution lifecycle. It consumes only the immutable context persisted in the
// successful run's input payload.
type RuntimeConsumer struct {
	ingestor     Ingestor
	consumptions *ConsumptionRepository
}

func decodeChapterPlanningResult(run workflowrun.WorkflowRun) (IngestInput, error) {
	var input struct {
		GenerationContext GenerationContextSnapshot `json:"generationContext"`
	}
	if json.Unmarshal(run.InputPayload, &input) != nil || input.GenerationContext.InputDigest == "" {
		return IngestInput{}, ErrOutputValidationFailed
	}
	var output NormalizedChapterPlanOutput
	decoder := json.NewDecoder(bytes.NewReader(run.OutputPayload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&output) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return IngestInput{}, ErrOutputValidationFailed
	}
	value := IngestInput{Run: RunReference{RunID: run.ID, ProjectID: run.ProjectID}, Context: input.GenerationContext, NormalizedOutput: output}
	if err := ValidateNormalizedOutput(value); err != nil {
		return IngestInput{}, err
	}
	return value, nil
}

func (c *RuntimeConsumer) ValidateResult(run workflowrun.WorkflowRun) error {
	_, err := decodeChapterPlanningResult(run)
	return err
}

func (c *RuntimeConsumer) ConsumeResultTx(ctx context.Context, tx pgx.Tx, run workflowrun.WorkflowRun) error {
	input, err := decodeChapterPlanningResult(run)
	if err != nil {
		return err
	}
	ingestor, ok := c.ingestor.(interface {
		IngestTx(context.Context, pgx.Tx, IngestInput) (CandidateBatch, error)
	})
	if !ok {
		return ErrIngestionTransaction
	}
	batch, err := ingestor.IngestTx(ctx, tx, input)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chapter_plan_result_consumptions(workflow_run_id,project_id,status,candidate_batch_id,consumed_at)
		VALUES($1,$2,'consumed',$3,NOW()) ON CONFLICT(workflow_run_id) DO UPDATE SET status='consumed',candidate_batch_id=EXCLUDED.candidate_batch_id,consumed_at=COALESCE(chapter_plan_result_consumptions.consumed_at,NOW()),failure_code=NULL,safe_reason=NULL,retry_action=NULL,version=chapter_plan_result_consumptions.version+1,updated_at=NOW()`, run.ID, run.ProjectID, batch.ID)
	if err != nil {
		return err
	}
	// Use the application clock clamped to run.created_at. PostgreSQL NOW() is
	// forbidden here: Docker clock drift vs the app clock produced DC-TIME-007
	// reverse-order events (result_consumed before run.created_at).
	eventAt := workflowrun.EventCreatedAt(time.Now().UTC(), run.CreatedAt)
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed')", run.ID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = workflowrun.AddEventTx(ctx, tx, workflowrun.Event{
		ID: uuid.New(), RunID: run.ID, EventType: "result_consumed", Status: run.Status,
		Payload: json.RawMessage(`{}`), CreatedAt: eventAt,
	})
	return err
}

func NewRuntimeConsumer(ingestor Ingestor, consumptions ...*ConsumptionRepository) *RuntimeConsumer {
	consumer := &RuntimeConsumer{ingestor: ingestor}
	if len(consumptions) > 0 {
		consumer.consumptions = consumptions[0]
	}
	return consumer
}
func (c *RuntimeConsumer) ConsumeSucceededRun(ctx context.Context, run workflowrun.WorkflowRun) error {
	if run.Stage != "chapter_planning" || run.Status != workflowrun.StatusSucceeded {
		return nil
	}
	var input struct {
		GenerationContext GenerationContextSnapshot `json:"generationContext"`
	}
	if c.consumptions != nil {
		if err := c.consumptions.Set(ctx, run.ID, run.ProjectID, ConsumptionConsuming, nil, nil, nil, nil); err != nil {
			return ErrIngestionTransaction
		}
	}
	if json.Unmarshal(run.InputPayload, &input) != nil || input.GenerationContext.InputDigest == "" {
		c.recordFailure(ctx, run, ConsumptionOutputValidationFailed, "output_validation_failed", "The runtime output does not match the frozen generation context.", "retry_run")
		return ErrOutputValidationFailed
	}
	var output NormalizedChapterPlanOutput
	decoder := json.NewDecoder(bytes.NewReader(run.OutputPayload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&output) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		c.recordFailure(ctx, run, ConsumptionOutputValidationFailed, "output_validation_failed", "The runtime output is invalid.", "retry_run")
		return ErrOutputValidationFailed
	}
	batch, err := c.ingestor.Ingest(ctx, IngestInput{Run: RunReference{RunID: run.ID, ProjectID: run.ProjectID}, Context: input.GenerationContext, NormalizedOutput: output})
	if errors.Is(err, ErrOutputValidationFailed) {
		c.recordFailure(ctx, run, ConsumptionOutputValidationFailed, "output_validation_failed", "The runtime output does not match the frozen generation context.", "retry_run")
		return err
	}
	if err != nil {
		c.recordFailure(ctx, run, ConsumptionResultConsumptionFailed, "result_consumption_failed", "The generated result could not be stored safely.", "retry_run")
		return ErrIngestionTransaction
	}
	if c.consumptions != nil {
		if err := c.consumptions.Set(ctx, run.ID, run.ProjectID, ConsumptionConsumed, &batch.ID, nil, nil, nil); err != nil {
			c.recordFailure(ctx, run, ConsumptionResultConsumptionFailed, "result_consumption_failed", "The generated result could not be stored safely.", "retry_run")
			return ErrIngestionTransaction
		}
	}
	return nil
}

func (c *RuntimeConsumer) recordFailure(ctx context.Context, run workflowrun.WorkflowRun, status ConsumptionStatus, code, safeReason, retryAction string) {
	if c.consumptions != nil {
		_ = c.consumptions.Set(ctx, run.ID, run.ProjectID, status, nil, &code, &safeReason, &retryAction)
	}
}

package chapterplan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

// RuntimeConsumer is injected into Iteration 14 rather than duplicating its
// execution lifecycle. It consumes only the immutable context persisted in the
// successful run's input payload.
type RuntimeConsumer struct {
	ingestor     Ingestor
	consumptions *ConsumptionRepository
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
		_ = c.consumptions.Set(ctx, run.ID, run.ProjectID, ConsumptionConsuming, nil, nil, nil, nil)
	}
	if json.Unmarshal(run.InputPayload, &input) != nil || input.GenerationContext.InputDigest == "" {
		c.recordFailure(ctx, run, ConsumptionOutputValidationFailed, "output_validation_failed", "The runtime output does not match the frozen generation context.", "retry_run")
		return ErrOutputValidationFailed
	}
	var output NormalizedChapterPlanOutput
	decoder := json.NewDecoder(bytes.NewReader(run.OutputPayload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&output) != nil || decoder.More() {
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
		_ = c.consumptions.Set(ctx, run.ID, run.ProjectID, ConsumptionConsumed, &batch.ID, nil, nil, nil)
	}
	return nil
}

func (c *RuntimeConsumer) recordFailure(ctx context.Context, run workflowrun.WorkflowRun, status ConsumptionStatus, code, safeReason, retryAction string) {
	if c.consumptions != nil {
		_ = c.consumptions.Set(ctx, run.ID, run.ProjectID, status, nil, &code, &safeReason, &retryAction)
	}
}

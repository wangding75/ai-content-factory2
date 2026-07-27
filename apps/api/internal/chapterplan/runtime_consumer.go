package chapterplan

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

// RuntimeConsumer is injected into Iteration 14 rather than duplicating its
// execution lifecycle. It consumes only the immutable context persisted in the
// successful run's input payload.
type RuntimeConsumer struct{ ingestor Ingestor }

func NewRuntimeConsumer(ingestor Ingestor) *RuntimeConsumer {
	return &RuntimeConsumer{ingestor: ingestor}
}
func (c *RuntimeConsumer) ConsumeSucceededRun(ctx context.Context, run workflowrun.WorkflowRun) error {
	if run.Stage != "chapter_planning" || run.Status != workflowrun.StatusSucceeded {
		return nil
	}
	var input struct {
		GenerationContext GenerationContextSnapshot `json:"generationContext"`
	}
	if json.Unmarshal(run.InputPayload, &input) != nil || input.GenerationContext.InputDigest == "" {
		return ErrOutputValidationFailed
	}
	var output NormalizedChapterPlanOutput
	if json.Unmarshal(run.OutputPayload, &output) != nil {
		return ErrOutputValidationFailed
	}
	_, err := c.ingestor.Ingest(ctx, IngestInput{Run: RunReference{RunID: run.ID, ProjectID: run.ProjectID}, Context: input.GenerationContext, NormalizedOutput: output})
	if errors.Is(err, ErrOutputValidationFailed) {
		return err
	}
	if err != nil {
		return ErrIngestionTransaction
	}
	return nil
}

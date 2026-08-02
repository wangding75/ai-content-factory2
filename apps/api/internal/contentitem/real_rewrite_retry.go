package contentitem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type RetryRewriteConsumptionRequest struct {
	ExpectedRunVersion int
	IdempotencyKey     string
}

func rewriteCommandHash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *RealRewriteService) RetryResultConsumption(ctx context.Context, runID uuid.UUID, request RetryRewriteConsumptionRequest) (RewriteResult, error) {
	if runID == uuid.Nil || request.ExpectedRunVersion < 1 ||
		strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 128 {
		return RewriteResult{}, ErrValidation
	}
	scope := "retryContentRewriteResultConsumption:" + runID.String()
	requestHash := rewriteCommandHash(struct {
		RunID   uuid.UUID
		Version int
	}{runID, request.ExpectedRunVersion})
	tx, err := s.repo.db.Begin(ctx)
	if err != nil {
		return RewriteResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "idempotency:"+scope+":"+request.IdempotencyKey); err != nil {
		return RewriteResult{}, err
	}
	idem := idempotency.NewPostgresRepositoryTx(tx)
	if record, getErr := idem.GetForUpdate(ctx, scope, request.IdempotencyKey); getErr == nil {
		if record.RequestHash != requestHash {
			return RewriteResult{}, workflowrun.ErrIdempotencyConflict
		}
		var replay struct {
			CandidateID uuid.UUID `json:"candidateId"`
		}
		if json.Unmarshal(record.ResponseBody, &replay) != nil || replay.CandidateID == uuid.Nil {
			return RewriteResult{}, ErrRewriteCandidateNotReady
		}
		if err = tx.Commit(ctx); err != nil {
			return RewriteResult{}, err
		}
		return s.Result(ctx, runID)
	} else if !errors.Is(getErr, idempotency.ErrNotFound) {
		return RewriteResult{}, getErr
	}
	discovered, err := workflowrun.NewPostgresRepositoryTx(tx).GetByID(ctx, runID)
	if err != nil {
		return RewriteResult{}, err
	}
	input, err := rewriteInput(discovered)
	if err != nil {
		return RewriteResult{}, ErrRewriteCandidateNotReady
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "content-item:"+input.ContentItemID.String()); err != nil {
		return RewriteResult{}, err
	}
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "rewrite-run:"+runID.String()); err != nil {
		return RewriteResult{}, err
	}
	run, err := workflowrun.NewPostgresRepositoryTx(tx).GetByIDForUpdate(ctx, runID)
	if err != nil {
		return RewriteResult{}, err
	}
	if run.Version != request.ExpectedRunVersion {
		return RewriteResult{}, workflowrun.ErrVersionConflict
	}
	if run.Stage != "rewrite" || (run.Status != workflowrun.StatusSucceeded && run.Status != workflowrun.StatusFailed) ||
		run.SubjectType == nil || *run.SubjectType != "review_report" || run.SubjectID == nil ||
		*run.SubjectID != input.ReviewReportID || len(run.OutputPayload) == 0 {
		return RewriteResult{}, ErrRewriteCandidateNotReady
	}
	var candidate ContentVersion
	candidate, err = scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE source_workflow_run_id=$1 AND source='workflow_rewrite' FOR UPDATE", run.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		var consumptionFailed, validationFailed, consumed bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumption_failed'),EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='output_validation_failed'),EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed')", run.ID).Scan(&consumptionFailed, &validationFailed, &consumed); err != nil {
			return RewriteResult{}, err
		}
		if !consumptionFailed || validationFailed || consumed {
			return RewriteResult{}, ErrRewriteCandidateNotReady
		}
		selectedIDs := make([]uuid.UUID, len(input.SelectedIssues))
		for i := range input.SelectedIssues {
			selectedIDs[i] = input.SelectedIssues[i].ReviewIssueID
		}
		output, outputErr := DecodeRewriteRuntimeOutput(run.OutputPayload, selectedIDs)
		if outputErr != nil {
			return RewriteResult{}, ErrRewriteOutputInvalid
		}
		candidate, err = s.consumeRewriteLocked(ctx, tx, run, input, output, true)
		if err != nil {
			return RewriteResult{}, ErrRewriteResultConsumption
		}
	} else if err != nil {
		return RewriteResult{}, err
	} else {
		var consumed bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed')", run.ID).Scan(&consumed); err != nil {
			return RewriteResult{}, err
		}
		if !consumed {
			return RewriteResult{}, ErrRewriteCandidateNotReady
		}
	}
	body, err := json.Marshal(struct {
		CandidateID uuid.UUID `json:"candidateId"`
	}{candidate.ID})
	if err != nil {
		return RewriteResult{}, err
	}
	if _, err = idem.Create(ctx, idempotency.Record{
		ID: uuid.New(), Scope: scope, Key: request.IdempotencyKey, RequestHash: requestHash,
		ResponseStatus: 200, ResponseBody: body,
	}); err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			return RewriteResult{}, workflowrun.ErrIdempotencyConflict
		}
		return RewriteResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return RewriteResult{}, ErrRewriteResultConsumption
	}
	if run.Status == workflowrun.StatusFailed && run.FailurePhase != nil && *run.FailurePhase == "result_consumption" {
		runtime, ok := s.runs.(interface {
			RetryResultConsumption(context.Context, uuid.UUID, int) (workflowrun.WorkflowRun, error)
		})
		if !ok {
			return RewriteResult{}, workflowrun.ErrNotRetryable
		}
		if _, err = runtime.RetryResultConsumption(ctx, run.ID, request.ExpectedRunVersion); err != nil {
			return RewriteResult{}, err
		}
	}
	return s.Result(ctx, run.ID)
}

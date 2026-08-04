package workflowrun

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
)

// Worker defaults keep each poll bounded and fair: oldest processable Runs first,
// never a fixed newest-100 UI list that can starve older work.
const (
	workerCandidateBatchLimit   = 50
	missingExternalIDGracePeriod = 2 * time.Minute
)

// RunWorker uses versioned transitions as its claim: every worker may observe a
// candidate, but only one can advance its version and write the terminal event.
// When ctx is cancelled the worker stops claiming new candidates and waits for
// in-flight candidate handling to finish before returning.
func (s *Service) RunWorker(ctx context.Context, interval time.Duration, onError func(error)) {
	if interval <= 0 {
		interval = time.Second
	}
	report := func(err error) {
		if err != nil && onError != nil && ctx.Err() == nil {
			onError(err)
		}
	}
	var inFlight sync.WaitGroup
	processOne := func(run WorkflowRun) {
		inFlight.Add(1)
		defer inFlight.Done()
		report(s.processWorkerCandidate(ctx, run))
	}
	process := func() {
		if ctx.Err() != nil {
			return
		}
		now := s.now()
		consumptions, err := s.store.ListRecoverableResultConsumptions(ctx, workerCandidateBatchLimit, now)
		if err != nil {
			report(err)
		} else {
			for _, run := range consumptions {
				if ctx.Err() != nil {
					return
				}
				processOne(run)
			}
		}
		if ctx.Err() != nil {
			return
		}
		candidates, err := s.store.ListWorkerCandidates(ctx, workerCandidateBatchLimit, now)
		if err != nil {
			report(err)
			return
		}
		for _, run := range candidates {
			if ctx.Err() != nil {
				return
			}
			processOne(run)
		}
	}
	process()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			inFlight.Wait()
			return
		case <-ticker.C:
			process()
		}
	}
}

func (s *Service) processWorkerCandidate(ctx context.Context, run WorkflowRun) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Pending consumption recovery is handled before candidates; if output is
	// already durable, never re-execute or re-query the external system.
	if validJSONObject(run.OutputPayload) && (run.Status == StatusRunning || run.Status == StatusCancelling ||
		(run.Status == StatusFailed && run.FailurePhase != nil && *run.FailurePhase == "result_consumption")) {
		_, err := s.consumeStoredResult(ctx, run)
		return err
	}
	if s.deadlineExpired(run) {
		_, err := s.expireRun(ctx, run)
		return err
	}
	switch run.Status {
	case StatusQueued:
		_, err := s.ExecuteRun(ctx, run.ID)
		return err
	case StatusCancelling:
		return s.completeCancellation(ctx, run)
	case StatusRunning:
		if run.ExternalExecutionID == nil || strings.TrimSpace(*run.ExternalExecutionID) == "" {
			return s.handleMissingExternalExecutionID(ctx, run)
		}
		request, err := executionRequest(run)
		if err != nil {
			return err
		}
		result, err := s.executor.Query(ctx, request)
		if err != nil {
			return err
		}
		_, err = s.applyExecutionResult(ctx, run, result)
		return err
	default:
		return nil
	}
}

// handleMissingExternalExecutionID waits inside the normal callback window and
// never re-submits Execute. Past the grace period it emits a diagnostic log so
// the run does not stay silently stuck while still not monopolizing the worker window
// (ListWorkerCandidates only returns these after the grace threshold).
func (s *Service) handleMissingExternalExecutionID(ctx context.Context, run WorkflowRun) error {
	reference := run.UpdatedAt
	if run.StartedAt != nil {
		reference = *run.StartedAt
	}
	if s.now().Sub(reference) < missingExternalIDGracePeriod {
		return nil
	}
	log.Printf("workflow run missing external execution id after grace run_id=%s status=%s started_at=%s updated_at=%s",
		run.ID, run.Status, reference.UTC().Format(time.RFC3339Nano), run.UpdatedAt.UTC().Format(time.RFC3339Nano))
	return nil
}

func (s *Service) completeCancellation(ctx context.Context, run WorkflowRun) error {
	if run.ExternalExecutionID == nil || strings.TrimSpace(*run.ExternalExecutionID) == "" {
		if run.CancellationReason != nil && *run.CancellationReason == "timeout" {
			_, err := s.timeoutExecution(ctx, run)
			return err
		}
		return ErrExecutorUnavailable
	}
	request, err := executionRequest(run)
	if err != nil {
		return err
	}
	result, err := s.executor.Cancel(ctx, request)
	if err != nil {
		return err
	}
	if result.Status == ExecutionSucceeded || result.Status == ExecutionFailed || result.Status == ExecutionCancelled {
		_, err = s.applyExecutionResult(ctx, run, result)
		return err
	}
	result, err = s.executor.Query(ctx, request)
	if err != nil {
		return err
	}
	if result.Status == ExecutionAccepted || result.Status == ExecutionRunning {
		return nil
	}
	_, err = s.applyExecutionResult(ctx, run, result)
	return err
}

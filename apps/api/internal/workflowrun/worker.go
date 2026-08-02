package workflowrun

import (
	"context"
	"errors"
	"strings"
	"time"
)

// RunWorker uses versioned transitions as its claim: every worker may observe a
// candidate, but only one can advance its version and write the terminal event.
func (s *Service) RunWorker(ctx context.Context, interval time.Duration, onError func(error)) {
	if interval <= 0 {
		interval = time.Second
	}
	report := func(err error) {
		if err != nil && onError != nil && ctx.Err() == nil {
			onError(err)
		}
	}
	process := func() {
		consumptions, err := s.store.ListRecoverableResultConsumptions(ctx, 100, s.now())
		if err != nil {
			report(err)
		} else {
			for _, run := range consumptions {
				_, consumeErr := s.consumeStoredResult(ctx, run)
				report(consumeErr)
			}
		}
		for _, status := range []Status{StatusCancelling, StatusQueued, StatusRunning} {
			runs, err := s.store.List(ctx, ListFilter{Status: string(status), Limit: 100})
			if err != nil {
				report(err)
				continue
			}
			for _, run := range runs {
				if run.Status == StatusCancelling {
					report(s.completeCancellation(ctx, run))
					continue
				}
				if run.Status == StatusQueued {
					_, err = s.ExecuteRun(ctx, run.ID)
					report(err)
					continue
				}
				if run.Status != StatusRunning {
					continue
				}
				// A running execution is recovered only when it has a durable external id.
				if validJSONObject(run.OutputPayload) {
					continue
				}
				if run.ExternalExecutionID == nil || strings.TrimSpace(*run.ExternalExecutionID) == "" {
					continue
				}
				request, e := executionRequest(run)
				if e != nil {
					report(e)
					continue
				}
				result, e := s.executor.Query(ctx, request)
				if errors.Is(e, ErrExecutionTimeout) {
					_, e = s.timeoutExecution(ctx, run)
				} else if e == nil {
					_, e = s.applyExecutionResult(ctx, run, result)
				}
				report(e)
			}
		}
	}
	process()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			process()
		}
	}
}

func (s *Service) completeCancellation(ctx context.Context, run WorkflowRun) error {
	if run.ExternalExecutionID == nil || strings.TrimSpace(*run.ExternalExecutionID) == "" {
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

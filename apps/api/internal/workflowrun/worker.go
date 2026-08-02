package workflowrun

import (
	"context"
	"time"
)

func (s *Service) RunWorker(ctx context.Context, interval time.Duration, onError func(error)) {
	if interval <= 0 {
		interval = time.Second
	}
	process := func() {
		runs, err := s.store.List(ctx, ListFilter{Status: string(StatusQueued), Limit: 100})
		if err != nil {
			if onError != nil && ctx.Err() == nil {
				onError(err)
			}
			return
		}
		for _, run := range runs {
			if _, err = s.ExecuteRun(ctx, run.ID); err != nil && onError != nil && ctx.Err() == nil {
				onError(err)
			}
		}
		cancelling, err := s.store.List(ctx, ListFilter{Status: string(StatusCancelling), Limit: 100})
		if err != nil {
			if onError != nil && ctx.Err() == nil {
				onError(err)
			}
			return
		}
		for _, run := range cancelling {
			if run.Status != StatusCancelling {
				continue
			}
			request, requestErr := executionRequest(run)
			if requestErr != nil {
				if onError != nil {
					onError(requestErr)
				}
				continue
			}
			_, cancelErr := s.executor.Cancel(ctx, request)
			if cancelErr != nil {
				if onError != nil {
					onError(cancelErr)
				}
				continue
			}
			next, transitionErr := run.Cancel(s.now())
			if transitionErr != nil {
				if onError != nil {
					onError(transitionErr)
				}
				continue
			}
			_, _, updateErr := s.store.UpdateStatusWithEvent(ctx, run, next, Event{ID: s.newID(), RunID: run.ID, EventType: "cancelled", Status: StatusCancelled, Payload: []byte(`{}`), CreatedAt: next.UpdatedAt})
			if updateErr != nil && onError != nil {
				onError(updateErr)
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

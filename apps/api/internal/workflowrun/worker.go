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

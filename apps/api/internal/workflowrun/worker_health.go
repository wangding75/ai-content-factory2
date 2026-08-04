package workflowrun

import (
	"strings"
	"sync"
	"time"
)

// WorkerHealth is a concurrent-safe view of the worker loop used by /readyz.
// It intentionally excludes per-Run business failures.
type WorkerHealth struct {
	mu                  sync.RWMutex
	started             bool
	stopped             bool
	fatalError          string
	lastLoopStartedAt   time.Time
	lastLoopCompletedAt time.Time
	lastLoopError       string
}

// NewWorkerHealth constructs an idle health record.
func NewWorkerHealth() *WorkerHealth {
	return &WorkerHealth{}
}

// WorkerHealthSnapshot is a safe, non-sensitive copy of worker loop state.
type WorkerHealthSnapshot struct {
	Started             bool
	Stopped             bool
	FatalError          string
	LastLoopStartedAt   time.Time
	LastLoopCompletedAt time.Time
	LastLoopError       string
}

func (h *WorkerHealth) MarkStarted() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.started = true
	h.stopped = false
	h.fatalError = ""
}

func (h *WorkerHealth) MarkStopped() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopped = true
}

func (h *WorkerHealth) MarkFatal(message string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fatalError = sanitizeWorkerMessage(message)
	h.stopped = true
}

func (h *WorkerHealth) MarkLoopStart(at time.Time) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastLoopStartedAt = at.UTC()
	h.lastLoopError = ""
}

func (h *WorkerHealth) MarkLoopComplete(at time.Time, loopErr error) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastLoopCompletedAt = at.UTC()
	if loopErr != nil {
		h.lastLoopError = sanitizeWorkerMessage(loopErr.Error())
	} else {
		h.lastLoopError = ""
	}
}

func (h *WorkerHealth) Snapshot() WorkerHealthSnapshot {
	if h == nil {
		return WorkerHealthSnapshot{}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return WorkerHealthSnapshot{
		Started:             h.started,
		Stopped:             h.stopped,
		FatalError:          h.fatalError,
		LastLoopStartedAt:   h.lastLoopStartedAt,
		LastLoopCompletedAt: h.lastLoopCompletedAt,
		LastLoopError:       h.lastLoopError,
	}
}

// Healthy reports whether the worker loop itself is usable for readiness.
// maxIdle is the maximum age of the last completed loop (e.g. 2 minutes).
func (h *WorkerHealth) Healthy(now time.Time, maxIdle time.Duration) bool {
	snap := h.Snapshot()
	if !snap.Started || snap.Stopped || snap.FatalError != "" {
		return false
	}
	if maxIdle <= 0 {
		maxIdle = 2 * time.Minute
	}
	now = now.UTC()
	// Grace for the first loop: if started but not yet completed, allow maxIdle.
	if snap.LastLoopCompletedAt.IsZero() {
		if snap.LastLoopStartedAt.IsZero() {
			return true
		}
		return now.Sub(snap.LastLoopStartedAt) <= maxIdle
	}
	return now.Sub(snap.LastLoopCompletedAt) <= maxIdle
}

func sanitizeWorkerMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 200 {
		return message[:200]
	}
	return message
}

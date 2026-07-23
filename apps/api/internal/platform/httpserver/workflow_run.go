package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type workflowRunApplication interface {
	CreateRun(context.Context, workflowrun.CreateRunCommand) (workflowrun.WorkflowRun, error)
	ListRuns(context.Context, workflowrun.ListRunsQuery) (workflowrun.RunList, error)
	GetRun(context.Context, uuid.UUID) (workflowrun.WorkflowRun, error)
	ListRunEvents(context.Context, uuid.UUID) ([]workflowrun.Event, error)
	CancelRun(context.Context, workflowrun.RunCommand) (workflowrun.WorkflowRun, error)
	RetryRun(context.Context, workflowrun.RetryCommand) (workflowrun.WorkflowRun, error)
	GetProjectRunSummary(context.Context, uuid.UUID) (workflowrun.Summary, error)
}

type createWorkflowRunRequest struct {
	ProjectID    string          `json:"projectId"`
	Stage        string          `json:"stage"`
	InputPayload json.RawMessage `json:"inputPayload"`
}

type workflowRunCommandRequest struct {
	ExpectedVersion int `json:"expectedVersion"`
}

type workflowRunRetryRequest struct {
	ExpectedVersion         int             `json:"expectedVersion"`
	UseCurrentConfiguration bool            `json:"useCurrentConfiguration"`
	InputOverride           json.RawMessage `json:"inputOverride"`
}

type workflowRunDTO struct {
	ID                      uuid.UUID       `json:"id"`
	RunNumber               string          `json:"runNumber"`
	ProjectID               uuid.UUID       `json:"projectId"`
	Stage                   string          `json:"stage"`
	WorkflowConfigurationID uuid.UUID       `json:"workflowConfigurationId"`
	TriggerSource           string          `json:"triggerSource"`
	Status                  workflowrun.Status `json:"status"`
	InputPayload            json.RawMessage `json:"inputPayload"`
	OutputPayload           json.RawMessage `json:"outputPayload"`
	ErrorCode               *string         `json:"errorCode"`
	ErrorMessage            *string         `json:"errorMessage"`
	ErrorDetails            json.RawMessage `json:"errorDetails"`
	ConfigurationSnapshot   json.RawMessage `json:"configurationSnapshot"`
	StartedAt               *time.Time      `json:"startedAt"`
	FinishedAt              *time.Time      `json:"finishedAt"`
	CancelledAt             *time.Time      `json:"cancelledAt"`
	CreatedAt               time.Time       `json:"createdAt"`
	UpdatedAt               time.Time       `json:"updatedAt"`
	Version                 int             `json:"version"`
}

type workflowRunEventDTO struct {
	ID        uuid.UUID          `json:"id"`
	RunID     uuid.UUID          `json:"runId"`
	EventType string             `json:"eventType"`
	Status    workflowrun.Status `json:"status"`
	Payload   json.RawMessage    `json:"payload"`
	CreatedAt time.Time          `json:"createdAt"`
}

func registerWorkflowRunRoutes(mux *http.ServeMux, app workflowRunApplication) {
	mux.HandleFunc("POST /api/v1/workflow-runs", createWorkflowRunHandler(app))
	mux.HandleFunc("GET /api/v1/workflow-runs", listWorkflowRunsHandler(app))
	mux.HandleFunc("GET /api/v1/workflow-runs/{runId}", getIteration14WorkflowRunHandler(app))
	mux.HandleFunc("GET /api/v1/workflow-runs/{runId}/events", listWorkflowRunEventsHandler(app))
	mux.HandleFunc("POST /api/v1/workflow-runs/{runId}/cancel", cancelWorkflowRunHandler(app))
	mux.HandleFunc("POST /api/v1/workflow-runs/{runId}/retries", retryWorkflowRunHandler(app))
	mux.HandleFunc("GET /api/v1/projects/{projectId}/workflow-run-summary", getProjectWorkflowRunSummaryHandler(app))
}

func createWorkflowRunHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := workflowRunIdempotencyKey(w, r)
		if !ok { return }
		var body createWorkflowRunRequest
		if err := decodeBody(r, &body); err != nil { workflowRunValidationError(w, r, "invalid request body"); return }
		projectID, err := uuid.Parse(body.ProjectID)
		if err != nil { workflowRunValidationError(w, r, "projectId must be a UUID"); return }
		run, err := app.CreateRun(r.Context(), workflowrun.CreateRunCommand{ProjectID: projectID, Stage: body.Stage, InputPayload: body.InputPayload, TriggerSource: "manual", IdempotencyKey: key})
		if err != nil { workflowRunServiceError(w, r, err); return }
		writeJSON(w, r, http.StatusCreated, iteration14WorkflowRunResponse(run))
	}
}

func listWorkflowRunsHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := workflowRunListFilter(r)
		if err != nil { workflowRunValidationError(w, r, err.Error()); return }
		result, err := app.ListRuns(r.Context(), workflowrun.ListRunsQuery{ListFilter: filter})
		if err != nil { workflowRunServiceError(w, r, err); return }
		items := make([]workflowRunDTO, len(result.Items))
		for i, run := range result.Items { items[i] = iteration14WorkflowRunResponse(run) }
		writeJSON(w, r, http.StatusOK, map[string]any{"items": items, "total": result.Total, "limit": result.Limit, "offset": result.Offset})
	}
}

func getIteration14WorkflowRunHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := workflowRunID(w, r); if !ok { return }
		run, err := app.GetRun(r.Context(), id)
		if err != nil { workflowRunServiceError(w, r, err); return }
		writeJSON(w, r, http.StatusOK, iteration14WorkflowRunResponse(run))
	}
}

func listWorkflowRunEventsHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := workflowRunID(w, r); if !ok { return }
		events, err := app.ListRunEvents(r.Context(), id)
		if err != nil { workflowRunServiceError(w, r, err); return }
		items := make([]workflowRunEventDTO, len(events))
		for i, event := range events { items[i] = workflowRunEventResponse(event) }
		writeJSON(w, r, http.StatusOK, map[string]any{"items": items})
	}
}

func cancelWorkflowRunHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := workflowRunID(w, r); if !ok { return }
		key, ok := workflowRunIdempotencyKey(w, r); if !ok { return }
		var body workflowRunCommandRequest
		if err := decodeBody(r, &body); err != nil || body.ExpectedVersion < 1 { workflowRunValidationError(w, r, "expectedVersion must be at least 1"); return }
		run, err := app.CancelRun(r.Context(), workflowrun.RunCommand{RunID: id, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: key})
		if err != nil { workflowRunServiceError(w, r, err); return }
		writeJSON(w, r, http.StatusOK, iteration14WorkflowRunResponse(run))
	}
}

func retryWorkflowRunHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := workflowRunID(w, r); if !ok { return }
		key, ok := workflowRunIdempotencyKey(w, r); if !ok { return }
		var body workflowRunRetryRequest
		if err := decodeBody(r, &body); err != nil || body.ExpectedVersion < 1 { workflowRunValidationError(w, r, "expectedVersion must be at least 1"); return }
		run, err := app.RetryRun(r.Context(), workflowrun.RetryCommand{RunID: id, ExpectedVersion: body.ExpectedVersion, UseCurrentConfiguration: body.UseCurrentConfiguration, InputOverride: body.InputOverride, IdempotencyKey: key})
		if err != nil { workflowRunServiceError(w, r, err); return }
		writeJSON(w, r, http.StatusCreated, iteration14WorkflowRunResponse(run))
	}
}

func getProjectWorkflowRunSummaryHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok { workflowRunValidationError(w, r, "projectId must be a UUID"); return }
		summary, err := app.GetProjectRunSummary(r.Context(), id)
		if err != nil { workflowRunServiceError(w, r, err); return }
		runs := make([]workflowRunDTO, len(summary.RecentRuns))
		for i, run := range summary.RecentRuns { runs[i] = iteration14WorkflowRunResponse(run) }
		writeJSON(w, r, http.StatusOK, map[string]any{"totalRuns": summary.TotalRuns, "activeRuns": summary.ActiveRuns, "recentFailedRuns": summary.RecentFailedRuns, "lastRunAt": summary.LastRunAt, "recentRuns": runs})
	}
}

func workflowRunListFilter(r *http.Request) (workflowrun.ListFilter, error) {
	q := r.URL.Query()
	filter := workflowrun.ListFilter{Stage: strings.TrimSpace(q.Get("stage")), WorkflowConfigurationID: strings.TrimSpace(q.Get("workflowConfigurationId")), Status: strings.TrimSpace(q.Get("status")), TriggerSource: strings.TrimSpace(q.Get("triggerSource")), RunNumber: strings.TrimSpace(q.Get("runNumber")), Query: strings.TrimSpace(q.Get("q")), Limit: 20}
	if raw := strings.TrimSpace(q.Get("projectId")); raw != "" { id, err := uuid.Parse(raw); if err != nil { return filter, errors.New("projectId must be a UUID") }; filter.ProjectID = &id }
	if filter.WorkflowConfigurationID != "" { if _, err := uuid.Parse(filter.WorkflowConfigurationID); err != nil { return filter, errors.New("workflowConfigurationId must be a UUID") } }
	if filter.Stage != "" && !workflowRunStage(filter.Stage) { return filter, errors.New("invalid stage") }
	if filter.Status != "" && !workflowRunStatus(filter.Status) { return filter, errors.New("invalid status") }
	if filter.TriggerSource != "" && !workflowRunTriggerSource(filter.TriggerSource) { return filter, errors.New("invalid triggerSource") }
	var err error
	if raw := strings.TrimSpace(q.Get("startTime")); raw != "" { value, parseErr := time.Parse(time.RFC3339, raw); if parseErr != nil { return filter, errors.New("startTime must be RFC3339") }; filter.StartTime = &value }
	if raw := strings.TrimSpace(q.Get("endTime")); raw != "" { value, parseErr := time.Parse(time.RFC3339, raw); if parseErr != nil { return filter, errors.New("endTime must be RFC3339") }; filter.EndTime = &value }
	if filter.StartTime != nil && filter.EndTime != nil && filter.StartTime.After(*filter.EndTime) { return filter, errors.New("startTime must not be after endTime") }
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" { filter.Limit, err = strconv.Atoi(raw); if err != nil || filter.Limit < 1 || filter.Limit > 100 { return filter, errors.New("invalid limit") } }
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" { filter.Offset, err = strconv.Atoi(raw); if err != nil || filter.Offset < 0 { return filter, errors.New("invalid offset") } }
	return filter, nil
}

func iteration14WorkflowRunResponse(run workflowrun.WorkflowRun) workflowRunDTO {
	return workflowRunDTO{ID: run.ID, RunNumber: run.RunNumber, ProjectID: run.ProjectID, Stage: run.Stage, WorkflowConfigurationID: run.WorkflowConfigurationID, TriggerSource: run.TriggerSource, Status: run.Status, InputPayload: workflowrun.RedactJSON(run.InputPayload), OutputPayload: workflowRunNullableJSON(run.OutputPayload), ErrorCode: run.ErrorCode, ErrorMessage: run.ErrorMessage, ErrorDetails: workflowRunNullableJSON(run.ErrorDetails), ConfigurationSnapshot: workflowrun.RedactJSON(run.ConfigurationSnapshot), StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, CancelledAt: run.CancelledAt, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt, Version: run.Version}
}

func workflowRunEventResponse(event workflowrun.Event) workflowRunEventDTO { return workflowRunEventDTO{ID: event.ID, RunID: event.RunID, EventType: event.EventType, Status: event.Status, Payload: workflowrun.RedactJSON(event.Payload), CreatedAt: event.CreatedAt} }
func workflowRunNullableJSON(value json.RawMessage) json.RawMessage { if value == nil { return nil }; return workflowrun.RedactJSON(value) }
func workflowRunID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) { id, err := uuid.Parse(r.PathValue("runId")); if err != nil { workflowRunValidationError(w, r, "runId must be a UUID"); return uuid.Nil, false }; return id, true }
func workflowRunIdempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) { key := strings.TrimSpace(r.Header.Get("Idempotency-Key")); if key == "" || len(key) > 128 { workflowRunValidationError(w, r, "Idempotency-Key is required"); return "", false }; return key, true }
func workflowRunValidationError(w http.ResponseWriter, r *http.Request, message string) { writeError(w, r, http.StatusBadRequest, "validation_error", message, map[string]any{}) }
func workflowRunStage(value string) bool { return value == "chapter_planning" || value == "content_generation" || value == "review" || value == "rewrite" }
func workflowRunStatus(value string) bool { return value == string(workflowrun.StatusQueued) || value == string(workflowrun.StatusRunning) || value == string(workflowrun.StatusSucceeded) || value == string(workflowrun.StatusFailed) || value == string(workflowrun.StatusCancelled) }
func workflowRunTriggerSource(value string) bool { return value == "manual" || value == "retry" || value == "system" || value == "api" }

func workflowRunServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, workflowrun.ErrValidation): workflowRunValidationError(w, r, "invalid workflow run request")
	case errors.Is(err, workflowrun.ErrProjectNotFound): writeError(w, r, http.StatusNotFound, "project_not_found", "project not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrBindingNotFound): writeError(w, r, http.StatusNotFound, "workflow_binding_not_found", "workflow binding not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrConfigurationNotFound): writeError(w, r, http.StatusNotFound, "workflow_configuration_not_found", "workflow configuration not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrConnectionNotFound): writeError(w, r, http.StatusNotFound, "workflow_connection_not_found", "workflow connection not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotRunnable): writeError(w, r, http.StatusConflict, "workflow_not_runnable", "workflow is not runnable", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotFound): writeError(w, r, http.StatusNotFound, "workflow_run_not_found", "workflow run not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrVersionConflict): writeError(w, r, http.StatusConflict, "version_conflict", "workflow run version conflict", map[string]any{})
	case errors.Is(err, workflowrun.ErrIdempotencyConflict): writeError(w, r, http.StatusConflict, "idempotency_key_reused_with_different_payload", "idempotency key conflicts with a different request", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotCancellable): writeError(w, r, http.StatusConflict, "validation_error", "workflow run cannot be cancelled", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotRetryable): writeError(w, r, http.StatusConflict, "validation_error", "workflow run cannot be retried", map[string]any{})
	case errors.Is(err, workflowrun.ErrExecutorUnavailable): writeError(w, r, http.StatusServiceUnavailable, "executor_unavailable", "workflow executor is unavailable", map[string]any{})
	default: writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error", map[string]any{})
	}
}

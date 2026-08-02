package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

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
	GetRetryOptions(context.Context, uuid.UUID) (workflowrun.RetryOptions, error)
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
	Mode                    *string         `json:"mode"`
	Reason                  *string         `json:"reason"`
	UseCurrentConfiguration *bool           `json:"useCurrentConfiguration"`
	InputOverride           json.RawMessage `json:"inputOverride"`
}

type workflowRunDTO struct {
	ID                           uuid.UUID          `json:"id"`
	RunNumber                    string             `json:"runNumber"`
	ProjectID                    uuid.UUID          `json:"projectId"`
	Stage                        string             `json:"stage"`
	SubjectType                  *string            `json:"subjectType"`
	SubjectID                    *uuid.UUID         `json:"subjectId"`
	WorkflowConfigurationID      uuid.UUID          `json:"workflowConfigurationId"`
	WorkflowName                 *string            `json:"workflowName"`
	WorkflowConfigurationVersion *int               `json:"workflowConfigurationVersion"`
	TriggerSource                string             `json:"triggerSource"`
	RetryOfRunID                 *uuid.UUID         `json:"retryOfRunId"`
	Status                       workflowrun.Status `json:"status"`
	DisplayStatus                string             `json:"displayStatus"`
	FailurePhase                 *string            `json:"failurePhase"`
	FailureCode                  *string            `json:"failureCode"`
	SafeError                    any                `json:"safeError"`
	DomainImpact                 []any              `json:"domainImpact"`
	ConnectionSummary            any                `json:"connectionSummary"`
	LlmPolicySummary             any                `json:"llmPolicySummary"`
	Retryability                 string             `json:"retryability"`
	RetryMode                    *string            `json:"retryMode"`
	ExternalExecutionID          *string            `json:"externalExecutionId"`
	CancellationRequestedAt      *time.Time         `json:"cancellationRequestedAt"`
	TimedOutAt                   *time.Time         `json:"timedOutAt"`
	InputPayload                 json.RawMessage    `json:"inputPayload"`
	OutputPayload                json.RawMessage    `json:"outputPayload"`
	ErrorCode                    *string            `json:"errorCode"`
	ErrorMessage                 *string            `json:"errorMessage"`
	ErrorDetails                 json.RawMessage    `json:"errorDetails"`
	ConfigurationSnapshot        json.RawMessage    `json:"configurationSnapshot"`
	BindingSnapshot              json.RawMessage    `json:"bindingSnapshot"`
	ConnectionSnapshot           json.RawMessage    `json:"connectionSnapshot"`
	LlmPolicySnapshot            json.RawMessage    `json:"llmPolicySnapshot"`
	StartedAt                    *time.Time         `json:"startedAt"`
	FinishedAt                   *time.Time         `json:"finishedAt"`
	CancelledAt                  *time.Time         `json:"cancelledAt"`
	CreatedAt                    time.Time          `json:"createdAt"`
	UpdatedAt                    time.Time          `json:"updatedAt"`
	Version                      int                `json:"version"`
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
	mux.HandleFunc("GET /api/v1/workflow-runs/{runId}/retry-options", getWorkflowRunRetryOptionsHandler(app))
	mux.HandleFunc("GET /api/v1/projects/{projectId}/workflow-run-summary", getProjectWorkflowRunSummaryHandler(app))
}

func createWorkflowRunHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key, ok := workflowRunIdempotencyKey(w, r)
		if !ok {
			return
		}
		var body createWorkflowRunRequest
		if err := decodeBody(r, &body); err != nil {
			workflowRunValidationError(w, r, "invalid request body")
			return
		}
		projectID, err := uuid.Parse(body.ProjectID)
		if err != nil {
			workflowRunValidationError(w, r, "projectId must be a UUID")
			return
		}
		run, err := app.CreateRun(r.Context(), workflowrun.CreateRunCommand{ProjectID: projectID, Stage: body.Stage, InputPayload: body.InputPayload, TriggerSource: "manual", IdempotencyKey: key})
		if err != nil {
			workflowRunServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusCreated, iteration14WorkflowRunResponse(run))
	}
}

func listWorkflowRunsHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter, err := workflowRunListFilter(r)
		if err != nil {
			workflowRunValidationError(w, r, err.Error())
			return
		}
		result, err := app.ListRuns(r.Context(), workflowrun.ListRunsQuery{ListFilter: filter})
		if err != nil {
			workflowRunServiceError(w, r, err)
			return
		}
		items := make([]workflowRunDTO, len(result.Items))
		for i, run := range result.Items {
			items[i] = iteration14WorkflowRunResponse(run)
		}
		writeJSON(w, r, http.StatusOK, map[string]any{"items": items, "total": result.Total, "limit": result.Limit, "offset": result.Offset})
	}
}

func getIteration14WorkflowRunHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := workflowRunID(w, r)
		if !ok {
			return
		}
		run, err := app.GetRun(r.Context(), id)
		if err != nil {
			workflowRunServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, iteration14WorkflowRunResponse(run))
	}
}

func listWorkflowRunEventsHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := workflowRunID(w, r)
		if !ok {
			return
		}
		events, err := app.ListRunEvents(r.Context(), id)
		if err != nil {
			workflowRunServiceError(w, r, err)
			return
		}
		items := make([]workflowRunEventDTO, len(events))
		for i, event := range events {
			items[i] = workflowRunEventResponse(event)
		}
		writeJSON(w, r, http.StatusOK, map[string]any{"items": items})
	}
}

func cancelWorkflowRunHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := workflowRunID(w, r)
		if !ok {
			return
		}
		key, ok := workflowRunIdempotencyKey(w, r)
		if !ok {
			return
		}
		var body workflowRunCommandRequest
		if err := decodeBody(r, &body); err != nil || body.ExpectedVersion < 1 {
			workflowRunValidationError(w, r, "expectedVersion must be at least 1")
			return
		}
		run, err := app.CancelRun(r.Context(), workflowrun.RunCommand{RunID: id, ExpectedVersion: body.ExpectedVersion, IdempotencyKey: key})
		if err != nil {
			workflowRunServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, iteration14WorkflowRunResponse(run))
	}
}

func retryWorkflowRunHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := workflowRunID(w, r)
		if !ok {
			return
		}
		key, ok := workflowRunIdempotencyKey(w, r)
		if !ok {
			return
		}
		var body workflowRunRetryRequest
		if err := decodeBody(r, &body); err != nil || body.ExpectedVersion < 1 {
			workflowRunValidationError(w, r, "expectedVersion must be at least 1")
			return
		}
		mode, modeErr := workflowRunRetryMode(body)
		if modeErr != nil {
			workflowRunValidationError(w, r, modeErr.Error())
			return
		}
		if body.Reason != nil && utf8.RuneCountInString(*body.Reason) > 500 {
			workflowRunValidationError(w, r, "reason must not exceed 500 characters")
			return
		}
		command := workflowrun.RetryCommand{RunID: id, ExpectedVersion: body.ExpectedVersion, Mode: mode, Reason: body.Reason, InputOverride: body.InputOverride, IdempotencyKey: key}
		run, replay, err := workflowrun.WorkflowRun{}, false, error(nil)
		if replayApplication, ok := app.(interface {
			RetryRunWithReplay(context.Context, workflowrun.RetryCommand) (workflowrun.WorkflowRun, bool, error)
		}); ok {
			run, replay, err = replayApplication.RetryRunWithReplay(r.Context(), command)
		} else {
			run, err = app.RetryRun(r.Context(), command)
		}
		if err != nil {
			workflowRunServiceError(w, r, err)
			return
		}
		status := http.StatusCreated
		if replay {
			status = http.StatusOK
		}
		writeJSON(w, r, status, iteration14WorkflowRunResponse(run))
	}
}

func getWorkflowRunRetryOptionsHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := workflowRunID(w, r)
		if !ok {
			return
		}
		options, err := app.GetRetryOptions(r.Context(), id)
		if err != nil {
			workflowRunServiceError(w, r, err)
			return
		}
		writeJSON(w, r, http.StatusOK, options)
	}
}

func workflowRunRetryMode(body workflowRunRetryRequest) (string, error) {
	if body.Mode == nil && body.UseCurrentConfiguration == nil {
		return "", errors.New("mode is required")
	}
	mode := ""
	if body.Mode != nil {
		mode = strings.TrimSpace(*body.Mode)
	}
	if mode != "" && mode != "current_configuration" && mode != "original_configuration" {
		return "", errors.New("invalid mode")
	}
	if body.UseCurrentConfiguration != nil {
		legacy := "original_configuration"
		if *body.UseCurrentConfiguration {
			legacy = "current_configuration"
		}
		if mode != "" && mode != legacy {
			return "", errors.New("mode conflicts with useCurrentConfiguration")
		}
		mode = legacy
	}
	if mode == "" {
		return "", errors.New("mode is required")
	}
	return mode, nil
}

func getProjectWorkflowRunSummaryHandler(app workflowRunApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := projectID(r)
		if !ok {
			workflowRunValidationError(w, r, "projectId must be a UUID")
			return
		}
		summary, err := app.GetProjectRunSummary(r.Context(), id)
		if err != nil {
			workflowRunServiceError(w, r, err)
			return
		}
		runs := make([]workflowRunDTO, len(summary.RecentRuns))
		for i, run := range summary.RecentRuns {
			runs[i] = iteration14WorkflowRunResponse(run)
		}
		writeJSON(w, r, http.StatusOK, map[string]any{"totalRuns": summary.TotalRuns, "activeRuns": summary.ActiveRuns, "recentFailedRuns": summary.RecentFailedRuns, "lastRunAt": summary.LastRunAt, "recentRuns": runs})
	}
}

func workflowRunListFilter(r *http.Request) (workflowrun.ListFilter, error) {
	q := r.URL.Query()
	filter := workflowrun.ListFilter{Stage: strings.TrimSpace(q.Get("stage")), WorkflowConfigurationID: strings.TrimSpace(q.Get("workflowConfigurationId")), Status: strings.TrimSpace(q.Get("status")), DisplayStatus: strings.TrimSpace(q.Get("displayStatus")), ConnectionID: strings.TrimSpace(q.Get("connectionId")), ProviderID: strings.TrimSpace(q.Get("providerId")), Model: strings.TrimSpace(q.Get("model")), Retryability: strings.TrimSpace(q.Get("retryability")), TriggerSource: strings.TrimSpace(q.Get("triggerSource")), RunNumber: strings.TrimSpace(q.Get("runNumber")), Query: strings.TrimSpace(q.Get("q")), Limit: 20}
	if raw := strings.TrimSpace(q.Get("projectId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return filter, errors.New("projectId must be a UUID")
		}
		filter.ProjectID = &id
	}
	if filter.WorkflowConfigurationID != "" {
		if _, err := uuid.Parse(filter.WorkflowConfigurationID); err != nil {
			return filter, errors.New("workflowConfigurationId must be a UUID")
		}
	}
	for name, value := range map[string]string{"connectionId": filter.ConnectionID, "providerId": filter.ProviderID} {
		if value != "" {
			if _, err := uuid.Parse(value); err != nil {
				return filter, errors.New(name + " must be a UUID")
			}
		}
	}
	if filter.Stage != "" && !workflowRunStage(filter.Stage) {
		return filter, errors.New("invalid stage")
	}
	if filter.Status != "" && !workflowRunStatus(filter.Status) {
		return filter, errors.New("invalid status")
	}
	if filter.DisplayStatus != "" && !workflowRunDisplayStatus(filter.DisplayStatus) {
		return filter, errors.New("invalid displayStatus")
	}
	if filter.Retryability != "" && filter.Retryability != "runtime_retry" && filter.Retryability != "result_consumption_retry" && filter.Retryability != "not_retryable" {
		return filter, errors.New("invalid retryability")
	}
	if filter.TriggerSource != "" && !workflowRunTriggerSource(filter.TriggerSource) {
		return filter, errors.New("invalid triggerSource")
	}
	var err error
	startRaw, endRaw := strings.TrimSpace(q.Get("startTime")), strings.TrimSpace(q.Get("endTime"))
	if startRaw == "" {
		startRaw = strings.TrimSpace(q.Get("from"))
	}
	if endRaw == "" {
		endRaw = strings.TrimSpace(q.Get("to"))
	}
	if startRaw != "" {
		value, parseErr := time.Parse(time.RFC3339, startRaw)
		if parseErr != nil {
			return filter, errors.New("from/startTime must be RFC3339")
		}
		filter.StartTime = &value
	}
	if endRaw != "" {
		value, parseErr := time.Parse(time.RFC3339, endRaw)
		if parseErr != nil {
			return filter, errors.New("to/endTime must be RFC3339")
		}
		filter.EndTime = &value
	}
	if filter.StartTime != nil && filter.EndTime != nil && filter.StartTime.After(*filter.EndTime) {
		return filter, errors.New("startTime must not be after endTime")
	}
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		filter.Limit, err = strconv.Atoi(raw)
		if err != nil || filter.Limit < 1 || filter.Limit > 100 {
			return filter, errors.New("invalid limit")
		}
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		filter.Offset, err = strconv.Atoi(raw)
		if err != nil || filter.Offset < 0 {
			return filter, errors.New("invalid offset")
		}
	}
	if raw := strings.TrimSpace(q.Get("configurationVersion")); raw != "" {
		filter.ConfigurationVersion, err = strconv.Atoi(raw)
		if err != nil || filter.ConfigurationVersion < 1 {
			return filter, errors.New("invalid configurationVersion")
		}
	}
	return filter, nil
}

func iteration14WorkflowRunResponse(run workflowrun.WorkflowRun) workflowRunDTO {
	inputPayload := workflowrun.RedactJSON(run.InputPayload)
	outputPayload := workflowRunNullableJSON(run.OutputPayload)
	errorDetails := workflowRunNullableJSON(run.ErrorDetails)
	configurationSnapshot := workflowrun.RedactJSON(run.ConfigurationSnapshot)
	bindingSnapshot := workflowrun.RedactJSON(run.BindingSnapshot)
	connectionSnapshot := workflowrun.RedactJSON(run.ConnectionSnapshot)
	llmPolicySnapshot := workflowrun.RedactJSON(run.LlmPolicySnapshot)
	if run.Stage == "rewrite" {
		inputPayload = safeRewriteRunInputPayload(run.InputPayload)
		outputPayload = nil
		errorDetails = nil
		configurationSnapshot = json.RawMessage(`{}`)
	}
	workflowName, configurationVersion := workflowRunConfigurationProjection(run.ConfigurationSnapshot)
	displayStatus := string(run.Status)
	if run.Status == workflowrun.StatusFailed && run.FailurePhase != nil && (*run.FailurePhase == "output_validation" || *run.FailurePhase == "result_consumption") {
		displayStatus = *run.FailurePhase + "_failed"
	}
	var safeError any
	code, message := run.FailureCode, run.SafeErrorMessage
	if code == nil {
		code = run.ErrorCode
	}
	if message == nil {
		message = run.ErrorMessage
	}
	if code != nil && message != nil {
		safeError = map[string]any{"code": *code, "message": *message}
	}
	retryability := run.Retryability
	if retryability == "" {
		retryability = "not_retryable"
	}
	return workflowRunDTO{ID: run.ID, RunNumber: run.RunNumber, ProjectID: run.ProjectID, Stage: run.Stage, SubjectType: run.SubjectType, SubjectID: run.SubjectID, WorkflowConfigurationID: run.WorkflowConfigurationID, WorkflowName: workflowName, WorkflowConfigurationVersion: configurationVersion, TriggerSource: run.TriggerSource, RetryOfRunID: run.RetryOfRunID, Status: run.Status, DisplayStatus: displayStatus, FailurePhase: run.FailurePhase, FailureCode: run.FailureCode, SafeError: safeError, DomainImpact: []any{}, ConnectionSummary: workflowRunConnectionSummary(connectionSnapshot, run.ConfigurationSnapshot), LlmPolicySummary: workflowRunLlmPolicySummary(llmPolicySnapshot), Retryability: retryability, RetryMode: run.RetryMode, ExternalExecutionID: run.ExternalExecutionID, CancellationRequestedAt: run.CancellationRequestedAt, TimedOutAt: run.TimedOutAt, InputPayload: inputPayload, OutputPayload: outputPayload, ErrorCode: run.ErrorCode, ErrorMessage: run.ErrorMessage, ErrorDetails: errorDetails, ConfigurationSnapshot: configurationSnapshot, BindingSnapshot: bindingSnapshot, ConnectionSnapshot: connectionSnapshot, LlmPolicySnapshot: llmPolicySnapshot, StartedAt: run.StartedAt, FinishedAt: run.FinishedAt, CancelledAt: run.CancelledAt, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt, Version: run.Version}
}

func workflowRunConfigurationProjection(raw json.RawMessage) (*string, *int) {
	var value struct {
		WorkflowConfiguration struct {
			Name    *string `json:"name"`
			Version *int    `json:"version"`
		} `json:"workflowConfiguration"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return nil, nil
	}
	return value.WorkflowConfiguration.Name, value.WorkflowConfiguration.Version
}

func workflowRunConnectionSummary(raw, configurationRaw json.RawMessage) any {
	var value struct {
		ID               uuid.UUID `json:"id"`
		Name             string    `json:"name"`
		ConnectionType   string    `json:"connectionType"`
		ValidationStatus string    `json:"validationStatus"`
		Enabled          bool      `json:"enabled"`
		Executable       bool      `json:"executable"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	var configuration struct {
		WorkflowConnection struct {
			ValidationStatus string `json:"validationStatus"`
			Enabled          bool   `json:"enabled"`
			Executable       bool   `json:"executable"`
		} `json:"workflowConnection"`
	}
	_ = json.Unmarshal(configurationRaw, &configuration)
	value.ValidationStatus, value.Enabled, value.Executable = configuration.WorkflowConnection.ValidationStatus, configuration.WorkflowConnection.Enabled, configuration.WorkflowConnection.Executable
	if value.ID == uuid.Nil || value.Name == "" || value.ConnectionType == "" || value.ValidationStatus == "" {
		return nil
	}
	return value
}

func workflowRunLlmPolicySummary(raw json.RawMessage) any {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil || value["strategy"] == nil {
		return nil
	}
	delete(value, "secretFingerprint")
	if _, ok := value["validationStatus"]; !ok {
		value["validationStatus"] = nil
	}
	if _, ok := value["executable"]; !ok {
		value["executable"] = value["strategy"] != "acf_managed"
	}
	return value
}

func safeRewriteRunInputPayload(value json.RawMessage) json.RawMessage {
	var input struct {
		ContentItemID  string `json:"contentItemId"`
		SelectedIssues []struct {
			ReviewIssueID string `json:"reviewIssueId"`
			IssueKey      string `json:"issueKey"`
			Position      int    `json:"position"`
			Title         string `json:"title"`
			Severity      string `json:"severity"`
		} `json:"selectedIssues"`
	}
	if err := json.Unmarshal(value, &input); err != nil {
		return json.RawMessage(`{}`)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return payload
}

func workflowRunEventResponse(event workflowrun.Event) workflowRunEventDTO {
	return workflowRunEventDTO{ID: event.ID, RunID: event.RunID, EventType: event.EventType, Status: event.Status, Payload: workflowrun.RedactJSON(event.Payload), CreatedAt: event.CreatedAt}
}
func workflowRunNullableJSON(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	return workflowrun.RedactJSON(value)
}
func workflowRunID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("runId"))
	if err != nil {
		workflowRunValidationError(w, r, "runId must be a UUID")
		return uuid.Nil, false
	}
	return id, true
}
func workflowRunIdempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		workflowRunValidationError(w, r, "Idempotency-Key is required")
		return "", false
	}
	return key, true
}
func workflowRunValidationError(w http.ResponseWriter, r *http.Request, message string) {
	writeError(w, r, http.StatusBadRequest, "validation_error", message, map[string]any{})
}
func workflowRunStage(value string) bool {
	return value == "chapter_planning" || value == "content_generation" || value == "review" || value == "rewrite"
}
func workflowRunStatus(value string) bool {
	return value == string(workflowrun.StatusQueued) || value == string(workflowrun.StatusRunning) || value == string(workflowrun.StatusCancelling) || value == string(workflowrun.StatusSucceeded) || value == string(workflowrun.StatusFailed) || value == string(workflowrun.StatusCancelled) || value == string(workflowrun.StatusTimedOut)
}
func workflowRunDisplayStatus(value string) bool {
	return workflowRunStatus(value) || value == "output_validation_failed" || value == "result_consumption_failed"
}
func workflowRunTriggerSource(value string) bool {
	return value == "manual" || value == "retry" || value == "system" || value == "api"
}

func workflowRunServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, workflowrun.ErrValidation):
		workflowRunValidationError(w, r, "invalid workflow run request")
	case errors.Is(err, workflowrun.ErrProtectedStage):
		writeError(w, r, http.StatusConflict, "validation_error", "workflow stage requires its dedicated command", map[string]any{})
	case errors.Is(err, workflowrun.ErrProjectNotFound):
		writeError(w, r, http.StatusNotFound, "project_not_found", "project not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrBindingNotFound):
		writeError(w, r, http.StatusNotFound, "workflow_binding_not_found", "workflow binding not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrConfigurationNotFound):
		writeError(w, r, http.StatusNotFound, "workflow_configuration_not_found", "workflow configuration not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrConnectionNotFound):
		writeError(w, r, http.StatusNotFound, "workflow_connection_not_found", "workflow connection not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotRunnable):
		writeError(w, r, http.StatusConflict, "workflow_not_runnable", "workflow is not runnable", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "workflow_run_not_found", "workflow run not found", map[string]any{})
	case errors.Is(err, workflowrun.ErrVersionConflict):
		writeError(w, r, http.StatusConflict, "version_conflict", "workflow run version conflict", map[string]any{})
	case errors.Is(err, workflowrun.ErrIdempotencyConflict):
		writeError(w, r, http.StatusConflict, "idempotency_key_reused_with_different_payload", "idempotency key conflicts with a different request", map[string]any{})
	case errors.Is(err, workflowrun.ErrRewriteVersionConflict):
		writeError(w, r, http.StatusConflict, "workflow_run_version_conflict", "workflow run version conflict", map[string]any{})
	case errors.Is(err, workflowrun.ErrRewriteIdempotencyConflict):
		writeError(w, r, http.StatusConflict, "idempotency_conflict", "idempotency key conflicts with a different request", map[string]any{})
	case errors.Is(err, workflowrun.ErrActiveRewriteRun):
		writeError(w, r, http.StatusConflict, "active_rewrite_run_conflict", "an active rewrite run already exists", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotCancellable):
		writeError(w, r, http.StatusConflict, "validation_error", "workflow run cannot be cancelled", map[string]any{})
	case errors.Is(err, workflowrun.ErrNotRetryable):
		writeError(w, r, http.StatusConflict, "validation_error", "workflow run cannot be retried", map[string]any{})
	case errors.Is(err, workflowrun.ErrExecutorUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "executor_unavailable", "workflow executor is unavailable", map[string]any{})
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error", map[string]any{})
	}
}

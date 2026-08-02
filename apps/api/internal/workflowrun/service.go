package workflowrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
)

var (
	ErrProjectNotFound            = errors.New("project not found")
	ErrBindingNotFound            = errors.New("workflow binding not found")
	ErrConfigurationNotFound      = errors.New("workflow configuration not found")
	ErrConnectionNotFound         = errors.New("workflow connection not found")
	ErrNotRunnable                = errors.New("workflow is not runnable")
	ErrNotCancellable             = errors.New("workflow run is not cancellable")
	ErrNotRetryable               = errors.New("workflow run is not retryable")
	ErrActiveRewriteRun           = errors.New("active rewrite run conflict")
	ErrIdempotencyConflict        = errors.New("idempotency key reused with different payload")
	ErrRewriteVersionConflict     = errors.New("rewrite workflow run version conflict")
	ErrRewriteIdempotencyConflict = errors.New("rewrite idempotency conflict")
	ErrProtectedStage             = errors.New("workflow stage requires its domain command")
)

type ProjectReader interface {
	Get(context.Context, uuid.UUID) (project.Project, error)
}
type BindingReader interface {
	GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error)
}
type ConfigurationReader interface {
	GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error)
}
type ConnectionReader interface {
	GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error)
}
type Store interface {
	CreateWithInitialEvent(context.Context, WorkflowRun, Event) (WorkflowRun, Event, error)
	GetByID(context.Context, uuid.UUID) (WorkflowRun, error)
	List(context.Context, ListFilter) ([]WorkflowRun, error)
	ListEvents(context.Context, uuid.UUID) ([]Event, error)
	AddEvent(context.Context, Event) (Event, error)
	UpdateStatusWithEvent(context.Context, WorkflowRun, WorkflowRun, Event) (WorkflowRun, Event, error)
	SaveExternalExecutionID(context.Context, WorkflowRun, string) (WorkflowRun, error)
	QuerySummary(context.Context, uuid.UUID, int) (Summary, error)
	Count(context.Context, ListFilter) (int, error)
	ExecuteIdempotent(context.Context, string, string, string, func(Store) (WorkflowRun, error)) (WorkflowRun, error)
	ExecuteIdempotentWithReplay(context.Context, string, string, string, func(Store) (WorkflowRun, error)) (WorkflowRun, bool, error)
	PreflightTokenUsed(context.Context, string) (bool, error)
}

type CreateRunCommand struct {
	ProjectID             uuid.UUID
	RunID                 uuid.UUID
	Stage                 string
	SubjectType           *string
	SubjectID             *uuid.UUID
	InputPayload          json.RawMessage
	TriggerSource         string
	IdempotencyKey        string
	PreparedConfiguration *PreparedRunConfiguration
}
type CreateRunPreparation func() (CreateRunCommand, error)
type CreateRunTxPreparation func(pgx.Tx) (CreateRunCommand, error)
type PreparedRunConfiguration struct {
	WorkflowConfigurationID uuid.UUID
	Snapshot                json.RawMessage
}
type RunCommand struct {
	RunID           uuid.UUID
	ExpectedVersion int
	IdempotencyKey  string
}
type RetryCommand struct {
	RunID           uuid.UUID
	ExpectedVersion int
	Mode            string
	Reason          *string
	InputOverride   json.RawMessage
	IdempotencyKey  string
}
type RetryReason struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	RepairAction string `json:"repairAction,omitempty"`
}
type ConfigurationDifference struct {
	Field   string `json:"field"`
	Changed bool   `json:"changed"`
	Summary string `json:"summary"`
}
type RetryOption struct {
	Mode                     string                    `json:"mode"`
	Enabled                  bool                      `json:"enabled"`
	Reasons                  []RetryReason             `json:"reasons"`
	ConfigurationDifferences []ConfigurationDifference `json:"configurationDifferences,omitempty"`
}
type RetryOptions struct {
	RunID                          uuid.UUID   `json:"runId"`
	Retryability                   string      `json:"retryability"`
	CurrentConfiguration           RetryOption `json:"currentConfiguration"`
	OriginalConfiguration          RetryOption `json:"originalConfiguration"`
	ResultConsumptionRetryRequired bool        `json:"resultConsumptionRetryRequired"`
}
type ListRunsQuery struct{ ListFilter }
type RunList struct {
	Items                []WorkflowRun
	Total, Limit, Offset int
}

type Service struct {
	store             Store
	projects          ProjectReader
	bindings          BindingReader
	configurations    ConfigurationReader
	connections       ConnectionReader
	now               func() time.Time
	newID             func() uuid.UUID
	newRunNumber      func() string
	executor          WorkflowExecutor
	succeededConsumer interface {
		ConsumeSucceededRun(context.Context, WorkflowRun) error
	}
	contentSucceededConsumer interface {
		ConsumeSucceededRun(context.Context, WorkflowRun) error
	}
	reviewSucceededConsumer interface {
		ConsumeSucceededRun(context.Context, WorkflowRun) error
	}
	rewriteSucceededConsumer interface {
		ConsumeSucceededRun(context.Context, WorkflowRun) error
	}
}

func NewService(store Store, projects ProjectReader, bindings BindingReader, configurations ConfigurationReader, connections ConnectionReader) *Service {
	return &Service{store: store, projects: projects, bindings: bindings, configurations: configurations, connections: connections, now: func() time.Time { return time.Now().UTC() }, newID: uuid.New, newRunNumber: func() string { return "WR-" + strings.ToUpper(uuid.NewString()[:8]) }, executor: UnavailableWorkflowExecutor{}}
}

func (s *Service) SetWorkflowExecutor(executor WorkflowExecutor) {
	if executor == nil {
		s.executor = UnavailableWorkflowExecutor{}
		return
	}
	s.executor = executor
}
func (s *Service) SetSucceededConsumer(consumer interface {
	ConsumeSucceededRun(context.Context, WorkflowRun) error
}) {
	s.succeededConsumer = consumer
}
func (s *Service) SetContentSucceededConsumer(consumer interface {
	ConsumeSucceededRun(context.Context, WorkflowRun) error
}) {
	s.contentSucceededConsumer = consumer
}
func (s *Service) SetReviewSucceededConsumer(consumer interface {
	ConsumeSucceededRun(context.Context, WorkflowRun) error
}) {
	s.reviewSucceededConsumer = consumer
}
func (s *Service) SetRewriteSucceededConsumer(consumer interface {
	ConsumeSucceededRun(context.Context, WorkflowRun) error
}) {
	s.rewriteSucceededConsumer = consumer
}

// ExecuteRun is an explicit application boundary. It never polls or schedules work.
func (s *Service) ExecuteRun(ctx context.Context, runID uuid.UUID) (WorkflowRun, error) {
	run, err := s.GetRun(ctx, runID)
	if err != nil {
		return WorkflowRun{}, err
	}
	request, err := executionRequest(run)
	if err != nil {
		return WorkflowRun{}, ErrValidation
	}
	if run.Status == StatusQueued {
		next, startErr := run.Start(s.now())
		if startErr != nil {
			return WorkflowRun{}, startErr
		}
		started := Event{ID: s.newID(), RunID: run.ID, EventType: "worker_started", Status: StatusRunning, Payload: json.RawMessage(`{}`), CreatedAt: next.UpdatedAt}
		run, _, err = s.store.UpdateStatusWithEvent(ctx, run, next, started)
		if err != nil {
			return WorkflowRun{}, mapStoreError(err)
		}
	} else if run.Status != StatusRunning {
		return WorkflowRun{}, ErrInvalidTransition
	}
	result, err := s.executor.Execute(ctx, request)
	if err != nil {
		if errors.Is(err, ErrExecutionTimeout) {
			return s.timeoutExecution(ctx, run)
		}
		return s.failExecution(ctx, run, executionErrorCode(err), "workflow execution failed")
	}
	if !validExecutionResult(result) {
		return s.failExecution(ctx, run, "invalid_response", "workflow execution returned an invalid result")
	}
	if strings.TrimSpace(result.ExternalExecutionID) != "" {
		run, err = s.store.SaveExternalExecutionID(ctx, run, result.ExternalExecutionID)
		if err != nil {
			return WorkflowRun{}, mapStoreError(err)
		}
	}
	return s.applyExecutionResult(ctx, run, result)
}

func (s *Service) applyExecutionResult(ctx context.Context, run WorkflowRun, result ExecutionResult) (WorkflowRun, error) {
	if result.Status == ExecutionAccepted {
		return run, nil
	}
	if result.Status == ExecutionRunning {
		return run, nil
	}
	if result.Status == ExecutionSucceeded {
		next, err := run.Succeed(s.now(), RedactJSON(result.Output))
		if err != nil {
			return WorkflowRun{}, err
		}
		event := Event{ID: s.newID(), RunID: run.ID, EventType: "succeeded", Status: StatusSucceeded, Payload: executionEventPayload(result), CreatedAt: next.UpdatedAt}
		updated, _, err := s.store.UpdateStatusWithEvent(ctx, run, next, event)
		if err != nil {
			return updated, mapStoreError(err)
		}
		if s.succeededConsumer != nil && updated.Stage == "chapter_planning" {
			if err := s.succeededConsumer.ConsumeSucceededRun(ctx, updated); err != nil {
				return updated, err
			}
		}
		if s.contentSucceededConsumer != nil && updated.Stage == "content_generation" {
			if err := s.contentSucceededConsumer.ConsumeSucceededRun(ctx, updated); err != nil {
				return updated, err
			}
		}
		if s.reviewSucceededConsumer != nil && updated.Stage == "review" {
			if err := s.reviewSucceededConsumer.ConsumeSucceededRun(ctx, updated); err != nil {
				return updated, err
			}
		}
		if s.rewriteSucceededConsumer != nil && updated.Stage == "rewrite" {
			if err := s.rewriteSucceededConsumer.ConsumeSucceededRun(ctx, updated); err != nil {
				return updated, err
			}
		}
		return updated, nil
	}
	if result.Status == ExecutionCancelled {
		next, err := run.Cancel(s.now())
		if err != nil {
			return WorkflowRun{}, err
		}
		event := Event{ID: s.newID(), RunID: run.ID, EventType: "cancelled", Status: StatusCancelled, Payload: executionEventPayload(result), CreatedAt: next.UpdatedAt}
		updated, _, err := s.store.UpdateStatusWithEvent(ctx, run, next, event)
		return updated, mapStoreError(err)
	}
	return s.failExecution(ctx, run, result.ErrorCode, result.ErrorMessage)
}

func (s *Service) failExecution(ctx context.Context, run WorkflowRun, code, message string) (WorkflowRun, error) {
	next, err := run.Fail(s.now(), Failure{Code: safeExecutionCode(code), Message: safeExecutionMessage(message), Details: json.RawMessage(`{}`)})
	if err != nil {
		return WorkflowRun{}, err
	}
	event := Event{ID: s.newID(), RunID: run.ID, EventType: "failed", Status: StatusFailed, Payload: json.RawMessage(`{}`), CreatedAt: next.UpdatedAt}
	updated, _, err := s.store.UpdateStatusWithEvent(ctx, run, next, event)
	return updated, mapStoreError(err)
}

func (s *Service) timeoutExecution(ctx context.Context, run WorkflowRun) (WorkflowRun, error) {
	next, err := run.Timeout(s.now(), Failure{Code: "upstream_timeout", Message: "workflow execution timed out"})
	if err != nil {
		return WorkflowRun{}, err
	}
	updated, _, err := s.store.UpdateStatusWithEvent(ctx, run, next, Event{ID: s.newID(), RunID: run.ID, EventType: "timed_out", Status: StatusTimedOut, Payload: json.RawMessage(`{}`), CreatedAt: next.UpdatedAt})
	return updated, mapStoreError(err)
}

func executionRequest(run WorkflowRun) (ExecutionRequest, error) {
	var snapshot struct {
		WorkflowConnection struct {
			ID uuid.UUID `json:"id"`
		} `json:"workflowConnection"`
		WorkflowConfiguration struct {
			DefaultParameters json.RawMessage `json:"defaultParameters"`
		} `json:"workflowConfiguration"`
	}
	if json.Unmarshal(run.ConfigurationSnapshot, &snapshot) != nil || snapshot.WorkflowConnection.ID == uuid.Nil {
		return ExecutionRequest{}, ErrValidation
	}
	request := ExecutionRequest{RunID: run.ID, ProjectID: run.ProjectID, Stage: run.Stage, WorkflowConfigurationID: run.WorkflowConfigurationID, WorkflowConnectionID: snapshot.WorkflowConnection.ID, ConfigurationSnapshot: RedactJSON(run.ConfigurationSnapshot), Input: RedactJSON(run.InputPayload), Parameters: RedactJSON(snapshot.WorkflowConfiguration.DefaultParameters), Metadata: map[string]string{}, CorrelationID: run.ID.String()}
	if run.ExternalExecutionID != nil {
		request.ExternalExecutionID = *run.ExternalExecutionID
	}
	return request, nil
}
func executionEventPayload(result ExecutionResult) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"externalExecutionId": result.ExternalExecutionID, "metadata": result.Metadata})
	return RedactJSON(b)
}
func executionErrorCode(err error) string {
	if errors.Is(err, ErrExecutorUnavailable) {
		return "executor_unavailable"
	}
	if errors.Is(err, ErrExecutionTimeout) {
		return "timeout"
	}
	return "execution_failed"
}
func safeExecutionCode(code string) string {
	if strings.TrimSpace(code) == "" {
		return "execution_failed"
	}
	return strings.TrimSpace(code)
}
func safeExecutionMessage(message string) string {
	if strings.TrimSpace(message) == "" {
		return "workflow execution failed"
	}
	return "workflow execution failed"
}

func (s *Service) CreateRun(ctx context.Context, command CreateRunCommand) (WorkflowRun, error) {
	if command.ProjectID == uuid.Nil || !validJSONObject(command.InputPayload) || strings.TrimSpace(command.IdempotencyKey) == "" {
		return WorkflowRun{}, ErrValidation
	}
	if command.TriggerSource == "" {
		command.TriggerSource = "manual"
	}
	if !validTriggerSource(command.TriggerSource) {
		return WorkflowRun{}, ErrValidation
	}
	if protectedStage(command.Stage) {
		return WorkflowRun{}, ErrProtectedStage
	}
	scope, fingerprint := commandScope("createWorkflowRun", command.ProjectID.String(), struct {
		ProjectID uuid.UUID
		Stage     string
		Input     json.RawMessage
		Trigger   string
	}{command.ProjectID, command.Stage, canonicalJSON(command.InputPayload), command.TriggerSource})
	return s.store.ExecuteIdempotent(ctx, scope, command.IdempotencyKey, fingerprint, func(store Store) (WorkflowRun, error) {
		return s.createRun(ctx, store, command)
	})
}

// CreateRunIdempotent establishes the replay boundary before callers rebuild
// mutable business input. Once the first Run exists, a replay must not fail a
// new active-run check.
func (s *Service) CreateRunIdempotent(ctx context.Context, projectID uuid.UUID, key, requestHash string, prepare CreateRunPreparation) (WorkflowRun, error) {
	return s.CreateRunIdempotentForScope(ctx, "createChapterPlanRun", projectID, key, requestHash, prepare)
}
func (s *Service) CreateRunIdempotentForScope(ctx context.Context, operation string, projectID uuid.UUID, key, requestHash string, prepare CreateRunPreparation) (WorkflowRun, error) {
	if projectID == uuid.Nil || strings.TrimSpace(key) == "" || strings.TrimSpace(requestHash) == "" || prepare == nil {
		return WorkflowRun{}, ErrValidation
	}
	scope := operation + ":" + projectID.String()
	return s.store.ExecuteIdempotent(ctx, scope, key, requestHash, func(store Store) (WorkflowRun, error) {
		command, err := prepare()
		if err != nil {
			return WorkflowRun{}, err
		}
		if command.ProjectID != projectID {
			return WorkflowRun{}, ErrValidation
		}
		if command.TriggerSource == "" {
			command.TriggerSource = "manual"
		}
		return s.createRun(ctx, store, command)
	})
}
func (s *Service) CreateRunForPreflightToken(ctx context.Context, projectID uuid.UUID, nonce, requestHash string, prepare CreateRunPreparation) (WorkflowRun, error) {
	if projectID == uuid.Nil || strings.TrimSpace(nonce) == "" || strings.TrimSpace(requestHash) == "" || prepare == nil {
		return WorkflowRun{}, ErrValidation
	}
	scope := "consumeContentGenerationPreflightToken:" + projectID.String()
	return s.store.ExecuteIdempotent(ctx, scope, nonce, requestHash, func(store Store) (WorkflowRun, error) {
		used, err := store.PreflightTokenUsed(ctx, nonce)
		if err != nil {
			return WorkflowRun{}, err
		}
		if used {
			return WorkflowRun{}, ErrPreflightTokenConsumed
		}
		command, err := prepare()
		if err != nil {
			return WorkflowRun{}, err
		}
		if command.ProjectID != projectID {
			return WorkflowRun{}, ErrValidation
		}
		if command.TriggerSource == "" {
			command.TriggerSource = "manual"
		}
		return s.createRun(ctx, store, command)
	})
}

func (s *Service) CreateRunForPreflightTokenIdempotent(ctx context.Context, projectID uuid.UUID, key, requestHash, nonce string, prepare CreateRunTxPreparation) (WorkflowRun, error) {
	return s.CreateRunForPreflightTokenIdempotentForScope(ctx, "createContentGenerationRun", projectID, key, requestHash, nonce, prepare)
}

func (s *Service) CreateRunForPreflightTokenIdempotentForScope(ctx context.Context, operation string, projectID uuid.UUID, key, requestHash, nonce string, prepare CreateRunTxPreparation) (WorkflowRun, error) {
	run, _, err := s.CreateRunForPreflightTokenIdempotentForScopeWithReplay(ctx, operation, projectID, key, requestHash, nonce, prepare)
	return run, err
}

func (s *Service) CreateRunForPreflightTokenIdempotentForScopeWithReplay(ctx context.Context, operation string, projectID uuid.UUID, key, requestHash, nonce string, prepare CreateRunTxPreparation) (WorkflowRun, bool, error) {
	if projectID == uuid.Nil || strings.TrimSpace(key) == "" || strings.TrimSpace(requestHash) == "" || strings.TrimSpace(nonce) == "" || prepare == nil {
		return WorkflowRun{}, false, ErrValidation
	}
	if operation != "createContentGenerationRun" && operation != "createContentReviewRun" && operation != "createContentRewriteRun" {
		return WorkflowRun{}, false, ErrValidation
	}
	scope := operation + ":" + projectID.String()
	return s.store.ExecuteIdempotentWithReplay(ctx, scope, key, requestHash, func(store Store) (WorkflowRun, error) {
		transactional, ok := store.(interface{ Transaction() pgx.Tx })
		if !ok || transactional.Transaction() == nil {
			return WorkflowRun{}, ErrValidation
		}
		if _, err := transactional.Transaction().Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "preflight-token:"+nonce); err != nil {
			return WorkflowRun{}, err
		}
		if operation == "createContentReviewRun" || operation == "createContentRewriteRun" {
			consumeScope := "consumeContentReviewPreflightToken:" + projectID.String()
			if operation == "createContentRewriteRun" {
				consumeScope = "consumeContentRewritePreflightToken:" + projectID.String()
			}
			var marker int
			markerErr := transactional.Transaction().QueryRow(ctx, "SELECT 1 FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2 FOR UPDATE", consumeScope, nonce).Scan(&marker)
			if markerErr == nil {
				return WorkflowRun{}, ErrPreflightTokenConsumed
			}
			if !errors.Is(markerErr, pgx.ErrNoRows) {
				return WorkflowRun{}, markerErr
			}
		} else {
			used, err := store.PreflightTokenUsed(ctx, nonce)
			if err != nil {
				return WorkflowRun{}, err
			}
			if used {
				return WorkflowRun{}, ErrPreflightTokenConsumed
			}
		}
		command, err := prepare(transactional.Transaction())
		if err != nil {
			return WorkflowRun{}, err
		}
		if command.ProjectID != projectID {
			return WorkflowRun{}, ErrValidation
		}
		if command.TriggerSource == "" {
			command.TriggerSource = "manual"
		}
		created, err := s.createRun(ctx, store, command)
		if err != nil {
			return WorkflowRun{}, err
		}
		if operation == "createContentReviewRun" || operation == "createContentRewriteRun" {
			body, marshalErr := json.Marshal(struct {
				RunID uuid.UUID `json:"runId"`
			}{created.ID})
			if marshalErr != nil {
				return WorkflowRun{}, marshalErr
			}
			consumeScope := "consumeContentReviewPreflightToken:" + projectID.String()
			if operation == "createContentRewriteRun" {
				consumeScope = "consumeContentRewritePreflightToken:" + projectID.String()
			}
			_, err = transactional.Transaction().Exec(ctx, "INSERT INTO idempotency_records(id,scope,idempotency_key,request_hash,response_status,response_body) VALUES($1,$2,$3,$4,201,$5)", uuid.New(), consumeScope, nonce, requestHash, body)
			if err != nil {
				return WorkflowRun{}, err
			}
		}
		return created, nil
	})
}

func (s *Service) createRun(ctx context.Context, store Store, command CreateRunCommand) (WorkflowRun, error) {
	if command.ProjectID == uuid.Nil || !validJSONObject(command.InputPayload) || !validTriggerSource(command.TriggerSource) {
		return WorkflowRun{}, ErrValidation
	}
	stage, err := workflowbinding.ParseStage(command.Stage)
	if err != nil {
		return WorkflowRun{}, ErrValidation
	}
	var configurationID uuid.UUID
	var snapshot json.RawMessage
	var binding workflowbinding.ProjectWorkflowBinding
	var configuration globalconfig.Workflow
	var connection globalconfig.Connection
	prepared := command.PreparedConfiguration != nil
	if command.PreparedConfiguration != nil {
		if (stage != workflowbinding.StageContentGeneration && stage != workflowbinding.StageReview && stage != workflowbinding.StageRewrite) || command.PreparedConfiguration.WorkflowConfigurationID == uuid.Nil || !validJSONObject(command.PreparedConfiguration.Snapshot) {
			return WorkflowRun{}, ErrValidation
		}
		configurationID = command.PreparedConfiguration.WorkflowConfigurationID
		snapshot = RedactJSON(command.PreparedConfiguration.Snapshot)
	} else {
		if _, err = s.projects.Get(ctx, command.ProjectID); err != nil {
			return WorkflowRun{}, mapProjectError(err)
		}
		var bindingErr error
		binding, bindingErr = s.bindings.GetByProjectAndStage(ctx, command.ProjectID, stage)
		if bindingErr != nil {
			return WorkflowRun{}, mapBindingError(bindingErr)
		}
		var configurationErr error
		configuration, connection, configurationErr = s.runnableConfiguration(ctx, binding.WorkflowConfigurationID, stage)
		if configurationErr != nil {
			return WorkflowRun{}, configurationErr
		}
		snapshot, err = configurationSnapshot(binding, configuration, connection, s.now())
		if err != nil {
			return WorkflowRun{}, fmt.Errorf("build workflow run snapshot: %w", err)
		}
		configurationID = configuration.ID
	}
	runID := command.RunID
	if runID == uuid.Nil {
		runID = s.newID()
	}
	run, err := New(runID, command.ProjectID, configurationID, s.newRunNumber(), stage.String(), command.TriggerSource, snapshot, command.InputPayload)
	if err != nil {
		return WorkflowRun{}, err
	}
	run.SubjectType, run.SubjectID = command.SubjectType, command.SubjectID
	if prepared {
		binding, _ = s.bindings.GetByProjectAndStage(ctx, command.ProjectID, stage)
		configuration, _ = s.configurations.GetWorkflow(ctx, configurationID)
		connection, _ = s.connections.GetConnection(ctx, configuration.ConnectionID)
	}
	s.populateRunSnapshots(ctx, &run, binding, configuration, connection)
	if _, err = NewFromDB(run); err != nil {
		return WorkflowRun{}, err
	}
	now := s.now()
	run.CreatedAt, run.UpdatedAt = now, now
	created, _, err := store.CreateWithInitialEvent(ctx, run, Event{ID: s.newID(), RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: now})
	return created, mapStoreError(err)
}

func (s *Service) ListRuns(ctx context.Context, query ListRunsQuery) (RunList, error) {
	f := query.ListFilter
	if !validListFilter(f) {
		return RunList{}, ErrValidation
	}
	if f.Limit == 0 {
		f.Limit = 50
	}
	items, err := s.store.List(ctx, f)
	if err != nil {
		return RunList{}, mapStoreError(err)
	}
	total, err := s.store.Count(ctx, f)
	if err != nil {
		return RunList{}, mapStoreError(err)
	}
	return RunList{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *Service) GetRun(ctx context.Context, id uuid.UUID) (WorkflowRun, error) {
	if id == uuid.Nil {
		return WorkflowRun{}, ErrValidation
	}
	run, err := s.store.GetByID(ctx, id)
	return run, mapStoreError(err)
}

func (s *Service) ListRunEvents(ctx context.Context, id uuid.UUID) ([]Event, error) {
	if _, err := s.GetRun(ctx, id); err != nil {
		return nil, err
	}
	events, err := s.store.ListEvents(ctx, id)
	return events, mapStoreError(err)
}
func (s *Service) AddEvent(ctx context.Context, event Event) (Event, error) {
	created, err := s.store.AddEvent(ctx, event)
	return created, mapStoreError(err)
}

func (s *Service) CancelRun(ctx context.Context, command RunCommand) (WorkflowRun, error) {
	if command.RunID == uuid.Nil || command.ExpectedVersion < 1 || strings.TrimSpace(command.IdempotencyKey) == "" {
		return WorkflowRun{}, ErrValidation
	}
	scope, fingerprint := commandScope("cancelWorkflowRun", command.RunID.String(), struct {
		ID      uuid.UUID
		Version int
	}{command.RunID, command.ExpectedVersion})
	return s.store.ExecuteIdempotent(ctx, scope, command.IdempotencyKey, fingerprint, func(store Store) (WorkflowRun, error) {
		current, err := store.GetByID(ctx, command.RunID)
		if err != nil {
			return WorkflowRun{}, mapStoreError(err)
		}
		if current.Version != command.ExpectedVersion {
			return WorkflowRun{}, ErrVersionConflict
		}
		if current.Status == StatusCancelling || current.Status == StatusCancelled {
			return current, nil
		}
		next, err := current.RequestCancellation(s.now())
		if errors.Is(err, ErrInvalidTransition) {
			return WorkflowRun{}, ErrNotCancellable
		}
		if err != nil {
			return WorkflowRun{}, err
		}
		updated, _, err := store.UpdateStatusWithEvent(ctx, current, next, Event{ID: s.newID(), RunID: next.ID, EventType: "cancel_requested", Status: StatusCancelling, Payload: json.RawMessage(`{}`), CreatedAt: next.UpdatedAt})
		return updated, mapStoreError(err)
	})
}

func (s *Service) GetRetryOptions(ctx context.Context, runID uuid.UUID) (RetryOptions, error) {
	if runID == uuid.Nil {
		return RetryOptions{}, ErrValidation
	}
	run, err := s.store.GetByID(ctx, runID)
	if err != nil {
		return RetryOptions{}, mapStoreError(err)
	}
	return s.retryOptionsForStore(ctx, s.store, run)
}

func disabledRetryOption(mode, code, message string) RetryOption {
	return RetryOption{Mode: mode, Reasons: []RetryReason{{Code: code, Message: message}}}
}

func (s *Service) retryOptionsForStore(ctx context.Context, store Store, run WorkflowRun) (RetryOptions, error) {
	result := RetryOptions{RunID: run.ID, Retryability: "not_retryable", CurrentConfiguration: disabledRetryOption("current_configuration", "workflow_run_not_retryable", "当前运行不可创建新的运行时重试。"), OriginalConfiguration: disabledRetryOption("original_configuration", "workflow_run_not_retryable", "当前运行不可创建新的运行时重试。")}
	events, err := store.ListEvents(ctx, run.ID)
	if err != nil {
		return RetryOptions{}, mapStoreError(err)
	}
	outputValidationFailed, resultConsumptionFailed, resultConsumed := false, false, false
	for _, event := range events {
		switch event.EventType {
		case EventTypeOutputValidationFailed:
			outputValidationFailed = true
		case EventTypeResultConsumptionFailed:
			resultConsumptionFailed = true
		case EventTypeResultConsumed:
			resultConsumed = true
		}
	}
	hasResult := false
	if run.Stage == "content_generation" {
		if candidateStore, ok := store.(interface {
			HasContentGenerationCandidate(context.Context, uuid.UUID) (bool, error)
		}); ok {
			hasResult, err = candidateStore.HasContentGenerationCandidate(ctx, run.ID)
		}
	} else if run.Stage == "review" {
		if reportStore, ok := store.(interface {
			HasReviewReport(context.Context, uuid.UUID) (bool, error)
		}); ok {
			hasResult, err = reportStore.HasReviewReport(ctx, run.ID)
		}
	} else if run.Stage == "rewrite" {
		if candidateStore, ok := store.(interface {
			HasRewriteCandidate(context.Context, uuid.UUID) (bool, error)
		}); ok {
			hasResult, err = candidateStore.HasRewriteCandidate(ctx, run.ID)
		}
	}
	if err != nil {
		return RetryOptions{}, err
	}
	if resultConsumptionFailed || run.Retryability == "result_consumption_retry" {
		result.Retryability, result.ResultConsumptionRetryRequired = "result_consumption_retry", true
		result.CurrentConfiguration = disabledRetryOption("current_configuration", "result_consumption_retry_required", "请使用当前业务阶段的结果消费重试。")
		result.OriginalConfiguration = disabledRetryOption("original_configuration", "result_consumption_retry_required", "请使用当前业务阶段的结果消费重试。")
		return result, nil
	}
	eligibleState := run.Status == StatusFailed || run.Status == StatusCancelled || run.Status == StatusTimedOut || run.Status == StatusSucceeded && outputValidationFailed
	if !eligibleState || resultConsumed || hasResult {
		return result, nil
	}
	result.Retryability = "runtime_retry"
	stage, parseErr := workflowbinding.ParseStage(run.Stage)
	if parseErr == nil {
		binding, bindErr := s.bindings.GetByProjectAndStage(ctx, run.ProjectID, stage)
		if bindErr == nil {
			configuration, connection, configErr := s.runnableConfiguration(ctx, binding.WorkflowConfigurationID, stage)
			if configErr == nil && configuration.Executable && connection.Executable {
				result.CurrentConfiguration = RetryOption{Mode: "current_configuration", Enabled: true, Reasons: []RetryReason{}}
			} else {
				result.CurrentConfiguration = disabledRetryOption("current_configuration", "current_configuration_not_executable", "当前工作流配置或连接不可执行。")
			}
		} else {
			result.CurrentConfiguration = disabledRetryOption("current_configuration", "workflow_binding_not_found", "当前项目未保留可用的工作流绑定。")
		}
	}
	result.OriginalConfiguration = s.originalRetryOption(ctx, run)
	return result, nil
}

func (s *Service) originalRetryOption(ctx context.Context, run WorkflowRun) RetryOption {
	option := disabledRetryOption("original_configuration", "snapshot_incomplete", "原始运行快照不完整，无法安全重放。")
	var binding struct {
		BindingID      uuid.UUID `json:"bindingId"`
		BindingVersion int       `json:"bindingVersion"`
		Stage          string    `json:"stage"`
	}
	var connection struct {
		ID                    uuid.UUID `json:"id"`
		Version               int       `json:"version"`
		CredentialFingerprint *string   `json:"credentialFingerprint"`
	}
	var policy struct {
		Strategy          string     `json:"strategy"`
		ProviderID        *uuid.UUID `json:"providerId"`
		ProviderVersion   *int       `json:"providerVersion"`
		Model             *string    `json:"model"`
		SecretFingerprint *string    `json:"secretFingerprint"`
	}
	var configuration struct {
		WorkflowConfiguration struct {
			ID      uuid.UUID `json:"id"`
			Version int       `json:"version"`
		} `json:"workflowConfiguration"`
	}
	if !snapshotHasKeys(run.BindingSnapshot, "bindingId", "bindingVersion", "stage") || !snapshotHasKeys(run.ConnectionSnapshot, "id", "name", "version", "connectionType", "baseUrl", "authType", "credentialFingerprint") || !snapshotHasKeys(run.LlmPolicySnapshot, "strategy", "providerId", "providerName", "providerVersion", "model", "secretFingerprint") || json.Unmarshal(run.BindingSnapshot, &binding) != nil || json.Unmarshal(run.ConnectionSnapshot, &connection) != nil || json.Unmarshal(run.LlmPolicySnapshot, &policy) != nil || json.Unmarshal(run.ConfigurationSnapshot, &configuration) != nil || binding.BindingID == uuid.Nil || binding.BindingVersion < 1 || binding.Stage != run.Stage || connection.ID == uuid.Nil || connection.Version < 1 || configuration.WorkflowConfiguration.ID == uuid.Nil || configuration.WorkflowConfiguration.Version < 1 || strings.TrimSpace(policy.Strategy) == "" {
		return option
	}
	currentConfiguration, err := s.configurations.GetWorkflow(ctx, configuration.WorkflowConfiguration.ID)
	if err != nil {
		return disabledRetryOption("original_configuration", "workflow_configuration_not_found", "原工作流配置已不存在。")
	}
	currentConnection, err := s.connections.GetConnection(ctx, connection.ID)
	if err != nil {
		return disabledRetryOption("original_configuration", "workflow_connection_not_found", "原工作流连接已不存在。")
	}
	if !sameStringPointer(connection.CredentialFingerprint, currentConnection.CredentialFingerprint) {
		return disabledRetryOption("original_configuration", "credential_fingerprint_changed", "连接凭据已变化，原配置不可重放。")
	}
	if safeBaseURL(currentConnection.BaseURL) == "" || !contains(currentConfiguration.ApplicableStages, run.Stage) {
		return disabledRetryOption("original_configuration", "snapshot_security_validation_failed", "原配置未通过当前安全校验。")
	}
	if policy.Strategy == "acf_managed" {
		reader, ok := s.configurations.(interface {
			GetProvider(context.Context, uuid.UUID) (globalconfig.Provider, error)
		})
		if !ok || policy.ProviderID == nil {
			return option
		}
		provider, providerErr := reader.GetProvider(ctx, *policy.ProviderID)
		if providerErr != nil {
			return disabledRetryOption("original_configuration", "llm_provider_not_found", "原 LLM Provider 已不存在。")
		}
		if !sameStringPointer(policy.SecretFingerprint, provider.SecretFingerprint) {
			return disabledRetryOption("original_configuration", "secret_fingerprint_changed", "LLM 密钥已变化，原配置不可重放。")
		}
	}
	option.Enabled, option.Reasons = true, []RetryReason{}
	option.ConfigurationDifferences = []ConfigurationDifference{
		{Field: "workflow_configuration", Changed: currentConfiguration.Version != configuration.WorkflowConfiguration.Version, Summary: "工作流配置版本比较"},
		{Field: "connection", Changed: currentConnection.Version != connection.Version, Summary: "连接版本比较"},
	}
	return option
}

func snapshotHasKeys(raw json.RawMessage, keys ...string) bool {
	var value map[string]any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	for _, key := range keys {
		if _, ok := value[key]; !ok {
			return false
		}
	}
	return true
}

func sameStringPointer(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (s *Service) RetryRun(ctx context.Context, command RetryCommand) (WorkflowRun, error) {
	run, _, err := s.RetryRunWithReplay(ctx, command)
	return run, err
}

func (s *Service) RetryRunWithReplay(ctx context.Context, command RetryCommand) (WorkflowRun, bool, error) {
	mode := strings.TrimSpace(command.Mode)
	if command.RunID == uuid.Nil || command.ExpectedVersion < 1 || strings.TrimSpace(command.IdempotencyKey) == "" || (mode != "current_configuration" && mode != "original_configuration") {
		return WorkflowRun{}, false, ErrValidation
	}
	reason := ""
	if command.Reason != nil {
		reason = strings.TrimSpace(*command.Reason)
		if utf8.RuneCountInString(reason) > 500 {
			return WorkflowRun{}, false, ErrValidation
		}
	}
	scope, fingerprint := commandScope("retryWorkflowRun", command.RunID.String(), struct {
		ID      uuid.UUID       `json:"runId"`
		Version int             `json:"expectedVersion"`
		Mode    string          `json:"mode"`
		Reason  string          `json:"reason"`
		Input   json.RawMessage `json:"inputOverride"`
	}{command.RunID, command.ExpectedVersion, mode, reason, canonicalJSON(command.InputOverride)})
	run, replay, executeErr := s.store.ExecuteIdempotentWithReplay(ctx, scope, command.IdempotencyKey, fingerprint, func(store Store) (WorkflowRun, error) {
		discovered, err := store.GetByID(ctx, command.RunID)
		if err != nil {
			return WorkflowRun{}, mapStoreError(err)
		}
		var original WorkflowRun
		if discovered.Stage == "rewrite" {
			rewriteStore, ok := store.(interface {
				LockRewriteRetryScope(context.Context, WorkflowRun) error
				GetByIDForUpdate(context.Context, uuid.UUID) (WorkflowRun, error)
			})
			if !ok {
				return WorkflowRun{}, ErrNotRetryable
			}
			if err = rewriteStore.LockRewriteRetryScope(ctx, discovered); err != nil {
				return WorkflowRun{}, err
			}
			original, err = rewriteStore.GetByIDForUpdate(ctx, command.RunID)
		} else if lockingStore, ok := store.(interface {
			GetByIDForUpdate(context.Context, uuid.UUID) (WorkflowRun, error)
		}); ok {
			original, err = lockingStore.GetByIDForUpdate(ctx, command.RunID)
		} else {
			original = discovered
		}
		if err != nil {
			return WorkflowRun{}, mapStoreError(err)
		}
		if original.Version != command.ExpectedVersion {
			return WorkflowRun{}, ErrVersionConflict
		}
		if (original.Stage == "content_generation" || original.Stage == "review" || original.Stage == "rewrite") && command.InputOverride != nil {
			return WorkflowRun{}, ErrValidation
		}
		options, optionErr := s.retryOptionsForStore(ctx, store, original)
		if optionErr != nil {
			return WorkflowRun{}, optionErr
		}
		selected := options.OriginalConfiguration
		if mode == "current_configuration" {
			selected = options.CurrentConfiguration
		}
		if !selected.Enabled {
			return WorkflowRun{}, ErrNotRetryable
		}
		if original.Stage == "rewrite" {
			rewriteStore, ok := store.(interface {
				ValidateRewriteRetryRelations(context.Context, WorkflowRun) error
			})
			if !ok {
				return WorkflowRun{}, ErrNotRetryable
			}
			if validationErr := rewriteStore.ValidateRewriteRetryRelations(ctx, original); validationErr != nil {
				return WorkflowRun{}, validationErr
			}
		}
		input := original.InputPayload
		if command.InputOverride != nil {
			if !validJSONObject(command.InputOverride) {
				return WorkflowRun{}, ErrValidation
			}
			input = command.InputOverride
		}
		snapshot, configurationID := original.ConfigurationSnapshot, original.WorkflowConfigurationID
		if mode == "current_configuration" {
			stage, parseErr := workflowbinding.ParseStage(original.Stage)
			if parseErr != nil {
				return WorkflowRun{}, ErrValidation
			}
			binding, bindErr := s.bindings.GetByProjectAndStage(ctx, original.ProjectID, stage)
			if bindErr != nil {
				return WorkflowRun{}, mapBindingError(bindErr)
			}
			configuration, connection, configErr := s.runnableConfiguration(ctx, binding.WorkflowConfigurationID, stage)
			if configErr != nil {
				return WorkflowRun{}, configErr
			}
			snapshot, configErr = configurationSnapshot(binding, configuration, connection, s.now())
			if configErr != nil {
				return WorkflowRun{}, fmt.Errorf("build workflow run snapshot: %w", configErr)
			}
			configurationID = configuration.ID
			// Snapshot projections always describe the configuration actually chosen for the retry.
			_ = binding
		}
		run, err := New(s.newID(), original.ProjectID, configurationID, s.newRunNumber(), original.Stage, "retry", snapshot, input)
		if err != nil {
			return WorkflowRun{}, err
		}
		now := s.now()
		run.SubjectType, run.SubjectID = original.SubjectType, original.SubjectID
		run.Retryability = "not_retryable"
		retryMode := mode
		run.RetryMode = &retryMode
		if mode == "current_configuration" {
			stage, _ := workflowbinding.ParseStage(original.Stage)
			binding, _ := s.bindings.GetByProjectAndStage(ctx, original.ProjectID, stage)
			configuration, _ := s.configurations.GetWorkflow(ctx, configurationID)
			connection, _ := s.connections.GetConnection(ctx, configuration.ConnectionID)
			s.populateRunSnapshots(ctx, &run, binding, configuration, connection)
		} else {
			run.BindingSnapshot = RedactJSON(original.BindingSnapshot)
			run.ConnectionSnapshot = RedactJSON(original.ConnectionSnapshot)
			run.LlmPolicySnapshot = RedactJSON(original.LlmPolicySnapshot)
		}
		if original.Stage == "rewrite" {
			input, err = rewriteRetryPayload(input, run.ID)
			if err != nil {
				return WorkflowRun{}, ErrNotRetryable
			}
			run.InputPayload = input
		}
		run.CreatedAt, run.UpdatedAt, run.RetryOfRunID = now, now, &original.ID
		created, _, err := store.CreateWithInitialEvent(ctx, run, Event{ID: s.newID(), RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: now})
		return created, mapStoreError(err)
	})
	if errors.Is(executeErr, ErrVersionConflict) || errors.Is(executeErr, ErrIdempotencyConflict) {
		if original, readErr := s.store.GetByID(ctx, command.RunID); readErr == nil && original.Stage == "rewrite" {
			if errors.Is(executeErr, ErrVersionConflict) {
				return WorkflowRun{}, false, ErrRewriteVersionConflict
			}
			return WorkflowRun{}, false, ErrRewriteIdempotencyConflict
		}
	}
	return run, replay, executeErr
}

func rewriteRetryPayload(raw json.RawMessage, runID uuid.UUID) (json.RawMessage, error) {
	var input struct {
		SchemaVersion               string          `json:"schemaVersion"`
		WorkflowRunID               uuid.UUID       `json:"workflowRunId"`
		CorrelationID               string          `json:"correlationId"`
		ProjectID                   uuid.UUID       `json:"projectId"`
		ContentItemID               uuid.UUID       `json:"contentItemId"`
		SourceContentVersionID      uuid.UUID       `json:"sourceContentVersionId"`
		SourceContentVersionVersion int             `json:"sourceContentVersionVersion"`
		SourceContentHash           string          `json:"sourceContentHash"`
		SourceTitle                 string          `json:"sourceTitle"`
		SourceContent               string          `json:"sourceContent"`
		ReviewReportID              uuid.UUID       `json:"reviewReportId"`
		ReportSnapshot              json.RawMessage `json:"reportSnapshot"`
		SelectedIssues              json.RawMessage `json:"selectedIssues"`
		OptionalInstructions        json.RawMessage `json:"optionalInstructions"`
		RewriteOptions              json.RawMessage `json:"rewriteOptions"`
	}
	if json.Unmarshal(raw, &input) != nil || input.SchemaVersion != "rewrite.input.v1" ||
		input.ProjectID == uuid.Nil || input.ContentItemID == uuid.Nil ||
		input.SourceContentVersionID == uuid.Nil || input.ReviewReportID == uuid.Nil ||
		len(input.ReportSnapshot) == 0 || len(input.SelectedIssues) == 0 ||
		len(input.OptionalInstructions) == 0 || len(input.RewriteOptions) == 0 {
		return nil, ErrNotRetryable
	}
	input.WorkflowRunID = runID
	input.CorrelationID = runID.String()
	return json.Marshal(input)
}

func protectedStage(stage string) bool {
	return stage == "content_generation" || stage == "chapter_planning" || stage == "review" || stage == "rewrite"
}

func (s *Service) GetProjectRunSummary(ctx context.Context, projectID uuid.UUID) (Summary, error) {
	if projectID == uuid.Nil {
		return Summary{}, ErrValidation
	}
	if _, err := s.projects.Get(ctx, projectID); err != nil {
		return Summary{}, mapProjectError(err)
	}
	summary, err := s.store.QuerySummary(ctx, projectID, 5)
	return summary, mapStoreError(err)
}

func (s *Service) runnableConfiguration(ctx context.Context, id uuid.UUID, stage workflowbinding.WorkflowBindingStage) (globalconfig.Workflow, globalconfig.Connection, error) {
	configuration, err := s.configurations.GetWorkflow(ctx, id)
	if err != nil {
		return globalconfig.Workflow{}, globalconfig.Connection{}, mapConfigurationError(err)
	}
	connection, err := s.connections.GetConnection(ctx, configuration.ConnectionID)
	if err != nil {
		return globalconfig.Workflow{}, globalconfig.Connection{}, mapConnectionError(err)
	}
	if !contains(configuration.ApplicableStages, stage.String()) {
		return globalconfig.Workflow{}, globalconfig.Connection{}, ErrNotRunnable
	}
	return configuration, connection, nil
}

func configurationSnapshot(binding workflowbinding.ProjectWorkflowBinding, configuration globalconfig.Workflow, connection globalconfig.Connection, createdAt time.Time) (json.RawMessage, error) {
	v := map[string]any{"projectId": binding.ProjectID, "stage": binding.Stage.String(), "binding": map[string]any{"id": binding.ID, "version": binding.Version}, "workflowConfiguration": map[string]any{"id": configuration.ID, "name": configuration.Name, "version": configuration.Version, "typeConfig": configuration.TypeConfig, "inputContractVersion": configuration.InputContractVersion, "outputContractVersion": configuration.OutputContractVersion, "defaultParameters": configuration.DefaultParameters, "llmStrategy": configuration.LlmStrategy, "llmProviderId": configuration.LlmProviderID, "llmModel": configuration.LlmModel, "validationStatus": configuration.ValidationStatus, "enabled": configuration.Enabled, "executable": configuration.Executable}, "workflowConnection": map[string]any{"id": connection.ID, "name": connection.Name, "version": connection.Version, "type": connection.ConnectionType, "baseUrl": safeBaseURL(connection.BaseURL), "authType": connection.AuthType, "timeoutSeconds": connection.TimeoutSeconds, "credentialFingerprint": connection.CredentialFingerprint, "typeConfig": connection.TypeConfig, "validationStatus": connection.ValidationStatus, "enabled": connection.Enabled, "executable": connection.Executable}, "createdAt": createdAt.UTC()}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return RedactJSON(b), nil
}

func (s *Service) populateRunSnapshots(ctx context.Context, run *WorkflowRun, binding workflowbinding.ProjectWorkflowBinding, configuration globalconfig.Workflow, connection globalconfig.Connection) {
	if run == nil || binding.ID == uuid.Nil || configuration.ID == uuid.Nil || connection.ID == uuid.Nil {
		return
	}
	run.BindingSnapshot = mustSafeJSON(map[string]any{"bindingId": binding.ID, "bindingVersion": binding.Version, "stage": binding.Stage.String()})
	run.ConnectionSnapshot = mustSafeJSON(map[string]any{"id": connection.ID, "name": connection.Name, "version": connection.Version, "connectionType": connection.ConnectionType, "baseUrl": safeBaseURL(connection.BaseURL), "authType": connection.AuthType, "credentialFingerprint": connection.CredentialFingerprint})
	strategy := configuration.LlmStrategy
	if strategy == "" {
		strategy = "none"
	}
	policy := map[string]any{"strategy": strategy, "providerId": configuration.LlmProviderID, "providerName": nil, "providerVersion": nil, "model": configuration.LlmModel, "secretFingerprint": nil}
	if configuration.LlmStrategy == "acf_managed" && configuration.LlmProviderID != nil {
		if reader, ok := s.configurations.(interface {
			GetProvider(context.Context, uuid.UUID) (globalconfig.Provider, error)
		}); ok {
			if provider, err := reader.GetProvider(ctx, *configuration.LlmProviderID); err == nil {
				policy["providerName"], policy["providerVersion"], policy["secretFingerprint"] = provider.Name, provider.Version, provider.SecretFingerprint
			}
		}
	}
	run.LlmPolicySnapshot = mustSafeJSON(policy)
}

func mustSafeJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return RedactJSON(encoded)
}

func BuildConfigurationSnapshot(binding workflowbinding.ProjectWorkflowBinding, configuration globalconfig.Workflow, connection globalconfig.Connection, createdAt time.Time) (json.RawMessage, error) {
	return configurationSnapshot(binding, configuration, connection, createdAt)
}

func safeBaseURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func validListFilter(f ListFilter) bool {
	if f.ProjectID != nil && *f.ProjectID == uuid.Nil || f.Limit < 0 || f.Limit > 100 || f.Offset < 0 || f.ConfigurationVersion < 0 || len(f.RunNumber) > 80 || len(f.Query) > 160 || len(f.Model) > 200 || f.StartTime != nil && f.EndTime != nil && f.StartTime.After(*f.EndTime) {
		return false
	}
	if f.Stage != "" {
		if _, err := workflowbinding.ParseStage(f.Stage); err != nil {
			return false
		}
	}
	if f.Status != "" && f.Status != string(StatusQueued) && f.Status != string(StatusRunning) && f.Status != string(StatusCancelling) && f.Status != string(StatusSucceeded) && f.Status != string(StatusFailed) && f.Status != string(StatusCancelled) && f.Status != string(StatusTimedOut) {
		return false
	}
	if f.DisplayStatus != "" && !validDisplayStatus(f.DisplayStatus) {
		return false
	}
	if f.Retryability != "" && f.Retryability != "runtime_retry" && f.Retryability != "result_consumption_retry" && f.Retryability != "not_retryable" {
		return false
	}
	return f.TriggerSource == "" || validTriggerSource(f.TriggerSource)
}

func validDisplayStatus(value string) bool {
	return value == string(StatusQueued) || value == string(StatusRunning) || value == string(StatusCancelling) || value == string(StatusSucceeded) || value == string(StatusFailed) || value == string(StatusCancelled) || value == string(StatusTimedOut) || value == "output_validation_failed" || value == "result_consumption_failed"
}
func mapProjectError(err error) error {
	if errors.Is(err, project.ErrNotFound) {
		return ErrProjectNotFound
	}
	return err
}
func mapBindingError(err error) error {
	if errors.Is(err, workflowbinding.ErrNotFound) {
		return ErrBindingNotFound
	}
	return err
}
func mapConfigurationError(err error) error {
	if errors.Is(err, globalconfig.ErrNotFound) {
		return ErrConfigurationNotFound
	}
	return err
}
func mapConnectionError(err error) error {
	if errors.Is(err, globalconfig.ErrNotFound) {
		return ErrConnectionNotFound
	}
	return err
}
func mapStoreError(err error) error {
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	return err
}
func Fingerprint(command any) string {
	b, _ := json.Marshal(command)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func canonicalJSON(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return value
	}
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return value
	}
	return normalized
}
func commandScope(operation, target string, payload any) (string, string) {
	return "workflow-run:" + operation + ":system:" + target, Fingerprint(payload)
}

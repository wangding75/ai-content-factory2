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

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/jackc/pgx/v5"
)

var (
	ErrProjectNotFound       = errors.New("project not found")
	ErrBindingNotFound       = errors.New("workflow binding not found")
	ErrConfigurationNotFound = errors.New("workflow configuration not found")
	ErrConnectionNotFound    = errors.New("workflow connection not found")
	ErrNotRunnable           = errors.New("workflow is not runnable")
	ErrNotCancellable        = errors.New("workflow run is not cancellable")
	ErrNotRetryable          = errors.New("workflow run is not retryable")
	ErrActiveRewriteRun      = errors.New("active rewrite run conflict")
	ErrIdempotencyConflict   = errors.New("idempotency key reused with different payload")
	ErrRewriteVersionConflict = errors.New("rewrite workflow run version conflict")
	ErrRewriteIdempotencyConflict = errors.New("rewrite idempotency conflict")
	ErrProtectedStage        = errors.New("workflow stage requires its domain command")
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
	QuerySummary(context.Context, uuid.UUID, int) (Summary, error)
	Count(context.Context, ListFilter) (int, error)
	ExecuteIdempotent(context.Context, string, string, string, func(Store) (WorkflowRun, error)) (WorkflowRun, error)
	ExecuteIdempotentWithReplay(context.Context, string, string, string, func(Store) (WorkflowRun, error)) (WorkflowRun, bool, error)
	PreflightTokenUsed(context.Context, string) (bool, error)
}

type CreateRunCommand struct {
	ProjectID      uuid.UUID
	RunID          uuid.UUID
	Stage          string
	SubjectType    *string
	SubjectID      *uuid.UUID
	InputPayload   json.RawMessage
	TriggerSource  string
	IdempotencyKey string
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
	RunID                   uuid.UUID
	ExpectedVersion         int
	UseCurrentConfiguration bool
	InputOverride           json.RawMessage
	IdempotencyKey          string
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
	contentSucceededConsumer interface { ConsumeSucceededRun(context.Context, WorkflowRun) error }
	reviewSucceededConsumer interface { ConsumeSucceededRun(context.Context, WorkflowRun) error }
	rewriteSucceededConsumer interface { ConsumeSucceededRun(context.Context, WorkflowRun) error }
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
func (s *Service) SetContentSucceededConsumer(consumer interface { ConsumeSucceededRun(context.Context, WorkflowRun) error }) { s.contentSucceededConsumer = consumer }
func (s *Service) SetReviewSucceededConsumer(consumer interface { ConsumeSucceededRun(context.Context, WorkflowRun) error }) { s.reviewSucceededConsumer = consumer }
func (s *Service) SetRewriteSucceededConsumer(consumer interface { ConsumeSucceededRun(context.Context, WorkflowRun) error }) { s.rewriteSucceededConsumer = consumer }

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
		return s.failExecution(ctx, run, executionErrorCode(err), "workflow execution failed")
	}
	if !validExecutionResult(result) {
		return s.failExecution(ctx, run, "invalid_response", "workflow execution returned an invalid result")
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
		if s.contentSucceededConsumer != nil && updated.Stage == "content_generation" { if err := s.contentSucceededConsumer.ConsumeSucceededRun(ctx,updated); err != nil { return updated,err } }
		if s.reviewSucceededConsumer != nil && updated.Stage == "review" { if err := s.reviewSucceededConsumer.ConsumeSucceededRun(ctx,updated); err != nil { return updated,err } }
		if s.rewriteSucceededConsumer != nil && updated.Stage == "rewrite" { if err := s.rewriteSucceededConsumer.ConsumeSucceededRun(ctx,updated); err != nil { return updated,err } }
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
	return ExecutionRequest{RunID: run.ID, ProjectID: run.ProjectID, Stage: run.Stage, WorkflowConfigurationID: run.WorkflowConfigurationID, WorkflowConnectionID: snapshot.WorkflowConnection.ID, ConfigurationSnapshot: RedactJSON(run.ConfigurationSnapshot), Input: RedactJSON(run.InputPayload), Parameters: RedactJSON(snapshot.WorkflowConfiguration.DefaultParameters), Metadata: map[string]string{}, CorrelationID: run.ID.String()}, nil
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
	if projectID == uuid.Nil || strings.TrimSpace(key) == "" || strings.TrimSpace(requestHash) == "" || strings.TrimSpace(nonce) == "" || prepare == nil { return WorkflowRun{}, false, ErrValidation }
	if operation != "createContentGenerationRun" && operation != "createContentReviewRun" && operation != "createContentRewriteRun" { return WorkflowRun{}, false, ErrValidation }
	scope := operation + ":" + projectID.String()
	return s.store.ExecuteIdempotentWithReplay(ctx, scope, key, requestHash, func(store Store) (WorkflowRun, error) {
		transactional, ok := store.(interface{ Transaction() pgx.Tx })
		if !ok || transactional.Transaction() == nil { return WorkflowRun{}, ErrValidation }
		if _, err := transactional.Transaction().Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "preflight-token:"+nonce); err != nil { return WorkflowRun{}, err }
		if operation == "createContentReviewRun" || operation == "createContentRewriteRun" {
			consumeScope := "consumeContentReviewPreflightToken:" + projectID.String()
			if operation == "createContentRewriteRun" { consumeScope = "consumeContentRewritePreflightToken:" + projectID.String() }
			var marker int
			markerErr := transactional.Transaction().QueryRow(ctx, "SELECT 1 FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2 FOR UPDATE", consumeScope, nonce).Scan(&marker)
			if markerErr == nil { return WorkflowRun{}, ErrPreflightTokenConsumed }
			if !errors.Is(markerErr, pgx.ErrNoRows) { return WorkflowRun{}, markerErr }
		} else {
			used, err := store.PreflightTokenUsed(ctx, nonce)
			if err != nil { return WorkflowRun{}, err }
			if used { return WorkflowRun{}, ErrPreflightTokenConsumed }
		}
		command, err := prepare(transactional.Transaction())
		if err != nil { return WorkflowRun{}, err }
		if command.ProjectID != projectID { return WorkflowRun{}, ErrValidation }
		if command.TriggerSource == "" { command.TriggerSource = "manual" }
		created, err := s.createRun(ctx, store, command)
		if err != nil { return WorkflowRun{}, err }
		if operation == "createContentReviewRun" || operation == "createContentRewriteRun" {
			body, marshalErr := json.Marshal(struct {
				RunID uuid.UUID `json:"runId"`
			}{created.ID})
			if marshalErr != nil { return WorkflowRun{}, marshalErr }
			consumeScope := "consumeContentReviewPreflightToken:" + projectID.String()
			if operation == "createContentRewriteRun" { consumeScope = "consumeContentRewritePreflightToken:" + projectID.String() }
			_, err = transactional.Transaction().Exec(ctx, "INSERT INTO idempotency_records(id,scope,idempotency_key,request_hash,response_status,response_body) VALUES($1,$2,$3,$4,201,$5)", uuid.New(), consumeScope, nonce, requestHash, body)
			if err != nil { return WorkflowRun{}, err }
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
		binding, bindingErr := s.bindings.GetByProjectAndStage(ctx, command.ProjectID, stage)
		if bindingErr != nil {
			return WorkflowRun{}, mapBindingError(bindingErr)
		}
		configuration, connection, configurationErr := s.runnableConfiguration(ctx, binding.WorkflowConfigurationID, stage)
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
	if runID == uuid.Nil { runID = s.newID() }
	run, err := New(runID, command.ProjectID, configurationID, s.newRunNumber(), stage.String(), command.TriggerSource, snapshot, command.InputPayload)
	if err != nil {
		return WorkflowRun{}, err
	}
	run.SubjectType, run.SubjectID = command.SubjectType, command.SubjectID
	if _, err = NewFromDB(run); err != nil { return WorkflowRun{}, err }
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
func (s *Service) AddEvent(ctx context.Context, event Event) (Event, error) { created, err := s.store.AddEvent(ctx,event); return created,mapStoreError(err) }

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
		next, err := current.Cancel(s.now())
		if errors.Is(err, ErrInvalidTransition) {
			return WorkflowRun{}, ErrNotCancellable
		}
		if err != nil {
			return WorkflowRun{}, err
		}
		updated, _, err := store.UpdateStatusWithEvent(ctx, current, next, Event{ID: s.newID(), RunID: next.ID, EventType: "cancelled", Status: StatusCancelled, Payload: json.RawMessage(`{}`), CreatedAt: next.UpdatedAt})
		return updated, mapStoreError(err)
	})
}

func (s *Service) RetryRun(ctx context.Context, command RetryCommand) (WorkflowRun, error) {
	run, _, err := s.RetryRunWithReplay(ctx, command)
	return run, err
}

func (s *Service) RetryRunWithReplay(ctx context.Context, command RetryCommand) (WorkflowRun, bool, error) {
	if command.RunID == uuid.Nil || command.ExpectedVersion < 1 || strings.TrimSpace(command.IdempotencyKey) == "" {
		return WorkflowRun{}, false, ErrValidation
	}
	scope, fingerprint := commandScope("retryWorkflowRun", command.RunID.String(), struct {
		ID      uuid.UUID
		Version int
		Current bool
		Input   json.RawMessage
	}{command.RunID, command.ExpectedVersion, command.UseCurrentConfiguration, canonicalJSON(command.InputOverride)})
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
		if (original.Stage == "review" || original.Stage == "rewrite") && command.UseCurrentConfiguration {
			return WorkflowRun{}, ErrValidation
		}
		if original.Stage == "content_generation" || original.Stage == "review" || original.Stage == "rewrite" {
			events, eventErr := store.ListEvents(ctx, original.ID)
			if eventErr != nil {
				return WorkflowRun{}, mapStoreError(eventErr)
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
			if original.Stage == "content_generation" {
				candidateStore, ok := store.(interface {
					HasContentGenerationCandidate(context.Context, uuid.UUID) (bool, error)
				})
				if !ok { return WorkflowRun{}, ErrNotRetryable }
				hasResult, eventErr = candidateStore.HasContentGenerationCandidate(ctx, original.ID)
			} else if original.Stage == "review" {
				reportStore, ok := store.(interface {
					HasReviewReport(context.Context, uuid.UUID) (bool, error)
				})
				if ok { hasResult, eventErr = reportStore.HasReviewReport(ctx, original.ID) }
			} else {
				candidateStore, ok := store.(interface {
					HasRewriteCandidate(context.Context, uuid.UUID) (bool, error)
				})
				if !ok {
					return WorkflowRun{}, ErrNotRetryable
				}
				hasResult, eventErr = candidateStore.HasRewriteCandidate(ctx, original.ID)
			}
			if eventErr != nil { return WorkflowRun{}, eventErr }
			if resultConsumptionFailed || resultConsumed || hasResult { return WorkflowRun{}, ErrNotRetryable }
			if original.Status != StatusFailed && original.Status != StatusCancelled && (original.Status != StatusSucceeded || !outputValidationFailed || command.UseCurrentConfiguration) {
				return WorkflowRun{}, ErrNotRetryable
			}
			if original.Stage == "rewrite" {
				rewriteStore, ok := store.(interface {
					ValidateRewriteRetryRelations(context.Context, WorkflowRun) error
				})
				if !ok {
					return WorkflowRun{}, ErrNotRetryable
				}
				if eventErr = rewriteStore.ValidateRewriteRetryRelations(ctx, original); eventErr != nil {
					return WorkflowRun{}, eventErr
				}
			}
		} else if original.Status != StatusFailed && original.Status != StatusCancelled {
			return WorkflowRun{}, ErrNotRetryable
		}
		input := original.InputPayload
		if command.InputOverride != nil {
			if !validJSONObject(command.InputOverride) {
				return WorkflowRun{}, ErrValidation
			}
			input = command.InputOverride
		}
		snapshot, configurationID := original.ConfigurationSnapshot, original.WorkflowConfigurationID
		if command.UseCurrentConfiguration {
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
		}
		run, err := New(s.newID(), original.ProjectID, configurationID, s.newRunNumber(), original.Stage, "retry", snapshot, input)
		if err != nil {
			return WorkflowRun{}, err
		}
		now := s.now()
		run.SubjectType, run.SubjectID = original.SubjectType, original.SubjectID
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
	v := map[string]any{"projectId": binding.ProjectID, "stage": binding.Stage.String(), "binding": map[string]any{"id": binding.ID, "version": binding.Version}, "workflowConfiguration": map[string]any{"id": configuration.ID, "version": configuration.Version, "typeConfig": configuration.TypeConfig, "inputContractVersion": configuration.InputContractVersion, "outputContractVersion": configuration.OutputContractVersion, "defaultParameters": configuration.DefaultParameters}, "workflowConnection": map[string]any{"id": connection.ID, "version": connection.Version, "type": connection.ConnectionType, "baseUrl": safeBaseURL(connection.BaseURL), "timeoutSeconds": connection.TimeoutSeconds, "typeConfig": connection.TypeConfig}, "createdAt": createdAt.UTC()}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return RedactJSON(b), nil
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
	if f.ProjectID != nil && *f.ProjectID == uuid.Nil || f.Limit < 0 || f.Limit > 100 || f.Offset < 0 || len(f.RunNumber) > 80 || len(f.Query) > 160 || f.StartTime != nil && f.EndTime != nil && f.StartTime.After(*f.EndTime) {
		return false
	}
	if f.Stage != "" {
		if _, err := workflowbinding.ParseStage(f.Stage); err != nil {
			return false
		}
	}
	if f.Status != "" && f.Status != string(StatusQueued) && f.Status != string(StatusRunning) && f.Status != string(StatusSucceeded) && f.Status != string(StatusFailed) && f.Status != string(StatusCancelled) {
		return false
	}
	return f.TriggerSource == "" || validTriggerSource(f.TriggerSource)
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

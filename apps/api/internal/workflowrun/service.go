package workflowrun

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
)

var (
	ErrProjectNotFound = errors.New("project not found")
	ErrBindingNotFound = errors.New("workflow binding not found")
	ErrConfigurationNotFound = errors.New("workflow configuration not found")
	ErrConnectionNotFound = errors.New("workflow connection not found")
	ErrNotRunnable = errors.New("workflow is not runnable")
	ErrNotCancellable = errors.New("workflow run is not cancellable")
	ErrNotRetryable = errors.New("workflow run is not retryable")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different payload")
)

type ProjectReader interface { Get(context.Context, uuid.UUID) (project.Project, error) }
type BindingReader interface { GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error) }
type ConfigurationReader interface { GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error) }
type ConnectionReader interface { GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error) }
type Store interface {
	CreateWithInitialEvent(context.Context, WorkflowRun, Event) (WorkflowRun, Event, error)
	GetByID(context.Context, uuid.UUID) (WorkflowRun, error)
	List(context.Context, ListFilter) ([]WorkflowRun, error)
	ListEvents(context.Context, uuid.UUID) ([]Event, error)
	UpdateStatusWithEvent(context.Context, WorkflowRun, WorkflowRun, Event) (WorkflowRun, Event, error)
	QuerySummary(context.Context, uuid.UUID, int) (Summary, error)
	Count(context.Context, ListFilter) (int, error)
}

type CreateRunCommand struct {
	ProjectID uuid.UUID
	Stage string
	InputPayload json.RawMessage
	TriggerSource string
	IdempotencyKey string
}
type RunCommand struct { RunID uuid.UUID; ExpectedVersion int; IdempotencyKey string }
type RetryCommand struct { RunID uuid.UUID; ExpectedVersion int; UseCurrentConfiguration bool; InputOverride json.RawMessage; IdempotencyKey string }
type ListRunsQuery struct { ListFilter }
type RunList struct { Items []WorkflowRun; Total, Limit, Offset int }

type Service struct {
	store Store
	projects ProjectReader
	bindings BindingReader
	configurations ConfigurationReader
	connections ConnectionReader
	now func() time.Time
	newID func() uuid.UUID
	newRunNumber func() string
	idempotencyMu sync.Mutex
	idempotency map[string]idempotentRun
}
type idempotentRun struct { fingerprint string; run WorkflowRun }

func NewService(store Store, projects ProjectReader, bindings BindingReader, configurations ConfigurationReader, connections ConnectionReader) *Service {
	return &Service{store: store, projects: projects, bindings: bindings, configurations: configurations, connections: connections, now: func() time.Time { return time.Now().UTC() }, newID: uuid.New, newRunNumber: func() string { return "WR-" + strings.ToUpper(uuid.NewString()[:8]) }, idempotency: map[string]idempotentRun{}}
}

func (s *Service) CreateRun(ctx context.Context, command CreateRunCommand) (WorkflowRun, error) {
	if command.ProjectID == uuid.Nil || !validJSONObject(command.InputPayload) || strings.TrimSpace(command.IdempotencyKey) == "" { return WorkflowRun{}, ErrValidation }
	if command.TriggerSource == "" { command.TriggerSource = "manual" }
	if !validTriggerSource(command.TriggerSource) { return WorkflowRun{}, ErrValidation }
	scope, fingerprint := commandScope("createWorkflowRun", command.ProjectID.String()+":"+command.Stage, command.IdempotencyKey, struct { ProjectID uuid.UUID; Stage string; Input json.RawMessage; Trigger string }{command.ProjectID, command.Stage, command.InputPayload, command.TriggerSource})
	if replay, ok, err := s.replay(scope, fingerprint); ok || err != nil { return replay, err }
	stage, err := workflowbinding.ParseStage(command.Stage)
	if err != nil { return WorkflowRun{}, ErrValidation }
	if _, err = s.projects.Get(ctx, command.ProjectID); err != nil { return WorkflowRun{}, mapProjectError(err) }
	binding, err := s.bindings.GetByProjectAndStage(ctx, command.ProjectID, stage)
	if err != nil { return WorkflowRun{}, mapBindingError(err) }
	configuration, connection, err := s.runnableConfiguration(ctx, binding.WorkflowConfigurationID, stage)
	if err != nil { return WorkflowRun{}, err }
	snapshot, err := configurationSnapshot(binding, configuration, connection, s.now())
	if err != nil { return WorkflowRun{}, fmt.Errorf("build workflow run snapshot: %w", err) }
	run, err := New(s.newID(), command.ProjectID, configuration.ID, s.newRunNumber(), stage.String(), command.TriggerSource, snapshot, command.InputPayload)
	if err != nil { return WorkflowRun{}, err }
	now := s.now(); run.CreatedAt, run.UpdatedAt = now, now
	event := Event{ID: s.newID(), RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: now}
	created, _, err := s.store.CreateWithInitialEvent(ctx, run, event)
	if err != nil { return WorkflowRun{}, mapStoreError(err) }; s.saveReplay(scope, fingerprint, created); return created, nil
}

func (s *Service) ListRuns(ctx context.Context, query ListRunsQuery) (RunList, error) {
	f := query.ListFilter
	if !validListFilter(f) { return RunList{}, ErrValidation }
	if f.Limit == 0 { f.Limit = 50 }
	items, err := s.store.List(ctx, f); if err != nil { return RunList{}, mapStoreError(err) }
	total, err := s.store.Count(ctx, f); if err != nil { return RunList{}, mapStoreError(err) }
	return RunList{Items: items, Total: total, Limit: f.Limit, Offset: f.Offset}, nil
}

func (s *Service) GetRun(ctx context.Context, id uuid.UUID) (WorkflowRun, error) {
	if id == uuid.Nil { return WorkflowRun{}, ErrValidation }
	run, err := s.store.GetByID(ctx, id); return run, mapStoreError(err)
}

func (s *Service) ListRunEvents(ctx context.Context, id uuid.UUID) ([]Event, error) {
	if _, err := s.GetRun(ctx, id); err != nil { return nil, err }
	events, err := s.store.ListEvents(ctx, id); return events, mapStoreError(err)
}

func (s *Service) CancelRun(ctx context.Context, command RunCommand) (WorkflowRun, error) {
	if command.RunID == uuid.Nil || command.ExpectedVersion < 1 || strings.TrimSpace(command.IdempotencyKey) == "" { return WorkflowRun{}, ErrValidation }
	scope, fingerprint := commandScope("cancelWorkflowRun", command.RunID.String(), command.IdempotencyKey, struct { ID uuid.UUID; Version int }{command.RunID, command.ExpectedVersion})
	if replay, ok, err := s.replay(scope, fingerprint); ok || err != nil { return replay, err }
	current, err := s.GetRun(ctx, command.RunID); if err != nil { return WorkflowRun{}, err }
	if current.Version != command.ExpectedVersion { return WorkflowRun{}, ErrVersionConflict }
	next, err := current.Cancel(s.now()); if errors.Is(err, ErrInvalidTransition) { return WorkflowRun{}, ErrNotCancellable }; if err != nil { return WorkflowRun{}, err }
	event := Event{ID: s.newID(), RunID: next.ID, EventType: "cancelled", Status: StatusCancelled, Payload: json.RawMessage(`{}`), CreatedAt: next.UpdatedAt}
	updated, _, err := s.store.UpdateStatusWithEvent(ctx, current, next, event)
	if err != nil { return WorkflowRun{}, mapStoreError(err) }; s.saveReplay(scope, fingerprint, updated); return updated, nil
}

func (s *Service) RetryRun(ctx context.Context, command RetryCommand) (WorkflowRun, error) {
	if command.RunID == uuid.Nil || command.ExpectedVersion < 1 || strings.TrimSpace(command.IdempotencyKey) == "" { return WorkflowRun{}, ErrValidation }
	scope, fingerprint := commandScope("retryWorkflowRun", command.RunID.String(), command.IdempotencyKey, struct { ID uuid.UUID; Version int; Current bool; Input json.RawMessage }{command.RunID, command.ExpectedVersion, command.UseCurrentConfiguration, command.InputOverride})
	if replay, ok, err := s.replay(scope, fingerprint); ok || err != nil { return replay, err }
	original, err := s.GetRun(ctx, command.RunID); if err != nil { return WorkflowRun{}, err }
	if original.Version != command.ExpectedVersion { return WorkflowRun{}, ErrVersionConflict }
	if original.Status != StatusFailed && original.Status != StatusCancelled { return WorkflowRun{}, ErrNotRetryable }
	input := original.InputPayload
	if command.InputOverride != nil { if !validJSONObject(command.InputOverride) { return WorkflowRun{}, ErrValidation }; input = command.InputOverride }
	snapshot, configurationID := original.ConfigurationSnapshot, original.WorkflowConfigurationID
	if command.UseCurrentConfiguration {
		stage, parseErr := workflowbinding.ParseStage(original.Stage); if parseErr != nil { return WorkflowRun{}, ErrValidation }
		binding, bindErr := s.bindings.GetByProjectAndStage(ctx, original.ProjectID, stage); if bindErr != nil { return WorkflowRun{}, mapBindingError(bindErr) }
		configuration, connection, configErr := s.runnableConfiguration(ctx, binding.WorkflowConfigurationID, stage); if configErr != nil { return WorkflowRun{}, configErr }
		snapshot, configErr = configurationSnapshot(binding, configuration, connection, s.now()); if configErr != nil { return WorkflowRun{}, fmt.Errorf("build workflow run snapshot: %w", configErr) }
		configurationID = configuration.ID
	}
	run, err := New(s.newID(), original.ProjectID, configurationID, s.newRunNumber(), original.Stage, "retry", snapshot, input)
	if err != nil { return WorkflowRun{}, err }
	now := s.now(); run.CreatedAt, run.UpdatedAt, run.RetryOfRunID = now, now, &original.ID
	event := Event{ID: s.newID(), RunID: run.ID, EventType: "queued", Status: StatusQueued, Payload: json.RawMessage(`{}`), CreatedAt: now}
	created, _, err := s.store.CreateWithInitialEvent(ctx, run, event)
	if err != nil { return WorkflowRun{}, mapStoreError(err) }; s.saveReplay(scope, fingerprint, created); return created, nil
}

func (s *Service) GetProjectRunSummary(ctx context.Context, projectID uuid.UUID) (Summary, error) {
	if projectID == uuid.Nil { return Summary{}, ErrValidation }
	if _, err := s.projects.Get(ctx, projectID); err != nil { return Summary{}, mapProjectError(err) }
	summary, err := s.store.QuerySummary(ctx, projectID, 5); return summary, mapStoreError(err)
}

func (s *Service) runnableConfiguration(ctx context.Context, id uuid.UUID, stage workflowbinding.WorkflowBindingStage) (globalconfig.Workflow, globalconfig.Connection, error) {
	configuration, err := s.configurations.GetWorkflow(ctx, id)
	if err != nil { return globalconfig.Workflow{}, globalconfig.Connection{}, mapConfigurationError(err) }
	connection, err := s.connections.GetConnection(ctx, configuration.ConnectionID)
	if err != nil { return globalconfig.Workflow{}, globalconfig.Connection{}, mapConnectionError(err) }
	if !configuration.Enabled || configuration.IntegrationStatus != "verified" || !connection.Enabled || connection.IntegrationStatus != "verified" || !contains(configuration.ApplicableStages, stage.String()) { return globalconfig.Workflow{}, globalconfig.Connection{}, ErrNotRunnable }
	return configuration, connection, nil
}

func configurationSnapshot(binding workflowbinding.ProjectWorkflowBinding, configuration globalconfig.Workflow, connection globalconfig.Connection, createdAt time.Time) (json.RawMessage, error) {
	v := map[string]any{"projectId": binding.ProjectID, "stage": binding.Stage.String(), "binding": map[string]any{"id": binding.ID, "version": binding.Version}, "workflowConfiguration": map[string]any{"id": configuration.ID, "version": configuration.Version, "typeConfig": configuration.TypeConfig, "inputContractVersion": configuration.InputContractVersion, "outputContractVersion": configuration.OutputContractVersion, "defaultParameters": configuration.DefaultParameters}, "workflowConnection": map[string]any{"id": connection.ID, "type": connection.ConnectionType, "baseUrl": connection.BaseURL, "timeoutSeconds": connection.TimeoutSeconds, "typeConfig": connection.TypeConfig}, "createdAt": createdAt.UTC()}
	b, err := json.Marshal(v); if err != nil { return nil, err }; return RedactJSON(b), nil
}

func contains(values []string, value string) bool { for _, item := range values { if item == value { return true } }; return false }
func validTriggerSource(value string) bool { return value == "manual" || value == "retry" || value == "system" || value == "api" }
func validListFilter(f ListFilter) bool {
	if f.ProjectID != nil && *f.ProjectID == uuid.Nil || f.Limit < 0 || f.Limit > 100 || f.Offset < 0 || len(f.RunNumber) > 80 || len(f.Query) > 160 || f.StartTime != nil && f.EndTime != nil && f.StartTime.After(*f.EndTime) { return false }
	if f.Stage != "" { if _, err := workflowbinding.ParseStage(f.Stage); err != nil { return false } }
	if f.Status != "" && f.Status != string(StatusQueued) && f.Status != string(StatusRunning) && f.Status != string(StatusSucceeded) && f.Status != string(StatusFailed) && f.Status != string(StatusCancelled) { return false }
	return f.TriggerSource == "" || f.TriggerSource == "project" || f.TriggerSource == "workflow_center" || f.TriggerSource == "retry"
}
func mapProjectError(err error) error { if errors.Is(err, project.ErrNotFound) { return ErrProjectNotFound }; return err }
func mapBindingError(err error) error { if errors.Is(err, workflowbinding.ErrNotFound) { return ErrBindingNotFound }; return err }
func mapConfigurationError(err error) error { if errors.Is(err, globalconfig.ErrNotFound) { return ErrConfigurationNotFound }; return err }
func mapConnectionError(err error) error { if errors.Is(err, globalconfig.ErrNotFound) { return ErrConnectionNotFound }; return err }
func mapStoreError(err error) error { if errors.Is(err, ErrNotFound) { return ErrNotFound }; return err }
func Fingerprint(command any) string { b, _ := json.Marshal(command); sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func commandScope(operation, target, key string, payload any) (string, string) { keyHash:=Fingerprint(key); return operation+":"+target+":"+keyHash, Fingerprint(payload) }
func (s *Service) replay(scope, fingerprint string) (WorkflowRun, bool, error) { s.idempotencyMu.Lock(); defer s.idempotencyMu.Unlock(); entry, ok:=s.idempotency[scope]; if !ok{return WorkflowRun{},false,nil}; if entry.fingerprint!=fingerprint{return WorkflowRun{},true,ErrIdempotencyConflict};return entry.run,true,nil }
func (s *Service) saveReplay(scope, fingerprint string, run WorkflowRun) { s.idempotencyMu.Lock(); defer s.idempotencyMu.Unlock(); s.idempotency[scope]=idempotentRun{fingerprint,run} }

package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
)

type serviceStore struct {
	runs   map[uuid.UUID]WorkflowRun
	events map[uuid.UUID][]Event
	createErr error
}

func (s *serviceStore) ExecuteIdempotent(_ context.Context, _ string, _ string, _ string, fn func(Store) (WorkflowRun, error)) (WorkflowRun, error) {
	return fn(s)
}
func (s *serviceStore) PreflightTokenUsed(_ context.Context, nonce string) (bool, error) {
	for _, run := range s.runs {
		var input struct { PreflightTokenNonce string `json:"preflightTokenNonce"` }
		if json.Unmarshal(run.InputPayload, &input) == nil && input.PreflightTokenNonce == nonce {
			return true, nil
		}
	}
	return false, nil
}
func (s *serviceStore) CreateWithInitialEvent(_ context.Context, run WorkflowRun, event Event) (WorkflowRun, Event, error) {
	if s.createErr != nil { return WorkflowRun{}, Event{}, s.createErr }
	s.runs[run.ID] = run
	s.events[run.ID] = append(s.events[run.ID], event)
	return run, event, nil
}

func TestPreflightTokenCreateFailureDoesNotConsumeToken(t *testing.T) {
	s, store, projectID := fixtureService(t)
	nonce := uuid.NewString()
	prepare := func() (CreateRunCommand, error) { return CreateRunCommand{ProjectID: projectID, Stage: "review", TriggerSource: "manual", InputPayload: json.RawMessage(`{"preflightTokenNonce":"` + nonce + `"}`)}, nil }
	store.createErr = errors.New("transaction create failed")
	if _, err := s.CreateRunForPreflightToken(context.Background(), projectID, nonce, "first", prepare); err == nil { t.Fatal("expected create failure") }
	if used, err := store.PreflightTokenUsed(context.Background(), nonce); err != nil || used { t.Fatalf("used=%v err=%v", used, err) }
	store.createErr = nil
	if _, err := s.CreateRunForPreflightToken(context.Background(), projectID, nonce, "retry", prepare); err != nil { t.Fatalf("retry=%v", err) }
	if len(store.runs) != 1 || len(store.events) != 1 { t.Fatalf("partial state runs=%d events=%d", len(store.runs), len(store.events)) }
}
func (s *serviceStore) GetByID(_ context.Context, id uuid.UUID) (WorkflowRun, error) {
	r, ok := s.runs[id]
	if !ok {
		return WorkflowRun{}, ErrNotFound
	}
	return r, nil
}
func (s *serviceStore) List(_ context.Context, _ ListFilter) ([]WorkflowRun, error) {
	out := []WorkflowRun{}
	for _, r := range s.runs {
		out = append(out, r)
	}
	return out, nil
}
func (s *serviceStore) Count(_ context.Context, _ ListFilter) (int, error) { return len(s.runs), nil }
func (s *serviceStore) ListEvents(_ context.Context, id uuid.UUID) ([]Event, error) {
	return s.events[id], nil
}
func (s *serviceStore) AddEvent(_ context.Context, event Event) (Event, error) {
	s.events[event.RunID] = append(s.events[event.RunID], event)
	return event, nil
}
func (s *serviceStore) UpdateStatusWithEvent(_ context.Context, current, next WorkflowRun, event Event) (WorkflowRun, Event, error) {
	if s.runs[current.ID].Version != current.Version {
		return WorkflowRun{}, Event{}, ErrVersionConflict
	}
	s.runs[next.ID] = next
	s.events[next.ID] = append(s.events[next.ID], event)
	return next, event, nil
}
func (s *serviceStore) QuerySummary(_ context.Context, _ uuid.UUID, _ int) (Summary, error) {
	return Summary{}, nil
}

type serviceProjects struct {
	p   project.Project
	err error
}

func (s serviceProjects) Get(context.Context, uuid.UUID) (project.Project, error) { return s.p, s.err }

type serviceBindings struct {
	b   workflowbinding.ProjectWorkflowBinding
	err error
}

func (s serviceBindings) GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error) {
	return s.b, s.err
}

type serviceConfigs struct {
	w   globalconfig.Workflow
	err error
}

func (s serviceConfigs) GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error) {
	return s.w, s.err
}

type serviceConnections struct {
	c   globalconfig.Connection
	err error
}

func (s serviceConnections) GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error) {
	return s.c, s.err
}

func fixtureService(t *testing.T) (*Service, *serviceStore, uuid.UUID) {
	t.Helper()
	projectID, configID, connectionID, bindingID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	store := &serviceStore{runs: map[uuid.UUID]WorkflowRun{}, events: map[uuid.UUID][]Event{}}
	s := NewService(store, serviceProjects{p: project.Project{ID: projectID}}, serviceBindings{b: workflowbinding.ProjectWorkflowBinding{ID: bindingID, ProjectID: projectID, Stage: workflowbinding.StageReview, WorkflowConfigurationID: configID, Version: 4}}, serviceConfigs{w: globalconfig.Workflow{Common: globalconfig.Common{ID: configID, Version: 3, Enabled: true, IntegrationStatus: "verified"}, ConnectionID: connectionID, ApplicableStages: []string{"review"}, TypeConfig: json.RawMessage(`{"webhook_secret":"x"}`), DefaultParameters: json.RawMessage(`{"token":"x"}`)}}, serviceConnections{c: globalconfig.Connection{Common: globalconfig.Common{ID: connectionID, Version: 2, Enabled: true, IntegrationStatus: "verified"}, ConnectionType: "n8n", TypeConfig: json.RawMessage(`{"api_key":"x"}`)}})
	s.now = func() time.Time { return now }
	return s, store, projectID
}
func TestCreateRunUsesLatestContractAndSafeSnapshot(t *testing.T) {
	s, store, projectID := fixtureService(t)
	r, e := s.CreateRun(context.Background(), CreateRunCommand{ProjectID: projectID, Stage: "review", InputPayload: json.RawMessage(`{"ok":true}`), IdempotencyKey: "key"})
	if e != nil {
		t.Fatal(e)
	}
	if r.Status != StatusQueued || r.TriggerSource != "manual" || len(store.events[r.ID]) != 1 {
		t.Fatalf("run=%+v events=%d", r, len(store.events[r.ID]))
	}
	if string(r.ConfigurationSnapshot) == "" || string(r.ConfigurationSnapshot) == `{"token":"x"}` || !json.Valid(r.ConfigurationSnapshot) {
		t.Fatal("unsafe snapshot")
	}
	if _, e = s.CreateRun(context.Background(), CreateRunCommand{ProjectID: projectID, Stage: "bad", InputPayload: json.RawMessage(`{}`), IdempotencyKey: "key"}); !errors.Is(e, ErrValidation) {
		t.Fatalf("err=%v", e)
	}
}
func TestRetryAndCancelVersionRules(t *testing.T) {
	s, store, projectID := fixtureService(t)
	id := uuid.New()
	now := s.now()
	original := WorkflowRun{ID: id, RunNumber: "WR-1", ProjectID: projectID, Stage: "review", WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusFailed, ConfigurationSnapshot: json.RawMessage(`{}`), InputPayload: json.RawMessage(`{"a":1}`), ErrorCode: ptr("x"), ErrorMessage: ptr("safe"), ErrorDetails: json.RawMessage(`{}`), StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2}
	store.runs[id] = original
	r, e := s.RetryRun(context.Background(), RetryCommand{RunID: id, ExpectedVersion: 2, InputOverride: json.RawMessage(`{"b":2}`), IdempotencyKey: "x"})
	if e != nil || r.TriggerSource != "retry" || r.RetryOfRunID == nil || string(r.InputPayload) != `{"b":2}` {
		t.Fatalf("run=%+v err=%v", r, e)
	}
	q := WorkflowRun{ID: uuid.New(), RunNumber: "WR-2", ProjectID: projectID, Stage: "review", WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued, ConfigurationSnapshot: json.RawMessage(`{}`), InputPayload: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now, Version: 1}
	store.runs[q.ID] = q
	cancelled, e := s.CancelRun(context.Background(), RunCommand{RunID: q.ID, ExpectedVersion: 1, IdempotencyKey: "x"})
	if e != nil || cancelled.Status != StatusCancelled || len(store.events[q.ID]) != 1 {
		t.Fatalf("run=%+v err=%v", cancelled, e)
	}
}

func TestWorkerExecutesQueuedRuns(t *testing.T) {
	s, store, projectID := fixtureService(t)
	connectionID, runID := uuid.New(), uuid.New()
	now := s.now()
	store.runs[runID] = WorkflowRun{ID: runID, RunNumber: "WR-WORKER", ProjectID: projectID, Stage: "review", WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued, ConfigurationSnapshot: json.RawMessage(`{"workflowConnection":{"id":"` + connectionID.String() + `"},"workflowConfiguration":{"defaultParameters":{}}}`), InputPayload: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now, Version: 1}
	fake := &FakeWorkflowExecutor{ExecuteResult: ExecutionResult{Status: ExecutionSucceeded, Output: json.RawMessage(`{"result":"ok"}`)}}
	s.SetWorkflowExecutor(fake)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.RunWorker(ctx, time.Hour, func(err error) { t.Errorf("worker error: %v", err) })
		close(done)
	}()
	deadline := time.Now().Add(time.Second)
	for store.runs[runID].Status != StatusSucceeded && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	<-done
	if store.runs[runID].Status != StatusSucceeded || fake.ExecuteCalls != 1 {
		t.Fatalf("run=%+v calls=%d", store.runs[runID], fake.ExecuteCalls)
	}
}

func ptr(s string) *string { return &s }

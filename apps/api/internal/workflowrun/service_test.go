package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
)

type serviceStore struct {
	runs        map[uuid.UUID]WorkflowRun
	events      map[uuid.UUID][]Event
	candidates  map[uuid.UUID]bool
	idempotency map[string]struct {
		hash string
		run  WorkflowRun
	}
	createErr error
}

func (s *serviceStore) ExecuteIdempotent(_ context.Context, scope string, key string, hash string, fn func(Store) (WorkflowRun, error)) (WorkflowRun, error) {
	run, _, err := s.ExecuteIdempotentWithReplay(context.Background(), scope, key, hash, fn)
	return run, err
}
func (s *serviceStore) ExecuteIdempotentWithReplay(_ context.Context, scope string, key string, hash string, fn func(Store) (WorkflowRun, error)) (WorkflowRun, bool, error) {
	id := scope + ":" + key
	if record, ok := s.idempotency[id]; ok {
		if record.hash != hash {
			return WorkflowRun{}, false, ErrIdempotencyConflict
		}
		return record.run, true, nil
	}
	run, err := fn(s)
	if err == nil {
		s.idempotency[id] = struct {
			hash string
			run  WorkflowRun
		}{hash, run}
	}
	return run, false, err
}
func (s *serviceStore) PreflightTokenUsed(_ context.Context, nonce string) (bool, error) {
	for _, run := range s.runs {
		var input struct {
			PreflightTokenNonce string `json:"preflightTokenNonce"`
		}
		if json.Unmarshal(run.InputPayload, &input) == nil && input.PreflightTokenNonce == nonce {
			return true, nil
		}
	}
	return false, nil
}
func (s *serviceStore) HasContentGenerationCandidate(_ context.Context, runID uuid.UUID) (bool, error) {
	return s.candidates[runID], nil
}
func (s *serviceStore) CreateWithInitialEvent(_ context.Context, run WorkflowRun, event Event) (WorkflowRun, Event, error) {
	if s.createErr != nil {
		return WorkflowRun{}, Event{}, s.createErr
	}
	s.runs[run.ID] = run
	s.events[run.ID] = append(s.events[run.ID], event)
	return run, event, nil
}

func TestPreflightTokenCreateFailureDoesNotConsumeToken(t *testing.T) {
	s, store, projectID := fixtureService(t)
	nonce := uuid.NewString()
	prepare := func() (CreateRunCommand, error) {
		return CreateRunCommand{ProjectID: projectID, Stage: "review", TriggerSource: "manual", InputPayload: json.RawMessage(`{"preflightTokenNonce":"` + nonce + `"}`)}, nil
	}
	store.createErr = errors.New("transaction create failed")
	if _, err := s.CreateRunForPreflightToken(context.Background(), projectID, nonce, "first", prepare); err == nil {
		t.Fatal("expected create failure")
	}
	if used, err := store.PreflightTokenUsed(context.Background(), nonce); err != nil || used {
		t.Fatalf("used=%v err=%v", used, err)
	}
	store.createErr = nil
	if _, err := s.CreateRunForPreflightToken(context.Background(), projectID, nonce, "retry", prepare); err != nil {
		t.Fatalf("retry=%v", err)
	}
	if len(store.runs) != 1 || len(store.events) != 1 {
		t.Fatalf("partial state runs=%d events=%d", len(store.runs), len(store.events))
	}
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
func (s *serviceStore) SaveExternalExecutionID(_ context.Context, current WorkflowRun, id string) (WorkflowRun, error) {
	r := s.runs[current.ID]
	if r.Version != current.Version {
		return WorkflowRun{}, ErrVersionConflict
	}
	if r.ExternalExecutionID != nil {
		if *r.ExternalExecutionID == id {
			return r, nil
		}
		return WorkflowRun{}, ErrVersionConflict
	}
	r.ExternalExecutionID = &id
	r.Version++
	r.UpdatedAt = time.Now().UTC()
	s.runs[r.ID] = r
	return r, nil
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
	store := &serviceStore{runs: map[uuid.UUID]WorkflowRun{}, events: map[uuid.UUID][]Event{}, candidates: map[uuid.UUID]bool{}, idempotency: map[string]struct {
		hash string
		run  WorkflowRun
	}{}}
	s := NewService(store, serviceProjects{p: project.Project{ID: projectID}}, serviceBindings{b: workflowbinding.ProjectWorkflowBinding{ID: bindingID, ProjectID: projectID, Stage: workflowbinding.StageReview, WorkflowConfigurationID: configID, Version: 4}}, serviceConfigs{w: globalconfig.Workflow{Common: globalconfig.Common{ID: configID, Version: 3, Enabled: true, Executable: true, IntegrationStatus: "verified"}, ConnectionID: connectionID, ApplicableStages: []string{"chapter_planning", "content_generation", "review", "rewrite"}, TypeConfig: json.RawMessage(`{"webhook_secret":"x"}`), DefaultParameters: json.RawMessage(`{"token":"x"}`)}}, serviceConnections{c: globalconfig.Connection{Common: globalconfig.Common{ID: connectionID, Version: 2, Enabled: true, Executable: true, IntegrationStatus: "verified"}, ConnectionType: "n8n", BaseURL: "http://localhost", AuthType: "api_key", TypeConfig: json.RawMessage(`{"api_key":"x"}`)}})
	s.now = func() time.Time { return now }
	return s, store, projectID
}
func TestCreateRunProtectsDomainOwnedStages(t *testing.T) {
	s, _, projectID := fixtureService(t)
	if _, e := s.CreateRun(context.Background(), CreateRunCommand{ProjectID: projectID, Stage: "bad", InputPayload: json.RawMessage(`{}`), IdempotencyKey: "bad-stage"}); !errors.Is(e, ErrValidation) {
		t.Fatalf("err=%v", e)
	}
	for _, stage := range []string{"content_generation", "chapter_planning", "review"} {
		if _, e := s.CreateRun(context.Background(), CreateRunCommand{ProjectID: projectID, Stage: stage, InputPayload: json.RawMessage(`{}`), IdempotencyKey: "protected-" + stage}); !errors.Is(e, ErrProtectedStage) {
			t.Fatalf("stage=%s err=%v", stage, e)
		}
	}
}

func TestContentGenerationRetryRejectsInputOverride(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	id := uuid.New()
	store.runs[id] = WorkflowRun{ID: id, RunNumber: "WR-CONTENT", ProjectID: projectID, Stage: "content_generation", WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusFailed, ConfigurationSnapshot: json.RawMessage(`{}`), InputPayload: json.RawMessage(`{"sourceContentVersionId":"11111111-1111-4111-8111-111111111111","sourceContentVersionVersion":1}`), ErrorCode: ptr("x"), ErrorMessage: ptr("safe"), ErrorDetails: json.RawMessage(`{}`), StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2}
	if _, err := s.RetryRun(context.Background(), RetryCommand{RunID: id, ExpectedVersion: 2, Mode: "current_configuration", InputOverride: json.RawMessage(`{"sourceContentVersionId":"22222222-2222-4222-8222-222222222222"}`), IdempotencyKey: "override"}); !errors.Is(err, ErrValidation) {
		t.Fatalf("err=%v", err)
	}
	if len(store.runs) != 1 {
		t.Fatal("content generation retry was created")
	}
}

func TestContentGenerationRuntimeRetryEligibilityMatrix(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    Status
		stage     string
		events    []Event
		candidate bool
		want      bool
	}{
		{name: "failed", status: StatusFailed, stage: "content_generation", want: true},
		{name: "cancelled", status: StatusCancelled, stage: "content_generation", want: true},
		{name: "output validation failed", status: StatusSucceeded, stage: "content_generation", events: []Event{{EventType: EventTypeOutputValidationFailed}}, want: true},
		{name: "ordinary succeeded", status: StatusSucceeded, stage: "content_generation"},
		{name: "result consumption failed", status: StatusSucceeded, stage: "content_generation", events: []Event{{EventType: EventTypeOutputValidationFailed}, {EventType: EventTypeResultConsumptionFailed}}},
		{name: "result consumed", status: StatusSucceeded, stage: "content_generation", events: []Event{{EventType: EventTypeOutputValidationFailed}, {EventType: EventTypeResultConsumed}}},
		{name: "candidate exists", status: StatusSucceeded, stage: "content_generation", events: []Event{{EventType: EventTypeOutputValidationFailed}}, candidate: true},
		{name: "failed with candidate", status: StatusFailed, stage: "content_generation", candidate: true},
		{name: "review output validation failed", status: StatusSucceeded, stage: "review", events: []Event{{EventType: EventTypeOutputValidationFailed}}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, store, projectID := fixtureService(t)
			id, subjectID := uuid.New(), uuid.New()
			subjectType := "content_item"
			now := s.now()
			original := WorkflowRun{ID: id, RunNumber: "WR-ORIGINAL", ProjectID: projectID, Stage: tc.stage, WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: tc.status, SubjectType: &subjectType, SubjectID: &subjectID, ConfigurationSnapshot: json.RawMessage(`{"frozen":true}`), InputPayload: json.RawMessage(`{"sourceContentVersionId":"11111111-1111-4111-8111-111111111111","sourceContentVersionVersion":1}`), CreatedAt: now, UpdatedAt: now, Version: 2}
			if tc.status == StatusFailed {
				original.ErrorCode = ptr("failed")
				original.ErrorMessage = ptr("safe")
				original.ErrorDetails = json.RawMessage(`{}`)
				original.StartedAt = &now
				original.FinishedAt = &now
			}
			if tc.status == StatusCancelled {
				original.StartedAt = &now
				original.FinishedAt = &now
				original.CancelledAt = &now
			}
			if tc.status == StatusSucceeded {
				original.StartedAt = &now
				original.FinishedAt = &now
				original.OutputPayload = json.RawMessage(`{"invalid":true}`)
			}
			store.runs[id] = original
			store.events[id] = tc.events
			store.candidates[id] = tc.candidate
			retried, err := s.RetryRun(context.Background(), RetryCommand{RunID: id, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "retry"})
			if !tc.want {
				if !errors.Is(err, ErrNotRetryable) {
					t.Fatalf("RetryRun error=%v", err)
				}
				if len(store.runs) != 1 {
					t.Fatalf("run count=%d", len(store.runs))
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if retried.ID == id || retried.RetryOfRunID == nil || *retried.RetryOfRunID != id || retried.SubjectType == nil || *retried.SubjectType != subjectType || retried.SubjectID == nil || *retried.SubjectID != subjectID || string(retried.InputPayload) != string(original.InputPayload) {
				t.Fatalf("retried=%+v original=%+v", retried, original)
			}
			replay, replayErr := s.RetryRun(context.Background(), RetryCommand{RunID: id, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "retry"})
			if replayErr != nil || replay.ID != retried.ID || len(store.runs) != 2 {
				t.Fatalf("replay=%+v err=%v runs=%d", replay, replayErr, len(store.runs))
			}
		})
	}
}
func TestRetryAndCancelVersionRules(t *testing.T) {
	s, store, projectID := fixtureService(t)
	id := uuid.New()
	now := s.now()
	original := WorkflowRun{ID: id, RunNumber: "WR-1", ProjectID: projectID, Stage: "review", WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusFailed, ConfigurationSnapshot: json.RawMessage(`{}`), InputPayload: json.RawMessage(`{"a":1}`), ErrorCode: ptr("x"), ErrorMessage: ptr("safe"), ErrorDetails: json.RawMessage(`{}`), StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2}
	store.runs[id] = original
	r, e := s.RetryRun(context.Background(), RetryCommand{RunID: id, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "x"})
	if e != nil || r.TriggerSource != "retry" || r.RetryOfRunID == nil || string(r.InputPayload) != `{"a":1}` {
		t.Fatalf("run=%+v err=%v", r, e)
	}
	if _, e = s.RetryRun(context.Background(), RetryCommand{RunID: id, ExpectedVersion: 2, Mode: "current_configuration", InputOverride: json.RawMessage(`{"b":2}`), IdempotencyKey: "override"}); !errors.Is(e, ErrValidation) {
		t.Fatalf("review input override error=%v", e)
	}
	reason := "different normalized reason"
	if _, e = s.RetryRun(context.Background(), RetryCommand{RunID: id, ExpectedVersion: 2, Mode: "current_configuration", Reason: &reason, IdempotencyKey: "x"}); !errors.Is(e, ErrIdempotencyConflict) {
		t.Fatalf("retry reason fingerprint error=%v", e)
	}
	q := WorkflowRun{ID: uuid.New(), RunNumber: "WR-2", ProjectID: projectID, Stage: "review", WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusQueued, ConfigurationSnapshot: json.RawMessage(`{}`), InputPayload: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now, Version: 1}
	store.runs[q.ID] = q
	cancelled, e := s.CancelRun(context.Background(), RunCommand{RunID: q.ID, ExpectedVersion: 1, IdempotencyKey: "x"})
	if e != nil || cancelled.Status != StatusCancelling || len(store.events[q.ID]) != 1 {
		t.Fatalf("run=%+v err=%v", cancelled, e)
	}
}

func TestRetryOptionsReuseSnapshotAndResultConsumptionQualification(t *testing.T) {
	s, store, projectID := fixtureService(t)
	config := s.configurations.(serviceConfigs).w
	connection := s.connections.(serviceConnections).c
	fingerprint := "credential-fingerprint-v1"
	connection.CredentialFingerprint = &fingerprint
	s.connections = serviceConnections{c: connection}
	now := s.now()
	run := WorkflowRun{ID: uuid.New(), RunNumber: "WR-OPTIONS", ProjectID: projectID, Stage: "review", WorkflowConfigurationID: config.ID, TriggerSource: "manual", Status: StatusFailed, ConfigurationSnapshot: mustSafeJSON(map[string]any{"workflowConfiguration": map[string]any{"id": config.ID, "version": config.Version}}), InputPayload: json.RawMessage(`{"safe":true}`), ErrorCode: ptr("failed"), ErrorMessage: ptr("safe"), ErrorDetails: json.RawMessage(`{}`), Retryability: "runtime_retry", BindingSnapshot: mustSafeJSON(map[string]any{"bindingId": uuid.New(), "bindingVersion": 1, "stage": "review"}), ConnectionSnapshot: mustSafeJSON(map[string]any{"id": connection.ID, "name": "n8n", "version": connection.Version, "connectionType": "n8n", "baseUrl": "http://localhost", "authType": "api_key", "credentialFingerprint": fingerprint}), LlmPolicySnapshot: json.RawMessage(`{"strategy":"none","providerId":null,"providerName":null,"providerVersion":null,"model":null,"secretFingerprint":null}`), StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2}
	store.runs[run.ID] = run
	options, err := s.GetRetryOptions(context.Background(), run.ID)
	if err != nil || !options.CurrentConfiguration.Enabled || !options.OriginalConfiguration.Enabled || options.Retryability != "runtime_retry" {
		t.Fatalf("options=%+v err=%v", options, err)
	}

	changed := "credential-fingerprint-v2"
	connection.CredentialFingerprint = &changed
	s.connections = serviceConnections{c: connection}
	options, err = s.GetRetryOptions(context.Background(), run.ID)
	if err != nil || options.OriginalConfiguration.Enabled || len(options.OriginalConfiguration.Reasons) == 0 || options.OriginalConfiguration.Reasons[0].Code != "credential_fingerprint_changed" {
		t.Fatalf("changed options=%+v err=%v", options, err)
	}

	store.events[run.ID] = []Event{{ID: uuid.New(), RunID: run.ID, EventType: EventTypeResultConsumptionFailed, Status: StatusSucceeded, Payload: json.RawMessage(`{}`), CreatedAt: now}}
	options, err = s.GetRetryOptions(context.Background(), run.ID)
	if err != nil || !options.ResultConsumptionRetryRequired || options.CurrentConfiguration.Enabled || options.OriginalConfiguration.Enabled {
		t.Fatalf("consumption options=%+v err=%v", options, err)
	}
}

func TestRetryOptionsDisableIncompleteSnapshot(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	run := WorkflowRun{ID: uuid.New(), RunNumber: "WR-INCOMPLETE", ProjectID: projectID, Stage: "review", WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusFailed, ConfigurationSnapshot: json.RawMessage(`{}`), InputPayload: json.RawMessage(`{}`), ErrorCode: ptr("failed"), ErrorMessage: ptr("safe"), ErrorDetails: json.RawMessage(`{}`), Retryability: "runtime_retry", BindingSnapshot: json.RawMessage(`{}`), ConnectionSnapshot: json.RawMessage(`{}`), LlmPolicySnapshot: json.RawMessage(`{}`), StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2}
	store.runs[run.ID] = run
	options, err := s.GetRetryOptions(context.Background(), run.ID)
	if err != nil || options.OriginalConfiguration.Enabled || options.OriginalConfiguration.Reasons[0].Code != "snapshot_incomplete" {
		t.Fatalf("options=%+v err=%v", options, err)
	}
}

func TestRuntimeRetryChainRecordsEachDirectParent(t *testing.T) {
	s, store, projectID := fixtureService(t)
	now := s.now()
	a := WorkflowRun{
		ID: uuid.New(), RunNumber: "WR-A", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusFailed,
		ConfigurationSnapshot: json.RawMessage(`{}`), InputPayload: json.RawMessage(`{"a":1}`),
		ErrorCode: ptr("failed"), ErrorMessage: ptr("safe"), ErrorDetails: json.RawMessage(`{}`),
		StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2,
	}
	store.runs[a.ID] = a
	b, err := s.RetryRun(context.Background(), RetryCommand{RunID: a.ID, ExpectedVersion: a.Version, Mode: "current_configuration", IdempotencyKey: "retry-a-b"})
	if err != nil {
		t.Fatal(err)
	}
	b.Status = StatusFailed
	b.ErrorCode, b.ErrorMessage, b.ErrorDetails = ptr("failed"), ptr("safe"), json.RawMessage(`{}`)
	b.StartedAt, b.FinishedAt, b.UpdatedAt, b.Version = &now, &now, now, 2
	store.runs[b.ID] = b
	c, err := s.RetryRun(context.Background(), RetryCommand{RunID: b.ID, ExpectedVersion: b.Version, Mode: "current_configuration", IdempotencyKey: "retry-b-c"})
	if err != nil {
		t.Fatal(err)
	}
	if b.RetryOfRunID == nil || *b.RetryOfRunID != a.ID {
		t.Fatalf("B retry parent = %v, want %s", b.RetryOfRunID, a.ID)
	}
	if c.RetryOfRunID == nil || *c.RetryOfRunID != b.ID {
		t.Fatalf("C retry parent = %v, want %s", c.RetryOfRunID, b.ID)
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

type succeededConsumerSpy struct {
	calls int
	stage string
}

func (spy *succeededConsumerSpy) ConsumeSucceededRun(_ context.Context, run WorkflowRun) error {
	spy.calls++
	spy.stage = run.Stage
	return nil
}

func TestSucceededResultRoutesOnlyToMatchingStageConsumer(t *testing.T) {
	for _, stage := range []string{"chapter_planning", "content_generation", "review", "rewrite"} {
		t.Run(stage, func(t *testing.T) {
			service, store, projectID := fixtureService(t)
			now := service.now()
			run := WorkflowRun{
				ID: uuid.New(), RunNumber: "WR-" + strings.ToUpper(uuid.NewString()[:8]),
				ProjectID: projectID, Stage: stage, WorkflowConfigurationID: uuid.New(),
				TriggerSource: "manual", Status: StatusRunning,
				ConfigurationSnapshot: json.RawMessage(`{}`), InputPayload: json.RawMessage(`{}`),
				StartedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2,
			}
			store.runs[run.ID] = run
			chapter, generation, review, rewrite := &succeededConsumerSpy{}, &succeededConsumerSpy{}, &succeededConsumerSpy{}, &succeededConsumerSpy{}
			service.SetSucceededConsumer(chapter)
			service.SetContentSucceededConsumer(generation)
			service.SetReviewSucceededConsumer(review)
			service.SetRewriteSucceededConsumer(rewrite)
			if _, err := service.applyExecutionResult(context.Background(), run, ExecutionResult{
				Status: ExecutionSucceeded, Output: json.RawMessage(`{"ok":true}`),
			}); err != nil {
				t.Fatal(err)
			}
			expected := map[string]*succeededConsumerSpy{
				"chapter_planning": chapter, "content_generation": generation,
				"review": review, "rewrite": rewrite,
			}
			for consumerStage, spy := range expected {
				want := 0
				if consumerStage == stage {
					want = 1
				}
				if spy.calls != want || want == 1 && spy.stage != stage {
					t.Fatalf("stage=%s consumer=%s calls=%d consumedStage=%s", stage, consumerStage, spy.calls, spy.stage)
				}
			}
		})
	}
}

func ptr(s string) *string { return &s }

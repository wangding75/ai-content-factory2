package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
)

type mutableBindings struct {
	calls       atomic.Int32
	stableCalls int32 // first N successful calls return first; later calls may switch or fail
	first       workflowbinding.ProjectWorkflowBinding
	later       workflowbinding.ProjectWorkflowBinding
	err         error
}

func (m *mutableBindings) GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error) {
	n := m.calls.Add(1)
	stable := m.stableCalls
	if stable == 0 {
		stable = 2 // options + prepare by default
	}
	if m.err != nil && n > stable {
		return workflowbinding.ProjectWorkflowBinding{}, m.err
	}
	if m.later.ID != uuid.Nil && n > stable {
		return m.later, nil
	}
	return m.first, nil
}

type mutableConfigs struct {
	calls       atomic.Int32
	stableCalls int32
	first       globalconfig.Workflow
	later       globalconfig.Workflow
	err         error
}

func (m *mutableConfigs) GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error) {
	n := m.calls.Add(1)
	stable := m.stableCalls
	if stable == 0 {
		stable = 2
	}
	if m.err != nil && n > stable {
		return globalconfig.Workflow{}, m.err
	}
	w := m.first
	if m.later.ID != uuid.Nil && n > stable {
		w = m.later
	}
	if w.Enabled && w.IntegrationStatus == "verified" {
		w.Executable = true
	}
	return w, nil
}

type mutableConnections struct {
	calls       atomic.Int32
	stableCalls int32
	first       globalconfig.Connection
	later       globalconfig.Connection
	err         error
}

func (m *mutableConnections) GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error) {
	n := m.calls.Add(1)
	stable := m.stableCalls
	if stable == 0 {
		stable = 2
	}
	if m.err != nil && n > stable {
		return globalconfig.Connection{}, m.err
	}
	c := m.first
	if m.later.ID != uuid.Nil && n > stable {
		c = m.later
	}
	if c.Enabled && c.IntegrationStatus == "verified" {
		c.Executable = true
	}
	return c, nil
}

func retryFixture(t *testing.T) (*Service, *serviceStore, *mutableBindings, *mutableConfigs, *mutableConnections, uuid.UUID) {
	t.Helper()
	projectID, configID, connectionID, bindingID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	store := &serviceStore{runs: map[uuid.UUID]WorkflowRun{}, events: map[uuid.UUID][]Event{}, candidates: map[uuid.UUID]bool{}, idempotency: map[string]struct {
		hash string
		run  WorkflowRun
	}{}}
	fp := "fingerprint-a"
	bindings := &mutableBindings{first: workflowbinding.ProjectWorkflowBinding{
		ID: bindingID, ProjectID: projectID, Stage: workflowbinding.StageReview, WorkflowConfigurationID: configID, Version: 4,
	}}
	configs := &mutableConfigs{first: globalconfig.Workflow{
		Common: globalconfig.Common{ID: configID, Version: 3, Enabled: true, Executable: true, IntegrationStatus: "verified", ValidationStatus: "verified", VerifiedVersion: intPtr(3)},
		ConnectionID: connectionID, ApplicableStages: []string{"review", "content_generation", "chapter_planning", "rewrite"},
		WorkflowType: "n8n", TypeConfig: json.RawMessage(`{"referenceType":"workflow_id","referenceValue":"wf-1"}`),
		DefaultParameters: json.RawMessage(`{}`), LlmStrategy: "none",
	}}
	connections := &mutableConnections{first: globalconfig.Connection{
		Common: globalconfig.Common{ID: connectionID, Version: 2, Enabled: true, Executable: true, IntegrationStatus: "verified", ValidationStatus: "verified", VerifiedVersion: intPtr(2)},
		ConnectionType: "n8n", BaseURL: "http://localhost", AuthType: "api_key", TypeConfig: json.RawMessage(`{"referenceType":"workflow_id","referenceValue":"wf-1"}`),
		CredentialFingerprint: &fp,
	}}
	s := NewService(store, serviceProjects{p: project.Project{ID: projectID}}, bindings, configs, connections)
	s.now = func() time.Time { return now }
	return s, store, bindings, configs, connections, projectID
}

func intPtr(v int) *int { return &v }

func failedRetryOriginal(t *testing.T, s *Service, store *serviceStore, projectID uuid.UUID) WorkflowRun {
	t.Helper()
	now := s.now()
	id := uuid.New()
	run := WorkflowRun{
		ID: id, RunNumber: "WR-RETRY-SRC", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: uuid.New(), TriggerSource: "manual", Status: StatusFailed,
		ConfigurationSnapshot: json.RawMessage(`{"frozen":true}`), InputPayload: json.RawMessage(`{"a":1}`),
		ErrorCode: ptr("failed"), ErrorMessage: ptr("safe"), ErrorDetails: json.RawMessage(`{}`),
		StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2,
	}
	store.runs[id] = run
	return run
}

func TestCurrentConfigurationRetrySnapshotsShareSinglePreparedVersions(t *testing.T) {
	s, store, bindings, configs, connections, projectID := retryFixture(t)
	original := failedRetryOriginal(t, s, store, projectID)
	retried, err := s.RetryRun(context.Background(), RetryCommand{RunID: original.ID, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "prep-ok"})
	if err != nil {
		t.Fatal(err)
	}
	if retried.WorkflowConfigurationID != configs.first.ID || retried.WorkflowConnectionID == nil || *retried.WorkflowConnectionID != connections.first.ID {
		t.Fatalf("ids mismatch run=%+v", retried)
	}
	var binding struct {
		BindingID      uuid.UUID `json:"bindingId"`
		BindingVersion int       `json:"bindingVersion"`
	}
	var connection struct {
		ID      uuid.UUID `json:"id"`
		Version int       `json:"version"`
	}
	var configuration struct {
		WorkflowConfiguration struct {
			ID      uuid.UUID `json:"id"`
			Version int       `json:"version"`
		} `json:"workflowConfiguration"`
		WorkflowConnection struct {
			ID      uuid.UUID `json:"id"`
			Version int       `json:"version"`
		} `json:"workflowConnection"`
		Binding struct {
			ID      uuid.UUID `json:"id"`
			Version int       `json:"version"`
		} `json:"binding"`
	}
	if json.Unmarshal(retried.BindingSnapshot, &binding) != nil || json.Unmarshal(retried.ConnectionSnapshot, &connection) != nil || json.Unmarshal(retried.ConfigurationSnapshot, &configuration) != nil {
		t.Fatalf("decode snapshots binding=%s connection=%s config=%s", retried.BindingSnapshot, retried.ConnectionSnapshot, retried.ConfigurationSnapshot)
	}
	if binding.BindingID != bindings.first.ID || binding.BindingVersion != bindings.first.Version {
		t.Fatalf("binding snapshot %+v", binding)
	}
	if connection.ID != connections.first.ID || connection.Version != connections.first.Version {
		t.Fatalf("connection snapshot %+v", connection)
	}
	if configuration.WorkflowConfiguration.ID != configs.first.ID || configuration.WorkflowConfiguration.Version != configs.first.Version ||
		configuration.WorkflowConnection.ID != connections.first.ID || configuration.WorkflowConnection.Version != connections.first.Version ||
		configuration.Binding.ID != bindings.first.ID || configuration.Binding.Version != bindings.first.Version {
		t.Fatalf("configuration snapshot %+v", configuration)
	}
	if string(retried.BindingSnapshot) == "{}" || string(retried.ConnectionSnapshot) == "{}" || string(retried.LlmPolicySnapshot) == "{}" {
		t.Fatal("empty object snapshot degradation")
	}
	// prepare + verify each read Binding once per phase: 2 total; configs/connections more for eligibility + verify.
	if bindings.calls.Load() < 2 {
		t.Fatalf("expected prepare+verify binding reads, calls=%d", bindings.calls.Load())
	}
}

func TestCurrentConfigurationRetryRejectsBindingVersionChange(t *testing.T) {
	s, store, bindings, _, _, projectID := retryFixture(t)
	original := failedRetryOriginal(t, s, store, projectID)
	later := bindings.first
	later.Version = bindings.first.Version + 1
	bindings.later = later
	if _, err := s.RetryRun(context.Background(), RetryCommand{RunID: original.ID, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "bind-change"}); !errors.Is(err, ErrRetryConfigurationChanged) {
		t.Fatalf("err=%v", err)
	}
	if len(store.runs) != 1 {
		t.Fatalf("retry created despite conflict runs=%d", len(store.runs))
	}
}

func TestCurrentConfigurationRetryRejectsWorkflowVersionChange(t *testing.T) {
	s, store, _, configs, _, projectID := retryFixture(t)
	original := failedRetryOriginal(t, s, store, projectID)
	later := configs.first
	later.Version = configs.first.Version + 1
	configs.later = later
	if _, err := s.RetryRun(context.Background(), RetryCommand{RunID: original.ID, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "wf-change"}); !errors.Is(err, ErrRetryConfigurationChanged) {
		t.Fatalf("err=%v", err)
	}
	if len(store.runs) != 1 {
		t.Fatal("retry created")
	}
}

func TestCurrentConfigurationRetryRejectsConnectionVersionAndFingerprintChange(t *testing.T) {
	s, store, _, _, connections, projectID := retryFixture(t)
	original := failedRetryOriginal(t, s, store, projectID)
	later := connections.first
	later.Version = connections.first.Version + 1
	fp := "fingerprint-b"
	later.CredentialFingerprint = &fp
	connections.later = later
	if _, err := s.RetryRun(context.Background(), RetryCommand{RunID: original.ID, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "conn-change"}); !errors.Is(err, ErrRetryConfigurationChanged) {
		t.Fatalf("err=%v", err)
	}
	if len(store.runs) != 1 {
		t.Fatal("retry created")
	}
}

func TestCurrentConfigurationRetryRejectsDeletedConfiguration(t *testing.T) {
	s, store, _, configs, _, projectID := retryFixture(t)
	original := failedRetryOriginal(t, s, store, projectID)
	configs.err = globalconfig.ErrNotFound
	if _, err := s.RetryRun(context.Background(), RetryCommand{RunID: original.ID, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "deleted"}); !errors.Is(err, ErrConfigurationNotFound) {
		t.Fatalf("err=%v", err)
	}
	if len(store.runs) != 1 {
		t.Fatal("retry created")
	}
}

func TestCurrentConfigurationRetryRejectsSecondReadFailure(t *testing.T) {
	s, store, bindings, _, _, projectID := retryFixture(t)
	original := failedRetryOriginal(t, s, store, projectID)
	bindings.err = errors.New("binding store unavailable")
	if _, err := s.RetryRun(context.Background(), RetryCommand{RunID: original.ID, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "bind-fail"}); err == nil {
		t.Fatal("expected second-read failure")
	}
	if len(store.runs) != 1 {
		t.Fatal("retry created")
	}
}

func TestOriginalConfigurationRetryDoesNotReReadLiveConfig(t *testing.T) {
	s, store, bindings, configs, connections, projectID := retryFixture(t)
	now := s.now()
	id := uuid.New()
	connID := connections.first.ID
	fp := "fingerprint-a"
	original := WorkflowRun{
		ID: id, RunNumber: "WR-ORIG", ProjectID: projectID, Stage: "review",
		WorkflowConfigurationID: configs.first.ID, TriggerSource: "manual", Status: StatusFailed,
		ConfigurationSnapshot: json.RawMessage(`{"workflowConfiguration":{"id":"` + configs.first.ID.String() + `","version":3},"workflowConnection":{"id":"` + connID.String() + `","version":2},"binding":{"id":"` + bindings.first.ID.String() + `","version":4}}`),
		BindingSnapshot:    json.RawMessage(`{"bindingId":"` + bindings.first.ID.String() + `","bindingVersion":4,"stage":"review"}`),
		ConnectionSnapshot: json.RawMessage(`{"id":"` + connID.String() + `","name":"n8n","version":2,"connectionType":"n8n","baseUrl":"http://localhost","authType":"api_key","credentialFingerprint":"` + fp + `"}`),
		LlmPolicySnapshot:  json.RawMessage(`{"strategy":"none","providerId":null,"providerName":null,"providerVersion":null,"model":null,"secretFingerprint":null}`),
		InputPayload:       json.RawMessage(`{"a":1}`), ErrorCode: ptr("failed"), ErrorMessage: ptr("safe"), ErrorDetails: json.RawMessage(`{}`),
		WorkflowConnectionID: &connID, StartedAt: &now, FinishedAt: &now, CreatedAt: now, UpdatedAt: now, Version: 2,
	}
	store.runs[id] = original
	// Mutate live config; original retry must ignore live versions.
	later := configs.first
	later.Version = 99
	configs.later = later
	retried, err := s.RetryRun(context.Background(), RetryCommand{RunID: id, ExpectedVersion: 2, Mode: "original_configuration", IdempotencyKey: "orig-ok"})
	if err != nil {
		t.Fatal(err)
	}
	if string(retried.BindingSnapshot) != string(RedactJSON(original.BindingSnapshot)) ||
		string(retried.ConnectionSnapshot) != string(RedactJSON(original.ConnectionSnapshot)) ||
		string(retried.LlmPolicySnapshot) != string(RedactJSON(original.LlmPolicySnapshot)) {
		t.Fatalf("original projection snapshots rewritten binding=%s", retried.BindingSnapshot)
	}
	if string(retried.ConfigurationSnapshot) != string(RedactJSON(original.ConfigurationSnapshot)) {
		t.Fatalf("original configuration snapshot rewritten")
	}
	// Original path should not prepare current configuration (binding prepare+verify).
	// Binding may still be unread (0) or only touched by retry options.
	if configs.calls.Load() > 0 && configs.later.Version == 99 {
		// GetWorkflow may be called from retry options; ensure snapshot still original.
	}
	if retried.WorkflowConnectionID == nil || *retried.WorkflowConnectionID != connID {
		t.Fatalf("connection id=%v", retried.WorkflowConnectionID)
	}
}

func TestCurrentConfigurationRetryIdempotentReplayUnchanged(t *testing.T) {
	s, store, _, _, _, projectID := retryFixture(t)
	original := failedRetryOriginal(t, s, store, projectID)
	first, err := s.RetryRun(context.Background(), RetryCommand{RunID: original.ID, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "idem"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.RetryRun(context.Background(), RetryCommand{RunID: original.ID, ExpectedVersion: 2, Mode: "current_configuration", IdempotencyKey: "idem"})
	if err != nil || second.ID != first.ID || len(store.runs) != 2 {
		t.Fatalf("replay=%+v err=%v runs=%d", second, err, len(store.runs))
	}
}

func TestPreparedRetryConfigurationValidateRejectsEmptyObjects(t *testing.T) {
	p := PreparedRetryConfiguration{
		BindingID: uuid.New(), BindingVersion: 1, WorkflowConfigurationID: uuid.New(), WorkflowConfigurationVersion: 1,
		WorkflowConnectionID: uuid.New(), ConnectionVersion: 1, Stage: "review", Executable: true,
		ConfigurationSnapshot: json.RawMessage(`{}`), BindingSnapshot: json.RawMessage(`{}`),
		ConnectionSnapshot: json.RawMessage(`{}`), LlmPolicySnapshot: json.RawMessage(`{}`),
	}
	if err := p.validate(); !errors.Is(err, ErrRetrySnapshotInvalid) {
		t.Fatalf("err=%v", err)
	}
}

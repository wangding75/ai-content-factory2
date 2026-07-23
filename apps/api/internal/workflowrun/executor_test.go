package workflowrun

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestUnavailableWorkflowExecutor(t *testing.T) {
	x := UnavailableWorkflowExecutor{}
	if err := x.Verify(context.Background(), ExecutionRequest{}); !errors.Is(err, ErrExecutorUnavailable) { t.Fatal(err) }
	if _, err := x.Execute(context.Background(), ExecutionRequest{}); !errors.Is(err, ErrExecutorUnavailable) { t.Fatal(err) }
	if _, err := x.Cancel(context.Background(), ExecutionRequest{}); !errors.Is(err, ErrExecutorUnavailable) { t.Fatal(err) }
}

func TestFakeWorkflowExecutorAndServiceMapping(t *testing.T) {
	s, store, projectID := fixtureService(t)
	connectionID := uuid.New()
	runID := uuid.New()
	now := s.now()
	run := WorkflowRun{ID:runID,RunNumber:"WR-EXEC",ProjectID:projectID,Stage:"review",WorkflowConfigurationID:uuid.New(),TriggerSource:"manual",Status:StatusQueued,ConfigurationSnapshot:json.RawMessage(`{"workflowConnection":{"id":"`+connectionID.String()+`"},"workflowConfiguration":{"defaultParameters":{"token":"hidden"}}}`),InputPayload:json.RawMessage(`{"text":"ok"}`),CreatedAt:now,UpdatedAt:now,Version:1}
	store.runs[runID] = run
	fake := &FakeWorkflowExecutor{ExecuteResult:ExecutionResult{Status:ExecutionSucceeded,Output:json.RawMessage(`{"result":"ok","access_token":"hidden"}`),Metadata:map[string]string{"token":"hidden"}}}
	s.SetWorkflowExecutor(fake)
	updated, err := s.ExecuteRun(context.Background(), runID)
	if err != nil || updated.Status != StatusSucceeded || fake.ExecuteCalls != 1 { t.Fatalf("run=%+v err=%v calls=%d",updated,err,fake.ExecuteCalls) }
	if len(store.events[runID]) != 2 || string(updated.OutputPayload) == "" { t.Fatalf("events=%+v output=%s",store.events[runID],updated.OutputPayload) }
	if string(fake.LastRequest.ConfigurationSnapshot) == "" || string(fake.LastRequest.Parameters) == "" { t.Fatal("missing request") }
}

func TestExecutionFailureIsDomainTransition(t *testing.T) {
	s, store, projectID := fixtureService(t)
	id, connectionID := uuid.New(), uuid.New(); now:=s.now()
	store.runs[id]=WorkflowRun{ID:id,RunNumber:"WR-FAIL",ProjectID:projectID,Stage:"review",WorkflowConfigurationID:uuid.New(),TriggerSource:"manual",Status:StatusQueued,ConfigurationSnapshot:json.RawMessage(`{"workflowConnection":{"id":"`+connectionID.String()+`"}}`),InputPayload:json.RawMessage(`{}`),CreatedAt:now,UpdatedAt:now,Version:1}
	s.SetWorkflowExecutor(&FakeWorkflowExecutor{ExecuteError:ErrExecutionTimeout})
	updated,err:=s.ExecuteRun(context.Background(),id)
	if err!=nil||updated.Status!=StatusFailed||updated.ErrorCode==nil||*updated.ErrorCode!="timeout"{t.Fatalf("run=%+v err=%v",updated,err)}
}

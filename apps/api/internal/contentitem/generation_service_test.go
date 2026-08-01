package contentitem

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type generationSummaryBindings struct {
	binding workflowbinding.ProjectWorkflowBinding
	err     error
}

func (f generationSummaryBindings) GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error) {
	return f.binding, f.err
}

type generationSummaryConfigs struct {
	workflow globalconfig.Workflow
	connection globalconfig.Connection
	workflowErr error
	connectionErr error
}

func (f generationSummaryConfigs) GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error) {
	return f.workflow, f.workflowErr
}

func (f generationSummaryConfigs) GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error) {
	return f.connection, f.connectionErr
}

type generationSummaryRuns struct {
	list workflowrun.RunList
	listErr error
	events []workflowrun.Event
	eventsErr error
	cancel func()
}

func (f *generationSummaryRuns) CreateRunIdempotentForScope(context.Context, string, uuid.UUID, string, string, workflowrun.CreateRunPreparation) (workflowrun.WorkflowRun, error) { return workflowrun.WorkflowRun{}, errors.New("unexpected create") }
func (f *generationSummaryRuns) CreateRunForPreflightToken(context.Context, uuid.UUID, string, string, workflowrun.CreateRunPreparation) (workflowrun.WorkflowRun, error) { return workflowrun.WorkflowRun{}, errors.New("unexpected create") }
func (f *generationSummaryRuns) CreateRunForPreflightTokenIdempotent(context.Context, uuid.UUID, string, string, string, workflowrun.CreateRunTxPreparation) (workflowrun.WorkflowRun, error) { return workflowrun.WorkflowRun{}, errors.New("unexpected create") }
func (f *generationSummaryRuns) ListRuns(context.Context, workflowrun.ListRunsQuery) (workflowrun.RunList, error) { return f.list, f.listErr }
func (f *generationSummaryRuns) ListRunEvents(context.Context, uuid.UUID) ([]workflowrun.Event, error) { if f.cancel != nil { f.cancel() }; return f.events, f.eventsErr }
func (f *generationSummaryRuns) GetRun(context.Context, uuid.UUID) (workflowrun.WorkflowRun, error) { return workflowrun.WorkflowRun{}, errors.New("unexpected get") }
func (f *generationSummaryRuns) AddEvent(context.Context, workflowrun.Event) (workflowrun.Event, error) { return workflowrun.Event{}, errors.New("unexpected event") }

func TestGenerationSummaryErrorPropagation(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	binding := workflowbinding.ProjectWorkflowBinding{ID: uuid.New(), ProjectID: item.Detail.Item.ProjectID, Stage: workflowbinding.StageContentGeneration, WorkflowConfigurationID: uuid.New(), Version: 1}
	configs := generationSummaryConfigs{
		workflow: globalconfig.Workflow{Common: globalconfig.Common{ID: binding.WorkflowConfigurationID, Enabled: true, IntegrationStatus: "verified", Version: 1}, ConnectionID: uuid.New()},
		connection: globalconfig.Connection{Common: globalconfig.Common{ID: uuid.New(), Enabled: true, IntegrationStatus: "verified", Version: 1}},
	}
	ordinaryBinding := errors.New("binding query failed")
	ordinaryConfig := errors.New("configuration query failed")
	ordinaryRuns := errors.New("run query failed")
	ordinaryEvents := errors.New("event query failed")
	candidateCtx, cancelCandidate := context.WithCancel(ctx)
	defer cancelCandidate()
	cases := []struct {
		name string
		bindings generationSummaryBindings
		configs generationSummaryConfigs
		runs *generationSummaryRuns
		callCtx context.Context
		want error
		wantState string
	}{
		{name: "binding ordinary error", bindings: generationSummaryBindings{err: ordinaryBinding}, configs: configs, runs: &generationSummaryRuns{}, callCtx: ctx, want: ordinaryBinding},
		{name: "configuration ordinary error", bindings: generationSummaryBindings{binding: binding}, configs: generationSummaryConfigs{workflowErr: ordinaryConfig}, runs: &generationSummaryRuns{}, callCtx: ctx, want: ordinaryConfig},
		{name: "active latest run ordinary error", bindings: generationSummaryBindings{binding: binding}, configs: configs, runs: &generationSummaryRuns{listErr: ordinaryRuns}, callCtx: ctx, want: ordinaryRuns},
		{name: "event ordinary error", bindings: generationSummaryBindings{binding: binding}, configs: configs, runs: &generationSummaryRuns{list: workflowrun.RunList{Items: []workflowrun.WorkflowRun{spy.run}}, eventsErr: ordinaryEvents}, callCtx: ctx, want: ordinaryEvents},
		{name: "candidate ordinary error", bindings: generationSummaryBindings{binding: binding}, configs: configs, runs: &generationSummaryRuns{list: workflowrun.RunList{Items: []workflowrun.WorkflowRun{spy.run}}, cancel: cancelCandidate}, callCtx: candidateCtx, want: context.Canceled},
		{name: "not found", bindings: generationSummaryBindings{err: workflowbinding.ErrNotFound}, configs: configs, runs: &generationSummaryRuns{}, callCtx: ctx, wantState: "not_configured"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewGenerationService(repo, tc.bindings, tc.configs, tc.runs, "x")
			summary, err := svc.Summary(tc.callCtx, item.Detail.Item.ID)
			if tc.want != nil {
				if !errors.Is(err, tc.want) { t.Fatalf("Summary error = %v, want %v", err, tc.want) }
				if summary.State == "idle" || summary.State == "not_configured" { t.Fatalf("ordinary error returned summary state %q", summary.State) }
				return
			}
			if err != nil || summary.State != tc.wantState { t.Fatalf("Summary = %+v, err = %v", summary, err) }
		})
	}
}

func TestGenerationPreflightRunnableErrorPropagation(t *testing.T) {
	repo,ctx,_,item,spy:=generationFixture(t)
	binding:=workflowbinding.ProjectWorkflowBinding{ID:uuid.New(),ProjectID:item.Detail.Item.ProjectID,Stage:workflowbinding.StageContentGeneration,WorkflowConfigurationID:uuid.New(),Version:1}
	connectionID:=uuid.New()
	ready:=generationSummaryConfigs{workflow:globalconfig.Workflow{Common:globalconfig.Common{ID:binding.WorkflowConfigurationID,Enabled:true,Version:1},ConnectionID:connectionID},connection:globalconfig.Connection{Common:globalconfig.Common{ID:connectionID,Enabled:true,IntegrationStatus:"verified",Version:1}}}
	ordinary:=errors.New("repository unavailable")
	beforeRuns:=count(t,ctx,repo.db,"SELECT count(*) FROM workflow_run_records WHERE project_id=$1",item.Detail.Item.ProjectID)
	beforeEvents:=count(t,ctx,repo.db,"SELECT count(*) FROM workflow_run_events e JOIN workflow_run_records r ON r.id=e.run_id WHERE r.project_id=$1",item.Detail.Item.ProjectID)
	beforeCandidates:=count(t,ctx,repo.db,"SELECT count(*) FROM content_versions WHERE content_item_id=$1 AND source='workflow_generated'",item.Detail.Item.ID)
	for _,tc:=range []struct{name string;bindings generationSummaryBindings;configs generationSummaryConfigs;wantBlocked bool;want error}{
		{name:"binding missing",bindings:generationSummaryBindings{err:workflowbinding.ErrNotFound},configs:ready,wantBlocked:true},
		{name:"configuration missing",bindings:generationSummaryBindings{binding:binding},configs:generationSummaryConfigs{workflowErr:globalconfig.ErrNotFound},wantBlocked:true},
		{name:"connection unavailable",bindings:generationSummaryBindings{binding:binding},configs:generationSummaryConfigs{workflow:ready.workflow,connection:globalconfig.Connection{Common:globalconfig.Common{ID:connectionID,Enabled:false,Version:1}}},wantBlocked:true},
		{name:"binding infrastructure error",bindings:generationSummaryBindings{err:ordinary},configs:ready,want:ordinary},
		{name:"configuration infrastructure error",bindings:generationSummaryBindings{binding:binding},configs:generationSummaryConfigs{workflowErr:ordinary},want:ordinary},
		{name:"connection infrastructure error",bindings:generationSummaryBindings{binding:binding},configs:generationSummaryConfigs{workflow:ready.workflow,connectionErr:ordinary},want:ordinary},
	}{
		t.Run(tc.name,func(t *testing.T){svc:=NewGenerationService(repo,tc.bindings,tc.configs,&generationSummaryRuns{},"x");result,err:=svc.Preflight(ctx,item.Detail.Item.ID,GenerationPreflightRequest{ExpectedCurrentVersionID:item.Detail.CurrentVersion.ID,ExpectedCurrentVersion:item.Detail.CurrentVersion.Version,ActorID:"actor"});if tc.want!=nil{if !errors.Is(err,tc.want){t.Fatalf("error=%v want=%v",err,tc.want)};if result.Passed{t.Fatal("infrastructure error passed")}}else if err!=nil||result.Passed||len(result.Checks)==0{t.Fatalf("result=%+v err=%v",result,err)}})
	}
	if got:=count(t,ctx,repo.db,"SELECT count(*) FROM workflow_run_records WHERE project_id=$1",item.Detail.Item.ProjectID);got!=beforeRuns{t.Fatalf("runs=%d before=%d",got,beforeRuns)}
	if got:=count(t,ctx,repo.db,"SELECT count(*) FROM workflow_run_events e JOIN workflow_run_records r ON r.id=e.run_id WHERE r.project_id=$1",item.Detail.Item.ProjectID);got!=beforeEvents{t.Fatalf("events=%d before=%d",got,beforeEvents)}
	if got:=count(t,ctx,repo.db,"SELECT count(*) FROM content_versions WHERE content_item_id=$1 AND source='workflow_generated'",item.Detail.Item.ID);got!=beforeCandidates{t.Fatalf("candidates=%d before=%d",got,beforeCandidates)}
	_ = spy
}

func TestGenerationSummaryRestoresPersistentStatesAndSafeErrors(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	missing := generationSummaryBindings{err: workflowbinding.ErrNotFound}
	message := "workflow execution failed"
	for _, tc := range []struct { name string; status workflowrun.Status; events []workflowrun.Event; want string; wantError bool }{
		{name:"queued",status:workflowrun.StatusQueued,want:"queued"},
		{name:"running",status:workflowrun.StatusRunning,want:"running"},
		{name:"runtime failed",status:workflowrun.StatusFailed,want:"runtime_failed",wantError:true},
		{name:"output validation",status:workflowrun.StatusSucceeded,events:[]workflowrun.Event{{EventType:workflowrun.EventTypeOutputValidationFailed,Payload:json.RawMessage(`{"message":"invalid generated output"}`)}},want:"output_validation_failed",wantError:true},
		{name:"result consumption",status:workflowrun.StatusSucceeded,events:[]workflowrun.Event{{EventType:workflowrun.EventTypeResultConsumptionFailed,Payload:json.RawMessage(`{"message":"candidate persistence failed"}`)}},want:"result_consumption_failed",wantError:true},
		{name:"non failure",status:workflowrun.StatusSucceeded,want:"idle"},
	} {
		t.Run(tc.name, func(t *testing.T) { run:=spy.run;run.Status=tc.status;if tc.status==workflowrun.StatusFailed{run.ErrorMessage=&message};summary,e:=NewGenerationService(repo,missing,generationSummaryConfigs{},&generationSummaryRuns{list:workflowrun.RunList{Items:[]workflowrun.WorkflowRun{run}},events:tc.events},"x").Summary(ctx,item.Detail.Item.ID);if e!=nil||summary.State!=tc.want||(summary.LatestError!=nil)!=tc.wantError{t.Fatalf("summary=%+v err=%v",summary,e)} })
	}
	summary,e:=NewGenerationService(repo,missing,generationSummaryConfigs{},&generationSummaryRuns{},"x").Summary(ctx,item.Detail.Item.ID)
	if e!=nil||summary.State!="not_configured"||summary.LatestError!=nil{t.Fatalf("empty summary=%+v err=%v",summary,e)}
}

type generationFailureTx struct {
	pgx.Tx
	eventErr error
	commitErr error
	sourceErr error
	eventAttempts int
	commitCalls int
	rollbackCalls int
	innerRolledBack bool
}

type generationErrorRow struct{ err error }
func (r generationErrorRow) Scan(...any) error{return r.err}

func (tx *generationFailureTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if strings.Contains(sql,"SELECT v.content_item_id,i.project_id,v.version FROM content_versions")&&tx.sourceErr!=nil{return generationErrorRow{tx.sourceErr}}
	return tx.Tx.QueryRow(ctx,sql,args...)
}

func (tx *generationFailureTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, "INSERT INTO workflow_run_events") && tx.eventErr != nil {
		tx.eventAttempts++
		return pgconn.CommandTag{}, tx.eventErr
	}
	return tx.Tx.Exec(ctx, sql, args...)
}

func (tx *generationFailureTx) Commit(ctx context.Context) error {
	tx.commitCalls++
	if tx.commitErr != nil {
		tx.innerRolledBack = true
		_ = tx.Tx.Rollback(ctx)
		return tx.commitErr
	}
	return tx.Tx.Commit(ctx)
}

func (tx *generationFailureTx) Rollback(ctx context.Context) error {
	tx.rollbackCalls++
	return tx.Tx.Rollback(ctx)
}

func TestGenerationConsumeResultConsumedEventFailure(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	eventErr := errors.New("result consumed event write failed")
	var mainTx *generationFailureTx
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	svc.begin = func(ctx context.Context) (pgx.Tx, error) {
		tx, err := repo.db.Begin(ctx)
		if err != nil { return nil, err }
		mainTx = &generationFailureTx{Tx: tx, eventErr: eventErr}
		return mainTx, nil
	}
	if err := svc.ConsumeSucceededRun(ctx, spy.run); !errors.Is(err, eventErr) { t.Fatalf("ConsumeSucceededRun error = %v, want %v", err, eventErr) }
	if mainTx == nil || mainTx.eventAttempts != 1 || mainTx.rollbackCalls == 0 { t.Fatalf("main transaction = %+v", mainTx) }
	if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", spy.run.ID) != 0 { t.Fatal("candidate persisted") }
	var current uuid.UUID
	if err := repo.db.QueryRow(ctx, "SELECT current_version_id FROM content_items WHERE id=$1", item.Detail.Item.ID).Scan(&current); err != nil || current != item.Detail.CurrentVersion.ID { t.Fatalf("current_version_id = %s, err = %v", current, err) }
	if count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", spy.run.ID) != 0 { t.Fatal("result_consumed persisted") }
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.events) != 1 || spy.events[0].EventType != workflowrun.EventTypeResultConsumptionFailed { t.Fatalf("independent failure events = %+v", spy.events) }
}

func TestGenerationConsumeCommitFailure(t *testing.T) {
	repo, ctx, _, item, spy := generationFixture(t)
	commitErr := errors.New("main transaction commit failed")
	var mainTx *generationFailureTx
	svc := NewGenerationService(repo, nil, nil, spy, "x")
	svc.begin = func(ctx context.Context) (pgx.Tx, error) {
		tx, err := repo.db.Begin(ctx)
		if err != nil { return nil, err }
		mainTx = &generationFailureTx{Tx: tx, commitErr: commitErr}
		return mainTx, nil
	}
	if err := svc.ConsumeSucceededRun(ctx, spy.run); !errors.Is(err, commitErr) { t.Fatalf("ConsumeSucceededRun error = %v, want %v", err, commitErr) }
	if mainTx == nil || mainTx.commitCalls != 1 || !mainTx.innerRolledBack { t.Fatalf("main transaction = %+v", mainTx) }
	if count(t, ctx, repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", spy.run.ID) != 0 { t.Fatal("candidate persisted after commit failure") }
	var current uuid.UUID
	if err := repo.db.QueryRow(ctx, "SELECT current_version_id FROM content_items WHERE id=$1", item.Detail.Item.ID).Scan(&current); err != nil || current != item.Detail.CurrentVersion.ID { t.Fatalf("current_version_id = %s, err = %v", current, err) }
	if count(t, ctx, repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", spy.run.ID) != 0 { t.Fatal("result_consumed persisted after commit failure") }
	spy.mu.Lock()
	defer spy.mu.Unlock()
	if len(spy.events) != 1 || spy.events[0].EventType != workflowrun.EventTypeResultConsumptionFailed { t.Fatalf("independent failure events = %+v", spy.events) }
}

func TestDecodeGenerationOutputStrictJSON(t *testing.T) {
	valid := json.RawMessage(`{"title":"标题","content":"正文","summary":"摘要","wordCount":2}`)
	if _, err := decodeGenerationOutput(valid); err != nil { t.Fatalf("valid output: %v", err) }
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"title":"标题","content":"正文","summary":"摘要","wordCount":2,"extra":true}`),
		json.RawMessage(`{"title":"标题","content":"正文","summary":"摘要","wordCount":2} {}`),
		json.RawMessage(`{"title":"标题","content":"正文","wordCount":2}`),
		json.RawMessage(`{"title":"标题","content":"","summary":"摘要","wordCount":0}`),
		json.RawMessage(`{"title":"标题","content":"正文","summary":"摘要","wordCount":-1}`),
	} { if _, err := decodeGenerationOutput(raw); !errors.Is(err, ErrValidation) { t.Fatalf("raw=%s err=%v", raw, err) } }
}

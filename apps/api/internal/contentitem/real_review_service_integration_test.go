package contentitem

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type realReviewFixture struct {
	service    *RealReviewService
	runs       *workflowrun.Service
	repo       *PostgresRepository
	ctx        context.Context
	item       CreateResult
	connection uuid.UUID
	workflow   uuid.UUID
}

func newRealReviewFixture(t *testing.T) realReviewFixture {
	t.Helper()
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	repo := NewPostgresRepository(db)
	item := create(t, ctx, repo, f)
	if _, err := db.Exec(ctx, "UPDATE content_versions SET title='待审核章节',content='这是一段已经保存、可以审核的正文。',word_count=17 WHERE id=$1", item.Detail.CurrentVersion.ID); err != nil {
		t.Fatal(err)
	}
	item.Detail, _ = repo.GetByID(ctx, item.Detail.Item.ID)
	connectionID, workflowID := uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,integration_status,enabled) VALUES($1,$2,'n8n','http://review-fixture','api_key',5,'{}','connected',true)", connectionID, "review-"+connectionID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version,default_parameters,integration_status,enabled) VALUES($1,$2,$3,'[\"review\"]','{}','review.input.v1','review.output.v1',$4,'connected',true)", workflowID, "review-"+workflowID.String(), connectionID, json.RawMessage(`{"reviewDimensions":["compliance","factual_consistency","language_quality","structural_logic","character_consistency"]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO project_workflow_bindings(id,project_id,stage,workflow_configuration_id) VALUES($1,$2,'review',$3)", uuid.New(), item.Detail.Item.ProjectID, workflowID); err != nil {
		t.Fatal(err)
	}
	configs, err := globalconfig.NewService(db, "real-review-integration-key")
	if err != nil {
		t.Fatal(err)
	}
	runs := workflowrun.NewService(
		workflowrun.NewPostgresRepository(db), project.NewPostgresRepository(db),
		workflowbinding.NewPostgresRepository(db), configs, configs,
	)
	service := NewRealReviewService(repo, workflowbinding.NewPostgresRepository(db), configs, runs, "real-review-token-secret")
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE id=$1", workflowID)
		_, _ = db.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", connectionID)
	})
	return realReviewFixture{service: service, runs: runs, repo: repo, ctx: ctx, item: item, connection: connectionID, workflow: workflowID}
}

func (f realReviewFixture) preflight(t *testing.T) ReviewPreflightResult {
	t.Helper()
	detail, err := f.repo.GetByID(f.ctx, f.item.Detail.Item.ID)
	if err != nil {
		t.Fatal(err)
	}
	result, err := f.service.Preflight(f.ctx, f.item.Detail.CurrentVersion.ID, ReviewPreflightRequest{
		SourceContentVersionVersion: detail.CurrentVersion.Version,
		OptionalInstructions: stringPointer("重点检查人物一致性"), ActorID: "reviewer",
	})
	if err != nil || result.Status != "passed" || result.PreflightToken == nil || result.ConfigurationSummary == nil {
		t.Fatalf("preflight=%+v err=%v", result, err)
	}
	return result
}

func stringPointer(value string) *string {
	return &value
}

func TestRealReviewPreflightCreateConsumeSummaryAndIssueDisposition(t *testing.T) {
	f := newRealReviewFixture(t)
	beforeRuns := count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE subject_id=$1", f.item.Detail.CurrentVersion.ID)
	preflight := f.preflight(t)
	if count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE subject_id=$1", f.item.Detail.CurrentVersion.ID) != beforeRuns ||
		count(t, f.ctx, f.repo.db, "SELECT count(*) FROM review_reports WHERE content_version_id=$1", f.item.Detail.CurrentVersion.ID) != 0 {
		t.Fatal("preflight created durable review facts")
	}
	run, err := f.service.CreateRun(f.ctx, f.item.Detail.CurrentVersion.ID, "reviewer", *preflight.PreflightToken, "create-review")
	if err != nil {
		t.Fatal(err)
	}
	if run.Stage != "review" || run.Status != workflowrun.StatusQueued || run.SubjectType == nil || *run.SubjectType != "content_version" ||
		run.SubjectID == nil || *run.SubjectID != f.item.Detail.CurrentVersion.ID {
		t.Fatalf("run=%+v", run)
	}
	var input ReviewRuntimeInputV1
	if json.Unmarshal(run.InputPayload, &input) != nil || input.SchemaVersion != "review.input.v1" ||
		input.WorkflowRunID != run.ID || input.SourceContentVersionID != f.item.Detail.CurrentVersion.ID ||
		input.SourceContentVersionVersion != f.item.Detail.CurrentVersion.Version+1 {
		t.Fatalf("input=%+v", input)
	}
	replay, err := f.service.CreateRun(f.ctx, f.item.Detail.CurrentVersion.ID, "reviewer", *preflight.PreflightToken, "create-review")
	if err != nil || replay.ID != run.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	if _, err = f.service.CreateRun(f.ctx, f.item.Detail.CurrentVersion.ID, "reviewer", *preflight.PreflightToken, "other-key"); !errors.Is(err, ErrReviewTokenConsumed) {
		t.Fatalf("token reuse error=%v", err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload=$1,started_at=NOW(),finished_at=NOW(),updated_at=NOW(),version=2 WHERE id=$2", json.RawMessage(validReviewOutput), run.ID); err != nil {
		t.Fatal(err)
	}
	run, err = f.runs.GetRun(f.ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.service.ConsumeSucceededRun(f.ctx, run); err != nil {
		t.Fatal(err)
	}
	if count(t, f.ctx, f.repo.db, "SELECT count(*) FROM review_reports WHERE workflow_run_id=$1", run.ID) != 1 ||
		count(t, f.ctx, f.repo.db, "SELECT count(*) FROM review_findings f JOIN review_reports r ON r.id=f.review_id WHERE r.workflow_run_id=$1", run.ID) != 1 ||
		count(t, f.ctx, f.repo.db, "SELECT count(*) FROM review_recommendations x JOIN review_reports r ON r.id=x.review_id WHERE r.workflow_run_id=$1", run.ID) != 1 ||
		count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", run.ID) != 1 {
		t.Fatal("review result tree was not consumed atomically")
	}
	if err = f.service.ConsumeSucceededRun(f.ctx, run); err != nil {
		t.Fatal(err)
	}
	summary, err := f.service.Summary(f.ctx, f.item.Detail.Item.ID)
	if err != nil || summary.State != "review_ready" || summary.LatestReport == nil || summary.IssueSummary == nil || summary.IssueSummary.Total != 1 {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
	result, err := f.service.realReviewResultByRun(f.ctx, f.repo.db, run, 4)
	if err != nil {
		t.Fatal(err)
	}
	issue := result.Issues[0]
	ignored, err := f.service.UpdateIssue(f.ctx, result.Report.ID, issue.ID, ReviewIssueUpdateRequest{
		Disposition: "ignored", ExpectedVersion: 1, IdempotencyKey: "ignore-issue", ActorID: "reviewer",
	})
	if err != nil || ignored.Disposition != "ignored" || ignored.Version != 2 || ignored.IgnoredAt == nil {
		t.Fatalf("ignored=%+v err=%v", ignored, err)
	}
	ignoredReplay, err := f.service.UpdateIssue(f.ctx, result.Report.ID, issue.ID, ReviewIssueUpdateRequest{
		Disposition: "ignored", ExpectedVersion: 1, IdempotencyKey: "ignore-issue", ActorID: "reviewer",
	})
	if err != nil || ignoredReplay.Version != 2 {
		t.Fatalf("issue replay=%+v err=%v", ignoredReplay, err)
	}
	if _, err = f.service.UpdateIssue(f.ctx, result.Report.ID, issue.ID, ReviewIssueUpdateRequest{
		Disposition: "open", ExpectedVersion: 1, IdempotencyKey: "stale-issue", ActorID: "reviewer",
	}); !errors.Is(err, ErrReviewIssueVersion) {
		t.Fatalf("stale issue error=%v", err)
	}
	detail, handled, err := f.service.GetRealReview(f.ctx, result.Report.ID)
	if err != nil || !handled || detail.Report.SourceContentVersionID != f.item.Detail.CurrentVersion.ID || len(detail.Issues) != 1 {
		t.Fatalf("detail=%+v handled=%v err=%v", detail, handled, err)
	}
	history, err := f.service.ListReviewHistory(f.ctx, f.item.Detail.Item.ID, 20, 0)
	if err != nil || history.Total != 1 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
}

func TestRealReviewInvalidOutputAndConsumptionRetryBoundaries(t *testing.T) {
	f := newRealReviewFixture(t)
	preflight := f.preflight(t)
	run, err := f.service.CreateRun(f.ctx, f.item.Detail.CurrentVersion.ID, "reviewer", *preflight.PreflightToken, "invalid-output")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload='{\"schemaVersion\":\"review.output.v1\",\"unknown\":true}',started_at=NOW(),finished_at=NOW(),updated_at=NOW(),version=2 WHERE id=$1", run.ID); err != nil {
		t.Fatal(err)
	}
	run, _ = f.runs.GetRun(f.ctx, run.ID)
	if err = f.service.ConsumeSucceededRun(f.ctx, run); !errors.Is(err, ErrReviewOutputInvalid) {
		t.Fatalf("invalid output error=%v", err)
	}
	if count(t, f.ctx, f.repo.db, "SELECT count(*) FROM review_reports WHERE workflow_run_id=$1", run.ID) != 0 ||
		count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='output_validation_failed'", run.ID) != 1 {
		t.Fatal("invalid output created report or missed failure event")
	}
	retried, err := f.runs.RetryRun(f.ctx, workflowrun.RetryCommand{RunID: run.ID, ExpectedVersion: 2, IdempotencyKey: "runtime-retry"})
	if err != nil || retried.RetryOfRunID == nil || *retried.RetryOfRunID != run.ID {
		t.Fatalf("runtime retry=%+v err=%v", retried, err)
	}
	if _, err = f.runs.RetryRun(f.ctx, workflowrun.RetryCommand{RunID: run.ID, ExpectedVersion: 2, InputOverride: json.RawMessage(`{"override":true}`), IdempotencyKey: "override"}); !errors.Is(err, workflowrun.ErrValidation) {
		t.Fatalf("review input override error=%v", err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='cancelled',started_at=NOW(),cancelled_at=NOW(),updated_at=NOW(),version=2 WHERE id=$1", retried.ID); err != nil {
		t.Fatal(err)
	}
	secondPreflight := f.preflight(t)
	consumptionRun, err := f.service.CreateRun(f.ctx, f.item.Detail.CurrentVersion.ID, "reviewer", *secondPreflight.PreflightToken, "consumption-retry")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload=$1,started_at=NOW(),finished_at=NOW(),updated_at=NOW(),version=2 WHERE id=$2", json.RawMessage(validReviewOutput), consumptionRun.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "INSERT INTO workflow_run_events(id,run_id,event_type,status,payload) VALUES($1,$2,'result_consumption_failed','succeeded','{}')", uuid.New(), consumptionRun.ID); err != nil {
		t.Fatal(err)
	}
	beforeRuns := count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE project_id=$1", f.item.Detail.Item.ProjectID)
	result, err := f.service.RetryResultConsumption(f.ctx, consumptionRun.ID, ReviewResultConsumptionRetryRequest{ExpectedRunVersion: 2, IdempotencyKey: "consume-retry"})
	if err != nil || result.Report.WorkflowRunID != consumptionRun.ID {
		t.Fatalf("consumption retry=%+v err=%v", result, err)
	}
	if count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE project_id=$1", f.item.Detail.Item.ProjectID) != beforeRuns ||
		count(t, f.ctx, f.repo.db, "SELECT count(*) FROM review_reports WHERE workflow_run_id=$1", consumptionRun.ID) != 1 {
		t.Fatal("consumption retry called runtime, created a run, or missed the report")
	}
	replay, err := f.service.RetryResultConsumption(f.ctx, consumptionRun.ID, ReviewResultConsumptionRetryRequest{ExpectedRunVersion: 2, IdempotencyKey: "consume-retry"})
	if err != nil || replay.Report.ID != result.Report.ID || len(replay.Issues) != len(result.Issues) {
		t.Fatalf("consumption retry replay=%+v err=%v", replay, err)
	}
}

func TestRealReviewSummaryRestoresAllEightStatesAndFactPriority(t *testing.T) {
	f := newRealReviewFixture(t)
	assertState := func(want string) ContentReviewSummary {
		t.Helper()
		summary, err := f.service.Summary(f.ctx, f.item.Detail.Item.ID)
		if err != nil || summary.State != want {
			t.Fatalf("summary=%+v err=%v want=%s", summary, err, want)
		}
		return summary
	}
	assertState("idle")
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_configurations SET enabled=false WHERE id=$1", f.workflow); err != nil {
		t.Fatal(err)
	}
	assertState("not_configured")
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_configurations SET enabled=true WHERE id=$1", f.workflow); err != nil {
		t.Fatal(err)
	}
	createRun := func(key string) workflowrun.WorkflowRun {
		t.Helper()
		preflight := f.preflight(t)
		run, err := f.service.CreateRun(f.ctx, f.item.Detail.CurrentVersion.ID, "reviewer", *preflight.PreflightToken, key)
		if err != nil {
			t.Fatal(err)
		}
		return run
	}
	queued := createRun("summary-queued")
	assertState("queued")
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_configurations SET enabled=false WHERE id=$1", f.workflow); err != nil {
		t.Fatal(err)
	}
	assertState("queued")
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_configurations SET enabled=true WHERE id=$1", f.workflow); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='running',started_at=NOW(),updated_at=NOW(),version=2 WHERE id=$1", queued.ID); err != nil {
		t.Fatal(err)
	}
	assertState("running")
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='failed',error_code='runtime_failed',error_message='workflow execution failed',error_details='{}',finished_at=NOW(),updated_at=NOW(),version=3 WHERE id=$1", queued.ID); err != nil {
		t.Fatal(err)
	}
	assertState("runtime_failed")
	invalid := createRun("summary-invalid")
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload='{\"schemaVersion\":\"review.output.v1\"}',started_at=NOW(),finished_at=NOW(),updated_at=NOW(),version=2 WHERE id=$1", invalid.ID); err != nil {
		t.Fatal(err)
	}
	invalid, _ = f.runs.GetRun(f.ctx, invalid.ID)
	if err := f.service.ConsumeSucceededRun(f.ctx, invalid); !errors.Is(err, ErrReviewOutputInvalid) {
		t.Fatal(err)
	}
	assertState("output_validation_failed")
	consumption := createRun("summary-consumption")
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload=$1,started_at=NOW(),finished_at=NOW(),updated_at=NOW(),version=2 WHERE id=$2", json.RawMessage(validReviewOutput), consumption.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.db.Exec(f.ctx, "INSERT INTO workflow_run_events(id,run_id,event_type,status,payload) VALUES($1,$2,'result_consumption_failed','succeeded','{}')", uuid.New(), consumption.ID); err != nil {
		t.Fatal(err)
	}
	assertState("result_consumption_failed")
	if _, err := f.service.RetryResultConsumption(f.ctx, consumption.ID, ReviewResultConsumptionRetryRequest{ExpectedRunVersion: 2, IdempotencyKey: "summary-consume"}); err != nil {
		t.Fatal(err)
	}
	assertState("review_ready")
}

func TestRealReviewConcurrentCreateAllowsOnlyOneActiveRun(t *testing.T) {
	f := newRealReviewFixture(t)
	first := f.preflight(t)
	second := f.preflight(t)
	tokens := []string{*first.PreflightToken, *second.PreflightToken}
	start := make(chan struct{})
	var wait sync.WaitGroup
	runs := make([]workflowrun.WorkflowRun, 2)
	errs := make([]error, 2)
	for index := range tokens {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			runs[index], errs[index] = f.service.CreateRun(f.ctx, f.item.Detail.CurrentVersion.ID, "reviewer", tokens[index], "concurrent-"+string(rune('a'+index)))
		}(index)
	}
	close(start)
	wait.Wait()
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("runs=%+v errors=%v", runs, errs)
	}
	if count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE subject_id=$1 AND stage='review' AND status IN ('queued','running')", f.item.Detail.CurrentVersion.ID) != 1 {
		t.Fatal("more than one active review run was persisted")
	}
}

func TestRealReviewContentItemStatusUsesImmutableSourceVersion(t *testing.T) {
	t.Run("passed current source marks reviewed", func(t *testing.T) {
		f := newRealReviewFixture(t)
		preflight := f.preflight(t)
		run, err := f.service.CreateRun(f.ctx, f.item.Detail.CurrentVersion.ID, "reviewer", *preflight.PreflightToken, "passed-current")
		if err != nil {
			t.Fatal(err)
		}
		passed := json.RawMessage(`{"schemaVersion":"review.output.v1","conclusion":"passed","summary":"审核通过","passedRuleCount":9,"issues":[],"recommendations":[]}`)
		if _, err = f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload=$1,started_at=NOW(),finished_at=NOW(),updated_at=NOW(),version=2 WHERE id=$2", passed, run.ID); err != nil {
			t.Fatal(err)
		}
		run, _ = f.runs.GetRun(f.ctx, run.ID)
		if err = f.service.ConsumeSucceededRun(f.ctx, run); err != nil {
			t.Fatal(err)
		}
		var status string
		var reviewed bool
		if err = f.repo.db.QueryRow(f.ctx, "SELECT status,reviewed_at IS NOT NULL FROM content_items WHERE id=$1", f.item.Detail.Item.ID).Scan(&status, &reviewed); err != nil {
			t.Fatal(err)
		}
		if status != "reviewed" || !reviewed {
			t.Fatalf("status=%s reviewed=%v", status, reviewed)
		}
	})
	t.Run("non-current source does not change item", func(t *testing.T) {
		f := newRealReviewFixture(t)
		sourceID := f.item.Detail.CurrentVersion.ID
		preflight := f.preflight(t)
		run, err := f.service.CreateRun(f.ctx, sourceID, "reviewer", *preflight.PreflightToken, "passed-historical")
		if err != nil {
			t.Fatal(err)
		}
		newVersionID := uuid.New()
		tx, err := f.repo.db.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(f.ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,word_count,source,status,version) VALUES($1,$2,2,'新版本','新的当前正文',6,'manual_created','editable_draft',1)", newVersionID, f.item.Detail.Item.ID); err == nil {
			_, err = tx.Exec(f.ctx, "UPDATE content_items SET current_version_id=$1,status='draft',reviewed_at=NULL,version=version+1,updated_at=NOW() WHERE id=$2", newVersionID, f.item.Detail.Item.ID)
		}
		if err == nil {
			err = tx.Commit(f.ctx)
		} else {
			_ = tx.Rollback(f.ctx)
		}
		if err != nil {
			t.Fatal(err)
		}
		passed := json.RawMessage(`{"schemaVersion":"review.output.v1","conclusion":"passed","summary":"历史版本通过","passedRuleCount":9,"issues":[],"recommendations":[]}`)
		if _, err = f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload=$1,started_at=NOW(),finished_at=NOW(),updated_at=NOW(),version=2 WHERE id=$2", passed, run.ID); err != nil {
			t.Fatal(err)
		}
		run, _ = f.runs.GetRun(f.ctx, run.ID)
		if err = f.service.ConsumeSucceededRun(f.ctx, run); err != nil {
			t.Fatal(err)
		}
		var currentID uuid.UUID
		var status string
		var reviewed bool
		if err = f.repo.db.QueryRow(f.ctx, "SELECT current_version_id,status,reviewed_at IS NOT NULL FROM content_items WHERE id=$1", f.item.Detail.Item.ID).Scan(&currentID, &status, &reviewed); err != nil {
			t.Fatal(err)
		}
		if currentID != newVersionID || status != "draft" || reviewed {
			t.Fatalf("current=%s status=%s reviewed=%v", currentID, status, reviewed)
		}
	})
}

func TestP0ReviewWithoutWorkflowRunRemainsReadable(t *testing.T) {
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	repo := NewPostgresRepository(db)
	item := create(t, ctx, repo, f)
	reviewID := uuid.New()
	if _, err := db.Exec(ctx, "UPDATE content_versions SET status='frozen',frozen_at=NOW(),version=version+1 WHERE id=$1", item.Detail.CurrentVersion.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO review_reports(id,project_id,content_item_id,content_version_id,workflow_run_id,provider_key,status,conclusion,score,summary) VALUES($1,$2,$3,$4,NULL,'mock','completed','revise',70,'历史 Mock 审核')", reviewID, item.Detail.Item.ProjectID, item.Detail.Item.ID, item.Detail.CurrentVersion.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO review_findings(id,review_id,category,severity,title,description,sort_order) VALUES($1,$2,'pacing','medium','节奏问题','历史问题',0)", uuid.New(), reviewID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO review_recommendations(id,review_id,priority,title,description,sort_order) VALUES($1,$2,'medium','调整节奏','历史建议',0)", uuid.New(), reviewID); err != nil {
		t.Fatal(err)
	}
	detail, err := repo.GetReview(ctx, reviewID)
	if err != nil || detail.Review.ID != reviewID || detail.WorkflowRun.ID != uuid.Nil ||
		len(detail.Findings) != 1 || len(detail.Recommendations) != 1 {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
	list, err := repo.ListReviews(ctx, item.Detail.Item.ID, 20, 0)
	if err != nil || list.Total != 1 || list.Items[0].WorkflowRunID != uuid.Nil {
		t.Fatalf("list=%+v err=%v", list, err)
	}
}

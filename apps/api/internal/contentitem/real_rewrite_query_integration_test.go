package contentitem

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

func setRewriteRunState(t *testing.T, f realRewriteFixture, runID uuid.UUID, status workflowrun.Status, output json.RawMessage) workflowrun.WorkflowRun {
	t.Helper()
	now := time.Now().UTC()
	var errorCode, errorMessage any
	if status == workflowrun.StatusFailed {
		errorCode, errorMessage = "runtime_failed", "重写执行失败"
	}
	_, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_run_records SET status=$1::varchar,output_payload=$2,error_code=$3,error_message=$4,error_details=CASE WHEN $1::varchar='failed' THEN '{}'::jsonb ELSE NULL END,started_at=CASE WHEN $1::varchar='queued' THEN NULL ELSE $5::timestamptz END,finished_at=CASE WHEN $1::varchar IN ('failed','cancelled','succeeded') THEN $5::timestamptz ELSE NULL END,cancelled_at=CASE WHEN $1::varchar='cancelled' THEN $5::timestamptz ELSE NULL END,updated_at=$5::timestamptz,version=version+1 WHERE id=$6", status, output, errorCode, errorMessage, now, runID)
	if err != nil {
		t.Fatal(err)
	}
	run, err := f.runs.GetRun(f.ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestRealRewriteSummaryEightStatesAndPersistedPriority(t *testing.T) {
	f := newRealRewriteFixture(t)
	summary, err := f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "idle" || !summary.CanStartRewrite {
		t.Fatalf("idle summary=%+v err=%v", summary, err)
	}
	preflight := f.preflight(t, "states")
	run, replay, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, "states")
	if err != nil || replay {
		t.Fatalf("create run=%+v replay=%v err=%v", run, replay, err)
	}
	summary, err = f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "queued" || summary.ActiveRun == nil || summary.ActiveRun.ID != run.ID {
		t.Fatalf("queued summary=%+v err=%v", summary, err)
	}
	run = setRewriteRunState(t, f, run.ID, workflowrun.StatusRunning, nil)
	summary, err = f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "running" {
		t.Fatalf("running summary=%+v err=%v", summary, err)
	}
	run = setRewriteRunState(t, f, run.ID, workflowrun.StatusFailed, nil)
	summary, err = f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "runtime_failed" || summary.LatestError == nil {
		t.Fatalf("runtime failed summary=%+v err=%v", summary, err)
	}
	validationRun := succeededRewriteRun(t, f, json.RawMessage(`{"schemaVersion":"rewrite.output.v1"}`))
	if err = f.service.ConsumeSucceededRun(f.ctx, validationRun); !errors.Is(err, ErrRewriteOutputInvalid) {
		t.Fatalf("validation consumption err=%v", err)
	}
	summary, err = f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "output_validation_failed" || summary.LatestError == nil {
		t.Fatalf("validation summary=%+v err=%v", summary, err)
	}
	consumptionRun := succeededRewriteRun(t, f, validRewriteOutput(f.issue.ID))
	if err = f.service.recordRewriteFailure(f.ctx, consumptionRun.ID, workflowrun.EventTypeResultConsumptionFailed); err != nil {
		t.Fatal(err)
	}
	summary, err = f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "result_consumption_failed" || summary.LatestError == nil {
		t.Fatalf("consumption summary=%+v err=%v", summary, err)
	}
	candidateRun := succeededRewriteRun(t, f, validRewriteOutput(f.issue.ID))
	candidate, err := f.service.ConsumeRewriteResult(f.ctx, candidateRun)
	if err != nil {
		t.Fatal(err)
	}
	summary, err = f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "candidate_ready" || summary.CandidateVersion == nil ||
		summary.CandidateVersion.ID != candidate.ID || !summary.CanSetCurrent {
		t.Fatalf("candidate summary=%+v err=%v", summary, err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "DELETE FROM project_workflow_bindings WHERE id=$1", f.binding); err != nil {
		t.Fatal(err)
	}
	summary, err = f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "candidate_ready" || summary.ConfigurationSummary != nil {
		t.Fatalf("configuration masking summary=%+v err=%v", summary, err)
	}

	notConfigured := newRealRewriteFixture(t)
	if _, err = notConfigured.repo.db.Exec(notConfigured.ctx, "DELETE FROM project_workflow_bindings WHERE id=$1", notConfigured.binding); err != nil {
		t.Fatal(err)
	}
	summary, err = notConfigured.service.Summary(notConfigured.ctx, notConfigured.report.ID)
	if err != nil || summary.State != "not_configured" {
		t.Fatalf("not configured summary=%+v err=%v", summary, err)
	}
}

func TestRealRewriteHistoryResultAndSetCurrentCAS(t *testing.T) {
	f := newRealRewriteFixture(t)
	run := succeededRewriteRun(t, f, validRewriteOutput(f.issue.ID))
	candidate, err := f.service.ConsumeRewriteResult(f.ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	history, err := f.service.History(f.ctx, f.item.Detail.Item.ID, 1, 0)
	if err != nil || history.Total != 1 || len(history.Items) != 1 ||
		history.Items[0].State != "candidate_ready" ||
		history.Items[0].CandidateVersion == nil ||
		history.Items[0].CandidateVersion.ID != candidate.ID ||
		history.Items[0].SourceContentVersionSummary.ID != f.report.SourceContentVersionID ||
		history.Items[0].SourceContentVersionSummary.VersionNo < 1 {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	result, err := f.service.Result(f.ctx, run.ID)
	if err != nil || result.CandidateVersion.ID != candidate.ID ||
		result.ReviewReportSnapshot.ReviewReportID != f.report.ID ||
		result.SourceContentVersionSummary.ID != f.report.SourceContentVersionID ||
		len(result.SelectedIssueSummary.Items) != 1 ||
		result.Output.AddressedIssues[0].ReviewIssueID != f.issue.ID ||
		!result.CanSetCurrent || result.CandidateIsCurrent {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	var reportStatus, issueDisposition string
	var issueVersion int
	if err = f.repo.db.QueryRow(f.ctx, "SELECT status FROM review_reports WHERE id=$1", f.report.ID).Scan(&reportStatus); err != nil {
		t.Fatal(err)
	}
	if err = f.repo.db.QueryRow(f.ctx, "SELECT disposition,version FROM review_findings WHERE id=$1", f.issue.ID).Scan(&issueDisposition, &issueVersion); err != nil {
		t.Fatal(err)
	}
	generation := NewGenerationService(f.repo, nil, nil, nil, "set-current")
	currentDetail, err := f.repo.GetByID(f.ctx, f.item.Detail.Item.ID)
	if err != nil {
		t.Fatal(err)
	}
	request := SetCurrentRequest{
		CandidateVersionID:       candidate.ID,
		ExpectedCurrentVersionID: currentDetail.CurrentVersion.ID,
		ExpectedCurrentVersion:   currentDetail.CurrentVersion.Version,
		IdempotencyKey:           "rewrite-set-current",
	}
	first, err := generation.SetCurrent(f.ctx, f.item.Detail.Item.ID, request)
	if err != nil || first.CurrentVersion.ID != candidate.ID ||
		first.Item.Status != "draft" || first.Item.ReviewedAt != nil {
		t.Fatalf("set current=%+v err=%v", first, err)
	}
	replay, err := generation.SetCurrent(f.ctx, f.item.Detail.Item.ID, request)
	if err != nil || replay.CurrentVersion.ID != first.CurrentVersion.ID {
		t.Fatalf("set replay=%+v err=%v", replay, err)
	}
	summary, err := f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "candidate_ready" || !summary.CandidateIsCurrent || summary.CanSetCurrent {
		t.Fatalf("current summary=%+v err=%v", summary, err)
	}
	var reportStatusAfter, issueDispositionAfter string
	var issueVersionAfter int
	if err = f.repo.db.QueryRow(f.ctx, "SELECT status FROM review_reports WHERE id=$1", f.report.ID).Scan(&reportStatusAfter); err != nil {
		t.Fatal(err)
	}
	if err = f.repo.db.QueryRow(f.ctx, "SELECT disposition,version FROM review_findings WHERE id=$1", f.issue.ID).Scan(&issueDispositionAfter, &issueVersionAfter); err != nil {
		t.Fatal(err)
	}
	if reportStatusAfter != reportStatus || issueDispositionAfter != issueDisposition || issueVersionAfter != issueVersion {
		t.Fatalf("report/issue mutated: %s/%s %s/%s %d/%d", reportStatus, reportStatusAfter, issueDisposition, issueDispositionAfter, issueVersion, issueVersionAfter)
	}
	noOp := request
	noOp.IdempotencyKey = "rewrite-set-current-noop"
	if _, err = generation.SetCurrent(f.ctx, f.item.Detail.Item.ID, noOp); err != nil {
		t.Fatalf("already current no-op err=%v", err)
	}
}

func TestRealRewriteConsumptionRetryIdempotentAndConcurrent(t *testing.T) {
	f := newRealRewriteFixture(t)
	executor := &workflowrun.FakeWorkflowExecutor{}
	f.runs.SetWorkflowExecutor(executor)
	run := succeededRewriteRun(t, f, validRewriteOutput(f.issue.ID))
	if err := f.service.recordRewriteFailure(f.ctx, run.ID, workflowrun.EventTypeResultConsumptionFailed); err != nil {
		t.Fatal(err)
	}
	beforeRuns := count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID)
	results := make([]RewriteResult, 2)
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for i := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errs[index] = f.service.RetryResultConsumption(f.ctx, run.ID, RetryRewriteConsumptionRequest{
				ExpectedRunVersion: run.Version, IdempotencyKey: "consume-retry-" + uuid.NewString(),
			})
		}(i)
	}
	wait.Wait()
	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("retry %d err=%v", i, errs[i])
		}
	}
	if results[0].CandidateVersion.ID != results[1].CandidateVersion.ID ||
		count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM content_versions WHERE source_workflow_run_id=$1 AND source='workflow_rewrite'", run.ID) != 1 ||
		count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", run.ID) != 1 ||
		count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID) != beforeRuns {
		t.Fatalf("concurrent results=%+v", results)
	}
	if executor.ExecuteCalls != 0 {
		t.Fatalf("consumption retry called runtime %d times", executor.ExecuteCalls)
	}
	request := RetryRewriteConsumptionRequest{ExpectedRunVersion: run.Version, IdempotencyKey: "consume-replay"}
	first, err := f.service.RetryResultConsumption(f.ctx, run.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.service.RetryResultConsumption(f.ctx, run.ID, request)
	if err != nil || replay.CandidateVersion.ID != first.CandidateVersion.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	conflict := request
	conflict.ExpectedRunVersion++
	if _, err = f.service.RetryResultConsumption(f.ctx, run.ID, conflict); !errors.Is(err, workflowrun.ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict=%v", err)
	}
	if _, err = f.service.RetryResultConsumption(f.ctx, run.ID, RetryRewriteConsumptionRequest{
		ExpectedRunVersion: run.Version + 1, IdempotencyKey: "consume-version-conflict",
	}); !errors.Is(err, workflowrun.ErrVersionConflict) {
		t.Fatalf("version conflict=%v", err)
	}
}

func TestRealRewriteConsumptionRetryRejectsIllegalRuntimeFacts(t *testing.T) {
	cases := []struct {
		name      string
		status    workflowrun.Status
		eventType string
	}{
		{name: "queued", status: workflowrun.StatusQueued},
		{name: "running", status: workflowrun.StatusRunning},
		{name: "runtime failed", status: workflowrun.StatusFailed},
		{name: "cancelled", status: workflowrun.StatusCancelled},
		{name: "output validation failed", status: workflowrun.StatusSucceeded, eventType: workflowrun.EventTypeOutputValidationFailed},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newRealRewriteFixture(t)
			preflight := f.preflight(t, test.name)
			run, _, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, uuid.NewString())
			if err != nil {
				t.Fatal(err)
			}
			if test.status != workflowrun.StatusQueued {
				output := json.RawMessage(nil)
				if test.status == workflowrun.StatusSucceeded {
					output = validRewriteOutput(f.issue.ID)
				}
				run = setRewriteRunState(t, f, run.ID, test.status, output)
			}
			if test.eventType != "" {
				if err = f.service.recordRewriteFailure(f.ctx, run.ID, test.eventType); err != nil {
					t.Fatal(err)
				}
			}
			beforeRuns := count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID)
			if _, err = f.service.RetryResultConsumption(f.ctx, run.ID, RetryRewriteConsumptionRequest{
				ExpectedRunVersion: run.Version, IdempotencyKey: "illegal-consumption",
			}); !errors.Is(err, ErrRewriteCandidateNotReady) {
				t.Fatalf("retry err=%v", err)
			}
			if count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID) != beforeRuns ||
				count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM content_versions WHERE source_workflow_run_id=$1 AND source='workflow_rewrite'", run.ID) != 0 {
				t.Fatal("illegal retry created durable result")
			}
		})
	}
}

func TestRealRewriteRuntimeRetryInheritanceReplayAndConcurrentSingleActive(t *testing.T) {
	f := newRealRewriteFixture(t)
	preflight := f.preflight(t, "runtime-retry")
	original, _, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, "runtime-original")
	if err != nil {
		t.Fatal(err)
	}
	original = setRewriteRunState(t, f, original.ID, workflowrun.StatusFailed, nil)
	command := workflowrun.RetryCommand{
		RunID: original.ID, ExpectedVersion: original.Version, Mode: "original_configuration", IdempotencyKey: "runtime-retry",
	}
	retried, replayed, err := f.runs.RetryRunWithReplay(f.ctx, command)
	if err != nil || replayed || retried.RetryOfRunID == nil || *retried.RetryOfRunID != original.ID ||
		retried.Status != workflowrun.StatusQueued || retried.TriggerSource != "retry" ||
		!jsonEqual(retried.ConfigurationSnapshot, original.ConfigurationSnapshot) {
		t.Fatalf("retried=%+v replayed=%v err=%v", retried, replayed, err)
	}
	var originalInput, retryInput RewriteRuntimeInputV1
	if json.Unmarshal(original.InputPayload, &originalInput) != nil || json.Unmarshal(retried.InputPayload, &retryInput) != nil ||
		retryInput.WorkflowRunID != retried.ID || retryInput.CorrelationID != retried.ID.String() ||
		retryInput.SourceContentVersionID != originalInput.SourceContentVersionID ||
		retryInput.ReviewReportID != originalInput.ReviewReportID ||
		len(retryInput.SelectedIssues) != len(originalInput.SelectedIssues) {
		t.Fatalf("original input=%+v retry input=%+v", originalInput, retryInput)
	}
	replay, replayed, err := f.runs.RetryRunWithReplay(f.ctx, command)
	if err != nil || !replayed || replay.ID != retried.ID {
		t.Fatalf("replay=%+v replayed=%v err=%v", replay, replayed, err)
	}

	concurrent := newRealRewriteFixture(t)
	preflight = concurrent.preflight(t, "runtime-concurrent")
	original, _, err = concurrent.service.CreateRun(concurrent.ctx, concurrent.report.ID, "rewriter", *preflight.PreflightToken, "runtime-concurrent-original")
	if err != nil {
		t.Fatal(err)
	}
	original = setRewriteRunState(t, concurrent, original.ID, workflowrun.StatusCancelled, nil)
	var successes int
	var activeConflicts int
	var mutex sync.Mutex
	var group sync.WaitGroup
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, _, retryErr := concurrent.runs.RetryRunWithReplay(concurrent.ctx, workflowrun.RetryCommand{
				RunID: original.ID, ExpectedVersion: original.Version, Mode: "original_configuration", IdempotencyKey: uuid.NewString(),
			})
			mutex.Lock()
			defer mutex.Unlock()
			if retryErr == nil {
				successes++
			} else if errors.Is(retryErr, workflowrun.ErrActiveRewriteRun) {
				activeConflicts++
			} else {
				t.Errorf("unexpected retry error=%v", retryErr)
			}
		}()
	}
	group.Wait()
	if successes != 1 || activeConflicts != 1 ||
		count(t, concurrent.ctx, concurrent.repo.db, "SELECT COUNT(*) FROM workflow_run_records WHERE retry_of_run_id=$1 AND status IN ('queued','running')", original.ID) != 1 ||
		count(t, concurrent.ctx, concurrent.repo.db, "SELECT COUNT(*) FROM workflow_run_events e JOIN workflow_run_records r ON r.id=e.run_id WHERE r.retry_of_run_id=$1", original.ID) != 1 ||
		count(t, concurrent.ctx, concurrent.repo.db, "SELECT COUNT(*) FROM idempotency_records WHERE scope=$1", "workflow-run:retryWorkflowRun:system:"+original.ID.String()) != 1 {
		t.Fatalf("successes=%d activeConflicts=%d", successes, activeConflicts)
	}
}

func TestRealRewriteRuntimeRetryEligibilityMatrix(t *testing.T) {
	cases := []struct {
		name        string
		status      workflowrun.Status
		eventType   string
		consume     bool
		wantAllowed bool
	}{
		{name: "queued", status: workflowrun.StatusQueued},
		{name: "running", status: workflowrun.StatusRunning},
		{name: "ordinary succeeded", status: workflowrun.StatusSucceeded},
		{name: "result consumption failed", status: workflowrun.StatusSucceeded, eventType: workflowrun.EventTypeResultConsumptionFailed},
		{name: "candidate ready", status: workflowrun.StatusSucceeded, consume: true},
		{name: "output validation failed", status: workflowrun.StatusSucceeded, eventType: workflowrun.EventTypeOutputValidationFailed, wantAllowed: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newRealRewriteFixture(t)
			preflight := f.preflight(t, test.name)
			run, _, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, uuid.NewString())
			if err != nil {
				t.Fatal(err)
			}
			output := json.RawMessage(nil)
			if test.status == workflowrun.StatusSucceeded {
				output = validRewriteOutput(f.issue.ID)
			}
			if test.status != workflowrun.StatusQueued {
				run = setRewriteRunState(t, f, run.ID, test.status, output)
			}
			if test.eventType != "" {
				if err = f.service.recordRewriteFailure(f.ctx, run.ID, test.eventType); err != nil {
					t.Fatal(err)
				}
			}
			if test.consume {
				if _, err = f.service.ConsumeRewriteResult(f.ctx, run); err != nil {
					t.Fatal(err)
				}
			}
			beforeRuns := count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID)
			retry, _, retryErr := f.runs.RetryRunWithReplay(f.ctx, workflowrun.RetryCommand{
				RunID: run.ID, ExpectedVersion: run.Version, Mode: "original_configuration", IdempotencyKey: "eligibility",
			})
			if test.wantAllowed {
				if retryErr != nil || retry.RetryOfRunID == nil || *retry.RetryOfRunID != run.ID {
					t.Fatalf("retry=%+v err=%v", retry, retryErr)
				}
				return
			}
			if !errors.Is(retryErr, workflowrun.ErrNotRetryable) {
				t.Fatalf("retry err=%v", retryErr)
			}
			if count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID) != beforeRuns {
				t.Fatal("rejected retry created a run")
			}
		})
	}
	f := newRealRewriteFixture(t)
	preflight := f.preflight(t, "override")
	run, _, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, "override-original")
	if err != nil {
		t.Fatal(err)
	}
	run = setRewriteRunState(t, f, run.ID, workflowrun.StatusFailed, nil)
	if _, _, err = f.runs.RetryRunWithReplay(f.ctx, workflowrun.RetryCommand{
		RunID: run.ID, ExpectedVersion: run.Version, Mode: "original_configuration", InputOverride: json.RawMessage(`{"override":true}`), IdempotencyKey: "override",
	}); !errors.Is(err, workflowrun.ErrValidation) {
		t.Fatalf("input override err=%v", err)
	}
	currentRetry, replayed, err := f.runs.RetryRunWithReplay(f.ctx, workflowrun.RetryCommand{
		RunID: run.ID, ExpectedVersion: run.Version, Mode: "current_configuration", IdempotencyKey: "current-config",
	})
	if err != nil || replayed || currentRetry.RetryOfRunID == nil || *currentRetry.RetryOfRunID != run.ID || currentRetry.RetryMode == nil || *currentRetry.RetryMode != "current_configuration" {
		t.Fatalf("current configuration retry=%+v replayed=%v err=%v", currentRetry, replayed, err)
	}
}

func TestRealRewriteSetCurrentConcurrentCASAndEligibility(t *testing.T) {
	f := newRealRewriteFixture(t)
	runA := succeededRewriteRun(t, f, validRewriteOutput(f.issue.ID))
	candidateA, err := f.service.ConsumeRewriteResult(f.ctx, runA)
	if err != nil {
		t.Fatal(err)
	}
	runB := succeededRewriteRun(t, f, validRewriteOutput(f.issue.ID))
	candidateB, err := f.service.ConsumeRewriteResult(f.ctx, runB)
	if err != nil {
		t.Fatal(err)
	}
	current, err := f.repo.GetByID(f.ctx, f.item.Detail.Item.ID)
	if err != nil {
		t.Fatal(err)
	}
	generation := NewGenerationService(f.repo, nil, nil, nil, "set-current")
	candidates := []uuid.UUID{candidateA.ID, candidateB.ID}
	results := make([]Detail, 2)
	errs := make([]error, 2)
	var wait sync.WaitGroup
	for i := range candidates {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errs[index] = generation.SetCurrent(f.ctx, current.Item.ID, SetCurrentRequest{
				CandidateVersionID:       candidates[index],
				ExpectedCurrentVersionID: current.CurrentVersion.ID,
				ExpectedCurrentVersion:   current.CurrentVersion.Version,
				IdempotencyKey:           "set-current-" + uuid.NewString(),
			})
		}(i)
	}
	wait.Wait()
	successes, conflicts := 0, 0
	for i := range errs {
		if errs[i] == nil {
			successes++
		} else if errors.Is(errs[i], ErrRewriteContentVersionConflict) {
			conflicts++
		} else {
			t.Fatalf("set current %d err=%v", i, errs[i])
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d results=%+v", successes, conflicts, results)
	}
	latest, err := f.repo.GetByID(f.ctx, current.Item.ID)
	if err != nil || (latest.CurrentVersion.ID != candidateA.ID && latest.CurrentVersion.ID != candidateB.ID) ||
		latest.Item.Version != current.Item.Version+1 {
		t.Fatalf("latest=%+v err=%v", latest, err)
	}
	winner := latest.CurrentVersion.ID
	loser := candidateA.ID
	if winner == candidateA.ID {
		loser = candidateB.ID
	}
	firstKey := "set-current-idempotency"
	firstRequest := SetCurrentRequest{
		CandidateVersionID: winner, ExpectedCurrentVersionID: current.CurrentVersion.ID,
		ExpectedCurrentVersion: current.CurrentVersion.Version, IdempotencyKey: firstKey,
	}
	if _, err = generation.SetCurrent(f.ctx, current.Item.ID, firstRequest); err != nil {
		t.Fatalf("already-current first command err=%v", err)
	}
	different := firstRequest
	different.CandidateVersionID = loser
	if _, err = generation.SetCurrent(f.ctx, current.Item.ID, different); !errors.Is(err, ErrRewriteIdempotencyConflict) {
		t.Fatalf("different payload err=%v", err)
	}

	other := newRealRewriteFixture(t)
	otherRun := succeededRewriteRun(t, other, validRewriteOutput(other.issue.ID))
	otherCandidate, err := other.service.ConsumeRewriteResult(other.ctx, otherRun)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = generation.SetCurrent(f.ctx, current.Item.ID, SetCurrentRequest{
		CandidateVersionID: otherCandidate.ID, ExpectedCurrentVersionID: latest.CurrentVersion.ID,
		ExpectedCurrentVersion: latest.CurrentVersion.Version, IdempotencyKey: "cross-item",
	}); !errors.Is(err, ErrRewriteCandidateNotFound) {
		t.Fatalf("cross-item err=%v", err)
	}
}

func jsonEqual(left, right json.RawMessage) bool {
	var leftValue, rightValue any
	return json.Unmarshal(left, &leftValue) == nil &&
		json.Unmarshal(right, &rightValue) == nil &&
		rewriteCommandHash(leftValue) == rewriteCommandHash(rightValue)
}

func TestRealRewriteResultIsReadOnlyForNotReadyRun(t *testing.T) {
	f := newRealRewriteFixture(t)
	preflight := f.preflight(t, "not-ready")
	run, _, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, "not-ready")
	if err != nil {
		t.Fatal(err)
	}
	before := count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM workflow_run_events WHERE run_id=$1", run.ID)
	if _, err = f.service.Result(context.Background(), run.ID); !errors.Is(err, ErrRewriteCandidateNotFound) {
		t.Fatalf("not ready result err=%v", err)
	}
	after := count(t, f.ctx, f.repo.db, "SELECT COUNT(*) FROM workflow_run_events WHERE run_id=$1", run.ID)
	if after != before {
		t.Fatalf("result query mutated events before=%d after=%d", before, after)
	}
}

func TestRealRewriteHistoryStablePaginationAndSummaryFindsActiveBeyondPage(t *testing.T) {
	f := newRealRewriteFixture(t)
	preflight := f.preflight(t, "history-active")
	active, _, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, "history-active")
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 25; index++ {
		runID := uuid.New()
		runNumber := "WR-HISTORY-" + uuid.NewString()[:8]
		createdAt := active.CreatedAt.Add(time.Duration(index+1) * time.Second)
		_, err = f.repo.db.Exec(f.ctx, "INSERT INTO workflow_run_records(id,run_number,project_id,stage,subject_type,subject_id,workflow_configuration_id,trigger_source,status,configuration_snapshot,input_payload,output_payload,error_code,error_message,error_details,started_at,finished_at,created_at,updated_at,version) SELECT $1::uuid,$2,project_id,stage,subject_type,subject_id,workflow_configuration_id,'retry','failed',configuration_snapshot,jsonb_set(jsonb_set(input_payload,'{workflowRunId}',to_jsonb(($1::uuid)::text),false),'{correlationId}',to_jsonb(($1::uuid)::text),false),NULL,'runtime_failed','重写执行失败','{}',$3,$3,$3,$3,2 FROM workflow_run_records WHERE id=$4", runID, runNumber, createdAt, active.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := f.service.History(f.ctx, f.item.Detail.Item.ID, 5, 0)
	if err != nil || first.Total != 26 || len(first.Items) != 5 {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	second, err := f.service.History(f.ctx, f.item.Detail.Item.ID, 5, 5)
	if err != nil || len(second.Items) != 5 {
		t.Fatalf("second page=%+v err=%v", second, err)
	}
	seen := map[uuid.UUID]bool{}
	for _, item := range first.Items {
		seen[item.WorkflowRun.ID] = true
	}
	for _, item := range second.Items {
		if seen[item.WorkflowRun.ID] {
			t.Fatalf("run %s appeared on both pages", item.WorkflowRun.ID)
		}
	}
	for index := 1; index < len(first.Items); index++ {
		previous, current := first.Items[index-1].WorkflowRun, first.Items[index].WorkflowRun
		if previous.CreatedAt.Before(current.CreatedAt) ||
			(previous.CreatedAt.Equal(current.CreatedAt) && previous.ID.String() < current.ID.String()) {
			t.Fatalf("unstable history order: %s then %s", previous.ID, current.ID)
		}
	}
	summary, err := f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "queued" || summary.ActiveRun == nil || summary.ActiveRun.ID != active.ID {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
}

func TestRealRewriteSummaryKeepsFixedSourceAfterCurrentVersionDrift(t *testing.T) {
	f := newRealRewriteFixture(t)
	firstRun := succeededRewriteRun(t, f, validRewriteOutput(f.issue.ID))
	if _, err := f.service.ConsumeRewriteResult(f.ctx, firstRun); err != nil {
		t.Fatal(err)
	}
	run := succeededRewriteRun(t, f, validRewriteOutput(f.issue.ID))
	candidate, err := f.service.ConsumeRewriteResult(f.ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	current, err := f.repo.GetByID(f.ctx, f.item.Detail.Item.ID)
	if err != nil {
		t.Fatal(err)
	}
	driftedID := uuid.New()
	_, err = f.repo.db.Exec(f.ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,word_count,source,status) VALUES($1,$2,$3,'漂移版本','后续保存的正文',7,'manual_created','editable_draft')", driftedID, current.Item.ID, candidate.VersionNo+1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "UPDATE content_items SET current_version_id=$1,version=version+1 WHERE id=$2", driftedID, current.Item.ID); err != nil {
		t.Fatal(err)
	}
	summary, err := f.service.Summary(f.ctx, f.report.ID)
	if err != nil || summary.State != "candidate_ready" || summary.CandidateVersion == nil ||
		summary.CandidateVersion.ID != candidate.ID || summary.CandidateIsCurrent || !summary.CanSetCurrent ||
		summary.SourceContentVersionSummary.ID != f.report.SourceContentVersionID {
		t.Fatalf("summary=%+v err=%v", summary, err)
	}
	result, err := f.service.Result(f.ctx, run.ID)
	if err != nil || result.SourceContentVersionSummary.ID != f.report.SourceContentVersionID ||
		result.CandidateVersion.ID != candidate.ID || !result.CanSetCurrent {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	generation := NewGenerationService(f.repo, nil, nil, nil, "set-current")
	if _, err = generation.SetCurrent(f.ctx, current.Item.ID, SetCurrentRequest{
		CandidateVersionID: candidate.ID, ExpectedCurrentVersionID: current.CurrentVersion.ID,
		ExpectedCurrentVersion: current.CurrentVersion.Version, IdempotencyKey: "stale-expected",
	}); !errors.Is(err, ErrRewriteContentVersionConflict) {
		t.Fatalf("old expected current err=%v", err)
	}
	latest, err := f.repo.GetByID(f.ctx, current.Item.ID)
	if err != nil || latest.CurrentVersion.ID != driftedID {
		t.Fatalf("latest=%+v err=%v", latest, err)
	}
	promoted, err := generation.SetCurrent(f.ctx, current.Item.ID, SetCurrentRequest{
		CandidateVersionID: candidate.ID, ExpectedCurrentVersionID: latest.CurrentVersion.ID,
		ExpectedCurrentVersion: latest.CurrentVersion.Version, IdempotencyKey: "refreshed-expected",
	})
	if err != nil || promoted.CurrentVersion.ID != candidate.ID {
		t.Fatalf("promoted=%+v err=%v", promoted, err)
	}
}

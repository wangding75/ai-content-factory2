package contentitem

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

func succeededRewriteRun(t *testing.T, fixture realRewriteFixture, output json.RawMessage) workflowrun.WorkflowRun {
	t.Helper()
	preflight := fixture.preflight(t, uuid.NewString())
	run, replay, err := fixture.service.CreateRun(
		fixture.ctx, fixture.report.ID, "rewriter", *preflight.PreflightToken, uuid.NewString(),
	)
	if err != nil || replay {
		t.Fatalf("create run=%+v replay=%v err=%v", run, replay, err)
	}
	now := time.Now().UTC()
	if _, err = fixture.repo.db.Exec(fixture.ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload=$1,started_at=$2,finished_at=$2,updated_at=$2,version=version+1 WHERE id=$3", output, now, run.ID); err != nil {
		t.Fatal(err)
	}
	run, err = fixture.runs.GetRun(fixture.ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestRealRewriteConsumesCandidateWithFrozenLineageAndIdempotency(t *testing.T) {
	fixture := newRealRewriteFixture(t)
	run := succeededRewriteRun(t, fixture, validRewriteOutput(fixture.issue.ID))
	currentBefore := fixture.item.Detail.Item.CurrentVersionID
	var reportStatusBefore string
	if err := fixture.repo.db.QueryRow(fixture.ctx, "SELECT status FROM review_reports WHERE id=$1", fixture.report.ID).Scan(&reportStatusBefore); err != nil {
		t.Fatal(err)
	}
	issueDispositionBefore, issueVersionBefore := fixture.issue.Disposition, fixture.issue.Version
	candidate, err := fixture.service.ConsumeRewriteResult(fixture.ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Source != ContentVersionSourceWorkflowRewrite ||
		candidate.Status != ContentVersionStatusEditableDraft || candidate.Version != 1 ||
		candidate.SourceContentVersionID == nil || *candidate.SourceContentVersionID != fixture.report.SourceContentVersionID ||
		candidate.SourceContentVersionVersion == nil || *candidate.SourceContentVersionVersion != fixture.report.SourceContentVersionVersion ||
		candidate.SourceWorkflowRunID == nil || *candidate.SourceWorkflowRunID != run.ID ||
		candidate.Title != "重写标题" || candidate.Content != "重写后的完整正文。" ||
		candidate.Summary == nil || *candidate.Summary != "修复选中的问题。" ||
		candidate.WordCount != wordCount(candidate.Content) {
		t.Fatalf("candidate=%+v", candidate)
	}
	var parameters struct {
		SchemaVersion    string                  `json:"schemaVersion"`
		AddressedIssues  []RewriteIssueOutcomeV1 `json:"addressedIssues"`
		UnresolvedIssues []RewriteIssueOutcomeV1 `json:"unresolvedIssues"`
		Warnings         []string                `json:"warnings"`
	}
	if json.Unmarshal(candidate.GenerationParameters, &parameters) != nil ||
		parameters.SchemaVersion != "rewrite.output.v1" ||
		len(parameters.AddressedIssues) != 1 ||
		parameters.AddressedIssues[0].ReviewIssueID != fixture.issue.ID ||
		strings.Contains(string(candidate.GenerationParameters), candidate.Content) {
		t.Fatalf("parameters=%s", candidate.GenerationParameters)
	}
	var current uuid.UUID
	if err = fixture.repo.db.QueryRow(fixture.ctx, "SELECT current_version_id FROM content_items WHERE id=$1", fixture.item.Detail.Item.ID).Scan(&current); err != nil || current != currentBefore {
		t.Fatalf("current=%s err=%v", current, err)
	}
	var reportStatus, issueDisposition string
	var issueVersion int
	if err = fixture.repo.db.QueryRow(fixture.ctx, "SELECT status FROM review_reports WHERE id=$1", fixture.report.ID).Scan(&reportStatus); err != nil {
		t.Fatal(err)
	}
	if err = fixture.repo.db.QueryRow(fixture.ctx, "SELECT disposition,version FROM review_findings WHERE id=$1", fixture.issue.ID).Scan(&issueDisposition, &issueVersion); err != nil {
		t.Fatal(err)
	}
	if reportStatus != reportStatusBefore || issueDisposition != issueDispositionBefore || issueVersion != issueVersionBefore {
		t.Fatalf("report=%s issue=%s/%d", reportStatus, issueDisposition, issueVersion)
	}
	var eventPayload json.RawMessage
	if err = fixture.repo.db.QueryRow(fixture.ctx, "SELECT payload FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", run.ID).Scan(&eventPayload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(eventPayload), candidate.ID.String()) ||
		!strings.Contains(string(eventPayload), fixture.report.ID.String()) ||
		!strings.Contains(string(eventPayload), fixture.report.SourceContentVersionID.String()) {
		t.Fatalf("event payload=%s", eventPayload)
	}
	replayed, err := fixture.service.ConsumeRewriteResult(fixture.ctx, run)
	if err != nil || replayed.ID != candidate.ID {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	different := strings.Replace(string(validRewriteOutput(fixture.issue.ID)), "重写后的完整正文。", "不应覆盖的正文。", 1)
	if _, err = fixture.repo.db.Exec(fixture.ctx, "UPDATE workflow_run_records SET output_payload=$1 WHERE id=$2", json.RawMessage(different), run.ID); err != nil {
		t.Fatal(err)
	}
	replayed, err = fixture.service.ConsumeRewriteResult(fixture.ctx, run)
	if err != nil || replayed.ID != candidate.ID || replayed.Content != candidate.Content {
		t.Fatalf("different-output replay=%+v err=%v", replayed, err)
	}
	if count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", run.ID) != 1 ||
		count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", run.ID) != 1 {
		t.Fatal("idempotent consumption duplicated durable facts")
	}
}

func TestRealRewriteConsumptionUsesFixedSourceAfterCurrentVersionDrift(t *testing.T) {
	fixture := newRealRewriteFixture(t)
	run := succeededRewriteRun(t, fixture, validRewriteOutput(fixture.issue.ID))
	newCurrentID := uuid.New()
	if _, err := fixture.repo.db.Exec(fixture.ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,word_count,source,status) VALUES($1,$2,2,'新当前版本','运行期间保存的新正文',1,'manual_created','editable_draft')", newCurrentID, fixture.item.Detail.Item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repo.db.Exec(fixture.ctx, "UPDATE content_items SET current_version_id=$1,version=version+1 WHERE id=$2", newCurrentID, fixture.item.Detail.Item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repo.db.Exec(fixture.ctx, "UPDATE review_findings SET disposition='ignored',ignored_at=NOW(),ignored_by='later-reviewer',version=version+1,updated_at=NOW() WHERE id=$1", fixture.issue.ID); err != nil {
		t.Fatal(err)
	}
	candidate, err := fixture.service.ConsumeRewriteResult(fixture.ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	var current uuid.UUID
	if err = fixture.repo.db.QueryRow(fixture.ctx, "SELECT current_version_id FROM content_items WHERE id=$1", fixture.item.Detail.Item.ID).Scan(&current); err != nil {
		t.Fatal(err)
	}
	var issueDisposition string
	var issueVersion int
	if err = fixture.repo.db.QueryRow(fixture.ctx, "SELECT disposition,version FROM review_findings WHERE id=$1", fixture.issue.ID).Scan(&issueDisposition, &issueVersion); err != nil {
		t.Fatal(err)
	}
	if current != newCurrentID || candidate.VersionNo != 3 ||
		issueDisposition != "ignored" || issueVersion != fixture.issue.Version+1 ||
		candidate.SourceContentVersionID == nil || *candidate.SourceContentVersionID != fixture.report.SourceContentVersionID {
		t.Fatalf("candidate=%+v current=%s issue=%s/%d", candidate, current, issueDisposition, issueVersion)
	}
}

func TestRealRewriteRuntimeSucceededPersistsThenConsumesSameOutput(t *testing.T) {
	fixture := newRealRewriteFixture(t)
	preflight := fixture.preflight(t, "runtime")
	run, replay, err := fixture.service.CreateRun(
		fixture.ctx, fixture.report.ID, "rewriter", *preflight.PreflightToken, uuid.NewString(),
	)
	if err != nil || replay {
		t.Fatalf("run=%+v replay=%v err=%v", run, replay, err)
	}
	output := validRewriteOutput(fixture.issue.ID)
	executor := &workflowrun.FakeWorkflowExecutor{
		ExecuteResult: workflowrun.ExecutionResult{
			Status: workflowrun.ExecutionSucceeded,
			Output: output,
		},
	}
	fixture.runs.SetWorkflowExecutor(executor)
	fixture.runs.SetRewriteSucceededConsumer(fixture.service)
	updated, err := fixture.runs.ExecuteRun(fixture.ctx, run.ID)
	if err != nil || updated.Status != workflowrun.StatusSucceeded || executor.ExecuteCalls != 1 {
		t.Fatalf("updated=%+v calls=%d err=%v", updated, executor.ExecuteCalls, err)
	}
	var persistedOutput json.RawMessage
	if err = fixture.repo.db.QueryRow(fixture.ctx, "SELECT output_payload FROM workflow_run_records WHERE id=$1", run.ID).Scan(&persistedOutput); err != nil {
		t.Fatal(err)
	}
	candidate, err := fixture.service.consumedRewriteCandidate(fixture.ctx, run.ID)
	if err != nil || candidate.Content != "重写后的完整正文。" ||
		!bytes.Equal(persistedOutput, updated.OutputPayload) {
		t.Fatalf("candidate=%+v persisted=%s updated=%s err=%v", candidate, persistedOutput, updated.OutputPayload, err)
	}
}

func TestRealRewriteConcurrentConsumptionCreatesOneCandidate(t *testing.T) {
	fixture := newRealRewriteFixture(t)
	run := succeededRewriteRun(t, fixture, validRewriteOutput(fixture.issue.ID))
	results := make(chan ContentVersion, 2)
	errs := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			candidate, err := fixture.service.ConsumeRewriteResult(fixture.ctx, run)
			results <- candidate
			errs <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var candidateID uuid.UUID
	for candidate := range results {
		if candidateID == uuid.Nil {
			candidateID = candidate.ID
		} else if candidate.ID != candidateID {
			t.Fatalf("candidate IDs differ: %s != %s", candidate.ID, candidateID)
		}
	}
	if count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1 AND source='workflow_rewrite'", run.ID) != 1 ||
		count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", run.ID) != 1 {
		t.Fatal("concurrent consumption duplicated durable facts")
	}
}

func TestRealRewriteOutputValidationFailureIsSafeAndDeduplicated(t *testing.T) {
	fixture := newRealRewriteFixture(t)
	invalid := strings.Replace(string(validRewriteOutput(fixture.issue.ID)), `"metadata":`, `"unknown":true,"metadata":`, 1)
	run := succeededRewriteRun(t, fixture, json.RawMessage(invalid))
	currentBefore := fixture.item.Detail.Item.CurrentVersionID
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := fixture.service.ConsumeRewriteResult(fixture.ctx, run); !errors.Is(err, ErrRewriteOutputInvalid) {
			t.Fatalf("attempt=%d err=%v", attempt, err)
		}
	}
	if count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", run.ID) != 0 ||
		count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='output_validation_failed'", run.ID) != 1 {
		t.Fatal("validation failure facts are not atomic or deduplicated")
	}
	var current uuid.UUID
	var payload string
	if err := fixture.repo.db.QueryRow(fixture.ctx, "SELECT current_version_id FROM content_items WHERE id=$1", fixture.item.Detail.Item.ID).Scan(&current); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repo.db.QueryRow(fixture.ctx, "SELECT payload::text FROM workflow_run_events WHERE run_id=$1 AND event_type='output_validation_failed'", run.ID).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(payload)
	if current != currentBefore || strings.Contains(lower, "unknown") || strings.Contains(lower, "sql") ||
		strings.Contains(lower, "postgres") || strings.Contains(lower, "stack") ||
		!strings.Contains(payload, `"code": "output_validation_failed"`) {
		t.Fatalf("current=%s payload=%s", current, payload)
	}
}

type rewriteConsumerFailureRow struct {
	err error
}

func (row rewriteConsumerFailureRow) Scan(...any) error {
	return row.err
}

type rewriteConsumerFailureTx struct {
	pgx.Tx
	candidateErr  error
	eventErr      error
	commitErr     error
	candidateHits int
	eventHits     int
	commitHits    int
}

func (tx *rewriteConsumerFailureTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "INSERT INTO content_versions") && tx.candidateErr != nil {
		tx.candidateHits++
		return rewriteConsumerFailureRow{err: tx.candidateErr}
	}
	return tx.Tx.QueryRow(ctx, query, args...)
}

func (tx *rewriteConsumerFailureTx) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(query, "INSERT INTO workflow_run_events") && tx.eventErr != nil {
		tx.eventHits++
		return pgconn.CommandTag{}, tx.eventErr
	}
	return tx.Tx.Exec(ctx, query, args...)
}

func (tx *rewriteConsumerFailureTx) Commit(ctx context.Context) error {
	tx.commitHits++
	if tx.commitErr != nil {
		_ = tx.Tx.Rollback(ctx)
		return tx.commitErr
	}
	return tx.Tx.Commit(ctx)
}

func TestRealRewriteConsumptionFailuresRollbackAndRecordSafeEvent(t *testing.T) {
	tests := []struct {
		name         string
		candidateErr error
		eventErr     error
		commitErr    error
	}{
		{name: "candidate create", candidateErr: errors.New("candidate create failed")},
		{name: "source relation", candidateErr: &pgconn.PgError{Code: "23503", Message: "source relation failed"}},
		{name: "result consumed event", eventErr: errors.New("result consumed event failed")},
		{name: "commit", commitErr: errors.New("commit failed")},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newRealRewriteFixture(t)
			run := succeededRewriteRun(t, fixture, validRewriteOutput(fixture.issue.ID))
			currentBefore := fixture.item.Detail.Item.CurrentVersionID
			var reportStatusBefore string
			if err := fixture.repo.db.QueryRow(fixture.ctx, "SELECT status FROM review_reports WHERE id=$1", fixture.report.ID).Scan(&reportStatusBefore); err != nil {
				t.Fatal(err)
			}
			issueDispositionBefore, issueVersionBefore := fixture.issue.Disposition, fixture.issue.Version
			var failureTx *rewriteConsumerFailureTx
			fixture.service.begin = func(ctx context.Context) (pgx.Tx, error) {
				inner, err := fixture.repo.db.Begin(ctx)
				if err != nil {
					return nil, err
				}
				failureTx = &rewriteConsumerFailureTx{
					Tx: inner, candidateErr: testCase.candidateErr,
					eventErr: testCase.eventErr, commitErr: testCase.commitErr,
				}
				return failureTx, nil
			}
			if _, err := fixture.service.ConsumeRewriteResult(fixture.ctx, run); !errors.Is(err, ErrRewriteResultConsumption) {
				t.Fatalf("error=%v", err)
			}
			if failureTx == nil {
				t.Fatal("main transaction was not started")
			}
			if count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM content_versions WHERE source_workflow_run_id=$1", run.ID) != 0 ||
				count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'", run.ID) != 0 ||
				count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumption_failed'", run.ID) != 1 {
				t.Fatal("consumption failure left partial or duplicate facts")
			}
			var current uuid.UUID
			var reportStatus, issueDisposition, failurePayload string
			var issueVersion int
			if err := fixture.repo.db.QueryRow(fixture.ctx, "SELECT current_version_id FROM content_items WHERE id=$1", fixture.item.Detail.Item.ID).Scan(&current); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repo.db.QueryRow(fixture.ctx, "SELECT status FROM review_reports WHERE id=$1", fixture.report.ID).Scan(&reportStatus); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repo.db.QueryRow(fixture.ctx, "SELECT disposition,version FROM review_findings WHERE id=$1", fixture.issue.ID).Scan(&issueDisposition, &issueVersion); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repo.db.QueryRow(fixture.ctx, "SELECT payload::text FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumption_failed'", run.ID).Scan(&failurePayload); err != nil {
				t.Fatal(err)
			}
			lower := strings.ToLower(failurePayload)
			if current != currentBefore || reportStatus != reportStatusBefore ||
				issueDisposition != issueDispositionBefore || issueVersion != issueVersionBefore ||
				strings.Contains(lower, "source relation") || strings.Contains(lower, "candidate create") ||
				strings.Contains(lower, "sql") || strings.Contains(lower, "stack") ||
				!strings.Contains(failurePayload, `"code": "result_consumption_failed"`) {
				t.Fatalf("current=%s report=%s issue=%s/%d payload=%s", current, reportStatus, issueDisposition, issueVersion, failurePayload)
			}
			if _, err := fixture.service.ConsumeRewriteResult(fixture.ctx, run); !errors.Is(err, ErrRewriteResultConsumption) {
				t.Fatalf("repeat error=%v", err)
			}
			if count(t, fixture.ctx, fixture.repo.db, "SELECT count(*) FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumption_failed'", run.ID) != 1 {
				t.Fatal("repeated consumption failure appended another event")
			}
		})
	}
}

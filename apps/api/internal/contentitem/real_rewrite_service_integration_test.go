package contentitem

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

type realRewriteFixture struct {
	service    *RealRewriteService
	runs       *workflowrun.Service
	repo       *PostgresRepository
	ctx        context.Context
	report     RealReviewReport
	issue      RealReviewIssue
	item       CreateResult
	binding    uuid.UUID
	workflow   uuid.UUID
	connection uuid.UUID
}

func newRealRewriteFixture(t *testing.T) realRewriteFixture {
	t.Helper()
	review := newRealReviewFixture(t)
	preflight := review.preflight(t)
	reviewRun, err := review.service.CreateRun(review.ctx, review.item.Detail.CurrentVersion.ID, "reviewer", *preflight.PreflightToken, "rewrite-source-review")
	if err != nil {
		t.Fatal(err)
	}
	reviewRun, err = review.runs.GetRun(review.ctx, reviewRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	reviewCompletedAt := time.Now().UTC()
	if reviewCompletedAt.Before(reviewRun.CreatedAt) {
		reviewCompletedAt = reviewRun.CreatedAt
	}
	if _, err = review.repo.db.Exec(review.ctx, "UPDATE workflow_run_records SET status='succeeded',output_payload=$1,started_at=$3,finished_at=$3,updated_at=$3,version=2 WHERE id=$2", json.RawMessage(validReviewOutput), reviewRun.ID, reviewCompletedAt); err != nil {
		t.Fatal(err)
	}
	reviewRun, err = review.runs.GetRun(review.ctx, reviewRun.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = review.service.ConsumeSucceededRun(review.ctx, reviewRun); err != nil {
		t.Fatal(err)
	}
	result, err := review.service.realReviewResultByRun(review.ctx, review.repo.db, reviewRun, 4)
	if err != nil || len(result.Issues) != 1 {
		t.Fatalf("review result=%+v err=%v", result, err)
	}
	connectionID, workflowID, bindingID := uuid.New(), uuid.New(), uuid.New()
	if _, err = review.repo.db.Exec(review.ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,integration_status,enabled,last_verified_version) VALUES($1,$2,'n8n','http://rewrite-fixture','api_key',5,'{\"referenceType\":\"webhook_path\",\"referenceValue\":\"rewrite-fixture\"}','verified',true,1)", connectionID, "rewrite-"+connectionID.String()); err != nil {
		t.Fatal(err)
	}
	typeConfig := json.RawMessage(`{"referenceType":"webhook_path","referenceValue":"rewrite-fixture"}`)
	if _, err = review.repo.db.Exec(review.ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version,default_parameters,integration_status,enabled,last_verified_version) VALUES($1,$2,$3,'[\"rewrite\"]',$4,'rewrite.input.v1','rewrite.output.v1','{}','verified',true,1)", workflowID, "rewrite-"+workflowID.String(), connectionID, typeConfig); err != nil {
		t.Fatal(err)
	}
	if _, err = review.repo.db.Exec(review.ctx, "INSERT INTO project_workflow_bindings(id,project_id,stage,workflow_configuration_id) VALUES($1,$2,'rewrite',$3)", bindingID, review.item.Detail.Item.ProjectID, workflowID); err != nil {
		t.Fatal(err)
	}
	configs, err := globalconfig.NewService(review.repo.db, "real-rewrite-integration-key")
	if err != nil {
		t.Fatal(err)
	}
	credential := "real-rewrite-runtime-credential"
	connection, err := configs.UpdateConnection(review.ctx, connectionID, globalconfig.ConnectionUpdate{ExpectedVersion: 1, Credential: &credential})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = review.repo.db.Exec(review.ctx, "UPDATE workflow_connections SET integration_status='verified',last_verified_version=version,enabled=true WHERE id=$1 AND version=$2", connectionID, connection.Version); err != nil {
		t.Fatal(err)
	}
	runs := workflowrun.NewService(
		workflowrun.NewPostgresRepository(review.repo.db), project.NewPostgresRepository(review.repo.db),
		workflowbinding.NewPostgresRepository(review.repo.db), configs, configs,
	)
	service := NewRealRewriteService(review.repo, workflowbinding.NewPostgresRepository(review.repo.db), configs, runs, "real-rewrite-token-secret")
	t.Cleanup(func() {
		_, _ = review.repo.db.Exec(context.Background(), "DELETE FROM project_workflow_bindings WHERE id=$1", bindingID)
		_, _ = review.repo.db.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE id=$1", workflowID)
		_, _ = review.repo.db.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", connectionID)
	})
	return realRewriteFixture{
		service: service, runs: runs, repo: review.repo, ctx: review.ctx,
		report: result.Report, issue: result.Issues[0], item: review.item,
		binding: bindingID, workflow: workflowID, connection: connectionID,
	}
}

func (f realRewriteFixture) preflight(t *testing.T, key string) RewritePreflightResult {
	t.Helper()
	instructions := "保持原叙事视角"
	result, err := f.service.Preflight(f.ctx, f.report.ID, RewritePreflightRequest{
		SelectedIssueIDs: []uuid.UUID{f.issue.ID}, OptionalInstructions: &instructions,
		RewriteOptions: RewriteOptions{Strategy: "targeted_fix"}, ActorID: "rewriter",
	})
	if err != nil || result.Status != "passed" || result.PreflightToken == nil ||
		result.ConfigurationSummary == nil || len(result.Checks) != 8 {
		t.Fatalf("%s preflight=%+v err=%v", key, result, err)
	}
	return result
}

func TestRealRewriteAvailabilityPreflightCreateReplaySnapshotAndDispatchDiscovery(t *testing.T) {
	f := newRealRewriteFixture(t)
	beforeRuns := count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID)
	beforeEvents := count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_events e JOIN workflow_run_records r ON r.id=e.run_id WHERE r.stage='rewrite' AND r.subject_id=$1", f.report.ID)
	beforeIdempotency := count(t, f.ctx, f.repo.db, "SELECT count(*) FROM idempotency_records WHERE scope LIKE 'createContentRewriteRun:%' OR scope LIKE 'consumeContentRewritePreflightToken:%'")
	availability, err := f.service.Availability(f.ctx, f.report.ID)
	if err != nil || !availability.Available || availability.Reason != nil ||
		availability.OpenIssueCount != 1 || availability.ConfigurationSummary == nil {
		t.Fatalf("availability=%+v err=%v", availability, err)
	}
	first := f.preflight(t, "first")
	second := f.preflight(t, "second")
	if count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID) != beforeRuns ||
		count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_events e JOIN workflow_run_records r ON r.id=e.run_id WHERE r.stage='rewrite' AND r.subject_id=$1", f.report.ID) != beforeEvents ||
		count(t, f.ctx, f.repo.db, "SELECT count(*) FROM idempotency_records WHERE scope LIKE 'createContentRewriteRun:%' OR scope LIKE 'consumeContentRewritePreflightToken:%'") != beforeIdempotency {
		t.Fatal("Availability or Preflight created durable facts")
	}
	currentBefore := f.item.Detail.Item.CurrentVersionID
	run, replay, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *first.PreflightToken, "create-rewrite")
	if err != nil || replay {
		t.Fatalf("run=%+v replay=%v err=%v", run, replay, err)
	}
	if run.Stage != "rewrite" || run.Status != workflowrun.StatusQueued || run.TriggerSource != "manual" ||
		run.SubjectType == nil || *run.SubjectType != "review_report" || run.SubjectID == nil || *run.SubjectID != f.report.ID {
		t.Fatalf("run=%+v", run)
	}
	var input RewriteRuntimeInputV1
	if json.Unmarshal(run.InputPayload, &input) != nil || input.SchemaVersion != "rewrite.input.v1" ||
		input.WorkflowRunID != run.ID || input.CorrelationID != run.ID.String() ||
		input.ProjectID != f.item.Detail.Item.ProjectID || input.ContentItemID != f.item.Detail.Item.ID ||
		input.SourceContentVersionID != f.report.SourceContentVersionID ||
		input.SourceContentVersionVersion != f.report.SourceContentVersionVersion ||
		input.SourceContentHash != f.report.SourceContentHash || input.ReviewReportID != f.report.ID ||
		len(input.SelectedIssues) != 1 || input.SelectedIssues[0].ReviewIssueID != f.issue.ID ||
		input.SelectedIssues[0].Version != f.issue.Version || input.SelectedIssues[0].Disposition != "open" ||
		input.RewriteOptions.Strategy != "targeted_fix" {
		t.Fatalf("input=%+v", input)
	}
	var currentAfter uuid.UUID
	if err = f.repo.db.QueryRow(f.ctx, "SELECT current_version_id FROM content_items WHERE id=$1", f.item.Detail.Item.ID).Scan(&currentAfter); err != nil || currentAfter != currentBefore {
		t.Fatalf("current version changed to %s err=%v", currentAfter, err)
	}
	events, err := f.runs.ListRunEvents(f.ctx, run.ID)
	if err != nil || len(events) != 1 || events[0].EventType != "queued" || events[0].Status != workflowrun.StatusQueued {
		t.Fatalf("events=%+v err=%v", events, err)
	}
	queued, err := f.runs.ListRuns(f.ctx, workflowrun.ListRunsQuery{ListFilter: workflowrun.ListFilter{Status: "queued", Stage: "rewrite", Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, queuedRun := range queued.Items {
		if queuedRun.ID == run.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("existing queued worker scan cannot discover Rewrite Run")
	}
	activeAvailability, err := f.service.Availability(f.ctx, f.report.ID)
	if err != nil || activeAvailability.Available || activeAvailability.Reason == nil ||
		*activeAvailability.Reason != "active_rewrite_run_conflict" || activeAvailability.ActiveRun == nil ||
		activeAvailability.ActiveRun.ID != run.ID {
		t.Fatalf("active availability=%+v err=%v", activeAvailability, err)
	}
	activePreflight, err := f.service.Preflight(f.ctx, f.report.ID, RewritePreflightRequest{
		SelectedIssueIDs: []uuid.UUID{f.issue.ID}, RewriteOptions: RewriteOptions{Strategy: "targeted_fix"}, ActorID: "rewriter",
	})
	if err != nil || activePreflight.Status != "blocked" || activePreflight.PreflightToken != nil ||
		activePreflight.Checks[len(activePreflight.Checks)-1].Code != "active_rewrite_run_absent" {
		t.Fatalf("active preflight=%+v err=%v", activePreflight, err)
	}
	replayed, replay, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *first.PreflightToken, "create-rewrite")
	if err != nil || !replay || replayed.ID != run.ID {
		t.Fatalf("replayed=%+v replay=%v err=%v", replayed, replay, err)
	}
	if _, _, err = f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *second.PreflightToken, "create-rewrite"); !errors.Is(err, workflowrun.ErrIdempotencyConflict) {
		t.Fatalf("same key different request error=%v", err)
	}
	if _, _, err = f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *second.PreflightToken, "active-rewrite"); !errors.Is(err, ErrRewriteActiveRun) {
		t.Fatalf("active Run error=%v", err)
	}
	if _, _, err = f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *first.PreflightToken, "other-key"); !errors.Is(err, ErrRewritePreflightConsumed) {
		t.Fatalf("consumed token error=%v", err)
	}
	var snapshot map[string]any
	if json.Unmarshal(run.ConfigurationSnapshot, &snapshot) != nil ||
		snapshot["stage"] != "rewrite" || strings.Contains(strings.ToLower(string(run.ConfigurationSnapshot)), "credential") {
		t.Fatalf("configuration snapshot=%s", run.ConfigurationSnapshot)
	}
	if _, err = f.repo.db.Exec(f.ctx, "UPDATE review_findings SET disposition='ignored',ignored_at=NOW(),ignored_by='test' WHERE review_id=$1", f.report.ID); err != nil {
		t.Fatal(err)
	}
	activeWithoutOpenIssues, err := f.service.Availability(f.ctx, f.report.ID)
	if err != nil || activeWithoutOpenIssues.OpenIssueCount != 0 ||
		activeWithoutOpenIssues.Reason == nil || *activeWithoutOpenIssues.Reason != "active_rewrite_run_conflict" ||
		activeWithoutOpenIssues.ActiveRun == nil || activeWithoutOpenIssues.ActiveRun.ID != run.ID {
		t.Fatalf("active without open issues=%+v err=%v", activeWithoutOpenIssues, err)
	}
}

func TestRealRewritePreflightTokenMaximumBoundaryPreservesFullSnapshot(t *testing.T) {
	f := newRealRewriteFixture(t)
	rows, err := f.repo.db.Query(f.ctx, `
		INSERT INTO review_findings(
			id,review_id,issue_key,sort_order,category,category_label,severity,title,description,
			evidence_json,location_json,suggestion,disposition,version,created_at,updated_at
		)
		SELECT gen_random_uuid(),review_id,'boundary-'||n,n,category,category_label,severity,
			title||n,description,evidence_json,location_json,suggestion,'open',1,NOW(),NOW()
		FROM review_findings CROSS JOIN generate_series(2,50) n
		WHERE id=$1
		RETURNING id`, f.issue.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{f.issue.ID}
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		t.Fatal(err)
	}
	rows.Close()
	if len(ids) != 50 {
		t.Fatalf("issues=%d", len(ids))
	}
	instructions := strings.Repeat("界", 2000)
	preflight, err := f.service.Preflight(f.ctx, f.report.ID, RewritePreflightRequest{
		SelectedIssueIDs: ids, OptionalInstructions: &instructions,
		RewriteOptions: RewriteOptions{Strategy: "targeted_fix"}, ActorID: "rewriter",
	})
	if err != nil || preflight.Status != "passed" || preflight.PreflightToken == nil {
		t.Fatalf("preflight=%+v err=%v", preflight, err)
	}
	if size := len(*preflight.PreflightToken); size <= 4096 || size > RewritePreflightTokenMaxLength {
		t.Fatalf("token length=%d", size)
	}
	run, replay, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, "boundary-create")
	if err != nil || replay {
		t.Fatalf("run=%+v replay=%v err=%v", run, replay, err)
	}
	var input RewriteRuntimeInputV1
	if err = json.Unmarshal(run.InputPayload, &input); err != nil {
		t.Fatal(err)
	}
	if len(input.SelectedIssues) != 50 || input.OptionalInstructions == nil || *input.OptionalInstructions != instructions {
		t.Fatalf("snapshot issues=%d instructions=%d", len(input.SelectedIssues), utf8.RuneCountInString(valueOrEmpty(input.OptionalInstructions)))
	}
	for index := 1; index < len(input.SelectedIssues); index++ {
		previous, current := input.SelectedIssues[index-1], input.SelectedIssues[index]
		if previous.Position > current.Position ||
			(previous.Position == current.Position && previous.ReviewIssueID.String() > current.ReviewIssueID.String()) {
			t.Fatalf("issue order drifted at %d", index)
		}
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func TestRealRewritePreflightValidationAndAvailabilityReasons(t *testing.T) {
	f := newRealRewriteFixture(t)
	_, err := f.service.Preflight(f.ctx, f.report.ID, RewritePreflightRequest{
		SelectedIssueIDs: nil, RewriteOptions: RewriteOptions{Strategy: "targeted_fix"}, ActorID: "rewriter",
	})
	if !errors.Is(err, ErrRewriteNotAvailable) {
		t.Fatalf("empty error=%v", err)
	}
	_, err = f.service.Preflight(f.ctx, f.report.ID, RewritePreflightRequest{
		SelectedIssueIDs: []uuid.UUID{f.issue.ID, f.issue.ID},
		RewriteOptions:   RewriteOptions{Strategy: "targeted_fix"}, ActorID: "rewriter",
	})
	if !errors.Is(err, ErrRewriteNotAvailable) {
		t.Fatalf("duplicate error=%v", err)
	}
	_, err = f.service.Preflight(f.ctx, f.report.ID, RewritePreflightRequest{
		SelectedIssueIDs: []uuid.UUID{f.issue.ID},
		RewriteOptions:   RewriteOptions{Strategy: "unknown"}, ActorID: "rewriter",
	})
	if !errors.Is(err, ErrRewriteNotAvailable) {
		t.Fatalf("strategy error=%v", err)
	}
	other := newRealRewriteFixture(t)
	_, err = f.service.Preflight(f.ctx, f.report.ID, RewritePreflightRequest{
		SelectedIssueIDs: []uuid.UUID{other.issue.ID},
		RewriteOptions:   RewriteOptions{Strategy: "targeted_fix"}, ActorID: "rewriter",
	})
	if !errors.Is(err, ErrReviewIssueNotFound) {
		t.Fatalf("cross Report error=%v", err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "DELETE FROM project_workflow_bindings WHERE id=$1", f.binding); err != nil {
		t.Fatal(err)
	}
	availability, err := f.service.Availability(f.ctx, f.report.ID)
	if err != nil || availability.Available || availability.Reason == nil || *availability.Reason != "rewrite_not_configured" {
		t.Fatalf("not configured availability=%+v err=%v", availability, err)
	}
	blocked, err := f.service.Preflight(f.ctx, f.report.ID, RewritePreflightRequest{
		SelectedIssueIDs: []uuid.UUID{f.issue.ID},
		RewriteOptions:   RewriteOptions{Strategy: "targeted_fix"}, ActorID: "rewriter",
	})
	if err != nil || blocked.Status != "blocked" || blocked.ConfigurationSummary != nil || blocked.PreflightToken != nil {
		t.Fatalf("not configured preflight=%+v err=%v", blocked, err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "INSERT INTO project_workflow_bindings(id,project_id,stage,workflow_configuration_id) VALUES($1,$2,'rewrite',$3)", f.binding, f.item.Detail.Item.ProjectID, f.workflow); err != nil {
		t.Fatal(err)
	}
	if _, err = f.repo.db.Exec(f.ctx, "UPDATE review_findings SET disposition='ignored',ignored_at=NOW(),ignored_by='test',version=version+1 WHERE id=$1", f.issue.ID); err != nil {
		t.Fatal(err)
	}
	availability, err = f.service.Availability(f.ctx, f.report.ID)
	if err != nil || availability.Available || availability.Reason == nil || *availability.Reason != "no_open_issues" {
		t.Fatalf("no issues availability=%+v err=%v", availability, err)
	}
	_, err = f.service.Preflight(f.ctx, f.report.ID, RewritePreflightRequest{
		SelectedIssueIDs: []uuid.UUID{f.issue.ID},
		RewriteOptions:   RewriteOptions{Strategy: "targeted_fix"}, ActorID: "rewriter",
	})
	if !errors.Is(err, ErrRewriteNotAvailable) {
		t.Fatalf("ignored error=%v", err)
	}
}

func TestRealRewriteAvailabilityAllowsWorkflowGeneratedReviewedSource(t *testing.T) {
	f := newRealRewriteFixture(t)
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE content_versions SET status='editable_draft',frozen_at=NULL WHERE id=$1", f.report.SourceContentVersionID); err != nil {
		t.Fatal(err)
	}
	availability, err := f.service.Availability(f.ctx, f.report.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !availability.Available || availability.Reason != nil || availability.ConfigurationSummary == nil || availability.OpenIssueCount != 1 {
		t.Fatalf("availability=%+v", availability)
	}
}

func TestRealRewriteCreateRollbackOnExpiredActorSourceIssueAndConfigurationDrift(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*testing.T, realRewriteFixture)
		error  error
	}{
		{"expired", func(_ *testing.T, f realRewriteFixture) {
			f.service.now = func() time.Time { return time.Now().Add(20 * time.Minute) }
		}, ErrRewritePreflightExpired},
		{"source hash", func(t *testing.T, f realRewriteFixture) {
			_, err := f.repo.db.Exec(f.ctx, "UPDATE content_versions SET content=content || '漂移' WHERE id=$1", f.report.SourceContentVersionID)
			if err != nil {
				t.Fatal(err)
			}
		}, ErrRewritePreflightStale},
		{"issue version", func(t *testing.T, f realRewriteFixture) {
			_, err := f.repo.db.Exec(f.ctx, "UPDATE review_findings SET version=version+1 WHERE id=$1", f.issue.ID)
			if err != nil {
				t.Fatal(err)
			}
		}, ErrRewritePreflightStale},
		{"issue disposition", func(t *testing.T, f realRewriteFixture) {
			_, err := f.repo.db.Exec(f.ctx, "UPDATE review_findings SET disposition='ignored',ignored_at=NOW(),ignored_by='test',version=version+1 WHERE id=$1", f.issue.ID)
			if err != nil {
				t.Fatal(err)
			}
		}, ErrRewritePreflightStale},
		{"binding", func(t *testing.T, f realRewriteFixture) {
			_, err := f.repo.db.Exec(f.ctx, "UPDATE project_workflow_bindings SET version=version+1 WHERE id=$1", f.binding)
			if err != nil {
				t.Fatal(err)
			}
		}, ErrRewritePreflightStale},
		{"configuration", func(t *testing.T, f realRewriteFixture) {
			_, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_configurations SET integration_status='stale',version=version+1 WHERE id=$1", f.workflow)
			if err != nil {
				t.Fatal(err)
			}
		}, ErrRewritePreflightStale},
		{"connection", func(t *testing.T, f realRewriteFixture) {
			_, err := f.repo.db.Exec(f.ctx, "UPDATE workflow_connections SET integration_status='stale',version=version+1 WHERE id=$1", f.connection)
			if err != nil {
				t.Fatal(err)
			}
		}, ErrRewritePreflightStale},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			f := newRealRewriteFixture(t)
			preflight := f.preflight(t, test.name)
			beforeRuns := count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID)
			beforeEvents := count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_events e JOIN workflow_run_records r ON r.id=e.run_id WHERE r.stage='rewrite' AND r.subject_id=$1", f.report.ID)
			test.mutate(t, f)
			_, _, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, "drift-"+test.name)
			if !errors.Is(err, test.error) {
				t.Fatalf("error=%v want=%v", err, test.error)
			}
			if count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID) != beforeRuns ||
				count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_events e JOIN workflow_run_records r ON r.id=e.run_id WHERE r.stage='rewrite' AND r.subject_id=$1", f.report.ID) != beforeEvents {
				t.Fatal("failed Create left a Run or Event")
			}
			claims, parseErr := f.service.parseRewriteToken(*preflight.PreflightToken)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			var consumed bool
			err = f.repo.db.QueryRow(f.ctx, "SELECT EXISTS(SELECT 1 FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2)", "consumeContentRewritePreflightToken:"+f.item.Detail.Item.ProjectID.String(), claims.Nonce).Scan(&consumed)
			if err != nil || consumed {
				t.Fatalf("token consumed=%v err=%v", consumed, err)
			}
		})
	}
	f := newRealRewriteFixture(t)
	preflight := f.preflight(t, "actor")
	if _, _, err := f.service.CreateRun(f.ctx, f.report.ID, "different-actor", *preflight.PreflightToken, "actor"); !errors.Is(err, ErrRewritePreflightStale) {
		t.Fatalf("actor error=%v", err)
	}
}

func TestRealRewriteCurrentVersionDriftDoesNotReplaceFixedSource(t *testing.T) {
	f := newRealRewriteFixture(t)
	preflight := f.preflight(t, "current-drift")
	newVersionID := uuid.New()
	if _, err := f.repo.db.Exec(f.ctx, "INSERT INTO content_versions(id,content_item_id,version_no,title,content,word_count,source,status) VALUES($1,$2,2,'新当前版本','新正文',3,'manual_created','editable_draft')", newVersionID, f.item.Detail.Item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.db.Exec(f.ctx, "UPDATE content_items SET current_version_id=$1,version=version+1 WHERE id=$2", newVersionID, f.item.Detail.Item.ID); err != nil {
		t.Fatal(err)
	}
	run, _, err := f.service.CreateRun(f.ctx, f.report.ID, "rewriter", *preflight.PreflightToken, "current-drift")
	if err != nil {
		t.Fatal(err)
	}
	var input RewriteRuntimeInputV1
	if json.Unmarshal(run.InputPayload, &input) != nil || input.SourceContentVersionID != f.report.SourceContentVersionID || input.SourceContentVersionID == newVersionID {
		t.Fatalf("input=%+v", input)
	}
}

func TestRealRewriteConcurrentSameRequestCreatesOneRun(t *testing.T) {
	f := newRealRewriteFixture(t)
	preflight := f.preflight(t, "concurrent")
	var wait sync.WaitGroup
	ids := make([]uuid.UUID, 2)
	replays := make([]bool, 2)
	errs := make([]error, 2)
	for index := range ids {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			run, replay, err := f.service.CreateRun(context.Background(), f.report.ID, "rewriter", *preflight.PreflightToken, "concurrent-same")
			ids[index], replays[index], errs[index] = run.ID, replay, err
		}(index)
	}
	wait.Wait()
	if errs[0] != nil || errs[1] != nil || ids[0] == uuid.Nil || ids[0] != ids[1] || replays[0] == replays[1] {
		t.Fatalf("ids=%v replays=%v errors=%v", ids, replays, errs)
	}
	if count(t, f.ctx, f.repo.db, "SELECT count(*) FROM workflow_run_records WHERE stage='rewrite' AND subject_id=$1", f.report.ID) != 1 {
		t.Fatal("concurrent same request created multiple Runs")
	}
}

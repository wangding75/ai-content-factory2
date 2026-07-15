package contentitem

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const reviewFingerprint = "1111111111111111111111111111111111111111111111111111111111111111"

func reviewRequest(f fx, x CreateResult, expected int, key, fp string) ReviewRequest {
	return ReviewRequest{ContentItemID: x.Detail.Item.ID, ContentVersionID: x.Detail.CurrentVersion.ID, ExpectedVersion: expected, IdempotencyKey: key, Fingerprint: fp, Result: ReviewResult{
		Conclusion: "revise", Score: 70, Summary: "deterministic review", InputJSON: []byte(`{"content_version_id":"fixed"}`), OutputJSON: []byte(`{"conclusion":"revise"}`),
		Findings:        []ReviewFinding{{Category: "pacing", Severity: "high", Title: "later", Description: "d", LocationJSON: []byte(`{"start_offset":3,"end_offset":4}`), SortOrder: 2}, {Category: "foreshadowing", Severity: "low", Title: "first", Description: "d", SortOrder: 1}},
		Recommendations: []ReviewRecommendation{{Priority: "medium", Title: "later", Description: "d", SortOrder: 2}, {Priority: "high", Title: "first", Description: "d", SortOrder: 1}},
	}}
}

func reviewCounts(t *testing.T, ctx context.Context, db *pgxpool.Pool, item uuid.UUID) (runs, reports, findings, recommendations int) {
	t.Helper()
	runs = count(t, ctx, db, "SELECT count(*) FROM workflow_runs WHERE content_item_id=$1 AND workflow_key='content_mock_review'", item)
	reports = count(t, ctx, db, "SELECT count(*) FROM review_reports WHERE content_item_id=$1", item)
	findings = count(t, ctx, db, "SELECT count(*) FROM review_findings rf JOIN review_reports rr ON rr.id=rf.review_id WHERE rr.content_item_id=$1", item)
	recommendations = count(t, ctx, db, "SELECT count(*) FROM review_recommendations rrn JOIN review_reports rr ON rr.id=rrn.review_id WHERE rr.content_item_id=$1", item)
	return
}

func TestPostgresMockReviewSuccessIdempotencyAndDetail(t *testing.T) {
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	r := NewPostgresRepository(db)
	x := create(t, ctx, r, f)
	o, err := r.PersistMockReview(ctx, reviewRequest(f, x, 1, "review-success", reviewFingerprint))
	if err != nil {
		t.Fatal(err)
	}
	if o.WorkflowRun.Status != "succeeded" || o.WorkflowRun.FinishedAt == nil || o.Review.ContentVersionID != x.Detail.CurrentVersion.ID || o.Detail.Item.Status != "reviewed" || o.Detail.Item.ReviewedAt == nil || o.Detail.Item.Version != 2 || o.Detail.CurrentVersion.Status != "frozen" || o.Detail.CurrentVersion.FrozenAt == nil || o.Detail.CurrentVersion.Version != 2 {
		t.Fatalf("out=%+v", o)
	}
	if len(o.Findings) != 2 || o.Findings[0].SortOrder != 1 || len(o.Recommendations) != 2 || o.Recommendations[0].SortOrder != 1 {
		t.Fatalf("unstable child order: %+v", o)
	}
	if a, b, c, d := reviewCounts(t, ctx, db, x.Detail.Item.ID); a != 1 || b != 1 || c != 2 || d != 2 {
		t.Fatalf("counts %d %d %d %d", a, b, c, d)
	}
	var itemStatus, versionStatus string
	var itemVersion, versionVersion int
	var reviewed, frozen any
	if err := db.QueryRow(ctx, "SELECT ci.status,ci.version,ci.reviewed_at,cv.status,cv.version,cv.frozen_at FROM content_items ci JOIN content_versions cv ON cv.id=ci.current_version_id WHERE ci.id=$1", x.Detail.Item.ID).Scan(&itemStatus, &itemVersion, &reviewed, &versionStatus, &versionVersion, &frozen); err != nil {
		t.Fatal(err)
	}
	if itemStatus != "reviewed" || versionStatus != "frozen" || itemVersion != 2 || versionVersion != 2 || reviewed == nil || frozen == nil {
		t.Fatal("database state did not freeze atomically")
	}
	d, err := r.GetReview(ctx, o.Review.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Review.ID != o.Review.ID || d.ContentVersion.ID != x.Detail.CurrentVersion.ID || d.WorkflowRun.ID != o.WorkflowRun.ID || len(d.Findings) != 2 || d.Findings[0].Title != "first" || len(d.Recommendations) != 2 || d.Recommendations[0].Title != "first" {
		t.Fatalf("detail=%+v", d)
	}
	again, err := r.PersistMockReview(ctx, reviewRequest(f, x, 1, "review-success", reviewFingerprint))
	if err != nil {
		t.Fatal(err)
	}
	if again.Review.ID != o.Review.ID || again.WorkflowRun.ID != o.WorkflowRun.ID || again.Detail.CurrentVersion.Version != 2 {
		t.Fatalf("retry=%+v", again)
	}
	if a, b, c, d := reviewCounts(t, ctx, db, x.Detail.Item.ID); a != 1 || b != 1 || c != 2 || d != 2 {
		t.Fatal("idempotency added rows")
	}
	if _, err = r.PersistMockReview(ctx, reviewRequest(f, x, 1, "review-success", "2222222222222222222222222222222222222222222222222222222222222222")); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("fingerprint err=%v", err)
	}
	if _, err = r.PersistMockReview(ctx, reviewRequest(f, x, 2, "new-key", reviewFingerprint)); !errors.Is(err, ErrContentVersionReviewed) {
		t.Fatalf("reviewed err=%v", err)
	}
	if _, err = r.GetReview(ctx, uuid.New()); !errors.Is(err, ErrReviewNotFound) {
		t.Fatalf("not found=%v", err)
	}
}

func TestPostgresMockReviewRollbackFailureAndRelations(t *testing.T) {
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	r := NewPostgresRepository(db)
	x := create(t, ctx, r, f)
	bad := reviewRequest(f, x, 1, "rollback", reviewFingerprint)
	bad.Result.Findings = []ReviewFinding{{Category: "pacing", Severity: "low", Title: "one", Description: "d", SortOrder: 0}, {Category: "pacing", Severity: "low", Title: "two", Description: "d", SortOrder: 0}}
	if _, err := r.PersistMockReview(ctx, bad); !errors.Is(err, ErrInvalidReviewResult) {
		t.Fatalf("rollback=%v", err)
	}
	if a, b, c, d := reviewCounts(t, ctx, db, x.Detail.Item.ID); a != 0 || b != 0 || c != 0 || d != 0 {
		t.Fatalf("partial rows %d %d %d %d", a, b, c, d)
	}
	d, err := r.GetByID(ctx, x.Detail.Item.ID)
	if err != nil || d.Item.Status != "draft" || d.Item.ReviewedAt != nil || d.CurrentVersion.Status != "editable_draft" || d.CurrentVersion.FrozenAt != nil || d.CurrentVersion.Version != 1 {
		t.Fatalf("rollback state=%+v %v", d, err)
	}
	if _, err = r.PersistMockReview(ctx, reviewRequest(f, x, 99, "stale", reviewFingerprint)); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale=%v", err)
	}
	failed, err := r.RecordMockReviewFailure(ctx, ReviewFailureRequest{ContentItemID: x.Detail.Item.ID, ContentVersionID: x.Detail.CurrentVersion.ID, IdempotencyKey: "failed", Fingerprint: reviewFingerprint, InputJSON: []byte(`{}`), ErrorCode: "MOCK_REVIEW_FAILED", ErrorSummary: "safe failure"})
	if err != nil {
		t.Fatal(err)
	}
	if failed.Status != "failed" || failed.FinishedAt == nil || failed.ErrorCode == nil || *failed.ErrorCode != "MOCK_REVIEW_FAILED" {
		t.Fatalf("failure=%+v", failed)
	}
	if count(t, ctx, db, "SELECT count(*) FROM workflow_runs WHERE content_item_id=$1 AND status='running'", x.Detail.Item.ID) != 0 {
		t.Fatal("running orphan")
	}
	if a, b, c, d := reviewCounts(t, ctx, db, x.Detail.Item.ID); a != 1 || b != 0 || c != 0 || d != 0 {
		t.Fatalf("failed rows %d %d %d %d", a, b, c, d)
	}
	otherPlan := uuid.New()
	if _, err = db.Exec(ctx, "INSERT INTO chapter_plans(id,project_id,chapter_no,title,summary,status,source,created_by,confirmed_at) VALUES($1,$2,3,'other','s','confirmed','mock_generated','i06',NOW())", otherPlan, f.project); err != nil {
		t.Fatal(err)
	}
	y, err := r.CreateOrGet(ctx, otherPlan)
	if err != nil {
		t.Fatal(err)
	}
	cross := reviewRequest(f, x, 1, "cross", reviewFingerprint)
	cross.ContentVersionID = y.Detail.CurrentVersion.ID
	if _, err = r.PersistMockReview(ctx, cross); !errors.Is(err, ErrCrossProjectRelation) {
		t.Fatalf("cross item=%v", err)
	}
	cross.ContentVersionID = uuid.New()
	if _, err = r.PersistMockReview(ctx, cross); !errors.Is(err, ErrContentVersionNotFound) {
		t.Fatalf("missing version=%v", err)
	}
}

func TestPostgresReviewListPaginationAndReconnect(t *testing.T) {
	db, ctx := openDB(t)
	f := fixture(t, ctx, db)
	r := NewPostgresRepository(db)
	x := create(t, ctx, r, f)
	o, err := r.PersistMockReview(ctx, reviewRequest(f, x, 1, "list-first", reviewFingerprint))
	if err != nil {
		t.Fatal(err)
	}
	// A historical report is seeded directly to exercise the frozen repository list order;
	// normal D2 writes cannot create a second report for the sole frozen v1.
	runID, reportID := uuid.New(), uuid.New()
	if _, err = db.Exec(ctx, "INSERT INTO workflow_runs(id,project_id,content_item_id,content_version_id,provider_key,workflow_key,subject_type,subject_id,status,idempotency_key,request_fingerprint,input_json,output_json,finished_at,created_at) VALUES($1,$2,$3,$4,'mock','content_mock_review','content_item',$3,'succeeded','history',$5,'{}','{}',NOW()-interval '1 hour',NOW()-interval '1 hour')", runID, f.project, x.Detail.Item.ID, x.Detail.CurrentVersion.ID, "3333333333333333333333333333333333333333333333333333333333333333"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO review_reports(id,project_id,content_item_id,content_version_id,workflow_run_id,provider_key,status,conclusion,score,summary,created_at,completed_at) VALUES($1,$2,$3,$4,$5,'mock','completed','pass',100,'old',NOW()-interval '1 hour',NOW()-interval '1 hour')", reportID, f.project, x.Detail.Item.ID, x.Detail.CurrentVersion.ID, runID); err != nil {
		t.Fatal(err)
	}
	// Tie the timestamps so this also proves the required id DESC tie-breaker.
	if _, err = db.Exec(ctx, "UPDATE review_reports SET created_at=(SELECT created_at FROM review_reports WHERE id=$1) WHERE id=$2", o.Review.ID, reportID); err != nil {
		t.Fatal(err)
	}
	first, second := o.Review.ID, reportID
	if first.String() < second.String() {
		first, second = second, first
	}
	list, err := r.ListReviews(ctx, x.Detail.Item.ID, 1, 0)
	if err != nil || list.Total != 2 || len(list.Items) != 1 || list.Items[0].ID != first {
		t.Fatalf("list=%+v %v", list, err)
	}
	page, err := r.ListReviews(ctx, x.Detail.Item.ID, 1, 1)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != second {
		t.Fatalf("page=%+v %v", page, err)
	}
	if _, err = r.ListReviews(ctx, x.Detail.Item.ID, 0, 0); !errors.Is(err, ErrInvalidReviewResult) {
		t.Fatalf("pagination=%v", err)
	}
	cfg := db.Config().Copy()
	db.Close()
	re, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(re.Close)
	t.Cleanup(func() {
		_, _ = re.Exec(context.Background(), "DELETE FROM projects WHERE id=$1 OR id=$2", f.project, f.other)
	})
	got, err := NewPostgresRepository(re).GetReview(ctx, o.Review.ID)
	if err != nil || got.Review.ID != o.Review.ID {
		t.Fatalf("reconnect=%+v %v", got, err)
	}
}

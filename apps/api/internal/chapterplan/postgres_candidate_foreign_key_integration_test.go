package chapterplan

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresCandidateBasePlanCompositeForeignKey(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	_, candidate := newCandidateForeignKeyFixture(t, ctx, db)
	samePlan, foreignPlan := seedCandidateForeignKeyPlans(t, ctx, db, candidate.ProjectID)

	if _, err := db.Exec(ctx, "UPDATE chapter_plan_candidates SET base_chapter_plan_id=$2 WHERE id=$1", candidate.ID, samePlan.ID); err != nil {
		t.Fatalf("same-project base plan should be accepted: %v", err)
	}
	assertCandidateBaseReference(t, ctx, db, candidate.ID, "base_chapter_plan_id", samePlan.ID)
	if _, err := db.Exec(ctx, "UPDATE chapter_plan_candidates SET base_chapter_plan_id=NULL WHERE id=$1", candidate.ID); err != nil {
		t.Fatal(err)
	}

	assertForeignCandidateReferenceRollsBack(t, ctx, db, candidate.ID, "base_chapter_plan_id", foreignPlan.ID, "chapter_plan_candidates_base_plan_fk")
}

func TestPostgresCandidateBaseRevisionCompositeForeignKey(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	_, candidate := newCandidateForeignKeyFixture(t, ctx, db)
	samePlan, foreignPlan := seedCandidateForeignKeyPlans(t, ctx, db, candidate.ProjectID)
	sameRevision := seedCandidateForeignKeyRevision(t, ctx, db, samePlan)
	foreignRevision := seedCandidateForeignKeyRevision(t, ctx, db, foreignPlan)

	if _, err := db.Exec(ctx, "UPDATE chapter_plan_candidates SET base_revision_id=$2 WHERE id=$1", candidate.ID, sameRevision); err != nil {
		t.Fatalf("same-project base revision should be accepted: %v", err)
	}
	assertCandidateBaseReference(t, ctx, db, candidate.ID, "base_revision_id", sameRevision)
	if _, err := db.Exec(ctx, "UPDATE chapter_plan_candidates SET base_revision_id=NULL WHERE id=$1", candidate.ID); err != nil {
		t.Fatal(err)
	}

	assertForeignCandidateReferenceRollsBack(t, ctx, db, candidate.ID, "base_revision_id", foreignRevision, "chapter_plan_candidates_base_revision_fk")
}

func TestPostgresCandidateBasePlanDeleteIsRestricted(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	_, candidate := newCandidateForeignKeyFixture(t, ctx, db)
	samePlan, _ := seedCandidateForeignKeyPlans(t, ctx, db, candidate.ProjectID)
	if _, err := db.Exec(ctx, "UPDATE chapter_plan_candidates SET base_chapter_plan_id=$2 WHERE id=$1", candidate.ID, samePlan.ID); err != nil {
		t.Fatal(err)
	}

	assertDeleteIsRestricted(t, ctx, db, "DELETE FROM chapter_plans WHERE id=$1", samePlan.ID, "chapter_plan_candidates_base_plan_fk")
	assertCandidateAndBaseRemain(t, ctx, db, candidate.ID, "base_chapter_plan_id", "chapter_plans", samePlan.ID)
}

func TestPostgresCandidateBaseRevisionDeleteIsRestricted(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	_, candidate := newCandidateForeignKeyFixture(t, ctx, db)
	samePlan, _ := seedCandidateForeignKeyPlans(t, ctx, db, candidate.ProjectID)
	revisionID := seedCandidateForeignKeyRevision(t, ctx, db, samePlan)
	if _, err := db.Exec(ctx, "UPDATE chapter_plan_candidates SET base_revision_id=$2 WHERE id=$1", candidate.ID, revisionID); err != nil {
		t.Fatal(err)
	}

	assertDeleteIsRestricted(t, ctx, db, "DELETE FROM chapter_plan_revisions WHERE id=$1", revisionID, "chapter_plan_candidates_base_revision_fk")
	assertCandidateAndBaseRemain(t, ctx, db, candidate.ID, "base_revision_id", "chapter_plan_revisions", revisionID)
}

func TestCandidateForeignKeyErrorsAreSafelyMapped(t *testing.T) {
	raw := &pgconn.PgError{Code: "23503", ConstraintName: "chapter_plan_candidates_base_revision_fk", Message: "insert or update on table chapter_plan_candidates violates foreign key constraint"}
	if got := classify(raw); !errors.Is(got, ErrInvalidReference) {
		t.Fatalf("repository foreign-key error=%v, want ErrInvalidReference", got)
	}
	if got := mapError(classify(raw)); !errors.Is(got, ErrValidation) {
		t.Fatalf("application foreign-key error=%v, want ErrValidation", got)
	}
}

func assertDeleteIsRestricted(t *testing.T, ctx context.Context, db *pgxpool.Pool, query string, id uuid.UUID, constraint string) {
	t.Helper()
	_, err := db.Exec(ctx, query, id)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != constraint {
		t.Fatalf("delete error=%v, want PostgreSQL 23503/%s", err, constraint)
	}
}

func assertCandidateAndBaseRemain(t *testing.T, ctx context.Context, db *pgxpool.Pool, candidateID uuid.UUID, candidateColumn string, baseTable string, baseID uuid.UUID) {
	t.Helper()
	var candidateBaseID uuid.UUID
	if err := db.QueryRow(ctx, "SELECT "+candidateColumn+" FROM chapter_plan_candidates WHERE id=$1", candidateID).Scan(&candidateBaseID); err != nil {
		t.Fatal(err)
	}
	if candidateBaseID != baseID {
		t.Fatalf("candidate %s=%s, want %s", candidateColumn, candidateBaseID, baseID)
	}
	var count int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM "+baseTable+" WHERE id=$1", baseID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("base %s id=%s count=%d, want 1", baseTable, baseID, count)
	}
}

func newCandidateForeignKeyFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) (*Repository, Candidate) {
	t.Helper()
	repo, _, candidate := newBulkAdoptFixture(t, ctx, db)
	return repo, candidate
}

func seedCandidateForeignKeyPlans(t *testing.T, ctx context.Context, db *pgxpool.Pool, projectID uuid.UUID) (Plan, Plan) {
	t.Helper()
	repo := mustNewRepo(t, db)
	foreignProjectID := uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','fk-test')", foreignProjectID, "foreign-fk-"+foreignProjectID.String()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE id=$1", foreignProjectID) })

	samePlan := Plan{ID: uuid.New(), ProjectID: projectID, ChapterNo: 501, Title: "same", Summary: "same", CreatedBy: "fk-test"}
	foreignPlan := Plan{ID: uuid.New(), ProjectID: foreignProjectID, ChapterNo: 501, Title: "foreign", Summary: "foreign", CreatedBy: "fk-test"}
	if err := repo.SaveMock(ctx, Run{ID: uuid.New(), ProjectID: samePlan.ProjectID}, []Plan{samePlan}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveMock(ctx, Run{ID: uuid.New(), ProjectID: foreignPlan.ProjectID}, []Plan{foreignPlan}); err != nil {
		t.Fatal(err)
	}
	return samePlan, foreignPlan
}

func seedCandidateForeignKeyRevision(t *testing.T, ctx context.Context, db *pgxpool.Pool, plan Plan) uuid.UUID {
	t.Helper()
	revisionID := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO chapter_plan_revisions (id,chapter_plan_id,project_id,revision_no,snapshot,change_type,created_by) VALUES ($1,$2,$3,2,'{}'::jsonb,'manual_create','fk-test')`, revisionID, plan.ID, plan.ProjectID); err != nil {
		t.Fatal(err)
	}
	return revisionID
}

func assertForeignCandidateReferenceRollsBack(t *testing.T, ctx context.Context, db *pgxpool.Pool, candidateID uuid.UUID, column string, foreignID uuid.UUID, constraint string) {
	t.Helper()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "UPDATE chapter_plan_candidates SET "+column+"=NULL WHERE id=$1", candidateID); err != nil {
		t.Fatal(err)
	}
	partialID := uuid.New()
	_, err = tx.Exec(ctx, "INSERT INTO chapter_plan_candidates (id,batch_id,project_id,chapter_no,sort_order,"+column+",generated_snapshot,current_snapshot,diff_type,status,created_by,updated_by,version) SELECT $1,batch_id,project_id,chapter_no+1000,sort_order+1000,$3,generated_snapshot,current_snapshot,diff_type,status,created_by,updated_by,1 FROM chapter_plan_candidates WHERE id=$2", partialID, candidateID, foreignID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" || pgErr.ConstraintName != constraint {
		t.Fatalf("cross-project %s error=%v, want PostgreSQL 23503/%s", column, err, constraint)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	var baseID *uuid.UUID
	if err = db.QueryRow(ctx, "SELECT "+column+" FROM chapter_plan_candidates WHERE id=$1", candidateID).Scan(&baseID); err != nil {
		t.Fatal(err)
	}
	if baseID != nil {
		t.Fatalf("failed transaction left partial %s=%s", column, *baseID)
	}
	var partialCount int
	if err = db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_candidates WHERE id=$1", partialID).Scan(&partialCount); err != nil {
		t.Fatal(err)
	}
	if partialCount != 0 {
		t.Fatalf("failed transaction persisted %d partial candidate rows", partialCount)
	}
}

func assertCandidateBaseReference(t *testing.T, ctx context.Context, db *pgxpool.Pool, candidateID uuid.UUID, column string, want uuid.UUID) {
	t.Helper()
	var got uuid.UUID
	if err := db.QueryRow(ctx, "SELECT "+column+" FROM chapter_plan_candidates WHERE id=$1", candidateID).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s=%s, want %s", column, got, want)
	}
}

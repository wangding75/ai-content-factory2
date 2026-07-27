package chapterplan

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
)

func TestPostgresIngestionClassifiesIdenticalCandidateAsNoChange(t *testing.T) {
	batch := ingestCandidateDiffFixture(t, nil)
	assertIngestedCandidateDiffType(t, batch, 1, "no_change")
	assertIngestedCandidateDiffType(t, batch, 2, "new")
}

func TestPostgresIngestionClassifiesChangedCandidateAsReplace(t *testing.T) {
	batch := ingestCandidateDiffFixture(t, func(candidate *NormalizedCandidate) {
		candidate.Summary = "changed summary"
	})
	assertIngestedCandidateDiffType(t, batch, 1, "replace")
}

func TestPostgresIngestionNormalizesReferenceOrdering(t *testing.T) {
	batch := ingestCandidateDiffFixture(t, nil)
	assertIngestedCandidateDiffType(t, batch, 1, "no_change")
}

func ingestCandidateDiffFixture(t *testing.T, mutate func(*NormalizedCandidate)) CandidateBatch {
	t.Helper()
	db, ctx := openIntegrationDB(t)
	fixture := newFixture(t, ctx, db)
	repo := mustNewRepo(t, db)
	basePlan := plan(fixture, 1)
	basePlan.Title = "One"
	basePlan.Summary = "one"
	save(t, ctx, repo, basePlan)

	input := newRuntimeValidationInput(t, ctx, db, fixture)
	input.Run.RunID = uuid.New()
	input.NormalizedOutput.SourceWorkflowRunID = input.Run.RunID
	input.Context.StorylineSnapshot = json.RawMessage(fmt.Sprintf(
		`{"available":[{"id":%q},{"id":%q}],"materials":[{"materialId":%q},{"materialId":%q}],"foreshadowings":[{"id":%q},{"id":%q}]}`,
		fixture.storylines[0], fixture.storylines[1], fixture.materials[0], fixture.materials[1], fixture.foreshadowings[0], fixture.foreshadowings[1],
	))

	candidate := &input.NormalizedOutput.Candidates[0]
	candidate.StorylineRefs = []NormalizedReference{
		{ID: fixture.storylines[1], ProjectID: fixture.project, Label: "storyline two", Relation: "secondary", Position: 1, Version: 1},
		{ID: fixture.storylines[0], ProjectID: fixture.project, Label: "storyline one", Relation: "primary", Position: 0, Version: 1},
		{ID: fixture.storylines[0], ProjectID: fixture.project, Label: "duplicate", Relation: "primary", Position: 2, Version: 1},
	}
	candidate.MaterialRefs = []NormalizedReference{
		{ID: fixture.materials[1], ProjectID: fixture.project, Label: "material two", Relation: "material_ref", Position: 1, Version: 1},
		{ID: fixture.materials[0], ProjectID: fixture.project, Label: "material one", Relation: "material_ref", Position: 0, Version: 1},
		{ID: fixture.materials[0], ProjectID: fixture.project, Label: "duplicate", Relation: "material_ref", Position: 2, Version: 1},
	}
	candidate.ForeshadowingRefs = []NormalizedReference{
		{ID: fixture.foreshadowings[1], ProjectID: fixture.project, Label: "foreshadowing two", Relation: "foreshadowing_ref", Position: 1, Version: 1},
		{ID: fixture.foreshadowings[0], ProjectID: fixture.project, Label: "foreshadowing one", Relation: "foreshadowing_ref", Position: 0, Version: 1},
		{ID: fixture.foreshadowings[0], ProjectID: fixture.project, Label: "duplicate", Relation: "foreshadowing_ref", Position: 2, Version: 1},
	}

	baseCandidate := *candidate
	baseCandidate.StorylineRefs = []NormalizedReference{candidate.StorylineRefs[1], candidate.StorylineRefs[0]}
	baseCandidate.MaterialRefs = []NormalizedReference{candidate.MaterialRefs[1], candidate.MaterialRefs[0]}
	baseCandidate.ForeshadowingRefs = []NormalizedReference{candidate.ForeshadowingRefs[1], candidate.ForeshadowingRefs[0]}
	baseSnapshot, err := json.Marshal(baseCandidate)
	if err != nil {
		t.Fatal(err)
	}
	revisionID := uuid.New()
	if _, err = db.Exec(ctx, `INSERT INTO chapter_plan_revisions(id,chapter_plan_id,project_id,revision_no,snapshot,change_type,created_by) VALUES($1,$2,$3,1,$4,'manual_create','diff-test')`, revisionID, basePlan.ID, fixture.project, baseSnapshot); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "UPDATE chapter_plans SET current_revision_id=$2 WHERE id=$1", basePlan.ID, revisionID); err != nil {
		t.Fatal(err)
	}
	input.Context.BaseChapterPlans = []BaseChapterPlanContext{{ID: basePlan.ID, ChapterNo: 1, Version: 1, RevisionID: &revisionID, Snapshot: baseSnapshot}}
	if mutate != nil {
		mutate(candidate)
	}
	seedRuntimeValidationRun(t, ctx, db, input)
	batch, err := NewResultIngestor(db).Ingest(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	return batch
}

func assertIngestedCandidateDiffType(t *testing.T, batch CandidateBatch, chapterNo int, want string) {
	t.Helper()
	db, ctx := openIntegrationDB(t)
	var got string
	if err := db.QueryRow(ctx, "SELECT diff_type FROM chapter_plan_candidates WHERE batch_id=$1 AND chapter_no=$2", batch.ID, chapterNo).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("chapter %d diff_type=%q, want %q", chapterNo, got, want)
	}
}

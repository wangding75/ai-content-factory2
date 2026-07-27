package chapterplan_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
)

func TestSchemaAndSemanticValidation(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()
	digest := "a1b2c3d4e5f60123456789abcdef0123456789abcdef0123456789abcdef0123"

	validInput := chapterplan.IngestInput{
		Run: chapterplan.RunReference{
			RunID:     runID,
			ProjectID: projectID,
		},
		Context: chapterplan.GenerationContextSnapshot{
			InputDigest:   digest,
			InputSnapshot: json.RawMessage(`{"generationMode":"full","target":{"startChapterNo":1,"endChapterNo":2,"requestedChapterCount":2}}`),
		},
		NormalizedOutput: chapterplan.NormalizedChapterPlanOutput{
			ProjectID:           projectID,
			GenerationMode:      "full",
			Target:              chapterplan.BatchTarget{StartChapterNo: 1, EndChapterNo: 2, RequestedChapterCount: 2},
			SourceWorkflowRunID: runID,
			Candidates: []chapterplan.NormalizedCandidate{
				{
					ChapterNo:      1,
					Title:          "Chapter 1",
					Summary:        "Summary 1",
					ChapterPurpose: "plot_advance",
					StorylineRefs:  []chapterplan.NormalizedReference{{ID: uuid.New(), ProjectID: projectID, Label: "Main", Relation: "primary", Position: 0, Version: 1}},
					GenerationBasis: chapterplan.GenerationBasis{
						ContextSummary: "Context",
					},
				},
				{
					ChapterNo:      2,
					Title:          "Chapter 2",
					Summary:        "Summary 2",
					ChapterPurpose: "conflict_escalation",
					StorylineRefs:  []chapterplan.NormalizedReference{{ID: uuid.New(), ProjectID: projectID, Label: "Main", Relation: "primary", Position: 0, Version: 1}},
					GenerationBasis: chapterplan.GenerationBasis{
						ContextSummary: "Context",
					},
				},
			},
			Metadata: chapterplan.OutputMetadata{
				InputDigest:         digest,
				GeneratedAt:         "2026-07-27T10:00:00Z",
				SafeProviderSummary: "OK",
			},
		},
	}
	storylineID := validInput.NormalizedOutput.Candidates[0].StorylineRefs[0].ID
	secondStorylineID := validInput.NormalizedOutput.Candidates[1].StorylineRefs[0].ID
	validInput.Context.StorylineSnapshot, _ = json.Marshal(map[string]any{"available": []map[string]any{{"id": storylineID}, {"id": secondStorylineID}}, "materials": []map[string]any{}, "foreshadowings": []map[string]any{}})

	// 1. Valid input -> PASS
	if err := chapterplan.ValidateNormalizedOutput(validInput); err != nil {
		t.Fatalf("expected valid input to pass validation, got: %v", err)
	}

	// 2. Project ID Mismatch -> FAIL
	invalidProject := validInput
	invalidProject.NormalizedOutput.ProjectID = uuid.New()
	if err := chapterplan.ValidateNormalizedOutput(invalidProject); err == nil {
		t.Errorf("expected error for project ID mismatch")
	}

	// 3. Source Workflow Run ID Mismatch -> FAIL
	invalidRun := validInput
	invalidRun.NormalizedOutput.SourceWorkflowRunID = uuid.New()
	if err := chapterplan.ValidateNormalizedOutput(invalidRun); err == nil {
		t.Errorf("expected error for run ID mismatch")
	}

	// 4. Input Digest Mismatch -> FAIL
	invalidDigest := validInput
	invalidDigest.NormalizedOutput.Metadata.InputDigest = "0000000000000000000000000000000000000000000000000000000000000000"
	if err := chapterplan.ValidateNormalizedOutput(invalidDigest); err == nil {
		t.Errorf("expected error for digest mismatch")
	}

	// 5. Duplicate Chapter Numbers -> FAIL
	duplicateChapter := validInput
	duplicateChapter.NormalizedOutput.Candidates[1].ChapterNo = 1
	if err := chapterplan.ValidateNormalizedOutput(duplicateChapter); err == nil {
		t.Errorf("expected error for duplicate chapter numbers")
	}

	// 6. Cross-Project Reference -> FAIL
	crossProjectRef := validInput
	crossProjectRef.NormalizedOutput.Candidates[0].StorylineRefs[0].ProjectID = uuid.New()
	if err := chapterplan.ValidateNormalizedOutput(crossProjectRef); err == nil {
		t.Errorf("expected error for cross-project reference")
	}

	// 7. A project-local but unfrozen storyline is equally invalid.
	unfrozenRef := validInput
	unfrozenRef.NormalizedOutput.Candidates[0].StorylineRefs[0].ID = uuid.New()
	if err := chapterplan.ValidateNormalizedOutput(unfrozenRef); err == nil {
		t.Errorf("expected error for unfrozen storyline reference")
	}
}

func TestSnapshotMappingAndDiffTypeLogic(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()
	digest := "a1b2c3d4e5f60123456789abcdef0123456789abcdef0123456789abcdef0123"
	planID := uuid.New()

	baseSnap, _ := json.Marshal(map[string]any{
		"chapterNo":         1,
		"title":             "Base Title 1",
		"summary":           "Base Summary 1",
		"chapterPurpose":    "plot_advance",
		"storylineRefs":     []any{},
		"materialRefs":      []any{},
		"foreshadowingRefs": []any{},
		"generationBasis": map[string]any{
			"contextSummary": "",
		},
	})

	ctxSnap := chapterplan.GenerationContextSnapshot{
		InputDigest:   digest,
		InputSnapshot: json.RawMessage(`{"generationMode":"full","target":{"startChapterNo":1,"endChapterNo":2,"requestedChapterCount":2}}`),
		BaseChapterPlans: []chapterplan.BaseChapterPlanContext{
			{
				ID:        planID,
				ChapterNo: 1,
				Version:   2,
				Snapshot:  baseSnap,
			},
		},
	}

	input := chapterplan.IngestInput{
		Run: chapterplan.RunReference{
			RunID:     runID,
			ProjectID: projectID,
		},
		Context: ctxSnap,
		NormalizedOutput: chapterplan.NormalizedChapterPlanOutput{
			ProjectID:           projectID,
			GenerationMode:      "full",
			Target:              chapterplan.BatchTarget{StartChapterNo: 1, EndChapterNo: 2, RequestedChapterCount: 2},
			SourceWorkflowRunID: runID,
			Candidates: []chapterplan.NormalizedCandidate{
				{
					ChapterNo:      1,
					Title:          "Base Title 1",
					Summary:        "Base Summary 1",
					ChapterPurpose: "plot_advance",
					StorylineRefs: []chapterplan.NormalizedReference{
						{ID: uuid.New(), ProjectID: projectID, Label: "Storyline 1", Relation: "primary", Position: 0, Version: 1},
					},
					MaterialRefs:      []chapterplan.NormalizedReference{},
					ForeshadowingRefs: []chapterplan.NormalizedReference{},
					GenerationBasis:   chapterplan.GenerationBasis{ContextSummary: ""},
				},
				{
					ChapterNo:      2,
					Title:          "New Chapter 2",
					Summary:        "Summary 2",
					ChapterPurpose: "transition",
					StorylineRefs: []chapterplan.NormalizedReference{
						{ID: uuid.New(), ProjectID: projectID, Label: "Storyline 1", Relation: "primary", Position: 0, Version: 1},
					},
					MaterialRefs:      []chapterplan.NormalizedReference{},
					ForeshadowingRefs: []chapterplan.NormalizedReference{},
					GenerationBasis:   chapterplan.GenerationBasis{ContextSummary: ""},
				},
			},
			Metadata: chapterplan.OutputMetadata{
				InputDigest:         digest,
				GeneratedAt:         "2026-07-27T10:00:00Z",
				SafeProviderSummary: "OK",
			},
		},
	}

	if err := chapterplan.ValidateNormalizedOutput(input); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

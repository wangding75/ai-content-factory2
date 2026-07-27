package chapterplan_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
)

func TestCandidateEditAndCompareLogic(t *testing.T) {
	snap1, _ := json.Marshal(map[string]any{
		"chapterNo":      1,
		"title":          "Original Title",
		"summary":        "Original Summary",
		"chapterPurpose": "plot_advance",
		"storylineRefs":  []any{},
		"materialRefs":   []any{},
		"foreshadowingRefs": []any{},
	})

	snap2, _ := json.Marshal(map[string]any{
		"chapterNo":      1,
		"title":          "Edited Title",
		"summary":        "Original Summary",
		"chapterPurpose": "plot_advance",
		"storylineRefs":  []any{},
		"materialRefs":   []any{},
		"foreshadowingRefs": []any{},
	})

	revID := uuid.New()
	targetPlan := &chapterplan.Plan{
		ID:        uuid.New(),
		ChapterNo: 1,
		Title:     "Original Title",
		Summary:   "Original Summary",
	}

	// 1. ComputeDiffTypeAndStale when snapshots match
	diffType, isStale := chapterplan.ComputeDiffTypeAndStale(snap1, snap1, &revID, targetPlan, &revID)
	if diffType != "no_change" || isStale {
		t.Errorf("expected no_change and not stale, got diffType=%s isStale=%v", diffType, isStale)
	}

	// 2. ComputeDiffTypeAndStale when candidate is edited
	diffType, isStale = chapterplan.ComputeDiffTypeAndStale(snap2, snap1, &revID, targetPlan, &revID)
	if diffType != "replace" || isStale {
		t.Errorf("expected replace and not stale, got diffType=%s isStale=%v", diffType, isStale)
	}

	// 3. ComputeDiffTypeAndStale when target plan revision changed (stale conflict)
	otherRevID := uuid.New()
	diffType, isStale = chapterplan.ComputeDiffTypeAndStale(snap1, snap1, &revID, targetPlan, &otherRevID)
	if diffType != "stale_conflict" || !isStale {
		t.Errorf("expected stale_conflict and isStale=true, got diffType=%s isStale=%v", diffType, isStale)
	}

	// 4. CalculateCandidateDiff
	cand := chapterplan.Candidate{
		ID:              uuid.New(),
		Version:         2,
		BaseSnapshot:    snap1,
		CurrentSnapshot: snap2,
		Status:          "pending",
		DiffType:        "replace",
		BaseRevisionID:  &revID,
	}

	diff := chapterplan.CalculateCandidateDiff(cand, targetPlan)
	if diff.Stale {
		t.Errorf("expected diff.Stale to be false")
	}
	if len(diff.Entries) == 0 {
		t.Fatalf("expected diff entries")
	}

	titleChanged := false
	for _, entry := range diff.Entries {
		if entry.Path == "title" {
			if entry.ChangeType != "changed" {
				t.Errorf("expected title changeType=changed, got %s", entry.ChangeType)
			}
			titleChanged = true
		}
	}
	if !titleChanged {
		t.Errorf("expected title diff entry")
	}
}

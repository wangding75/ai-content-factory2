package material

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestMergeUsageUpdatesOnlySuppliedFields(t *testing.T) {
	start, end := 1, 3
	current := ProjectMaterialUsage{ID: uuid.New(), UsageType: "lead", RoleName: "old", Notes: "old", StartChapter: &start, EndChapter: &end}
	role := "new"
	next, err := mergeUsage(current, UpdateProjectMaterialUsageRequest{ExpectedVersion: ptr(1), RoleName: &role, EndChapter: json.RawMessage("null")})
	if err != nil || next.UsageType != "lead" || next.RoleName != "new" || next.Notes != "old" || next.StartChapter == nil || next.EndChapter != nil {
		t.Fatalf("next=%#v err=%v", next, err)
	}
}

func TestMergeUsageRejectsInvalidChapterRange(t *testing.T) {
	current := ProjectMaterialUsage{UsageType: "lead"}
	_, err := mergeUsage(current, UpdateProjectMaterialUsageRequest{ExpectedVersion: ptr(1), StartChapter: json.RawMessage("3"), EndChapter: json.RawMessage("2")})
	if err != ErrValidation {
		t.Fatalf("error=%v", err)
	}
}

func ptr(value int) *int { return &value }

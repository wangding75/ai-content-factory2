package chapterplan

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
)

func TestFreezeStorylinesUsesRuntimeJSONFieldNames(t *testing.T) {
	projectID := uuid.New()
	storylineID := uuid.New()
	now := time.Date(2026, 7, 28, 5, 27, 37, 0, time.UTC)
	raw, err := json.Marshal(freezeStorylines([]storyline.PlotLine{{
		ID:        storylineID,
		ProjectID: projectID,
		Type:      "main",
		Relation:  "root",
		Name:      "Iteration 14.5 Local n8n Test Storyline",
		Summary:   "Isolated verification storyline.",
		Status:    "active",
		SortOrder: 0,
		Version:   1,
		CreatedBy: "must-not-leak",
		CreatedAt: now,
		UpdatedAt: now,
	}}))
	if err != nil {
		t.Fatalf("marshal frozen storylines: %v", err)
	}

	var decoded []map[string]json.RawMessage
	if err = json.Unmarshal(raw, &decoded); err != nil || len(decoded) != 1 {
		t.Fatalf("decode frozen storylines: value=%s err=%v", raw, err)
	}
	item := decoded[0]
	for _, field := range []string{"id", "project_id", "name", "relation", "version"} {
		if _, ok := item[field]; !ok {
			t.Fatalf("frozen storyline missing %q: %s", field, raw)
		}
	}
	for _, field := range []string{"ID", "ProjectID", "Name", "CreatedBy", "created_by"} {
		if _, ok := item[field]; ok {
			t.Fatalf("frozen storyline exposed invalid field %q: %s", field, raw)
		}
	}

	var gotID uuid.UUID
	if err = json.Unmarshal(item["id"], &gotID); err != nil || gotID != storylineID {
		t.Fatalf("frozen storyline id=%s want=%s err=%v", gotID, storylineID, err)
	}
}

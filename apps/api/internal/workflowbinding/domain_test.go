package workflowbinding

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAllStagesReturnsFourFixedValues(t *testing.T) {
	stages := AllStages()
	if len(stages) != 4 {
		t.Fatalf("AllStages() len = %d, want 4", len(stages))
	}
	seen := map[WorkflowBindingStage]bool{}
	for _, s := range stages {
		seen[s] = true
	}
	for _, want := range []WorkflowBindingStage{
		StageChapterPlanning,
		StageContentGeneration,
		StageReview,
		StageRewrite,
	} {
		if !seen[want] {
			t.Fatalf("AllStages() missing %q", want)
		}
	}
}

func TestParseStageAcceptsFourValidStages(t *testing.T) {
	for _, raw := range []string{
		"chapter_planning",
		"content_generation",
		"review",
		"rewrite",
	} {
		s, err := ParseStage(raw)
		if err != nil {
			t.Fatalf("ParseStage(%q) error = %v", raw, err)
		}
		if s.String() != raw {
			t.Fatalf("ParseStage(%q) = %q", raw, s)
		}
	}
}

func TestParseStageRejectsInvalidStage(t *testing.T) {
	for _, raw := range []string{"", "unknown", "chapter_planning_extra", "content_gen"} {
		if _, err := ParseStage(raw); err != ErrInvalidStage {
			t.Fatalf("ParseStage(%q) error = %v, want ErrInvalidStage", raw, err)
		}
	}
}

func TestNewCreatesBindingWithVersionOne(t *testing.T) {
	id := uuid.New()
	projectID := uuid.New()
	wfID := uuid.New()
	b, err := New(id, projectID, wfID, StageChapterPlanning)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if b.ID != id {
		t.Fatalf("ID = %s, want %s", b.ID, id)
	}
	if b.ProjectID != projectID {
		t.Fatalf("ProjectID = %s, want %s", b.ProjectID, projectID)
	}
	if b.WorkflowConfigurationID != wfID {
		t.Fatalf("WorkflowConfigurationID = %s, want %s", b.WorkflowConfigurationID, wfID)
	}
	if b.Stage != StageChapterPlanning {
		t.Fatalf("Stage = %s, want %s", b.Stage, StageChapterPlanning)
	}
	if b.Version != 1 {
		t.Fatalf("Version = %d, want 1", b.Version)
	}
	if b.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}
	if b.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt is zero")
	}
}

func TestNewRejectsNilID(t *testing.T) {
	if _, err := New(uuid.Nil, uuid.New(), uuid.New(), StageReview); err != ErrValidation {
		t.Fatalf("New with nil ID error = %v, want ErrValidation", err)
	}
}

func TestNewRejectsNilProjectID(t *testing.T) {
	if _, err := New(uuid.New(), uuid.Nil, uuid.New(), StageReview); err != ErrValidation {
		t.Fatalf("New with nil ProjectID error = %v, want ErrValidation", err)
	}
}

func TestNewRejectsNilWorkflowConfigurationID(t *testing.T) {
	if _, err := New(uuid.New(), uuid.New(), uuid.Nil, StageReview); err != ErrValidation {
		t.Fatalf("New with nil WorkflowConfigurationID error = %v, want ErrValidation", err)
	}
}

func TestNewRejectsInvalidStage(t *testing.T) {
	if _, err := New(uuid.New(), uuid.New(), uuid.New(), WorkflowBindingStage("invalid")); err != ErrInvalidStage {
		t.Fatalf("New with invalid stage error = %v, want ErrInvalidStage", err)
	}
}

func TestNewFromDBRejectsVersionBelowOne(t *testing.T) {
	now := time.Now().UTC()
	if _, err := NewFromDB(uuid.New(), uuid.New(), uuid.New(), StageRewrite, 0, now, now); err != ErrValidation {
		t.Fatalf("NewFromDB with version 0 error = %v, want ErrValidation", err)
	}
	if _, err := NewFromDB(uuid.New(), uuid.New(), uuid.New(), StageRewrite, -1, now, now); err != ErrValidation {
		t.Fatalf("NewFromDB with version -1 error = %v, want ErrValidation", err)
	}
}

func TestNewFromDBRejectsNilID(t *testing.T) {
	now := time.Now().UTC()
	if _, err := NewFromDB(uuid.Nil, uuid.New(), uuid.New(), StageRewrite, 1, now, now); err != ErrValidation {
		t.Fatalf("NewFromDB with nil ID error = %v, want ErrValidation", err)
	}
}

func TestNewFromDBAcceptsValidData(t *testing.T) {
	id := uuid.New()
	projectID := uuid.New()
	wfID := uuid.New()
	now := time.Now().UTC()
	b, err := NewFromDB(id, projectID, wfID, StageContentGeneration, 5, now, now)
	if err != nil {
		t.Fatalf("NewFromDB() error = %v", err)
	}
	if b.Version != 5 {
		t.Fatalf("Version = %d, want 5", b.Version)
	}
	if b.CreatedAt != now {
		t.Fatalf("CreatedAt = %v, want %v", b.CreatedAt, now)
	}
}

func TestRebindToIncrementsVersion(t *testing.T) {
	id := uuid.New()
	projectID := uuid.New()
	oldWfID := uuid.New()
	newWfID := uuid.New()
	b, _ := New(id, projectID, oldWfID, StageChapterPlanning)
	rebound, err := b.RebindTo(newWfID)
	if err != nil {
		t.Fatalf("RebindTo() error = %v", err)
	}
	if rebound.Version != 2 {
		t.Fatalf("Version = %d, want 2", rebound.Version)
	}
	if rebound.WorkflowConfigurationID != newWfID {
		t.Fatalf("WorkflowConfigurationID = %s, want %s", rebound.WorkflowConfigurationID, newWfID)
	}
	if rebound.ID != b.ID {
		t.Fatalf("ID changed after rebind: %s -> %s", b.ID, rebound.ID)
	}
	if rebound.ProjectID != b.ProjectID {
		t.Fatalf("ProjectID changed after rebind")
	}
	if rebound.Stage != b.Stage {
		t.Fatalf("Stage changed after rebind")
	}
}

func TestRebindToSameWorkflowIsNoOp(t *testing.T) {
	id := uuid.New()
	projectID := uuid.New()
	wfID := uuid.New()
	b, _ := New(id, projectID, wfID, StageRewrite)
	rebound, err := b.RebindTo(wfID)
	if err != ErrNoChange {
		t.Fatalf("RebindTo same ID error = %v, want ErrNoChange", err)
	}
	if rebound.Version != b.Version {
		t.Fatalf("Version changed from %d to %d", b.Version, rebound.Version)
	}
	if rebound.WorkflowConfigurationID != wfID {
		t.Fatalf("WorkflowConfigurationID changed")
	}
}

func TestRebindToSameWorkflowDoesNotModifyUpdatedAt(t *testing.T) {
	id := uuid.New()
	projectID := uuid.New()
	wfID := uuid.New()
	b, _ := New(id, projectID, wfID, StageContentGeneration)
	rebound, err := b.RebindTo(wfID)
	if err != ErrNoChange {
		t.Fatalf("RebindTo same ID error = %v, want ErrNoChange", err)
	}
	if !rebound.UpdatedAt.Equal(b.UpdatedAt) {
		t.Fatalf("UpdatedAt changed from %v to %v", b.UpdatedAt, rebound.UpdatedAt)
	}
}

func TestRebindToRejectsNilWorkflowConfigurationID(t *testing.T) {
	b, _ := New(uuid.New(), uuid.New(), uuid.New(), StageReview)
	if _, err := b.RebindTo(uuid.Nil); err != ErrValidation {
		t.Fatalf("RebindTo nil ID error = %v, want ErrValidation", err)
	}
}

func TestRebindToUpdatesUpdatedAt(t *testing.T) {
	b, _ := New(uuid.New(), uuid.New(), uuid.New(), StageChapterPlanning)
	rebound, err := b.RebindTo(uuid.New())
	if err != nil {
		t.Fatalf("RebindTo() error = %v", err)
	}
	if rebound.UpdatedAt.Before(b.UpdatedAt) {
		t.Fatalf("UpdatedAt went backwards: before=%v, after=%v", b.UpdatedAt, rebound.UpdatedAt)
	}
}
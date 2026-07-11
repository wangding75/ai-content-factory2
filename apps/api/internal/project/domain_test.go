package project

import "testing"

func TestNewNovelProjectHasDefaults(t *testing.T) {
	p, err := New("Example", TypeNovel, "description")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if p.ID.String() == "" || p.Status != StatusPlanning || p.CurrentStage != StageProjectSetup {
		t.Fatalf("unexpected defaults: %#v", p)
	}
}
func TestNewRejectsInvalidType(t *testing.T) {
	if _, err := New("Example", "screenplay", ""); err != ErrValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}
func TestValidateUpdate(t *testing.T) {
	if err := ValidateUpdate(nil, nil); err != ErrValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
	name := " "
	if err := ValidateUpdate(&name, nil); err != ErrValidation {
		t.Fatalf("expected validation error, got %v", err)
	}
}

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
func TestProjectTypeCatalogueAndCreationUseTheSameEnabledTypes(t *testing.T) {
	want := []string{TypeNovel, TypeShortFilm, TypeSeries, TypeGraphicText, TypeImage}
	types := ProjectTypes()
	if len(types) != len(want) {
		t.Fatalf("catalogue length = %d, want %d", len(types), len(want))
	}
	for i, code := range want {
		if types[i].Code != code || !types[i].Enabled || types[i].Name == "" || types[i].Description == "" || types[i].SortOrder != (i+1)*10 {
			t.Fatalf("invalid catalogue item: %#v", types[i])
		}
		if _, err := New("Example", code, ""); err != nil {
			t.Fatalf("New(%q) error = %v", code, err)
		}
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

package foreshadowing

import (
	"testing"

	"github.com/google/uuid"
)

func TestStatusTransitions(t *testing.T) {
	cases := []struct {
		from, to string
		ok       bool
	}{
		{"planned", "planned", true},
		{"planned", "planted", true},
		{"planted", "paid_off", true},
		{"planned", "paid_off", false},
		{"paid_off", "planted", false},
	}
	for _, test := range cases {
		if valid(test.from, test.to) != test.ok {
			t.Fatalf("%s->%s", test.from, test.to)
		}
	}
}

func TestSameIncludesReferencesAndChapterPlans(t *testing.T) {
	planted, payoff := uuid.New(), uuid.New()
	plantChapter, payoffChapter := 2, 5
	base := Foreshadowing{Title: "title", Description: "description", Priority: "high", Status: "planned", PlantedPlotLineID: &planted, PayoffPlotLineID: &payoff, PlannedPlantChapter: &plantChapter, PlannedPayoffChapter: &payoffChapter}
	if !same(base, base) {
		t.Fatal("identical values must match")
	}
	changed := base
	newPayoff := uuid.New()
	changed.PayoffPlotLineID = &newPayoff
	if same(base, changed) {
		t.Fatal("payoff storyline change must not be idempotent")
	}
	changed = base
	newPayoffChapter := 6
	changed.PlannedPayoffChapter = &newPayoffChapter
	if same(base, changed) {
		t.Fatal("planned payoff chapter change must not be idempotent")
	}
}

package storyline

import "testing"

func TestSameIncludesAllMutableFields(t *testing.T) {
	start, end := 1, 2
	base := PlotLine{Name: "name", Summary: "summary", Status: "active", SortOrder: 1, StartChapter: &start, EndChapter: &end}
	if !same(base, base) {
		t.Fatal("identical values must match")
	}
	changed := base
	changed.SortOrder = 2
	if same(base, changed) {
		t.Fatal("sort_order change must not be idempotent")
	}
}

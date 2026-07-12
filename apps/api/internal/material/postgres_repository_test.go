package material

import "testing"

func TestMaterialOrderUsesContractWhitelist(t *testing.T) {
	for _, sort := range []string{"", "updated_at_desc", "updated_at_asc", "name_asc", "name_desc"} {
		if _, err := materialOrder(sort); err != nil {
			t.Fatalf("sort %q: %v", sort, err)
		}
	}
	if _, err := materialOrder("name; DROP TABLE materials"); err != ErrInvalidSort {
		t.Fatalf("invalid sort error = %v", err)
	}
}

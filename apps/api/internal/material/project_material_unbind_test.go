package material

import (
	"testing"

	"github.com/google/uuid"
)

func TestUnbindProjectMaterialResultKeepsMaterial(t *testing.T) {
	result := UnbindProjectMaterialResult{ProjectID: uuid.New(), MaterialID: uuid.New(), MaterialRetained: true}
	if !result.MaterialRetained || result.Unbound {
		t.Fatalf("result=%#v", result)
	}
}

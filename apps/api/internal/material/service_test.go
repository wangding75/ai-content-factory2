package material

import (
	"encoding/json"
	"testing"
)

func materialForTest() Material {
	return Material{Type: TypeCharacter, Name: "Name", Summary: "Summary", ContentJSON: json.RawMessage("{}"), Tags: []string{"tag"}}
}
func TestValidateMaterialTypes(t *testing.T) {
	for _, typ := range []string{TypeCharacter, TypeWorldview, TypeLocation, TypeOrganization, TypeItem, TypeReference} {
		v := materialForTest()
		v.Type = typ
		if err := validate(v); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
	}
}
func TestValidateMaterialRejectsInvalidType(t *testing.T) {
	v := materialForTest()
	v.Type = "other"
	if err := validate(v); err == nil {
		t.Fatal("expected invalid type")
	}
}
func TestValidateMaterialRejectsInvalidTags(t *testing.T) {
	v := materialForTest()
	v.Tags = []string{"tag", "tag"}
	if err := validate(v); err == nil {
		t.Fatal("expected duplicate tags")
	}
	v = materialForTest()
	v.Tags = []string{""}
	if err := validate(v); err == nil {
		t.Fatal("expected empty tag")
	}
}
func TestCreateMaterialRequiresAllContractFields(t *testing.T) {
	name := "name"
	summary := "summary"
	tags := []string{}
	if _, err := createValue(CreateRequest{Type: TypeItem, Name: &name, Summary: &summary, Tags: &tags}, "actor"); err == nil {
		t.Fatal("expected missing content")
	}
}
func TestMaterialSameContent(t *testing.T) {
	a := materialForTest()
	b := a
	if !same(a, b) {
		t.Fatal("same content not recognized")
	}
	b.Name = "other"
	if same(a, b) {
		t.Fatal("different content treated as same")
	}
}
func TestMaterialRequestHashIsDeterministic(t *testing.T) {
	name, summary := "name", "summary"
	tags := []string{"tag"}
	r := CreateRequest{Type: TypeItem, Name: &name, Summary: &summary, ContentJSON: json.RawMessage("{}"), Tags: &tags}
	a, err := hash(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := hash(r)
	if err != nil || a != b {
		t.Fatalf("hashes %q %q %v", a, b, err)
	}
}

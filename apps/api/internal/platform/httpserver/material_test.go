package httpserver

import (
	"context"
	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeMaterials struct {
	list   material.ListOptions
	key    string
	create material.CreateRequest
	id     uuid.UUID
	patch  material.UpdateRequest
	err    error
}

func (f *fakeMaterials) ListMaterials(_ context.Context, o material.ListOptions) ([]material.Material, int, error) {
	f.list = o
	return []material.Material{{ID: uuid.New(), Type: material.TypeItem, Name: "one", Version: 1}}, 2, f.err
}
func (f *fakeMaterials) CreateMaterial(_ context.Context, r material.CreateRequest, k, _ string) (material.Material, error) {
	f.create = r
	f.key = k
	return material.Material{ID: uuid.New(), Type: r.Type, Name: *r.Name, Summary: *r.Summary, ContentJSON: r.ContentJSON, Tags: *r.Tags, Version: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()}, f.err
}
func (f *fakeMaterials) GetMaterial(_ context.Context, id uuid.UUID) (material.Detail, error) {
	f.id = id
	return material.Detail{References: []material.Reference{}, ReferenceCount: 0}, f.err
}
func (f *fakeMaterials) UpdateMaterial(_ context.Context, id uuid.UUID, r material.UpdateRequest, _ string) (material.Material, error) {
	f.id = id
	f.patch = r
	return material.Material{ID: id, Version: 2, CreatedAt: time.Now(), UpdatedAt: time.Now()}, f.err
}
func TestMaterialListHandlerValidation(t *testing.T) {
	f := &fakeMaterials{}
	for _, q := range []string{"?type=x", "?sort=x", "?limit=0", "?offset=-1"} {
		w := doRequest(listMaterialsHandler(f), http.MethodGet, "/api/v1/materials"+q, "")
		if w.Code != 400 || !strings.Contains(w.Body.String(), "VALIDATION_ERROR") {
			t.Fatal(w.Body.String())
		}
	}
}
func TestMaterialCreateHandlerMapping(t *testing.T) {
	f := &fakeMaterials{}
	body := "{\"type\":\"item\",\"name\":\"n\",\"summary\":\"s\",\"content_json\":{},\"tags_json\":[]}"
	w := doRequest(createMaterialHandler(f), http.MethodPost, "/api/v1/materials", body)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}
func TestMaterialDetailHandlerInvalidID(t *testing.T) {
	f := &fakeMaterials{}
	w := doRequest(getMaterialHandler(f), http.MethodGet, "/api/v1/materials/not-uuid", "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "INVALID_MATERIAL_ID") {
		t.Fatal(w.Body.String())
	}
}

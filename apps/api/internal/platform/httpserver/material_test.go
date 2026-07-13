package httpserver

import (
	"context"
	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"net/http"
	"net/http/httptest"
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

func TestMaterialHandlersNormalRequests(t *testing.T) {
	materialID := uuid.New()
	newMux := func(fake *fakeMaterials) *http.ServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/v1/materials", listMaterialsHandler(fake))
		mux.HandleFunc("POST /api/v1/materials", createMaterialHandler(fake))
		mux.HandleFunc("GET /api/v1/materials/{materialId}", getMaterialHandler(fake))
		mux.HandleFunc("PATCH /api/v1/materials/{materialId}", updateMaterialHandler(fake))
		return mux
	}
	t.Run("list", func(t *testing.T) {
		fake := &fakeMaterials{}
		w := doRequest(newMux(fake), http.MethodGet, "/api/v1/materials?q=%20one%20&type=item&sort=name_asc&limit=2&offset=1", "")
		if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "application/json") || fake.list != (material.ListOptions{Query: "one", Type: "item", Sort: "name_asc", Limit: 2, Offset: 1}) || !strings.Contains(w.Body.String(), "\"total\":2") {
			t.Fatalf("options=%#v response=%s", fake.list, w.Body.String())
		}
	})
	t.Run("create", func(t *testing.T) {
		fake := &fakeMaterials{}
		r := httptest.NewRequest(http.MethodPost, "/api/v1/materials", strings.NewReader("{\"type\":\"item\",\"name\":\"n\",\"summary\":\"s\",\"content_json\":{},\"tags_json\":[]}"))
		r.Header.Set("Idempotency-Key", "normal-create")
		w := httptest.NewRecorder()
		newMux(fake).ServeHTTP(w, r)
		if w.Code != http.StatusCreated || !strings.Contains(w.Header().Get("Content-Type"), "application/json") || fake.key != "normal-create" || fake.create.Name == nil || *fake.create.Name != "n" || !strings.Contains(w.Body.String(), "\"name\":\"n\"") {
			t.Fatalf("request=%#v response=%s", fake.create, w.Body.String())
		}
	})
	t.Run("detail", func(t *testing.T) {
		fake := &fakeMaterials{}
		w := doRequest(newMux(fake), http.MethodGet, "/api/v1/materials/"+materialID.String(), "")
		if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "application/json") || fake.id != materialID || strings.Contains(w.Body.String(), "project_id") || strings.Contains(w.Body.String(), "usage_type") || strings.Contains(w.Body.String(), "role_name") || strings.Contains(w.Body.String(), "notes") {
			t.Fatalf("response=%s", w.Body.String())
		}
	})
	t.Run("patch", func(t *testing.T) {
		fake := &fakeMaterials{}
		w := doRequest(newMux(fake), http.MethodPatch, "/api/v1/materials/"+materialID.String(), "{\"expected_version\":1,\"name\":\"changed\"}")
		if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "application/json") || fake.id != materialID || fake.patch.ExpectedVersion == nil || *fake.patch.ExpectedVersion != 1 || fake.patch.Name == nil || *fake.patch.Name != "changed" || !strings.Contains(w.Body.String(), "\"version\":2") || strings.Contains(w.Body.String(), "project_id") || strings.Contains(w.Body.String(), "usage_type") || strings.Contains(w.Body.String(), "role_name") || strings.Contains(w.Body.String(), "notes") {
			t.Fatalf("patch=%#v response=%s", fake.patch, w.Body.String())
		}
	})
}

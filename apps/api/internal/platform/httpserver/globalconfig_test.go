package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

func globalConfigurationHandler(t *testing.T) http.Handler {
	t.Helper()
	service, err := globalconfig.NewService(nil, "iteration-12-http-test-key")
	if err != nil {
		t.Fatal(err)
	}
	return New(":0", project.NewService(newMemoryRepository()), service).httpServer.Handler
}

func TestGlobalConfigurationProviderCRUDIntegration(t *testing.T) {
	pool, ctx := globalConfigurationTestDatabase(t)
	service, err := globalconfig.NewService(pool, "iteration-12-integration-key")
	if err != nil {
		t.Fatal(err)
	}
	debugSecret := "debug-secret"
	if _, err := service.CreateProvider(ctx, globalconfig.ProviderCreate{Name: "provider-direct", ProviderType: "openai_compatible", BaseURL: "https://api.example.test/v1", DefaultModel: "gpt-4.1-mini", TimeoutSeconds: 30, Secret: &debugSecret}, "provider-direct-create"); err != nil {
		t.Fatalf("service create provider: %v", err)
	}
	h := New(":0", project.NewService(newMemoryRepository()), service).httpServer.Handler
	call := func(method, path string, body any, key string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	create := map[string]any{"name": "provider-http", "providerType": "openai_compatible", "baseUrl": "https://api.example.test/v1", "defaultModel": "gpt-4.1-mini", "timeoutSeconds": 30, "secret": "super-secret"}
	w := call(http.MethodPost, "/api/v1/llm-providers", create, "provider-http-create")
	if w.Code != http.StatusCreated || strings.Contains(w.Body.String(), "super-secret") {
		t.Fatalf("create provider: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		Data struct {
			ID        string `json:"id"`
			Version   int    `json:"version"`
			HasSecret bool   `json:"hasSecret"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.ID == "" || created.Data.Version != 1 || !created.Data.HasSecret {
		t.Fatalf("unsafe create result: %s", w.Body.String())
	}
	get := call(http.MethodGet, "/api/v1/llm-providers/"+created.Data.ID, nil, "")
	if get.Code != http.StatusOK || strings.Contains(get.Body.String(), "super-secret") {
		t.Fatalf("get provider: %d %s", get.Code, get.Body.String())
	}
	update := call(http.MethodPatch, "/api/v1/llm-providers/"+created.Data.ID, map[string]any{"expectedVersion": 1, "defaultModel": "gpt-4.1"}, "")
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), `"version":2`) {
		t.Fatalf("update provider: %d %s", update.Code, update.Body.String())
	}
	conflict := call(http.MethodPatch, "/api/v1/llm-providers/"+created.Data.ID, map[string]any{"expectedVersion": 1, "defaultModel": "gpt-4.1-nano"}, "")
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "version_conflict") {
		t.Fatalf("version conflict: %d %s", conflict.Code, conflict.Body.String())
	}
	var ciphertext string
	if err := pool.QueryRow(ctx, "SELECT encrypted_secret FROM llm_provider_configurations WHERE id=$1", created.Data.ID).Scan(&ciphertext); err != nil || strings.Contains(ciphertext, "super-secret") {
		t.Fatalf("credential storage: %q %v", ciphertext, err)
	}
}

func TestGlobalConfigurationConnectionWorkflowAndPlatformCRUDIntegration(t *testing.T) {
	pool, _ := globalConfigurationTestDatabase(t)
	service, err := globalconfig.NewService(pool, "iteration-12-integration-key")
	if err != nil {
		t.Fatal(err)
	}
	h := New(":0", project.NewService(newMemoryRepository()), service).httpServer.Handler
	call := func(method, path string, body any, key string) (int, map[string]any, string) {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		var result struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		return w.Code, result.Data, w.Body.String()
	}
	n8n := map[string]any{"referenceType": "workflow_id", "referenceValue": "workflow-1"}
	status, connection, text := call(http.MethodPost, "/api/v1/workflow-connections", map[string]any{"name": "connection-http", "connectionType": "n8n", "baseUrl": "https://n8n.example.test", "authType": "api_key", "timeoutSeconds": 30, "typeConfig": n8n, "credential": "connection-secret"}, "connection-create")
	if status != http.StatusCreated || strings.Contains(text, "connection-secret") {
		t.Fatalf("create connection: %d %s", status, text)
	}
	connectionID, _ := connection["id"].(string)
	if connectionID == "" || connection["hasCredential"] != true {
		t.Fatalf("connection result: %s", text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-connections/"+connectionID, map[string]any{"expectedVersion": 1, "timeoutSeconds": 40}, "")
	if status != http.StatusOK || strings.Contains(text, "connection-secret") {
		t.Fatalf("update connection: %d %s", status, text)
	}
	status, workflow, text := call(http.MethodPost, "/api/v1/workflow-configurations", map[string]any{"name": "workflow-http", "connectionId": connectionID, "applicableStages": []string{"chapter_planning"}, "typeConfig": n8n, "inputContractVersion": "v1", "outputContractVersion": "v1", "defaultParameters": map[string]any{}}, "workflow-create")
	if status != http.StatusCreated || workflow["workflowType"] != "n8n" || workflow["connectionType"] != "n8n" {
		t.Fatalf("create workflow: %d %s", status, text)
	}
	workflowID, _ := workflow["id"].(string)
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-configurations/"+workflowID, map[string]any{"expectedVersion": 1, "name": "workflow-http-updated"}, "")
	if status != http.StatusOK {
		t.Fatalf("update workflow: %d %s", status, text)
	}
	status, platform, text := call(http.MethodPost, "/api/v1/distribution-platforms", map[string]any{"name": "platform-http", "platformType": "custom", "accountIdentifier": "account", "endpointUrl": "https://publish.example.test", "authType": "api_key", "timeoutSeconds": 30, "typeConfig": map[string]any{}, "credential": "platform-secret"}, "platform-create")
	if status != http.StatusCreated || strings.Contains(text, "platform-secret") {
		t.Fatalf("create platform: %d %s", status, text)
	}
	platformID, _ := platform["id"].(string)
	status, _, text = call(http.MethodPatch, "/api/v1/distribution-platforms/"+platformID, map[string]any{"expectedVersion": 1, "accountIdentifier": "account-updated"}, "")
	if status != http.StatusOK || strings.Contains(text, "platform-secret") {
		t.Fatalf("update platform: %d %s", status, text)
	}
}

func globalConfigurationTestDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	database := fmt.Sprintf("ai_content_factory_i12_test_%d", time.Now().UTC().UnixNano())
	admin, err := pgx.Connect(ctx, "postgres://postgres:postgres@127.0.0.1:15433/postgres?sslmode=disable")
	if err != nil {
		t.Skipf("PostgreSQL integration test skipped: %v", err)
	}
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+database); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+database+" WITH (FORCE)")
		_ = admin.Close(context.Background())
	})
	pool, err := pgxpool.New(ctx, "postgres://postgres:postgres@127.0.0.1:15433/"+database+"?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, file := range files {
		sql, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, string(sql)); e != nil {
			t.Fatalf("apply %s: %v", file, e)
		}
	}
	return pool, ctx
}

func TestGlobalConfigurationTypeCataloguesAndRequestBoundaries(t *testing.T) {
	h := globalConfigurationHandler(t)
	for _, path := range []string{"/api/v1/llm-provider-types", "/api/v1/workflow-connection-types", "/api/v1/distribution-platform-types"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "request_id") {
			t.Fatalf("catalogue %s: %d %s", path, w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct{ path, body string }{
		{"/api/v1/llm-providers", `{"name":"p","providerType":"openai_compatible","unexpected":true}`},
		{"/api/v1/workflow-connections/00000000-0000-4000-8000-000000000001", `{"expectedVersion":1,"connectionType":"n8n"}`},
		{"/api/v1/workflow-configurations/00000000-0000-4000-8000-000000000001", `{"expectedVersion":1,"workflowType":"n8n"}`},
		{"/api/v1/distribution-platforms/00000000-0000-4000-8000-000000000001", `{"expectedVersion":1,"platformType":"custom"}`},
	} {
		method := http.MethodPatch
		if tc.path == "/api/v1/llm-providers" {
			method = http.MethodPost
		}
		r := httptest.NewRequest(method, tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "validation_error") {
			t.Fatalf("boundary %s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
}

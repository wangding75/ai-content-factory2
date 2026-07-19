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
	replay := call(http.MethodPost, "/api/v1/llm-providers", create, "provider-http-create")
	if replay.Code != http.StatusCreated || !strings.Contains(replay.Body.String(), created.Data.ID) {
		t.Fatalf("provider idempotent replay: %d %s", replay.Code, replay.Body.String())
	}
	different := call(http.MethodPost, "/api/v1/llm-providers", map[string]any{"name": "provider-http-other", "providerType": "openai_compatible", "baseUrl": "https://api.example.test/v1", "defaultModel": "gpt-4.1-mini", "timeoutSeconds": 30}, "provider-http-create")
	if different.Code != http.StatusConflict || !strings.Contains(different.Body.String(), "idempotency_key_reused_with_different_payload") {
		t.Fatalf("provider idempotent conflict: %d %s", different.Code, different.Body.String())
	}
	var providers int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM llm_provider_configurations WHERE name='provider-http'").Scan(&providers); err != nil || providers != 1 {
		t.Fatalf("provider replay count=%d err=%v", providers, err)
	}
	assertGlobalConfigurationCount(t, ctx, pool, "provider create audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='llm_provider.create'", created.Data.ID)
	assertGlobalConfigurationCount(t, ctx, pool, "provider idempotency", "SELECT COUNT(*) FROM idempotency_records WHERE scope='llm-provider:create' AND idempotency_key='provider-http-create'")
	list := call(http.MethodGet, "/api/v1/llm-providers", nil, "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), created.Data.ID) {
		t.Fatalf("list providers: %d %s", list.Code, list.Body.String())
	}
	get := call(http.MethodGet, "/api/v1/llm-providers/"+created.Data.ID, nil, "")
	if get.Code != http.StatusOK || strings.Contains(get.Body.String(), "super-secret") {
		t.Fatalf("get provider: %d %s", get.Code, get.Body.String())
	}
	update := call(http.MethodPatch, "/api/v1/llm-providers/"+created.Data.ID, map[string]any{"expectedVersion": 1, "defaultModel": "gpt-4.1"}, "provider-update")
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), `"version":2`) {
		t.Fatalf("update provider: %d %s", update.Code, update.Body.String())
	}
	replayedUpdate := call(http.MethodPatch, "/api/v1/llm-providers/"+created.Data.ID, map[string]any{"expectedVersion": 1, "defaultModel": "gpt-4.1"}, "provider-update")
	if replayedUpdate.Code != http.StatusOK || !strings.Contains(replayedUpdate.Body.String(), `"version":2`) {
		t.Fatalf("replay provider update: %d %s", replayedUpdate.Code, replayedUpdate.Body.String())
	}
	differentUpdate := call(http.MethodPatch, "/api/v1/llm-providers/"+created.Data.ID, map[string]any{"expectedVersion": 1, "defaultModel": "gpt-4.1-nano"}, "provider-update")
	if differentUpdate.Code != http.StatusConflict || !strings.Contains(differentUpdate.Body.String(), "idempotency_key_reused_with_different_payload") {
		t.Fatalf("provider update idempotency conflict: %d %s", differentUpdate.Code, differentUpdate.Body.String())
	}
	conflict := call(http.MethodPatch, "/api/v1/llm-providers/"+created.Data.ID, map[string]any{"expectedVersion": 1, "defaultModel": "gpt-4.1-nano"}, "provider-conflict")
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), "version_conflict") {
		t.Fatalf("version conflict: %d %s", conflict.Code, conflict.Body.String())
	}
	missing := call(http.MethodPatch, "/api/v1/llm-providers/00000000-0000-4000-8000-000000000004", map[string]any{"expectedVersion": 1, "defaultModel": "missing"}, "provider-missing")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("provider not found: %d %s", missing.Code, missing.Body.String())
	}
	var ciphertext string
	if err := pool.QueryRow(ctx, "SELECT encrypted_secret FROM llm_provider_configurations WHERE id=$1", created.Data.ID).Scan(&ciphertext); err != nil || strings.Contains(ciphertext, "super-secret") {
		t.Fatalf("credential storage: %q %v", ciphertext, err)
	}
	var auditPayload string
	if err := pool.QueryRow(ctx, "SELECT payload::text FROM audit_logs WHERE subject_id=$1 AND action='llm_provider.update'", created.Data.ID).Scan(&auditPayload); err != nil || strings.Contains(auditPayload, "super-secret") {
		t.Fatalf("provider update audit=%q err=%v", auditPayload, err)
	}
	assertGlobalConfigurationCount(t, ctx, pool, "provider update audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='llm_provider.update'", created.Data.ID)
}

func TestGlobalConfigurationConnectionWorkflowAndPlatformCRUDIntegration(t *testing.T) {
	pool, ctx := globalConfigurationTestDatabase(t)
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
	status, replayConnection, text := call(http.MethodPost, "/api/v1/workflow-connections", map[string]any{"name": "connection-http", "connectionType": "n8n", "baseUrl": "https://n8n.example.test", "authType": "api_key", "timeoutSeconds": 30, "typeConfig": n8n, "credential": "connection-secret"}, "connection-create")
	if status != http.StatusCreated || replayConnection["id"] != connectionID {
		t.Fatalf("connection replay: %d %s", status, text)
	}
	status, _, text = call(http.MethodPost, "/api/v1/workflow-connections", map[string]any{"name": "connection-different", "connectionType": "n8n", "baseUrl": "https://n8n.example.test", "authType": "api_key", "timeoutSeconds": 30, "typeConfig": n8n}, "connection-create")
	if status != http.StatusConflict {
		t.Fatalf("connection idempotency conflict: %d %s", status, text)
	}
	assertGlobalConfigurationCount(t, ctx, pool, "connection resource", "SELECT COUNT(*) FROM workflow_connections WHERE id=$1", connectionID)
	assertGlobalConfigurationCount(t, ctx, pool, "connection create audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='workflow_connection.create'", connectionID)
	assertGlobalConfigurationCount(t, ctx, pool, "connection idempotency", "SELECT COUNT(*) FROM idempotency_records WHERE scope='workflow-connection:create' AND idempotency_key='connection-create'")
	status, _, text = call(http.MethodGet, "/api/v1/workflow-connections", nil, "")
	if status != http.StatusOK || !strings.Contains(text, connectionID) {
		t.Fatalf("list connection: %d %s", status, text)
	}
	status, _, text = call(http.MethodGet, "/api/v1/workflow-connections/"+connectionID, nil, "")
	if status != http.StatusOK || strings.Contains(text, "connection-secret") {
		t.Fatalf("get connection: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-connections/"+connectionID, map[string]any{"expectedVersion": 1, "timeoutSeconds": 40}, "connection-update")
	if status != http.StatusOK || strings.Contains(text, "connection-secret") {
		t.Fatalf("update connection: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-connections/"+connectionID, map[string]any{"expectedVersion": 1, "timeoutSeconds": 40}, "connection-update")
	if status != http.StatusOK || !strings.Contains(text, `"version":2`) {
		t.Fatalf("replay connection update: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-connections/"+connectionID, map[string]any{"expectedVersion": 1, "timeoutSeconds": 41}, "connection-update")
	if status != http.StatusConflict || !strings.Contains(text, "idempotency_key_reused_with_different_payload") {
		t.Fatalf("connection update key conflict: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-connections/00000000-0000-4000-8000-000000000001", map[string]any{"expectedVersion": 1, "timeoutSeconds": 40}, "connection-missing")
	if status != http.StatusNotFound {
		t.Fatalf("connection not found: %d %s", status, text)
	}
	var connectionCipher, connectionAudit string
	if err := pool.QueryRow(ctx, "SELECT encrypted_credential FROM workflow_connections WHERE id=$1", connectionID).Scan(&connectionCipher); err != nil || strings.Contains(connectionCipher, "connection-secret") {
		t.Fatalf("connection ciphertext=%q err=%v", connectionCipher, err)
	}
	if err := pool.QueryRow(ctx, "SELECT payload::text FROM audit_logs WHERE subject_id=$1 AND action='workflow_connection.update'", connectionID).Scan(&connectionAudit); err != nil || strings.Contains(connectionAudit, "connection-secret") {
		t.Fatalf("connection audit=%q err=%v", connectionAudit, err)
	}
	assertGlobalConfigurationCount(t, ctx, pool, "connection update audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='workflow_connection.update'", connectionID)
	status, workflow, text := call(http.MethodPost, "/api/v1/workflow-configurations", map[string]any{"name": "workflow-http", "connectionId": connectionID, "applicableStages": []string{"chapter_planning"}, "typeConfig": n8n, "inputContractVersion": "v1", "outputContractVersion": "v1", "defaultParameters": map[string]any{}}, "workflow-create")
	if status != http.StatusCreated || workflow["workflowType"] != "n8n" || workflow["connectionType"] != "n8n" {
		t.Fatalf("create workflow: %d %s", status, text)
	}
	workflowID, _ := workflow["id"].(string)
	status, replayWorkflow, text := call(http.MethodPost, "/api/v1/workflow-configurations", map[string]any{"name": "workflow-http", "connectionId": connectionID, "applicableStages": []string{"chapter_planning"}, "typeConfig": n8n, "inputContractVersion": "v1", "outputContractVersion": "v1", "defaultParameters": map[string]any{}}, "workflow-create")
	if status != http.StatusCreated || replayWorkflow["id"] != workflowID {
		t.Fatalf("workflow replay: %d %s", status, text)
	}
	status, _, text = call(http.MethodPost, "/api/v1/workflow-configurations", map[string]any{"name": "workflow-different", "connectionId": connectionID, "applicableStages": []string{"chapter_planning"}, "typeConfig": n8n, "inputContractVersion": "v1", "outputContractVersion": "v1", "defaultParameters": map[string]any{}}, "workflow-create")
	if status != http.StatusConflict {
		t.Fatalf("workflow idempotency conflict: %d %s", status, text)
	}
	assertGlobalConfigurationCount(t, ctx, pool, "workflow resource", "SELECT COUNT(*) FROM workflow_configurations WHERE id=$1", workflowID)
	assertGlobalConfigurationCount(t, ctx, pool, "workflow create audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='workflow_configuration.create'", workflowID)
	assertGlobalConfigurationCount(t, ctx, pool, "workflow idempotency", "SELECT COUNT(*) FROM idempotency_records WHERE scope='workflow-configuration:create' AND idempotency_key='workflow-create'")
	status, _, text = call(http.MethodGet, "/api/v1/workflow-configurations", nil, "")
	if status != http.StatusOK || !strings.Contains(text, workflowID) {
		t.Fatalf("list workflow: %d %s", status, text)
	}
	status, _, text = call(http.MethodGet, "/api/v1/workflow-configurations/"+workflowID, nil, "")
	if status != http.StatusOK {
		t.Fatalf("get workflow: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-configurations/"+workflowID, map[string]any{"expectedVersion": 1, "name": "workflow-http-updated"}, "workflow-update")
	if status != http.StatusOK {
		t.Fatalf("update workflow: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-configurations/"+workflowID, map[string]any{"expectedVersion": 1, "name": "workflow-http-updated"}, "workflow-update")
	if status != http.StatusOK || !strings.Contains(text, `"version":2`) {
		t.Fatalf("replay workflow update: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-configurations/"+workflowID, map[string]any{"expectedVersion": 1, "name": "workflow-other"}, "workflow-update")
	if status != http.StatusConflict || !strings.Contains(text, "idempotency_key_reused_with_different_payload") {
		t.Fatalf("workflow update key conflict: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-configurations/00000000-0000-4000-8000-000000000002", map[string]any{"expectedVersion": 1, "name": "missing"}, "workflow-missing")
	if status != http.StatusNotFound {
		t.Fatalf("workflow not found: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/workflow-configurations/"+workflowID, map[string]any{"expectedVersion": 1, "name": "conflict"}, "workflow-conflict")
	if status != http.StatusConflict {
		t.Fatalf("workflow conflict: %d %s", status, text)
	}
	assertGlobalConfigurationCount(t, ctx, pool, "workflow update audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='workflow_configuration.update'", workflowID)
	status, platform, text := call(http.MethodPost, "/api/v1/distribution-platforms", map[string]any{"name": "platform-http", "platformType": "custom", "accountIdentifier": "account", "endpointUrl": "https://publish.example.test", "authType": "api_key", "timeoutSeconds": 30, "typeConfig": map[string]any{}, "credential": "platform-secret"}, "platform-create")
	if status != http.StatusCreated || strings.Contains(text, "platform-secret") {
		t.Fatalf("create platform: %d %s", status, text)
	}
	platformID, _ := platform["id"].(string)
	status, replayPlatform, text := call(http.MethodPost, "/api/v1/distribution-platforms", map[string]any{"name": "platform-http", "platformType": "custom", "accountIdentifier": "account", "endpointUrl": "https://publish.example.test", "authType": "api_key", "timeoutSeconds": 30, "typeConfig": map[string]any{}, "credential": "platform-secret"}, "platform-create")
	if status != http.StatusCreated || replayPlatform["id"] != platformID {
		t.Fatalf("platform replay: %d %s", status, text)
	}
	status, _, text = call(http.MethodPost, "/api/v1/distribution-platforms", map[string]any{"name": "platform-different", "platformType": "custom", "accountIdentifier": "account", "endpointUrl": "https://publish.example.test", "authType": "api_key", "timeoutSeconds": 30, "typeConfig": map[string]any{}}, "platform-create")
	if status != http.StatusConflict {
		t.Fatalf("platform idempotency conflict: %d %s", status, text)
	}
	assertGlobalConfigurationCount(t, ctx, pool, "platform resource", "SELECT COUNT(*) FROM distribution_platform_configurations WHERE id=$1", platformID)
	assertGlobalConfigurationCount(t, ctx, pool, "platform create audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='distribution_platform.create'", platformID)
	assertGlobalConfigurationCount(t, ctx, pool, "platform idempotency", "SELECT COUNT(*) FROM idempotency_records WHERE scope='distribution-platform:create' AND idempotency_key='platform-create'")
	status, _, text = call(http.MethodGet, "/api/v1/distribution-platforms", nil, "")
	if status != http.StatusOK || !strings.Contains(text, platformID) {
		t.Fatalf("list platform: %d %s", status, text)
	}
	status, _, text = call(http.MethodGet, "/api/v1/distribution-platforms/"+platformID, nil, "")
	if status != http.StatusOK || strings.Contains(text, "platform-secret") {
		t.Fatalf("get platform: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/distribution-platforms/00000000-0000-4000-8000-000000000003", map[string]any{"expectedVersion": 1, "accountIdentifier": "missing"}, "platform-missing")
	if status != http.StatusNotFound {
		t.Fatalf("platform not found: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/distribution-platforms/"+platformID, map[string]any{"expectedVersion": 1, "accountIdentifier": "account-updated"}, "platform-update")
	if status != http.StatusOK || strings.Contains(text, "platform-secret") {
		t.Fatalf("update platform: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/distribution-platforms/"+platformID, map[string]any{"expectedVersion": 1, "accountIdentifier": "account-updated"}, "platform-update")
	if status != http.StatusOK || !strings.Contains(text, `"version":2`) {
		t.Fatalf("replay platform update: %d %s", status, text)
	}
	status, _, text = call(http.MethodPatch, "/api/v1/distribution-platforms/"+platformID, map[string]any{"expectedVersion": 1, "accountIdentifier": "account-other"}, "platform-update")
	if status != http.StatusConflict || !strings.Contains(text, "idempotency_key_reused_with_different_payload") {
		t.Fatalf("platform update key conflict: %d %s", status, text)
	}
	var platformCipher, platformAudit string
	if err := pool.QueryRow(ctx, "SELECT encrypted_credential FROM distribution_platform_configurations WHERE id=$1", platformID).Scan(&platformCipher); err != nil || strings.Contains(platformCipher, "platform-secret") {
		t.Fatalf("platform ciphertext=%q err=%v", platformCipher, err)
	}
	if err := pool.QueryRow(ctx, "SELECT payload::text FROM audit_logs WHERE subject_id=$1 AND action='distribution_platform.update'", platformID).Scan(&platformAudit); err != nil || strings.Contains(platformAudit, "platform-secret") {
		t.Fatalf("platform audit=%q err=%v", platformAudit, err)
	}
	assertGlobalConfigurationCount(t, ctx, pool, "platform update audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='distribution_platform.update'", platformID)
}

func assertGlobalConfigurationCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label, query string, args ...any) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, query, args...).Scan(&count); err != nil || count != 1 {
		t.Fatalf("%s count=%d err=%v", label, count, err)
	}
}

func globalConfigurationTestDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	database := fmt.Sprintf("ai_content_factory_i12_test_%d", time.Now().UTC().UnixNano())
	admin, err := pgx.Connect(ctx, "postgres://postgres:postgres@127.0.0.1:15433/postgres?sslmode=disable")
	if err != nil {
		if os.Getenv("REQUIRE_POSTGRES_INTEGRATION") == "1" {
			t.Fatalf("PostgreSQL integration is required: %v", err)
		}
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
	for _, tc := range []struct{ path, body string }{
		{"/api/v1/llm-providers/00000000-0000-4000-8000-000000000001", `{"expectedVersion":1,"defaultModel":"x"}`},
		{"/api/v1/workflow-connections/00000000-0000-4000-8000-000000000001", `{"expectedVersion":1,"timeoutSeconds":30}`},
		{"/api/v1/workflow-configurations/00000000-0000-4000-8000-000000000001", `{"expectedVersion":1,"name":"x"}`},
		{"/api/v1/distribution-platforms/00000000-0000-4000-8000-000000000001", `{"expectedVersion":1,"accountIdentifier":"x"}`},
	} {
		r := httptest.NewRequest(http.MethodPatch, tc.path, strings.NewReader(tc.body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "Idempotency-Key") {
			t.Fatalf("missing patch key %s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
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

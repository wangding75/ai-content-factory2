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

	"github.com/google/uuid"
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

func TestApplicableStageWorkflowConfigurationsIntegration(t *testing.T) {
	pool, ctx := workflowBindingIntegrationDatabase(t)
	svc, err := globalconfig.NewService(pool, "iteration-13-test-key-32bytes!")
	if err != nil {
		t.Fatalf("new globalconfig service: %v", err)
	}
	handler := New(":0", project.NewService(newMemoryRepository()), svc).httpServer.Handler

	doGet := func(path string) (int, map[string]any, string) {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		var res map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		return w.Code, res, w.Body.String()
	}

	// 1. Four valid values
	for _, stage := range []string{"chapter_planning", "content_generation", "review", "rewrite"} {
		code, _, text := doGet("/api/v1/workflow-configurations?applicableStage=" + stage)
		if code != http.StatusOK {
			t.Fatalf("valid applicableStage %s returned status %d: %s", stage, code, text)
		}
	}

	// 2. Invalid value returns HTTP 400 validation_error and fields.applicableStage=invalid_enum
	code, res, text := doGet("/api/v1/workflow-configurations?applicableStage=invalid_stage")
	if code != http.StatusBadRequest {
		t.Fatalf("invalid applicableStage expected 400, got %d: %s", code, text)
	}
	errObj, ok := res["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object in response: %s", text)
	}
	if errObj["code"] != "validation_error" {
		t.Fatalf("expected error code validation_error, got %v in: %s", errObj["code"], text)
	}
	details, _ := errObj["details"].(map[string]any)
	fields, _ := details["fields"].(map[string]any)
	if fields["applicableStage"] != "invalid_enum" {
		t.Fatalf("expected fields.applicableStage=invalid_enum, got %v in: %s", fields["applicableStage"], text)
	}

	// Setup fixtures in DB
	prefix := "stg_" + uuid.New().String()[:8]
	conn1ID := uuid.New()
	conn2ID := uuid.New()
	w1ID := uuid.New()
	w2ID := uuid.New()
	w3ID := uuid.New()
	w4ID := uuid.New()
	w5ID := uuid.New()

	now := time.Now().UTC()
	_, err = pool.Exec(ctx, `INSERT INTO workflow_connections (id, name, connection_type, base_url, auth_type, timeout_seconds, type_config, integration_status, created_at, updated_at)
		VALUES ($1, $2, 'n8n', 'http://localhost:5678', 'api_key', 30, '{"referenceType":"workflow_id","referenceValue":"w1"}', 'verified', $3, $4)`,
		conn1ID, prefix+"_conn1", now, now)
	if err != nil {
		t.Fatalf("insert conn1: %v", err)
	}

	_, err = pool.Exec(ctx, `INSERT INTO workflow_connections (id, name, connection_type, base_url, auth_type, timeout_seconds, type_config, integration_status, created_at, updated_at)
		VALUES ($1, $2, 'n8n', 'http://localhost:8080', 'api_key', 30, '{"referenceType":"workflow_id","referenceValue":"w2"}', 'not_connected', $3, $4)`,
		conn2ID, prefix+"_conn2", now, now)
	if err != nil {
		t.Fatalf("insert conn2: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM workflow_configurations WHERE id IN ($1, $2, $3, $4, $5)`, w1ID, w2ID, w3ID, w4ID, w5ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM workflow_connections WHERE id IN ($1, $2)`, conn1ID, conn2ID)
	})

	// W1: prefix_alpha, Conn1 (n8n), applicableStages: ["chapter_planning", "review"], enabled: true
	_, err = pool.Exec(ctx, `INSERT INTO workflow_configurations (id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version, default_parameters, integration_status, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, '["chapter_planning", "review"]'::jsonb, '{"referenceType":"workflow_id","referenceValue":"w1"}', 'v1', 'v1', '{}', 'verified', true, $4, $5)`,
		w1ID, prefix+"_alpha_planner", conn1ID, now, now)
	if err != nil {
		t.Fatalf("insert w1: %v", err)
	}

	// W2: prefix_beta, Conn1 (n8n), applicableStages: ["content_generation"], enabled: true
	_, err = pool.Exec(ctx, `INSERT INTO workflow_configurations (id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version, default_parameters, integration_status, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, '["content_generation"]'::jsonb, '{"referenceType":"workflow_id","referenceValue":"w2"}', 'v1', 'v1', '{}', 'verified', true, $4, $5)`,
		w2ID, prefix+"_beta_generator", conn1ID, now, now)
	if err != nil {
		t.Fatalf("insert w2: %v", err)
	}

	// W3: prefix_gamma, Conn1 (n8n), applicableStages: ["review"], enabled: false
	_, err = pool.Exec(ctx, `INSERT INTO workflow_configurations (id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version, default_parameters, integration_status, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, '["review"]'::jsonb, '{"referenceType":"workflow_id","referenceValue":"w3"}', 'v1', 'v1', '{}', 'verified', false, $4, $5)`,
		w3ID, prefix+"_gamma_reviewer", conn1ID, now, now)
	if err != nil {
		t.Fatalf("insert w3: %v", err)
	}

	// W4: prefix_delta, Conn2 (dify), applicableStages: ["rewrite"], enabled: true
	_, err = pool.Exec(ctx, `INSERT INTO workflow_configurations (id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version, default_parameters, integration_status, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, '["rewrite"]'::jsonb, '{}', 'v1', 'v1', '{}', 'not_connected', true, $4, $5)`,
		w4ID, prefix+"_delta_rewriter", conn2ID, now, now)
	if err != nil {
		t.Fatalf("insert w4: %v", err)
	}

	// W5: prefix_epsilon, Conn2 (dify), applicableStages: ["chapter_planning", "rewrite"], enabled: true
	_, err = pool.Exec(ctx, `INSERT INTO workflow_configurations (id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version, default_parameters, integration_status, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, '["chapter_planning", "rewrite"]'::jsonb, '{}', 'v1', 'v1', '{}', 'not_connected', true, $4, $5)`,
		w5ID, prefix+"_epsilon_multi", conn2ID, now, now)
	if err != nil {
		t.Fatalf("insert w5: %v", err)
	}

	getItemIDs := func(res map[string]any) map[string]bool {
		ids := make(map[string]bool)
		data, _ := res["data"].(map[string]any)
		items, _ := data["items"].([]any)
		for _, item := range items {
			m, ok := item.(map[string]any)
			if ok {
				if idStr, ok := m["id"].(string); ok {
					ids[idStr] = true
				}
			}
		}
		return ids
	}

	// 3. Results only include configs containing target stage
	_, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix + "&applicableStage=content_generation")
	ids := getItemIDs(res)
	if !ids[w2ID.String()] || ids[w1ID.String()] || ids[w3ID.String()] || ids[w4ID.String()] || ids[w5ID.String()] {
		t.Fatalf("applicableStage=content_generation filter failed, got ids %v: %s", ids, text)
	}

	_, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix + "&applicableStage=chapter_planning")
	ids = getItemIDs(res)
	if !ids[w1ID.String()] || !ids[w5ID.String()] || ids[w2ID.String()] || ids[w3ID.String()] || ids[w4ID.String()] {
		t.Fatalf("applicableStage=chapter_planning filter failed, got ids %v: %s", ids, text)
	}

	// 4. disabled=false/true return when enabled parameter is not specified
	_, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix + "&applicableStage=review")
	ids = getItemIDs(res)
	if !ids[w1ID.String()] || !ids[w3ID.String()] {
		t.Fatalf("applicableStage=review without enabled param should return both enabled and disabled, got ids %v: %s", ids, text)
	}

	// 5. AND combinations with q, connectionId, connectionType, integrationStatus, enabled
	// 5a. q
	_, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix + "_alpha&applicableStage=chapter_planning")
	ids = getItemIDs(res)
	if !ids[w1ID.String()] || ids[w5ID.String()] {
		t.Fatalf("applicableStage + q failed, got ids %v: %s", ids, text)
	}

	// 5b. connectionId
	_, res, text = doGet("/api/v1/workflow-configurations?connectionId=" + conn1ID.String() + "&applicableStage=chapter_planning")
	ids = getItemIDs(res)
	if !ids[w1ID.String()] || ids[w5ID.String()] {
		t.Fatalf("applicableStage + connectionId failed, got ids %v: %s", ids, text)
	}

	// 5c. connectionType
	_, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix + "&connectionType=n8n&applicableStage=rewrite")
	ids = getItemIDs(res)
	if !ids[w4ID.String()] || !ids[w5ID.String()] || ids[w1ID.String()] {
		t.Fatalf("applicableStage + connectionType failed, got ids %v: %s", ids, text)
	}

	// 5d. integrationStatus
	_, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix + "&integrationStatus=verified&applicableStage=review")
	ids = getItemIDs(res)
	if !ids[w1ID.String()] || !ids[w3ID.String()] {
		t.Fatalf("applicableStage + integrationStatus failed, got ids %v: %s", ids, text)
	}

	// 5e. enabled
	_, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix + "&enabled=false&applicableStage=review")
	ids = getItemIDs(res)
	if !ids[w3ID.String()] || ids[w1ID.String()] {
		t.Fatalf("applicableStage + enabled=false failed, got ids %v: %s", ids, text)
	}

	// 6. limit, offset, total
	code, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix + "&applicableStage=chapter_planning&limit=1&offset=0")
	if code != http.StatusOK {
		t.Fatalf("limit/offset status %d: %s", code, text)
	}
	data, _ := res["data"].(map[string]any)
	total, _ := data["total"].(float64)
	limit, _ := data["limit"].(float64)
	offset, _ := data["offset"].(float64)
	items, _ := data["items"].([]any)
	if total != 2 || limit != 1 || offset != 0 || len(items) != 1 {
		t.Fatalf("limit/offset/total mismatch: total=%v limit=%v offset=%v len(items)=%d: %s", total, limit, offset, len(items), text)
	}

	code, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix + "&applicableStage=chapter_planning&limit=1&offset=1")
	data, _ = res["data"].(map[string]any)
	offset, _ = data["offset"].(float64)
	items, _ = data["items"].([]any)
	if offset != 1 || len(items) != 1 {
		t.Fatalf("offset=1 mismatch: offset=%v len(items)=%d: %s", offset, len(items), text)
	}

	// 7. Omitted applicableStage retains Iteration 12 original behavior
	_, res, text = doGet("/api/v1/workflow-configurations?q=" + prefix)
	ids = getItemIDs(res)
	if len(ids) != 5 {
		t.Fatalf("omitted applicableStage should return all 5 workflows for q, got len %d: %s", len(ids), text)
	}
}

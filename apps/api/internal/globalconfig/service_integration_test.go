package globalconfig

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateProviderConcurrentIdempotencyUsesPostgresLock(t *testing.T) {
	pool, ctx := globalConfigIntegrationDatabase(t)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM audit_logs WHERE subject_id IN (SELECT id::text FROM llm_provider_configurations WHERE name='concurrent-provider')")
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM idempotency_records WHERE scope='llm-provider:create' AND idempotency_key='concurrent-provider-key'")
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM llm_provider_configurations WHERE name='concurrent-provider'")
	})
	service, err := NewService(pool, "iteration-12-concurrency-key")
	if err != nil {
		t.Fatal(err)
	}
	const requests = 8
	ready := make(chan struct{}, requests)
	release := make(chan struct{})
	service.beforeIdempotencyLock = func() {
		ready <- struct{}{}
		<-release
	}

	results := make(chan Provider, requests)
	errs := make(chan error, requests)
	secret := "concurrent-provider-secret"
	request := ProviderCreate{Name: "concurrent-provider", ProviderType: "openai_compatible", BaseURL: "https://api.example.test/v1", DefaultModel: "gpt-4.1-mini", TimeoutSeconds: 30, Secret: &secret}
	var workers sync.WaitGroup
	for range requests {
		workers.Add(1)
		go func() {
			defer workers.Done()
			provider, createErr := service.CreateProvider(ctx, request, "concurrent-provider-key")
			results <- provider
			errs <- createErr
		}()
	}
	for range requests {
		<-ready
	}
	close(release)
	workers.Wait()
	close(results)
	close(errs)

	var id string
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent create returned %v", err)
		}
	}
	for provider := range results {
		if id == "" {
			id = provider.ID.String()
		} else if provider.ID.String() != id {
			t.Fatalf("concurrent create returned different IDs: %s and %s", id, provider.ID)
		}
	}
	for table, query := range map[string]string{
		"provider":    "SELECT COUNT(*) FROM llm_provider_configurations WHERE name='concurrent-provider'",
		"audit":       "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='llm_provider.create'",
		"idempotency": "SELECT COUNT(*) FROM idempotency_records WHERE scope='llm-provider:create' AND idempotency_key='concurrent-provider-key'",
	} {
		var count int
		var queryErr error
		if table == "provider" || table == "idempotency" {
			queryErr = pool.QueryRow(ctx, query).Scan(&count)
		} else {
			queryErr = pool.QueryRow(ctx, query, id).Scan(&count)
		}
		if queryErr != nil || count != 1 {
			t.Fatalf("%s count=%d err=%v", table, count, queryErr)
		}
	}
	conflicting := request
	conflicting.Name = "different-payload"
	if _, err = service.CreateProvider(ctx, conflicting, "concurrent-provider-key"); !errors.Is(err, ErrIdempotency) {
		t.Fatalf("different payload error=%v, want ErrIdempotency", err)
	}
}

func TestVerificationIdempotencyReplayBehavior(t *testing.T) {
	pool, ctx := verificationIntegrationDatabase(t)

	t.Run("connection success replay conflict and disable", func(t *testing.T) {
		connectionID := insertVerificationConnection(t, ctx, pool, false)
		cleanupVerificationFixtures(t, pool, connectionID)
		service, err := NewService(pool, "verification-replay-key")
		if err != nil {
			t.Fatal(err)
		}
		probes, transactionDuringProbe := configureVerificationProbe(service, pool, false)

		first, err := service.VerifyConnection(ctx, connectionID, 1, "connection-success")
		if err != nil {
			t.Fatalf("first verify: %v", err)
		}
		if first.Enabled || first.IntegrationStatus != "verified" || first.Version != 1 || first.VerifiedVersion == nil || *first.VerifiedVersion != 1 {
			t.Fatalf("first result=%+v", first)
		}
		replay, err := service.VerifyConnection(ctx, connectionID, 1, "connection-success")
		if err != nil {
			t.Fatalf("replay verify: %v", err)
		}
		if replay.ID != first.ID || replay.Version != first.Version || replay.IntegrationStatus != first.IntegrationStatus || replay.Enabled != first.Enabled {
			t.Fatalf("replay=%+v, first=%+v", replay, first)
		}
		if _, err = service.VerifyConnection(ctx, connectionID, 2, "connection-success"); !errors.Is(err, ErrIdempotency) {
			t.Fatalf("different payload error=%v, want ErrIdempotency", err)
		}
		if probes.Load() != 1 {
			t.Fatalf("probe count=%d, want 1", probes.Load())
		}
		if transactionDuringProbe.Load() {
			t.Fatal("verification probe ran while a database connection was held")
		}
		assertVerificationState(t, ctx, pool, "workflow_connections", connectionID, 1, 1, 1)
		if _, err = service.SetResourceEnabled(ctx, ValidationResourceConnection, connectionID, 1, true, "connection-enable"); err != nil {
			t.Fatalf("enable: %v", err)
		}

		disabled, err := service.DisableConnection(ctx, connectionID, 2, "connection-disable")
		if err != nil {
			t.Fatalf("disable: %v", err)
		}
		disabledReplay, err := service.DisableConnection(ctx, connectionID, 2, "connection-disable")
		if err != nil || disabledReplay.Version != disabled.Version || disabledReplay.Enabled {
			t.Fatalf("disable replay=%+v err=%v", disabledReplay, err)
		}
		if probes.Load() != 1 {
			t.Fatalf("disable triggered probe count=%d", probes.Load())
		}
		assertVerificationRecordCount(t, ctx, pool, "connection disable audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='workflow_connection.disable'", connectionID.String(), 1)
		assertVerificationRecordCount(t, ctx, pool, "connection disable idempotency", "SELECT COUNT(*) FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2", "workflow-connection:disable:"+connectionID.String(), "connection-disable", 1)
	})

	t.Run("connection failure replay", func(t *testing.T) {
		connectionID := insertVerificationConnection(t, ctx, pool, false)
		cleanupVerificationFixtures(t, pool, connectionID)
		service, err := NewService(pool, "verification-replay-key")
		if err != nil {
			t.Fatal(err)
		}
		probes, transactionDuringProbe := configureVerificationProbe(service, pool, true)

		first, firstErr := service.VerifyConnection(ctx, connectionID, 1, "connection-failure")
		replay, replayErr := service.VerifyConnection(ctx, connectionID, 1, "connection-failure")
		if !errors.Is(firstErr, ErrVerification) || !errors.Is(replayErr, ErrVerification) {
			t.Fatalf("failure errors first=%v replay=%v", firstErr, replayErr)
		}
		if first.Version != 1 || replay.Version != first.Version || first.Enabled || replay.Enabled || first.IntegrationStatus != "failed" || replay.IntegrationStatus != "failed" {
			t.Fatalf("failure first=%+v replay=%+v", first, replay)
		}
		if probes.Load() != 1 || transactionDuringProbe.Load() {
			t.Fatalf("probe count=%d transactionDuringProbe=%v", probes.Load(), transactionDuringProbe.Load())
		}
		assertVerificationState(t, ctx, pool, "workflow_connections", connectionID, 1, 1, 1)
	})

	t.Run("workflow success replay and conflict", func(t *testing.T) {
		connectionID := insertVerificationConnection(t, ctx, pool, true)
		workflowID := insertVerificationWorkflow(t, ctx, pool, connectionID)
		cleanupVerificationFixtures(t, pool, connectionID, workflowID)
		service, err := NewService(pool, "verification-replay-key")
		if err != nil {
			t.Fatal(err)
		}
		probes, transactionDuringProbe := configureVerificationProbe(service, pool, false)

		first, err := service.VerifyWorkflowConfiguration(ctx, workflowID, 1, "workflow-success")
		if err != nil {
			t.Fatalf("first verify: %v", err)
		}
		replay, err := service.VerifyWorkflowConfiguration(ctx, workflowID, 1, "workflow-success")
		if err != nil {
			t.Fatalf("replay verify: %v", err)
		}
		if first.Enabled || first.IntegrationStatus != "verified" || first.Version != 1 || first.VerifiedVersion == nil || *first.VerifiedVersion != 1 || replay.ID != first.ID || replay.Version != first.Version {
			t.Fatalf("first=%+v replay=%+v", first, replay)
		}
		if _, err = service.VerifyWorkflowConfiguration(ctx, workflowID, 2, "workflow-success"); !errors.Is(err, ErrIdempotency) {
			t.Fatalf("different payload error=%v, want ErrIdempotency", err)
		}
		if probes.Load() != 1 || transactionDuringProbe.Load() {
			t.Fatalf("probe count=%d transactionDuringProbe=%v", probes.Load(), transactionDuringProbe.Load())
		}
		assertVerificationState(t, ctx, pool, "workflow_configurations", workflowID, 1, 1, 1)
		if _, err = service.SetResourceEnabled(ctx, ValidationResourceWorkflow, workflowID, 1, true, "workflow-enable"); err != nil {
			t.Fatalf("enable: %v", err)
		}

		disabled, err := service.DisableWorkflowConfiguration(ctx, workflowID, 2, "workflow-disable")
		if err != nil {
			t.Fatalf("disable: %v", err)
		}
		disabledReplay, err := service.DisableWorkflowConfiguration(ctx, workflowID, 2, "workflow-disable")
		if err != nil || disabledReplay.Version != disabled.Version || disabledReplay.Enabled {
			t.Fatalf("disable replay=%+v err=%v", disabledReplay, err)
		}
		if probes.Load() != 1 {
			t.Fatalf("disable triggered probe count=%d", probes.Load())
		}
		assertVerificationRecordCount(t, ctx, pool, "workflow disable audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action='workflow_configuration.disable'", workflowID.String(), 1)
		assertVerificationRecordCount(t, ctx, pool, "workflow disable idempotency", "SELECT COUNT(*) FROM idempotency_records WHERE scope=$1 AND idempotency_key=$2", "workflow-configuration:disable:"+workflowID.String(), "workflow-disable", 1)
	})

	t.Run("workflow failure replay", func(t *testing.T) {
		connectionID := insertVerificationConnection(t, ctx, pool, true)
		workflowID := insertVerificationWorkflow(t, ctx, pool, connectionID)
		cleanupVerificationFixtures(t, pool, connectionID, workflowID)
		service, err := NewService(pool, "verification-replay-key")
		if err != nil {
			t.Fatal(err)
		}
		probes, transactionDuringProbe := configureVerificationProbe(service, pool, true)

		first, firstErr := service.VerifyWorkflowConfiguration(ctx, workflowID, 1, "workflow-failure")
		replay, replayErr := service.VerifyWorkflowConfiguration(ctx, workflowID, 1, "workflow-failure")
		if !errors.Is(firstErr, ErrVerification) || !errors.Is(replayErr, ErrVerification) {
			t.Fatalf("failure errors first=%v replay=%v", firstErr, replayErr)
		}
		if first.Version != 1 || replay.Version != first.Version || first.Enabled || replay.Enabled || first.IntegrationStatus != "failed" || replay.IntegrationStatus != "failed" {
			t.Fatalf("failure first=%+v replay=%+v", first, replay)
		}
		if probes.Load() != 1 || transactionDuringProbe.Load() {
			t.Fatalf("probe count=%d transactionDuringProbe=%v", probes.Load(), transactionDuringProbe.Load())
		}
		assertVerificationState(t, ctx, pool, "workflow_configurations", workflowID, 1, 1, 1)
	})
}

func TestIteration19ConfigurationPersistenceRoundTrip(t *testing.T) {
	pool, ctx := verificationIntegrationDatabase(t)
	service, err := NewService(pool, "iteration-19-round-trip")
	if err != nil {
		t.Fatal(err)
	}
	providerID, connectionID, workflowID := uuid.New(), uuid.New(), uuid.New()
	encryptedCredential, credentialFingerprint, err := service.seal("iteration-19-n8n-api-key")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE id=$1", workflowID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", connectionID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM llm_provider_configurations WHERE id=$1", providerID)
	})

	if _, err = pool.Exec(ctx, `INSERT INTO llm_provider_configurations(
		id,name,provider_type,base_url,default_model,timeout_seconds,integration_status,enabled,last_verified_version,last_verified_at,validation_details,version
	) VALUES($1,$2,'openai_compatible','https://provider.example.test/v1','model-i19',30,'verified',true,3,NOW(),'{"catalog":"safe"}',3)`, providerID, "provider-"+providerID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO workflow_connections(
		id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,encrypted_credential,credential_fingerprint,integration_status,enabled,last_verified_version,last_verified_at,validation_details,version
	) VALUES($1,$2,'n8n','https://n8n.example.test','api_key',30,'{}',$3,$4,'verified',true,4,NOW(),'{"probe":"safe"}',4)`, connectionID, "connection-"+connectionID.String(), encryptedCredential, credentialFingerprint); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO workflow_configurations(
		id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version,default_parameters,
		integration_status,enabled,last_verified_version,last_verified_at,validation_details,version,llm_strategy,llm_provider_id,llm_model
	) VALUES($1,$2,$3,'["review"]','{"referenceType":"workflow_id","referenceValue":"fixture"}','v1','v1','{}',
		'verified',true,5,NOW(),'{"layers":["connection","workflow"]}',5,'acf_managed',$4,'model-i19')`, workflowID, "workflow-"+workflowID.String(), connectionID, providerID); err != nil {
		t.Fatal(err)
	}
	seenAt := time.Now().UTC().Truncate(time.Microsecond)
	model, err := service.UpsertProviderModel(ctx, ProviderModel{ProviderID: providerID, ModelKey: "model-i19", Source: "discovered", Availability: "available", LastSeenAt: &seenAt})
	if err != nil {
		t.Fatal(err)
	}
	models, err := service.ListProviderModels(ctx, providerID)
	if err != nil || len(models) != 1 || models[0].ID != model.ID || models[0].ModelKey != "model-i19" || models[0].LastSeenAt == nil {
		t.Fatalf("model catalog round-trip models=%+v err=%v", models, err)
	}

	provider, err := service.GetProvider(ctx, providerID)
	if err != nil || provider.VerifiedVersion == nil || *provider.VerifiedVersion != 3 || provider.ValidationStatus != "verified" || !provider.Executable || string(provider.ValidationDetails) != `{"catalog": "safe"}` && string(provider.ValidationDetails) != `{"catalog":"safe"}` {
		t.Fatalf("provider round-trip=%+v err=%v", provider, err)
	}
	connection, err := service.GetConnection(ctx, connectionID)
	if err != nil || connection.VerifiedVersion == nil || *connection.VerifiedVersion != 4 || connection.ValidationStatus != "verified" || !connection.Executable {
		t.Fatalf("connection round-trip=%+v err=%v", connection, err)
	}
	workflow, err := service.GetWorkflow(ctx, workflowID)
	if err != nil || workflow.VerifiedVersion == nil || *workflow.VerifiedVersion != 5 || workflow.LlmProviderID == nil || *workflow.LlmProviderID != providerID || workflow.LlmModel == nil || *workflow.LlmModel != "model-i19" || workflow.LlmStrategy != "acf_managed" || !workflow.Executable {
		t.Fatalf("workflow round-trip=%+v err=%v", workflow, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE workflow_connections SET encrypted_credential=NULL,credential_fingerprint=NULL WHERE id=$1", connectionID); err != nil {
		t.Fatal(err)
	}
	eligibility, err := service.EvaluateWorkflowExecutionEligibility(ctx, workflowID, "review")
	if err != nil || eligibility.Executable {
		t.Fatalf("missing credential eligibility=%+v err=%v", eligibility, err)
	}
	foundCredentialReason := false
	for _, reason := range eligibility.Reasons {
		if reason.Code == "connection_credential_unavailable" {
			foundCredentialReason = true
		}
	}
	if !foundCredentialReason {
		t.Fatalf("missing credential reasons=%+v", eligibility.Reasons)
	}
	name := "must-not-write"
	if _, err = service.UpdateWorkflow(ctx, workflowID, WorkflowUpdate{ExpectedVersion: 4, Name: &name}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("version conflict error=%v", err)
	}
}

func verificationIntegrationDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is not set; verification integration test is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse DATABASE_URL: %v", err)
	}
	if config.ConnConfig.Database != "ai_content_factory" {
		t.Fatalf("DATABASE_URL database=%q, want ai_content_factory", config.ConnConfig.Database)
	}
	// The idempotency race test deliberately holds each transaction at a
	// synchronization barrier before it acquires its advisory lock. Keep the
	// test pool large enough for every participant to reach that barrier.
	if config.MaxConns < 8 {
		config.MaxConns = 8
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect verification database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

func insertVerificationConnection(t *testing.T, ctx context.Context, pool *pgxpool.Pool, connected bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	service, err := NewService(pool, "verification-replay-key")
	if err != nil {
		t.Fatal(err)
	}
	encryptedCredential, credentialFingerprint, err := service.seal("verification-api-key")
	if err != nil {
		t.Fatal(err)
	}
	status, enabled := "unverified", false
	if connected {
		status, enabled = "verified", true
	}
	_, err = pool.Exec(ctx, `INSERT INTO workflow_connections
		(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,encrypted_credential,credential_fingerprint,integration_status,enabled,last_verified_version)
		VALUES ($1,$2,'n8n','http://verification.example.test:5678','api_key',30,'{"referenceType":"workflow_id","referenceValue":"verification"}',$3,$4,$5::text,$6,CASE WHEN $5::text='verified' THEN 1 ELSE NULL END)`,
		id, "verification-connection-"+id.String(), encryptedCredential, credentialFingerprint, status, enabled)
	if err != nil {
		t.Fatalf("insert verification connection: %v", err)
	}
	return id
}

func insertVerificationWorkflow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, connectionID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO workflow_configurations
		(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version,default_parameters)
		VALUES ($1,$2,$3,'["chapter_planning"]','{"referenceType":"webhook_path","referenceValue":"verification"}','v1','v1','{}')`,
		id, "verification-workflow-"+id.String(), connectionID)
	if err != nil {
		t.Fatalf("insert verification workflow: %v", err)
	}
	return id
}

func cleanupVerificationFixtures(t *testing.T, pool *pgxpool.Pool, connectionID uuid.UUID, workflowIDs ...uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ids := []string{connectionID.String()}
		for _, id := range workflowIDs {
			ids = append(ids, id.String())
		}
		if _, err := pool.Exec(ctx, "DELETE FROM idempotency_records WHERE scope LIKE $1 OR scope LIKE $2", "%:"+connectionID.String(), verificationWorkflowScopePattern(workflowIDs)); err != nil {
			t.Errorf("cleanup verification idempotency: %v", err)
		}
		if _, err := pool.Exec(ctx, "DELETE FROM audit_logs WHERE subject_id=ANY($1)", ids); err != nil {
			t.Errorf("cleanup verification audits: %v", err)
		}
		for _, id := range workflowIDs {
			if _, err := pool.Exec(ctx, "DELETE FROM workflow_configurations WHERE id=$1", id); err != nil {
				t.Errorf("cleanup verification workflow: %v", err)
			}
		}
		if _, err := pool.Exec(ctx, "DELETE FROM workflow_connections WHERE id=$1", connectionID); err != nil {
			t.Errorf("cleanup verification connection: %v", err)
		}
	})
}

func verificationWorkflowScopePattern(ids []uuid.UUID) string {
	if len(ids) == 0 {
		return "verification-no-workflow"
	}
	return "%:" + ids[0].String()
}

func configureVerificationProbe(service *Service, pool *pgxpool.Pool, fail bool) (*atomic.Int32, *atomic.Bool) {
	var probes atomic.Int32
	var transactionDuringProbe atomic.Bool
	service.resolveHost = func(context.Context, string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("203.0.113.20")}, nil
	}
	service.dialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
		probes.Add(1)
		if pool.Stat().AcquiredConns() > 1 {
			transactionDuringProbe.Store(true)
		}
		client, server := net.Pipe()
		go serveVerificationProbe(server, fail)
		return client, nil
	}
	return &probes, &transactionDuringProbe
}

func serveVerificationProbe(connection net.Conn, fail bool) {
	defer connection.Close()
	request, err := http.ReadRequest(bufio.NewReader(connection))
	if err != nil {
		return
	}
	defer request.Body.Close()
	if fail {
		_, _ = io.WriteString(connection, "HTTP/1.1 503 Service Unavailable\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		return
	}
	response := []byte(`{}`)
	if strings.Contains(request.URL.Path, "/api/v1/workflows/") {
		response = []byte(`{"id":"verification","active":true,"tags":[{"name":"acf-stage:chapter_planning"}]}`)
	}
	if request.Method == http.MethodPost {
		var payload struct {
			Stage           string `json:"stage"`
			ContractVersion string `json:"contractVersion"`
			RequestID       string `json:"requestId"`
		}
		if json.NewDecoder(request.Body).Decode(&payload) != nil {
			return
		}
		response, _ = json.Marshal(map[string]any{"verified": true, "stage": payload.Stage, "contractVersion": payload.ContractVersion, "requestId": payload.RequestID})
	}
	_, _ = fmt.Fprintf(connection, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(response), response)
}

func assertVerificationState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, id uuid.UUID, version, auditCount, idempotencyCount int) {
	t.Helper()
	var storedVersion int
	if err := pool.QueryRow(ctx, "SELECT version FROM "+table+" WHERE id=$1", id).Scan(&storedVersion); err != nil || storedVersion != version {
		t.Fatalf("%s version=%d err=%v, want %d", table, storedVersion, err, version)
	}
	assertVerificationRecordCount(t, ctx, pool, table+" audit", "SELECT COUNT(*) FROM audit_logs WHERE subject_id=$1 AND action LIKE $2", id.String(), "%verify%", auditCount)
	assertVerificationRecordCount(t, ctx, pool, table+" idempotency", "SELECT COUNT(*) FROM idempotency_records WHERE scope LIKE $1", "%:verify:"+id.String(), idempotencyCount)
}

func assertVerificationRecordCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label, query string, args ...any) {
	t.Helper()
	want := args[len(args)-1].(int)
	args = args[:len(args)-1]
	var count int
	if err := pool.QueryRow(ctx, query, args...).Scan(&count); err != nil || count != want {
		t.Fatalf("%s count=%d err=%v, want %d", label, count, err, want)
	}
}

func globalConfigIntegrationDatabase(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	return verificationIntegrationDatabase(t)
}

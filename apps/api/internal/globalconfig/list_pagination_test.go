package globalconfig

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openListDB(t *testing.T) (*pgxpool.Pool, context.Context, *Service) {
	t.Helper()
	raw := os.Getenv("DATABASE_URL")
	if raw == "" {
		t.Fatal("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	svc, err := NewService(pool, "list-pagination-test-secret-key")
	if err != nil {
		t.Fatal(err)
	}
	return pool, context.Background(), svc
}

func boolPtr(v bool) *bool { return &v }

func TestProviderListFiltersThenPaginatesAcrossPages(t *testing.T) {
	pool, ctx, svc := openListDB(t)
	// Create 55 providers: first 40 non-executable (disabled), last 15 executable shape.
	// With old page-first semantics and limit=10, page 0 would only see disabled rows.
	// Scope via unique name prefix so concurrent suite data cannot inflate totals.
	prefix := "list-provider-" + uuid.NewString()[:8]
	ids := make([]uuid.UUID, 0, 55)
	for i := 0; i < 55; i++ {
		id := uuid.New()
		ids = append(ids, id)
		enabled := i >= 40
		status := "unverified"
		verified := any(nil)
		if enabled {
			status = "verified"
			verified = 1
		}
		_, err := pool.Exec(ctx, `INSERT INTO llm_provider_configurations(
			id,name,provider_type,base_url,default_model,timeout_seconds,integration_status,enabled,last_verified_version,version
		) VALUES($1,$2,'openai_compatible','https://provider.example.test/v1','model',30,$3,$4,$5,1)`,
			id, fmt.Sprintf("%s-%02d", prefix, i), status, enabled, verified)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM llm_provider_configurations WHERE id=$1", id) })
	}
	items, total, err := svc.ListProviders(ctx, ListOptions{Query: prefix, Executable: boolPtr(true), Limit: 10, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if total != 15 {
		t.Fatalf("total=%d want 15", total)
	}
	if len(items) != 10 {
		t.Fatalf("page0 len=%d", len(items))
	}
	for _, item := range items {
		if !item.Executable || item.ValidationStatus != "verified" {
			t.Fatalf("item not executable/verified: %+v", item)
		}
	}
	page1, total1, err := svc.ListProviders(ctx, ListOptions{Query: prefix, Executable: boolPtr(true), Limit: 10, Offset: 10})
	if err != nil || total1 != 15 || len(page1) != 5 {
		t.Fatalf("page1 len=%d total=%d err=%v", len(page1), total1, err)
	}
	seen := map[uuid.UUID]bool{}
	for _, item := range append(items, page1...) {
		if seen[item.ID] {
			t.Fatalf("duplicate %s", item.ID)
		}
		seen[item.ID] = true
	}
	if len(seen) != 15 {
		t.Fatalf("unique=%d", len(seen))
	}
	empty, totalOver, err := svc.ListProviders(ctx, ListOptions{Query: prefix, Executable: boolPtr(true), Limit: 10, Offset: 100})
	if err != nil || totalOver != 15 || len(empty) != 0 {
		t.Fatalf("overshoot empty=%d total=%d err=%v", len(empty), totalOver, err)
	}
	// executable=false should capture the first 40 disabled/unverified rows.
	disabled, disabledTotal, err := svc.ListProviders(ctx, ListOptions{Query: prefix, Executable: boolPtr(false), Limit: 50, Offset: 0})
	if err != nil || disabledTotal != 40 || len(disabled) != 40 {
		t.Fatalf("disabled total=%d len=%d err=%v", disabledTotal, len(disabled), err)
	}
}

func TestConnectionListExecutableFilterAndTotal(t *testing.T) {
	pool, ctx, svc := openListDB(t)
	prefix := "list-conn-" + uuid.NewString()[:8]
	for i := 0; i < 30; i++ {
		id := uuid.New()
		enabled := i >= 20
		status := "unverified"
		var verified any
		var enc any
		var fp any
		if enabled {
			status = "verified"
			verified = 1
			// seal a credential so hydrate marks credentialReadable.
			encVal, fpVal, err := svc.seal("connection-secret")
			if err != nil {
				t.Fatal(err)
			}
			enc, fp = encVal, fpVal
		}
		_, err := pool.Exec(ctx, `INSERT INTO workflow_connections(
			id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,encrypted_credential,credential_fingerprint,integration_status,enabled,last_verified_version,version
		) VALUES($1,$2,'n8n','https://n8n.example.test','api_key',30,'{"referenceType":"workflow_id","referenceValue":"wf"}',$3,$4,$5,$6,$7,1)`,
			id, fmt.Sprintf("%s-%02d", prefix, i), enc, fp, status, enabled, verified)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", id) })
	}
	items, total, err := svc.ListConnections(ctx, ListOptions{Query: prefix, Executable: boolPtr(true), Limit: 5, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if total != 10 || len(items) != 5 {
		t.Fatalf("total=%d len=%d", total, len(items))
	}
	for _, item := range items {
		if !item.Executable {
			t.Fatalf("non-executable returned: %+v", item)
		}
	}
	page1, total1, err := svc.ListConnections(ctx, ListOptions{Query: prefix, Executable: boolPtr(true), Limit: 5, Offset: 5})
	if err != nil || total1 != 10 || len(page1) != 5 {
		t.Fatalf("page1 total=%d len=%d err=%v", total1, len(page1), err)
	}
}

func TestWorkflowListFiltersExecutableAfterHydrateAcrossPages(t *testing.T) {
	pool, ctx, svc := openListDB(t)
	// Shared executable connection.
	connID := uuid.New()
	enc, fp, err := svc.seal("workflow-list-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO workflow_connections(
		id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,encrypted_credential,credential_fingerprint,integration_status,enabled,last_verified_version,version
	) VALUES($1,$2,'n8n','https://n8n.example.test','api_key',30,'{"referenceType":"workflow_id","referenceValue":"wf"}',$3,$4,'verified',true,1,1)`,
		connID, "list-wf-conn-"+connID.String()[:8], enc, fp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", connID) })

	// 25 non-executable (disabled) then 12 executable verified workflows.
	// Old page-first with limit=10 would return empty for executable=true on page 0.
	prefix := "list-wf-" + uuid.NewString()[:8]
	for i := 0; i < 37; i++ {
		id := uuid.New()
		enabled := i >= 25
		status := "unverified"
		var verified any
		if enabled {
			status = "verified"
			verified = 1
		}
		_, err = pool.Exec(ctx, `INSERT INTO workflow_configurations(
			id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version,default_parameters,llm_strategy,integration_status,enabled,last_verified_version,version
		) VALUES($1,$2,$3,'["review"]','{"referenceType":"workflow_id","referenceValue":"wf"}','v1','v1','{}','none',$4,$5,$6,1)`,
			id, fmt.Sprintf("%s-%02d", prefix, i), connID, status, enabled, verified)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM workflow_configurations WHERE id=$1", id) })
	}
	items, total, err := svc.ListWorkflows(ctx, ListOptions{Query: prefix, Executable: boolPtr(true), Limit: 10, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if total != 12 {
		t.Fatalf("total=%d want 12", total)
	}
	if len(items) != 10 {
		t.Fatalf("page0 len=%d want 10 (must not be empty when matches exist later)", len(items))
	}
	for _, item := range items {
		if !item.Executable {
			t.Fatalf("non-executable workflow returned: %+v", item)
		}
	}
	page1, total1, err := svc.ListWorkflows(ctx, ListOptions{Query: prefix, Executable: boolPtr(true), Limit: 10, Offset: 10})
	if err != nil || total1 != 12 || len(page1) != 2 {
		t.Fatalf("page1 total=%d len=%d err=%v", total1, len(page1), err)
	}
	// Multi-condition: executable + validationStatus.
	_, totalV, err := svc.ListWorkflows(ctx, ListOptions{Query: prefix, Executable: boolPtr(true), ValidationStatus: "verified", Limit: 50, Offset: 0})
	if err != nil || totalV != 12 {
		t.Fatalf("combined total=%d err=%v", totalV, err)
	}
}

func TestPlatformListFinalizesAndFiltersExecutable(t *testing.T) {
	pool, ctx, svc := openListDB(t)
	// 22 platforms: first 15 disabled/unverified, last 7 enabled connected (validationStatus finalized).
	prefix := "list-platform-" + uuid.NewString()[:8]
	for i := 0; i < 22; i++ {
		id := uuid.New()
		enabled := i >= 15
		status := "not_connected"
		if enabled {
			status = "connected"
		}
		_, err := pool.Exec(ctx, `INSERT INTO distribution_platform_configurations(
			id,name,platform_type,account_identifier,endpoint_url,auth_type,timeout_seconds,type_config,integration_status,enabled,version
		) VALUES($1,$2,'custom',$3,'https://platform.example.test','api_key',30,'{}',$4,$5,1)`,
			id, fmt.Sprintf("%s-%02d", prefix, i), "acct-"+id.String()[:8], status, enabled)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM distribution_platform_configurations WHERE id=$1", id) })
	}
	items, total, err := svc.ListPlatforms(ctx, ListOptions{Query: prefix, Limit: 50, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if total != 22 || len(items) != 22 {
		t.Fatalf("total=%d len=%d", total, len(items))
	}
	for _, item := range items {
		// finalizeCommon must populate validationStatus from integration_status.
		if item.ValidationStatus == "" {
			t.Fatalf("validationStatus not finalized: %+v", item)
		}
	}
	// Filter by validationStatus mapped from integration_status.
	connected, connectedTotal, err := svc.ListPlatforms(ctx, ListOptions{Query: prefix, ValidationStatus: "connected", Limit: 10, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if connectedTotal != 7 {
		t.Fatalf("connected total=%d", connectedTotal)
	}
	if len(connected) != 7 {
		t.Fatalf("connected len=%d", len(connected))
	}
	for _, item := range connected {
		if item.ValidationStatus != "connected" {
			t.Fatalf("validationStatus=%s", item.ValidationStatus)
		}
	}
	// executable filter must operate after finalize (platforms lack verified_version so typically false).
	execTrue, execTotal, err := svc.ListPlatforms(ctx, ListOptions{Query: prefix, Executable: boolPtr(true), Limit: 50, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if execTotal != len(execTrue) {
		t.Fatalf("executable total mismatch total=%d len=%d", execTotal, len(execTrue))
	}
	for _, item := range execTrue {
		if !item.Executable {
			t.Fatalf("executable=false in true filter: %+v", item)
		}
	}
	execFalse, falseTotal, err := svc.ListPlatforms(ctx, ListOptions{Query: prefix, Executable: boolPtr(false), Limit: 50, Offset: 0})
	if err != nil || falseTotal != len(execFalse) || falseTotal+execTotal != 22 {
		t.Fatalf("falseTotal=%d trueTotal=%d falseLen=%d err=%v", falseTotal, execTotal, len(execFalse), err)
	}
	// Page across filtered connected set with limit 3.
	p0, t0, err := svc.ListPlatforms(ctx, ListOptions{Query: prefix, ValidationStatus: "connected", Limit: 3, Offset: 0})
	if err != nil || t0 != 7 || len(p0) != 3 {
		t.Fatalf("p0 total=%d len=%d err=%v", t0, len(p0), err)
	}
	p1, t1, err := svc.ListPlatforms(ctx, ListOptions{Query: prefix, ValidationStatus: "connected", Limit: 3, Offset: 3})
	if err != nil || t1 != 7 || len(p1) != 3 {
		t.Fatalf("p1 total=%d len=%d err=%v", t1, len(p1), err)
	}
	p2, t2, err := svc.ListPlatforms(ctx, ListOptions{Query: prefix, ValidationStatus: "connected", Limit: 3, Offset: 6})
	if err != nil || t2 != 7 || len(p2) != 1 {
		t.Fatalf("p2 total=%d len=%d err=%v", t2, len(p2), err)
	}
}

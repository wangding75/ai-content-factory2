package globalconfig

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func openRuntimeDB(t *testing.T) (*pgxpool.Pool, context.Context, *Service) {
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
	svc, err := NewService(pool, "runtime-meta-test-encryption-key")
	if err != nil {
		t.Fatal(err)
	}
	return pool, context.Background(), svc
}

func insertRuntimeConnection(t *testing.T, pool *pgxpool.Pool, ctx context.Context, svc *Service, name string, enabled bool, status string, version int, verified *int, withCred bool, typeConfig string, badCipher bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var enc any
	var fp any
	if withCred {
		if badCipher {
			enc = "not-valid-ciphertext"
			fp = "deadbeef"
		} else {
			sealed, fingerprint, err := svc.seal("runtime-n8n-api-key")
			if err != nil {
				t.Fatal(err)
			}
			enc, fp = sealed, fingerprint
		}
	}
	if typeConfig == "" {
		typeConfig = `{"referenceType":"workflow_id","referenceValue":"wf"}`
	}
	_, err := pool.Exec(ctx, `INSERT INTO workflow_connections(
		id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,encrypted_credential,credential_fingerprint,integration_status,enabled,last_verified_version,version
	) VALUES($1,$2,'n8n','https://n8n.example.test','api_key',30,$3,$4,$5,$6,$7,$8,$9)`,
		id, name, typeConfig, enc, fp, status, enabled, verified, version)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", id) })
	return id
}

func TestN8NRuntimeConfiguredRequiresDecryptableCredential(t *testing.T) {
	pool, ctx, svc := openRuntimeDB(t)
	// No connections
	if svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("expected false with no connections")
	}
	// Disabled
	insertRuntimeConnection(t, pool, ctx, svc, "rt-disabled-"+uuid.NewString()[:8], false, "verified", 1, intPointer(1), true, "", false)
	if svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("disabled should not configure runtime")
	}
	// Unverified
	insertRuntimeConnection(t, pool, ctx, svc, "rt-unverified-"+uuid.NewString()[:8], true, "unverified", 1, nil, true, "", false)
	if svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("unverified should not configure runtime")
	}
	// Stale integration status is not verified capability.
	insertRuntimeConnection(t, pool, ctx, svc, "rt-stale-"+uuid.NewString()[:8], true, "stale", 3, nil, true, "", false)
	if svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("stale status should not configure runtime")
	}
	// Credential missing
	insertRuntimeConnection(t, pool, ctx, svc, "rt-nocred-"+uuid.NewString()[:8], true, "verified", 1, intPointer(1), false, "", false)
	if svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("missing credential should not configure runtime")
	}
	// Bad ciphertext
	insertRuntimeConnection(t, pool, ctx, svc, "rt-badcipher-"+uuid.NewString()[:8], true, "verified", 1, intPointer(1), true, "", true)
	if svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("bad cipher should not configure runtime")
	}
	// Invalid type config
	insertRuntimeConnection(t, pool, ctx, svc, "rt-badtype-"+uuid.NewString()[:8], true, "verified", 1, intPointer(1), true, `{"broken":true}`, false)
	if svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("invalid type config should not configure runtime")
	}
	// Valid
	insertRuntimeConnection(t, pool, ctx, svc, "rt-valid-"+uuid.NewString()[:8], true, "verified", 1, intPointer(1), true, "", false)
	if !svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("expected true with valid credential-backed connection")
	}
}

func TestN8NRuntimeConfiguredRejectsWrongEncryptionKey(t *testing.T) {
	pool, ctx, svc := openRuntimeDB(t)
	id := uuid.New()
	sealed, fp, err := svc.seal("secret-key-value")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO workflow_connections(
		id,name,connection_type,base_url,auth_type,timeout_seconds,type_config,encrypted_credential,credential_fingerprint,integration_status,enabled,last_verified_version,version
	) VALUES($1,$2,'n8n','https://n8n.example.test','api_key',30,'{"referenceType":"workflow_id","referenceValue":"wf"}',$3,$4,'verified',true,1,1)`,
		id, "rt-wrong-key-"+id.String()[:8], sealed, fp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM workflow_connections WHERE id=$1", id) })
	other, err := NewService(pool, "a-different-encryption-key-value")
	if err != nil {
		t.Fatal(err)
	}
	if other.N8NRuntimeConfigured(ctx) {
		t.Fatal("wrong encryption key must report runtime not configured")
	}
	if !svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("original key must still configure runtime")
	}
}

func TestN8NRuntimeConfiguredOneValidAmongInvalid(t *testing.T) {
	pool, ctx, svc := openRuntimeDB(t)
	insertRuntimeConnection(t, pool, ctx, svc, "rt-mix-bad-"+uuid.NewString()[:8], true, "verified", 1, intPointer(1), true, "", true)
	insertRuntimeConnection(t, pool, ctx, svc, "rt-mix-good-"+uuid.NewString()[:8], true, "verified", 1, intPointer(1), true, "", false)
	if !svc.N8NRuntimeConfigured(ctx) {
		t.Fatal("one valid connection should configure runtime")
	}
}

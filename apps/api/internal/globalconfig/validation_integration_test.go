package globalconfig

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestValidationCompletionUsesVersionAndSingleWinnerCAS(t *testing.T) {
	pool, ctx := globalConfigIntegrationDatabase(t)
	service, err := NewService(pool, "iteration-19-validation-key")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("stale completion cannot overwrite a newer configuration", func(t *testing.T) {
		id := insertValidationProvider(t, ctx, pool, false)
		cleanupValidationProvider(t, pool, id)
		attempt, err := service.BeginValidation(ctx, ValidationResourceProvider, id, 1)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, "UPDATE llm_provider_configurations SET version=version+1,integration_status='stale' WHERE id=$1", id); err != nil {
			t.Fatal(err)
		}
		err = service.CompleteValidation(ctx, attempt, ValidationOutcome{Success: true, Details: json.RawMessage(`{"checks":[]}`)})
		if !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("completion error=%v", err)
		}
		var status string
		var version int
		var verifiedVersion *int
		if err = pool.QueryRow(ctx, "SELECT integration_status,version,last_verified_version FROM llm_provider_configurations WHERE id=$1", id).Scan(&status, &version, &verifiedVersion); err != nil {
			t.Fatal(err)
		}
		if status != "stale" || version != 2 || verifiedVersion != nil {
			t.Fatalf("status=%s version=%d verified=%v", status, version, verifiedVersion)
		}
	})

	t.Run("only one concurrent completion becomes the final fact", func(t *testing.T) {
		id := insertValidationProvider(t, ctx, pool, false)
		cleanupValidationProvider(t, pool, id)
		attempt, err := service.BeginValidation(ctx, ValidationResourceProvider, id, 1)
		if err != nil {
			t.Fatal(err)
		}
		outcomes := []ValidationOutcome{
			{Success: true, Details: json.RawMessage(`{"checks":[{"status":"passed"}]}`)},
			{Success: false, Code: "upstream_unavailable", Message: "The provider is unavailable.", Details: json.RawMessage(`{"checks":[{"status":"failed"}]}`)},
		}
		start := make(chan struct{})
		errs := make(chan error, len(outcomes))
		var workers sync.WaitGroup
		for _, outcome := range outcomes {
			outcome := outcome
			workers.Add(1)
			go func() { defer workers.Done(); <-start; errs <- service.CompleteValidation(ctx, attempt, outcome) }()
		}
		close(start)
		workers.Wait()
		close(errs)
		successes, conflicts := 0, 0
		for completionErr := range errs {
			if completionErr == nil {
				successes++
			} else if errors.Is(completionErr, ErrVersionConflict) {
				conflicts++
			} else {
				t.Fatalf("completion error=%v", completionErr)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
		}
		var auditCount int
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM audit_logs WHERE subject_id=$1 AND action='llm_provider.verify'", id.String()).Scan(&auditCount); err != nil {
			t.Fatal(err)
		}
		if auditCount != 1 {
			t.Fatalf("audit count=%d", auditCount)
		}
	})
}

func TestValidationCommandConcurrentIdempotencyRunsValidatorOnce(t *testing.T) {
	pool, ctx := globalConfigIntegrationDatabase(t)
	service, err := NewService(pool, "iteration-19-validation-command-key")
	if err != nil {
		t.Fatal(err)
	}
	id := insertValidationProvider(t, ctx, pool, false)
	cleanupValidationProvider(t, pool, id)
	var calls atomic.Int32
	validator := func(context.Context) ValidationOutcome {
		calls.Add(1)
		time.Sleep(25 * time.Millisecond)
		return ValidationOutcome{Success: true, Details: json.RawMessage(`{"checks":[{"code":"endpoint","status":"passed"}]}`)}
	}
	start := make(chan struct{})
	results := make(chan ValidationCommandResult, 2)
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			result, commandErr := service.RunValidationCommand(ctx, ValidationResourceProvider, id, 1, "same-key", validator)
			results <- result
			errs <- commandErr
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errs)
	for commandErr := range errs {
		if commandErr != nil {
			t.Fatal(commandErr)
		}
	}
	var first uuid.UUID
	for result := range results {
		if first == uuid.Nil {
			first = result.ID
		} else if result.ID != first {
			t.Fatalf("different replay results: %s %s", first, result.ID)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("validator calls=%d", calls.Load())
	}
	var enabled bool
	if err = pool.QueryRow(ctx, "SELECT enabled FROM llm_provider_configurations WHERE id=$1", id).Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("successful verification unexpectedly enabled the provider")
	}
	if _, err = service.RunValidationCommand(ctx, ValidationResourceProvider, id, 2, "same-key", validator); !errors.Is(err, ErrIdempotency) {
		t.Fatalf("different payload error=%v", err)
	}
	var audits, records int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM audit_logs WHERE subject_id=$1 AND action='llm_provider.verify'", id.String()).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM idempotency_records WHERE scope=$1 AND idempotency_key='same-key'", "llm-provider:verify:"+id.String()).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if audits != 1 || records != 1 {
		t.Fatalf("audits=%d records=%d", audits, records)
	}
}

func TestEnableCommandConcurrentIdempotencyAndDisable(t *testing.T) {
	pool, ctx := globalConfigIntegrationDatabase(t)
	service, err := NewService(pool, "iteration-19-enable-command-key")
	if err != nil {
		t.Fatal(err)
	}
	id := insertValidationProvider(t, ctx, pool, false)
	cleanupValidationProvider(t, pool, id)
	if _, err = pool.Exec(ctx, "UPDATE llm_provider_configurations SET integration_status='verified',last_verified_version=1 WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan EnableCommandResult, 2)
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			result, commandErr := service.SetResourceEnabled(ctx, ValidationResourceProvider, id, 1, true, "enable-key")
			results <- result
			errs <- commandErr
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errs)
	for commandErr := range errs {
		if commandErr != nil {
			t.Fatal(commandErr)
		}
	}
	for result := range results {
		if !result.Enabled || !result.Executable || result.Version != 2 {
			t.Fatalf("enable result=%+v", result)
		}
	}
	if _, err = service.SetResourceEnabled(ctx, ValidationResourceProvider, id, 2, true, "enable-key"); !errors.Is(err, ErrIdempotency) {
		t.Fatalf("different payload error=%v", err)
	}
	disabled, err := service.SetResourceEnabled(ctx, ValidationResourceProvider, id, 2, false, "disable-key")
	if err != nil || disabled.Enabled || disabled.Executable || disabled.Version != 3 {
		t.Fatalf("disable=%+v err=%v", disabled, err)
	}
	replay, err := service.SetResourceEnabled(ctx, ValidationResourceProvider, id, 2, false, "disable-key")
	if err != nil || replay.Version != disabled.Version {
		t.Fatalf("disable replay=%+v err=%v", replay, err)
	}
	var enableAudits, disableAudits int
	if err = pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE action='llm_provider.enable'),count(*) FILTER (WHERE action='llm_provider.disable') FROM audit_logs WHERE subject_id=$1", id.String()).Scan(&enableAudits, &disableAudits); err != nil {
		t.Fatal(err)
	}
	if enableAudits != 1 || disableAudits != 1 {
		t.Fatalf("enable audits=%d disable audits=%d", enableAudits, disableAudits)
	}
}

func TestSecretPatchPreserveReplaceClearAndCanarySafety(t *testing.T) {
	pool, ctx := globalConfigIntegrationDatabase(t)
	service, err := NewService(pool, "iteration-19-secret-key")
	if err != nil {
		t.Fatal(err)
	}
	canary := "I19-CANARY-SECRET-never-emit"
	id := insertValidationProvider(t, ctx, pool, true)
	cleanupValidationProvider(t, pool, id)
	if _, err = pool.Exec(ctx, "UPDATE llm_provider_configurations SET encrypted_secret=$2,secret_fingerprint=$3 WHERE id=$1", id, "sealed-old", secureFingerprint("old-secret")); err != nil {
		t.Fatal(err)
	}

	name := "preserved-secret-provider"
	preserved, err := service.UpdateProvider(ctx, id, ProviderUpdate{ExpectedVersion: 1, Name: &name})
	if err != nil || !preserved.HasSecret || preserved.SecretFingerprint == nil || *preserved.SecretFingerprint != secureFingerprint("old-secret") {
		t.Fatalf("preserve=%+v err=%v", preserved, err)
	}
	replaced, err := service.UpdateProvider(ctx, id, ProviderUpdate{ExpectedVersion: 2, Secret: &canary})
	if err != nil || replaced.SecretFingerprint == nil || *replaced.SecretFingerprint != secureFingerprint(canary) {
		t.Fatalf("replace=%+v err=%v", replaced, err)
	}
	clear := true
	cleared, err := service.UpdateProvider(ctx, id, ProviderUpdate{ExpectedVersion: 3, ClearSecret: &clear})
	if err != nil || cleared.HasSecret || cleared.SecretFingerprint != nil {
		t.Fatalf("clear=%+v err=%v", cleared, err)
	}

	unsafe := json.RawMessage(`{"authorization":"Bearer I19-CANARY-SECRET-never-emit","nested":[{"api_key":"I19-CANARY-SECRET-never-emit"}],"code":"actionable"}`)
	safe := sanitizeJSON(unsafe)
	if strings.Contains(string(safe), canary) || !strings.Contains(string(safe), "actionable") {
		t.Fatalf("sanitized=%s", safe)
	}
	var emitted string
	if err = pool.QueryRow(ctx, "SELECT coalesce(string_agg(payload::text,' '),'') FROM audit_logs WHERE subject_id=$1", id.String()).Scan(&emitted); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal([]any{preserved, replaced, cleared, safe, emitted})
	if strings.Contains(string(encoded), canary) {
		t.Fatalf("secret canary leaked: %s", encoded)
	}
	if secureFingerprint(canary) == canary || len(secureFingerprint(canary)) != 32 {
		t.Fatal("unsafe fingerprint")
	}
}

func insertValidationProvider(t *testing.T, ctx context.Context, pool *pgxpool.Pool, enabled bool) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO llm_provider_configurations(id,name,provider_type,base_url,default_model,timeout_seconds,enabled) VALUES($1,$2,'openai_compatible','https://api.example.test/v1','fixture-model',30,$3)", id, "validation-"+id.String()[:8], enabled)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func cleanupValidationProvider(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(ctx, "DELETE FROM audit_logs WHERE subject_id=$1", id.String())
		_, _ = pool.Exec(ctx, "DELETE FROM llm_provider_configurations WHERE id=$1", id)
	})
}

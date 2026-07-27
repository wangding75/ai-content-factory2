package chapterplan

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresBulkAdoptDifferentIdempotencyKeysSerializeByBatch(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	repo, batch, candidate := newBulkAdoptFixture(t, ctx, db)
	command := func(key string) BulkAdoptCommand {
		return BulkAdoptCommand{BatchID: batch.ID, ExpectedBatchVersion: batch.Version, Candidates: []BulkAdoptCandidateItemCommand{{CandidateID: candidate.ID, ExpectedCandidateVersion: candidate.Version}}, IdempotencyKey: key, ActorID: "concurrent-test"}
	}

	start := make(chan struct{})
	results := make(chan struct {
		key    string
		result BulkAdoptResult
		err    error
	}, 2)
	for _, key := range []string{"different-key-a", "different-key-b"} {
		go func(key string) {
			<-start
			result, err := repo.BulkAdoptCandidates(ctx, command(key))
			results <- struct {
				key    string
				result BulkAdoptResult
				err    error
			}{key, result, err}
		}(key)
	}
	close(start)

	first, second := <-results, <-results
	all := []struct {
		key    string
		result BulkAdoptResult
		err    error
	}{first, second}
	successes := 0
	conflicts := 0
	var success BulkAdoptResult
	var successKey string
	for _, result := range all {
		if result.err == nil {
			successes++
			success = result.result
			successKey = result.key
			continue
		}
		if errors.Is(result.err, ErrVersionConflict) || errors.Is(result.err, ErrBatchAlreadyFinalized) {
			conflicts++
			continue
		}
		t.Fatalf("concurrent bulk adopt returned unexpected error: %v", result.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent different-key outcomes: successes=%d conflicts=%d", successes, conflicts)
	}
	if len(success.Items) != 1 || success.Items[0].Outcome != "adopted" {
		t.Fatalf("successful bulk adoption=%+v", success)
	}

	assertBulkAdoptPersistedOnce(t, ctx, db, batch.ID, candidate.ID, 2)
	replay, err := repo.BulkAdoptCandidates(ctx, command(successKey))
	if err != nil || len(replay.Items) != 1 || replay.Items[0].Outcome != "adopted" {
		t.Fatalf("same-key bulk replay changed behavior: result=%+v err=%v", replay, err)
	}
	assertBulkAdoptPersistedOnce(t, ctx, db, batch.ID, candidate.ID, 2)
}

func TestPostgresBulkAdoptConcurrentRequestsDoNotDuplicateRevisions(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	repo, batch, candidate := newBulkAdoptFixture(t, ctx, db)
	command := func(key string) BulkAdoptCommand {
		return BulkAdoptCommand{BatchID: batch.ID, ExpectedBatchVersion: batch.Version, Candidates: []BulkAdoptCandidateItemCommand{{CandidateID: candidate.ID, ExpectedCandidateVersion: candidate.Version}}, IdempotencyKey: key, ActorID: "revision-dedupe-test"}
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, key := range []string{"revision-key-a", "revision-key-b"} {
		go func(key string) {
			<-start
			_, err := repo.BulkAdoptCandidates(ctx, command(key))
			errs <- err
		}(key)
	}
	close(start)
	first, second := <-errs, <-errs
	if (first == nil) == (second == nil) {
		t.Fatalf("expected exactly one successful concurrent request, got %v and %v", first, second)
	}
	if err := first; err != nil && !errors.Is(err, ErrVersionConflict) && !errors.Is(err, ErrBatchAlreadyFinalized) {
		t.Fatal(err)
	}
	if err := second; err != nil && !errors.Is(err, ErrVersionConflict) && !errors.Is(err, ErrBatchAlreadyFinalized) {
		t.Fatal(err)
	}
	assertBulkAdoptPersistedOnce(t, ctx, db, batch.ID, candidate.ID, 2)
}

func newBulkAdoptFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) (*Repository, CandidateBatch, Candidate) {
	t.Helper()
	fixture := newFixture(t, ctx, db)
	input := newRuntimeValidationInput(t, ctx, db, fixture)
	input.Run.RunID = uuid.New()
	input.NormalizedOutput.SourceWorkflowRunID = input.Run.RunID
	seedRuntimeValidationRun(t, ctx, db, input)
	batch, err := NewResultIngestor(db).Ingest(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewPostgresRepository(db, "test-hmac-secret-1234567890")
	if err != nil {
		t.Fatal(err)
	}
	items, err := repo.ListCandidates(ctx, batch.ID, CandidateFilter{Limit: 10})
	if err != nil || len(items.Items) != 2 {
		t.Fatalf("candidate fixture list=%+v err=%v", items, err)
	}
	return repo, batch, items.Items[0]
}

func assertBulkAdoptPersistedOnce(t *testing.T, ctx context.Context, db *pgxpool.Pool, batchID, candidateID uuid.UUID, expectedBatchVersion int) {
	t.Helper()
	var candidateStatus string
	var revisions, plans, batchVersion, candidateVersion int
	if err := db.QueryRow(ctx, "SELECT status, version FROM chapter_plan_candidates WHERE id=$1", candidateID).Scan(&candidateStatus, &candidateVersion); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_revisions WHERE source_candidate_id=$1", candidateID).Scan(&revisions); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plans WHERE source_candidate_id=$1", candidateID).Scan(&plans); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, "SELECT version FROM chapter_plan_candidate_batches WHERE id=$1", batchID).Scan(&batchVersion); err != nil {
		t.Fatal(err)
	}
	if candidateStatus != "adopted" || candidateVersion != 2 || revisions != 1 || plans != 1 || batchVersion != expectedBatchVersion {
		t.Fatalf("persisted bulk adoption candidate=%s/%d revisions=%d plans=%d batchVersion=%d", candidateStatus, candidateVersion, revisions, plans, batchVersion)
	}
}

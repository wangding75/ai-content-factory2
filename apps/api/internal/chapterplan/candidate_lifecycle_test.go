package chapterplan_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
)

func TestCandidateLifecycleCommandStructs(t *testing.T) {
	candID := uuid.New()
	batchID := uuid.New()
	expPlanVer := 1

	adoptCmd := chapterplan.AdoptCandidateCommand{
		CandidateID:                candID,
		ExpectedCandidateVersion:   1,
		ExpectedChapterPlanVersion: &expPlanVer,
		IdempotencyKey:             "test-key-1",
		ActorID:                    "tester",
	}
	if adoptCmd.CandidateID != candID || adoptCmd.ExpectedCandidateVersion != 1 {
		t.Errorf("unexpected adoptCmd values")
	}

	bulkCmd := chapterplan.BulkAdoptCommand{
		BatchID:              batchID,
		ExpectedBatchVersion: 1,
		Candidates: []chapterplan.BulkAdoptCandidateItemCommand{
			{CandidateID: candID, ExpectedCandidateVersion: 1},
		},
		IdempotencyKey: "bulk-key-1",
		ActorID:        "tester",
	}
	if bulkCmd.BatchID != batchID || len(bulkCmd.Candidates) != 1 {
		t.Errorf("unexpected bulkCmd values")
	}

	reason := "user test discard"
	discardCmd := chapterplan.DiscardCandidateCommand{
		CandidateID:              candID,
		ExpectedCandidateVersion: 1,
		Reason:                   &reason,
		IdempotencyKey:           "discard-key-1",
		ActorID:                  "tester",
	}
	if discardCmd.CandidateID != candID || *discardCmd.Reason != reason {
		t.Errorf("unexpected discardCmd values")
	}

	abandonCmd := chapterplan.AbandonBatchCommand{
		BatchID:                            batchID,
		ExpectedBatchVersion:               1,
		Reason:                             &reason,
		AcknowledgeAdoptedChaptersRemain:   true,
		IdempotencyKey:                     "abandon-key-1",
		ActorID:                            "tester",
	}
	if abandonCmd.BatchID != batchID || !abandonCmd.AcknowledgeAdoptedChaptersRemain {
		t.Errorf("unexpected abandonCmd values")
	}
}

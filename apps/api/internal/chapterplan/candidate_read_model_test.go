package chapterplan_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
)

type mockStore struct {
	batches   []chapterplan.CandidateBatch
	candidates []chapterplan.Candidate
	revisions  []chapterplan.Revision
	plans      []chapterplan.Plan
}

func (m *mockStore) ListByProject(ctx context.Context, id uuid.UUID) ([]chapterplan.Plan, error) {
	return m.plans, nil
}
func (m *mockStore) GetByID(ctx context.Context, id uuid.UUID) (chapterplan.Plan, error) {
	for _, p := range m.plans {
		if p.ID == id {
			return p, nil
		}
	}
	return chapterplan.Plan{}, chapterplan.ErrNotFound
}
func (m *mockStore) SaveMock(ctx context.Context, run chapterplan.Run, plans []chapterplan.Plan) error {
	return nil
}
func (m *mockStore) Update(ctx context.Context, p chapterplan.Plan, v int) (chapterplan.Plan, error) {
	return p, nil
}
func (m *mockStore) Delete(ctx context.Context, id uuid.UUID, v int) error {
	return nil
}
func (m *mockStore) Confirm(ctx context.Context, s []chapterplan.Selection) ([]chapterplan.Plan, error) {
	return nil, nil
}

func (m *mockStore) ListCandidateBatches(ctx context.Context, projectID uuid.UUID, f chapterplan.BatchFilter) (chapterplan.BatchListResult, error) {
	var filtered []chapterplan.CandidateBatch
	for _, b := range m.batches {
		if b.ProjectID != projectID {
			continue
		}
		if f.Status != nil && b.Status != *f.Status {
			continue
		}
		if f.GenerationMode != nil && b.GenerationMode != *f.GenerationMode {
			continue
		}
		if f.SourceWorkflowRunID != nil && b.SourceWorkflowRunID != *f.SourceWorkflowRunID {
			continue
		}
		filtered = append(filtered, b)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	return chapterplan.BatchListResult{
		Items:  filtered,
		Total:  len(filtered),
		Limit:  limit,
		Offset: f.Offset,
	}, nil
}

func (m *mockStore) GetCandidateBatchByID(ctx context.Context, batchID uuid.UUID) (chapterplan.CandidateBatch, error) {
	for _, b := range m.batches {
		if b.ID == batchID {
			return b, nil
		}
	}
	return chapterplan.CandidateBatch{}, chapterplan.ErrBatchNotFound
}

func (m *mockStore) ListCandidates(ctx context.Context, batchID uuid.UUID, f chapterplan.CandidateFilter) (chapterplan.CandidateListResult, error) {
	var filtered []chapterplan.Candidate
	for _, c := range m.candidates {
		if c.BatchID != batchID {
			continue
		}
		if f.Status != nil && c.Status != *f.Status {
			continue
		}
		if f.DiffType != nil && c.DiffType != *f.DiffType {
			continue
		}
		filtered = append(filtered, c)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	return chapterplan.CandidateListResult{
		Items:  filtered,
		Total:  len(filtered),
		Limit:  limit,
		Offset: f.Offset,
	}, nil
}

func (m *mockStore) GetCandidateByID(ctx context.Context, candidateID uuid.UUID) (chapterplan.Candidate, error) {
	for _, c := range m.candidates {
		if c.ID == candidateID {
			return c, nil
		}
	}
	return chapterplan.Candidate{}, chapterplan.ErrCandidateNotFound
}

func (m *mockStore) ListRevisions(ctx context.Context, chapterPlanID uuid.UUID, limit, offset int) (chapterplan.RevisionListResult, error) {
	var filtered []chapterplan.Revision
	for _, r := range m.revisions {
		if r.ChapterPlanID == chapterPlanID {
			filtered = append(filtered, r)
		}
	}
	if limit <= 0 {
		limit = 20
	}
	return chapterplan.RevisionListResult{
		Items:  filtered,
		Total:  len(filtered),
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (m *mockStore) GetChapterPlanningSummary(ctx context.Context, projectID uuid.UUID) (chapterplan.Summary, error) {
	return chapterplan.Summary{
		CurrentChapterCount:     len(m.plans),
		PendingConfirmationCount: len(m.plans),
		ConfirmedChapterCount:   0,
		CandidateBatchCounts: chapterplan.CandidateBatchCounts{
			Ready: len(m.batches),
		},
		ActiveRun: []byte("null"),
	}, nil
}

type mockProjectReader struct{}

func (m mockProjectReader) Get(ctx context.Context, id uuid.UUID) (any, error) {
	return nil, nil
}

func TestCandidateReadModel_QueryAndSummary(t *testing.T) {
	projectID := uuid.New()
	batchID := uuid.New()
	runID := uuid.New()
	candidateID := uuid.New()
	planID := uuid.New()

	store := &mockStore{
		batches: []chapterplan.CandidateBatch{
			{
				ID:                  batchID,
				ProjectID:           projectID,
				SourceWorkflowRunID: runID,
				GenerationMode:      "full",
				Status:              "ready",
				CandidateCount:      1,
				PendingCount:        1,
				CreatedAt:           time.Now(),
			},
		},
		candidates: []chapterplan.Candidate{
			{
				ID:        candidateID,
				BatchID:   batchID,
				ProjectID: projectID,
				ChapterNo: 1,
				DiffType:  "new",
				Status:    "pending",
			},
		},
		revisions: []chapterplan.Revision{
			{
				ID:            uuid.New(),
				ChapterPlanID: planID,
				ProjectID:     projectID,
				RevisionNo:    1,
				ChangeType:    "legacy_backfill",
			},
		},
		plans: []chapterplan.Plan{
			{
				ID:        planID,
				ProjectID: projectID,
				ChapterNo: 1,
				Title:     "Chapter 1",
				Status:    "pending_confirmation",
			},
		},
	}

	ctx := context.Background()

	// 1. List Batches
	res, err := store.ListCandidateBatches(ctx, projectID, chapterplan.BatchFilter{})
	if err != nil {
		t.Fatalf("ListCandidateBatches failed: %v", err)
	}
	if res.Total != 1 || res.Items[0].ID != batchID {
		t.Errorf("unexpected batch list result: %+v", res)
	}

	// 2. Get Batch
	batch, err := store.GetCandidateBatchByID(ctx, batchID)
	if err != nil {
		t.Fatalf("GetCandidateBatchByID failed: %v", err)
	}
	if batch.ID != batchID {
		t.Errorf("unexpected batch: %+v", batch)
	}

	// 3. List Candidates
	cRes, err := store.ListCandidates(ctx, batchID, chapterplan.CandidateFilter{})
	if err != nil {
		t.Fatalf("ListCandidates failed: %v", err)
	}
	if cRes.Total != 1 || cRes.Items[0].ID != candidateID {
		t.Errorf("unexpected candidate list result: %+v", cRes)
	}

	// 4. Get Candidate
	candidate, err := store.GetCandidateByID(ctx, candidateID)
	if err != nil {
		t.Fatalf("GetCandidateByID failed: %v", err)
	}
	if candidate.ID != candidateID {
		t.Errorf("unexpected candidate: %+v", candidate)
	}

	// 5. List Revisions
	revRes, err := store.ListRevisions(ctx, planID, 20, 0)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}
	if revRes.Total != 1 || revRes.Items[0].ChapterPlanID != planID {
		t.Errorf("unexpected revision list result: %+v", revRes)
	}

	// 6. Summary
	summary, err := store.GetChapterPlanningSummary(ctx, projectID)
	if err != nil {
		t.Fatalf("GetChapterPlanningSummary failed: %v", err)
	}
	if summary.CurrentChapterCount != 1 || summary.CandidateBatchCounts.Ready != 1 {
		t.Errorf("unexpected summary result: %+v", summary)
	}
}

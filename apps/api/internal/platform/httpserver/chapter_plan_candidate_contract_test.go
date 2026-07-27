package httpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
	"github.com/local/ai-content-factory/apps/api/internal/platform/httpserver"
)

type candidateMockApp struct {
	batches    []chapterplan.CandidateBatch
	candidates []chapterplan.Candidate
	revisions  []chapterplan.Revision
}

func (m *candidateMockApp) List(ctx context.Context, id uuid.UUID) ([]chapterplan.Plan, error) {
	return nil, nil
}
func (m *candidateMockApp) Get(ctx context.Context, id uuid.UUID) (chapterplan.Plan, error) {
	return chapterplan.Plan{}, chapterplan.ErrNotFound
}
func (m *candidateMockApp) GenerateMock(ctx context.Context, id uuid.UUID, cmd chapterplan.MockGenerateCommand) (chapterplan.MockGenerateResult, error) {
	return chapterplan.MockGenerateResult{}, nil
}
func (m *candidateMockApp) Update(ctx context.Context, id uuid.UUID, cmd chapterplan.UpdateCommand) (chapterplan.Plan, error) {
	return chapterplan.Plan{}, nil
}
func (m *candidateMockApp) Delete(ctx context.Context, id uuid.UUID, v int) error { return nil }
func (m *candidateMockApp) Confirm(ctx context.Context, id uuid.UUID, s []chapterplan.Selection) ([]chapterplan.Plan, error) {
	return nil, nil
}

func (m *candidateMockApp) ListCandidateBatches(ctx context.Context, projectID uuid.UUID, f chapterplan.BatchFilter) (chapterplan.BatchListResult, error) {
	return chapterplan.BatchListResult{
		Items:  m.batches,
		Total:  len(m.batches),
		Limit:  20,
		Offset: 0,
	}, nil
}

func (m *candidateMockApp) GetCandidateBatchByID(ctx context.Context, batchID uuid.UUID) (chapterplan.CandidateBatch, error) {
	for _, b := range m.batches {
		if b.ID == batchID {
			return b, nil
		}
	}
	return chapterplan.CandidateBatch{}, chapterplan.ErrBatchNotFound
}

func (m *candidateMockApp) ListCandidates(ctx context.Context, batchID uuid.UUID, f chapterplan.CandidateFilter) (chapterplan.CandidateListResult, error) {
	return chapterplan.CandidateListResult{
		Items:  m.candidates,
		Total:  len(m.candidates),
		Limit:  20,
		Offset: 0,
	}, nil
}

func (m *candidateMockApp) GetCandidateByID(ctx context.Context, candidateID uuid.UUID) (chapterplan.Candidate, error) {
	for _, c := range m.candidates {
		if c.ID == candidateID {
			return c, nil
		}
	}
	return chapterplan.Candidate{}, chapterplan.ErrCandidateNotFound
}

func (m *candidateMockApp) ListRevisions(ctx context.Context, chapterPlanID uuid.UUID, limit, offset int) (chapterplan.RevisionListResult, error) {
	return chapterplan.RevisionListResult{
		Items:  m.revisions,
		Total:  len(m.revisions),
		Limit:  20,
		Offset: 0,
	}, nil
}

func (m *candidateMockApp) GetChapterPlanningSummary(ctx context.Context, projectID uuid.UUID) (chapterplan.Summary, error) {
	return chapterplan.Summary{
		CurrentChapterCount:      1,
		PendingConfirmationCount: 1,
		ConfirmedChapterCount:    0,
		CandidateBatchCounts: chapterplan.CandidateBatchCounts{
			Ready: 1,
		},
		ActiveRun: []byte("null"),
	}, nil
}

func (m *candidateMockApp) UpdateCandidate(ctx context.Context, cmd chapterplan.UpdateCandidateCommand) (chapterplan.Candidate, error) {
	for i := range m.candidates {
		if m.candidates[i].ID == cmd.CandidateID {
			if m.candidates[i].Version != cmd.ExpectedCandidateVersion {
				return chapterplan.Candidate{}, chapterplan.ErrVersionConflict
			}
			m.candidates[i].Version++
			m.candidates[i].CurrentSnapshot = cmd.CurrentSnapshot
			return m.candidates[i], nil
		}
	}
	return chapterplan.Candidate{}, chapterplan.ErrCandidateNotFound
}

func (m *candidateMockApp) CompareCandidate(ctx context.Context, candidateID uuid.UUID) (chapterplan.CandidateComparison, error) {
	for _, c := range m.candidates {
		if c.ID == candidateID {
			return chapterplan.CandidateComparison{Candidate: c}, nil
		}
	}
	return chapterplan.CandidateComparison{}, chapterplan.ErrCandidateNotFound
}

func (m *candidateMockApp) RecompareCandidate(ctx context.Context, cmd chapterplan.RecompareCandidateCommand) (chapterplan.CandidateComparison, error) {
	for i := range m.candidates {
		if m.candidates[i].ID == cmd.CandidateID {
			if m.candidates[i].Version != cmd.ExpectedCandidateVersion {
				return chapterplan.CandidateComparison{}, chapterplan.ErrVersionConflict
			}
			m.candidates[i].Version++
			m.candidates[i].Status = "pending"
			return chapterplan.CandidateComparison{Candidate: m.candidates[i]}, nil
		}
	}
	return chapterplan.CandidateComparison{}, chapterplan.ErrCandidateNotFound
}

func (m *candidateMockApp) AdoptCandidate(ctx context.Context, cmd chapterplan.AdoptCandidateCommand) (chapterplan.AdoptCandidateResult, error) {
	for i := range m.candidates {
		if m.candidates[i].ID == cmd.CandidateID {
			m.candidates[i].Status = "adopted"
			m.candidates[i].Version++
			return chapterplan.AdoptCandidateResult{Outcome: "adopted", Candidate: m.candidates[i]}, nil
		}
	}
	return chapterplan.AdoptCandidateResult{}, chapterplan.ErrCandidateNotFound
}

func (m *candidateMockApp) BulkAdoptCandidates(ctx context.Context, cmd chapterplan.BulkAdoptCommand) (chapterplan.BulkAdoptResult, error) {
	return chapterplan.BulkAdoptResult{}, nil
}

func (m *candidateMockApp) DiscardCandidate(ctx context.Context, cmd chapterplan.DiscardCandidateCommand) (chapterplan.Candidate, error) {
	for i := range m.candidates {
		if m.candidates[i].ID == cmd.CandidateID {
			m.candidates[i].Status = "discarded"
			m.candidates[i].Version++
			return m.candidates[i], nil
		}
	}
	return chapterplan.Candidate{}, chapterplan.ErrCandidateNotFound
}

func (m *candidateMockApp) AbandonBatch(ctx context.Context, cmd chapterplan.AbandonBatchCommand) (chapterplan.CandidateBatch, error) {
	for i := range m.batches {
		if m.batches[i].ID == cmd.BatchID {
			m.batches[i].Status = "abandoned"
			m.batches[i].Version++
			return m.batches[i], nil
		}
	}
	return chapterplan.CandidateBatch{}, chapterplan.ErrBatchNotFound
}

func TestChapterPlanCandidateHTTPContract(t *testing.T) {
	projectID := uuid.New()
	batchID := uuid.New()
	candidateID := uuid.New()
	planID := uuid.New()

	app := &candidateMockApp{
		batches: []chapterplan.CandidateBatch{
			{
				ID:                  batchID,
				ProjectID:           projectID,
				SourceWorkflowRunID: uuid.New(),
				GenerationMode:      "full",
				Status:              "ready",
				CandidateCount:      1,
				Version:             1,
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			},
		},
		candidates: []chapterplan.Candidate{
			{
				ID:                candidateID,
				BatchID:           batchID,
				ProjectID:         projectID,
				ChapterNo:         1,
				GeneratedSnapshot: []byte(`{}`),
				CurrentSnapshot:   []byte(`{}`),
				DiffType:          "new",
				Status:            "pending",
				Version:           1,
			},
		},
		revisions: []chapterplan.Revision{
			{
				ID:            uuid.New(),
				ChapterPlanID: planID,
				ProjectID:     projectID,
				RevisionNo:    1,
				Snapshot:      []byte(`{}`),
				ChangeType:    "legacy_backfill",
			},
		},
	}

	server := httpserver.New(":0", nil, app)

	// 1. List Batches
	req := httptest.NewRequest("GET", "/api/v1/projects/"+projectID.String()+"/chapter-plan-candidate-batches", nil)
	w := httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for list batches, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Get Batch
	req = httptest.NewRequest("GET", "/api/v1/chapter-plan-candidate-batches/"+batchID.String(), nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for get batch, got %d: %s", w.Code, w.Body.String())
	}

	// 3. List Candidates
	req = httptest.NewRequest("GET", "/api/v1/chapter-plan-candidate-batches/"+batchID.String()+"/candidates", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for list candidates, got %d: %s", w.Code, w.Body.String())
	}

	// 4. Get Candidate
	req = httptest.NewRequest("GET", "/api/v1/chapter-plan-candidates/"+candidateID.String(), nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for get candidate, got %d: %s", w.Code, w.Body.String())
	}

	// 5. Update Candidate
	patchBody := `{"expectedCandidateVersion":1,"currentSnapshot":{"title":"Updated"}}`
	req = httptest.NewRequest("PATCH", "/api/v1/chapter-plan-candidates/"+candidateID.String(), strings.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-update-key")
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for update candidate, got %d: %s", w.Code, w.Body.String())
	}

	// 6. Compare Candidate
	req = httptest.NewRequest("GET", "/api/v1/chapter-plan-candidates/"+candidateID.String()+"/compare", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for compare candidate, got %d: %s", w.Code, w.Body.String())
	}

	// 7. Recompare Candidate
	recompareBody := `{"expectedCandidateVersion":2}`
	req = httptest.NewRequest("POST", "/api/v1/chapter-plan-candidates/"+candidateID.String()+"/recompare", strings.NewReader(recompareBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-recompare-key")
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for recompare candidate, got %d: %s", w.Code, w.Body.String())
	}

	// 8. Adopt Candidate
	adoptBody := `{"expectedCandidateVersion":3}`
	req = httptest.NewRequest("POST", "/api/v1/chapter-plan-candidates/"+candidateID.String()+"/adopt", strings.NewReader(adoptBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-adopt-key")
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for adopt candidate, got %d: %s", w.Code, w.Body.String())
	}

	// 9. Abandon Batch
	abandonBody := `{"expectedBatchVersion":1,"acknowledgeAdoptedChaptersRemain":true}`
	req = httptest.NewRequest("POST", "/api/v1/chapter-plan-candidate-batches/"+batchID.String()+"/abandon", strings.NewReader(abandonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-abandon-key")
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for abandon batch, got %d: %s", w.Code, w.Body.String())
	}

	// 10. List Revisions
	req = httptest.NewRequest("GET", "/api/v1/chapter-plans/"+planID.String()+"/revisions", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for list revisions, got %d: %s", w.Code, w.Body.String())
	}

	// 11. Get Summary
	req = httptest.NewRequest("GET", "/api/v1/projects/"+projectID.String()+"/chapter-planning-summary", nil)
	w = httptest.NewRecorder()
	server.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for get summary, got %d: %s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			CurrentChapterCount int `json:"currentChapterCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal summary response: %v", err)
	}
	if env.Data.CurrentChapterCount != 1 {
		t.Errorf("expected currentChapterCount 1, got %d", env.Data.CurrentChapterCount)
	}
}

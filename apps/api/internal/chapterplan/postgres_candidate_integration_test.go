package chapterplan

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPostgresCandidateIntegration(t *testing.T) {
	db, ctx := openIntegrationDB(t)

	repo, err := NewPostgresRepository(db, "test-hmac-secret-1234567890")
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	ingestor := NewResultIngestor(db)

	projectID := uuid.New()
	runID := uuid.New()

	// Seed Project and WorkflowRun
	_, err = db.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','test')", projectID, "Candidate Test Project")
	if err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}

	connID := uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO workflow_connections (id, name, connection_type, base_url, auth_type, timeout_seconds, type_config) VALUES ($1, $2, 'n8n', 'http://localhost:5678', 'api_key', 30, '{}'::jsonb)", connID, "Test Connection "+connID.String()); err != nil {
		t.Fatalf("failed to seed connection: %v", err)
	}

	configID := uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO workflow_configurations (id, name, connection_id, applicable_stages, type_config, input_contract_version, output_contract_version) VALUES ($1, $2, $3, '[\"chapter_planning\"]'::jsonb, '{}'::jsonb, 'v1', 'v1')", configID, "Test Config "+configID.String(), connID); err != nil {
		t.Fatalf("failed to seed configuration: %v", err)
	}

	runNumber := fmt.Sprintf("run-%d", time.Now().UnixNano())
	_, err = db.Exec(ctx, `
		INSERT INTO workflow_run_records (
			id, run_number, project_id, stage, workflow_configuration_id, trigger_source, status,
			configuration_snapshot, input_payload, started_at, finished_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, 'chapter_planning', $4, 'manual', 'succeeded',
			'{}'::jsonb, '{}'::jsonb, NOW(), NOW(), NOW(), NOW()
		)
	`, runID, runNumber, projectID, configID)
	if err != nil {
		t.Fatalf("failed to seed workflow run: %v", err)
	}

	digest := "11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff"

	// Seed Storyline, Material, Foreshadowing
	stID := uuid.New()
	matID := uuid.New()
	foreID := uuid.New()

	_, _ = db.Exec(ctx, "INSERT INTO storylines(id,project_id,type,relation,name,status,sort_order,created_by) VALUES($1,$2,'main','root','Main Arc','active',0,'test')", stID, projectID)
	_, _ = db.Exec(ctx, "INSERT INTO materials(id,type,name,created_by) VALUES($1,'reference','Sword','test')", matID)
	_, _ = db.Exec(ctx, "INSERT INTO project_material_usages(id,project_id,material_id,usage_type,created_by) VALUES($1,$2,$3,'reference','test')", uuid.New(), projectID, matID)
	_, _ = db.Exec(ctx, "INSERT INTO foreshadowings(id,project_id,title,priority,status,created_by) VALUES($1,$2,'Secret Prophecy','medium','planned','test')", foreID, projectID)

	input := IngestInput{
		Run: RunReference{
			RunID:     runID,
			ProjectID: projectID,
		},
		Context: GenerationContextSnapshot{
			InputDigest: digest,
		},
		NormalizedOutput: NormalizedChapterPlanOutput{
			ProjectID:           projectID,
			GenerationMode:      "full",
			Target:              BatchTarget{StartChapterNo: 1, EndChapterNo: 2, RequestedChapterCount: 2},
			SourceWorkflowRunID: runID,
			Candidates: []NormalizedCandidate{
				{
					ChapterNo:      1,
					Title:          "Chapter 1 - The Beginning",
					Summary:        "Summary 1",
					ChapterPurpose: "plot_advance",
					StorylineRefs: []NormalizedReference{
						{ID: stID, ProjectID: projectID, Label: "Main Arc", Relation: "primary", Position: 0, Version: 1},
					},
					MaterialRefs: []NormalizedReference{
						{ID: matID, ProjectID: projectID, Label: "Sword", Relation: "material_ref", Position: 0, Version: 1},
					},
					ForeshadowingRefs: []NormalizedReference{
						{ID: foreID, ProjectID: projectID, Label: "Secret Prophecy", Relation: "foreshadowing_ref", Position: 0, Version: 1},
					},
					GenerationBasis: GenerationBasis{ContextSummary: "Basis 1"},
				},
				{
					ChapterNo:      2,
					Title:          "Chapter 2 - The Conflict",
					Summary:        "Summary 2",
					ChapterPurpose: "conflict_escalation",
					StorylineRefs: []NormalizedReference{
						{ID: stID, ProjectID: projectID, Label: "Main Arc", Relation: "primary", Position: 0, Version: 1},
					},
					GenerationBasis: GenerationBasis{ContextSummary: "Basis 2"},
				},
			},
			Metadata: OutputMetadata{
				InputDigest:         digest,
				GeneratedAt:         time.Now().Format(time.RFC3339),
				SafeProviderSummary: "OK",
			},
		},
	}

	// 1. Result Ingestion
	batch, err := ingestor.Ingest(ctx, input)
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	if batch.CandidateCount != 2 || batch.Status != "ready" {
		t.Errorf("unexpected batch status: count=%d status=%s", batch.CandidateCount, batch.Status)
	}

	// 2. Concurrent Replay of same RunReference
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b2, err := ingestor.Ingest(ctx, input)
			if err != nil {
				t.Errorf("concurrent ingest error: %v", err)
			}
			if b2.ID != batch.ID {
				t.Errorf("expected same batch ID on replay, got %s vs %s", b2.ID, batch.ID)
			}
		}()
	}
	wg.Wait()

	// List Candidates
	candsResult, err := repo.ListCandidates(ctx, batch.ID, CandidateFilter{Limit: 10})
	if err != nil || len(candsResult.Items) != 2 {
		t.Fatalf("ListCandidates failed: err=%v count=%d", err, len(candsResult.Items))
	}

	cand1 := candsResult.Items[0]

	// 3. New Chapter Adopt
	adoptRes, err := repo.AdoptCandidate(ctx, AdoptCandidateCommand{
		CandidateID:                cand1.ID,
		ExpectedCandidateVersion:   cand1.Version,
		ExpectedChapterPlanVersion: nil,
		IdempotencyKey:             "pg-adopt-key-1",
		ActorID:                    "pg-test",
	})
	if err != nil {
		t.Fatalf("AdoptCandidate failed: %v", err)
	}
	if adoptRes.Outcome != "adopted" || adoptRes.ChapterPlan == nil || adoptRes.Revision == nil {
		t.Fatalf("unexpected adopt result: outcome=%s", adoptRes.Outcome)
	}

	// Verify Relationship Table Replacement & References
	var stCount, matCount, foreCount int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_storylines WHERE chapter_plan_id = $1", adoptRes.ChapterPlan.ID).Scan(&stCount)
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_materials WHERE chapter_plan_id = $1", adoptRes.ChapterPlan.ID).Scan(&matCount)
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_foreshadowings WHERE chapter_plan_id = $1", adoptRes.ChapterPlan.ID).Scan(&foreCount)

	if stCount != 1 || matCount != 1 || foreCount != 1 {
		t.Errorf("relationship table counts mismatch: st=%d mat=%d fore=%d", stCount, matCount, foreCount)
	}

	// 4. Same Key Idempotency Replay
	adoptResReplay, err := repo.AdoptCandidate(ctx, AdoptCandidateCommand{
		CandidateID:                cand1.ID,
		ExpectedCandidateVersion:   cand1.Version,
		ExpectedChapterPlanVersion: nil,
		IdempotencyKey:             "pg-adopt-key-1",
		ActorID:                    "pg-test",
	})
	if err != nil {
		t.Fatalf("AdoptCandidate replay failed: %v", err)
	}
	if adoptResReplay.Outcome != "adopted" || adoptResReplay.ChapterPlan.ID != adoptRes.ChapterPlan.ID {
		t.Errorf("idempotency replay mismatched result")
	}

	// 5. Same Key Different Payload Conflict
	_, err = repo.AdoptCandidate(ctx, AdoptCandidateCommand{
		CandidateID:                cand1.ID,
		ExpectedCandidateVersion:   999,
		ExpectedChapterPlanVersion: nil,
		IdempotencyKey:             "pg-adopt-key-1",
		ActorID:                    "pg-test",
	})
	if !errors.Is(err, ErrIdempotencyKeyReused) {
		t.Errorf("expected ErrIdempotencyKeyReused, got %v", err)
	}

	// 6. Abandon Batch
	bCurrent, _ := repo.GetCandidateBatchByID(ctx, batch.ID)
	abandonedBatch, err := repo.AbandonBatch(ctx, AbandonBatchCommand{
		BatchID:                          batch.ID,
		ExpectedBatchVersion:             bCurrent.Version,
		Reason:                           nil,
		AcknowledgeAdoptedChaptersRemain: true,
		IdempotencyKey:                   "pg-abandon-key-1",
		ActorID:                          "pg-test",
	})
	if err != nil {
		t.Fatalf("AbandonBatch failed: %v", err)
	}
	if abandonedBatch.Status != "abandoned" {
		t.Errorf("expected batch status abandoned, got %s", abandonedBatch.Status)
	}

	// Verify Adopted Chapter Remains Intact
	planCheck, err := repo.GetByID(ctx, adoptRes.ChapterPlan.ID)
	if err != nil || planCheck.Status != "pending_confirmation" {
		t.Errorf("adopted chapter plan deleted or rolled back improperly: %v", err)
	}
}

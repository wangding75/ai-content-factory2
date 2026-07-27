package chapterplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
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
			InputDigest:   digest,
			InputSnapshot: json.RawMessage(`{"generationMode":"full","target":{"startChapterNo":1,"endChapterNo":2,"requestedChapterCount":2}}`),
			StorylineSnapshot: json.RawMessage(fmt.Sprintf(`{"available":[{"id":%q}],"materials":[{"materialId":%q}],"foreshadowings":[{"id":%q}]}`,
				stID.String(), matID.String(), foreID.String())),
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

func TestPostgresChapterPlanningConsumptionSummaryErrorsAreSafe(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	repo, err := NewPostgresRepository(db, "test-hmac-secret-1234567890")
	if err != nil {
		t.Fatal(err)
	}
	projectID, connectionID, configurationID, runID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err = db.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','test')", projectID, "consumption summary"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE id=$1", projectID) })
	if _, err = db.Exec(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config) VALUES($1,$2,'n8n','http://localhost:5678','api_key',30,'{}')", connectionID, "summary connection "+connectionID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version) VALUES($1,$2,$3,'[\"chapter_planning\"]','{}','v1','v1')", configurationID, "summary config "+configurationID.String(), connectionID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO workflow_run_records(id,run_number,project_id,stage,workflow_configuration_id,trigger_source,status,configuration_snapshot,input_payload,started_at,finished_at,created_at,updated_at) VALUES($1,$2,$3,'chapter_planning',$4,'manual','succeeded','{}','{}',NOW(),NOW(),NOW(),NOW())", runID, "summary-"+runID.String(), projectID, configurationID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		status ConsumptionStatus
		want   error
	}{
		{ConsumptionOutputValidationFailed, ErrOutputValidationFailed},
		{ConsumptionResultConsumptionFailed, ErrIngestionTransaction},
	} {
		t.Run(string(tc.status), func(t *testing.T) {
			code, reason, action := string(tc.status), "safe reason", "retry_run"
			if _, err := db.Exec(ctx, `INSERT INTO chapter_plan_result_consumptions(workflow_run_id,project_id,status,failure_code,safe_reason,retry_action) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(workflow_run_id) DO UPDATE SET status=EXCLUDED.status,failure_code=EXCLUDED.failure_code,safe_reason=EXCLUDED.safe_reason,retry_action=EXCLUDED.retry_action,updated_at=NOW()`, runID, projectID, tc.status, code, reason, action); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.GetChapterPlanningSummary(ctx, projectID); !errors.Is(err, tc.want) {
				t.Fatalf("summary error=%v, want %v", err, tc.want)
			}
		})
	}
}

func seedConsumptionRun(t *testing.T, ctx context.Context, db *pgxpool.Pool, status string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	projectID, connectionID, configurationID, runID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if _, err := db.Exec(ctx, "INSERT INTO projects(id,name,type,created_by) VALUES($1,$2,'novel','test')", projectID, "consumption-"+projectID.String()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.Exec(context.Background(), "DELETE FROM projects WHERE id=$1", projectID) })
	if _, err := db.Exec(ctx, "INSERT INTO workflow_connections(id,name,connection_type,base_url,auth_type,timeout_seconds,type_config) VALUES($1,$2,'n8n','http://localhost:5678','api_key',30,'{}')", connectionID, "consumption connection "+connectionID.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO workflow_configurations(id,name,connection_id,applicable_stages,type_config,input_contract_version,output_contract_version) VALUES($1,$2,$3,'[\"chapter_planning\"]','{}','v1','v1')", configurationID, "consumption configuration "+configurationID.String(), connectionID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO workflow_run_records(id,run_number,project_id,stage,workflow_configuration_id,trigger_source,status,configuration_snapshot,input_payload,output_payload,error_code,error_message,error_details,started_at,finished_at,cancelled_at,created_at,updated_at) VALUES($1,$2,$3,'chapter_planning',$4,'manual',$5::text,'{}','{}','{}',CASE WHEN $5::text='failed' THEN 'runtime_failed' END,CASE WHEN $5::text='failed' THEN 'The runtime failed safely.' END,CASE WHEN $5::text='failed' THEN '{}'::jsonb ELSE NULL END,NOW(),NOW(),CASE WHEN $5::text='cancelled' THEN NOW() END,NOW(),NOW())`, runID, "consumption-run-"+runID.String(), projectID, configurationID, status); err != nil {
		t.Fatal(err)
	}
	return projectID, runID
}

func TestPostgresConsumptionStatePersistsAcrossRepositoryInstances(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	projectID, runID := seedConsumptionRun(t, ctx, db, "succeeded")
	batchID := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO chapter_plan_candidate_batches(id,project_id,source_workflow_run_id,generation_mode,range_start,range_end,requested_chapter_count,input_digest,input_snapshot,storyline_selection_snapshot,context_options,workflow_binding_snapshot) VALUES($1,$2,$3,'range',1,1,1,$4,'{}','{}','{}','{}')`, batchID, projectID, runID, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}
	first := NewConsumptionRepository(db)
	code, reason, action := "result_consumption_failed", "The generated result could not be stored safely.", "retry_run"
	if err := first.Set(ctx, runID, projectID, ConsumptionResultConsumptionFailed, nil, &code, &reason, &action); err != nil {
		t.Fatal(err)
	}
	// A fresh repository proves the state comes only from PostgreSQL.
	second := NewConsumptionRepository(db)
	failed, err := second.Get(ctx, runID)
	if err != nil || failed.Status != ConsumptionResultConsumptionFailed || failed.FailureCode == nil || *failed.FailureCode != code || failed.SafeReason == nil || *failed.SafeReason != reason || failed.RetryAction == nil || *failed.RetryAction != action || failed.CandidateBatchID != nil || failed.ConsumedAt != nil {
		t.Fatalf("persisted failure=%+v err=%v", failed, err)
	}
	consumer := NewRuntimeConsumer(successfulConsumptionIngestor{batch: CandidateBatch{ID: batchID}}, second)
	run := workflowrun.WorkflowRun{
		ID:            runID,
		ProjectID:     projectID,
		Stage:         "chapter_planning",
		Status:        workflowrun.StatusSucceeded,
		InputPayload:  []byte(`{"generationContext":{"inputDigest":"a"}}`),
		OutputPayload: []byte(`{"projectId":"` + projectID.String() + `","generationMode":"range","target":{"startChapterNo":1,"endChapterNo":1,"requestedChapterCount":1},"sourceWorkflowRunId":"` + runID.String() + `","candidates":[],"metadata":{"inputDigest":"a","generatedAt":"now","safeProviderSummary":"safe"}}`),
	}
	if err := consumer.ConsumeSucceededRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	third := NewConsumptionRepository(db)
	consumed, err := third.Get(ctx, runID)
	if err != nil || consumed.Status != ConsumptionConsumed || consumed.CandidateBatchID == nil || *consumed.CandidateBatchID != batchID || consumed.ConsumedAt == nil || consumed.FailureCode != nil || consumed.SafeReason != nil || consumed.RetryAction != nil {
		t.Fatalf("persisted consumption=%+v err=%v", consumed, err)
	}
}

type failingConsumptionIngestor struct{ err error }

func (i failingConsumptionIngestor) Ingest(context.Context, IngestInput) (CandidateBatch, error) {
	return CandidateBatch{}, i.err
}

type successfulConsumptionIngestor struct{ batch CandidateBatch }

func (i successfulConsumptionIngestor) Ingest(context.Context, IngestInput) (CandidateBatch, error) {
	return i.batch, nil
}

func TestPostgresConsumptionFailureCanRetryToConsumed(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	projectID, runID := seedConsumptionRun(t, ctx, db, "succeeded")
	consumer := NewRuntimeConsumer(failingConsumptionIngestor{err: errors.New("database unavailable")}, NewConsumptionRepository(db))
	run := workflowrun.WorkflowRun{ID: runID, ProjectID: projectID, Stage: "chapter_planning", Status: workflowrun.StatusSucceeded, InputPayload: []byte(`{"generationContext":{"inputDigest":"a"}}`), OutputPayload: []byte(`{}`)}
	if err := consumer.ConsumeSucceededRun(ctx, run); !errors.Is(err, ErrIngestionTransaction) {
		t.Fatalf("failure consumption error=%v", err)
	}
	state, err := NewConsumptionRepository(db).Get(ctx, runID)
	if err != nil || state.Status != ConsumptionResultConsumptionFailed || state.FailureCode == nil || *state.FailureCode != "result_consumption_failed" {
		t.Fatalf("failure state=%+v err=%v", state, err)
	}
	batchID := uuid.New()
	if _, err := db.Exec(ctx, `INSERT INTO chapter_plan_candidate_batches(id,project_id,source_workflow_run_id,generation_mode,range_start,range_end,requested_chapter_count,input_digest,input_snapshot,storyline_selection_snapshot,context_options,workflow_binding_snapshot) VALUES($1,$2,$3,'range',1,1,1,$4,'{}','{}','{}','{}')`, batchID, projectID, runID, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); err != nil {
		t.Fatal(err)
	}
	if err := NewConsumptionRepository(db).Set(ctx, runID, projectID, ConsumptionConsumed, &batchID, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if summary, err := mustNewRepo(t, db).GetChapterPlanningSummary(ctx, projectID); err != nil || summary.ActiveRun == nil {
		t.Fatalf("summary after retry=%+v err=%v", summary, err)
	}
	state, err = NewConsumptionRepository(db).Get(ctx, runID)
	if err != nil || state.Status != ConsumptionConsumed || state.CandidateBatchID == nil || *state.CandidateBatchID != batchID || state.ConsumedAt == nil {
		t.Fatalf("retried state=%+v err=%v", state, err)
	}
}

func TestRuntimeFailureDoesNotTriggerChapterPlanConsumption(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	for _, status := range []workflowrun.Status{workflowrun.StatusFailed, workflowrun.StatusCancelled} {
		t.Run(string(status), func(t *testing.T) {
			projectID, runID := seedConsumptionRun(t, ctx, db, string(status))
			consumer := NewRuntimeConsumer(failingConsumptionIngestor{err: errors.New("must not run")}, NewConsumptionRepository(db))
			run := workflowrun.WorkflowRun{ID: runID, ProjectID: projectID, Stage: "chapter_planning", Status: status}
			if err := consumer.ConsumeSucceededRun(ctx, run); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM chapter_plan_result_consumptions WHERE workflow_run_id=$1", runID).Scan(&count); err != nil || count != 0 {
				t.Fatalf("consumption rows=%d err=%v", count, err)
			}
			if _, err := mustNewRepo(t, db).GetChapterPlanningSummary(ctx, projectID); err != nil {
				t.Fatalf("runtime %s was mapped as consumption failure: %v", status, err)
			}
		})
	}
}

func TestSummaryChangesToConsumedAfterRetry(t *testing.T) {
	db, ctx := openIntegrationDB(t)
	projectID, runID := seedConsumptionRun(t, ctx, db, "succeeded")
	consumptions := NewConsumptionRepository(db)
	code, reason, action := "output_validation_failed", "The runtime output is invalid.", "retry_run"
	if err := consumptions.Set(ctx, runID, projectID, ConsumptionOutputValidationFailed, nil, &code, &reason, &action); err != nil {
		t.Fatal(err)
	}
	repo := mustNewRepo(t, db)
	if _, err := repo.GetChapterPlanningSummary(ctx, projectID); !errors.Is(err, ErrOutputValidationFailed) {
		t.Fatalf("output validation summary error=%v", err)
	}
	if err := consumptions.Set(ctx, runID, projectID, ConsumptionConsumed, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetChapterPlanningSummary(ctx, projectID); err != nil {
		t.Fatalf("consumed retry summary error=%v", err)
	}
}

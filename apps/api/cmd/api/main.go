package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
	"github.com/local/ai-content-factory/apps/api/internal/foreshadowing"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/material"
	"github.com/local/ai-content-factory/apps/api/internal/planning"
	"github.com/local/ai-content-factory/apps/api/internal/platform/config"
	"github.com/local/ai-content-factory/apps/api/internal/platform/httpserver"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	projectRepository := project.NewPostgresRepository(pool)
	projects := project.NewService(projectRepository)
	plannings := planning.NewPostgresService(projectRepository, pool)
	materials := material.NewService(pool)
	projectMaterials := material.NewPostgresProjectMaterialService(projectRepository, pool)
	storylines := storyline.NewPostgresService(projectRepository, pool)
	foreshadowings := foreshadowing.NewPostgresService(projectRepository, pool)
	hmacSecret := cfg.ChapterPlanIdempotencyHMACSecret
	if hmacSecret == "" {
		log.Fatal("CHAPTER_PLAN_IDEMPOTENCY_HMAC_SECRET environment variable is required")
	}
	chapterPlans, err := chapterplan.NewPostgresService(projectRepository, pool, hmacSecret)
	if err != nil {
		log.Fatal(err)
	}
	contentRepository := contentitem.NewPostgresRepository(pool)
	contentItems := contentitem.NewApplication(contentRepository, nil)
	contentQueries := contentitem.NewQueryService(contentRepository)
	iteration07 := contentitem.NewIteration07Application(nil, contentQueries)
	iteration08 := contentitem.NewGlobalLiteService(contentQueries)
	globalConfigurations, err := globalconfig.NewService(pool, cfg.ConfigurationEncryptionKey)
	if err != nil {
		log.Fatal(err)
	}
	workflowRuns := workflowrun.NewService(
		workflowrun.NewPostgresRepository(pool),
		projectRepository,
		workflowbinding.NewPostgresRepository(pool),
		globalConfigurations,
		globalConfigurations,
	)
	workflowRuns.SetWorkflowExecutor(workflowrun.NewN8NWorkflowExecutor(globalConfigurations.RuntimeHTTPClient(), globalConfigurations.RuntimeConnectionCredential))
	runtimeBridge := workflowrun.NewRuntimeBridge(workflowRuns)
	contentGeneration := contentitem.NewGenerationService(contentRepository, workflowbinding.NewPostgresRepository(pool), globalConfigurations, runtimeBridge, hmacSecret)
	workflowRuns.SetContentSucceededConsumer(contentGeneration)
	realReview := contentitem.NewRealReviewService(contentRepository, workflowbinding.NewPostgresRepository(pool), globalConfigurations, runtimeBridge, hmacSecret)
	contentItems.SetRealReviewService(realReview)
	workflowRuns.SetReviewSucceededConsumer(realReview)
	realRewrite := contentitem.NewRealRewriteService(contentRepository, workflowbinding.NewPostgresRepository(pool), globalConfigurations, runtimeBridge, hmacSecret)
	workflowRuns.SetRewriteSucceededConsumer(realRewrite)
	chapterPlans.ConfigureChapterPlanningRuntime(workflowbinding.NewPostgresRepository(pool), globalConfigurations, runtimeBridge)
	workflowRuns.SetSucceededConsumer(chapterplan.NewRuntimeConsumer(chapterplan.NewResultIngestor(pool), chapterplan.NewConsumptionRepository(pool)))
	workerContext, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	go workflowRuns.RunWorker(workerContext, time.Second, func(workerErr error) {
		log.Printf("workflow worker: %v", workerErr)
	})
	server := httpserver.New(cfg.APIAddress, projects, plannings, materials, projectMaterials, storylines, foreshadowings, chapterPlans, contentItems, iteration07, iteration08, contentGeneration, realReview, realRewrite, globalConfigurations, workflowbinding.NewCloseLoop(pool, projectRepository, globalConfigurations), workflowRuns)
	log.Printf("api listening on %s", cfg.APIAddress)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

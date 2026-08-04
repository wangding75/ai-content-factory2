package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer bootCancel()
	pool, err := pgxpool.New(bootCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := pool.Ping(bootCtx); err != nil {
		pool.Close()
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
		pool.Close()
		log.Fatal("CHAPTER_PLAN_IDEMPOTENCY_HMAC_SECRET environment variable is required")
	}
	chapterPlans, err := chapterplan.NewPostgresService(projectRepository, pool, hmacSecret)
	if err != nil {
		pool.Close()
		log.Fatal(err)
	}
	contentRepository := contentitem.NewPostgresRepository(pool)
	contentItems := contentitem.NewApplication(contentRepository, nil)
	contentQueries := contentitem.NewQueryService(contentRepository)
	iteration07 := contentitem.NewIteration07Application(nil, contentQueries)
	iteration08 := contentitem.NewGlobalLiteService(contentQueries)
	globalConfigurations, err := globalconfig.NewService(pool, cfg.ConfigurationEncryptionKey)
	if err != nil {
		pool.Close()
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

	// Graceful lifecycle: SIGTERM/SIGINT stop workers first, drain HTTP, then close the pool.
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		workflowRuns.RunWorker(rootCtx, time.Second, func(workerErr error) {
			log.Printf("workflow worker: %v", workerErr)
		})
	}()

	server := httpserver.New(cfg.APIAddress, projects, plannings, materials, projectMaterials, storylines, foreshadowings, chapterPlans, contentItems, iteration07, iteration08, contentGeneration, realReview, realRewrite, globalConfigurations, workflowbinding.NewCloseLoop(pool, projectRepository, globalConfigurations), workflowRuns)
	httpErr := make(chan error, 1)
	go func() {
		log.Printf("api listening on %s", cfg.APIAddress)
		httpErr <- server.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		log.Printf("shutdown signal received")
	case err := <-httpErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server failed: %v", err)
			stop()
		}
	}

	// 1-3: worker context cancelled; wait for current candidate work to finish.
	workerWait, workerCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer workerCancel()
	select {
	case <-workerDone:
	case <-workerWait.Done():
		log.Printf("workflow worker drain timed out")
	}

	// 4-6: stop accepting requests and drain in-flight handlers (execution-started, cancel, consume).
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
	select {
	case err := <-httpErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server exit: %v", err)
		}
	default:
	}

	// 7-8: close PostgreSQL last; only force after the drain budget.
	pool.Close()
	log.Printf("api stopped")
}

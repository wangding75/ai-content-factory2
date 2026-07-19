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
	chapterPlans := chapterplan.NewPostgresService(projectRepository, pool)
	contentRepository := contentitem.NewPostgresRepository(pool)
	contentItems := contentitem.NewApplication(contentRepository, nil)
	rewriteService := contentitem.NewMockRewriteService(contentRepository, contentitem.DeterministicMockRewriteProvider{}, contentitem.NewPgxRewriteTransactionRunner(pool))
	iteration07 := contentitem.NewIteration07Application(rewriteService, contentitem.NewQueryService(contentRepository))
	iteration08 := contentitem.NewGlobalLiteService(contentitem.NewQueryService(contentRepository))
	globalConfigurations, err := globalconfig.NewService(pool, cfg.ConfigurationEncryptionKey)
	if err != nil {
		log.Fatal(err)
	}
	server := httpserver.New(cfg.APIAddress, projects, plannings, materials, projectMaterials, storylines, foreshadowings, chapterPlans, contentItems, iteration07, iteration08, globalConfigurations)
	log.Printf("api listening on %s", cfg.APIAddress)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

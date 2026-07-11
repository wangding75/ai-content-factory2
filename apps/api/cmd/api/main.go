package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/local/ai-content-factory/apps/api/internal/platform/config"
	"github.com/local/ai-content-factory/apps/api/internal/platform/httpserver"
)

func main() {
	cfg := config.Load()
	server := httpserver.New(cfg.APIAddress)

	log.Printf("api listening on %s", cfg.APIAddress)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

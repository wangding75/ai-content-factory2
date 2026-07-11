package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
}

type envelope struct {
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

func New(address string) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler)
	mux.HandleFunc("GET /api/v1/meta", metaHandler)

	return &Server{
		httpServer: &http.Server{
			Addr:              address,
			Handler:           withRequestID(mux),
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
	}
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown() error {
	return s.httpServer.Close()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "api",
	})
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{
		"status": "ready",
		"checks": map[string]string{
			"api": "ok",
		},
	})
}

func metaHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]any{
		"product":           "AI Content Factory 2.0",
		"scope":             "P0",
		"content_packs":     []string{"novel"},
		"workflow_provider": "mock",
		"real_ai":           "disabled",
		"external_workflow": "disabled",
		"publishing":        "disabled",
	})
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(envelope{
		Data:      data,
		RequestID: requestIDFrom(r),
	})
}

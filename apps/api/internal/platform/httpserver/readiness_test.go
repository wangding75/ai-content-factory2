package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/local/ai-content-factory/apps/api/internal/platform/ready"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

func TestHealthzIsLivenessOnly(t *testing.T) {
	server := New(":0", project.NewService(nil))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyzWithoutCheckerIsProcessReady(t *testing.T) {
	server := New(":0", project.NewService(nil))
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyzUsesCheckerForNotReady(t *testing.T) {
	health := workflowrun.NewWorkerHealth()
	// Worker never started => not ready
	checker := &ready.Checker{
		Pool:                  nil,
		ExpectedMigrationHead: 23,
		Worker:                health,
		Now:                   func() time.Time { return time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC) },
	}
	server := New(":0", project.NewService(nil), checker)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Status string            `json:"status"`
			Checks map[string]string `json:"checks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Status != "not_ready" || body.Data.Checks["ready"] != "fail" {
		t.Fatalf("body=%+v", body.Data)
	}
}

func TestReadyzDoesNotStartWorker(t *testing.T) {
	health := workflowrun.NewWorkerHealth()
	checker := &ready.Checker{Worker: health, ExpectedMigrationHead: 1, Now: time.Now}
	server := New(":0", project.NewService(nil), checker)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil).WithContext(context.Background())
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)
	snap := health.Snapshot()
	if snap.Started {
		t.Fatal("readyz must not start the worker")
	}
}

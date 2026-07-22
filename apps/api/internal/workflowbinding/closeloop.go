package workflowbinding

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

// CloseLoop wires the Service together for the Iteration 13 closed loop and
// exposes the shared BindingService so the httpserver package can mount the
// routes without importing a handler from this domain package.
type CloseLoop struct {
	Service BindingService
}

// NewCloseLoop builds the closed loop using the shared pool and existing modules.
func NewCloseLoop(pool *pgxpool.Pool, projects project.Repository, workflows *globalconfig.Service) *CloseLoop {
	return &CloseLoop{Service: NewService(pool, NewProjectAuthorizer(projects), NewWorkflowReader(workflows), "system")}
}

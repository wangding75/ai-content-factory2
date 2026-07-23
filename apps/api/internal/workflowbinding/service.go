package workflowbinding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/local/ai-content-factory/apps/api/internal/audit"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
)

// Service implements the Iteration 13 project workflow binding closed loop.
type Service struct {
	pool      *pgxpool.Pool
	projects  ProjectRepository
	workflows WorkflowRepository
	actorID   string

	// beforeCreate, when set, is invoked just before the binding INSERT in
	// createTx.  It exists for concurrency tests that need to synchronize two
	// first-time bind attempts so both observe NotFound before either inserts
	// (mirroring the beforeIdempotencyLock hook in globalconfig).  It is nil in
	// production.
	beforeCreate func()
}

// ProjectRepository authorizes project existence and modification access.
type ProjectRepository interface {
	ExistsForModify(ctx context.Context, id uuid.UUID) error
}

// WorkflowRepository loads the global workflow configuration.
type WorkflowRepository interface {
	GetWorkflow(ctx context.Context, id uuid.UUID) (ReadWorkflowConfiguration, error)
}

func NewService(pool *pgxpool.Pool, projects ProjectRepository, workflows WorkflowRepository, actorID string) *Service {
	return &Service{pool: pool, projects: projects, workflows: workflows, actorID: actorID}
}

// ListStages returns the four fixed stages with their current binding and the
// read-only global workflow summary.  It never writes audit events.  Loading a
// bound stage's workflow summary is mandatory: a failure surfaces as an error
// rather than producing the illegal bound=true / summary=null combination.
func (s *Service) ListStages(ctx context.Context, projectID uuid.UUID) ([]StageRead, error) {
	if err := s.projects.ExistsForModify(ctx, projectID); err != nil {
		return nil, err
	}
	bindings, err := NewPostgresRepository(s.pool).ListByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	byStage := map[WorkflowBindingStage]ProjectWorkflowBinding{}
	for _, b := range bindings {
		byStage[b.Stage] = b
	}
	out := make([]StageRead, 0, len(AllStages()))
	for _, stage := range AllStages() {
		read := StageRead{Stage: stage, Bound: false}
		if b, ok := byStage[stage]; ok {
			read.Bound = true
			clone := b
			read.Binding = &clone
			summary, err := s.loadSummary(ctx, b.WorkflowConfigurationID)
			if err != nil {
				return nil, err
			}
			read.WorkflowConfigurationSummary = &summary
		}
		out = append(out, read)
	}
	return out, nil
}

func (s *Service) loadSummary(ctx context.Context, workflowID uuid.UUID) (ReadWorkflowConfiguration, error) {
	return s.workflows.GetWorkflow(ctx, workflowID)
}

// PutRequest is the decoded PUT body.
type PutRequest struct {
	WorkflowConfigurationID uuid.UUID `json:"workflowConfigurationId"`
	ExpectedVersion         *int      `json:"expectedVersion"`
	expectedVersionProvided bool
}

// UnmarshalJSON preserves the distinction between an omitted expectedVersion
// (valid for a first bind) and an explicit null (always invalid).
func (r *PutRequest) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for name := range fields {
		if name != "workflowConfigurationId" && name != "expectedVersion" {
			return fmt.Errorf("unknown field %q", name)
		}
	}
	if raw, ok := fields["workflowConfigurationId"]; ok {
		if err := json.Unmarshal(raw, &r.WorkflowConfigurationID); err != nil {
			return err
		}
	}
	r.ExpectedVersion = nil
	r.expectedVersionProvided = false
	if raw, ok := fields["expectedVersion"]; ok {
		r.expectedVersionProvided = true
		if string(raw) != "null" {
			var version int
			if err := json.Unmarshal(raw, &version); err != nil {
				return err
			}
			r.ExpectedVersion = &version
		}
	}
	return nil
}

func (r PutRequest) invalidExpectedVersion() bool {
	return r.expectedVersionProvided && r.ExpectedVersion == nil || r.ExpectedVersion != nil && *r.ExpectedVersion < 1
}

// PutResult reports the operation result and whether it was a no-op.
type PutResult struct {
	Stage    WorkflowBindingStage
	Binding  ProjectWorkflowBinding
	Summary  ReadWorkflowConfiguration
	NoChange bool
	Created  bool
}

// Put binds or rebinds a single stage.  Validation and not-applicable / disabled
// failures return without writing audit.  Version conflicts return a
// VersionConflictError so the HTTP layer can populate the frozen 409 details.
func (s *Service) Put(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, req PutRequest) (PutResult, error) {
	if err := s.projects.ExistsForModify(ctx, projectID); err != nil {
		return PutResult{}, err
	}
	if req.WorkflowConfigurationID == uuid.Nil {
		return PutResult{}, ErrValidation
	}
	if req.invalidExpectedVersion() {
		return PutResult{}, ErrValidation
	}
	wf, err := s.workflows.GetWorkflow(ctx, req.WorkflowConfigurationID)
	if err != nil {
		return PutResult{}, err
	}
	if !stageApplicable(stage, wf.ApplicableStages) {
		return PutResult{}, ErrNotApplicable
	}
	repo := NewPostgresRepository(s.pool)
	existing, err := repo.GetByProjectAndStage(ctx, projectID, stage)
	if errors.Is(err, ErrNotFound) {
		if req.ExpectedVersion != nil {
			return PutResult{}, ErrValidation
		}
		return s.create(ctx, projectID, stage, req.WorkflowConfigurationID, wf)
	}
	if err != nil {
		return PutResult{}, err
	}
	if req.ExpectedVersion == nil {
		return PutResult{}, &VersionConflictError{ProjectID: projectID, Stage: stage, ExpectedVersion: 0, CurrentVersion: existing.Version, Missing: true}
	}
	if *req.ExpectedVersion != existing.Version {
		return PutResult{}, &VersionConflictError{ProjectID: projectID, Stage: stage, ExpectedVersion: *req.ExpectedVersion, CurrentVersion: existing.Version}
	}
	if existing.WorkflowConfigurationID == req.WorkflowConfigurationID {
		return PutResult{Stage: stage, Binding: existing, Summary: wf, NoChange: true}, nil
	}
	return s.replace(ctx, existing, req.WorkflowConfigurationID, wf)
}

// create / replace run Put's write path outside the idempotency transaction, for
// direct callers of Put.  They each open their own transaction so audit and the
// binding write commit atomically.
func (s *Service) create(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, wfID uuid.UUID, wf ReadWorkflowConfiguration) (PutResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PutResult{}, err
	}
	defer tx.Rollback(ctx)
	res, err := s.createTx(ctx, tx, NewPostgresRepositoryTx(tx), projectID, stage, wfID, wf)
	if err != nil {
		return PutResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PutResult{}, err
	}
	return res, nil
}

// ErrBindingAlreadyExists must surface from createTx for a duplicate INSERT so
// the HTTP layer maps it to 409 binding_already_exists.  The UNIQUE(project_id,
// stage) constraint violation is converted in repository.Create.
var _ = ErrBindingAlreadyExists

func (s *Service) replace(ctx context.Context, existing ProjectWorkflowBinding, wfID uuid.UUID, wf ReadWorkflowConfiguration) (PutResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PutResult{}, err
	}
	defer tx.Rollback(ctx)
	res, err := s.replaceTx(ctx, tx, NewPostgresRepositoryTx(tx), existing, wfID, wf)
	if err != nil {
		return PutResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PutResult{}, err
	}
	return res, nil
}

// PutWithIdempotency wraps Put with the operation-scoped idempotency guarantee.
// The whole request runs in a single transaction guarded by a PostgreSQL
// advisory transaction lock: acquire lock -> read idempotency record -> replay
// or execute -> audit -> write idempotency record -> commit.  Replay returns
// the first response and status code verbatim and performs no business writes.
func (s *Service) PutWithIdempotency(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, req PutRequest, key string) (PutResult, int, error) {
	if strings.TrimSpace(key) == "" || len(key) > 128 {
		return PutResult{}, 0, ErrValidation
	}
	scope := putScope(s.actorID, projectID, stage)
	hash := putFingerprint(s.actorID, projectID, stage, req)
	var result PutResult
	var status int
	body, cachedStatus, err := s.idempotent(ctx, scope, key, hash, func(tx pgx.Tx) ([]byte, int, error) {
		res, err := s.putTx(ctx, tx, projectID, stage, req)
		if err != nil {
			return nil, 0, err
		}
		result = res
		status = 200
		if res.Created {
			status = 201
		}
		payload := stageDTO(StageRead{Stage: res.Stage, Bound: true, Binding: &res.Binding, WorkflowConfigurationSummary: &res.Summary})
		body, mErr := marshalJSON(payload)
		return body, status, mErr
	})
	if err != nil {
		return PutResult{}, 0, err
	}
	if body != nil {
		// Cache replay: reconstruct the result/status from the cached response
		// so the second caller observes the same binding id, version, summary,
		// and status code as the original request.  No business write occurred.
		var dto WorkflowBindingStageDTO
		if uerr := unmarshalJSON(body, &dto); uerr != nil {
			return PutResult{}, 0, uerr
		}
		result, _ = putResultFromDTO(dto)
		status = cachedStatus
	}
	return result, status, nil
}

// DeleteWithIdempotency wraps Delete with the operation-scoped idempotency
// guarantee using the same transactional advisory-lock pattern as Put.
func (s *Service) DeleteWithIdempotency(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, req DeleteRequest, key string) (UnbindResult, int, error) {
	if strings.TrimSpace(key) == "" || len(key) > 128 {
		return UnbindResult{}, 0, ErrValidation
	}
	scope := deleteScope(s.actorID, projectID, stage)
	hash := deleteFingerprint(s.actorID, projectID, stage, req)
	var result UnbindResult
	body, _, err := s.idempotent(ctx, scope, key, hash, func(tx pgx.Tx) ([]byte, int, error) {
		res, err := s.deleteTx(ctx, tx, projectID, stage, req)
		if err != nil {
			return nil, 0, err
		}
		result = res
		payload := unbindDTO(res)
		body, mErr := marshalJSON(payload)
		return body, 200, mErr
	})
	if err != nil {
		return UnbindResult{}, 0, err
	}
	status := 200
	if body != nil {
		var dto UnbindResultDTO
		if uerr := unmarshalJSON(body, &dto); uerr != nil {
			return UnbindResult{}, 0, uerr
		}
		result = UnbindResult{ProjectID: dto.ProjectID, Stage: WorkflowBindingStage(dto.Stage), Unbound: dto.Unbound, WorkflowConfigurationRetained: dto.WorkflowConfigurationRetained}
	}
	return result, status, nil
}

func (s *Service) putTx(ctx context.Context, tx pgx.Tx, projectID uuid.UUID, stage WorkflowBindingStage, req PutRequest) (PutResult, error) {
	if err := s.projects.ExistsForModify(ctx, projectID); err != nil {
		return PutResult{}, err
	}
	if req.WorkflowConfigurationID == uuid.Nil {
		return PutResult{}, ErrValidation
	}
	if req.invalidExpectedVersion() {
		return PutResult{}, ErrValidation
	}
	wf, err := s.workflows.GetWorkflow(ctx, req.WorkflowConfigurationID)
	if err != nil {
		return PutResult{}, err
	}
	if !stageApplicable(stage, wf.ApplicableStages) {
		return PutResult{}, ErrNotApplicable
	}
	repo := NewPostgresRepositoryTx(tx)
	existing, err := repo.GetByProjectAndStage(ctx, projectID, stage)
	if errors.Is(err, ErrNotFound) {
		if req.ExpectedVersion != nil {
			return PutResult{}, ErrValidation
		}
		return s.createTx(ctx, tx, repo, projectID, stage, req.WorkflowConfigurationID, wf)
	}
	if err != nil {
		return PutResult{}, err
	}
	if req.ExpectedVersion == nil {
		return PutResult{}, &VersionConflictError{ProjectID: projectID, Stage: stage, ExpectedVersion: 0, CurrentVersion: existing.Version, Missing: true}
	}
	if *req.ExpectedVersion != existing.Version {
		return PutResult{}, &VersionConflictError{ProjectID: projectID, Stage: stage, ExpectedVersion: *req.ExpectedVersion, CurrentVersion: existing.Version}
	}
	if existing.WorkflowConfigurationID == req.WorkflowConfigurationID {
		// No-op: same workflow and correct expectedVersion.  No DB write, no
		// audit.  Return the unchanged current binding with the full summary.
		return PutResult{Stage: stage, Binding: existing, Summary: wf, NoChange: true}, nil
	}
	return s.replaceTx(ctx, tx, repo, existing, req.WorkflowConfigurationID, wf)
}

func (s *Service) createTx(ctx context.Context, tx pgx.Tx, repo *Repository, projectID uuid.UUID, stage WorkflowBindingStage, wfID uuid.UUID, wf ReadWorkflowConfiguration) (PutResult, error) {
	b, err := New(uuid.New(), projectID, wfID, stage)
	if err != nil {
		return PutResult{}, err
	}
	if s.beforeCreate != nil {
		s.beforeCreate()
	}
	created, err := repo.Create(ctx, b)
	if err != nil {
		return PutResult{}, err
	}
	if err := s.audit(ctx, tx, "project_workflow_binding.create", created.ID, map[string]any{
		"projectId":                projectID.String(),
		"stage":                    stage.String(),
		"bindingId":                created.ID.String(),
		"workflowConfigurationId": wfID.String(),
		"newVersion":               created.Version,
	}); err != nil {
		return PutResult{}, err
	}
	return PutResult{Stage: stage, Binding: created, Summary: wf, Created: true}, nil
}

func (s *Service) replaceTx(ctx context.Context, tx pgx.Tx, repo *Repository, existing ProjectWorkflowBinding, wfID uuid.UUID, wf ReadWorkflowConfiguration) (PutResult, error) {
	updated, err := repo.Replace(ctx, existing.ProjectID, existing.Stage, existing.Version, wfID)
	if err != nil {
		return PutResult{}, err
	}
	if err := s.audit(ctx, tx, "project_workflow_binding.replace", updated.ID, map[string]any{
		"projectId":                   existing.ProjectID.String(),
		"stage":                       existing.Stage.String(),
		"bindingId":                   updated.ID.String(),
		"oldWorkflowConfigurationId": existing.WorkflowConfigurationID.String(),
		"newWorkflowConfigurationId": wfID.String(),
		"oldVersion":                  existing.Version,
		"newVersion":                  updated.Version,
	}); err != nil {
		return PutResult{}, err
	}
	return PutResult{Stage: existing.Stage, Binding: updated, Summary: wf}, nil
}

// DeleteRequest carries the DELETE query parameter.
type DeleteRequest struct {
	ExpectedVersion int
}

func (s *Service) deleteTx(ctx context.Context, tx pgx.Tx, projectID uuid.UUID, stage WorkflowBindingStage, req DeleteRequest) (UnbindResult, error) {
	if err := s.projects.ExistsForModify(ctx, projectID); err != nil {
		return UnbindResult{}, err
	}
	repo := NewPostgresRepositoryTx(tx)
	removed, err := repo.Delete(ctx, projectID, stage, req.ExpectedVersion)
	if errors.Is(err, ErrNotFound) {
		return UnbindResult{}, ErrNotFound
	}
	if err != nil {
		return UnbindResult{}, err
	}
	if err := s.audit(ctx, tx, "project_workflow_binding.remove", removed.ID, map[string]any{
		"projectId":                   projectID.String(),
		"stage":                       stage.String(),
		"bindingId":                   removed.ID.String(),
		"oldWorkflowConfigurationId": removed.WorkflowConfigurationID.String(),
		"oldVersion":                  removed.Version,
	}); err != nil {
		return UnbindResult{}, err
	}
	return UnbindResult{ProjectID: projectID, Stage: stage, Unbound: true, WorkflowConfigurationRetained: true}, nil
}

// Delete unbinds a stage outside the idempotency transaction.  It is retained
// for direct service callers (tests, internal tooling); the HTTP path always
// goes through DeleteWithIdempotency.
func (s *Service) Delete(ctx context.Context, projectID uuid.UUID, stage WorkflowBindingStage, req DeleteRequest) (UnbindResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return UnbindResult{}, err
	}
	defer tx.Rollback(ctx)
	result, err := s.deleteTx(ctx, tx, projectID, stage, req)
	if err != nil {
		return UnbindResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return UnbindResult{}, err
	}
	return result, nil
}

// idempotent runs fn inside a single transaction guarded by an operation-scoped
// advisory lock.  On a same-key match it returns the cached response body and
// the cached HTTP status without invoking fn.  fn receives the transaction and
// must perform all business writes, audit, and return the serialized response
// body plus the HTTP status to cache.  The idempotency record is written in the
// same transaction so a business/audit failure rolls back the cached response
// too.  The returned []byte is non-nil only on a cache hit (the replayed first
// response, with its original status code in status); on a fresh execution it
// is nil and status is 0 because the caller already has the in-memory result
// and the status it computed inside fn.
func (s *Service) idempotent(ctx context.Context, scope, key, hash string, fn func(pgx.Tx) ([]byte, int, error)) ([]byte, int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", scope+":"+key); err != nil {
		return nil, 0, fmt.Errorf("lock idempotency request: %w", err)
	}
	if record, err := idempotency.NewPostgresRepositoryTx(tx).Get(ctx, scope, key); err == nil {
		if record.RequestHash != hash {
			return nil, 0, ErrIdempotencyReused
		}
		return record.ResponseBody, record.ResponseStatus, nil
	} else if !errors.Is(err, idempotency.ErrNotFound) {
		return nil, 0, err
	}
	body, status, err := fn(tx)
	if err != nil {
		return nil, 0, err
	}
	if _, err := idempotency.NewPostgresRepositoryTx(tx).Create(ctx, idempotency.Record{ID: uuid.New(), Scope: scope, Key: key, RequestHash: hash, ResponseStatus: status, ResponseBody: body}); err != nil {
		if errors.Is(err, idempotency.ErrConflict) {
			return nil, 0, ErrIdempotencyReused
		}
		return nil, 0, fmt.Errorf("create idempotency record: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, 0, err
	}
	return nil, 0, nil
}

func (s *Service) audit(ctx context.Context, tx pgx.Tx, action string, subjectID uuid.UUID, payload map[string]any) error {
	b, err := marshalJSON(payload)
	if err != nil {
		return err
	}
	return audit.NewRepository(tx).Insert(ctx, audit.Entry{ID: uuid.New(), ActorID: s.actorID, Action: action, SubjectType: "project_workflow_binding", SubjectID: subjectID.String(), Payload: b})
}

// putResultFromDTO rebuilds a PutResult (and infers the cached status code)
// from a replayed WorkflowBindingStageDTO so a cache hit returns the same
// binding id, version, summary and status as the original request.  A bound
// stage with a binding is treated as 201 only when the caller can prove it was
// a create; the cached status is encoded in the idempotency record's
// response_status column instead, so this helper focuses on the data shape and
// leaves status to the caller.  Callers use the cached status from the record.
func putResultFromDTO(dto WorkflowBindingStageDTO) (PutResult, int) {
	result := PutResult{Stage: WorkflowBindingStage(dto.Stage)}
	if dto.Binding != nil {
		result.Binding = ProjectWorkflowBinding{
			ID:                      dto.Binding.ID,
			ProjectID:               dto.Binding.ProjectID,
			Stage:                   WorkflowBindingStage(dto.Binding.Stage),
			WorkflowConfigurationID: dto.Binding.WorkflowConfigurationID,
			Version:                 dto.Binding.Version,
			CreatedAt:               dto.Binding.CreatedAt,
			UpdatedAt:               dto.Binding.UpdatedAt,
		}
	}
	if dto.WorkflowConfigurationSummary != nil {
		result.Summary = ReadWorkflowConfiguration{
			ID:                    dto.WorkflowConfigurationSummary.ID,
			Name:                  dto.WorkflowConfigurationSummary.Name,
			ConnectionID:          dto.WorkflowConfigurationSummary.ConnectionID,
			ConnectionName:        dto.WorkflowConfigurationSummary.ConnectionName,
			ConnectionType:        dto.WorkflowConfigurationSummary.ConnectionType,
			WorkflowType:          dto.WorkflowConfigurationSummary.WorkflowType,
			ApplicableStages:      dto.WorkflowConfigurationSummary.ApplicableStages,
			TypeConfig:            dto.WorkflowConfigurationSummary.TypeConfig,
			InputContractVersion:  dto.WorkflowConfigurationSummary.InputContractVersion,
			OutputContractVersion: dto.WorkflowConfigurationSummary.OutputContractVersion,
			DefaultParameters:     dto.WorkflowConfigurationSummary.DefaultParameters,
			Note:                  dto.WorkflowConfigurationSummary.Note,
			IntegrationStatus:     dto.WorkflowConfigurationSummary.IntegrationStatus,
			Enabled:               dto.WorkflowConfigurationSummary.Enabled,
			LastVerifiedAt:        dto.WorkflowConfigurationSummary.LastVerifiedAt,
			LastErrorCode:         dto.WorkflowConfigurationSummary.LastErrorCode,
			LastErrorMessage:      dto.WorkflowConfigurationSummary.LastErrorMessage,
			Version:               dto.WorkflowConfigurationSummary.Version,
			CreatedAt:             dto.WorkflowConfigurationSummary.CreatedAt,
			UpdatedAt:             dto.WorkflowConfigurationSummary.UpdatedAt,
		}
	}
	return result, 0
}

func stageApplicable(stage WorkflowBindingStage, stages []string) bool {
	for _, s := range stages {
		if s == stage.String() {
			return true
		}
	}
	return false
}

// putScope isolates an idempotent operation by actor, HTTP method, projectId,
// and stage.  Even though the current platform runs in single-user system mode,
// the actor dimension is kept so a future multi-user release cannot reuse one
// actor's idempotency scope for another.
func putScope(actor string, projectID uuid.UUID, stage WorkflowBindingStage) string {
	return "project_workflow_binding:put:" + actor + ":" + projectID.String() + ":" + stage.String()
}

func deleteScope(actor string, projectID uuid.UUID, stage WorkflowBindingStage) string {
	return "project_workflow_binding:delete:" + actor + ":" + projectID.String() + ":" + stage.String()
}

// putFingerprint covers actor, HTTP method, projectId, stage,
// workflowConfigurationId, expectedVersion and operation type so a reused key
// with any differing request facet is rejected.
func putFingerprint(actor string, projectID uuid.UUID, stage WorkflowBindingStage, req PutRequest) string {
	return safeHashJSON(struct {
		Actor                    string     `json:"actor"`
		Method                   string     `json:"method"`
		ProjectID                uuid.UUID  `json:"projectId"`
		Stage                    string     `json:"stage"`
		WorkflowConfigurationID  uuid.UUID  `json:"workflowConfigurationId"`
		ExpectedVersion          *int       `json:"expectedVersion"`
		Operation                string     `json:"operation"`
	}{actor, "PUT", projectID, stage.String(), req.WorkflowConfigurationID, req.ExpectedVersion, "bind"})
}

func deleteFingerprint(actor string, projectID uuid.UUID, stage WorkflowBindingStage, req DeleteRequest) string {
	return safeHashJSON(struct {
		Actor           string    `json:"actor"`
		Method          string    `json:"method"`
		ProjectID       uuid.UUID `json:"projectId"`
		Stage           string    `json:"stage"`
		ExpectedVersion int       `json:"expectedVersion"`
		Operation       string    `json:"operation"`
	}{actor, "DELETE", projectID, stage.String(), req.ExpectedVersion, "unbind"})
}

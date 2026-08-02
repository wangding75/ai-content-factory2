package contentitem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

var (
	ErrGenerationNotConfigured = errors.New("content generation workflow is not configured")
	ErrGenerationActiveRun     = errors.New("content generation run is already active")
	ErrGenerationInputChanged  = errors.New("content generation preflight input changed")
	ErrGenerationTokenInvalid  = errors.New("content generation preflight token is invalid")
	ErrGenerationTokenExpired  = errors.New("content generation preflight token expired")
	ErrGenerationTokenConsumed = workflowrun.ErrPreflightTokenConsumed
	ErrCandidateStale          = errors.New("content generation candidate is stale")
	ErrCandidateNotEligible    = errors.New("content generation candidate is not eligible")
	ErrCandidateItemMismatch   = errors.New("content generation candidate item mismatch")
	ErrRunNotConsumable        = errors.New("content generation run is not consumable")
	ErrGenerationSourceInvalid = errors.New("content generation source version is invalid")
)

type GenerationContextOptions struct{ IncludePriorChapterSummaries, IncludeProjectMaterials, IncludeStoryContext, IncludeForeshadowings bool }
type GenerationPreflightRequest struct {
	ExpectedCurrentVersionID uuid.UUID
	ExpectedCurrentVersion   int
	ContextOptions           GenerationContextOptions
	AdditionalInstructions   *string
	ActorID                  string
}
type GenerationCheck struct{ Code, Status, Message string }
type GenerationWorkflowSummary struct {
	ID                                                uuid.UUID
	Name, InputContractVersion, OutputContractVersion string
	Version                                           int
}
type GenerationContextSummary struct{ ChapterGoalCount, KeyPlotCount, PriorChapterCount, MaterialCount, StorylineCount, ForeshadowingCount int }
type GenerationPreflightResult struct {
	Passed          bool
	Token           string
	ExpiresAt       time.Time
	SourceVersion   ContentVersion
	TargetVersionNo int
	Workflow        *GenerationWorkflowSummary
	ContextSummary  GenerationContextSummary
	ContextSnapshot json.RawMessage
	Checks          []GenerationCheck
}
type GenerationAllowedActions struct{ CanPreflight, CanStartGeneration, CanCancelRuntime, CanRetryRuntime, CanRetryResultConsumption, CanViewCandidate, CanSetCandidateCurrent bool }
type GenerationSummary struct {
	Detail                                 Detail
	WorkflowConfigured                     bool
	State                                  string
	ActiveRun, LatestRun                   *workflowrun.WorkflowRun
	LatestEvents                           []workflowrun.Event
	LatestCandidate                        *ContentVersion
	LatestError                            *GenerationSafeError
	CanGenerate, CandidateCanBecomeCurrent bool
	AllowedActions                         GenerationAllowedActions
}
type GenerationSafeError struct {
	Code, Message string
	Details       map[string]any
}
type ContentGenerationResult struct {
	ContentItem      ContentItem
	CandidateVersion ContentVersion
	WorkflowRun      workflowrun.WorkflowRun
}
type RetryConsumptionRequest struct {
	ExpectedRunVersion int
	IdempotencyKey     string
}
type SetCurrentRequest struct {
	CandidateVersionID, ExpectedCurrentVersionID uuid.UUID
	ExpectedCurrentVersion                       int
	IdempotencyKey                               string
}

type GenerationService struct {
	repo     *PostgresRepository
	bindings interface {
		GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error)
	}
	configs interface {
		GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error)
		GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error)
	}
	runs interface {
		CreateRunIdempotentForScope(context.Context, string, uuid.UUID, string, string, workflowrun.CreateRunPreparation) (workflowrun.WorkflowRun, error)
		CreateRunForPreflightToken(context.Context, uuid.UUID, string, string, workflowrun.CreateRunPreparation) (workflowrun.WorkflowRun, error)
		CreateRunForPreflightTokenIdempotent(context.Context, uuid.UUID, string, string, string, workflowrun.CreateRunTxPreparation) (workflowrun.WorkflowRun, error)
		ListRuns(context.Context, workflowrun.ListRunsQuery) (workflowrun.RunList, error)
		ListRunEvents(context.Context, uuid.UUID) ([]workflowrun.Event, error)
		GetRun(context.Context, uuid.UUID) (workflowrun.WorkflowRun, error)
		AddEvent(context.Context, workflowrun.Event) (workflowrun.Event, error)
	}
	secret []byte
	now    func() time.Time
	begin  func(context.Context) (pgx.Tx, error)
}

func NewGenerationService(repo *PostgresRepository, bindings interface {
	GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error)
}, configs interface {
	GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error)
	GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error)
}, runs interface {
	CreateRunIdempotentForScope(context.Context, string, uuid.UUID, string, string, workflowrun.CreateRunPreparation) (workflowrun.WorkflowRun, error)
	CreateRunForPreflightToken(context.Context, uuid.UUID, string, string, workflowrun.CreateRunPreparation) (workflowrun.WorkflowRun, error)
	CreateRunForPreflightTokenIdempotent(context.Context, uuid.UUID, string, string, string, workflowrun.CreateRunTxPreparation) (workflowrun.WorkflowRun, error)
	ListRuns(context.Context, workflowrun.ListRunsQuery) (workflowrun.RunList, error)
	ListRunEvents(context.Context, uuid.UUID) ([]workflowrun.Event, error)
	GetRun(context.Context, uuid.UUID) (workflowrun.WorkflowRun, error)
	AddEvent(context.Context, workflowrun.Event) (workflowrun.Event, error)
}, secret string) *GenerationService {
	return &GenerationService{repo: repo, bindings: bindings, configs: configs, runs: runs, secret: []byte(secret), now: time.Now, begin: repo.db.Begin}
}

func generationDigest(item Detail, request GenerationPreflightRequest, binding workflowbinding.ProjectWorkflowBinding, workflow globalconfig.Workflow, connection globalconfig.Connection, contextSnapshot json.RawMessage) string {
	raw, _ := json.Marshal(struct {
		ItemID, VersionID    uuid.UUID
		Version, ItemVersion int
		Context              GenerationContextOptions
		Instructions         *string
		BindingID            uuid.UUID
		BindingVersion       int
		WorkflowID           uuid.UUID
		WorkflowVersion      int
		ConnectionID         uuid.UUID
		ConnectionVersion    int
		Snapshot             json.RawMessage
	}{item.Item.ID, item.CurrentVersion.ID, item.CurrentVersion.Version, item.Item.Version, request.ContextOptions, request.AdditionalInstructions, binding.ID, binding.Version, workflow.ID, workflow.Version, connection.ID, connection.Version, contextSnapshot})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func (s *GenerationService) runnable(ctx context.Context, projectID uuid.UUID) (workflowbinding.ProjectWorkflowBinding, globalconfig.Workflow, globalconfig.Connection, error) {
	binding, err := s.bindings.GetByProjectAndStage(ctx, projectID, workflowbinding.StageContentGeneration)
	if errors.Is(err, workflowbinding.ErrNotFound) {
		return workflowbinding.ProjectWorkflowBinding{}, globalconfig.Workflow{}, globalconfig.Connection{}, ErrGenerationNotConfigured
	}
	if err != nil {
		return workflowbinding.ProjectWorkflowBinding{}, globalconfig.Workflow{}, globalconfig.Connection{}, err
	}
	workflow, err := s.configs.GetWorkflow(ctx, binding.WorkflowConfigurationID)
	if errors.Is(err, globalconfig.ErrNotFound) {
		return binding, globalconfig.Workflow{}, globalconfig.Connection{}, ErrGenerationNotConfigured
	}
	if err != nil {
		return binding, globalconfig.Workflow{}, globalconfig.Connection{}, err
	}
	if !workflow.Enabled {
		return binding, globalconfig.Workflow{}, globalconfig.Connection{}, ErrGenerationNotConfigured
	}
	connection, err := s.configs.GetConnection(ctx, workflow.ConnectionID)
	if errors.Is(err, globalconfig.ErrNotFound) {
		return binding, workflow, globalconfig.Connection{}, ErrGenerationNotConfigured
	}
	if err != nil {
		return binding, workflow, globalconfig.Connection{}, err
	}
	if !connection.Enabled || connection.IntegrationStatus != "verified" {
		return binding, workflow, connection, ErrGenerationNotConfigured
	}
	return binding, workflow, connection, nil
}
func (s *GenerationService) active(ctx context.Context, item Detail) (*workflowrun.WorkflowRun, error) {
	kind := "content_item"
	list, err := s.runs.ListRuns(ctx, workflowrun.ListRunsQuery{ListFilter: workflowrun.ListFilter{ProjectID: &item.Item.ProjectID, Stage: "content_generation", SubjectType: &kind, SubjectID: &item.Item.ID, Limit: 20}})
	if err != nil {
		return nil, err
	}
	for i := range list.Items {
		if list.Items[i].Status == workflowrun.StatusQueued || list.Items[i].Status == workflowrun.StatusRunning {
			return &list.Items[i], nil
		}
	}
	return nil, nil
}
func validGenerationInput(r GenerationPreflightRequest) bool {
	return r.ExpectedCurrentVersionID != uuid.Nil && r.ExpectedCurrentVersion >= 1 && strings.TrimSpace(r.ActorID) != "" && (r.AdditionalInstructions == nil || len([]rune(strings.TrimSpace(*r.AdditionalInstructions))) <= 2000)
}

// buildGenerationContext reads only rows in the ContentItem's project and keeps
// the persisted Run input deterministic. Optional sources are not queried when
// their corresponding option is disabled.
type generationContextQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (s *GenerationService) buildGenerationContext(ctx context.Context, q generationContextQueryer, detail Detail, options GenerationContextOptions, instructions *string) (json.RawMessage, GenerationContextSummary, error) {
	type textItem struct {
		Title   string `json:"title"`
		Summary string `json:"summary"`
	}
	type sourceVersion struct {
		ID      uuid.UUID `json:"id"`
		Version int       `json:"version"`
		Title   string    `json:"title"`
		Content string    `json:"content"`
		Summary *string   `json:"summary"`
	}
	summary := GenerationContextSummary{ChapterGoalCount: 0}
	var chapterNo int
	var chapterGoal string
	if err := q.QueryRow(ctx, "SELECT chapter_no, COALESCE(chapter_goal,'') FROM chapter_plans WHERE id=$1 AND project_id=$2 FOR SHARE", detail.Item.ChapterPlanID, detail.Item.ProjectID).Scan(&chapterNo, &chapterGoal); err != nil {
		return nil, summary, err
	}
	if strings.TrimSpace(chapterGoal) != "" {
		summary.ChapterGoalCount = 1
	}
	contextValue := struct {
		SourceContentVersion   sourceVersion `json:"sourceContentVersion"`
		ChapterGoal            string        `json:"chapterGoal"`
		PriorChapterSummaries  []textItem    `json:"priorChapterSummaries,omitempty"`
		ProjectMaterials       []textItem    `json:"projectMaterials,omitempty"`
		StoryContext           []textItem    `json:"storyContext,omitempty"`
		Foreshadowings         []textItem    `json:"foreshadowings,omitempty"`
		AdditionalInstructions *string       `json:"additionalInstructions"`
	}{SourceContentVersion: sourceVersion{detail.CurrentVersion.ID, detail.CurrentVersion.Version, detail.CurrentVersion.Title, detail.CurrentVersion.Content, detail.CurrentVersion.Summary}, ChapterGoal: chapterGoal, AdditionalInstructions: instructions}
	if options.IncludePriorChapterSummaries {
		rows, err := q.Query(ctx, "SELECT v.title, COALESCE(v.summary,'') FROM content_items i JOIN chapter_plans p ON p.id=i.chapter_plan_id AND p.project_id=i.project_id JOIN content_versions v ON v.id=i.current_version_id AND v.content_item_id=i.id WHERE i.project_id=$1 AND p.chapter_no<$2 ORDER BY p.chapter_no ASC,i.id ASC LIMIT 20 FOR SHARE OF i,p,v", detail.Item.ProjectID, chapterNo)
		if err != nil {
			return nil, summary, err
		}
		defer rows.Close()
		for rows.Next() {
			var value textItem
			if err := rows.Scan(&value.Title, &value.Summary); err != nil {
				return nil, summary, err
			}
			contextValue.PriorChapterSummaries = append(contextValue.PriorChapterSummaries, value)
		}
		if err := rows.Err(); err != nil {
			return nil, summary, err
		}
		summary.PriorChapterCount = len(contextValue.PriorChapterSummaries)
	}
	if options.IncludeProjectMaterials {
		rows, err := q.Query(ctx, "SELECT m.name,m.summary FROM chapter_plan_materials x JOIN materials m ON m.id=x.material_id WHERE x.chapter_plan_id=$1 AND x.project_id=$2 ORDER BY x.position ASC,m.id ASC LIMIT 50 FOR SHARE OF x,m", detail.Item.ChapterPlanID, detail.Item.ProjectID)
		if err != nil {
			return nil, summary, err
		}
		defer rows.Close()
		for rows.Next() {
			var value textItem
			if err := rows.Scan(&value.Title, &value.Summary); err != nil {
				return nil, summary, err
			}
			contextValue.ProjectMaterials = append(contextValue.ProjectMaterials, value)
		}
		if err := rows.Err(); err != nil {
			return nil, summary, err
		}
		summary.MaterialCount = len(contextValue.ProjectMaterials)
	}
	if options.IncludeStoryContext {
		rows, err := q.Query(ctx, "SELECT name,summary FROM storylines WHERE project_id=$1 AND status='active' ORDER BY sort_order ASC,id ASC LIMIT 50 FOR SHARE", detail.Item.ProjectID)
		if err != nil {
			return nil, summary, err
		}
		defer rows.Close()
		for rows.Next() {
			var value textItem
			if err := rows.Scan(&value.Title, &value.Summary); err != nil {
				return nil, summary, err
			}
			contextValue.StoryContext = append(contextValue.StoryContext, value)
		}
		if err := rows.Err(); err != nil {
			return nil, summary, err
		}
		summary.StorylineCount = len(contextValue.StoryContext)
		summary.KeyPlotCount = summary.StorylineCount
	}
	if options.IncludeForeshadowings {
		rows, err := q.Query(ctx, "SELECT f.title,f.description FROM chapter_plan_foreshadowings x JOIN foreshadowings f ON f.id=x.foreshadowing_id AND f.project_id=x.project_id WHERE x.chapter_plan_id=$1 AND x.project_id=$2 ORDER BY x.position ASC,f.id ASC LIMIT 50 FOR SHARE OF x,f", detail.Item.ChapterPlanID, detail.Item.ProjectID)
		if err != nil {
			return nil, summary, err
		}
		defer rows.Close()
		for rows.Next() {
			var value textItem
			if err := rows.Scan(&value.Title, &value.Summary); err != nil {
				return nil, summary, err
			}
			contextValue.Foreshadowings = append(contextValue.Foreshadowings, value)
		}
		if err := rows.Err(); err != nil {
			return nil, summary, err
		}
		summary.ForeshadowingCount = len(contextValue.Foreshadowings)
	}
	snapshot, err := json.Marshal(contextValue)
	return snapshot, summary, err
}
func (s *GenerationService) Preflight(ctx context.Context, itemID uuid.UUID, request GenerationPreflightRequest) (GenerationPreflightResult, error) {
	if !validGenerationInput(request) {
		return GenerationPreflightResult{}, ErrValidation
	}
	detail, err := s.repo.GetByID(ctx, itemID)
	if err != nil {
		return GenerationPreflightResult{}, err
	}
	targetVersionNo, err := nextContentVersionNo(ctx, s.repo.db, detail.Item.ID)
	if err != nil {
		return GenerationPreflightResult{}, err
	}
	result := GenerationPreflightResult{SourceVersion: detail.CurrentVersion, TargetVersionNo: targetVersionNo}
	contextSnapshot, contextSummary, err := s.buildGenerationContext(ctx, s.repo.db, detail, request.ContextOptions, request.AdditionalInstructions)
	if err != nil {
		return result, err
	}
	result.ContextSnapshot, result.ContextSummary = contextSnapshot, contextSummary
	checks := func(code, status, message string) {
		result.Checks = append(result.Checks, GenerationCheck{code, status, message})
	}
	var chapterStatus string
	if err = s.repo.db.QueryRow(ctx, "SELECT status FROM chapter_plans WHERE id=$1 AND project_id=$2", detail.Item.ChapterPlanID, detail.Item.ProjectID).Scan(&chapterStatus); errors.Is(err, pgx.ErrNoRows) || chapterStatus != "confirmed" {
		checks("chapter_plan_confirmed", "blocked", "章节计划尚未确认")
		return result, nil
	} else if err != nil {
		return result, err
	}
	checks("chapter_plan_confirmed", "passed", "章节计划已确认")
	if detail.CurrentVersion.ID != request.ExpectedCurrentVersionID || detail.CurrentVersion.Version != request.ExpectedCurrentVersion {
		checks("current_version_unchanged", "blocked", "当前正文版本已变化")
		return result, nil
	}
	checks("current_version_unchanged", "passed", "当前正文版本未变化")
	binding, workflow, connection, err := s.runnable(ctx, detail.Item.ProjectID)
	if errors.Is(err, ErrGenerationNotConfigured) {
		checks("project_binding_available", "blocked", "尚未配置正文生成工作流")
		checks("execution_integration_available", "blocked", "工作流执行连接不可用")
		return result, nil
	} else if err != nil {
		return result, err
	}
	result.Workflow = &GenerationWorkflowSummary{workflow.ID, workflow.Name, workflow.InputContractVersion, workflow.OutputContractVersion, workflow.Version}
	checks("project_binding_available", "passed", "项目工作流绑定可用")
	checks("execution_integration_available", "passed", "工作流执行连接可用")
	active, err := s.active(ctx, detail)
	if err != nil {
		return result, err
	}
	if active != nil {
		checks("active_run_absent", "blocked", "当前正文已有运行中的生成任务")
		return result, nil
	}
	checks("active_run_absent", "passed", "当前正文没有活跃生成任务")
	checks("generation_input_valid", "passed", "生成输入有效")
	contextOptions, _ := json.Marshal(request.ContextOptions)
	digest := generationDigest(detail, request, binding, workflow, connection, contextSnapshot)
	now := s.now().UTC()
	claims := chapterplan.PreflightTokenClaims{ProjectID: detail.Item.ProjectID, ActorID: request.ActorID, Stage: "content_generation", ContextOptions: contextOptions, AdditionalInstructions: request.AdditionalInstructions, InputDigest: digest, BindingID: binding.ID, BindingVersion: binding.Version, IssuedAt: now.Unix(), ExpiresAt: now.Add(10 * time.Minute).Unix(), Nonce: uuid.NewString()}
	token, err := chapterplan.SignPreflightToken(s.secret, claims)
	if err != nil {
		return result, err
	}
	result.Passed, result.Token, result.ExpiresAt = true, token, time.Unix(claims.ExpiresAt, 0).UTC()
	return result, nil
}

func generationDetailForCreate(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) (Detail, error) {
	item, err := scanItem(tx.QueryRow(ctx, "SELECT "+itemColumns+" FROM content_items WHERE id=$1 FOR UPDATE", itemID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrContentItemNotFound
	}
	if err != nil {
		return Detail{}, err
	}
	version, err := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=$1 AND content_item_id=$2 FOR SHARE", item.CurrentVersionID, item.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrGenerationInputChanged
	}
	if err != nil {
		return Detail{}, err
	}
	return Detail{Item: item, CurrentVersion: version}, nil
}

func runnableGenerationForCreate(ctx context.Context, tx pgx.Tx, projectID uuid.UUID) (workflowbinding.ProjectWorkflowBinding, globalconfig.Workflow, globalconfig.Connection, error) {
	var id, bindingProjectID, configurationID uuid.UUID
	var stage workflowbinding.WorkflowBindingStage
	var version int
	var createdAt, updatedAt time.Time
	err := tx.QueryRow(ctx, "SELECT id,project_id,stage,workflow_configuration_id,version,created_at,updated_at FROM project_workflow_bindings WHERE project_id=$1 AND stage='content_generation' FOR SHARE", projectID).Scan(&id, &bindingProjectID, &stage, &configurationID, &version, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return workflowbinding.ProjectWorkflowBinding{}, globalconfig.Workflow{}, globalconfig.Connection{}, ErrGenerationNotConfigured
	}
	if err != nil {
		return workflowbinding.ProjectWorkflowBinding{}, globalconfig.Workflow{}, globalconfig.Connection{}, err
	}
	binding, err := workflowbinding.NewFromDB(id, bindingProjectID, configurationID, stage, version, createdAt, updatedAt)
	if err != nil {
		return workflowbinding.ProjectWorkflowBinding{}, globalconfig.Workflow{}, globalconfig.Connection{}, err
	}
	workflow, err := globalconfig.GetWorkflowForShare(ctx, tx, binding.WorkflowConfigurationID)
	if errors.Is(err, globalconfig.ErrNotFound) {
		return binding, globalconfig.Workflow{}, globalconfig.Connection{}, ErrGenerationNotConfigured
	}
	if err != nil {
		return binding, globalconfig.Workflow{}, globalconfig.Connection{}, err
	}
	if !workflow.Enabled {
		return binding, workflow, globalconfig.Connection{}, ErrGenerationNotConfigured
	}
	connection, err := globalconfig.GetConnectionForShare(ctx, tx, workflow.ConnectionID)
	if errors.Is(err, globalconfig.ErrNotFound) {
		return binding, workflow, globalconfig.Connection{}, ErrGenerationNotConfigured
	}
	if err != nil {
		return binding, workflow, globalconfig.Connection{}, err
	}
	if !connection.Enabled || connection.IntegrationStatus != "verified" {
		return binding, workflow, connection, ErrGenerationNotConfigured
	}
	return binding, workflow, connection, nil
}

func (s *GenerationService) CreateRun(ctx context.Context, itemID uuid.UUID, actorID, token, key string) (workflowrun.WorkflowRun, error) {
	claims, err := chapterplan.VerifyPreflightToken(s.secret, token, s.now())
	if errors.Is(err, chapterplan.ErrPreflightTokenExpired) {
		return workflowrun.WorkflowRun{}, ErrGenerationTokenExpired
	}
	if err != nil || claims.Stage != "content_generation" {
		return workflowrun.WorkflowRun{}, ErrGenerationTokenInvalid
	}
	if strings.TrimSpace(key) == "" {
		return workflowrun.WorkflowRun{}, ErrValidation
	}
	tokenSum := sha256.Sum256([]byte(token))
	hash := workflowrun.Fingerprint(struct {
		ItemID                          uuid.UUID
		TokenDigest, Actor, InputDigest string
	}{itemID, hex.EncodeToString(tokenSum[:]), actorID, claims.InputDigest})
	return s.runs.CreateRunForPreflightTokenIdempotent(ctx, claims.ProjectID, key, hash, claims.Nonce, func(tx pgx.Tx) (workflowrun.CreateRunCommand, error) {
		request := GenerationPreflightRequest{ExpectedCurrentVersionID: uuid.Nil, ActorID: actorID}
		if json.Unmarshal(claims.ContextOptions, &request.ContextOptions) != nil {
			return workflowrun.CreateRunCommand{}, ErrGenerationTokenInvalid
		}
		detail, e := generationDetailForCreate(ctx, tx, itemID)
		if e != nil {
			return workflowrun.CreateRunCommand{}, e
		}
		if detail.Item.ProjectID != claims.ProjectID || claims.ActorID != actorID {
			return workflowrun.CreateRunCommand{}, ErrGenerationInputChanged
		}
		binding, workflow, connection, e := runnableGenerationForCreate(ctx, tx, detail.Item.ProjectID)
		if errors.Is(e, ErrGenerationNotConfigured) {
			return workflowrun.CreateRunCommand{}, ErrGenerationInputChanged
		}
		if e != nil {
			return workflowrun.CreateRunCommand{}, e
		}
		request.ExpectedCurrentVersionID, request.ExpectedCurrentVersion = detail.CurrentVersion.ID, detail.CurrentVersion.Version
		request.AdditionalInstructions = claims.AdditionalInstructions
		contextSnapshot, _, e := s.buildGenerationContext(ctx, tx, detail, request.ContextOptions, request.AdditionalInstructions)
		if e != nil {
			return workflowrun.CreateRunCommand{}, e
		}
		if generationDigest(detail, request, binding, workflow, connection, contextSnapshot) != claims.InputDigest || binding.ID != claims.BindingID || binding.Version != claims.BindingVersion {
			return workflowrun.CreateRunCommand{}, ErrGenerationInputChanged
		}
		if _, e = workflowrun.NewPostgresRepositoryTx(tx).FindActive(ctx, detail.Item.ProjectID, "content_generation", "content_item", detail.Item.ID); e == nil {
			return workflowrun.CreateRunCommand{}, ErrGenerationActiveRun
		} else if !errors.Is(e, workflowrun.ErrNotFound) {
			return workflowrun.CreateRunCommand{}, e
		}
		snapshot, e := workflowrun.BuildConfigurationSnapshot(binding, workflow, connection, s.now())
		if e != nil {
			return workflowrun.CreateRunCommand{}, e
		}
		subjectType, subjectID := "content_item", detail.Item.ID
		payload, _ := json.Marshal(map[string]any{"stage": "content_generation", "sourceContentVersionId": detail.CurrentVersion.ID, "sourceContentVersionVersion": detail.CurrentVersion.Version, "preflightTokenNonce": claims.Nonce, "additionalInstructions": claims.AdditionalInstructions, "contextOptions": request.ContextOptions, "generationContext": json.RawMessage(contextSnapshot)})
		return workflowrun.CreateRunCommand{ProjectID: detail.Item.ProjectID, Stage: "content_generation", SubjectType: &subjectType, SubjectID: &subjectID, InputPayload: payload, TriggerSource: "manual", PreparedConfiguration: &workflowrun.PreparedRunConfiguration{WorkflowConfigurationID: workflow.ID, Snapshot: snapshot}}, nil
	})
}
func (s *GenerationService) Summary(ctx context.Context, itemID uuid.UUID) (GenerationSummary, error) {
	detail, err := s.repo.GetByID(ctx, itemID)
	if err != nil {
		return GenerationSummary{}, err
	}
	out := GenerationSummary{Detail: detail, State: "idle", LatestEvents: []workflowrun.Event{}}
	_, _, _, configuredErr := s.runnable(ctx, detail.Item.ProjectID)
	if configuredErr != nil && !errors.Is(configuredErr, ErrGenerationNotConfigured) {
		return GenerationSummary{}, configuredErr
	}
	out.WorkflowConfigured = configuredErr == nil
	kind := "content_item"
	list, err := s.runs.ListRuns(ctx, workflowrun.ListRunsQuery{ListFilter: workflowrun.ListFilter{ProjectID: &detail.Item.ProjectID, Stage: "content_generation", SubjectType: &kind, SubjectID: &detail.Item.ID, Limit: 20}})
	if err != nil {
		return GenerationSummary{}, err
	}
	if len(list.Items) > 0 {
		out.LatestRun = &list.Items[0]
		for i := range list.Items {
			if list.Items[i].Status == workflowrun.StatusQueued || list.Items[i].Status == workflowrun.StatusRunning {
				out.ActiveRun = &list.Items[i]
				break
			}
		}
		out.LatestEvents, err = s.runs.ListRunEvents(ctx, out.LatestRun.ID)
		if err != nil {
			return GenerationSummary{}, err
		}
	}
	if out.LatestRun != nil {
		candidate, e := scanVersion(s.repo.db.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE content_item_id=$1 AND source_workflow_run_id=$2 AND id<>$3 LIMIT 1", detail.Item.ID, out.LatestRun.ID, detail.Item.CurrentVersionID))
		if e == nil {
			out.LatestCandidate = &candidate
			out.CandidateCanBecomeCurrent = candidate.SourceContentVersionID != nil && candidate.SourceContentVersionVersion != nil && *candidate.SourceContentVersionID == detail.CurrentVersion.ID && *candidate.SourceContentVersionVersion == detail.CurrentVersion.Version
		} else if !errors.Is(e, pgx.ErrNoRows) {
			return GenerationSummary{}, e
		}
	}
	if out.LatestRun != nil && out.LatestRun.Status == workflowrun.StatusQueued {
		out.State = "queued"
	} else if out.LatestRun != nil && out.LatestRun.Status == workflowrun.StatusRunning {
		out.State = "running"
	} else if out.LatestCandidate != nil {
		out.State = "candidate_ready"
	} else if out.LatestRun != nil {
		for _, event := range out.LatestEvents {
			if event.EventType == workflowrun.EventTypeResultConsumptionFailed {
				out.State = "result_consumption_failed"
				out.LatestError = generationEventError(event, out.State)
			}
			if event.EventType == workflowrun.EventTypeOutputValidationFailed {
				out.State = "output_validation_failed"
				out.LatestError = generationEventError(event, out.State)
			}
		}
		if out.State == "idle" && (out.LatestRun.Status == workflowrun.StatusFailed || out.LatestRun.Status == workflowrun.StatusCancelled) {
			out.State = "runtime_failed"
			out.LatestError = generationRunError(*out.LatestRun)
		}
	} else if !out.WorkflowConfigured {
		out.State = "not_configured"
	}
	out.CanGenerate = out.State == "idle" || (out.State == "candidate_ready" && !out.CandidateCanBecomeCurrent)
	out.AllowedActions = GenerationAllowedActions{CanPreflight: out.CanGenerate, CanStartGeneration: out.CanGenerate, CanCancelRuntime: out.ActiveRun != nil, CanRetryRuntime: out.State == "runtime_failed" || out.State == "output_validation_failed", CanRetryResultConsumption: out.State == "result_consumption_failed", CanViewCandidate: out.LatestCandidate != nil, CanSetCandidateCurrent: out.CandidateCanBecomeCurrent}
	return out, nil
}
func (s *GenerationService) ConsumeSucceededRun(ctx context.Context, run workflowrun.WorkflowRun) error {
	if run.Stage != "content_generation" || run.Status != workflowrun.StatusSucceeded {
		return ErrRunNotConsumable
	}
	_, err := s.consume(ctx, run)
	return err
}
func (s *GenerationService) ValidateResult(run workflowrun.WorkflowRun) error {
	_, err := decodeGenerationOutput(run.OutputPayload)
	return err
}
func (s *GenerationService) ConsumeResultTx(ctx context.Context, tx pgx.Tx, run workflowrun.WorkflowRun) error {
	_, err := s.consumeLocked(ctx, tx, run, uuid.Nil)
	return err
}
func (s *GenerationService) RetryConsumption(ctx context.Context, runID uuid.UUID, request RetryConsumptionRequest) (ContentGenerationResult, error) {
	if runID == uuid.Nil || request.ExpectedRunVersion < 1 || strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 128 {
		return ContentGenerationResult{}, ErrValidation
	}
	hash := generationCommandFingerprint(struct {
		RunID    uuid.UUID
		Expected int
	}{runID, request.ExpectedRunVersion})
	scope := "retryContentGenerationResultConsumption:" + runID.String()
	tx, e := s.begin(ctx)
	if e != nil {
		return ContentGenerationResult{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", scope+":"+request.IdempotencyKey); e != nil {
		return ContentGenerationResult{}, e
	}
	idem := idempotency.NewPostgresRepositoryTx(tx)
	if record, get := idem.Get(ctx, scope, request.IdempotencyKey); get == nil {
		if record.RequestHash != hash {
			return ContentGenerationResult{}, workflowrun.ErrIdempotencyConflict
		}
		var replay struct{ CandidateID uuid.UUID }
		if json.Unmarshal(record.ResponseBody, &replay) != nil {
			return ContentGenerationResult{}, ErrValidation
		}
		return s.generationResult(ctx, tx, runID, replay.CandidateID)
	} else if !errors.Is(get, idempotency.ErrNotFound) {
		return ContentGenerationResult{}, get
	}
	var version int
	e = tx.QueryRow(ctx, "SELECT version FROM workflow_run_records WHERE id=$1 FOR UPDATE", runID).Scan(&version)
	if errors.Is(e, pgx.ErrNoRows) {
		return ContentGenerationResult{}, workflowrun.ErrNotFound
	}
	if e != nil {
		return ContentGenerationResult{}, e
	}
	if version != request.ExpectedRunVersion {
		return ContentGenerationResult{}, workflowrun.ErrVersionConflict
	}
	run, e := workflowrun.NewPostgresRepositoryTx(tx).GetByID(ctx, runID)
	if e != nil {
		return ContentGenerationResult{}, e
	}
	if run.Status != workflowrun.StatusSucceeded && !(run.Status == workflowrun.StatusFailed && run.FailurePhase != nil && *run.FailurePhase == "result_consumption") {
		return ContentGenerationResult{}, ErrRunNotConsumable
	}
	candidate, e := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE source_workflow_run_id=$1", runID))
	if errors.Is(e, pgx.ErrNoRows) {
		var consumptionFailed, validationFailed, consumed bool
		e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumption_failed'),EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='output_validation_failed'),EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed')", runID).Scan(&consumptionFailed, &validationFailed, &consumed)
		if e != nil {
			return ContentGenerationResult{}, e
		}
		if !consumptionFailed || validationFailed || consumed {
			return ContentGenerationResult{}, ErrRunNotConsumable
		}
		candidate, e = s.consumeLocked(ctx, tx, run, uuid.Nil)
	} else if e != nil {
		return ContentGenerationResult{}, e
	}
	if e != nil {
		_ = tx.Rollback(ctx)
		if !errors.Is(e, ErrRunNotConsumable) {
			s.recordConsumptionFailure(ctx, run, errors.Is(e, ErrValidation))
		}
		return ContentGenerationResult{}, e
	}
	body, _ := json.Marshal(struct{ CandidateID uuid.UUID }{candidate.ID})
	if _, e = idem.Create(ctx, idempotency.Record{ID: uuid.New(), Scope: scope, Key: request.IdempotencyKey, RequestHash: hash, ResponseStatus: 200, ResponseBody: body}); e != nil {
		return ContentGenerationResult{}, e
	}
	out, e := s.generationResult(ctx, tx, runID, candidate.ID)
	if e != nil {
		return ContentGenerationResult{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		s.recordConsumptionFailure(ctx, run, false)
		return ContentGenerationResult{}, e
	}
	if run.Status == workflowrun.StatusFailed && run.FailurePhase != nil && *run.FailurePhase == "result_consumption" {
		runtime, ok := s.runs.(interface {
			RetryResultConsumption(context.Context, uuid.UUID, int) (workflowrun.WorkflowRun, error)
		})
		if !ok {
			return ContentGenerationResult{}, workflowrun.ErrNotRetryable
		}
		completed, completeErr := runtime.RetryResultConsumption(ctx, runID, request.ExpectedRunVersion)
		if completeErr != nil {
			return ContentGenerationResult{}, completeErr
		}
		out.WorkflowRun = completed
	}
	return out, nil
}
func (s *GenerationService) consume(ctx context.Context, run workflowrun.WorkflowRun) (ContentVersion, error) {
	if run.ID == uuid.Nil {
		return ContentVersion{}, ErrRunNotConsumable
	}
	tx, e := s.begin(ctx)
	if e != nil {
		return ContentVersion{}, e
	}
	candidate, e := s.consumeLocked(ctx, tx, run, uuid.Nil)
	if e != nil {
		_ = tx.Rollback(ctx)
		if !errors.Is(e, ErrRunNotConsumable) {
			s.recordConsumptionFailure(ctx, run, errors.Is(e, ErrValidation))
		}
		return ContentVersion{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		s.recordConsumptionFailure(ctx, run, false)
		return ContentVersion{}, e
	}
	return candidate, nil
}
func (s *GenerationService) SetCurrent(ctx context.Context, itemID uuid.UUID, request SetCurrentRequest) (Detail, error) {
	if itemID == uuid.Nil || request.CandidateVersionID == uuid.Nil ||
		request.ExpectedCurrentVersionID == uuid.Nil || request.ExpectedCurrentVersion < 1 ||
		strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 128 {
		return Detail{}, ErrValidation
	}
	hash := generationCommandFingerprint(struct {
		Candidate uuid.UUID
		Current   uuid.UUID
		Version   int
	}{request.CandidateVersionID, request.ExpectedCurrentVersionID, request.ExpectedCurrentVersion})
	scope := "setCurrentContentVersion:" + itemID.String()
	tx, e := s.repo.db.Begin(ctx)
	if e != nil {
		return Detail{}, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "idempotency:"+scope+":"+request.IdempotencyKey); e != nil {
		return Detail{}, e
	}
	idem := idempotency.NewPostgresRepositoryTx(tx)
	if record, get := idem.GetForUpdate(ctx, scope, request.IdempotencyKey); get == nil {
		if record.RequestHash != hash {
			var first struct {
				Detail Detail
			}
			if json.Unmarshal(record.ResponseBody, &first) == nil &&
				first.Detail.CurrentVersion.Source == ContentVersionSourceWorkflowRewrite {
				return Detail{}, ErrRewriteIdempotencyConflict
			}
			return Detail{}, workflowrun.ErrIdempotencyConflict
		}
		var replay struct {
			Detail Detail
		}
		if json.Unmarshal(record.ResponseBody, &replay) != nil || replay.Detail.CurrentVersion.ID == uuid.Nil {
			return Detail{}, ErrValidation
		}
		return replay.Detail, nil
	} else if !errors.Is(get, idempotency.ErrNotFound) {
		return Detail{}, get
	}
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "content-item:"+itemID.String()); e != nil {
		return Detail{}, e
	}
	item, e := scanItem(tx.QueryRow(ctx, "SELECT "+itemColumns+" FROM content_items WHERE id=$1 FOR UPDATE", itemID))
	if errors.Is(e, pgx.ErrNoRows) {
		return Detail{}, ErrContentItemNotFound
	}
	if e != nil {
		return Detail{}, e
	}
	current, e := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=$1 AND content_item_id=$2", item.CurrentVersionID, item.ID))
	if e != nil {
		return Detail{}, e
	}
	candidate, e := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=$1 FOR UPDATE", request.CandidateVersionID))
	if errors.Is(e, pgx.ErrNoRows) {
		return Detail{}, ErrRewriteCandidateNotFound
	}
	if e != nil {
		return Detail{}, e
	}
	if candidate.ContentItemID != item.ID {
		if candidate.Source == ContentVersionSourceWorkflowRewrite {
			return Detail{}, ErrRewriteCandidateNotFound
		}
		return Detail{}, ErrCandidateItemMismatch
	}
	isRewrite := candidate.Source == ContentVersionSourceWorkflowRewrite
	if candidate.Status != ContentVersionStatusEditableDraft ||
		(candidate.Source != ContentVersionSourceWorkflowGenerated && !isRewrite) {
		if isRewrite {
			return Detail{}, ErrRewriteCandidateNotReady
		}
		return Detail{}, ErrCandidateNotEligible
	}
	if candidate.SourceContentVersionID == nil || candidate.SourceContentVersionVersion == nil ||
		candidate.SourceWorkflowRunID == nil {
		if isRewrite {
			return Detail{}, ErrRewriteCandidateNotReady
		}
		return Detail{}, ErrCandidateNotEligible
	}
	if isRewrite {
		var ready bool
		e = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_records r JOIN review_reports p ON p.id=r.subject_id WHERE r.id=$1 AND r.project_id=$2 AND r.stage='rewrite' AND r.status='succeeded' AND r.subject_type='review_report' AND p.project_id=$2 AND p.content_item_id=$3 AND p.content_version_id=$4 AND p.source_content_version_version=$5 AND r.input_payload->>'sourceContentVersionId'=$4::text AND (r.input_payload->>'sourceContentVersionVersion')::integer=$5 AND EXISTS(SELECT 1 FROM workflow_run_events e WHERE e.run_id=r.id AND e.event_type='result_consumed'))", *candidate.SourceWorkflowRunID, item.ProjectID, item.ID, *candidate.SourceContentVersionID, *candidate.SourceContentVersionVersion).Scan(&ready)
		if e != nil {
			return Detail{}, e
		}
		if !ready {
			return Detail{}, ErrRewriteCandidateNotReady
		}
	}
	if candidate.ID == item.CurrentVersionID {
		out := Detail{Item: item, CurrentVersion: candidate}
		body, marshalErr := json.Marshal(struct {
			Detail Detail
		}{out})
		if marshalErr != nil {
			return Detail{}, marshalErr
		}
		if _, e = idem.Create(ctx, idempotency.Record{ID: uuid.New(), Scope: scope, Key: request.IdempotencyKey, RequestHash: hash, ResponseStatus: 200, ResponseBody: body}); e != nil {
			return Detail{}, e
		}
		if e = tx.Commit(ctx); e != nil {
			return Detail{}, e
		}
		return out, nil
	}
	if current.ID != request.ExpectedCurrentVersionID || current.Version != request.ExpectedCurrentVersion {
		if isRewrite {
			return Detail{}, ErrRewriteContentVersionConflict
		}
		return Detail{}, ErrVersionConflict
	}
	if !isRewrite && (*candidate.SourceContentVersionID != current.ID || *candidate.SourceContentVersionVersion != current.Version) {
		return Detail{}, ErrCandidateStale
	}
	if e = setCurrentContentItemPointer(ctx, tx, itemID, candidate.ID, request.ExpectedCurrentVersionID, item.Version); e != nil {
		if isRewrite && errors.Is(e, ErrVersionConflict) {
			return Detail{}, ErrRewriteContentVersionConflict
		}
		return Detail{}, e
	}
	out, e := s.repo.detail(ctx, tx, itemID)
	if e != nil {
		return Detail{}, e
	}
	body, e := json.Marshal(struct {
		Detail Detail
	}{out})
	if e != nil {
		return Detail{}, e
	}
	if _, e = idem.Create(ctx, idempotency.Record{ID: uuid.New(), Scope: scope, Key: request.IdempotencyKey, RequestHash: hash, ResponseStatus: 200, ResponseBody: body}); e != nil {
		if errors.Is(e, idempotency.ErrConflict) {
			return Detail{}, workflowrun.ErrIdempotencyConflict
		}
		return Detail{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return Detail{}, e
	}
	return out, nil
}

func setCurrentContentItemPointer(ctx context.Context, tx pgx.Tx, itemID, candidateID, expectedCurrentID uuid.UUID, expectedItemVersion int) error {
	tag, e := tx.Exec(ctx, "UPDATE content_items SET current_version_id=$1,status='draft',reviewed_at=NULL,version=version+1,updated_at=NOW() WHERE id=$2 AND current_version_id=$3 AND version=$4", candidateID, itemID, expectedCurrentID, expectedItemVersion)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (s *GenerationService) recordConsumptionFailure(ctx context.Context, run workflowrun.WorkflowRun, validation bool) {
	eventType := workflowrun.EventTypeResultConsumptionFailed
	if validation {
		eventType = workflowrun.EventTypeOutputValidationFailed
	}
	_, _ = s.runs.AddEvent(ctx, workflowrun.Event{ID: uuid.New(), RunID: run.ID, EventType: eventType, Status: run.Status, Payload: json.RawMessage(`{}`), CreatedAt: s.now().UTC()})
}

type generationOutput struct {
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	Summary   *string `json:"summary"`
	WordCount int     `json:"wordCount"`
}

func decodeGenerationOutput(raw json.RawMessage) (generationOutput, error) {
	var output generationOutput
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&output); err != nil {
		return output, ErrValidation
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return output, ErrValidation
	}
	if strings.TrimSpace(output.Title) == "" || len([]rune(output.Title)) > 120 || strings.TrimSpace(output.Content) == "" || len([]rune(output.Content)) > 200000 || output.Summary == nil || len([]rune(*output.Summary)) > 5000 || output.WordCount <= 0 || output.WordCount != len([]rune(output.Content)) {
		return output, ErrValidation
	}
	return output, nil
}
func (s *GenerationService) consumeLocked(ctx context.Context, tx pgx.Tx, run workflowrun.WorkflowRun, subjectID uuid.UUID) (ContentVersion, error) {
	var locked workflowrun.WorkflowRun
	locked.ID = run.ID
	var subjectType string
	var lockedSubjectID *uuid.UUID
	e := tx.QueryRow(ctx, "SELECT project_id,stage,status,subject_type,subject_id,input_payload,output_payload FROM workflow_run_records WHERE id=$1 FOR UPDATE", run.ID).Scan(&locked.ProjectID, &locked.Stage, &locked.Status, &subjectType, &lockedSubjectID, &locked.InputPayload, &locked.OutputPayload)
	if errors.Is(e, pgx.ErrNoRows) {
		return ContentVersion{}, workflowrun.ErrNotFound
	}
	if e != nil {
		return ContentVersion{}, e
	}
	if subjectType != "content_item" || locked.SubjectID == nil {
		locked.SubjectID = lockedSubjectID
	}
	if locked.Stage != "content_generation" || (locked.Status != workflowrun.StatusSucceeded && locked.Status != workflowrun.StatusFailed && locked.Status != workflowrun.StatusRunning) || subjectType != "content_item" || lockedSubjectID == nil || *lockedSubjectID == uuid.Nil {
		return ContentVersion{}, ErrRunNotConsumable
	}
	if subjectID != uuid.Nil && subjectID != *lockedSubjectID {
		return ContentVersion{}, ErrRunNotConsumable
	}
	locked.SubjectType = &subjectType
	locked.SubjectID = lockedSubjectID
	run = locked
	if v, e := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE source_workflow_run_id=$1", run.ID)); e == nil {
		return v, nil
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return ContentVersion{}, e
	}
	var input struct {
		SourceContentVersionID      uuid.UUID `json:"sourceContentVersionId"`
		SourceContentVersionVersion int       `json:"sourceContentVersionVersion"`
	}
	output, outputErr := decodeGenerationOutput(run.OutputPayload)
	if json.Unmarshal(run.InputPayload, &input) != nil || input.SourceContentVersionID == uuid.Nil || input.SourceContentVersionVersion < 1 || outputErr != nil {
		return ContentVersion{}, ErrValidation
	}
	if _, e = tx.Exec(ctx, "SELECT 1 FROM content_items WHERE id=$1 FOR UPDATE", *lockedSubjectID); e != nil {
		return ContentVersion{}, e
	}
	detail, e := s.repo.detail(ctx, tx, *lockedSubjectID)
	if errors.Is(e, pgx.ErrNoRows) {
		return ContentVersion{}, ErrRunNotConsumable
	}
	if e != nil {
		return ContentVersion{}, e
	}
	if detail.Item.ProjectID != run.ProjectID {
		return ContentVersion{}, ErrRunNotConsumable
	}
	var sourceItem, sourceProject uuid.UUID
	var sourceVersion int
	e = tx.QueryRow(ctx, "SELECT v.content_item_id,i.project_id,v.version FROM content_versions v JOIN content_items i ON i.id=v.content_item_id WHERE v.id=$1", input.SourceContentVersionID).Scan(&sourceItem, &sourceProject, &sourceVersion)
	if errors.Is(e, pgx.ErrNoRows) {
		return ContentVersion{}, ErrGenerationSourceInvalid
	}
	if e != nil {
		return ContentVersion{}, e
	}
	if sourceItem != detail.Item.ID || sourceProject != detail.Item.ProjectID || sourceVersion != input.SourceContentVersionVersion {
		return ContentVersion{}, ErrGenerationSourceInvalid
	}
	versionNo, e := nextContentVersionNo(ctx, tx, detail.Item.ID)
	if e != nil {
		return ContentVersion{}, e
	}
	candidate := ContentVersion{ID: uuid.New(), ContentItemID: detail.Item.ID, VersionNo: versionNo, SourceContentVersionID: &input.SourceContentVersionID, SourceContentVersionVersion: &input.SourceContentVersionVersion, SourceWorkflowRunID: &run.ID, Title: strings.TrimSpace(output.Title), Content: output.Content, Summary: output.Summary, WordCount: output.WordCount, Source: ContentVersionSourceWorkflowGenerated, Status: ContentVersionStatusEditableDraft, Version: 1}
	created, e := s.repo.CreateContentVersion(ctx, tx, candidate)
	if e != nil {
		return ContentVersion{}, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO workflow_run_events(id,run_id,event_type,status,payload,created_at) VALUES($1,$2,'result_consumed',$3,'{}',NOW())", uuid.New(), run.ID, run.Status); e != nil {
		return ContentVersion{}, e
	}
	return created, nil
}

func nextContentVersionNo(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, itemID uuid.UUID) (int, error) {
	var value int
	if err := q.QueryRow(ctx, "SELECT COALESCE(MAX(version_no), 0) + 1 FROM content_versions WHERE content_item_id=$1", itemID).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}
func generationRunError(run workflowrun.WorkflowRun) *GenerationSafeError {
	message := "正文生成任务未完成"
	if run.ErrorMessage != nil {
		message = *run.ErrorMessage
	}
	code := "runtime_failed"
	if run.ErrorCode != nil {
		code = *run.ErrorCode
	}
	return safeGenerationError(code, message, "正文生成任务未完成")
}
func generationEventError(event workflowrun.Event, state string) *GenerationSafeError {
	fallback := "正文生成结果处理失败"
	if state == "output_validation_failed" {
		fallback = "生成结果未通过校验"
	}
	var payload struct{ Code, Message string }
	_ = json.Unmarshal(event.Payload, &payload)
	if payload.Code == "" {
		payload.Code = state
	}
	return safeGenerationError(payload.Code, payload.Message, fallback)
}
func safeGenerationError(code, message, fallback string) *GenerationSafeError {
	message = strings.TrimSpace(message)
	lower := strings.ToLower(message)
	if message == "" || len([]rune(message)) > 500 || strings.ContainsAny(message, "\r\n") {
		message = fallback
	}
	for _, forbidden := range []string{"sql", "postgres", "stack", "traceback", "/", "\\", "token", "authorization", "credential"} {
		if strings.Contains(lower, forbidden) {
			message = fallback
			break
		}
	}
	code = strings.TrimSpace(code)
	safeCode := code != "" && len(code) <= 120
	for _, r := range code {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			safeCode = false
		}
	}
	if !safeCode {
		code = "generation_failed"
	}
	return &GenerationSafeError{Code: code, Message: message, Details: map[string]any{}}
}
func (s *GenerationService) generationResult(ctx context.Context, tx pgx.Tx, runID, candidateID uuid.UUID) (ContentGenerationResult, error) {
	candidate, e := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=$1", candidateID))
	if e != nil {
		return ContentGenerationResult{}, e
	}
	detail, e := s.repo.detail(ctx, tx, candidate.ContentItemID)
	if e != nil {
		return ContentGenerationResult{}, e
	}
	run, e := workflowrun.NewPostgresRepositoryTx(tx).GetByID(ctx, runID)
	if e != nil {
		return ContentGenerationResult{}, e
	}
	return ContentGenerationResult{detail.Item, candidate, run}, nil
}
func generationCommandFingerprint(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

package contentitem

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/idempotency"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

var (
	ErrReviewNotConfigured       = errors.New("review workflow is not configured")
	ErrReviewNotReviewable       = errors.New("content version is not reviewable")
	ErrReviewActiveRun           = errors.New("review run is already active")
	ErrReviewPreflightChanged    = errors.New("review preflight input changed")
	ErrReviewTokenInvalid        = errors.New("review preflight token is invalid")
	ErrReviewTokenExpired        = errors.New("review preflight token expired")
	ErrReviewTokenConsumed       = workflowrun.ErrPreflightTokenConsumed
	ErrReviewOutputInvalid       = errors.New("review output validation failed")
	ErrReviewResultNotRetryable  = errors.New("review result is not retryable")
	ErrReviewResultConsumption   = errors.New("review result consumption failed")
	ErrReviewIssueNotFound       = errors.New("review issue not found")
	ErrReviewIssueVersion        = errors.New("review issue version conflict")
)

var frozenReviewDimensions = []string{
	"compliance",
	"factual_consistency",
	"language_quality",
	"structural_logic",
	"character_consistency",
}

type ReviewPreflightRequest struct {
	SourceContentVersionVersion int
	OptionalInstructions        *string
	ActorID                      string
}

type ReviewCheck struct {
	Code    string `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ReviewSourceVersionSummary struct {
	ID            uuid.UUID `json:"id"`
	ContentItemID uuid.UUID `json:"contentItemId"`
	VersionNo     int       `json:"versionNo"`
	Version       int       `json:"version"`
	Title         string    `json:"title"`
	WordCount     int       `json:"wordCount"`
	ContentHash   string    `json:"contentHash"`
	Frozen        bool      `json:"frozen"`
}

type ReviewConfigurationSummary struct {
	BindingVersion               int       `json:"bindingVersion"`
	WorkflowConfigurationID      uuid.UUID `json:"workflowConfigurationId"`
	WorkflowConfigurationName    string    `json:"workflowConfigurationName"`
	WorkflowConfigurationVersion int       `json:"workflowConfigurationVersion"`
	ConnectionID                 uuid.UUID `json:"connectionId"`
	ConnectionVersion            int       `json:"connectionVersion"`
	InputContract                string    `json:"inputContract"`
	OutputContract               string    `json:"outputContract"`
}

type ReviewPreflightResult struct {
	Status                      string                      `json:"status"`
	Checks                      []ReviewCheck               `json:"checks"`
	SourceContentVersionSummary ReviewSourceVersionSummary  `json:"sourceContentVersionSummary"`
	ReviewDimensions            []string                    `json:"reviewDimensions"`
	ConfigurationSummary        *ReviewConfigurationSummary `json:"configurationSummary"`
	PreflightToken              *string                     `json:"preflightToken"`
	ExpiresAt                   *time.Time                  `json:"expiresAt"`
}

type ReviewRuntimeInputV1 struct {
	SchemaVersion               string      `json:"schemaVersion"`
	ProjectID                   uuid.UUID   `json:"projectId"`
	ContentItemID               uuid.UUID   `json:"contentItemId"`
	SourceContentVersionID      uuid.UUID   `json:"sourceContentVersionId"`
	SourceContentVersionVersion int         `json:"sourceContentVersionVersion"`
	SourceContentHash           string      `json:"sourceContentHash"`
	SourceTitle                 string      `json:"sourceTitle"`
	SourceContent               string      `json:"sourceContent"`
	OptionalInstructions        *string     `json:"optionalInstructions"`
	ReviewDimensions            []string    `json:"reviewDimensions"`
	WorkflowRunID               uuid.UUID   `json:"workflowRunId"`
	CorrelationID               string      `json:"correlationId"`
}

type ReviewRuntimeEvidenceV1 struct {
	Quote      *string  `json:"quote"`
	SourceRefs []string `json:"sourceRefs"`
}

type ReviewRuntimeLocationV1 struct {
	ParagraphStart int `json:"paragraphStart"`
	ParagraphEnd   int `json:"paragraphEnd"`
	SentenceStart  int `json:"sentenceStart"`
	SentenceEnd    int `json:"sentenceEnd"`
}

type ReviewRuntimeIssueV1 struct {
	IssueKey     string                   `json:"issueKey"`
	Position     int                      `json:"position"`
	CategoryKey  string                   `json:"categoryKey"`
	CategoryLabel string                  `json:"categoryLabel"`
	Severity     string                   `json:"severity"`
	Title        string                   `json:"title"`
	Description  string                   `json:"description"`
	Evidence     ReviewRuntimeEvidenceV1  `json:"evidence"`
	Location     *ReviewRuntimeLocationV1 `json:"location"`
	Suggestion   *string                  `json:"suggestion"`
}

type ReviewRuntimeRecommendationV1 struct {
	Position    int    `json:"position"`
	Priority    string `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ReviewRuntimeOutputV1 struct {
	SchemaVersion  string                          `json:"schemaVersion"`
	Conclusion     string                          `json:"conclusion"`
	Summary        string                          `json:"summary"`
	PassedRuleCount int                            `json:"passedRuleCount"`
	Issues         []ReviewRuntimeIssueV1          `json:"issues"`
	Recommendations []ReviewRuntimeRecommendationV1 `json:"recommendations"`
}

type RealReviewReport struct {
	ID                          uuid.UUID `json:"id"`
	ContentItemID               uuid.UUID `json:"contentItemId"`
	SourceContentVersionID      uuid.UUID `json:"sourceContentVersionId"`
	SourceContentVersionVersion int       `json:"sourceContentVersionVersion"`
	SourceContentHash           string    `json:"sourceContentHash"`
	WorkflowRunID               uuid.UUID `json:"workflowRunId"`
	SchemaVersion               string    `json:"schemaVersion"`
	Conclusion                  string    `json:"conclusion"`
	Summary                     string    `json:"summary"`
	PassedRuleCount             int       `json:"passedRuleCount"`
	CreatedAt                   time.Time `json:"createdAt"`
	CompletedAt                 time.Time `json:"completedAt"`
}

type RealReviewIssue struct {
	ID            uuid.UUID                `json:"id"`
	ReviewID      uuid.UUID                `json:"reviewId"`
	IssueKey      string                   `json:"issueKey"`
	Position      int                      `json:"position"`
	CategoryKey   string                   `json:"categoryKey"`
	CategoryLabel string                   `json:"categoryLabel"`
	Severity      string                   `json:"severity"`
	Title         string                   `json:"title"`
	Description   string                   `json:"description"`
	Evidence      ReviewRuntimeEvidenceV1  `json:"evidence"`
	Location      *ReviewRuntimeLocationV1 `json:"location"`
	Suggestion    *string                  `json:"suggestion"`
	Disposition   string                   `json:"disposition"`
	Version       int                      `json:"version"`
	IgnoredAt     *time.Time               `json:"ignoredAt"`
	IgnoredBy     *string                  `json:"ignoredBy"`
	CreatedAt     time.Time                `json:"createdAt"`
	UpdatedAt     time.Time                `json:"updatedAt"`
}

type RealReviewRecommendation struct {
	ID          uuid.UUID `json:"id"`
	ReviewID    uuid.UUID `json:"reviewId"`
	Position    int       `json:"position"`
	Priority    string    `json:"priority"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type RealReviewResult struct {
	Report          RealReviewReport           `json:"report"`
	Issues          []RealReviewIssue          `json:"issues"`
	Recommendations []RealReviewRecommendation `json:"recommendations"`
	WorkflowRun     workflowrun.WorkflowRun    `json:"workflowRun"`
}

type ReviewIssueSummary struct {
	Total      int `json:"total"`
	Critical   int `json:"critical"`
	Warning    int `json:"warning"`
	Suggestion int `json:"suggestion"`
	Open       int `json:"open"`
	Ignored    int `json:"ignored"`
}

type ReviewSafeError struct {
	Code          string    `json:"code"`
	Message       string    `json:"message"`
	CorrelationID string    `json:"correlationId"`
	AttemptCount  int       `json:"attemptCount"`
	OccurredAt    time.Time `json:"occurredAt"`
}

type ReviewReportSummary struct {
	ID                     uuid.UUID `json:"id"`
	WorkflowRunID          uuid.UUID `json:"workflowRunId"`
	SourceContentVersionID uuid.UUID `json:"sourceContentVersionId"`
	Conclusion             string    `json:"conclusion"`
	Summary                string    `json:"summary"`
	CompletedAt            time.Time `json:"completedAt"`
}

type ContentReviewSummary struct {
	ContentItemID        uuid.UUID                   `json:"contentItemId"`
	State                string                      `json:"state"`
	CanStartReview       bool                        `json:"canStartReview"`
	ActiveRun            *workflowrun.WorkflowRun    `json:"activeRun"`
	LatestRun            *workflowrun.WorkflowRun    `json:"latestRun"`
	LatestReport         *ReviewReportSummary        `json:"latestReport"`
	IssueSummary         *ReviewIssueSummary         `json:"issueSummary"`
	LatestError          *ReviewSafeError            `json:"latestError"`
	ConfigurationSummary *ReviewConfigurationSummary `json:"configurationSummary"`
}

type ReviewResultConsumptionRetryRequest struct {
	ExpectedRunVersion int
	IdempotencyKey     string
}

type ReviewIssueUpdateRequest struct {
	Disposition   string
	ExpectedVersion int
	IdempotencyKey string
	ActorID        string
}

type ReviewHistoryItem struct {
	WorkflowRun                 workflowrun.WorkflowRun       `json:"workflowRun"`
	SourceContentVersionSummary ReviewSourceVersionSummary    `json:"sourceContentVersionSummary"`
	ReportSummary               *ReviewReportSummary          `json:"reportSummary"`
	State                       string                        `json:"state"`
	LatestError                 *ReviewSafeError              `json:"latestError"`
}

type ReviewHistoryPage struct {
	Items  []any `json:"items"`
	Total  int   `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type RealReviewDetail struct {
	Report                      RealReviewReport            `json:"report"`
	SourceContentVersionSummary ReviewSourceVersionSummary  `json:"sourceContentVersionSummary"`
	Issues                      []RealReviewIssue           `json:"issues"`
	Recommendations             []RealReviewRecommendation  `json:"recommendations"`
	WorkflowRunSummary          workflowrun.WorkflowRun     `json:"workflowRunSummary"`
}

type reviewRunService interface {
	CreateRunForPreflightTokenIdempotentForScope(context.Context, string, uuid.UUID, string, string, string, workflowrun.CreateRunTxPreparation) (workflowrun.WorkflowRun, error)
	CreateRunForPreflightTokenIdempotentForScopeWithReplay(context.Context, string, uuid.UUID, string, string, string, workflowrun.CreateRunTxPreparation) (workflowrun.WorkflowRun, bool, error)
	ListRuns(context.Context, workflowrun.ListRunsQuery) (workflowrun.RunList, error)
	ListRunEvents(context.Context, uuid.UUID) ([]workflowrun.Event, error)
	GetRun(context.Context, uuid.UUID) (workflowrun.WorkflowRun, error)
	AddEvent(context.Context, workflowrun.Event) (workflowrun.Event, error)
}

type RealReviewService struct {
	repo     *PostgresRepository
	bindings interface {
		GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error)
	}
	configs interface {
		GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error)
		GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error)
	}
	runs   reviewRunService
	secret []byte
	now    func() time.Time
}

func NewRealReviewService(
	repo *PostgresRepository,
	bindings interface {
		GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error)
	},
	configs interface {
		GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error)
		GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error)
	},
	runs reviewRunService,
	secret string,
) *RealReviewService {
	return &RealReviewService{repo: repo, bindings: bindings, configs: configs, runs: runs, secret: []byte(secret), now: time.Now}
}

type reviewSource struct {
	Version ContentVersion
	Item    ContentItem
}

func (s *RealReviewService) source(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, versionID uuid.UUID, lock bool) (reviewSource, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE OF v"
	}
	var out reviewSource
	err := q.QueryRow(ctx, "SELECT "+qualifiedColumns("v", versionColumns)+","+qualifiedColumns("i", itemColumns)+" FROM content_versions v JOIN content_items i ON i.id=v.content_item_id WHERE v.id=$1"+suffix, versionID).Scan(
		&out.Version.ID, &out.Version.ContentItemID, &out.Version.VersionNo, &out.Version.SourceContentVersionID,
		&out.Version.SourceContentVersionVersion, &out.Version.SourceWorkflowRunID, &out.Version.Title,
		&out.Version.Content, &out.Version.Summary, &out.Version.WordCount, &out.Version.Source,
		&out.Version.Status, &out.Version.GenerationParameters, &out.Version.Version, &out.Version.FrozenAt,
		&out.Version.CreatedAt, &out.Version.UpdatedAt,
		&out.Item.ID, &out.Item.ProjectID, &out.Item.ChapterPlanID, &out.Item.Title, &out.Item.Status,
		&out.Item.CurrentVersionID, &out.Item.Version, &out.Item.ReviewedAt, &out.Item.CreatedAt, &out.Item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return reviewSource{}, ErrContentVersionNotFound
	}
	return out, err
}

func reviewContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func reviewSourceSummary(source reviewSource) ReviewSourceVersionSummary {
	return ReviewSourceVersionSummary{
		ID: source.Version.ID, ContentItemID: source.Item.ID, VersionNo: source.Version.VersionNo,
		Version: source.Version.Version, Title: source.Version.Title, WordCount: source.Version.WordCount,
		ContentHash: reviewContentHash(source.Version.Content), Frozen: source.Version.Status == ContentVersionStatusFrozen,
	}
}

func reviewDimensions(workflow globalconfig.Workflow) ([]string, error) {
	var configured struct {
		ReviewDimensions []string `json:"reviewDimensions"`
	}
	if len(workflow.DefaultParameters) > 0 && json.Unmarshal(workflow.DefaultParameters, &configured) != nil {
		return nil, ErrReviewNotConfigured
	}
	if len(configured.ReviewDimensions) == 0 {
		return append([]string(nil), frozenReviewDimensions...), nil
	}
	seen := map[string]bool{}
	for _, value := range configured.ReviewDimensions {
		if seen[value] || !containsString(frozenReviewDimensions, value) {
			return nil, ErrReviewNotConfigured
		}
		seen[value] = true
	}
	return append([]string(nil), configured.ReviewDimensions...), nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (s *RealReviewService) runnable(ctx context.Context, projectID uuid.UUID) (workflowbinding.ProjectWorkflowBinding, globalconfig.Workflow, globalconfig.Connection, []string, error) {
	binding, err := s.bindings.GetByProjectAndStage(ctx, projectID, workflowbinding.StageReview)
	if errors.Is(err, workflowbinding.ErrNotFound) {
		return binding, globalconfig.Workflow{}, globalconfig.Connection{}, nil, ErrReviewNotConfigured
	}
	if err != nil {
		return binding, globalconfig.Workflow{}, globalconfig.Connection{}, nil, err
	}
	workflow, err := s.configs.GetWorkflow(ctx, binding.WorkflowConfigurationID)
	if errors.Is(err, globalconfig.ErrNotFound) {
		return binding, workflow, globalconfig.Connection{}, nil, ErrReviewNotConfigured
	}
	if err != nil {
		return binding, workflow, globalconfig.Connection{}, nil, err
	}
	connection, err := s.configs.GetConnection(ctx, workflow.ConnectionID)
	if errors.Is(err, globalconfig.ErrNotFound) {
		return binding, workflow, connection, nil, ErrReviewNotConfigured
	}
	if err != nil {
		return binding, workflow, connection, nil, err
	}
	if !workflow.Enabled || !connection.Enabled || connection.IntegrationStatus != "connected" ||
		!containsString(workflow.ApplicableStages, "review") ||
		workflow.InputContractVersion != "review.input.v1" || workflow.OutputContractVersion != "review.output.v1" {
		return binding, workflow, connection, nil, ErrReviewNotConfigured
	}
	dimensions, err := reviewDimensions(workflow)
	if err != nil {
		return binding, workflow, connection, nil, err
	}
	return binding, workflow, connection, dimensions, nil
}

func reviewConfigurationSummary(binding workflowbinding.ProjectWorkflowBinding, workflow globalconfig.Workflow, connection globalconfig.Connection) *ReviewConfigurationSummary {
	return &ReviewConfigurationSummary{
		BindingVersion: binding.Version, WorkflowConfigurationID: workflow.ID,
		WorkflowConfigurationName: workflow.Name, WorkflowConfigurationVersion: workflow.Version,
		ConnectionID: connection.ID, ConnectionVersion: connection.Version,
		InputContract: "review.input.v1", OutputContract: "review.output.v1",
	}
}

type reviewTokenClaims struct {
	ProjectID                   uuid.UUID `json:"projectId"`
	ActorID                     string    `json:"actorId"`
	ContentItemID               uuid.UUID `json:"contentItemId"`
	SourceContentVersionID      uuid.UUID `json:"sourceContentVersionId"`
	SourceContentVersionVersion int       `json:"sourceContentVersionVersion"`
	SourceContentHash           string    `json:"sourceContentHash"`
	OptionalInstructions        *string   `json:"optionalInstructions"`
	ReviewDimensions            []string  `json:"reviewDimensions"`
	BindingID                   uuid.UUID `json:"bindingId"`
	BindingVersion              int       `json:"bindingVersion"`
	ConfigurationID             uuid.UUID `json:"configurationId"`
	ConfigurationVersion        int       `json:"configurationVersion"`
	ConnectionID                uuid.UUID `json:"connectionId"`
	ConnectionVersion           int       `json:"connectionVersion"`
	InputDigest                 string    `json:"inputDigest"`
	Nonce                       string    `json:"nonce"`
	IssuedAt                    int64     `json:"iat"`
	ExpiresAt                   int64     `json:"exp"`
}

func (s *RealReviewService) signReviewToken(claims reviewTokenClaims) (string, error) {
	if len(s.secret) == 0 || claims.ProjectID == uuid.Nil || claims.SourceContentVersionID == uuid.Nil ||
		claims.SourceContentVersionVersion < 1 || len(claims.SourceContentHash) != 64 || claims.ExpiresAt != claims.IssuedAt+600 {
		return "", ErrReviewTokenInvalid
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *RealReviewService) parseReviewToken(raw string) (reviewTokenClaims, error) {
	var claims reviewTokenClaims
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || len(s.secret) == 0 {
		return claims, ErrReviewTokenInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, ErrReviewTokenInvalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims, ErrReviewTokenInvalid
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(payload, &claims) != nil {
		return claims, ErrReviewTokenInvalid
	}
	if claims.ProjectID == uuid.Nil || claims.ContentItemID == uuid.Nil || claims.SourceContentVersionID == uuid.Nil ||
		claims.SourceContentVersionVersion < 1 || strings.TrimSpace(claims.ActorID) == "" ||
		claims.BindingID == uuid.Nil || claims.ConfigurationID == uuid.Nil || claims.ConnectionID == uuid.Nil ||
		len(claims.InputDigest) != 64 || strings.TrimSpace(claims.Nonce) == "" {
		return claims, ErrReviewTokenInvalid
	}
	return claims, nil
}

func (s *RealReviewService) verifyReviewToken(raw string) (reviewTokenClaims, error) {
	claims, err := s.parseReviewToken(raw)
	if err != nil {
		return claims, err
	}
	if claims.ExpiresAt <= s.now().Unix() {
		return claims, ErrReviewTokenExpired
	}
	return claims, nil
}

func reviewInputDigest(source reviewSource, instructions *string, dimensions []string, binding workflowbinding.ProjectWorkflowBinding, workflow globalconfig.Workflow, connection globalconfig.Connection) string {
	value := struct {
		ProjectID                   uuid.UUID `json:"projectId"`
		ContentItemID               uuid.UUID `json:"contentItemId"`
		SourceContentVersionID      uuid.UUID `json:"sourceContentVersionId"`
		SourceContentVersionVersion int       `json:"sourceContentVersionVersion"`
		SourceContentHash           string    `json:"sourceContentHash"`
		SourceTitle                 string    `json:"sourceTitle"`
		SourceContent               string    `json:"sourceContent"`
		OptionalInstructions        *string   `json:"optionalInstructions"`
		ReviewDimensions            []string  `json:"reviewDimensions"`
		BindingID                   uuid.UUID `json:"bindingId"`
		BindingVersion              int       `json:"bindingVersion"`
		ConfigurationID             uuid.UUID `json:"configurationId"`
		ConfigurationVersion        int       `json:"configurationVersion"`
		ConnectionID                uuid.UUID `json:"connectionId"`
		ConnectionVersion           int       `json:"connectionVersion"`
	}{
		source.Item.ProjectID, source.Item.ID, source.Version.ID, source.Version.Version,
		reviewContentHash(source.Version.Content), source.Version.Title, source.Version.Content,
		instructions, dimensions, binding.ID, binding.Version, workflow.ID, workflow.Version,
		connection.ID, connection.Version,
	}
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *RealReviewService) Preflight(ctx context.Context, versionID uuid.UUID, request ReviewPreflightRequest) (ReviewPreflightResult, error) {
	if versionID == uuid.Nil || request.SourceContentVersionVersion < 1 || strings.TrimSpace(request.ActorID) == "" ||
		(request.OptionalInstructions != nil && utf8.RuneCountInString(*request.OptionalInstructions) > 2000) {
		return ReviewPreflightResult{}, ErrValidation
	}
	source, err := s.source(ctx, s.repo.db, versionID, false)
	if err != nil {
		return ReviewPreflightResult{}, err
	}
	result := ReviewPreflightResult{
		Status: "blocked", Checks: []ReviewCheck{}, SourceContentVersionSummary: reviewSourceSummary(source),
		ReviewDimensions: append([]string(nil), frozenReviewDimensions...),
	}
	add := func(code, status, message string) {
		result.Checks = append(result.Checks, ReviewCheck{Code: code, Status: status, Message: message})
	}
	if strings.TrimSpace(source.Version.Title) == "" || strings.TrimSpace(source.Version.Content) == "" {
		add("source_version_saved", "blocked", "来源正文尚不可审核")
		return result, nil
	}
	add("source_version_saved", "passed", "来源正文已保存")
	if source.Version.Version != request.SourceContentVersionVersion {
		add("source_version_unchanged", "blocked", "来源正文版本已变化")
		return result, nil
	}
	add("source_version_unchanged", "passed", "来源正文版本未变化")
	binding, workflow, connection, dimensions, err := s.runnable(ctx, source.Item.ProjectID)
	if errors.Is(err, ErrReviewNotConfigured) {
		add("project_binding_available", "blocked", "项目审核工作流尚未配置")
		add("workflow_configuration_available", "blocked", "审核工作流配置不可用")
		add("workflow_connection_available", "blocked", "审核工作流连接不可用")
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.ReviewDimensions = dimensions
	result.ConfigurationSummary = reviewConfigurationSummary(binding, workflow, connection)
	add("project_binding_available", "passed", "项目审核工作流绑定可用")
	add("workflow_configuration_available", "passed", "审核工作流配置可用")
	add("workflow_connection_available", "passed", "审核工作流连接可用")
	var active bool
	err = s.repo.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_records WHERE project_id=$1 AND stage='review' AND subject_type='content_version' AND subject_id=$2 AND status IN ('queued','running'))", source.Item.ProjectID, versionID).Scan(&active)
	if err != nil {
		return result, err
	}
	if active {
		add("active_review_run_absent", "blocked", "该正文版本已有审核任务")
		return result, nil
	}
	add("active_review_run_absent", "passed", "该正文版本没有活跃审核任务")
	add("review_input_valid", "passed", "审核输入有效")
	now := s.now().UTC()
	claims := reviewTokenClaims{
		ProjectID: source.Item.ProjectID, ActorID: request.ActorID, ContentItemID: source.Item.ID,
		SourceContentVersionID: source.Version.ID, SourceContentVersionVersion: source.Version.Version,
		SourceContentHash: reviewContentHash(source.Version.Content), OptionalInstructions: request.OptionalInstructions,
		ReviewDimensions: dimensions, BindingID: binding.ID, BindingVersion: binding.Version,
		ConfigurationID: workflow.ID, ConfigurationVersion: workflow.Version,
		ConnectionID: connection.ID, ConnectionVersion: connection.Version,
		InputDigest: reviewInputDigest(source, request.OptionalInstructions, dimensions, binding, workflow, connection),
		Nonce: uuid.NewString(), IssuedAt: now.Unix(), ExpiresAt: now.Add(10 * time.Minute).Unix(),
	}
	token, err := s.signReviewToken(claims)
	if err != nil {
		return result, err
	}
	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	result.Status, result.PreflightToken, result.ExpiresAt = "passed", &token, &expiresAt
	return result, nil
}

func runnableReviewForCreate(ctx context.Context, tx pgx.Tx, projectID uuid.UUID) (workflowbinding.ProjectWorkflowBinding, globalconfig.Workflow, globalconfig.Connection, []string, error) {
	var id, bindingProjectID, configurationID uuid.UUID
	var stage workflowbinding.WorkflowBindingStage
	var version int
	var createdAt, updatedAt time.Time
	err := tx.QueryRow(ctx, "SELECT id,project_id,stage,workflow_configuration_id,version,created_at,updated_at FROM project_workflow_bindings WHERE project_id=$1 AND stage='review' FOR SHARE", projectID).Scan(
		&id, &bindingProjectID, &stage, &configurationID, &version, &createdAt, &updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return workflowbinding.ProjectWorkflowBinding{}, globalconfig.Workflow{}, globalconfig.Connection{}, nil, ErrReviewNotConfigured
	}
	if err != nil {
		return workflowbinding.ProjectWorkflowBinding{}, globalconfig.Workflow{}, globalconfig.Connection{}, nil, err
	}
	binding, err := workflowbinding.NewFromDB(id, bindingProjectID, configurationID, stage, version, createdAt, updatedAt)
	if err != nil {
		return binding, globalconfig.Workflow{}, globalconfig.Connection{}, nil, err
	}
	workflow, err := globalconfig.GetWorkflowForShare(ctx, tx, configurationID)
	if errors.Is(err, globalconfig.ErrNotFound) {
		return binding, workflow, globalconfig.Connection{}, nil, ErrReviewNotConfigured
	}
	if err != nil {
		return binding, workflow, globalconfig.Connection{}, nil, err
	}
	connection, err := globalconfig.GetConnectionForShare(ctx, tx, workflow.ConnectionID)
	if errors.Is(err, globalconfig.ErrNotFound) {
		return binding, workflow, connection, nil, ErrReviewNotConfigured
	}
	if err != nil {
		return binding, workflow, connection, nil, err
	}
	if !workflow.Enabled || !connection.Enabled || connection.IntegrationStatus != "connected" ||
		!containsString(workflow.ApplicableStages, "review") ||
		workflow.InputContractVersion != "review.input.v1" || workflow.OutputContractVersion != "review.output.v1" {
		return binding, workflow, connection, nil, ErrReviewNotConfigured
	}
	dimensions, err := reviewDimensions(workflow)
	if err != nil {
		return binding, workflow, connection, nil, err
	}
	return binding, workflow, connection, dimensions, nil
}

func (s *RealReviewService) CreateRun(ctx context.Context, versionID uuid.UUID, actorID, token, key string) (workflowrun.WorkflowRun, error) {
	run, _, err := s.CreateRunWithReplay(ctx, versionID, actorID, token, key)
	return run, err
}

func (s *RealReviewService) CreateRunWithReplay(ctx context.Context, versionID uuid.UUID, actorID, token, key string) (workflowrun.WorkflowRun, bool, error) {
	claims, err := s.parseReviewToken(token)
	if err != nil {
		return workflowrun.WorkflowRun{}, false, err
	}
	if versionID == uuid.Nil || strings.TrimSpace(actorID) == "" || strings.TrimSpace(key) == "" || len(key) > 128 {
		return workflowrun.WorkflowRun{}, false, ErrValidation
	}
	if claims.SourceContentVersionID != versionID || claims.ActorID != actorID {
		return workflowrun.WorkflowRun{}, false, ErrReviewPreflightChanged
	}
	tokenSum := sha256.Sum256([]byte(token))
	requestHash := workflowrun.Fingerprint(struct {
		VersionID   uuid.UUID `json:"versionId"`
		TokenDigest string    `json:"tokenDigest"`
		ActorID     string    `json:"actorId"`
		InputDigest string    `json:"inputDigest"`
	}{versionID, hex.EncodeToString(tokenSum[:]), actorID, claims.InputDigest})
	return s.runs.CreateRunForPreflightTokenIdempotentForScopeWithReplay(ctx, "createContentReviewRun", claims.ProjectID, key, requestHash, claims.Nonce, func(tx pgx.Tx) (workflowrun.CreateRunCommand, error) {
		if claims.ExpiresAt <= s.now().Unix() {
			return workflowrun.CreateRunCommand{}, ErrReviewTokenExpired
		}
		source, err := s.source(ctx, tx, versionID, true)
		if err != nil {
			return workflowrun.CreateRunCommand{}, err
		}
		if _, err = tx.Exec(ctx, "SELECT 1 FROM content_items WHERE id=$1 FOR UPDATE", source.Item.ID); err != nil {
			return workflowrun.CreateRunCommand{}, err
		}
		if source.Item.ProjectID != claims.ProjectID || source.Item.ID != claims.ContentItemID ||
			source.Version.Version != claims.SourceContentVersionVersion ||
			reviewContentHash(source.Version.Content) != claims.SourceContentHash ||
			strings.TrimSpace(source.Version.Title) == "" || strings.TrimSpace(source.Version.Content) == "" {
			return workflowrun.CreateRunCommand{}, ErrReviewPreflightChanged
		}
		binding, workflow, connection, dimensions, err := runnableReviewForCreate(ctx, tx, source.Item.ProjectID)
		if errors.Is(err, ErrReviewNotConfigured) {
			return workflowrun.CreateRunCommand{}, ErrReviewPreflightChanged
		}
		if err != nil {
			return workflowrun.CreateRunCommand{}, err
		}
		if binding.ID != claims.BindingID || binding.Version != claims.BindingVersion ||
			workflow.ID != claims.ConfigurationID || workflow.Version != claims.ConfigurationVersion ||
			connection.ID != claims.ConnectionID || connection.Version != claims.ConnectionVersion ||
			!sameStrings(dimensions, claims.ReviewDimensions) ||
			reviewInputDigest(source, claims.OptionalInstructions, dimensions, binding, workflow, connection) != claims.InputDigest {
			return workflowrun.CreateRunCommand{}, ErrReviewPreflightChanged
		}
		var active bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_records WHERE project_id=$1 AND stage='review' AND subject_type='content_version' AND subject_id=$2 AND status IN ('queued','running'))", source.Item.ProjectID, source.Version.ID).Scan(&active); err != nil {
			return workflowrun.CreateRunCommand{}, err
		}
		if active {
			return workflowrun.CreateRunCommand{}, ErrReviewActiveRun
		}
		sourceVersion := source.Version.Version
		if source.Version.Status == ContentVersionStatusEditableDraft {
			err = tx.QueryRow(ctx, "UPDATE content_versions SET status='frozen',frozen_at=NOW(),version=version+1,updated_at=NOW() WHERE id=$1 AND version=$2 RETURNING version", source.Version.ID, source.Version.Version).Scan(&sourceVersion)
			if errors.Is(err, pgx.ErrNoRows) {
				return workflowrun.CreateRunCommand{}, ErrReviewPreflightChanged
			}
			if err != nil {
				return workflowrun.CreateRunCommand{}, err
			}
		} else if source.Version.Status != ContentVersionStatusFrozen {
			return workflowrun.CreateRunCommand{}, ErrReviewNotReviewable
		}
		snapshot, err := workflowrun.BuildConfigurationSnapshot(binding, workflow, connection, s.now())
		if err != nil {
			return workflowrun.CreateRunCommand{}, err
		}
		runID := uuid.New()
		input := ReviewRuntimeInputV1{
			SchemaVersion: "review.input.v1", ProjectID: source.Item.ProjectID, ContentItemID: source.Item.ID,
			SourceContentVersionID: source.Version.ID, SourceContentVersionVersion: sourceVersion,
			SourceContentHash: claims.SourceContentHash, SourceTitle: source.Version.Title,
			SourceContent: source.Version.Content, OptionalInstructions: claims.OptionalInstructions,
			ReviewDimensions: dimensions, WorkflowRunID: runID, CorrelationID: runID.String(),
		}
		payload, err := json.Marshal(input)
		if err != nil {
			return workflowrun.CreateRunCommand{}, err
		}
		subjectType, subjectID := "content_version", source.Version.ID
		return workflowrun.CreateRunCommand{
			ProjectID: source.Item.ProjectID, RunID: runID, Stage: "review", SubjectType: &subjectType,
			SubjectID: &subjectID, InputPayload: payload, TriggerSource: "manual",
			PreparedConfiguration: &workflowrun.PreparedRunConfiguration{WorkflowConfigurationID: workflow.ID, Snapshot: snapshot},
		}, nil
	})
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func rejectDuplicateJSONKeys(raw json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			keys := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok || keys[key] {
					return ErrReviewOutputInvalid
				}
				keys[key] = true
				if err = walk(); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return ErrReviewOutputInvalid
			}
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return ErrReviewOutputInvalid
			}
		default:
			return ErrReviewOutputInvalid
		}
		return nil
	}
	if err := walk(); err != nil {
		return ErrReviewOutputInvalid
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return ErrReviewOutputInvalid
	}
	return nil
}

func DecodeReviewRuntimeOutput(raw json.RawMessage) (ReviewRuntimeOutputV1, error) {
	var output ReviewRuntimeOutputV1
	if len(raw) == 0 || rejectDuplicateJSONKeys(raw) != nil {
		return output, ErrReviewOutputInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&output); err != nil {
		return output, ErrReviewOutputInvalid
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return output, ErrReviewOutputInvalid
	}
	if output.SchemaVersion != "review.output.v1" ||
		(output.Conclusion != "passed" && output.Conclusion != "needs_changes") ||
		strings.TrimSpace(output.Summary) == "" || utf8.RuneCountInString(output.Summary) > 5000 ||
		output.PassedRuleCount < 0 || output.PassedRuleCount > 10000 ||
		len(output.Issues) > 200 || len(output.Recommendations) > 100 ||
		(output.Conclusion == "needs_changes" && len(output.Issues) == 0) {
		return output, ErrReviewOutputInvalid
	}
	issueKeys := map[string]bool{}
	positions := map[int]bool{}
	for _, issue := range output.Issues {
		if !validIssueKey(issue.IssueKey) || issueKeys[issue.IssueKey] ||
			issue.Position < 1 || issue.Position > 200 || positions[issue.Position] ||
			!validCategoryKey(issue.CategoryKey) ||
			strings.TrimSpace(issue.CategoryLabel) == "" || utf8.RuneCountInString(issue.CategoryLabel) > 120 ||
			!containsString([]string{"critical", "warning", "suggestion"}, issue.Severity) ||
			strings.TrimSpace(issue.Title) == "" || utf8.RuneCountInString(issue.Title) > 200 ||
			strings.TrimSpace(issue.Description) == "" || utf8.RuneCountInString(issue.Description) > 5000 ||
			(issue.Evidence.Quote != nil && utf8.RuneCountInString(*issue.Evidence.Quote) > 1000) ||
			len(issue.Evidence.SourceRefs) > 20 ||
			(issue.Suggestion != nil && utf8.RuneCountInString(*issue.Suggestion) > 5000) ||
			!validReviewLocation(issue.Location) {
			return output, ErrReviewOutputInvalid
		}
		refs := map[string]bool{}
		for _, ref := range issue.Evidence.SourceRefs {
			if strings.TrimSpace(ref) == "" || utf8.RuneCountInString(ref) > 200 || refs[ref] {
				return output, ErrReviewOutputInvalid
			}
			refs[ref] = true
		}
		issueKeys[issue.IssueKey], positions[issue.Position] = true, true
	}
	for position := 1; position <= len(output.Issues); position++ {
		if !positions[position] {
			return output, ErrReviewOutputInvalid
		}
	}
	recommendationPositions := map[int]bool{}
	for _, recommendation := range output.Recommendations {
		if recommendation.Position < 1 || recommendation.Position > 100 || recommendationPositions[recommendation.Position] ||
			!containsString([]string{"low", "medium", "high"}, recommendation.Priority) ||
			strings.TrimSpace(recommendation.Title) == "" || utf8.RuneCountInString(recommendation.Title) > 200 ||
			strings.TrimSpace(recommendation.Description) == "" || utf8.RuneCountInString(recommendation.Description) > 5000 {
			return output, ErrReviewOutputInvalid
		}
		recommendationPositions[recommendation.Position] = true
	}
	for position := 1; position <= len(output.Recommendations); position++ {
		if !recommendationPositions[position] {
			return output, ErrReviewOutputInvalid
		}
	}
	return output, nil
}

func validIssueKey(value string) bool {
	if utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > 120 {
		return false
	}
	for index, char := range value {
		if index == 0 && !isASCIIAlphaNumeric(char) {
			return false
		}
		if !isASCIIAlphaNumeric(char) && char != '.' && char != '_' && char != ':' && char != '-' {
			return false
		}
	}
	return true
}

func validCategoryKey(value string) bool {
	if len(value) < 1 || len(value) > 80 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' {
			return false
		}
	}
	return true
}

func isASCIIAlphaNumeric(char rune) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
}

func validReviewLocation(location *ReviewRuntimeLocationV1) bool {
	return location == nil || location.ParagraphStart >= 1 && location.ParagraphEnd >= location.ParagraphStart &&
		location.SentenceStart >= 1 && location.SentenceEnd >= location.SentenceStart
}

func decodeReviewRuntimeInput(raw json.RawMessage) (ReviewRuntimeInputV1, error) {
	var input ReviewRuntimeInputV1
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		input.SchemaVersion != "review.input.v1" || input.ProjectID == uuid.Nil ||
		input.ContentItemID == uuid.Nil || input.SourceContentVersionID == uuid.Nil ||
		input.SourceContentVersionVersion < 1 || len(input.SourceContentHash) != 64 ||
		strings.TrimSpace(input.SourceTitle) == "" || utf8.RuneCountInString(input.SourceTitle) > 120 ||
		utf8.RuneCountInString(input.SourceContent) > 200000 ||
		input.OptionalInstructions != nil && utf8.RuneCountInString(*input.OptionalInstructions) > 2000 ||
		!validReviewDimensions(input.ReviewDimensions) ||
		input.WorkflowRunID == uuid.Nil || strings.TrimSpace(input.CorrelationID) == "" ||
		utf8.RuneCountInString(input.CorrelationID) > 128 {
		return input, ErrReviewOutputInvalid
	}
	return input, nil
}

func validReviewDimensions(dimensions []string) bool {
	if len(dimensions) < 1 || len(dimensions) > 5 {
		return false
	}
	seen := map[string]bool{}
	for _, dimension := range dimensions {
		if seen[dimension] || !containsString(frozenReviewDimensions, dimension) {
			return false
		}
		seen[dimension] = true
	}
	return true
}

func (s *RealReviewService) ConsumeSucceededRun(ctx context.Context, run workflowrun.WorkflowRun) error {
	if run.Stage != "review" || run.Status != workflowrun.StatusSucceeded {
		return ErrReviewResultNotRetryable
	}
	output, err := DecodeReviewRuntimeOutput(run.OutputPayload)
	if err != nil {
		if eventErr := s.recordReviewFailure(ctx, run, workflowrun.EventTypeOutputValidationFailed); eventErr != nil {
			return eventErr
		}
		return ErrReviewOutputInvalid
	}
	tx, err := s.repo.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = s.consumeReviewLocked(ctx, tx, run.ID, output); err != nil {
		_ = tx.Rollback(ctx)
		if !errors.Is(err, ErrReviewResultNotRetryable) {
			if eventErr := s.recordReviewFailure(ctx, run, workflowrun.EventTypeResultConsumptionFailed); eventErr != nil {
				return eventErr
			}
			return ErrReviewResultConsumption
		}
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		if eventErr := s.recordReviewFailure(ctx, run, workflowrun.EventTypeResultConsumptionFailed); eventErr != nil {
			return eventErr
		}
		return ErrReviewResultConsumption
	}
	return nil
}

func (s *RealReviewService) consumeReviewLocked(ctx context.Context, tx pgx.Tx, runID uuid.UUID, output ReviewRuntimeOutputV1) (RealReviewResult, error) {
	run, err := workflowrun.NewPostgresRepositoryTx(tx).GetByIDForUpdate(ctx, runID)
	if err != nil {
		return RealReviewResult{}, err
	}
	if run.Stage != "review" || run.Status != workflowrun.StatusSucceeded ||
		run.SubjectType == nil || *run.SubjectType != "content_version" || run.SubjectID == nil {
		return RealReviewResult{}, ErrReviewResultNotRetryable
	}
	if existing, err := s.realReviewResultByRun(ctx, tx, run, output.PassedRuleCount); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrReviewNotFound) {
		return RealReviewResult{}, err
	}
	input, err := decodeReviewRuntimeInput(run.InputPayload)
	if err != nil || input.WorkflowRunID != run.ID || input.ProjectID != run.ProjectID ||
		input.SourceContentVersionID != *run.SubjectID {
		return RealReviewResult{}, ErrReviewResultNotRetryable
	}
	source, err := s.source(ctx, tx, input.SourceContentVersionID, true)
	if err != nil {
		return RealReviewResult{}, err
	}
	if _, err = tx.Exec(ctx, "SELECT 1 FROM content_items WHERE id=$1 FOR UPDATE", source.Item.ID); err != nil {
		return RealReviewResult{}, err
	}
	if source.Item.ID != input.ContentItemID || source.Item.ProjectID != input.ProjectID ||
		source.Version.Version != input.SourceContentVersionVersion ||
		reviewContentHash(source.Version.Content) != input.SourceContentHash {
		return RealReviewResult{}, ErrReviewResultNotRetryable
	}
	report := RealReviewReport{
		ID: uuid.New(), ContentItemID: source.Item.ID, SourceContentVersionID: source.Version.ID,
		SourceContentVersionVersion: input.SourceContentVersionVersion, SourceContentHash: input.SourceContentHash,
		WorkflowRunID: run.ID, SchemaVersion: "review.output.v1", Conclusion: output.Conclusion,
		Summary: output.Summary, PassedRuleCount: output.PassedRuleCount,
	}
	err = tx.QueryRow(ctx, "INSERT INTO review_reports(id,project_id,content_item_id,content_version_id,workflow_run_id,provider_key,status,conclusion,score,summary,schema_version,source_content_version_version,source_content_hash) VALUES($1,$2,$3,$4,$5,'runtime','completed',$6,NULL,$7,'review.output.v1',$8,$9) RETURNING created_at,completed_at",
		report.ID, source.Item.ProjectID, source.Item.ID, source.Version.ID, run.ID, output.Conclusion,
		output.Summary, input.SourceContentVersionVersion, input.SourceContentHash,
	).Scan(&report.CreatedAt, &report.CompletedAt)
	if err != nil {
		return RealReviewResult{}, err
	}
	issues := make([]RealReviewIssue, 0, len(output.Issues))
	for _, value := range output.Issues {
		evidence, _ := json.Marshal(value.Evidence)
		var location any
		if value.Location != nil {
			location, _ = json.Marshal(value.Location)
		}
		issue := RealReviewIssue{
			ID: uuid.New(), ReviewID: report.ID, IssueKey: value.IssueKey, Position: value.Position,
			CategoryKey: value.CategoryKey, CategoryLabel: value.CategoryLabel, Severity: value.Severity,
			Title: value.Title, Description: value.Description, Evidence: value.Evidence,
			Location: value.Location, Suggestion: value.Suggestion, Disposition: "open", Version: 1,
		}
		err = tx.QueryRow(ctx, "INSERT INTO review_findings(id,review_id,category,severity,title,description,location_json,sort_order,issue_key,category_label,evidence_json,suggestion,disposition,version) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'open',1) RETURNING created_at,updated_at",
			issue.ID, report.ID, issue.CategoryKey, issue.Severity, issue.Title, issue.Description,
			location, issue.Position, issue.IssueKey, issue.CategoryLabel, evidence, issue.Suggestion,
		).Scan(&issue.CreatedAt, &issue.UpdatedAt)
		if err != nil {
			return RealReviewResult{}, err
		}
		issues = append(issues, issue)
	}
	recommendations := make([]RealReviewRecommendation, 0, len(output.Recommendations))
	for _, value := range output.Recommendations {
		recommendation := RealReviewRecommendation{
			ID: uuid.New(), ReviewID: report.ID, Position: value.Position, Priority: value.Priority,
			Title: value.Title, Description: value.Description,
		}
		err = tx.QueryRow(ctx, "INSERT INTO review_recommendations(id,review_id,priority,title,description,sort_order) VALUES($1,$2,$3,$4,$5,$6) RETURNING created_at",
			recommendation.ID, report.ID, recommendation.Priority, recommendation.Title,
			recommendation.Description, recommendation.Position,
		).Scan(&recommendation.CreatedAt)
		if err != nil {
			return RealReviewResult{}, err
		}
		recommendations = append(recommendations, recommendation)
	}
	if source.Item.CurrentVersionID == source.Version.ID {
		if output.Conclusion == "passed" {
			_, err = tx.Exec(ctx, "UPDATE content_items SET status='reviewed',reviewed_at=NOW(),version=version+1,updated_at=NOW() WHERE id=$1 AND current_version_id=$2", source.Item.ID, source.Version.ID)
		} else {
			_, err = tx.Exec(ctx, "UPDATE content_items SET status='draft',reviewed_at=NULL,version=version+1,updated_at=NOW() WHERE id=$1 AND current_version_id=$2", source.Item.ID, source.Version.ID)
		}
		if err != nil {
			return RealReviewResult{}, err
		}
	}
	if _, err = tx.Exec(ctx, "INSERT INTO workflow_run_events(id,run_id,event_type,status,payload,created_at) VALUES($1,$2,'result_consumed','succeeded','{}',NOW())", uuid.New(), run.ID); err != nil {
		return RealReviewResult{}, err
	}
	return RealReviewResult{Report: report, Issues: issues, Recommendations: recommendations, WorkflowRun: run}, nil
}

func (s *RealReviewService) recordReviewFailure(ctx context.Context, run workflowrun.WorkflowRun, eventType string) error {
	tx, err := s.repo.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT 1 FROM workflow_run_records WHERE id=$1 FOR UPDATE", run.ID); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type=$2)", run.ID, eventType).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return tx.Commit(ctx)
	}
	message := "审核结果处理失败"
	if eventType == workflowrun.EventTypeOutputValidationFailed {
		message = "审核输出未通过结构校验"
	}
	payload, _ := json.Marshal(map[string]any{"code": eventType, "message": message, "correlationId": run.ID.String()})
	if _, err = tx.Exec(ctx, "INSERT INTO workflow_run_events(id,run_id,event_type,status,payload,created_at) VALUES($1,$2,$3,$4,$5,$6)", uuid.New(), run.ID, eventType, run.Status, payload, s.now().UTC()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *RealReviewService) RetryResultConsumption(ctx context.Context, runID uuid.UUID, request ReviewResultConsumptionRetryRequest) (RealReviewResult, error) {
	if runID == uuid.Nil || request.ExpectedRunVersion < 1 || strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 128 {
		return RealReviewResult{}, ErrValidation
	}
	scope := "retryReviewResultConsumption:" + runID.String()
	requestHash := workflowrun.Fingerprint(struct {
		RunID   uuid.UUID `json:"runId"`
		Expected int      `json:"expectedRunVersion"`
	}{runID, request.ExpectedRunVersion})
	tx, err := s.repo.db.Begin(ctx)
	if err != nil {
		return RealReviewResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", scope+":"+request.IdempotencyKey); err != nil {
		return RealReviewResult{}, err
	}
	idem := idempotency.NewPostgresRepositoryTx(tx)
	if record, getErr := idem.Get(ctx, scope, request.IdempotencyKey); getErr == nil {
		if record.RequestHash != requestHash {
			return RealReviewResult{}, workflowrun.ErrIdempotencyConflict
		}
		var replay struct {
			ReportID uuid.UUID `json:"reportId"`
		}
		if json.Unmarshal(record.ResponseBody, &replay) != nil {
			return RealReviewResult{}, ErrReviewResultNotRetryable
		}
		return s.realReviewResultByReport(ctx, tx, replay.ReportID)
	} else if !errors.Is(getErr, idempotency.ErrNotFound) {
		return RealReviewResult{}, getErr
	}
	run, err := workflowrun.NewPostgresRepositoryTx(tx).GetByIDForUpdate(ctx, runID)
	if err != nil {
		return RealReviewResult{}, err
	}
	if run.Version != request.ExpectedRunVersion {
		return RealReviewResult{}, workflowrun.ErrVersionConflict
	}
	output, outputErr := DecodeReviewRuntimeOutput(run.OutputPayload)
	if outputErr != nil {
		return RealReviewResult{}, ErrReviewResultNotRetryable
	}
	if existing, existingErr := s.realReviewResultByRun(ctx, tx, run, output.PassedRuleCount); existingErr == nil {
		return s.finishReviewRetry(ctx, tx, idem, scope, request.IdempotencyKey, requestHash, existing)
	} else if !errors.Is(existingErr, ErrReviewNotFound) {
		return RealReviewResult{}, existingErr
	}
	var consumptionFailed, validationFailed, consumed bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumption_failed'),EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='output_validation_failed'),EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed')", runID).Scan(&consumptionFailed, &validationFailed, &consumed)
	if err != nil {
		return RealReviewResult{}, err
	}
	if run.Stage != "review" || run.Status != workflowrun.StatusSucceeded || !consumptionFailed || validationFailed || consumed {
		return RealReviewResult{}, ErrReviewResultNotRetryable
	}
	result, err := s.consumeReviewLocked(ctx, tx, runID, output)
	if err != nil {
		_ = tx.Rollback(ctx)
		_ = s.recordReviewFailure(ctx, run, workflowrun.EventTypeResultConsumptionFailed)
		return RealReviewResult{}, ErrReviewResultConsumption
	}
	return s.finishReviewRetry(ctx, tx, idem, scope, request.IdempotencyKey, requestHash, result)
}

func (s *RealReviewService) finishReviewRetry(ctx context.Context, tx pgx.Tx, idem *idempotency.PostgresRepository, scope, key, requestHash string, result RealReviewResult) (RealReviewResult, error) {
	body, _ := json.Marshal(struct {
		ReportID uuid.UUID `json:"reportId"`
	}{result.Report.ID})
	if _, err := idem.Create(ctx, idempotency.Record{
		ID: uuid.New(), Scope: scope, Key: key, RequestHash: requestHash, ResponseStatus: 200, ResponseBody: body,
	}); err != nil {
		return RealReviewResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RealReviewResult{}, err
	}
	return result, nil
}

type reviewQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (s *RealReviewService) realReviewResultByRun(ctx context.Context, q reviewQueryer, run workflowrun.WorkflowRun, passedRuleCount int) (RealReviewResult, error) {
	var reportID uuid.UUID
	err := q.QueryRow(ctx, "SELECT id FROM review_reports WHERE workflow_run_id=$1 AND provider_key='runtime'", run.ID).Scan(&reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return RealReviewResult{}, ErrReviewNotFound
	}
	if err != nil {
		return RealReviewResult{}, err
	}
	result, err := s.realReviewResultByReport(ctx, q, reportID)
	if err == nil {
		result.WorkflowRun = run
		result.Report.PassedRuleCount = passedRuleCount
	}
	return result, err
}

func (s *RealReviewService) realReviewResultByReport(ctx context.Context, q reviewQueryer, reportID uuid.UUID) (RealReviewResult, error) {
	var result RealReviewResult
	err := q.QueryRow(ctx, "SELECT id,content_item_id,content_version_id,source_content_version_version,source_content_hash,workflow_run_id,schema_version,conclusion,summary,created_at,completed_at FROM review_reports WHERE id=$1 AND provider_key='runtime'", reportID).Scan(
		&result.Report.ID, &result.Report.ContentItemID, &result.Report.SourceContentVersionID,
		&result.Report.SourceContentVersionVersion, &result.Report.SourceContentHash,
		&result.Report.WorkflowRunID, &result.Report.SchemaVersion, &result.Report.Conclusion,
		&result.Report.Summary, &result.Report.CreatedAt, &result.Report.CompletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RealReviewResult{}, ErrReviewNotFound
	}
	if err != nil {
		return RealReviewResult{}, err
	}
	run, err := s.runs.GetRun(ctx, result.Report.WorkflowRunID)
	if err != nil {
		if tx, ok := q.(pgx.Tx); ok {
			run, err = workflowrun.NewPostgresRepositoryTx(tx).GetByID(ctx, result.Report.WorkflowRunID)
		}
	}
	if err != nil {
		return RealReviewResult{}, err
	}
	result.WorkflowRun = run
	if output, decodeErr := DecodeReviewRuntimeOutput(run.OutputPayload); decodeErr == nil {
		result.Report.PassedRuleCount = output.PassedRuleCount
	}
	result.Issues, err = queryRealReviewIssues(ctx, q, reportID)
	if err != nil {
		return RealReviewResult{}, err
	}
	result.Recommendations, err = queryRealReviewRecommendations(ctx, q, reportID)
	return result, err
}

func queryRealReviewIssues(ctx context.Context, q reviewQueryer, reportID uuid.UUID) ([]RealReviewIssue, error) {
	rows, err := q.Query(ctx, "SELECT id,review_id,issue_key,sort_order,category,category_label,severity,title,description,evidence_json,location_json,suggestion,disposition,version,ignored_at,ignored_by,created_at,updated_at FROM review_findings WHERE review_id=$1 AND issue_key IS NOT NULL ORDER BY sort_order,id", reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RealReviewIssue{}
	for rows.Next() {
		var issue RealReviewIssue
		var evidence json.RawMessage
		var location json.RawMessage
		if err = rows.Scan(
			&issue.ID, &issue.ReviewID, &issue.IssueKey, &issue.Position, &issue.CategoryKey,
			&issue.CategoryLabel, &issue.Severity, &issue.Title, &issue.Description, &evidence,
			&location, &issue.Suggestion, &issue.Disposition, &issue.Version, &issue.IgnoredAt,
			&issue.IgnoredBy, &issue.CreatedAt, &issue.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if json.Unmarshal(evidence, &issue.Evidence) != nil {
			return nil, ErrReviewOutputInvalid
		}
		if len(location) > 0 && string(location) != "null" {
			var decoded ReviewRuntimeLocationV1
			if json.Unmarshal(location, &decoded) != nil {
				return nil, ErrReviewOutputInvalid
			}
			issue.Location = &decoded
		}
		out = append(out, issue)
	}
	return out, rows.Err()
}

func queryRealReviewRecommendations(ctx context.Context, q reviewQueryer, reportID uuid.UUID) ([]RealReviewRecommendation, error) {
	rows, err := q.Query(ctx, "SELECT id,review_id,sort_order,priority,title,description,created_at FROM review_recommendations WHERE review_id=$1 ORDER BY sort_order,id", reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RealReviewRecommendation{}
	for rows.Next() {
		var recommendation RealReviewRecommendation
		if err = rows.Scan(&recommendation.ID, &recommendation.ReviewID, &recommendation.Position, &recommendation.Priority, &recommendation.Title, &recommendation.Description, &recommendation.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, recommendation)
	}
	return out, rows.Err()
}

func (s *RealReviewService) Summary(ctx context.Context, itemID uuid.UUID) (ContentReviewSummary, error) {
	if itemID == uuid.Nil {
		return ContentReviewSummary{}, ErrValidation
	}
	detail, err := s.repo.GetByID(ctx, itemID)
	if err != nil {
		return ContentReviewSummary{}, err
	}
	summary := ContentReviewSummary{ContentItemID: itemID, State: "idle"}
	binding, workflow, connection, _, configuredErr := s.runnable(ctx, detail.Item.ProjectID)
	if configuredErr == nil {
		summary.ConfigurationSummary = reviewConfigurationSummary(binding, workflow, connection)
	} else if !errors.Is(configuredErr, ErrReviewNotConfigured) {
		return ContentReviewSummary{}, configuredErr
	}
	var latestID uuid.UUID
	err = s.repo.db.QueryRow(ctx, "SELECT r.id FROM workflow_run_records r JOIN content_versions v ON v.id=r.subject_id AND v.content_item_id=$1 WHERE r.project_id=$2 AND r.stage='review' AND r.subject_type='content_version' ORDER BY r.created_at DESC,r.id DESC LIMIT 1", itemID, detail.Item.ProjectID).Scan(&latestID)
	if errors.Is(err, pgx.ErrNoRows) {
		if errors.Is(configuredErr, ErrReviewNotConfigured) {
			summary.State = "not_configured"
		}
		summary.CanStartReview = configuredErr == nil
		return summary, nil
	}
	if err != nil {
		return ContentReviewSummary{}, err
	}
	latest, err := s.runs.GetRun(ctx, latestID)
	if err != nil {
		return ContentReviewSummary{}, err
	}
	summary.LatestRun = &latest
	var activeID uuid.UUID
	activeErr := s.repo.db.QueryRow(ctx, "SELECT r.id FROM workflow_run_records r JOIN content_versions v ON v.id=r.subject_id AND v.content_item_id=$1 WHERE r.project_id=$2 AND r.stage='review' AND r.subject_type='content_version' AND r.status IN ('queued','running') ORDER BY r.created_at DESC,r.id DESC LIMIT 1", itemID, detail.Item.ProjectID).Scan(&activeID)
	if activeErr == nil {
		active, runErr := s.runs.GetRun(ctx, activeID)
		if runErr != nil {
			return ContentReviewSummary{}, runErr
		}
		summary.ActiveRun = &active
		events, eventsErr := s.runs.ListRunEvents(ctx, active.ID)
		if eventsErr != nil {
			return ContentReviewSummary{}, eventsErr
		}
		summary.State = reviewStateFromFacts(active, events)
		summary.CanStartReview = false
		return summary, nil
	}
	if !errors.Is(activeErr, pgx.ErrNoRows) {
		return ContentReviewSummary{}, activeErr
	}
	events, err := s.runs.ListRunEvents(ctx, latest.ID)
	if err != nil {
		return ContentReviewSummary{}, err
	}
	output, _ := DecodeReviewRuntimeOutput(latest.OutputPayload)
	if result, reportErr := s.realReviewResultByRun(ctx, s.repo.db, latest, output.PassedRuleCount); reportErr == nil {
		summary.LatestReport = &ReviewReportSummary{
			ID: result.Report.ID, WorkflowRunID: latest.ID, SourceContentVersionID: result.Report.SourceContentVersionID,
			Conclusion: result.Report.Conclusion, Summary: result.Report.Summary, CompletedAt: result.Report.CompletedAt,
		}
		issueSummary := ReviewIssueSummary{}
		err = s.repo.db.QueryRow(ctx, "SELECT COUNT(*),COUNT(*) FILTER(WHERE severity='critical'),COUNT(*) FILTER(WHERE severity='warning'),COUNT(*) FILTER(WHERE severity='suggestion'),COUNT(*) FILTER(WHERE disposition='open'),COUNT(*) FILTER(WHERE disposition='ignored') FROM review_findings WHERE review_id=$1", result.Report.ID).Scan(
			&issueSummary.Total, &issueSummary.Critical, &issueSummary.Warning, &issueSummary.Suggestion,
			&issueSummary.Open, &issueSummary.Ignored,
		)
		if err != nil {
			return ContentReviewSummary{}, err
		}
		summary.IssueSummary = &issueSummary
		summary.State = "review_ready"
	} else if !errors.Is(reportErr, ErrReviewNotFound) {
		return ContentReviewSummary{}, reportErr
	} else {
		summary.State = reviewStateFromFacts(latest, events)
		if summary.State == "runtime_failed" || summary.State == "output_validation_failed" || summary.State == "result_consumption_failed" {
			summary.LatestError = reviewSafeError(latest, events, summary.State)
		}
	}
	summary.CanStartReview = summary.ActiveRun == nil && (summary.State == "idle" || summary.State == "review_ready" || summary.State == "runtime_failed" || summary.State == "output_validation_failed")
	return summary, nil
}

func reviewStateFromFacts(run workflowrun.WorkflowRun, events []workflowrun.Event) string {
	if run.Status == workflowrun.StatusQueued {
		return "queued"
	}
	if run.Status == workflowrun.StatusRunning {
		return "running"
	}
	if run.Status == workflowrun.StatusFailed || run.Status == workflowrun.StatusCancelled {
		return "runtime_failed"
	}
	outputValidation, resultConsumption := false, false
	for _, event := range events {
		if event.EventType == workflowrun.EventTypeOutputValidationFailed {
			outputValidation = true
		}
		if event.EventType == workflowrun.EventTypeResultConsumptionFailed {
			resultConsumption = true
		}
	}
	if resultConsumption {
		return "result_consumption_failed"
	}
	if outputValidation {
		return "output_validation_failed"
	}
	return "idle"
}

func reviewSafeError(run workflowrun.WorkflowRun, events []workflowrun.Event, state string) *ReviewSafeError {
	message := "审核任务未完成"
	occurredAt := run.UpdatedAt
	if state == "output_validation_failed" {
		message = "审核输出未通过结构校验"
	}
	if state == "result_consumption_failed" {
		message = "审核结果未能安全保存"
	}
	if state == "runtime_failed" && run.ErrorMessage != nil {
		message = safeReviewMessage(*run.ErrorMessage, message)
	}
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].EventType == state {
			occurredAt = events[index].CreatedAt
			break
		}
	}
	return &ReviewSafeError{Code: state, Message: message, CorrelationID: run.ID.String(), AttemptCount: 1, OccurredAt: occurredAt}
}

func safeReviewMessage(value, fallback string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if value == "" || utf8.RuneCountInString(value) > 300 || strings.ContainsAny(value, "\r\n/\\") {
		return fallback
	}
	for _, forbidden := range []string{"sql", "postgres", "stack", "traceback", "token", "authorization", "credential", "cookie"} {
		if strings.Contains(lower, forbidden) {
			return fallback
		}
	}
	return value
}

func (s *RealReviewService) UpdateIssue(ctx context.Context, reviewID, issueID uuid.UUID, request ReviewIssueUpdateRequest) (RealReviewIssue, error) {
	if reviewID == uuid.Nil || issueID == uuid.Nil || request.ExpectedVersion < 1 ||
		(request.Disposition != "open" && request.Disposition != "ignored") ||
		strings.TrimSpace(request.IdempotencyKey) == "" || len(request.IdempotencyKey) > 128 ||
		strings.TrimSpace(request.ActorID) == "" {
		return RealReviewIssue{}, ErrValidation
	}
	scope := "updateReviewIssue:" + issueID.String() + ":" + strings.TrimSpace(request.ActorID)
	requestHash := workflowrun.Fingerprint(struct {
		ReviewID    uuid.UUID `json:"reviewId"`
		IssueID     uuid.UUID `json:"issueId"`
		Disposition string    `json:"disposition"`
		Expected    int       `json:"expectedVersion"`
	}{reviewID, issueID, request.Disposition, request.ExpectedVersion})
	tx, err := s.repo.db.Begin(ctx)
	if err != nil {
		return RealReviewIssue{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", scope+":"+request.IdempotencyKey); err != nil {
		return RealReviewIssue{}, err
	}
	idem := idempotency.NewPostgresRepositoryTx(tx)
	if record, getErr := idem.Get(ctx, scope, request.IdempotencyKey); getErr == nil {
		if record.RequestHash != requestHash {
			return RealReviewIssue{}, workflowrun.ErrIdempotencyConflict
		}
		var replay RealReviewIssue
		if json.Unmarshal(record.ResponseBody, &replay) != nil || replay.ID != issueID || replay.ReviewID != reviewID {
			return RealReviewIssue{}, ErrReviewIssueNotFound
		}
		return replay, nil
	} else if !errors.Is(getErr, idempotency.ErrNotFound) {
		return RealReviewIssue{}, getErr
	}
	var currentVersion int
	err = tx.QueryRow(ctx, "SELECT f.version FROM review_findings f JOIN review_reports r ON r.id=f.review_id WHERE f.id=$1 AND f.review_id=$2 AND f.issue_key IS NOT NULL AND r.provider_key='runtime' FOR UPDATE OF f", issueID, reviewID).Scan(&currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return RealReviewIssue{}, ErrReviewIssueNotFound
	}
	if err != nil {
		return RealReviewIssue{}, err
	}
	if currentVersion != request.ExpectedVersion {
		return RealReviewIssue{}, ErrReviewIssueVersion
	}
	if request.Disposition == "ignored" {
		_, err = tx.Exec(ctx, "UPDATE review_findings SET disposition='ignored',ignored_at=NOW(),ignored_by=$1,version=version+1,updated_at=NOW() WHERE id=$2 AND version=$3", request.ActorID, issueID, request.ExpectedVersion)
	} else {
		_, err = tx.Exec(ctx, "UPDATE review_findings SET disposition='open',ignored_at=NULL,ignored_by=NULL,version=version+1,updated_at=NOW() WHERE id=$1 AND version=$2", issueID, request.ExpectedVersion)
	}
	if err != nil {
		return RealReviewIssue{}, err
	}
	issue, err := queryRealReviewIssue(ctx, tx, issueID)
	if err != nil {
		return RealReviewIssue{}, err
	}
	body, _ := json.Marshal(issue)
	if _, err = idem.Create(ctx, idempotency.Record{
		ID: uuid.New(), Scope: scope, Key: request.IdempotencyKey, RequestHash: requestHash,
		ResponseStatus: 200, ResponseBody: body,
	}); err != nil {
		return RealReviewIssue{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return RealReviewIssue{}, err
	}
	return issue, nil
}

func queryRealReviewIssue(ctx context.Context, q reviewQueryer, issueID uuid.UUID) (RealReviewIssue, error) {
	var issue RealReviewIssue
	var evidence json.RawMessage
	var location json.RawMessage
	err := q.QueryRow(ctx, "SELECT id,review_id,issue_key,sort_order,category,category_label,severity,title,description,evidence_json,location_json,suggestion,disposition,version,ignored_at,ignored_by,created_at,updated_at FROM review_findings WHERE id=$1 AND issue_key IS NOT NULL", issueID).Scan(
		&issue.ID, &issue.ReviewID, &issue.IssueKey, &issue.Position, &issue.CategoryKey,
		&issue.CategoryLabel, &issue.Severity, &issue.Title, &issue.Description, &evidence,
		&location, &issue.Suggestion, &issue.Disposition, &issue.Version, &issue.IgnoredAt,
		&issue.IgnoredBy, &issue.CreatedAt, &issue.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return RealReviewIssue{}, ErrReviewIssueNotFound
	}
	if err != nil {
		return RealReviewIssue{}, err
	}
	if json.Unmarshal(evidence, &issue.Evidence) != nil {
		return RealReviewIssue{}, ErrReviewOutputInvalid
	}
	if len(location) > 0 && string(location) != "null" {
		var decoded ReviewRuntimeLocationV1
		if json.Unmarshal(location, &decoded) != nil {
			return RealReviewIssue{}, ErrReviewOutputInvalid
		}
		issue.Location = &decoded
	}
	return issue, nil
}

func (s *RealReviewService) GetRealReview(ctx context.Context, reviewID uuid.UUID) (RealReviewDetail, bool, error) {
	var provider string
	err := s.repo.db.QueryRow(ctx, "SELECT provider_key FROM review_reports WHERE id=$1", reviewID).Scan(&provider)
	if errors.Is(err, pgx.ErrNoRows) {
		return RealReviewDetail{}, false, ErrReviewNotFound
	}
	if err != nil {
		return RealReviewDetail{}, false, err
	}
	if provider != "runtime" {
		return RealReviewDetail{}, false, nil
	}
	result, err := s.realReviewResultByReport(ctx, s.repo.db, reviewID)
	if err != nil {
		return RealReviewDetail{}, true, err
	}
	source, err := s.source(ctx, s.repo.db, result.Report.SourceContentVersionID, false)
	if err != nil {
		return RealReviewDetail{}, true, err
	}
	return RealReviewDetail{
		Report: result.Report, SourceContentVersionSummary: reviewSourceSummary(source),
		Issues: result.Issues, Recommendations: result.Recommendations, WorkflowRunSummary: result.WorkflowRun,
	}, true, nil
}

type reviewHistorySortable struct {
	CreatedAt time.Time
	ID        uuid.UUID
	Value     any
}

func (s *RealReviewService) ListReviewHistory(ctx context.Context, itemID uuid.UUID, limit, offset int) (ReviewHistoryPage, error) {
	if itemID == uuid.Nil || limit < 1 || limit > 100 || offset < 0 {
		return ReviewHistoryPage{}, ErrInvalidPagination
	}
	detail, err := s.repo.GetByID(ctx, itemID)
	if err != nil {
		return ReviewHistoryPage{}, err
	}
	values := []reviewHistorySortable{}
	rows, err := s.repo.db.Query(ctx, "SELECT id,content_version_id,workflow_run_id,conclusion,score,summary,created_at FROM review_reports WHERE project_id=$1 AND content_item_id=$2 AND provider_key='mock'", detail.Item.ProjectID, itemID)
	if err != nil {
		return ReviewHistoryPage{}, err
	}
	for rows.Next() {
		var id, versionID uuid.UUID
		var runID *uuid.UUID
		var conclusion, summary string
		var score *int
		var createdAt time.Time
		if err = rows.Scan(&id, &versionID, &runID, &conclusion, &score, &summary, &createdAt); err != nil {
			rows.Close()
			return ReviewHistoryPage{}, err
		}
		value := map[string]any{
			"id": id, "content_item_id": itemID, "content_version_id": versionID,
			"provider_key": "mock", "status": "completed", "conclusion": conclusion,
			"score": score, "summary": summary, "created_at": createdAt,
		}
		values = append(values, reviewHistorySortable{CreatedAt: createdAt, ID: id, Value: value})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return ReviewHistoryPage{}, err
	}
	rows, err = s.repo.db.Query(ctx, "SELECT r.id FROM workflow_run_records r JOIN content_versions v ON v.id=r.subject_id AND v.content_item_id=$1 WHERE r.project_id=$2 AND r.stage='review' AND r.subject_type='content_version' ORDER BY r.created_at DESC,r.id DESC", itemID, detail.Item.ProjectID)
	if err != nil {
		return ReviewHistoryPage{}, err
	}
	for rows.Next() {
		var runID uuid.UUID
		if err = rows.Scan(&runID); err != nil {
			rows.Close()
			return ReviewHistoryPage{}, err
		}
		run, runErr := s.runs.GetRun(ctx, runID)
		if runErr != nil {
			rows.Close()
			return ReviewHistoryPage{}, runErr
		}
		source, sourceErr := s.source(ctx, s.repo.db, *run.SubjectID, false)
		if sourceErr != nil {
			rows.Close()
			return ReviewHistoryPage{}, sourceErr
		}
		var reportSummary *ReviewReportSummary
		events, eventsErr := s.runs.ListRunEvents(ctx, run.ID)
		if eventsErr != nil {
			rows.Close()
			return ReviewHistoryPage{}, eventsErr
		}
		output, _ := DecodeReviewRuntimeOutput(run.OutputPayload)
		if result, reportErr := s.realReviewResultByRun(ctx, s.repo.db, run, output.PassedRuleCount); reportErr == nil {
			reportSummary = &ReviewReportSummary{
				ID: result.Report.ID, WorkflowRunID: run.ID, SourceContentVersionID: result.Report.SourceContentVersionID,
				Conclusion: result.Report.Conclusion, Summary: result.Report.Summary, CompletedAt: result.Report.CompletedAt,
			}
		} else if !errors.Is(reportErr, ErrReviewNotFound) {
			rows.Close()
			return ReviewHistoryPage{}, reportErr
		}
		state := reviewStateFromFacts(run, events)
		if reportSummary != nil {
			state = "review_ready"
		}
		var latestError *ReviewSafeError
		if state == "runtime_failed" || state == "output_validation_failed" || state == "result_consumption_failed" {
			latestError = reviewSafeError(run, events, state)
		}
		value := ReviewHistoryItem{
			WorkflowRun: run, SourceContentVersionSummary: reviewSourceSummary(source),
			ReportSummary: reportSummary, State: state, LatestError: latestError,
		}
		values = append(values, reviewHistorySortable{CreatedAt: run.CreatedAt, ID: run.ID, Value: value})
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return ReviewHistoryPage{}, err
	}
	sort.Slice(values, func(left, right int) bool {
		if values[left].CreatedAt.Equal(values[right].CreatedAt) {
			return values[left].ID.String() > values[right].ID.String()
		}
		return values[left].CreatedAt.After(values[right].CreatedAt)
	})
	page := ReviewHistoryPage{Items: []any{}, Total: len(values), Limit: limit, Offset: offset}
	if offset >= len(values) {
		return page, nil
	}
	end := offset + limit
	if end > len(values) {
		end = len(values)
	}
	for _, value := range values[offset:end] {
		page.Items = append(page.Items, value.Value)
	}
	return page, nil
}

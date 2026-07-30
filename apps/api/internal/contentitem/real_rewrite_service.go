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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

const RewritePreflightTokenMaxLength = 32768

var (
	ErrRewriteNotAvailable     = errors.New("rewrite is not available")
	ErrRewriteNotConfigured    = errors.New("rewrite workflow is not configured")
	ErrRewritePreflightExpired = errors.New("rewrite preflight token expired")
	ErrRewritePreflightStale   = errors.New("rewrite preflight input changed")
	ErrRewritePreflightConsumed = workflowrun.ErrPreflightTokenConsumed
	ErrRewriteTokenInvalid     = errors.New("rewrite preflight token is invalid")
	ErrRewriteActiveRun        = errors.New("rewrite run is already active")
)

type RewriteOptions struct {
	Strategy string `json:"strategy"`
}

type RewritePreflightRequest struct {
	SelectedIssueIDs    []uuid.UUID
	OptionalInstructions *string
	RewriteOptions      RewriteOptions
	ActorID             string
}

type RewriteCheck struct {
	Code    string `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type RewriteSourceVersionSummary struct {
	ID            uuid.UUID `json:"id"`
	ContentItemID uuid.UUID `json:"contentItemId"`
	VersionNo     int       `json:"versionNo"`
	Version       int       `json:"version"`
	Title         string    `json:"title"`
	WordCount     int       `json:"wordCount"`
	ContentHash   string    `json:"contentHash"`
}

type RewriteReportSnapshot struct {
	ReviewReportID             uuid.UUID `json:"reviewReportId"`
	SourceContentVersionID     uuid.UUID `json:"sourceContentVersionId"`
	SourceContentVersionVersion int      `json:"sourceContentVersionVersion"`
	SourceContentHash          string    `json:"sourceContentHash"`
	Conclusion                string    `json:"conclusion"`
	Summary                   string    `json:"summary"`
	CompletedAt               time.Time `json:"completedAt"`
}

type RewriteConfigurationSummary struct {
	BindingVersion               int       `json:"bindingVersion"`
	WorkflowConfigurationID      uuid.UUID `json:"workflowConfigurationId"`
	WorkflowConfigurationName    string    `json:"workflowConfigurationName"`
	WorkflowConfigurationVersion int       `json:"workflowConfigurationVersion"`
	ConnectionID                 uuid.UUID `json:"connectionId"`
	ConnectionVersion            int       `json:"connectionVersion"`
	InputContract                string    `json:"inputContract"`
	OutputContract               string    `json:"outputContract"`
}

type RewriteIssueSnapshot struct {
	ReviewIssueID uuid.UUID                `json:"reviewIssueId"`
	ReviewReportID uuid.UUID               `json:"reviewReportId"`
	IssueKey      string                   `json:"issueKey"`
	Position      int                      `json:"position"`
	Version       int                      `json:"version"`
	CategoryKey   string                   `json:"categoryKey"`
	CategoryLabel string                   `json:"categoryLabel"`
	Severity      string                   `json:"severity"`
	Title         string                   `json:"title"`
	Description   string                   `json:"description"`
	Evidence      ReviewRuntimeEvidenceV1  `json:"evidence"`
	Location      *ReviewRuntimeLocationV1 `json:"location"`
	Suggestion    *string                  `json:"suggestion"`
	Disposition   string                   `json:"disposition"`
}

type RewriteSelectedIssueSummary struct {
	Total int                    `json:"total"`
	Items []RewriteIssueSnapshot `json:"items"`
}

type RewriteAvailability struct {
	ReviewReportID             uuid.UUID                    `json:"reviewReportId"`
	ContentItemID              uuid.UUID                    `json:"contentItemId"`
	SourceContentVersionSummary RewriteSourceVersionSummary `json:"sourceContentVersionSummary"`
	Available                  bool                         `json:"available"`
	Reason                     *string                      `json:"reason"`
	OpenIssueCount             int                          `json:"openIssueCount"`
	ActiveRun                  *workflowrun.WorkflowRun     `json:"activeRun"`
	ConfigurationSummary       *RewriteConfigurationSummary `json:"configurationSummary"`
}

type RewritePreflightResult struct {
	Status                      string                       `json:"status"`
	Checks                      []RewriteCheck               `json:"checks"`
	ReviewReportSnapshot        RewriteReportSnapshot        `json:"reviewReportSnapshot"`
	SourceContentVersionSummary RewriteSourceVersionSummary  `json:"sourceContentVersionSummary"`
	SelectedIssueSummary        RewriteSelectedIssueSummary  `json:"selectedIssueSummary"`
	RewriteOptions              RewriteOptions               `json:"rewriteOptions"`
	OptionalInstructions        *string                      `json:"optionalInstructions"`
	ConfigurationSummary        *RewriteConfigurationSummary `json:"configurationSummary"`
	PreflightToken              *string                      `json:"preflightToken"`
	ExpiresAt                   *time.Time                    `json:"expiresAt"`
}

type RewriteRuntimeInputV1 struct {
	SchemaVersion               string                 `json:"schemaVersion"`
	WorkflowRunID               uuid.UUID              `json:"workflowRunId"`
	CorrelationID               string                 `json:"correlationId"`
	ProjectID                   uuid.UUID              `json:"projectId"`
	ContentItemID               uuid.UUID              `json:"contentItemId"`
	SourceContentVersionID      uuid.UUID              `json:"sourceContentVersionId"`
	SourceContentVersionVersion int                    `json:"sourceContentVersionVersion"`
	SourceContentHash           string                 `json:"sourceContentHash"`
	SourceTitle                 string                 `json:"sourceTitle"`
	SourceContent               string                 `json:"sourceContent"`
	ReviewReportID              uuid.UUID              `json:"reviewReportId"`
	ReportSnapshot              RewriteReportSnapshot  `json:"reportSnapshot"`
	SelectedIssues              []RewriteIssueSnapshot `json:"selectedIssues"`
	OptionalInstructions        *string                `json:"optionalInstructions"`
	RewriteOptions              RewriteOptions         `json:"rewriteOptions"`
}

type rewriteRunService interface {
	CreateRunForPreflightTokenIdempotentForScopeWithReplay(context.Context, string, uuid.UUID, string, string, string, workflowrun.CreateRunTxPreparation) (workflowrun.WorkflowRun, bool, error)
	ListRuns(context.Context, workflowrun.ListRunsQuery) (workflowrun.RunList, error)
}

type RealRewriteService struct {
	repo     *PostgresRepository
	bindings interface {
		GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error)
	}
	configs interface {
		GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error)
		GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error)
	}
	runs   rewriteRunService
	secret []byte
	now    func() time.Time
	begin  func(context.Context) (pgx.Tx, error)
}

func NewRealRewriteService(
	repo *PostgresRepository,
	bindings interface {
		GetByProjectAndStage(context.Context, uuid.UUID, workflowbinding.WorkflowBindingStage) (workflowbinding.ProjectWorkflowBinding, error)
	},
	configs interface {
		GetWorkflow(context.Context, uuid.UUID) (globalconfig.Workflow, error)
		GetConnection(context.Context, uuid.UUID) (globalconfig.Connection, error)
	},
	runs rewriteRunService,
	secret string,
) *RealRewriteService {
	return &RealRewriteService{
		repo: repo, bindings: bindings, configs: configs, runs: runs,
		secret: []byte(secret), now: time.Now, begin: repo.db.Begin,
	}
}

type rewriteReportFacts struct {
	ID                          uuid.UUID
	ProjectID                   uuid.UUID
	ContentItemID               uuid.UUID
	SourceContentVersionID      uuid.UUID
	SourceContentVersionVersion int
	SourceContentHash           string
	ProviderKey                 string
	Status                      string
	SchemaVersion               *string
	Conclusion                  string
	Summary                     string
	CompletedAt                 *time.Time
	Source                      reviewSource
}

func (s *RealRewriteService) source(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, versionID uuid.UUID, lock bool) (reviewSource, error) {
	suffix := ""
	if lock { suffix = " FOR UPDATE OF v" }
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
	if errors.Is(err, pgx.ErrNoRows) { return reviewSource{}, ErrContentVersionNotFound }
	return out, err
}

func (s *RealRewriteService) report(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, reportID uuid.UUID, lock bool) (rewriteReportFacts, error) {
	var out rewriteReportFacts
	var schemaVersion *string
	var completedAt *time.Time
	suffix := ""
	if lock { suffix = " FOR UPDATE" }
	err := q.QueryRow(ctx, "SELECT id,project_id,content_item_id,content_version_id,source_content_version_version,source_content_hash,provider_key,status,schema_version,conclusion,summary,completed_at FROM review_reports WHERE id=$1"+suffix, reportID).Scan(
		&out.ID, &out.ProjectID, &out.ContentItemID, &out.SourceContentVersionID,
		&out.SourceContentVersionVersion, &out.SourceContentHash, &out.ProviderKey,
		&out.Status, &schemaVersion, &out.Conclusion, &out.Summary, &completedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return out, ErrReviewNotFound }
	if err != nil { return out, err }
	out.SchemaVersion, out.CompletedAt = schemaVersion, completedAt
	source, err := s.source(ctx, q, out.SourceContentVersionID, false)
	if errors.Is(err, ErrContentVersionNotFound) { return out, ErrReviewNotFound }
	if err != nil { return out, err }
	out.Source = source
	return out, nil
}

func validRewriteReport(report rewriteReportFacts) bool {
	return report.ID != uuid.Nil && report.ProviderKey == "runtime" && report.Status == "completed" &&
		report.SchemaVersion != nil && *report.SchemaVersion == "review.output.v1" && report.CompletedAt != nil &&
		report.ProjectID == report.Source.Item.ProjectID && report.ContentItemID == report.Source.Item.ID &&
		report.SourceContentVersionID == report.Source.Version.ID &&
		report.SourceContentVersionVersion == report.Source.Version.Version &&
		report.SourceContentHash == reviewContentHash(report.Source.Version.Content) &&
		report.Source.Version.Status == ContentVersionStatusFrozen && report.Source.Version.FrozenAt != nil &&
		(report.Conclusion == "passed" || report.Conclusion == "needs_changes") &&
		utf8.RuneCountInString(strings.TrimSpace(report.Source.Version.Title)) >= 1 &&
		utf8.RuneCountInString(report.Source.Version.Title) <= 120 &&
		utf8.RuneCountInString(strings.TrimSpace(report.Source.Version.Content)) >= 1 &&
		utf8.RuneCountInString(report.Source.Version.Content) <= 200000 &&
		utf8.RuneCountInString(strings.TrimSpace(report.Summary)) >= 1 &&
		utf8.RuneCountInString(report.Summary) <= 5000
}

func rewriteSourceSummary(report rewriteReportFacts) RewriteSourceVersionSummary {
	return RewriteSourceVersionSummary{
		ID: report.Source.Version.ID, ContentItemID: report.Source.Item.ID,
		VersionNo: report.Source.Version.VersionNo, Version: report.Source.Version.Version,
		Title: report.Source.Version.Title, WordCount: report.Source.Version.WordCount,
		ContentHash: reviewContentHash(report.Source.Version.Content),
	}
}

func rewriteReportSnapshot(report rewriteReportFacts) RewriteReportSnapshot {
	completedAt := time.Time{}
	if report.CompletedAt != nil { completedAt = report.CompletedAt.UTC() }
	return RewriteReportSnapshot{
		ReviewReportID: report.ID, SourceContentVersionID: report.SourceContentVersionID,
		SourceContentVersionVersion: report.SourceContentVersionVersion,
		SourceContentHash: report.SourceContentHash, Conclusion: report.Conclusion,
		Summary: report.Summary, CompletedAt: completedAt,
	}
}

func rewriteIssueSnapshot(issue RealReviewIssue) RewriteIssueSnapshot {
	return RewriteIssueSnapshot{
		ReviewIssueID: issue.ID, ReviewReportID: issue.ReviewID, IssueKey: issue.IssueKey,
		Position: issue.Position, Version: issue.Version, CategoryKey: issue.CategoryKey,
		CategoryLabel: issue.CategoryLabel, Severity: issue.Severity, Title: issue.Title,
		Description: issue.Description, Evidence: issue.Evidence, Location: issue.Location,
		Suggestion: issue.Suggestion, Disposition: issue.Disposition,
	}
}

func selectedRewriteIssues(ctx context.Context, q reviewQueryer, reportID uuid.UUID, ids []uuid.UUID, lock bool) ([]RewriteIssueSnapshot, error) {
	query := "SELECT id,review_id,issue_key,sort_order,category,category_label,severity,title,description,evidence_json,location_json,suggestion,disposition,version,ignored_at,ignored_by,created_at,updated_at FROM review_findings WHERE review_id=$1 AND id=ANY($2) AND issue_key IS NOT NULL ORDER BY sort_order,id"
	if lock { query += " FOR UPDATE" }
	rows, err := q.Query(ctx, query, reportID, ids)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []RewriteIssueSnapshot{}
	for rows.Next() {
		var issue RealReviewIssue
		var evidence json.RawMessage
		var location json.RawMessage
		err = rows.Scan(
			&issue.ID, &issue.ReviewID, &issue.IssueKey, &issue.Position, &issue.CategoryKey,
			&issue.CategoryLabel, &issue.Severity, &issue.Title, &issue.Description, &evidence,
			&location, &issue.Suggestion, &issue.Disposition, &issue.Version, &issue.IgnoredAt,
			&issue.IgnoredBy, &issue.CreatedAt, &issue.UpdatedAt,
		)
		if err != nil { return nil, err }
		if json.Unmarshal(evidence, &issue.Evidence) != nil { return nil, ErrRewriteNotAvailable }
		if len(location) > 0 && string(location) != "null" {
			var decoded ReviewRuntimeLocationV1
			if json.Unmarshal(location, &decoded) != nil { return nil, ErrRewriteNotAvailable }
			issue.Location = &decoded
		}
		out = append(out, rewriteIssueSnapshot(issue))
	}
	if err = rows.Err(); err != nil { return nil, err }
	if len(out) != len(ids) { return nil, ErrReviewIssueNotFound }
	return out, nil
}

func uniqueRewriteIssueIDs(ids []uuid.UUID) bool {
	if len(ids) < 1 || len(ids) > 50 { return false }
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if id == uuid.Nil || seen[id] { return false }
		seen[id] = true
	}
	return true
}

func normalizeRewriteInstructions(value *string) *string {
	if value == nil { return nil }
	if strings.TrimSpace(*value) == "" { return nil }
	copyValue := *value
	return &copyValue
}

func forbiddenRewriteInstructions(value *string) bool {
	if value == nil { return false }
	return containsForbiddenRewriteMaterial(*value)
}

var forbiddenRewriteMaterialPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?im)^\s*(authorization|cookie|set-cookie)\s*:`),
	regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`(?i)\b(password|token|api[_-]?key|access[_-]?key|private[_-]?key|secret)\s*[:=]\s*["']?[^\s"',;]{4,}`),
	regexp.MustCompile(`(?i)\b(postgres(?:ql)?|mysql|mongodb)://[^\s]+|\bjdbc:[^\s]+`),
	regexp.MustCompile(`(?i)https?://(?:localhost|internal(?:[.-][a-z0-9-]+)*|127(?:\.\d{1,3}){3}|0\.0\.0\.0|10(?:\.\d{1,3}){3}|192\.168(?:\.\d{1,3}){2}|172\.(?:1[6-9]|2\d|3[01])(?:\.\d{1,3}){2}|[^/\s]+\.(?:internal|local))(?:[:/][^\s]*)?`),
	regexp.MustCompile(`(?i)\b(select\s+.+\s+from|insert\s+into|update\s+\S+\s+set|delete\s+from|drop\s+table|alter\s+table|create\s+table|truncate\s+table)\b`),
	regexp.MustCompile(`(?i)\b(sqlstate|stack\s+trace|traceback|panic:\s|goroutine\s+\d+\s+\[)`),
}

func containsForbiddenRewriteMaterial(value string) bool {
	for _, pattern := range forbiddenRewriteMaterialPatterns {
		if pattern.MatchString(value) { return true }
	}
	return false
}

func validRewriteRequest(request RewritePreflightRequest) bool {
	return uniqueRewriteIssueIDs(request.SelectedIssueIDs) &&
		(request.RewriteOptions.Strategy == "targeted_fix" || request.RewriteOptions.Strategy == "creative_rewrite") &&
		strings.TrimSpace(request.ActorID) != "" &&
		(request.OptionalInstructions == nil || utf8.RuneCountInString(*request.OptionalInstructions) <= 2000) &&
		!forbiddenRewriteInstructions(request.OptionalInstructions)
}

func (s *RealRewriteService) runnable(ctx context.Context, projectID uuid.UUID) (workflowbinding.ProjectWorkflowBinding, globalconfig.Workflow, globalconfig.Connection, error) {
	binding, err := s.bindings.GetByProjectAndStage(ctx, projectID, workflowbinding.StageRewrite)
	if errors.Is(err, workflowbinding.ErrNotFound) { return binding, globalconfig.Workflow{}, globalconfig.Connection{}, ErrRewriteNotConfigured }
	if err != nil { return binding, globalconfig.Workflow{}, globalconfig.Connection{}, err }
	workflow, err := s.configs.GetWorkflow(ctx, binding.WorkflowConfigurationID)
	if errors.Is(err, globalconfig.ErrNotFound) { return binding, workflow, globalconfig.Connection{}, ErrRewriteNotConfigured }
	if err != nil { return binding, workflow, globalconfig.Connection{}, err }
	connection, err := s.configs.GetConnection(ctx, workflow.ConnectionID)
	if errors.Is(err, globalconfig.ErrNotFound) { return binding, workflow, connection, ErrRewriteNotConfigured }
	if err != nil { return binding, workflow, connection, err }
	if !workflow.Enabled || workflow.IntegrationStatus != "connected" ||
		!connection.Enabled || connection.IntegrationStatus != "connected" ||
		!containsString(workflow.ApplicableStages, "rewrite") ||
		workflow.InputContractVersion != "rewrite.input.v1" ||
		workflow.OutputContractVersion != "rewrite.output.v1" {
		return binding, workflow, connection, ErrRewriteNotConfigured
	}
	return binding, workflow, connection, nil
}

func runnableRewriteForCreate(ctx context.Context, tx pgx.Tx, projectID uuid.UUID) (workflowbinding.ProjectWorkflowBinding, globalconfig.Workflow, globalconfig.Connection, error) {
	var id, bindingProjectID, configurationID uuid.UUID
	var stage workflowbinding.WorkflowBindingStage
	var version int
	var createdAt, updatedAt time.Time
	err := tx.QueryRow(ctx, "SELECT id,project_id,stage,workflow_configuration_id,version,created_at,updated_at FROM project_workflow_bindings WHERE project_id=$1 AND stage='rewrite' FOR SHARE", projectID).Scan(
		&id, &bindingProjectID, &stage, &configurationID, &version, &createdAt, &updatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) { return workflowbinding.ProjectWorkflowBinding{}, globalconfig.Workflow{}, globalconfig.Connection{}, ErrRewriteNotConfigured }
	if err != nil { return workflowbinding.ProjectWorkflowBinding{}, globalconfig.Workflow{}, globalconfig.Connection{}, err }
	binding, err := workflowbinding.NewFromDB(id, bindingProjectID, configurationID, stage, version, createdAt, updatedAt)
	if err != nil { return binding, globalconfig.Workflow{}, globalconfig.Connection{}, err }
	workflow, err := globalconfig.GetWorkflowForShare(ctx, tx, configurationID)
	if errors.Is(err, globalconfig.ErrNotFound) { return binding, workflow, globalconfig.Connection{}, ErrRewriteNotConfigured }
	if err != nil { return binding, workflow, globalconfig.Connection{}, err }
	connection, err := globalconfig.GetConnectionForShare(ctx, tx, workflow.ConnectionID)
	if errors.Is(err, globalconfig.ErrNotFound) { return binding, workflow, connection, ErrRewriteNotConfigured }
	if err != nil { return binding, workflow, connection, err }
	if !workflow.Enabled || workflow.IntegrationStatus != "connected" ||
		!connection.Enabled || connection.IntegrationStatus != "connected" ||
		!containsString(workflow.ApplicableStages, "rewrite") ||
		workflow.InputContractVersion != "rewrite.input.v1" ||
		workflow.OutputContractVersion != "rewrite.output.v1" {
		return binding, workflow, connection, ErrRewriteNotConfigured
	}
	return binding, workflow, connection, nil
}

func rewriteConfigurationSummary(binding workflowbinding.ProjectWorkflowBinding, workflow globalconfig.Workflow, connection globalconfig.Connection) *RewriteConfigurationSummary {
	return &RewriteConfigurationSummary{
		BindingVersion: binding.Version, WorkflowConfigurationID: workflow.ID,
		WorkflowConfigurationName: workflow.Name, WorkflowConfigurationVersion: workflow.Version,
		ConnectionID: connection.ID, ConnectionVersion: connection.Version,
		InputContract: "rewrite.input.v1", OutputContract: "rewrite.output.v1",
	}
}

func (s *RealRewriteService) activeRun(ctx context.Context, projectID, reportID uuid.UUID) (*workflowrun.WorkflowRun, error) {
	subjectType := "review_report"
	list, err := s.runs.ListRuns(ctx, workflowrun.ListRunsQuery{ListFilter: workflowrun.ListFilter{
		ProjectID: &projectID, Stage: "rewrite", SubjectType: &subjectType,
		SubjectID: &reportID, Limit: 2,
	}})
	if err != nil { return nil, err }
	for _, run := range list.Items {
		if run.Status == workflowrun.StatusQueued || run.Status == workflowrun.StatusRunning {
			copyRun := run
			return &copyRun, nil
		}
	}
	return nil, nil
}

func (s *RealRewriteService) Availability(ctx context.Context, reportID uuid.UUID) (RewriteAvailability, error) {
	if reportID == uuid.Nil { return RewriteAvailability{}, ErrValidation }
	report, err := s.report(ctx, s.repo.db, reportID, false)
	if err != nil { return RewriteAvailability{}, err }
	result := RewriteAvailability{
		ReviewReportID: report.ID, ContentItemID: report.ContentItemID,
		SourceContentVersionSummary: rewriteSourceSummary(report),
	}
	if report.ProviderKey != "runtime" || report.Status != "completed" || report.SchemaVersion == nil ||
		*report.SchemaVersion != "review.output.v1" || report.CompletedAt == nil {
		reason := "review_not_completed"
		result.Reason = &reason
		return result, nil
	}
	if !validRewriteReport(report) {
		return RewriteAvailability{}, ErrReviewNotFound
	}
	err = s.repo.db.QueryRow(ctx, "SELECT COUNT(*) FROM review_findings WHERE review_id=$1 AND issue_key IS NOT NULL AND disposition='open'", reportID).Scan(&result.OpenIssueCount)
	if err != nil { return result, err }
	result.ActiveRun, err = s.activeRun(ctx, report.ProjectID, report.ID)
	if err != nil { return result, err }
	if result.ActiveRun != nil {
		reason := "active_rewrite_run_conflict"
		result.Reason = &reason
		return result, nil
	}
	if result.OpenIssueCount == 0 {
		reason := "no_open_issues"
		result.Reason = &reason
		return result, nil
	}
	binding, workflow, connection, err := s.runnable(ctx, report.ProjectID)
	if errors.Is(err, ErrRewriteNotConfigured) {
		reason := "rewrite_not_configured"
		result.Reason = &reason
		return result, nil
	}
	if err != nil { return result, err }
	result.ConfigurationSummary = rewriteConfigurationSummary(binding, workflow, connection)
	result.Available = true
	return result, nil
}

type rewriteTokenClaims struct {
	ActorID                     string         `json:"actorId"`
	ProjectID                   uuid.UUID      `json:"projectId"`
	ContentItemID               uuid.UUID      `json:"contentItemId"`
	ReviewReportID              uuid.UUID      `json:"reviewReportId"`
	SourceContentVersionID      uuid.UUID      `json:"sourceContentVersionId"`
	SourceContentVersionVersion int            `json:"sourceContentVersionVersion"`
	SourceContentHash           string         `json:"sourceContentHash"`
	SelectedIssueIDs            []uuid.UUID    `json:"selectedIssueIds"`
	SelectedIssuesDigest        string         `json:"selectedIssuesDigest"`
	ReportDigest                string         `json:"reportDigest"`
	OptionalInstructions        *string        `json:"optionalInstructions"`
	RewriteOptions              RewriteOptions `json:"rewriteOptions"`
	BindingID                   uuid.UUID      `json:"bindingId"`
	BindingVersion              int            `json:"bindingVersion"`
	ConfigurationID             uuid.UUID      `json:"configurationId"`
	ConfigurationVersion        int            `json:"configurationVersion"`
	ConnectionID                uuid.UUID      `json:"connectionId"`
	ConnectionVersion           int            `json:"connectionVersion"`
	Stage                       string         `json:"stage"`
	InputContract               string         `json:"inputContract"`
	OutputContract              string         `json:"outputContract"`
	RequestDigest               string         `json:"requestDigest"`
	Nonce                       string         `json:"nonce"`
	IssuedAt                    int64          `json:"iat"`
	ExpiresAt                   int64          `json:"exp"`
}

func digestRewriteValue(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func rewriteRequestDigest(actorID string, report rewriteReportFacts, issues []RewriteIssueSnapshot, instructions *string, options RewriteOptions, binding workflowbinding.ProjectWorkflowBinding, workflow globalconfig.Workflow, connection globalconfig.Connection) string {
	return digestRewriteValue(struct {
		ActorID             string                   `json:"actorId"`
		ProjectID           uuid.UUID                `json:"projectId"`
		ContentItemID       uuid.UUID                `json:"contentItemId"`
		Source              RewriteSourceVersionSummary `json:"source"`
		Report              RewriteReportSnapshot    `json:"report"`
		Issues              []RewriteIssueSnapshot   `json:"issues"`
		OptionalInstructions *string                 `json:"optionalInstructions"`
		RewriteOptions      RewriteOptions            `json:"rewriteOptions"`
		BindingID           uuid.UUID                 `json:"bindingId"`
		BindingVersion      int                       `json:"bindingVersion"`
		ConfigurationID     uuid.UUID                 `json:"configurationId"`
		ConfigurationVersion int                      `json:"configurationVersion"`
		ConnectionID        uuid.UUID                 `json:"connectionId"`
		ConnectionVersion   int                       `json:"connectionVersion"`
		InputContract       string                    `json:"inputContract"`
		OutputContract      string                    `json:"outputContract"`
	}{
		actorID, report.ProjectID, report.ContentItemID, rewriteSourceSummary(report),
		rewriteReportSnapshot(report), issues, instructions, options, binding.ID, binding.Version,
		workflow.ID, workflow.Version, connection.ID, connection.Version,
		"rewrite.input.v1", "rewrite.output.v1",
	})
}

func (s *RealRewriteService) signRewriteToken(claims rewriteTokenClaims) (string, error) {
	if len(s.secret) == 0 || claims.ProjectID == uuid.Nil || claims.ReviewReportID == uuid.Nil ||
		claims.Stage != "rewrite" || claims.InputContract != "rewrite.input.v1" ||
		claims.OutputContract != "rewrite.output.v1" || claims.ExpiresAt != claims.IssuedAt+600 {
		return "", ErrRewriteTokenInvalid
	}
	payload, err := json.Marshal(claims)
	if err != nil { return "", err }
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *RealRewriteService) parseRewriteToken(raw string) (rewriteTokenClaims, error) {
	var claims rewriteTokenClaims
	if len(raw) == 0 || len(raw) > RewritePreflightTokenMaxLength { return claims, ErrRewriteTokenInvalid }
	parts := strings.Split(raw, ".")
	if len(parts) != 2 || len(s.secret) == 0 { return claims, ErrRewriteTokenInvalid }
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil { return claims, ErrRewriteTokenInvalid }
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil { return claims, ErrRewriteTokenInvalid }
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) { return claims, ErrRewriteTokenInvalid }
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&claims) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return claims, ErrRewriteTokenInvalid
	}
	if claims.ProjectID == uuid.Nil || claims.ContentItemID == uuid.Nil || claims.ReviewReportID == uuid.Nil ||
		claims.SourceContentVersionID == uuid.Nil || claims.SourceContentVersionVersion < 1 ||
		strings.TrimSpace(claims.ActorID) == "" || !uniqueRewriteIssueIDs(claims.SelectedIssueIDs) ||
		len(claims.SourceContentHash) != 64 || len(claims.SelectedIssuesDigest) != 64 ||
		len(claims.ReportDigest) != 64 || len(claims.RequestDigest) != 64 ||
		claims.BindingID == uuid.Nil || claims.BindingVersion < 1 ||
		claims.ConfigurationID == uuid.Nil || claims.ConfigurationVersion < 1 ||
		claims.ConnectionID == uuid.Nil || claims.ConnectionVersion < 1 ||
		claims.Stage != "rewrite" || claims.InputContract != "rewrite.input.v1" ||
		claims.OutputContract != "rewrite.output.v1" || strings.TrimSpace(claims.Nonce) == "" ||
		claims.ExpiresAt != claims.IssuedAt+600 {
		return claims, ErrRewriteTokenInvalid
	}
	return claims, nil
}

func (s *RealRewriteService) Preflight(ctx context.Context, reportID uuid.UUID, request RewritePreflightRequest) (RewritePreflightResult, error) {
	request.OptionalInstructions = normalizeRewriteInstructions(request.OptionalInstructions)
	if reportID == uuid.Nil || !validRewriteRequest(request) {
		return RewritePreflightResult{}, ErrRewriteNotAvailable
	}
	report, err := s.report(ctx, s.repo.db, reportID, false)
	if err != nil { return RewritePreflightResult{}, err }
	if !validRewriteReport(report) { return RewritePreflightResult{}, ErrRewriteNotAvailable }
	issues, err := selectedRewriteIssues(ctx, s.repo.db, reportID, request.SelectedIssueIDs, false)
	if err != nil { return RewritePreflightResult{}, err }
	for _, issue := range issues {
		if issue.Disposition != "open" { return RewritePreflightResult{}, ErrRewriteNotAvailable }
	}
	result := RewritePreflightResult{
		Status: "blocked", Checks: []RewriteCheck{},
		ReviewReportSnapshot: rewriteReportSnapshot(report),
		SourceContentVersionSummary: rewriteSourceSummary(report),
		SelectedIssueSummary: RewriteSelectedIssueSummary{Total: len(issues), Items: issues},
		RewriteOptions: request.RewriteOptions, OptionalInstructions: request.OptionalInstructions,
	}
	add := func(code, status, message string) {
		result.Checks = append(result.Checks, RewriteCheck{Code: code, Status: status, Message: message})
	}
	add("review_report_ready", "passed", "审核报告可用于重写")
	add("source_version_fixed", "passed", "来源正文版本已固定")
	add("selected_issues_valid", "passed", "所选审核问题有效")
	binding, workflow, connection, err := s.runnable(ctx, report.ProjectID)
	if errors.Is(err, ErrRewriteNotConfigured) {
		add("project_binding_available", "blocked", "项目重写工作流尚未配置")
		add("workflow_configuration_available", "blocked", "重写工作流配置不可用")
		add("workflow_connection_available", "blocked", "重写工作流连接不可用")
		return result, nil
	}
	if err != nil { return result, err }
	result.ConfigurationSummary = rewriteConfigurationSummary(binding, workflow, connection)
	add("project_binding_available", "passed", "项目重写工作流绑定可用")
	add("workflow_configuration_available", "passed", "重写工作流配置可用")
	add("workflow_connection_available", "passed", "重写工作流连接可用")
	active, err := s.activeRun(ctx, report.ProjectID, report.ID)
	if err != nil { return result, err }
	if active != nil {
		add("active_rewrite_run_absent", "blocked", "该审核报告已有活跃重写任务")
		return result, nil
	}
	add("active_rewrite_run_absent", "passed", "该审核报告没有活跃重写任务")
	add("rewrite_input_valid", "passed", "重写输入有效")
	now := s.now().UTC()
	claims := rewriteTokenClaims{
		ActorID: request.ActorID, ProjectID: report.ProjectID, ContentItemID: report.ContentItemID,
		ReviewReportID: report.ID, SourceContentVersionID: report.SourceContentVersionID,
		SourceContentVersionVersion: report.SourceContentVersionVersion,
		SourceContentHash: report.SourceContentHash,
		SelectedIssueIDs: make([]uuid.UUID, len(issues)),
		SelectedIssuesDigest: digestRewriteValue(issues),
		ReportDigest: digestRewriteValue(rewriteReportSnapshot(report)),
		OptionalInstructions: request.OptionalInstructions, RewriteOptions: request.RewriteOptions,
		BindingID: binding.ID, BindingVersion: binding.Version,
		ConfigurationID: workflow.ID, ConfigurationVersion: workflow.Version,
		ConnectionID: connection.ID, ConnectionVersion: connection.Version,
		Stage: "rewrite", InputContract: "rewrite.input.v1", OutputContract: "rewrite.output.v1",
		RequestDigest: rewriteRequestDigest(request.ActorID, report, issues, request.OptionalInstructions, request.RewriteOptions, binding, workflow, connection),
		Nonce: uuid.NewString(), IssuedAt: now.Unix(), ExpiresAt: now.Add(10 * time.Minute).Unix(),
	}
	for index := range issues { claims.SelectedIssueIDs[index] = issues[index].ReviewIssueID }
	token, err := s.signRewriteToken(claims)
	if err != nil { return result, err }
	if len(token) > RewritePreflightTokenMaxLength { return result, ErrRewriteTokenInvalid }
	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	result.Status, result.PreflightToken, result.ExpiresAt = "passed", &token, &expiresAt
	return result, nil
}

func (s *RealRewriteService) CreateRun(ctx context.Context, reportID uuid.UUID, actorID, token, key string) (workflowrun.WorkflowRun, bool, error) {
	claims, err := s.parseRewriteToken(token)
	if err != nil { return workflowrun.WorkflowRun{}, false, err }
	if reportID == uuid.Nil || reportID != claims.ReviewReportID || strings.TrimSpace(actorID) == "" ||
		actorID != claims.ActorID || strings.TrimSpace(key) == "" || len(key) > 128 {
		return workflowrun.WorkflowRun{}, false, ErrRewritePreflightStale
	}
	tokenSum := sha256.Sum256([]byte(token))
	requestHash := workflowrun.Fingerprint(struct {
		ReviewReportID uuid.UUID `json:"reviewReportId"`
		ActorID        string    `json:"actorId"`
		TokenDigest    string    `json:"tokenDigest"`
	}{reportID, actorID, hex.EncodeToString(tokenSum[:])})
	prepare := func(tx pgx.Tx) (workflowrun.CreateRunCommand, error) {
		if claims.ExpiresAt <= s.now().Unix() { return workflowrun.CreateRunCommand{}, ErrRewritePreflightExpired }
		if _, lockErr := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "content-item:"+claims.ContentItemID.String()); lockErr != nil {
			return workflowrun.CreateRunCommand{}, lockErr
		}
		if _, lockErr := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "rewrite-active:"+claims.ProjectID.String()+":"+claims.ReviewReportID.String()); lockErr != nil {
			return workflowrun.CreateRunCommand{}, lockErr
		}
		source, sourceErr := s.source(ctx, tx, claims.SourceContentVersionID, true)
		if sourceErr != nil { return workflowrun.CreateRunCommand{}, sourceErr }
		var itemID, projectID uuid.UUID
		if sourceErr = tx.QueryRow(ctx, "SELECT id,project_id FROM content_items WHERE id=$1 FOR UPDATE", claims.ContentItemID).Scan(&itemID, &projectID); errors.Is(sourceErr, pgx.ErrNoRows) {
			return workflowrun.CreateRunCommand{}, ErrRewritePreflightStale
		}
		if sourceErr != nil { return workflowrun.CreateRunCommand{}, sourceErr }
		report, reportErr := s.report(ctx, tx, reportID, true)
		if reportErr != nil { return workflowrun.CreateRunCommand{}, reportErr }
		if !validRewriteReport(report) || source.Version.ID != report.Source.Version.ID ||
			itemID != claims.ContentItemID || projectID != claims.ProjectID ||
			report.ProjectID != claims.ProjectID || report.ContentItemID != claims.ContentItemID ||
			report.SourceContentVersionID != claims.SourceContentVersionID ||
			report.SourceContentVersionVersion != claims.SourceContentVersionVersion ||
			report.SourceContentHash != claims.SourceContentHash ||
			digestRewriteValue(rewriteReportSnapshot(report)) != claims.ReportDigest {
			return workflowrun.CreateRunCommand{}, ErrRewritePreflightStale
		}
		issues, issueErr := selectedRewriteIssues(ctx, tx, reportID, claims.SelectedIssueIDs, true)
		if errors.Is(issueErr, ErrReviewIssueNotFound) { return workflowrun.CreateRunCommand{}, ErrRewritePreflightStale }
		if issueErr != nil { return workflowrun.CreateRunCommand{}, issueErr }
		for _, issue := range issues {
			if issue.Disposition != "open" { return workflowrun.CreateRunCommand{}, ErrRewritePreflightStale }
		}
		if digestRewriteValue(issues) != claims.SelectedIssuesDigest {
			return workflowrun.CreateRunCommand{}, ErrRewritePreflightStale
		}
		binding, workflow, connection, configErr := runnableRewriteForCreate(ctx, tx, claims.ProjectID)
		if errors.Is(configErr, ErrRewriteNotConfigured) { return workflowrun.CreateRunCommand{}, ErrRewritePreflightStale }
		if configErr != nil { return workflowrun.CreateRunCommand{}, configErr }
		if binding.ID != claims.BindingID || binding.Version != claims.BindingVersion ||
			workflow.ID != claims.ConfigurationID || workflow.Version != claims.ConfigurationVersion ||
			connection.ID != claims.ConnectionID || connection.Version != claims.ConnectionVersion ||
			rewriteRequestDigest(actorID, report, issues, claims.OptionalInstructions, claims.RewriteOptions, binding, workflow, connection) != claims.RequestDigest {
			return workflowrun.CreateRunCommand{}, ErrRewritePreflightStale
		}
		if _, activeErr := workflowrun.NewPostgresRepositoryTx(tx).FindActive(ctx, claims.ProjectID, "rewrite", "review_report", reportID); activeErr == nil {
			return workflowrun.CreateRunCommand{}, ErrRewriteActiveRun
		} else if !errors.Is(activeErr, workflowrun.ErrNotFound) {
			return workflowrun.CreateRunCommand{}, activeErr
		}
		snapshot, snapshotErr := workflowrun.BuildConfigurationSnapshot(binding, workflow, connection, s.now())
		if snapshotErr != nil { return workflowrun.CreateRunCommand{}, snapshotErr }
		runID := uuid.New()
		input := RewriteRuntimeInputV1{
			SchemaVersion: "rewrite.input.v1", WorkflowRunID: runID, CorrelationID: runID.String(),
			ProjectID: claims.ProjectID, ContentItemID: claims.ContentItemID,
			SourceContentVersionID: source.Version.ID,
			SourceContentVersionVersion: source.Version.Version,
			SourceContentHash: reviewContentHash(source.Version.Content),
			SourceTitle: source.Version.Title, SourceContent: source.Version.Content,
			ReviewReportID: report.ID, ReportSnapshot: rewriteReportSnapshot(report),
			SelectedIssues: issues, OptionalInstructions: claims.OptionalInstructions,
			RewriteOptions: claims.RewriteOptions,
		}
		payload, marshalErr := json.Marshal(input)
		if marshalErr != nil { return workflowrun.CreateRunCommand{}, marshalErr }
		subjectType, subjectID := "review_report", report.ID
		return workflowrun.CreateRunCommand{
			ProjectID: claims.ProjectID, RunID: runID, Stage: "rewrite",
			SubjectType: &subjectType, SubjectID: &subjectID, InputPayload: payload,
			TriggerSource: "manual",
			PreparedConfiguration: &workflowrun.PreparedRunConfiguration{
				WorkflowConfigurationID: workflow.ID, Snapshot: snapshot,
			},
		}, nil
	}
	var run workflowrun.WorkflowRun
	var replay bool
	for attempt := 0; attempt < 3; attempt++ {
		run, replay, err = s.runs.CreateRunForPreflightTokenIdempotentForScopeWithReplay(ctx, "createContentRewriteRun", claims.ProjectID, key, requestHash, claims.Nonce, prepare)
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "40001" { break }
	}
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" &&
			databaseError.ConstraintName == "workflow_run_records_active_rewrite_subject_idx" {
			return workflowrun.WorkflowRun{}, false, ErrRewriteActiveRun
		}
	}
	return run, replay, err
}

package contentitem

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/globalconfig"
	"github.com/local/ai-content-factory/apps/api/internal/workflowbinding"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

var (
	ErrRewriteCandidateNotFound = errors.New("rewrite candidate not found")
	ErrRewriteCandidateNotReady = errors.New("rewrite candidate not ready")
	ErrRewriteContentVersionConflict = errors.New("rewrite content version conflict")
	ErrRewriteIdempotencyConflict = errors.New("rewrite idempotency conflict")
)

type RewriteSafeError struct {
	Code          string    `json:"code"`
	Message       string    `json:"message"`
	CorrelationID string    `json:"correlationId"`
	OccurredAt    time.Time `json:"occurredAt"`
}

type RewriteSummary struct {
	ReviewReportID             uuid.UUID                    `json:"reviewReportId"`
	ContentItemID              uuid.UUID                    `json:"contentItemId"`
	State                      string                       `json:"state"`
	CanStartRewrite            bool                         `json:"canStartRewrite"`
	ActiveRun                  *workflowrun.WorkflowRun     `json:"activeRun"`
	LatestRun                  *workflowrun.WorkflowRun     `json:"latestRun"`
	SourceContentVersionSummary RewriteSourceVersionSummary `json:"sourceContentVersionSummary"`
	SelectedIssueSummary       *RewriteSelectedIssueSummary `json:"selectedIssueSummary"`
	CandidateVersion           *ContentVersion              `json:"candidateVersion"`
	CandidateIsCurrent         bool                         `json:"candidateIsCurrent"`
	CanSetCurrent              bool                         `json:"canSetCurrent"`
	LatestError                *RewriteSafeError            `json:"latestError"`
	ConfigurationSummary       *RewriteConfigurationSummary `json:"configurationSummary"`
}

type RewriteHistoryItem struct {
	ReviewReportSnapshot        RewriteReportSnapshot        `json:"reviewReportSnapshot"`
	SourceContentVersionSummary RewriteSourceVersionSummary  `json:"sourceContentVersionSummary"`
	WorkflowRun                 workflowrun.WorkflowRun       `json:"workflowRun"`
	State                       string                        `json:"state"`
	CandidateVersion            *ContentVersion               `json:"candidateVersion"`
	CandidateIsCurrent          bool                          `json:"candidateIsCurrent"`
	LatestError                 *RewriteSafeError             `json:"latestError"`
}

type RewriteHistory struct {
	Items  []RewriteHistoryItem `json:"items"`
	Total  int                  `json:"total"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

type RewriteResult struct {
	ReviewReportSnapshot        RewriteReportSnapshot       `json:"reviewReportSnapshot"`
	SourceContentVersionSummary RewriteSourceVersionSummary `json:"sourceContentVersionSummary"`
	SelectedIssueSummary        RewriteSelectedIssueSummary `json:"selectedIssueSummary"`
	WorkflowRun                 workflowrun.WorkflowRun      `json:"workflowRun"`
	Output                      RewriteRuntimeOutputV1       `json:"output"`
	CandidateVersion            ContentVersion               `json:"candidateVersion"`
	CandidateIsCurrent          bool                         `json:"candidateIsCurrent"`
	CanSetCurrent               bool                         `json:"canSetCurrent"`
}

type rewriteRunFact struct {
	Run                       workflowrun.WorkflowRun
	OutputValidationFailed    bool
	ResultConsumptionFailed   bool
	ResultConsumed            bool
	FailureEventPayload        json.RawMessage
	FailureEventCreatedAt      *time.Time
	Candidate                  *ContentVersion
}

const rewriteWorkflowRunColumns = "id,run_number,project_id,stage,subject_type,subject_id,workflow_configuration_id,trigger_source,status,configuration_snapshot,input_payload,output_payload,error_code,error_message,error_details,retry_of_run_id,started_at,finished_at,cancelled_at,created_at,updated_at,version"

func scanRewriteWorkflowRun(row pgx.Row) (workflowrun.WorkflowRun, error) {
	var run workflowrun.WorkflowRun
	err := row.Scan(
		&run.ID, &run.RunNumber, &run.ProjectID, &run.Stage, &run.SubjectType, &run.SubjectID,
		&run.WorkflowConfigurationID, &run.TriggerSource, &run.Status, &run.ConfigurationSnapshot,
		&run.InputPayload, &run.OutputPayload, &run.ErrorCode, &run.ErrorMessage, &run.ErrorDetails,
		&run.RetryOfRunID, &run.StartedAt, &run.FinishedAt, &run.CancelledAt, &run.CreatedAt,
		&run.UpdatedAt, &run.Version,
	)
	if err != nil {
		return workflowrun.WorkflowRun{}, err
	}
	return workflowrun.NewFromDB(run)
}

func rewriteInput(run workflowrun.WorkflowRun) (RewriteRuntimeInputV1, error) {
	input, err := decodeRewriteRuntimeInputForConsumption(run.InputPayload)
	if err != nil || input.WorkflowRunID != run.ID || input.ProjectID != run.ProjectID ||
		run.SubjectType == nil || *run.SubjectType != "review_report" || run.SubjectID == nil ||
		input.ReviewReportID != *run.SubjectID {
		return RewriteRuntimeInputV1{}, ErrRewriteCandidateNotReady
	}
	return input, nil
}

func safeRewriteMessage(value, fallback string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if value == "" || len([]rune(value)) > 300 || strings.ContainsAny(value, "\r\n") {
		return fallback
	}
	for _, forbidden := range []string{
		"sql", "postgres", "stack", "traceback", "authorization", "credential",
		"token", "webhook", "http://", "https://", "\\", "/",
	} {
		if strings.Contains(lower, forbidden) {
			return fallback
		}
	}
	return value
}

func rewriteFailure(run workflowrun.WorkflowRun, state string, payload json.RawMessage, occurredAt *time.Time) *RewriteSafeError {
	fallback := "重写任务未完成"
	if state == "output_validation_failed" {
		fallback = "重写输出未通过结构校验"
	}
	if state == "result_consumption_failed" {
		fallback = "重写结果未能安全保存"
	}
	message := fallback
	correlationID := run.ID.String()
	at := run.UpdatedAt
	if state == "runtime_failed" && run.ErrorMessage != nil {
		message = *run.ErrorMessage
	}
	var event struct {
		Message       string    `json:"message"`
		CorrelationID string    `json:"correlationId"`
		OccurredAt    time.Time `json:"occurredAt"`
	}
	if len(payload) > 0 && json.Unmarshal(payload, &event) == nil {
		message = event.Message
		if event.CorrelationID != "" && len(event.CorrelationID) <= 128 {
			correlationID = event.CorrelationID
		}
		if !event.OccurredAt.IsZero() {
			at = event.OccurredAt
		}
	}
	if occurredAt != nil && event.OccurredAt.IsZero() {
		at = *occurredAt
	}
	return &RewriteSafeError{
		Code: state, Message: safeRewriteMessage(message, fallback),
		CorrelationID: correlationID, OccurredAt: at.UTC(),
	}
}

func rewriteFactState(fact rewriteRunFact) (string, *RewriteSafeError) {
	switch fact.Run.Status {
	case workflowrun.StatusQueued:
		return "queued", nil
	case workflowrun.StatusRunning:
		return "running", nil
	}
	if fact.Candidate != nil && fact.ResultConsumed && fact.Run.Status == workflowrun.StatusSucceeded {
		return "candidate_ready", nil
	}
	if fact.ResultConsumptionFailed {
		return "result_consumption_failed", rewriteFailure(fact.Run, "result_consumption_failed", fact.FailureEventPayload, fact.FailureEventCreatedAt)
	}
	if fact.OutputValidationFailed {
		return "output_validation_failed", rewriteFailure(fact.Run, "output_validation_failed", fact.FailureEventPayload, fact.FailureEventCreatedAt)
	}
	if fact.Run.Status == workflowrun.StatusFailed || fact.Run.Status == workflowrun.StatusCancelled {
		return "runtime_failed", rewriteFailure(fact.Run, "runtime_failed", nil, nil)
	}
	return "idle", nil
}

func (s *RealRewriteService) rewriteCandidate(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, runID uuid.UUID) (*ContentVersion, error) {
	candidate, err := scanVersion(q.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE source_workflow_run_id=$1 AND source='workflow_rewrite'", runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &candidate, nil
}

func (s *RealRewriteService) rewriteRunFacts(ctx context.Context, reportID uuid.UUID) ([]rewriteRunFact, error) {
	rows, err := s.repo.db.Query(ctx, "SELECT "+rewriteWorkflowRunColumns+" FROM workflow_run_records WHERE stage='rewrite' AND subject_type='review_report' AND subject_id=$1 ORDER BY created_at DESC,id DESC", reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	facts := []rewriteRunFact{}
	for rows.Next() {
		run, scanErr := scanRewriteWorkflowRun(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		facts = append(facts, rewriteRunFact{Run: run})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(facts) == 0 {
		return facts, nil
	}
	runIDs := make([]uuid.UUID, len(facts))
	index := make(map[uuid.UUID]int, len(facts))
	for i := range facts {
		runIDs[i] = facts[i].Run.ID
		index[facts[i].Run.ID] = i
	}
	eventRows, err := s.repo.db.Query(ctx, "SELECT DISTINCT ON (run_id) run_id,event_type,payload,created_at FROM workflow_run_events WHERE run_id=ANY($1) AND event_type IN ('output_validation_failed','result_consumption_failed','result_consumed') ORDER BY run_id,created_at DESC,id DESC", runIDs)
	if err != nil {
		return nil, err
	}
	for eventRows.Next() {
		var runID uuid.UUID
		var eventType string
		var payload json.RawMessage
		var createdAt time.Time
		if err = eventRows.Scan(&runID, &eventType, &payload, &createdAt); err != nil {
			eventRows.Close()
			return nil, err
		}
		fact := &facts[index[runID]]
		switch eventType {
		case workflowrun.EventTypeOutputValidationFailed:
			fact.OutputValidationFailed = true
		case workflowrun.EventTypeResultConsumptionFailed:
			fact.ResultConsumptionFailed = true
		case workflowrun.EventTypeResultConsumed:
			fact.ResultConsumed = true
		}
		if eventType != workflowrun.EventTypeResultConsumed {
			fact.FailureEventPayload = payload
			fact.FailureEventCreatedAt = &createdAt
		}
	}
	if err = eventRows.Err(); err != nil {
		eventRows.Close()
		return nil, err
	}
	eventRows.Close()
	candidateRows, err := s.repo.db.Query(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE source='workflow_rewrite' AND source_workflow_run_id=ANY($1)", runIDs)
	if err != nil {
		return nil, err
	}
	for candidateRows.Next() {
		candidate, scanErr := scanVersion(candidateRows)
		if scanErr != nil {
			candidateRows.Close()
			return nil, scanErr
		}
		if candidate.SourceWorkflowRunID != nil {
			facts[index[*candidate.SourceWorkflowRunID]].Candidate = &candidate
		}
	}
	if err = candidateRows.Err(); err != nil {
		candidateRows.Close()
		return nil, err
	}
	candidateRows.Close()
	return facts, nil
}

func (s *RealRewriteService) Summary(ctx context.Context, reportID uuid.UUID) (RewriteSummary, error) {
	if reportID == uuid.Nil {
		return RewriteSummary{}, ErrValidation
	}
	report, err := s.report(ctx, s.repo.db, reportID, false)
	if err != nil {
		return RewriteSummary{}, err
	}
	if !validRewriteReport(report) {
		return RewriteSummary{}, ErrReviewNotFound
	}
	out := RewriteSummary{
		ReviewReportID: report.ID, ContentItemID: report.ContentItemID, State: "idle",
		SourceContentVersionSummary: rewriteSourceSummary(report),
	}
	_, binding, workflow, connection, configuredErr := s.summaryConfiguration(ctx, report.ProjectID)
	if configuredErr == nil {
		out.ConfigurationSummary = rewriteConfigurationSummary(binding, workflow, connection)
	} else if !errors.Is(configuredErr, ErrRewriteNotConfigured) {
		return RewriteSummary{}, configuredErr
	}
	facts, err := s.rewriteRunFacts(ctx, report.ID)
	if err != nil {
		return RewriteSummary{}, err
	}
	if len(facts) > 0 {
		out.LatestRun = &facts[0].Run
	}
	var displayed *rewriteRunFact
	for i := range facts {
		if facts[i].Run.Status == workflowrun.StatusQueued || facts[i].Run.Status == workflowrun.StatusRunning {
			displayed = &facts[i]
			out.ActiveRun = &facts[i].Run
			break
		}
	}
	if displayed == nil {
		for i := range facts {
			state, _ := rewriteFactState(facts[i])
			if state == "runtime_failed" || state == "output_validation_failed" ||
				state == "result_consumption_failed" || state == "candidate_ready" {
				displayed = &facts[i]
				break
			}
		}
	}
	if displayed != nil {
		out.State, out.LatestError = rewriteFactState(*displayed)
		input, inputErr := rewriteInput(displayed.Run)
		if inputErr == nil {
			summary := RewriteSelectedIssueSummary{Total: len(input.SelectedIssues), Items: input.SelectedIssues}
			out.SelectedIssueSummary = &summary
		}
		if displayed.Candidate != nil && displayed.ResultConsumed {
			out.CandidateVersion = displayed.Candidate
			out.CandidateIsCurrent = report.Source.Item.CurrentVersionID == displayed.Candidate.ID
			out.CanSetCurrent = !out.CandidateIsCurrent
		}
	}
	if displayed == nil && errors.Is(configuredErr, ErrRewriteNotConfigured) {
		out.State = "not_configured"
	}
	var openIssueCount int
	if err = s.repo.db.QueryRow(ctx, "SELECT COUNT(*) FROM review_findings WHERE review_id=$1 AND disposition='open'", report.ID).Scan(&openIssueCount); err != nil {
		return RewriteSummary{}, err
	}
	out.CanStartRewrite = out.ActiveRun == nil && configuredErr == nil && openIssueCount > 0
	return out, nil
}

func (s *RealRewriteService) summaryConfiguration(ctx context.Context, projectID uuid.UUID) (bool, workflowbinding.ProjectWorkflowBinding, globalconfig.Workflow, globalconfig.Connection, error) {
	binding, workflow, connection, err := s.runnable(ctx, projectID)
	return err == nil, binding, workflow, connection, err
}

func (s *RealRewriteService) History(ctx context.Context, itemID uuid.UUID, limit, offset int) (RewriteHistory, error) {
	if itemID == uuid.Nil || limit < 1 || limit > 100 || offset < 0 {
		return RewriteHistory{}, ErrValidation
	}
	detail, err := s.repo.GetByID(ctx, itemID)
	if err != nil {
		return RewriteHistory{}, err
	}
	var total int
	if err = s.repo.db.QueryRow(ctx, "SELECT COUNT(*) FROM workflow_run_records r JOIN review_reports p ON p.id=r.subject_id WHERE r.project_id=$1 AND r.stage='rewrite' AND r.subject_type='review_report' AND p.content_item_id=$2", detail.Item.ProjectID, itemID).Scan(&total); err != nil {
		return RewriteHistory{}, err
	}
	rows, err := s.repo.db.Query(ctx, "SELECT "+qualifiedColumns("r", rewriteWorkflowRunColumns)+" FROM workflow_run_records r JOIN review_reports p ON p.id=r.subject_id WHERE r.project_id=$1 AND r.stage='rewrite' AND r.subject_type='review_report' AND p.content_item_id=$2 ORDER BY r.created_at DESC,r.id DESC LIMIT $3 OFFSET $4", detail.Item.ProjectID, itemID, limit, offset)
	if err != nil {
		return RewriteHistory{}, err
	}
	runs := []workflowrun.WorkflowRun{}
	for rows.Next() {
		run, scanErr := scanRewriteWorkflowRun(rows)
		if scanErr != nil {
			rows.Close()
			return RewriteHistory{}, scanErr
		}
		runs = append(runs, run)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return RewriteHistory{}, err
	}
	rows.Close()
	out := RewriteHistory{Items: []RewriteHistoryItem{}, Total: total, Limit: limit, Offset: offset}
	if len(runs) == 0 {
		return out, nil
	}
	runIDs := make([]uuid.UUID, len(runs))
	facts := make(map[uuid.UUID]*rewriteRunFact, len(runs))
	inputs := make(map[uuid.UUID]RewriteRuntimeInputV1, len(runs))
	sourceIDs := make([]uuid.UUID, 0, len(runs))
	sourceSeen := map[uuid.UUID]bool{}
	for i := range runs {
		runIDs[i] = runs[i].ID
		facts[runs[i].ID] = &rewriteRunFact{Run: runs[i]}
		input, inputErr := rewriteInput(runs[i])
		if inputErr != nil {
			return RewriteHistory{}, ErrRewriteCandidateNotReady
		}
		inputs[runs[i].ID] = input
		if !sourceSeen[input.SourceContentVersionID] {
			sourceSeen[input.SourceContentVersionID] = true
			sourceIDs = append(sourceIDs, input.SourceContentVersionID)
		}
	}
	eventRows, err := s.repo.db.Query(ctx, "SELECT run_id,event_type,payload,created_at FROM workflow_run_events WHERE run_id=ANY($1) AND event_type IN ('output_validation_failed','result_consumption_failed','result_consumed') ORDER BY created_at DESC,id DESC", runIDs)
	if err != nil {
		return RewriteHistory{}, err
	}
	for eventRows.Next() {
		var runID uuid.UUID
		var eventType string
		var payload json.RawMessage
		var createdAt time.Time
		if err = eventRows.Scan(&runID, &eventType, &payload, &createdAt); err != nil {
			eventRows.Close()
			return RewriteHistory{}, err
		}
		fact := facts[runID]
		switch eventType {
		case workflowrun.EventTypeOutputValidationFailed:
			fact.OutputValidationFailed = true
		case workflowrun.EventTypeResultConsumptionFailed:
			fact.ResultConsumptionFailed = true
		case workflowrun.EventTypeResultConsumed:
			fact.ResultConsumed = true
		}
		if fact.FailureEventCreatedAt == nil && eventType != workflowrun.EventTypeResultConsumed {
			fact.FailureEventPayload = payload
			fact.FailureEventCreatedAt = &createdAt
		}
	}
	eventRows.Close()
	candidateRows, err := s.repo.db.Query(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE source='workflow_rewrite' AND source_workflow_run_id=ANY($1)", runIDs)
	if err != nil {
		return RewriteHistory{}, err
	}
	for candidateRows.Next() {
		candidate, scanErr := scanVersion(candidateRows)
		if scanErr != nil {
			candidateRows.Close()
			return RewriteHistory{}, scanErr
		}
		if candidate.SourceWorkflowRunID != nil {
			facts[*candidate.SourceWorkflowRunID].Candidate = &candidate
		}
	}
	candidateRows.Close()
	sources := make(map[uuid.UUID]ContentVersion, len(sourceIDs))
	sourceRows, err := s.repo.db.Query(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE id=ANY($1)", sourceIDs)
	if err != nil {
		return RewriteHistory{}, err
	}
	for sourceRows.Next() {
		source, scanErr := scanVersion(sourceRows)
		if scanErr != nil {
			sourceRows.Close()
			return RewriteHistory{}, scanErr
		}
		sources[source.ID] = source
	}
	sourceRows.Close()
	for _, run := range runs {
		fact := facts[run.ID]
		input := inputs[run.ID]
		source, sourceExists := sources[input.SourceContentVersionID]
		if !sourceExists || source.ContentItemID != input.ContentItemID ||
			source.Version != input.SourceContentVersionVersion {
			return RewriteHistory{}, ErrRewriteCandidateNotReady
		}
		state, latestError := rewriteFactState(*fact)
		item := RewriteHistoryItem{
			ReviewReportSnapshot: input.ReportSnapshot,
			SourceContentVersionSummary: RewriteSourceVersionSummary{
				ID: input.SourceContentVersionID, ContentItemID: input.ContentItemID,
				VersionNo: source.VersionNo, Version: input.SourceContentVersionVersion,
				Title: input.SourceTitle, WordCount: source.WordCount,
				ContentHash: input.SourceContentHash,
			},
			WorkflowRun: run, State: state, CandidateVersion: fact.Candidate,
			LatestError: latestError,
		}
		if fact.Candidate != nil {
			item.CandidateIsCurrent = fact.Candidate.ID == detail.Item.CurrentVersionID
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

func (s *RealRewriteService) Result(ctx context.Context, runID uuid.UUID) (RewriteResult, error) {
	if runID == uuid.Nil {
		return RewriteResult{}, ErrValidation
	}
	run, err := workflowrun.NewPostgresRepository(s.repo.db).GetByID(ctx, runID)
	if err != nil {
		return RewriteResult{}, err
	}
	if run.Stage != "rewrite" || run.SubjectType == nil || *run.SubjectType != "review_report" || run.SubjectID == nil {
		return RewriteResult{}, ErrRewriteCandidateNotFound
	}
	input, err := rewriteInput(run)
	if err != nil {
		return RewriteResult{}, ErrRewriteCandidateNotReady
	}
	candidate, err := s.consumedRewriteCandidate(ctx, run.ID)
	if errors.Is(err, ErrContentVersionNotFound) {
		return RewriteResult{}, ErrRewriteCandidateNotFound
	}
	if err != nil {
		return RewriteResult{}, ErrRewriteCandidateNotReady
	}
	report, err := s.report(ctx, s.repo.db, input.ReviewReportID, false)
	if err != nil || !validRewriteReport(report) ||
		report.ContentItemID != input.ContentItemID ||
		report.SourceContentVersionID != input.SourceContentVersionID ||
		candidate.ContentItemID != input.ContentItemID ||
		candidate.SourceWorkflowRunID == nil || *candidate.SourceWorkflowRunID != run.ID ||
		candidate.SourceContentVersionID == nil || *candidate.SourceContentVersionID != input.SourceContentVersionID ||
		candidate.SourceContentVersionVersion == nil || *candidate.SourceContentVersionVersion != input.SourceContentVersionVersion {
		return RewriteResult{}, ErrRewriteCandidateNotReady
	}
	selectedIDs := make([]uuid.UUID, len(input.SelectedIssues))
	for i := range input.SelectedIssues {
		selectedIDs[i] = input.SelectedIssues[i].ReviewIssueID
	}
	output, err := DecodeRewriteRuntimeOutput(run.OutputPayload, selectedIDs)
	if err != nil {
		return RewriteResult{}, ErrRewriteCandidateNotReady
	}
	candidateIsCurrent := report.Source.Item.CurrentVersionID == candidate.ID
	canSetCurrent := !candidateIsCurrent
	return RewriteResult{
		ReviewReportSnapshot: input.ReportSnapshot,
		SourceContentVersionSummary: rewriteSourceSummary(report),
		SelectedIssueSummary: RewriteSelectedIssueSummary{Total: len(input.SelectedIssues), Items: input.SelectedIssues},
		WorkflowRun: run, Output: output, CandidateVersion: candidate,
		CandidateIsCurrent: candidateIsCurrent, CanSetCurrent: canSetCurrent,
	}, nil
}

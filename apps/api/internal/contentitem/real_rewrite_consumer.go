package contentitem

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/local/ai-content-factory/apps/api/internal/workflowrun"
)

var (
	ErrRewriteOutputInvalid     = errors.New("rewrite output validation failed")
	ErrRewriteResultConsumption = errors.New("rewrite result consumption failed")
	ErrRewriteRunNotConsumable  = errors.New("rewrite run is not consumable")
)

type RewriteIssueOutcomeV1 struct {
	ReviewIssueID uuid.UUID `json:"reviewIssueId"`
	Summary       string    `json:"summary"`
}

type RewriteOutputMetadataV1 struct {
	ChangeSummary *string `json:"changeSummary"`
}

type RewriteRuntimeOutputV1 struct {
	SchemaVersion    string                   `json:"schemaVersion"`
	Title            string                   `json:"title"`
	Content          string                   `json:"content"`
	Summary          string                   `json:"summary"`
	AddressedIssues  []RewriteIssueOutcomeV1  `json:"addressedIssues"`
	UnresolvedIssues []RewriteIssueOutcomeV1  `json:"unresolvedIssues"`
	Warnings         []string                 `json:"warnings"`
	Metadata         *RewriteOutputMetadataV1 `json:"metadata"`
}

type rewriteRuntimeOutputWire struct {
	SchemaVersion    *string                  `json:"schemaVersion"`
	Title            *string                  `json:"title"`
	Content          *string                  `json:"content"`
	Summary          *string                  `json:"summary"`
	AddressedIssues  *[]RewriteIssueOutcomeV1 `json:"addressedIssues"`
	UnresolvedIssues *[]RewriteIssueOutcomeV1 `json:"unresolvedIssues"`
	Warnings         *[]string                `json:"warnings"`
	Metadata         json.RawMessage          `json:"metadata"`
}

// DecodeRewriteRuntimeOutput is the only rewrite.output.v1 decoding boundary.
// It validates both the strict JSON schema and the exact selected-Issue partition.
func DecodeRewriteRuntimeOutput(raw json.RawMessage, selectedIssueIDs []uuid.UUID) (RewriteRuntimeOutputV1, error) {
	var output RewriteRuntimeOutputV1
	if len(bytes.TrimSpace(raw)) == 0 || !utf8.Valid(raw) || rejectDuplicateJSONKeys(raw) != nil {
		return output, ErrRewriteOutputInvalid
	}
	var wire rewriteRuntimeOutputWire
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil {
		return output, ErrRewriteOutputInvalid
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return output, ErrRewriteOutputInvalid
	}
	if wire.SchemaVersion == nil || wire.Title == nil || wire.Content == nil || wire.Summary == nil ||
		wire.AddressedIssues == nil || wire.UnresolvedIssues == nil || wire.Warnings == nil ||
		len(wire.Metadata) == 0 {
		return output, ErrRewriteOutputInvalid
	}
	output = RewriteRuntimeOutputV1{
		SchemaVersion: *wire.SchemaVersion, Title: *wire.Title, Content: *wire.Content,
		Summary: *wire.Summary, AddressedIssues: *wire.AddressedIssues,
		UnresolvedIssues: *wire.UnresolvedIssues, Warnings: *wire.Warnings,
	}
	if output.SchemaVersion != "rewrite.output.v1" ||
		!validRequiredRewriteString(output.Title, 120) ||
		!validRequiredRewriteString(output.Content, 200000) ||
		!validRequiredRewriteString(output.Summary, 5000) ||
		len(output.AddressedIssues) > 50 || len(output.UnresolvedIssues) > 50 ||
		len(output.Warnings) > 50 {
		return RewriteRuntimeOutputV1{}, ErrRewriteOutputInvalid
	}
	metadata, err := decodeRewriteOutputMetadata(wire.Metadata)
	if err != nil {
		return RewriteRuntimeOutputV1{}, err
	}
	output.Metadata = metadata
	selected := make(map[uuid.UUID]bool, len(selectedIssueIDs))
	for _, issueID := range selectedIssueIDs {
		if issueID == uuid.Nil || selected[issueID] || len(selectedIssueIDs) < 1 || len(selectedIssueIDs) > 50 {
			return RewriteRuntimeOutputV1{}, ErrRewriteOutputInvalid
		}
		selected[issueID] = true
	}
	covered := make(map[uuid.UUID]bool, len(selected))
	for _, group := range [][]RewriteIssueOutcomeV1{output.AddressedIssues, output.UnresolvedIssues} {
		for _, issue := range group {
			if issue.ReviewIssueID == uuid.Nil || !selected[issue.ReviewIssueID] ||
				covered[issue.ReviewIssueID] || !validRequiredRewriteString(issue.Summary, 2000) {
				return RewriteRuntimeOutputV1{}, ErrRewriteOutputInvalid
			}
			covered[issue.ReviewIssueID] = true
		}
	}
	if len(covered) != len(selected) {
		return RewriteRuntimeOutputV1{}, ErrRewriteOutputInvalid
	}
	for _, warning := range output.Warnings {
		if !validRequiredRewriteString(warning, 1000) {
			return RewriteRuntimeOutputV1{}, ErrRewriteOutputInvalid
		}
	}
	if forbiddenRewriteOutput(output) {
		return RewriteRuntimeOutputV1{}, ErrRewriteOutputInvalid
	}
	return output, nil
}

func decodeRewriteOutputMetadata(raw json.RawMessage) (*RewriteOutputMetadataV1, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	var wire struct {
		ChangeSummary json.RawMessage `json:"changeSummary"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || len(wire.ChangeSummary) == 0 {
		return nil, ErrRewriteOutputInvalid
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, ErrRewriteOutputInvalid
	}
	metadata := &RewriteOutputMetadataV1{}
	if bytes.Equal(bytes.TrimSpace(wire.ChangeSummary), []byte("null")) {
		return metadata, nil
	}
	var changeSummary string
	if json.Unmarshal(wire.ChangeSummary, &changeSummary) != nil ||
		utf8.RuneCountInString(changeSummary) > 2000 {
		return nil, ErrRewriteOutputInvalid
	}
	metadata.ChangeSummary = &changeSummary
	return metadata, nil
}

func validRequiredRewriteString(value string, maximum int) bool {
	count := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && strings.TrimSpace(value) != "" && count >= 1 && count <= maximum
}

func forbiddenRewriteOutput(output RewriteRuntimeOutputV1) bool {
	values := []string{output.Title, output.Content, output.Summary}
	for _, issue := range output.AddressedIssues {
		values = append(values, issue.Summary)
	}
	for _, issue := range output.UnresolvedIssues {
		values = append(values, issue.Summary)
	}
	values = append(values, output.Warnings...)
	if output.Metadata != nil && output.Metadata.ChangeSummary != nil {
		values = append(values, *output.Metadata.ChangeSummary)
	}
	for _, value := range values {
		if forbiddenRewriteOutputString(value) {
			return true
		}
	}
	return false
}

func forbiddenRewriteOutputString(value string) bool {
	return containsForbiddenRewriteMaterial(value)
}

func decodeRewriteRuntimeInputForConsumption(raw json.RawMessage) (RewriteRuntimeInputV1, error) {
	var input RewriteRuntimeInputV1
	if len(bytes.TrimSpace(raw)) == 0 || !utf8.Valid(raw) || rejectDuplicateJSONKeys(raw) != nil {
		return input, ErrRewriteRunNotConsumable
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		return input, ErrRewriteRunNotConsumable
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return input, ErrRewriteRunNotConsumable
	}
	if input.SchemaVersion != "rewrite.input.v1" || input.WorkflowRunID == uuid.Nil ||
		strings.TrimSpace(input.CorrelationID) == "" || utf8.RuneCountInString(input.CorrelationID) > 128 ||
		input.ProjectID == uuid.Nil || input.ContentItemID == uuid.Nil ||
		input.SourceContentVersionID == uuid.Nil || input.SourceContentVersionVersion < 1 ||
		!validLowerHexHash(input.SourceContentHash) ||
		!validRequiredRewriteString(input.SourceTitle, 120) ||
		!validRequiredRewriteString(input.SourceContent, 200000) ||
		input.ReviewReportID == uuid.Nil ||
		input.ReportSnapshot.ReviewReportID != input.ReviewReportID ||
		input.ReportSnapshot.SourceContentVersionID != input.SourceContentVersionID ||
		input.ReportSnapshot.SourceContentVersionVersion != input.SourceContentVersionVersion ||
		input.ReportSnapshot.SourceContentHash != input.SourceContentHash ||
		input.ReportSnapshot.CompletedAt.IsZero() ||
		!validRequiredRewriteString(input.ReportSnapshot.Summary, 5000) ||
		(input.ReportSnapshot.Conclusion != "passed" && input.ReportSnapshot.Conclusion != "needs_changes") ||
		(input.RewriteOptions.Strategy != "targeted_fix" && input.RewriteOptions.Strategy != "creative_rewrite") ||
		input.OptionalInstructions != nil && utf8.RuneCountInString(*input.OptionalInstructions) > 2000 ||
		forbiddenRewriteInstructions(input.OptionalInstructions) ||
		len(input.SelectedIssues) < 1 || len(input.SelectedIssues) > 50 {
		return RewriteRuntimeInputV1{}, ErrRewriteRunNotConsumable
	}
	seen := map[uuid.UUID]bool{}
	for _, issue := range input.SelectedIssues {
		if issue.ReviewIssueID == uuid.Nil || seen[issue.ReviewIssueID] ||
			issue.ReviewReportID != input.ReviewReportID || issue.Position < 1 || issue.Position > 200 ||
			issue.Version < 1 || issue.Disposition != "open" ||
			!validRequiredRewriteString(issue.IssueKey, 120) ||
			!validRequiredRewriteString(issue.CategoryKey, 80) ||
			!validRequiredRewriteString(issue.CategoryLabel, 120) ||
			(issue.Severity != "critical" && issue.Severity != "warning" && issue.Severity != "suggestion") ||
			!validRequiredRewriteString(issue.Title, 200) ||
			!validRequiredRewriteString(issue.Description, 5000) {
			return RewriteRuntimeInputV1{}, ErrRewriteRunNotConsumable
		}
		seen[issue.ReviewIssueID] = true
	}
	return input, nil
}

func validLowerHexHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func rewriteConfigurationContracts(raw json.RawMessage) bool {
	var snapshot struct {
		Stage                 string `json:"stage"`
		WorkflowConfiguration struct {
			InputContractVersion  string `json:"inputContractVersion"`
			OutputContractVersion string `json:"outputContractVersion"`
		} `json:"workflowConfiguration"`
	}
	return json.Unmarshal(raw, &snapshot) == nil && snapshot.Stage == "rewrite" &&
		snapshot.WorkflowConfiguration.InputContractVersion == "rewrite.input.v1" &&
		snapshot.WorkflowConfiguration.OutputContractVersion == "rewrite.output.v1"
}

func (s *RealRewriteService) ConsumeSucceededRun(ctx context.Context, run workflowrun.WorkflowRun) error {
	_, err := s.ConsumeRewriteResult(ctx, run)
	return err
}

func decodeRewriteConsumption(run workflowrun.WorkflowRun) (RewriteRuntimeInputV1, RewriteRuntimeOutputV1, error) {
	input, err := decodeRewriteRuntimeInputForConsumption(run.InputPayload)
	if err != nil || input.WorkflowRunID != run.ID || input.ProjectID != run.ProjectID {
		return RewriteRuntimeInputV1{}, RewriteRuntimeOutputV1{}, ErrRewriteOutputInvalid
	}
	selected := make([]uuid.UUID, len(input.SelectedIssues))
	for i := range input.SelectedIssues {
		selected[i] = input.SelectedIssues[i].ReviewIssueID
	}
	output, err := DecodeRewriteRuntimeOutput(run.OutputPayload, selected)
	return input, output, err
}

func (s *RealRewriteService) ValidateResult(run workflowrun.WorkflowRun) error {
	_, _, err := decodeRewriteConsumption(run)
	return err
}

func (s *RealRewriteService) ConsumeResultTx(ctx context.Context, tx pgx.Tx, run workflowrun.WorkflowRun) error {
	input, output, err := decodeRewriteConsumption(run)
	if err != nil {
		return err
	}
	_, err = s.consumeRewriteLocked(ctx, tx, run, input, output, true)
	return err
}

func (s *RealRewriteService) ConsumeRewriteResult(ctx context.Context, signal workflowrun.WorkflowRun) (ContentVersion, error) {
	if signal.ID == uuid.Nil {
		return ContentVersion{}, ErrRewriteRunNotConsumable
	}
	run, err := workflowrun.NewPostgresRepository(s.repo.db).GetByID(ctx, signal.ID)
	if err != nil {
		return ContentVersion{}, ErrRewriteRunNotConsumable
	}
	if run.Stage != "rewrite" || run.Status != workflowrun.StatusSucceeded ||
		run.SubjectType == nil || *run.SubjectType != "review_report" || run.SubjectID == nil {
		return ContentVersion{}, ErrRewriteRunNotConsumable
	}
	if existing, existingErr := s.consumedRewriteCandidate(ctx, run.ID); existingErr == nil {
		return existing, nil
	} else if !errors.Is(existingErr, ErrContentVersionNotFound) {
		return ContentVersion{}, existingErr
	}
	input, inputErr := decodeRewriteRuntimeInputForConsumption(run.InputPayload)
	if inputErr != nil || input.WorkflowRunID != run.ID || input.ProjectID != run.ProjectID ||
		input.ReviewReportID != *run.SubjectID || !rewriteConfigurationContracts(run.ConfigurationSnapshot) {
		if eventErr := s.recordRewriteFailure(ctx, run.ID, workflowrun.EventTypeResultConsumptionFailed); eventErr != nil {
			return ContentVersion{}, eventErr
		}
		return ContentVersion{}, ErrRewriteResultConsumption
	}
	selectedIssueIDs := make([]uuid.UUID, 0, len(input.SelectedIssues))
	for _, issue := range input.SelectedIssues {
		selectedIssueIDs = append(selectedIssueIDs, issue.ReviewIssueID)
	}
	output, outputErr := DecodeRewriteRuntimeOutput(run.OutputPayload, selectedIssueIDs)
	if outputErr != nil {
		if eventErr := s.recordRewriteFailure(ctx, run.ID, workflowrun.EventTypeOutputValidationFailed); eventErr != nil {
			return ContentVersion{}, eventErr
		}
		return ContentVersion{}, ErrRewriteOutputInvalid
	}
	tx, err := s.begin(ctx)
	if err != nil {
		if eventErr := s.recordRewriteFailure(ctx, run.ID, workflowrun.EventTypeResultConsumptionFailed); eventErr != nil {
			return ContentVersion{}, eventErr
		}
		return ContentVersion{}, ErrRewriteResultConsumption
	}
	defer tx.Rollback(ctx)
	candidate, err := s.consumeRewriteLocked(ctx, tx, run, input, output, false)
	if err != nil {
		_ = tx.Rollback(ctx)
		if errors.Is(err, ErrRewriteCandidateAlreadyExists) {
			if existing, existingErr := s.consumedRewriteCandidate(ctx, run.ID); existingErr == nil {
				return existing, nil
			}
		}
		if errors.Is(err, ErrRewriteOutputInvalid) {
			return ContentVersion{}, err
		}
		if eventErr := s.recordRewriteFailure(ctx, run.ID, workflowrun.EventTypeResultConsumptionFailed); eventErr != nil {
			return ContentVersion{}, eventErr
		}
		return ContentVersion{}, ErrRewriteResultConsumption
	}
	if err = tx.Commit(ctx); err != nil {
		if eventErr := s.recordRewriteFailure(ctx, run.ID, workflowrun.EventTypeResultConsumptionFailed); eventErr != nil {
			return ContentVersion{}, eventErr
		}
		return ContentVersion{}, ErrRewriteResultConsumption
	}
	return candidate, nil
}

func (s *RealRewriteService) consumeRewriteLocked(
	ctx context.Context,
	tx pgx.Tx,
	persisted workflowrun.WorkflowRun,
	input RewriteRuntimeInputV1,
	output RewriteRuntimeOutputV1,
	allowConsumptionFailure bool,
) (ContentVersion, error) {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "content-item:"+input.ContentItemID.String()); err != nil {
		return ContentVersion{}, err
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "rewrite-run:"+persisted.ID.String()); err != nil {
		return ContentVersion{}, err
	}
	run, err := workflowrun.NewPostgresRepositoryTx(tx).GetByIDForUpdate(ctx, persisted.ID)
	if err != nil {
		return ContentVersion{}, ErrRewriteRunNotConsumable
	}
	if run.Stage != "rewrite" || (run.Status != workflowrun.StatusSucceeded && run.Status != workflowrun.StatusRunning && run.Status != workflowrun.StatusFailed) ||
		run.SubjectType == nil || *run.SubjectType != "review_report" || run.SubjectID == nil ||
		*run.SubjectID != input.ReviewReportID || run.ProjectID != input.ProjectID ||
		!bytes.Equal(run.InputPayload, persisted.InputPayload) ||
		!bytes.Equal(run.OutputPayload, persisted.OutputPayload) ||
		!bytes.Equal(run.ConfigurationSnapshot, persisted.ConfigurationSnapshot) {
		return ContentVersion{}, ErrRewriteRunNotConsumable
	}
	source, err := s.source(ctx, tx, input.SourceContentVersionID, true)
	if err != nil {
		return ContentVersion{}, err
	}
	if _, err = tx.Exec(ctx, "SELECT 1 FROM content_items WHERE id=$1 FOR UPDATE", input.ContentItemID); err != nil {
		return ContentVersion{}, err
	}
	detail, err := s.repo.detail(ctx, tx, input.ContentItemID)
	if err != nil {
		return ContentVersion{}, err
	}
	report, err := s.report(ctx, tx, input.ReviewReportID, true)
	if err != nil {
		return ContentVersion{}, err
	}
	if !validRewriteReport(report) || detail.Item.ProjectID != run.ProjectID ||
		source.Item.ID != detail.Item.ID || source.Item.ProjectID != detail.Item.ProjectID ||
		source.Version.ID != report.SourceContentVersionID ||
		source.Version.Version != input.SourceContentVersionVersion ||
		source.Version.Title != input.SourceTitle || source.Version.Content != input.SourceContent ||
		reviewContentHash(source.Version.Content) != input.SourceContentHash ||
		report.ID != input.ReportSnapshot.ReviewReportID ||
		report.SourceContentVersionID != input.ReportSnapshot.SourceContentVersionID ||
		report.SourceContentVersionVersion != input.ReportSnapshot.SourceContentVersionVersion ||
		report.SourceContentHash != input.ReportSnapshot.SourceContentHash ||
		report.Conclusion != input.ReportSnapshot.Conclusion ||
		report.Summary != input.ReportSnapshot.Summary || report.CompletedAt == nil ||
		!report.CompletedAt.Equal(input.ReportSnapshot.CompletedAt) {
		return ContentVersion{}, ErrRewriteRunNotConsumable
	}
	issueIDs := make([]uuid.UUID, 0, len(input.SelectedIssues))
	for _, issue := range input.SelectedIssues {
		issueIDs = append(issueIDs, issue.ReviewIssueID)
	}
	lockedIssues, err := selectedRewriteIssues(ctx, tx, report.ID, issueIDs, true)
	if err != nil || len(lockedIssues) != len(input.SelectedIssues) {
		return ContentVersion{}, ErrRewriteRunNotConsumable
	}
	lockedIDs := map[uuid.UUID]bool{}
	for _, issue := range lockedIssues {
		if issue.ReviewReportID != report.ID {
			return ContentVersion{}, ErrRewriteRunNotConsumable
		}
		lockedIDs[issue.ReviewIssueID] = true
	}
	for _, issue := range input.SelectedIssues {
		if !lockedIDs[issue.ReviewIssueID] {
			return ContentVersion{}, ErrRewriteRunNotConsumable
		}
	}
	existing, existingErr := scanVersion(tx.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE source_workflow_run_id=$1 AND source='workflow_rewrite' FOR UPDATE", run.ID))
	if existingErr == nil {
		return existing, nil
	}
	if !errors.Is(existingErr, pgx.ErrNoRows) {
		return ContentVersion{}, existingErr
	}
	var validationFailed, consumptionFailed, consumed bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='output_validation_failed'),EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumption_failed'),EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed')", run.ID).Scan(&validationFailed, &consumptionFailed, &consumed); err != nil {
		return ContentVersion{}, err
	}
	if validationFailed {
		return ContentVersion{}, ErrRewriteOutputInvalid
	}
	if consumptionFailed && !allowConsumptionFailure {
		return ContentVersion{}, ErrRewriteResultConsumption
	}
	if consumed {
		return ContentVersion{}, ErrRewriteRunNotConsumable
	}
	versionNo, err := nextContentVersionNo(ctx, tx, detail.Item.ID)
	if err != nil {
		return ContentVersion{}, err
	}
	parameters, err := rewriteCandidateParameters(output)
	if err != nil {
		return ContentVersion{}, err
	}
	candidate := mapRewriteOutputToCandidate(run.ID, source.Version, detail.Item.ID, versionNo, output, parameters)
	created, err := s.repo.CreateContentVersion(ctx, tx, candidate)
	if err != nil {
		return ContentVersion{}, err
	}
	payload, err := json.Marshal(map[string]any{
		"candidateVersionId": created.ID, "reviewReportId": report.ID,
		"sourceContentVersionId": source.Version.ID, "schemaVersion": output.SchemaVersion,
	})
	if err != nil {
		return ContentVersion{}, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO workflow_run_events(id,run_id,event_type,status,payload,created_at) VALUES($1,$2,'result_consumed','succeeded',$3,$4)", uuid.New(), run.ID, payload, s.now().UTC()); err != nil {
		return ContentVersion{}, err
	}
	return created, nil
}

func mapRewriteOutputToCandidate(
	runID uuid.UUID,
	source ContentVersion,
	contentItemID uuid.UUID,
	versionNo int,
	output RewriteRuntimeOutputV1,
	parameters json.RawMessage,
) ContentVersion {
	summary := output.Summary
	return ContentVersion{
		ID: uuid.New(), ContentItemID: contentItemID, VersionNo: versionNo,
		SourceContentVersionID:      &source.ID,
		SourceContentVersionVersion: &source.Version,
		SourceWorkflowRunID:         &runID, Title: output.Title, Content: output.Content,
		Summary: &summary, WordCount: wordCount(output.Content),
		Source: ContentVersionSourceWorkflowRewrite, Status: ContentVersionStatusEditableDraft,
		GenerationParameters: parameters, Version: 1,
	}
}

func rewriteCandidateParameters(output RewriteRuntimeOutputV1) (json.RawMessage, error) {
	return json.Marshal(struct {
		SchemaVersion    string                   `json:"schemaVersion"`
		AddressedIssues  []RewriteIssueOutcomeV1  `json:"addressedIssues"`
		UnresolvedIssues []RewriteIssueOutcomeV1  `json:"unresolvedIssues"`
		Warnings         []string                 `json:"warnings"`
		Metadata         *RewriteOutputMetadataV1 `json:"metadata"`
	}{
		SchemaVersion: output.SchemaVersion, AddressedIssues: output.AddressedIssues,
		UnresolvedIssues: output.UnresolvedIssues, Warnings: output.Warnings,
		Metadata: output.Metadata,
	})
}

func (s *RealRewriteService) consumedRewriteCandidate(ctx context.Context, runID uuid.UUID) (ContentVersion, error) {
	candidate, err := scanVersion(s.repo.db.QueryRow(ctx, "SELECT "+versionColumns+" FROM content_versions WHERE source_workflow_run_id=$1 AND source='workflow_rewrite'", runID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ContentVersion{}, ErrContentVersionNotFound
	}
	if err != nil {
		return ContentVersion{}, err
	}
	var consumed bool
	if err = s.repo.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed')", runID).Scan(&consumed); err != nil {
		return ContentVersion{}, err
	}
	if !consumed {
		return ContentVersion{}, ErrRewriteResultConsumption
	}
	return candidate, nil
}

func (s *RealRewriteService) recordRewriteFailure(ctx context.Context, runID uuid.UUID, eventType string) error {
	if eventType != workflowrun.EventTypeOutputValidationFailed &&
		eventType != workflowrun.EventTypeResultConsumptionFailed {
		return ErrRewriteRunNotConsumable
	}
	tx, err := s.repo.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "rewrite-run:"+runID.String()); err != nil {
		return err
	}
	run, err := workflowrun.NewPostgresRepositoryTx(tx).GetByIDForUpdate(ctx, runID)
	if err != nil {
		return err
	}
	if run.Stage != "rewrite" || run.Status != workflowrun.StatusSucceeded {
		return ErrRewriteRunNotConsumable
	}
	var candidateOrConsumed, exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM content_versions WHERE source_workflow_run_id=$1 AND source='workflow_rewrite') OR EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type='result_consumed'),EXISTS(SELECT 1 FROM workflow_run_events WHERE run_id=$1 AND event_type=$2)", runID, eventType).Scan(&candidateOrConsumed, &exists); err != nil {
		return err
	}
	if candidateOrConsumed || exists {
		return tx.Commit(ctx)
	}
	message := "重写结果未能安全保存"
	if eventType == workflowrun.EventTypeOutputValidationFailed {
		message = "重写输出未通过结构校验"
	}
	occurredAt := s.now().UTC()
	payload, err := json.Marshal(map[string]any{
		"code": eventType, "message": message, "correlationId": run.ID.String(),
		"occurredAt": occurredAt,
	})
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO workflow_run_events(id,run_id,event_type,status,payload,created_at) VALUES($1,$2,$3,'succeeded',$4,$5)", uuid.New(), run.ID, eventType, payload, occurredAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

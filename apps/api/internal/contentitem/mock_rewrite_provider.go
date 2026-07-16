package contentitem

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrContentVersionNotFrozen  = errors.New("content version is not frozen")
	ErrReviewNotCompleted       = errors.New("review report is not completed")
	ErrSourceVersionMismatch    = errors.New("source version and review report do not match")
	ErrInvalidRewriteParameters = errors.New("invalid rewrite parameters")
	ErrMockRewriteFailed        = errors.New("mock rewrite failed")
)

const MockRewriteControlledFailureInputID = "mock_rewrite_fail"

type MockRewriteParameters struct {
	RewriteFocus   []string
	PreserveEnding bool
	Instructions   *string
}

type MockRewriteInput struct {
	ContentItem          ContentItem
	SourceContentVersion ContentVersion
	SourceReviewReport   ReviewReport
	Parameters           MockRewriteParameters
	BusinessInputID      string
}

type MockRewriteOutput struct {
	Title, Content, Summary string
	WordCount               int
	ProviderKey             string
	OutputSummary           []byte
}

type MockRewriteProvider interface {
	Rewrite(context.Context, MockRewriteInput) (MockRewriteOutput, error)
}

// DeterministicMockRewriteProvider is a deliberately small test provider, not
// a business-intelligence algorithm. It is pure in-memory and has no clock,
// randomness, repository, network, or process-state dependency.
type DeterministicMockRewriteProvider struct{}

func (DeterministicMockRewriteProvider) Rewrite(ctx context.Context, input MockRewriteInput) (MockRewriteOutput, error) {
	if err := ctx.Err(); err != nil {
		return MockRewriteOutput{}, err
	}
	if input.BusinessInputID == MockRewriteControlledFailureInputID {
		return MockRewriteOutput{}, ErrMockRewriteFailed
	}
	if err := validateMockRewriteInput(input); err != nil {
		return MockRewriteOutput{}, err
	}
	focus := strings.Join(input.Parameters.RewriteFocus, ", ")
	instructionState := "no instructions"
	if input.Parameters.Instructions != nil && strings.TrimSpace(*input.Parameters.Instructions) != "" {
		instructionState = "instructions supplied"
	}
	prefix := "[Deterministic mock rewrite: " + focus + "; " + instructionState + "]\n"
	content := prefix + input.SourceContentVersion.Content
	if !input.Parameters.PreserveEnding {
		content += "\n\nDeterministic mock rewrite completes the scene."
	}
	output := MockRewriteOutput{
		Title:       input.SourceContentVersion.Title + " (Mock Rewrite)",
		Content:     content,
		Summary:     "Deterministic mock rewrite focused on " + focus + ".",
		WordCount:   wordCount(content),
		ProviderKey: WorkflowProviderMock,
	}
	output.OutputSummary, _ = json.Marshal(struct {
		ProviderKey    string   `json:"provider_key"`
		RewriteFocus   []string `json:"rewrite_focus"`
		PreserveEnding bool     `json:"preserve_ending"`
		WordCount      int      `json:"word_count"`
	}{output.ProviderKey, append([]string(nil), input.Parameters.RewriteFocus...), input.Parameters.PreserveEnding, output.WordCount})
	return output, nil
}

func validateMockRewriteInput(input MockRewriteInput) error {
	if input.ContentItem.ID == uuid.Nil || strings.TrimSpace(input.BusinessInputID) == "" || input.SourceContentVersion.ID == uuid.Nil || input.SourceContentVersion.ContentItemID != input.ContentItem.ID || strings.TrimSpace(input.SourceContentVersion.Content) == "" {
		return ErrInvalidRewriteParameters
	}
	if input.SourceContentVersion.VersionNo != 1 || input.SourceContentVersion.Status != ContentVersionStatusFrozen {
		return ErrContentVersionNotFrozen
	}
	if input.SourceReviewReport.Status != "completed" {
		return ErrReviewNotCompleted
	}
	if input.SourceReviewReport.ID == uuid.Nil || input.SourceReviewReport.ProjectID != input.ContentItem.ProjectID || input.SourceReviewReport.ContentItemID != input.ContentItem.ID || input.SourceReviewReport.ContentVersionID != input.SourceContentVersion.ID {
		return ErrSourceVersionMismatch
	}
	if len(input.Parameters.RewriteFocus) == 0 || len(input.Parameters.RewriteFocus) > 4 || (input.Parameters.Instructions != nil && len(*input.Parameters.Instructions) > 2000) {
		return ErrInvalidRewriteParameters
	}
	seen := map[string]bool{}
	for _, focus := range input.Parameters.RewriteFocus {
		if seen[focus] || !oneOf(focus, "pacing", "foreshadowing", "character_consistency", "world_consistency") {
			return ErrInvalidRewriteParameters
		}
		seen[focus] = true
	}
	return nil
}

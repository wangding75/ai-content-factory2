package contentitem

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func mockRewriteProviderInput() MockRewriteInput {
	project, item, version, review := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	return MockRewriteInput{
		ContentItem:          ContentItem{ID: item, ProjectID: project, Title: "Chapter One"},
		SourceContentVersion: ContentVersion{ID: version, ContentItemID: item, VersionNo: 1, Version: 3, Status: ContentVersionStatusFrozen, Source: ContentVersionSourceMockGenerated, Title: "Chapter One", Content: "The lantern flickered. Mira chose the northern road."},
		SourceReviewReport:   ReviewReport{ID: review, ProjectID: project, ContentItemID: item, ContentVersionID: version, Status: "completed", Conclusion: "revise", Score: 70, Summary: "review"},
		Parameters:           MockRewriteParameters{RewriteFocus: []string{"pacing", "foreshadowing"}, PreserveEnding: true},
		BusinessInputID:      "stable-rewrite-input",
	}
}

func TestDeterministicMockRewriteProviderSuccess(t *testing.T) {
	in := mockRewriteProviderInput()
	before := in
	before.Parameters.RewriteFocus = append([]string(nil), in.Parameters.RewriteFocus...)
	p := DeterministicMockRewriteProvider{}
	out, err := p.Rewrite(context.Background(), in)
	if err != nil || out.ProviderKey != WorkflowProviderMock || out.WordCount != wordCount(out.Content) || !strings.Contains(out.Content, in.SourceContentVersion.Content) || strings.Contains(string(out.OutputSummary), "sql") {
		t.Fatalf("out=%+v err=%v", out, err)
	}
	if !reflect.DeepEqual(in, before) {
		t.Fatal("provider mutated input")
	}
	if !strings.HasSuffix(out.Content, in.SourceContentVersion.Content) {
		t.Fatal("preserve_ending did not retain source ending")
	}
}

func TestDeterministicMockRewriteProviderIsStableAndParameterSensitive(t *testing.T) {
	p, in := DeterministicMockRewriteProvider{}, mockRewriteProviderInput()
	first, err := p.Rewrite(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.Rewrite(context.Background(), in)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("first=%+v second=%+v err=%v", first, second, err)
	}
	changed := in
	changed.Parameters = MockRewriteParameters{RewriteFocus: []string{"world_consistency"}, PreserveEnding: false}
	different, err := p.Rewrite(context.Background(), changed)
	if err != nil || reflect.DeepEqual(first, different) || !strings.HasSuffix(different.Content, "Deterministic mock rewrite completes the scene.") {
		t.Fatalf("different=%+v err=%v", different, err)
	}
}

func TestDeterministicMockRewriteProviderValidation(t *testing.T) {
	p := DeterministicMockRewriteProvider{}
	for _, tc := range []struct {
		name   string
		change func(*MockRewriteInput)
		want   error
	}{
		{"not frozen", func(v *MockRewriteInput) { v.SourceContentVersion.Status = ContentVersionStatusEditableDraft }, ErrContentVersionNotFrozen},
		{"review incomplete", func(v *MockRewriteInput) { v.SourceReviewReport.Status = "running" }, ErrReviewNotCompleted},
		{"mismatched version", func(v *MockRewriteInput) { v.SourceReviewReport.ContentVersionID = uuid.New() }, ErrSourceVersionMismatch},
		{"invalid focus", func(v *MockRewriteInput) { v.Parameters.RewriteFocus = []string{"unknown"} }, ErrInvalidRewriteParameters},
		{"empty content", func(v *MockRewriteInput) { v.SourceContentVersion.Content = " " }, ErrInvalidRewriteParameters},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := mockRewriteProviderInput()
			tc.change(&in)
			_, err := p.Rewrite(context.Background(), in)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
		})
	}
}

func TestDeterministicMockRewriteProviderCancellationAndControlledFailure(t *testing.T) {
	p, in := DeterministicMockRewriteProvider{}, mockRewriteProviderInput()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Rewrite(ctx, in); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	in.BusinessInputID = MockRewriteControlledFailureInputID
	if _, err := p.Rewrite(context.Background(), in); !errors.Is(err, ErrMockRewriteFailed) || strings.Contains(err.Error(), "sql") {
		t.Fatalf("failure=%v", err)
	}
}

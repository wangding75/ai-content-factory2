package contentitem

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func validRewriteOutput(issueID uuid.UUID) json.RawMessage {
	value, _ := json.Marshal(RewriteRuntimeOutputV1{
		SchemaVersion: "rewrite.output.v1",
		Title:         "重写标题",
		Content:       "重写后的完整正文。",
		Summary:       "修复选中的问题。",
		AddressedIssues: []RewriteIssueOutcomeV1{{
			ReviewIssueID: issueID,
			Summary:       "已修复人物设定冲突。",
		}},
		UnresolvedIssues: []RewriteIssueOutcomeV1{},
		Warnings:         []string{},
		Metadata: &RewriteOutputMetadataV1{
			ChangeSummary: stringPointer("保留原叙事视角。"),
		},
	})
	return value
}

func TestDecodeRewriteRuntimeOutputStrictContract(t *testing.T) {
	selected := uuid.New()
	output, err := DecodeRewriteRuntimeOutput(validRewriteOutput(selected), []uuid.UUID{selected})
	if err != nil || output.SchemaVersion != "rewrite.output.v1" ||
		len(output.AddressedIssues) != 1 || output.AddressedIssues[0].ReviewIssueID != selected ||
		output.Metadata == nil || output.Metadata.ChangeSummary == nil {
		t.Fatalf("output=%+v err=%v", output, err)
	}
	nullMetadata := strings.Replace(string(validRewriteOutput(selected)), `"metadata":{"changeSummary":"保留原叙事视角。"}`, `"metadata":null`, 1)
	output, err = DecodeRewriteRuntimeOutput(json.RawMessage(nullMetadata), []uuid.UUID{selected})
	if err != nil || output.Metadata != nil {
		t.Fatalf("nullable metadata output=%+v err=%v", output, err)
	}
}

func TestDecodeRewriteRuntimeOutputRejectsFrozenInvalidShapes(t *testing.T) {
	selected, other := uuid.New(), uuid.New()
	valid := string(validRewriteOutput(selected))
	longWarnings := make([]string, 51)
	for index := range longWarnings {
		longWarnings[index] = "warning"
	}
	longWarningJSON, _ := json.Marshal(longWarnings)
	cases := []struct {
		name     string
		raw      json.RawMessage
		selected []uuid.UUID
	}{
		{"empty", nil, []uuid.UUID{selected}},
		{"wrong type", json.RawMessage(`[]`), []uuid.UUID{selected}},
		{"unknown field", json.RawMessage(strings.Replace(valid, `"metadata":`, `"candidateVersionId":"`+uuid.NewString()+`","metadata":`, 1)), []uuid.UUID{selected}},
		{"trailing JSON", json.RawMessage(valid + ` {}`), []uuid.UUID{selected}},
		{"multiple JSON", json.RawMessage(valid + valid), []uuid.UUID{selected}},
		{"duplicate key", json.RawMessage(strings.Replace(valid, `"title":"重写标题"`, `"title":"重写标题","title":"重复"`, 1)), []uuid.UUID{selected}},
		{"schema version", json.RawMessage(strings.Replace(valid, "rewrite.output.v1", "rewrite.output.v2", 1)), []uuid.UUID{selected}},
		{"missing title", json.RawMessage(strings.Replace(valid, `"title":"重写标题",`, "", 1)), []uuid.UUID{selected}},
		{"null array", json.RawMessage(strings.Replace(valid, `"warnings":[]`, `"warnings":null`, 1)), []uuid.UUID{selected}},
		{"blank title", json.RawMessage(strings.Replace(valid, "重写标题", "   ", 1)), []uuid.UUID{selected}},
		{"title too long", json.RawMessage(strings.Replace(valid, "重写标题", strings.Repeat("题", 121), 1)), []uuid.UUID{selected}},
		{"content too long", json.RawMessage(strings.Replace(valid, "重写后的完整正文。", strings.Repeat("文", 200001), 1)), []uuid.UUID{selected}},
		{"summary too long", json.RawMessage(strings.Replace(valid, "修复选中的问题。", strings.Repeat("摘", 5001), 1)), []uuid.UUID{selected}},
		{"warnings too many", json.RawMessage(strings.Replace(valid, `"warnings":[]`, `"warnings":`+string(longWarningJSON), 1)), []uuid.UUID{selected}},
		{"blank warning", json.RawMessage(strings.Replace(valid, `"warnings":[]`, `"warnings":[""]`, 1)), []uuid.UUID{selected}},
		{"invalid issue UUID", json.RawMessage(strings.Replace(valid, selected.String(), "not-a-uuid", 1)), []uuid.UUID{selected}},
		{"duplicate selected issue", validRewriteOutput(selected), []uuid.UUID{selected, selected}},
		{"duplicate outcome issue", json.RawMessage(strings.Replace(valid, `"unresolvedIssues":[]`, `"unresolvedIssues":[{"reviewIssueId":"`+selected.String()+`","summary":"重复"}]`, 1)), []uuid.UUID{selected}},
		{"cross selected issue", json.RawMessage(strings.Replace(valid, selected.String(), other.String(), 1)), []uuid.UUID{selected}},
		{"incomplete partition", json.RawMessage(strings.Replace(valid, `"addressedIssues":[{"reviewIssueId":"`+selected.String()+`","summary":"已修复人物设定冲突。"}]`, `"addressedIssues":[]`, 1)), []uuid.UUID{selected}},
		{"metadata unknown", json.RawMessage(strings.Replace(valid, `"changeSummary":"保留原叙事视角。"`, `"changeSummary":"保留原叙事视角。","extra":true`, 1)), []uuid.UUID{selected}},
		{"metadata missing field", json.RawMessage(strings.Replace(valid, `"metadata":{"changeSummary":"保留原叙事视角。"}`, `"metadata":{}`, 1)), []uuid.UUID{selected}},
		{"sensitive secret assignment", json.RawMessage(strings.Replace(valid, "修复选中的问题。", "secret=abcdef123456", 1)), []uuid.UUID{selected}},
		{"sensitive cookie", json.RawMessage(strings.Replace(valid, "修复选中的问题。", "Cookie: session=x", 1)), []uuid.UUID{selected}},
		{"database connection", json.RawMessage(strings.Replace(valid, "修复选中的问题。", "postgres://user:pass@127.0.0.1/db", 1)), []uuid.UUID{selected}},
		{"internal URL", json.RawMessage(strings.Replace(valid, "修复选中的问题。", "http://127.0.0.1:5678/internal", 1)), []uuid.UUID{selected}},
		{"SQL", json.RawMessage(strings.Replace(valid, "修复选中的问题。", "SELECT * FROM credentials", 1)), []uuid.UUID{selected}},
		{"stack", json.RawMessage(strings.Replace(valid, "修复选中的问题。", "stack trace follows", 1)), []uuid.UUID{selected}},
		{"illegal encoding", json.RawMessage([]byte{'{', '"', 't', 'i', 't', 'l', 'e', '"', ':', '"', 0xff, '"', '}'}), []uuid.UUID{selected}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, decodeErr := DecodeRewriteRuntimeOutput(testCase.raw, testCase.selected); !errors.Is(decodeErr, ErrRewriteOutputInvalid) {
				t.Fatalf("error=%v", decodeErr)
			}
		})
	}
}

func TestDecodeRewriteRuntimeOutputAllowsOrdinaryTechnicalProse(t *testing.T) {
	selected := uuid.New()
	raw := validRewriteOutput(selected)
	raw = json.RawMessage(strings.Replace(
		string(raw),
		"修复选中的问题。",
		"正文讨论 password、token、webhook、n8n、SQL 与 secret 等普通概念。",
		1,
	))
	if _, err := DecodeRewriteRuntimeOutput(raw, []uuid.UUID{selected}); err != nil {
		t.Fatalf("ordinary technical prose rejected: %v", err)
	}
}

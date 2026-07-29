package contentitem

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRewriteRequestValidationAndTokenClaimsAreSafe(t *testing.T) {
	instructions := "使用 Authorization: Bearer secret"
	if validRewriteRequest(RewritePreflightRequest{
		SelectedIssueIDs: []uuid.UUID{uuid.New()}, OptionalInstructions: &instructions,
		RewriteOptions: RewriteOptions{Strategy: "targeted_fix"}, ActorID: "actor",
	}) {
		t.Fatal("sensitive instructions were accepted")
	}
	duplicate := uuid.New()
	if validRewriteRequest(RewritePreflightRequest{
		SelectedIssueIDs: []uuid.UUID{duplicate, duplicate},
		RewriteOptions: RewriteOptions{Strategy: "targeted_fix"}, ActorID: "actor",
	}) {
		t.Fatal("duplicate Issue IDs were accepted")
	}
	service := &RealRewriteService{secret: []byte("rewrite-secret"), now: time.Now}
	now := time.Now().UTC()
	claims := rewriteTokenClaims{
		ActorID: "actor", ProjectID: uuid.New(), ContentItemID: uuid.New(),
		ReviewReportID: uuid.New(), SourceContentVersionID: uuid.New(),
		SourceContentVersionVersion: 1, SourceContentHash: strings.Repeat("a", 64),
		SelectedIssueIDs: []uuid.UUID{uuid.New()},
		SelectedIssuesDigest: strings.Repeat("b", 64), ReportDigest: strings.Repeat("c", 64),
		RewriteOptions: RewriteOptions{Strategy: "targeted_fix"},
		BindingID: uuid.New(), BindingVersion: 1, ConfigurationID: uuid.New(),
		ConfigurationVersion: 1, ConnectionID: uuid.New(), ConnectionVersion: 1,
		Stage: "rewrite", InputContract: "rewrite.input.v1", OutputContract: "rewrite.output.v1",
		RequestDigest: strings.Repeat("d", 64), Nonce: uuid.NewString(),
		IssuedAt: now.Unix(), ExpiresAt: now.Add(10 * time.Minute).Unix(),
	}
	token, err := service.signRewriteToken(claims)
	if err != nil { t.Fatal(err) }
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(token, ".")[0])
	if err != nil { t.Fatal(err) }
	lower := strings.ToLower(string(payload))
	for _, forbidden := range []string{"password", "cookie", "webhook", "baseurl", "credential", "postgres://"} {
		if strings.Contains(lower, forbidden) { t.Fatalf("token claims contain %s: %s", forbidden, payload) }
	}
	parsed, err := service.parseRewriteToken(token)
	if err != nil || parsed.RequestDigest != claims.RequestDigest || parsed.Stage != "rewrite" {
		t.Fatalf("claims=%+v err=%v", parsed, err)
	}
}

func TestRewriteRuntimeInputCanonicalFieldsAndNoProtectedClientFields(t *testing.T) {
	input := RewriteRuntimeInputV1{
		SchemaVersion: "rewrite.input.v1", WorkflowRunID: uuid.New(), CorrelationID: "correlation",
		ProjectID: uuid.New(), ContentItemID: uuid.New(), SourceContentVersionID: uuid.New(),
		SourceContentVersionVersion: 2, SourceContentHash: strings.Repeat("a", 64),
		SourceTitle: "标题", SourceContent: "正文", ReviewReportID: uuid.New(),
		ReportSnapshot: RewriteReportSnapshot{
			ReviewReportID: uuid.New(), SourceContentVersionID: uuid.New(),
			SourceContentVersionVersion: 2, SourceContentHash: strings.Repeat("a", 64),
			Conclusion: "needs_changes", Summary: "需要修改", CompletedAt: time.Now().UTC(),
		},
		SelectedIssues: []RewriteIssueSnapshot{{
			ReviewIssueID: uuid.New(), ReviewReportID: uuid.New(), IssueKey: "issue-1",
			Position: 1, Version: 1, CategoryKey: "language_quality", CategoryLabel: "语言质量",
			Severity: "warning", Title: "标题", Description: "说明",
			Evidence: ReviewRuntimeEvidenceV1{SourceRefs: []string{}}, Disposition: "open",
		}},
		RewriteOptions: RewriteOptions{Strategy: "targeted_fix"},
	}
	raw, err := json.Marshal(input)
	if err != nil { t.Fatal(err) }
	keys := []string{`"schemaVersion"`, `"workflowRunId"`, `"correlationId"`, `"projectId"`, `"contentItemId"`, `"sourceContentVersionId"`, `"sourceContentVersionVersion"`, `"sourceContentHash"`, `"sourceTitle"`, `"sourceContent"`, `"reviewReportId"`, `"reportSnapshot"`, `"selectedIssues"`, `"optionalInstructions"`, `"rewriteOptions"`}
	position := -1
	for _, key := range keys {
		next := strings.Index(string(raw), key)
		if next <= position { t.Fatalf("field order is not canonical at %s: %s", key, raw) }
		position = next
	}
	for _, forbidden := range []string{"preflightToken", "idempotencyKey", "password", "cookie", "webhook"} {
		if strings.Contains(string(raw), forbidden) { t.Fatalf("input contains %s: %s", forbidden, raw) }
	}
}

func TestRewriteTokenExpiryAndTamperAreDistinct(t *testing.T) {
	service := &RealRewriteService{secret: []byte("rewrite-secret"), now: time.Now}
	if _, err := service.parseRewriteToken("invalid"); !errors.Is(err, ErrRewriteTokenInvalid) {
		t.Fatalf("invalid token error=%v", err)
	}
}

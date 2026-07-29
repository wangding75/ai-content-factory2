package contentitem

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestContentVersionRewriteShapeAcceptsExistingSourcesAndFrozenV2(t *testing.T) {
	for _, version := range []ContentVersion{
		{Source: ContentVersionSourceManualCreated, VersionNo: 1, Status: ContentVersionStatusEditableDraft},
		{Source: ContentVersionSourceMockGenerated, VersionNo: 1, Status: ContentVersionStatusFrozen},
		{Source: ContentVersionSourceMockRewrite, VersionNo: 2, Status: ContentVersionStatusEditableDraft},
	} {
		if err := version.ValidateRewriteShape(); err != nil {
			t.Fatalf("version=%+v err=%v", version, err)
		}
	}
}

func TestContentVersionRewriteShapeRejectsInvalidV2Forms(t *testing.T) {
	for _, version := range []ContentVersion{
		{Source: ContentVersionSourceMockRewrite, VersionNo: 1, Status: ContentVersionStatusEditableDraft},
		{Source: ContentVersionSourceMockRewrite, VersionNo: 2, Status: ContentVersionStatusFrozen},
		{Source: "unexpected", VersionNo: 1, Status: ContentVersionStatusEditableDraft},
	} {
		if !errors.Is(version.ValidateRewriteShape(), ErrInvalidContentVersion) {
			t.Fatalf("version=%+v", version)
		}
	}
}

func TestContentVersionWorkflowGeneratedShapeRequiresCompleteSourceTrace(t *testing.T) {
	sourceVersionID, sourceRunID := uuid.New(), uuid.New()
	sourceVersion := 3
	candidate := ContentVersion{
		Source: ContentVersionSourceWorkflowGenerated, VersionNo: 4, Status: ContentVersionStatusEditableDraft, Version: 1,
		SourceContentVersionID: &sourceVersionID, SourceContentVersionVersion: &sourceVersion, SourceWorkflowRunID: &sourceRunID,
	}
	if err := candidate.ValidateRewriteShape(); err != nil { t.Fatalf("candidate=%+v err=%v", candidate, err) }
	candidate.SourceWorkflowRunID = nil
	if !errors.Is(candidate.ValidateRewriteShape(), ErrInvalidContentVersion) { t.Fatal("candidate without source run accepted") }
}

func TestContentVersionWorkflowRewriteShapeRequiresFrozenLineage(t *testing.T) {
	candidateID, sourceVersionID, sourceRunID := uuid.New(), uuid.New(), uuid.New()
	sourceVersion := 4
	candidate := ContentVersion{
		ID: candidateID, Source: ContentVersionSourceWorkflowRewrite, VersionNo: 5,
		Status: ContentVersionStatusEditableDraft, Version: 1,
		SourceContentVersionID: &sourceVersionID,
		SourceContentVersionVersion: &sourceVersion,
		SourceWorkflowRunID: &sourceRunID,
	}
	if err := candidate.ValidateRewriteShape(); err != nil {
		t.Fatalf("candidate=%+v err=%v", candidate, err)
	}
	for _, invalid := range []ContentVersion{
		{ID: candidateID, Source: ContentVersionSourceWorkflowRewrite, Status: ContentVersionStatusEditableDraft, Version: 1},
		{ID: candidateID, Source: ContentVersionSourceWorkflowRewrite, Status: ContentVersionStatusFrozen, Version: 1, SourceContentVersionID: &sourceVersionID, SourceContentVersionVersion: &sourceVersion, SourceWorkflowRunID: &sourceRunID},
		{ID: candidateID, Source: ContentVersionSourceWorkflowRewrite, Status: ContentVersionStatusEditableDraft, Version: 2, SourceContentVersionID: &sourceVersionID, SourceContentVersionVersion: &sourceVersion, SourceWorkflowRunID: &sourceRunID},
		{ID: candidateID, Source: ContentVersionSourceWorkflowRewrite, Status: ContentVersionStatusEditableDraft, Version: 1, SourceContentVersionID: &candidateID, SourceContentVersionVersion: &sourceVersion, SourceWorkflowRunID: &sourceRunID},
	} {
		if !errors.Is(invalid.ValidateRewriteShape(), ErrInvalidContentVersion) {
			t.Fatalf("invalid candidate accepted: %+v", invalid)
		}
	}
}

func TestWorkflowRunRewriteShapeSucceedsAndFailsWithFrozenNullability(t *testing.T) {
	reviewID, targetID := uuid.New(), uuid.New()
	finished := time.Now().UTC()
	succeeded := WorkflowRun{
		ProjectID: uuid.New(), ContentItemID: uuid.New(), ContentVersionID: uuid.New(), SourceReviewReportID: &reviewID,
		TargetContentVersionID: &targetID, ProviderKey: WorkflowProviderMock, WorkflowKey: WorkflowKeyMockRewrite,
		Status: WorkflowRunStatusSucceeded, OutputJSON: []byte(`{"target_version_no":2}`), FinishedAt: &finished,
	}
	if err := succeeded.ValidateRewriteShape(); err != nil {
		t.Fatalf("succeeded=%+v err=%v", succeeded, err)
	}
	code, summary := "mock_rewrite_failed", "mock rewrite failed"
	failed := succeeded
	failed.Status, failed.TargetContentVersionID, failed.OutputJSON = WorkflowRunStatusFailed, nil, nil
	failed.ErrorCode, failed.ErrorSummary = &code, &summary
	if err := failed.ValidateRewriteShape(); err != nil {
		t.Fatalf("failed=%+v err=%v", failed, err)
	}
	failed.TargetContentVersionID = &targetID
	if !errors.Is(failed.ValidateRewriteShape(), ErrInvalidMockRewriteRun) {
		t.Fatal("failed rewrite accepted a target version")
	}
}

func TestContentVersionLineageKeepsV1RelationsNullable(t *testing.T) {
	if !(ContentVersionLineage{}).Empty() {
		t.Fatal("v1 lineage must be nullable")
	}
	v1 := ContentVersion{ID: uuid.New(), VersionNo: 1, Status: ContentVersionStatusFrozen}
	if (ContentVersionLineage{SourceContentVersion: &v1}).Empty() {
		t.Fatal("source relation was discarded")
	}
}

func TestRewriteRepositoryContractsUseDomainTypesAndFrozenPagination(t *testing.T) {
	for _, contract := range []reflect.Type{
		reflect.TypeOf((*ContentVersionRepository)(nil)).Elem(),
		reflect.TypeOf((*WorkflowRunRepository)(nil)).Elem(),
	} {
		for i := 0; i < contract.NumMethod(); i++ {
			method := contract.Method(i)
			for j := 0; j < method.Type.NumIn(); j++ {
				assertDomainContractType(t, method.Type.In(j))
			}
			for j := 0; j < method.Type.NumOut(); j++ {
				assertDomainContractType(t, method.Type.Out(j))
			}
		}
	}
	if ContentVersionListOrder != "version_no DESC, id DESC" {
		t.Fatalf("sort=%q", ContentVersionListOrder)
	}
	if reflect.TypeOf(ContentVersionPageOptions{}).Field(0).Type.Kind() != reflect.Int {
		t.Fatal("pagination must use the existing integer model")
	}
	var _ ContentVersionRepository = (*fakeContentVersionRepository)(nil)
	var _ WorkflowRunRepository = (*fakeWorkflowRunRepository)(nil)
}

func assertDomainContractType(t *testing.T, typ reflect.Type) {
	t.Helper()
	if strings.Contains(typ.PkgPath(), "platform/httpserver") || strings.Contains(typ.String(), "Envelope") {
		t.Fatalf("HTTP DTO leaked into repository contract: %s", typ)
	}
}

type fakeContentVersionRepository struct{}

func (*fakeContentVersionRepository) GetContentVersion(context.Context, uuid.UUID) (ContentVersion, error) {
	return ContentVersion{}, nil
}
func (*fakeContentVersionRepository) ListContentVersions(context.Context, uuid.UUID, ContentVersionPageOptions) (ContentVersionPage, error) {
	return ContentVersionPage{}, nil
}
func (*fakeContentVersionRepository) CountContentVersions(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}
func (*fakeContentVersionRepository) CreateContentVersion(context.Context, pgx.Tx, ContentVersion) (ContentVersion, error) {
	return ContentVersion{}, nil
}
func (*fakeContentVersionRepository) GetContentVersionByNumber(context.Context, uuid.UUID, int) (ContentVersion, error) {
	return ContentVersion{}, nil
}

type fakeWorkflowRunRepository struct{}

func (*fakeWorkflowRunRepository) GetWorkflowRun(context.Context, uuid.UUID) (WorkflowRun, error) {
	return WorkflowRun{}, nil
}
func (*fakeWorkflowRunRepository) CreateMockRewriteRunning(context.Context, pgx.Tx, WorkflowRun) (WorkflowRun, error) {
	return WorkflowRun{}, nil
}
func (*fakeWorkflowRunRepository) MarkMockRewriteSucceeded(context.Context, pgx.Tx, uuid.UUID, uuid.UUID, []byte, time.Time) (WorkflowRun, error) {
	return WorkflowRun{}, nil
}
func (*fakeWorkflowRunRepository) MarkMockRewriteFailed(context.Context, pgx.Tx, uuid.UUID, string, string, time.Time) (WorkflowRun, error) {
	return WorkflowRun{}, nil
}
func (*fakeWorkflowRunRepository) FindMockRewriteByIdempotencyKey(context.Context, uuid.UUID, string) (WorkflowRun, error) {
	return WorkflowRun{}, nil
}
func (*fakeWorkflowRunRepository) FindMockRewriteByFingerprint(context.Context, uuid.UUID, string) (WorkflowRun, error) {
	return WorkflowRun{}, nil
}
func (*fakeWorkflowRunRepository) LatestMockRewrite(context.Context, uuid.UUID) (WorkflowRun, error) {
	return WorkflowRun{}, nil
}

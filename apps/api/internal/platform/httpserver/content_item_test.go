package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
)

type fakeContentItemApplication struct {
	create                                                                                        contentitem.CreateResult
	detail                                                                                        contentitem.Detail
	generated                                                                                     contentitem.MockGenerateResult
	reviewed                                                                                      contentitem.MockReviewResult
	reviews                                                                                       contentitem.ReviewList
	reviewDetail                                                                                  contentitem.ReviewDetail
	err                                                                                           error
	createCalls, getCalls, saveCalls, generateCalls, reviewCalls, listReviewCalls, getReviewCalls int
	createCommand                                                                                 contentitem.CreateOrGetCommand
	getCommand                                                                                    contentitem.GetCommand
	saveCommand                                                                                   contentitem.SaveDraftCommand
	generateCommand                                                                               contentitem.MockGenerateCommand
	reviewCommand                                                                                 contentitem.MockReviewCommand
	listReviewCommand                                                                             contentitem.ListReviewsCommand
	getReviewCommand                                                                              contentitem.GetReviewCommand
}

func (f *fakeContentItemApplication) CreateOrGet(_ context.Context, c contentitem.CreateOrGetCommand) (contentitem.CreateResult, error) {
	f.createCalls++
	f.createCommand = c
	return f.create, f.err
}
func (f *fakeContentItemApplication) Get(_ context.Context, c contentitem.GetCommand) (contentitem.Detail, error) {
	f.getCalls++
	f.getCommand = c
	return f.detail, f.err
}
func (f *fakeContentItemApplication) SaveDraft(_ context.Context, c contentitem.SaveDraftCommand) (contentitem.Detail, error) {
	f.saveCalls++
	f.saveCommand = c
	return f.detail, f.err
}
func (f *fakeContentItemApplication) MockGenerate(_ context.Context, c contentitem.MockGenerateCommand) (contentitem.MockGenerateResult, error) {
	f.generateCalls++
	f.generateCommand = c
	return f.generated, f.err
}
func (f *fakeContentItemApplication) MockReview(_ context.Context, c contentitem.MockReviewCommand) (contentitem.MockReviewResult, error) {
	f.reviewCalls++
	f.reviewCommand = c
	return f.reviewed, f.err
}
func (f *fakeContentItemApplication) ListReviews(_ context.Context, c contentitem.ListReviewsCommand) (contentitem.ReviewList, error) {
	f.listReviewCalls++
	f.listReviewCommand = c
	return f.reviews, f.err
}
func (f *fakeContentItemApplication) GetReview(_ context.Context, c contentitem.GetReviewCommand) (contentitem.ReviewDetail, error) {
	f.getReviewCalls++
	f.getReviewCommand = c
	return f.reviewDetail, f.err
}
func contentHTTPDetail() contentitem.Detail {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	item := uuid.New()
	version := uuid.New()
	return contentitem.Detail{Item: contentitem.ContentItem{ID: item, ChapterPlanID: uuid.New(), Title: "Chapter", Status: "draft", CurrentVersionID: version, CreatedAt: now, UpdatedAt: now}, CurrentVersion: contentitem.ContentVersion{ID: version, ContentItemID: item, VersionNo: 1, Version: 2, Status: "editable_draft", Source: "manual_created", Title: "Chapter", Content: "Text", WordCount: 1, CreatedAt: now, UpdatedAt: now}}
}
func contentRequest(h http.Handler, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if p == "chapter-plans" && i+1 < len(parts) {
			r.SetPathValue("chapterPlanId", parts[i+1])
		}
		if p == "content-items" && i+1 < len(parts) {
			r.SetPathValue("contentItemId", parts[i+1])
		}
		if p == "reviews" && i+1 < len(parts) && parts[i+1] != "mock" {
			r.SetPathValue("reviewId", parts[i+1])
		}
	}
	withRequestID(h).ServeHTTP(w, r)
	return w
}
func requireContentError(t *testing.T, w *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if w.Code != status || !strings.Contains(w.Body.String(), `"code":"`+code+`"`) || !strings.Contains(w.Body.String(), `"request_id":"req_`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestContentItemCreateOrGetHandler(t *testing.T) {
	id := uuid.New()
	f := &fakeContentItemApplication{create: contentitem.CreateResult{Detail: contentHTTPDetail(), Created: true}}
	w := contentRequest(createContentItemHandler(f), http.MethodPost, "/api/v1/chapter-plans/"+id.String()+"/content", "", nil)
	if w.Code != 201 || f.createCalls != 1 || f.createCommand.ChapterPlanID != id || !strings.Contains(w.Body.String(), `"content_item"`) {
		t.Fatalf("%d %s %#v", w.Code, w.Body.String(), f)
	}
	f.create.Created = false
	w = contentRequest(createContentItemHandler(f), http.MethodPost, "/api/v1/chapter-plans/"+id.String()+"/content", "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	w = contentRequest(createContentItemHandler(f), http.MethodPost, "/api/v1/chapter-plans/"+id.String()+"/content", `{}`, nil)
	requireContentError(t, w, 400, "validation_error")
	w = contentRequest(createContentItemHandler(f), http.MethodPost, "/api/v1/chapter-plans/bad/content", "", nil)
	requireContentError(t, w, 400, "invalid_uuid")
	for _, x := range []struct {
		e error
		s int
		c string
	}{{contentitem.ErrChapterPlanNotFound, 404, "chapter_plan_not_found"}, {contentitem.ErrChapterPlanNotConfirmed, 409, "chapter_plan_not_confirmed"}, {errors.New("private cause"), 500, "internal_error"}} {
		f.err = x.e
		w = contentRequest(createContentItemHandler(f), http.MethodPost, "/api/v1/chapter-plans/"+id.String()+"/content", "", nil)
		requireContentError(t, w, x.s, x.c)
		if strings.Contains(w.Body.String(), "private cause") {
			t.Fatal("leak")
		}
	}
}
func TestContentItemGetHandler(t *testing.T) {
	d := contentHTTPDetail()
	f := &fakeContentItemApplication{detail: d}
	w := contentRequest(getContentItemHandler(f), http.MethodGet, "/api/v1/content-items/"+d.Item.ID.String(), "", nil)
	if w.Code != 200 || f.getCalls != 1 || f.getCommand.ContentItemID != d.Item.ID || !strings.Contains(w.Body.String(), `"current_version"`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = contentRequest(getContentItemHandler(f), http.MethodGet, "/api/v1/content-items/bad", "", nil)
	requireContentError(t, w, 400, "invalid_uuid")
	f.err = contentitem.ErrContentItemNotFound
	w = contentRequest(getContentItemHandler(f), http.MethodGet, "/api/v1/content-items/"+d.Item.ID.String(), "", nil)
	requireContentError(t, w, 404, "content_item_not_found")
}
func TestContentItemSaveDraftHandler(t *testing.T) {
	d := contentHTTPDetail()
	f := &fakeContentItemApplication{detail: d}
	path := "/api/v1/content-items/" + d.Item.ID.String() + "/draft"
	w := contentRequest(saveContentDraftHandler(f), http.MethodPut, path, `{"expected_version":2,"title":"New","content":"","summary":null}`, nil)
	if w.Code != 200 || f.saveCalls != 1 || f.saveCommand.ExpectedVersion != 2 || f.saveCommand.Title.Value == nil || *f.saveCommand.Title.Value != "New" || !f.saveCommand.Content.Set || f.saveCommand.Content.Value == nil || *f.saveCommand.Content.Value != "" || !f.saveCommand.Summary.Set || f.saveCommand.Summary.Value != nil {
		t.Fatalf("%d %#v", w.Code, f.saveCommand)
	}
	w = contentRequest(saveContentDraftHandler(f), http.MethodPut, path, `{"expected_version":2,"content":"x"}`, nil)
	if !f.saveCommand.Content.Set || f.saveCommand.Title.Set || f.saveCommand.Summary.Set {
		t.Fatalf("omitted %#v", f.saveCommand)
	}
	for _, body := range []string{`{`, `{"expected_version":2,"content":"x","id":"bad"}`, `{"expected_version":2,"title":null}`, `{"expected_version":2}`} {
		w = contentRequest(saveContentDraftHandler(f), http.MethodPut, path, body, nil)
		requireContentError(t, w, 400, "validation_error")
	}
	for _, x := range []struct {
		e error
		s int
		c string
	}{{contentitem.ErrContentVersionLocked, 409, "content_version_locked"}, {contentitem.ErrVersionConflict, 409, "version_conflict"}, {errors.New("hidden"), 500, "internal_error"}} {
		f.err = x.e
		w = contentRequest(saveContentDraftHandler(f), http.MethodPut, path, `{"expected_version":2,"content":"x"}`, nil)
		requireContentError(t, w, x.s, x.c)
	}
}
func TestContentItemMockGenerateHandler(t *testing.T) {
	d := contentHTTPDetail()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	f := &fakeContentItemApplication{generated: contentitem.MockGenerateResult{Detail: d, WorkflowRun: contentitem.WorkflowRun{ID: uuid.New(), ProviderKey: "mock", WorkflowKey: "content_mock_generate", Status: "succeeded", StartedAt: now}}}
	path := "/api/v1/content-items/" + d.Item.ID.String() + "/mock-generate"
	ref := uuid.New()
	body := `{"expected_version":2,"parameters":{"chapter_goal":null,"storyline_refs_json":["` + ref.String() + `"],"material_refs_json":[],"foreshadowing_refs_json":[],"creation_notes":""}}`
	w := contentRequest(mockGenerateContentHandler(f), http.MethodPost, path, body, map[string]string{"Idempotency-Key": " key "})
	if w.Code != 200 || f.generateCalls != 1 || f.generateCommand.IdempotencyKey != "key" || f.generateCommand.ExpectedVersion != 2 || !f.generateCommand.Parameters.ChapterGoal.Set || f.generateCommand.Parameters.ChapterGoal.Value != nil || f.generateCommand.Parameters.CreationNotes.Value == nil || *f.generateCommand.Parameters.CreationNotes.Value != "" || len(f.generateCommand.Parameters.StorylineRefs.Value) != 1 {
		t.Fatalf("%d %s %#v", w.Code, w.Body.String(), f.generateCommand)
	}
	for _, h := range []map[string]string{nil, {"Idempotency-Key": "  "}} {
		w = contentRequest(mockGenerateContentHandler(f), http.MethodPost, path, body, h)
		requireContentError(t, w, 400, "idempotency_key_required")
	}
	for _, bad := range []string{`{`, strings.Replace(body, `"creation_notes":""`, `"creation_notes":"","version":1`, 1)} {
		w = contentRequest(mockGenerateContentHandler(f), http.MethodPost, path, bad, map[string]string{"Idempotency-Key": "x"})
		requireContentError(t, w, 400, "validation_error")
	}
	for _, x := range []struct {
		e error
		s int
		c string
	}{{contentitem.ErrInvalidGenerationParameters, 422, "invalid_generation_parameters"}, {contentitem.ErrIdempotencyConflict, 409, "idempotency_key_reused_with_different_payload"}, {contentitem.ErrMockGenerationFailed, 422, "mock_generation_failed"}, {contentitem.ErrVersionConflict, 409, "version_conflict"}, {errors.New("provider secret"), 500, "internal_error"}} {
		f.err = x.e
		w = contentRequest(mockGenerateContentHandler(f), http.MethodPost, path, body, map[string]string{"Idempotency-Key": "x"})
		requireContentError(t, w, x.s, x.c)
		if strings.Contains(w.Body.String(), "provider secret") {
			t.Fatal("leak")
		}
	}
}

func reviewHTTPDetail() contentitem.ReviewDetail {
	now := time.Date(2026, 7, 15, 12, 3, 0, 0, time.UTC)
	itemID, versionID, reviewID, runID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	return contentitem.ReviewDetail{
		Review:          contentitem.ReviewReport{ID: reviewID, ContentItemID: itemID, ContentVersionID: versionID, WorkflowRunID: runID, ProviderKey: "mock", Status: "completed", Conclusion: "revise", Score: 70, Summary: "Review", CreatedAt: now, CompletedAt: now},
		ContentVersion:  contentitem.ContentVersion{ID: versionID, ContentItemID: itemID, VersionNo: 1, Version: 2, Title: "Frozen", WordCount: 12, Source: "mock_generated", Status: "frozen", FrozenAt: &now},
		Findings:        []contentitem.ReviewFinding{{ID: uuid.New(), ReviewID: reviewID, Category: "pacing", Severity: "high", Title: "first", Description: "d", LocationJSON: []byte(`{"start_offset":0,"end_offset":1}`)}, {ID: uuid.New(), ReviewID: reviewID, Category: "foreshadowing", Severity: "low", Title: "second", Description: "d"}},
		Recommendations: []contentitem.ReviewRecommendation{{ID: uuid.New(), ReviewID: reviewID, Priority: "high", Title: "first", Description: "d", CreatedAt: now}, {ID: uuid.New(), ReviewID: reviewID, Priority: "low", Title: "second", Description: "d", CreatedAt: now}},
		WorkflowRun:     contentitem.WorkflowRun{ID: runID, ProviderKey: "mock", WorkflowKey: "content_mock_review", Status: "succeeded", StartedAt: now, FinishedAt: &now},
	}
}

func TestMockReviewHandler(t *testing.T) {
	d := reviewHTTPDetail()
	itemID := d.Review.ContentItemID
	f := &fakeContentItemApplication{reviewed: contentitem.MockReviewResult{Detail: contentitem.Detail{Item: contentitem.ContentItem{ID: itemID, ChapterPlanID: uuid.New(), CurrentVersionID: d.ContentVersion.ID, Status: "reviewed"}, CurrentVersion: d.ContentVersion}, Review: d.Review, Findings: d.Findings, Recommendations: d.Recommendations, WorkflowRun: d.WorkflowRun}}
	path := "/api/v1/content-items/" + itemID.String() + "/reviews/mock"
	body := `{"content_version_id":"` + d.ContentVersion.ID.String() + `","expected_version":2}`
	w := contentRequest(mockReviewContentHandler(f), http.MethodPost, path, body, map[string]string{"Idempotency-Key": " key "})
	if w.Code != 200 || f.reviewCalls != 1 || f.reviewCommand.ContentItemID != itemID || f.reviewCommand.ContentVersionID != d.ContentVersion.ID || f.reviewCommand.ExpectedVersion != 2 || f.reviewCommand.IdempotencyKey != "key" || strings.Contains(w.Body.String(), "running") || strings.Contains(w.Body.String(), "in_review") {
		t.Fatalf("%d %s %#v", w.Code, w.Body.String(), f.reviewCommand)
	}
	for _, h := range []map[string]string{nil, {"Idempotency-Key": "  "}} {
		requireContentError(t, contentRequest(mockReviewContentHandler(f), http.MethodPost, path, body, h), 400, "idempotency_key_required")
	}
	for _, bad := range []string{`{`, `{"content_version_id":"bad","expected_version":2}`, `{"content_version_id":"` + d.ContentVersion.ID.String() + `","expected_version":2,"extra":true}`} {
	requireContentError(t, contentRequest(mockReviewContentHandler(f), http.MethodPost, path, bad, map[string]string{"Idempotency-Key": "x"}), 422, "invalid_review_parameters")
	}
	requireContentError(t, contentRequest(mockReviewContentHandler(f), http.MethodPost, "/api/v1/content-items/bad/reviews/mock", body, map[string]string{"Idempotency-Key": "x"}), 400, "invalid_uuid")
	for _, x := range []struct {
		e error
		s int
		c string
	}{{contentitem.ErrInvalidReviewParameters, 422, "invalid_review_parameters"}, {contentitem.ErrContentItemNotFound, 404, "content_item_not_found"}, {contentitem.ErrContentVersionNotFound, 404, "content_version_not_found"}, {contentitem.ErrContentVersionLocked, 409, "content_version_locked"}, {contentitem.ErrContentVersionReviewed, 409, "content_version_already_reviewed"}, {contentitem.ErrVersionConflict, 409, "version_conflict"}, {contentitem.ErrIdempotencyConflict, 409, "idempotency_key_reused_with_different_payload"}, {contentitem.ErrMockReviewFailed, 422, "mock_review_failed"}, {errors.New("generator secret"), 500, "internal_error"}} {
		f.err = x.e
		w = contentRequest(mockReviewContentHandler(f), http.MethodPost, path, body, map[string]string{"Idempotency-Key": "x"})
		requireContentError(t, w, x.s, x.c)
		if strings.Contains(w.Body.String(), "generator secret") {
			t.Fatal("leak")
		}
	}
}

func TestReviewListHandler(t *testing.T) {
	d := reviewHTTPDetail()
	f := &fakeContentItemApplication{reviews: contentitem.ReviewList{Items: []contentitem.ReviewReport{d.Review}, Total: 1, Limit: 20, Offset: 0}}
	path := "/api/v1/content-items/" + d.Review.ContentItemID.String() + "/reviews"
	w := contentRequest(listReviewsHandler(f), http.MethodGet, path, "", nil)
	if w.Code != 200 || f.listReviewCalls != 1 || f.listReviewCommand.Limit != 20 || f.listReviewCommand.Offset != 0 || !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatalf("%d %s %#v", w.Code, w.Body.String(), f)
	}
	f.reviews.Limit, f.reviews.Offset = 5, 3
	w = contentRequest(listReviewsHandler(f), http.MethodGet, path+"?limit=5&offset=3", "", nil)
	if w.Code != 200 || f.listReviewCalls != 2 || f.listReviewCommand.Limit != 5 || f.listReviewCommand.Offset != 3 {
		t.Fatalf("%d %#v", w.Code, f.listReviewCommand)
	}
	for _, q := range []string{"?limit=0", "?limit=x", "?offset=-1", "?offset=x"} {
		requireContentError(t, contentRequest(listReviewsHandler(f), http.MethodGet, path+q, "", nil), 400, "invalid_pagination")
	}
	requireContentError(t, contentRequest(listReviewsHandler(f), http.MethodGet, "/api/v1/content-items/bad/reviews", "", nil), 400, "invalid_uuid")
}

func TestReviewDetailHandler(t *testing.T) {
	d := reviewHTTPDetail()
	f := &fakeContentItemApplication{reviewDetail: d}
	path := "/api/v1/reviews/" + d.Review.ID.String()
	w := contentRequest(getReviewHandler(f), http.MethodGet, path, "", nil)
	if w.Code != 200 || f.getReviewCalls != 1 || f.getReviewCommand.ReviewID != d.Review.ID || !strings.Contains(w.Body.String(), `"title":"Frozen"`) || strings.Index(w.Body.String(), `"title":"first"`) > strings.Index(w.Body.String(), `"title":"second"`) {
		t.Fatalf("%d %s %#v", w.Code, w.Body.String(), f)
	}
	requireContentError(t, contentRequest(getReviewHandler(f), http.MethodGet, "/api/v1/reviews/bad", "", nil), 400, "invalid_uuid")
	for _, x := range []struct {
		e error
		s int
		c string
	}{{contentitem.ErrReviewNotFound, 404, "review_not_found"}, {errors.New("database host"), 500, "internal_error"}} {
		f.err = x.e
		w = contentRequest(getReviewHandler(f), http.MethodGet, path, "", nil)
		requireContentError(t, w, x.s, x.c)
		if strings.Contains(w.Body.String(), "database host") {
			t.Fatal("leak")
		}
	}
}
func TestContentItemRoutesRegistered(t *testing.T) {
	f := &fakeContentItemApplication{create: contentitem.CreateResult{Detail: contentHTTPDetail()}}
	h := New(":0", nil, f).httpServer.Handler
	id := uuid.New()
	for _, x := range []struct{ m, p, b string }{{http.MethodPost, "/api/v1/chapter-plans/" + id.String() + "/content", ""}, {http.MethodGet, "/api/v1/content-items/" + id.String(), ""}, {http.MethodPut, "/api/v1/content-items/" + id.String() + "/draft", `{"expected_version":1,"content":"x"}`}} {
		w := contentRequest(h, x.m, x.p, x.b, nil)
		if w.Code == 404 || w.Code == 405 {
			t.Fatalf("%s %s=%d", x.m, x.p, w.Code)
		}
	}
}
func TestContentResponsesAreJSON(t *testing.T) {
	if _, err := json.Marshal(contentDetailResponse(contentHTTPDetail())); err != nil {
		t.Fatal(err)
	}
}

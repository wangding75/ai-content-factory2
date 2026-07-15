package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/contentitem"
)

// contentItemApplication is the complete, intentionally small boundary between
// the D1 transport and the content-item application.  Handlers never see a
// repository, generator, or fingerprint.
type contentItemApplication interface {
	CreateOrGet(context.Context, contentitem.CreateOrGetCommand) (contentitem.CreateResult, error)
	Get(context.Context, contentitem.GetCommand) (contentitem.Detail, error)
	SaveDraft(context.Context, contentitem.SaveDraftCommand) (contentitem.Detail, error)
	MockGenerate(context.Context, contentitem.MockGenerateCommand) (contentitem.MockGenerateResult, error)
	MockReview(context.Context, contentitem.MockReviewCommand) (contentitem.MockReviewResult, error)
	ListReviews(context.Context, contentitem.ListReviewsCommand) (contentitem.ReviewList, error)
	GetReview(context.Context, contentitem.GetReviewCommand) (contentitem.ReviewDetail, error)
}

func registerContentItemRoutes(mux *http.ServeMux, app contentItemApplication) {
	mux.HandleFunc("POST /api/v1/chapter-plans/{chapterPlanId}/content", createContentItemHandler(app))
	mux.HandleFunc("GET /api/v1/content-items/{contentItemId}", getContentItemHandler(app))
	mux.HandleFunc("PUT /api/v1/content-items/{contentItemId}/draft", saveContentDraftHandler(app))
	mux.HandleFunc("POST /api/v1/content-items/{contentItemId}/mock-generate", mockGenerateContentHandler(app))
	mux.HandleFunc("POST /api/v1/content-items/{contentItemId}/reviews/mock", mockReviewContentHandler(app))
	mux.HandleFunc("GET /api/v1/content-items/{contentItemId}/reviews", listReviewsHandler(app))
	mux.HandleFunc("GET /api/v1/reviews/{reviewId}", getReviewHandler(app))
}

type contentItemResponse struct {
	ID               uuid.UUID `json:"id"`
	ChapterPlanID    uuid.UUID `json:"chapter_plan_id"`
	Title            string    `json:"title"`
	Status           string    `json:"status"`
	CurrentVersionID uuid.UUID `json:"current_version_id"`
	ReviewedAt       *string   `json:"reviewed_at"`
	CreatedAt        string    `json:"created_at"`
	UpdatedAt        string    `json:"updated_at"`
}
type contentVersionResponse struct {
	ID            uuid.UUID `json:"id"`
	ContentItemID uuid.UUID `json:"content_item_id"`
	VersionNo     int       `json:"version_no"`
	Version       int       `json:"version"`
	Status        string    `json:"status"`
	Source        string    `json:"source"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	Summary       *string   `json:"summary"`
	WordCount     int       `json:"word_count"`
	FrozenAt      *string   `json:"frozen_at"`
	CreatedAt     string    `json:"created_at"`
	UpdatedAt     string    `json:"updated_at"`
}
type workflowRunResponse struct {
	ID          uuid.UUID `json:"id"`
	ProviderKey string    `json:"provider_key"`
	WorkflowKey string    `json:"workflow_key"`
	Status      string    `json:"status"`
	StartedAt   string    `json:"started_at"`
	FinishedAt  *string   `json:"finished_at"`
}
type contentItemDetailResponse struct {
	ContentItem    contentItemResponse    `json:"content_item"`
	CurrentVersion contentVersionResponse `json:"current_version"`
}

type reviewReportResponse struct {
	ID               uuid.UUID `json:"id"`
	ContentItemID    uuid.UUID `json:"content_item_id"`
	ContentVersionID uuid.UUID `json:"content_version_id"`
	ProviderKey      string    `json:"provider_key"`
	Status           string    `json:"status"`
	Conclusion       string    `json:"conclusion"`
	Score            int       `json:"score"`
	Summary          string    `json:"summary"`
	CreatedAt        string    `json:"created_at"`
}
type reviewFindingResponse struct {
	ID, ReviewID                           uuid.UUID `json:"-"`
	Category, Severity, Title, Description string    `json:"-"`
	Location                               any       `json:"-"`
}
type reviewRecommendationResponse struct {
	ID, ReviewID                 uuid.UUID `json:"-"`
	Priority, Title, Description string    `json:"-"`
	CreatedAt                    string    `json:"-"`
}
type contentVersionSummaryResponse struct {
	ID        uuid.UUID `json:"id"`
	VersionNo int       `json:"version_no"`
	Version   int       `json:"version"`
	Title     string    `json:"title"`
	WordCount int       `json:"word_count"`
	Source    string    `json:"source"`
	FrozenAt  string    `json:"frozen_at"`
}

func (v reviewFindingResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID          uuid.UUID `json:"id"`
		ReviewID    uuid.UUID `json:"review_id"`
		Category    string    `json:"category"`
		Severity    string    `json:"severity"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		Location    any       `json:"location"`
	}{v.ID, v.ReviewID, v.Category, v.Severity, v.Title, v.Description, v.Location})
}
func (v reviewRecommendationResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID          uuid.UUID `json:"id"`
		ReviewID    uuid.UUID `json:"review_id"`
		Priority    string    `json:"priority"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		CreatedAt   string    `json:"created_at"`
	}{v.ID, v.ReviewID, v.Priority, v.Title, v.Description, v.CreatedAt})
}

func reviewResponse(v contentitem.ReviewReport) reviewReportResponse {
	return reviewReportResponse{v.ID, v.ContentItemID, v.ContentVersionID, v.ProviderKey, v.Status, v.Conclusion, v.Score, v.Summary, v.CreatedAt.UTC().Format(time.RFC3339Nano)}
}
func findingsResponse(in []contentitem.ReviewFinding) []reviewFindingResponse {
	out := make([]reviewFindingResponse, len(in))
	for i, v := range in {
		var location any
		if len(v.LocationJSON) != 0 {
			_ = json.Unmarshal(v.LocationJSON, &location)
		}
		out[i] = reviewFindingResponse{v.ID, v.ReviewID, v.Category, v.Severity, v.Title, v.Description, location}
	}
	return out
}
func recommendationsResponse(in []contentitem.ReviewRecommendation) []reviewRecommendationResponse {
	out := make([]reviewRecommendationResponse, len(in))
	for i, v := range in {
		out[i] = reviewRecommendationResponse{v.ID, v.ReviewID, v.Priority, v.Title, v.Description, v.CreatedAt.UTC().Format(time.RFC3339Nano)}
	}
	return out
}
func workflowResponse(v contentitem.WorkflowRun) workflowRunResponse {
	var finished *string
	if v.FinishedAt != nil {
		x := v.FinishedAt.UTC().Format(time.RFC3339Nano)
		finished = &x
	}
	return workflowRunResponse{v.ID, v.ProviderKey, v.WorkflowKey, v.Status, v.StartedAt.UTC().Format(time.RFC3339Nano), finished}
}

func contentDetailResponse(d contentitem.Detail) contentItemDetailResponse {
	timePtr := func(v *time.Time) *string {
		if v == nil {
			return nil
		}
		s := v.UTC().Format(time.RFC3339Nano)
		return &s
	}
	timeString := func(v time.Time) string { return v.UTC().Format(time.RFC3339Nano) }
	return contentItemDetailResponse{
		ContentItem:    contentItemResponse{ID: d.Item.ID, ChapterPlanID: d.Item.ChapterPlanID, Title: d.Item.Title, Status: d.Item.Status, CurrentVersionID: d.Item.CurrentVersionID, ReviewedAt: timePtr(d.Item.ReviewedAt), CreatedAt: timeString(d.Item.CreatedAt), UpdatedAt: timeString(d.Item.UpdatedAt)},
		CurrentVersion: contentVersionResponse{ID: d.CurrentVersion.ID, ContentItemID: d.CurrentVersion.ContentItemID, VersionNo: d.CurrentVersion.VersionNo, Version: d.CurrentVersion.Version, Status: d.CurrentVersion.Status, Source: d.CurrentVersion.Source, Title: d.CurrentVersion.Title, Content: d.CurrentVersion.Content, Summary: d.CurrentVersion.Summary, WordCount: d.CurrentVersion.WordCount, FrozenAt: timePtr(d.CurrentVersion.FrozenAt), CreatedAt: timeString(d.CurrentVersion.CreatedAt), UpdatedAt: timeString(d.CurrentVersion.UpdatedAt)},
	}
}

func chapterPlanID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("chapterPlanId"))
	return id, err == nil
}
func contentItemID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("contentItemId"))
	return id, err == nil
}

func createContentItemHandler(app contentItemApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := chapterPlanID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "chapterPlanId must be a UUID", map[string]any{})
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || len(body) != 0 {
			writeError(w, r, 400, "validation_error", "request body is not allowed", map[string]any{})
			return
		}
		result, err := app.CreateOrGet(r.Context(), contentitem.CreateOrGetCommand{ChapterPlanID: id})
		if err != nil {
			contentItemServiceError(w, r, err)
			return
		}
		status := http.StatusOK
		if result.Created {
			status = http.StatusCreated
		}
		writeJSON(w, r, status, contentDetailResponse(result.Detail))
	}
}
func getContentItemHandler(app contentItemApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := contentItemID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "contentItemId must be a UUID", map[string]any{})
			return
		}
		detail, err := app.Get(r.Context(), contentitem.GetCommand{ContentItemID: id})
		if err != nil {
			contentItemServiceError(w, r, err)
			return
		}
		writeJSON(w, r, 200, contentDetailResponse(detail))
	}
}

type saveContentDraftRequest struct {
	ExpectedVersion json.RawMessage `json:"expected_version"`
	Title           json.RawMessage `json:"title"`
	Content         json.RawMessage `json:"content"`
	Summary         json.RawMessage `json:"summary"`
}

func optionalString(raw json.RawMessage, allowNull bool) (contentitem.OptionalString, error) {
	if raw == nil {
		return contentitem.OptionalString{}, nil
	}
	if string(raw) == "null" {
		if !allowNull {
			return contentitem.OptionalString{}, errors.New("null")
		}
		return contentitem.OptionalString{Set: true}, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return contentitem.OptionalString{}, err
	}
	return contentitem.OptionalString{Set: true, Value: &value}, nil
}
func decodeSaveContentDraft(r *http.Request) (contentitem.SaveDraftCommand, error) {
	var body saveContentDraftRequest
	if err := decodeBody(r, &body); err != nil || body.ExpectedVersion == nil {
		return contentitem.SaveDraftCommand{}, errors.New("invalid request")
	}
	var expected int
	if err := json.Unmarshal(body.ExpectedVersion, &expected); err != nil || expected < 1 {
		return contentitem.SaveDraftCommand{}, errors.New("invalid expected version")
	}
	title, e := optionalString(body.Title, false)
	if e != nil {
		return contentitem.SaveDraftCommand{}, e
	}
	content, e := optionalString(body.Content, false)
	if e != nil {
		return contentitem.SaveDraftCommand{}, e
	}
	summary, e := optionalString(body.Summary, true)
	if e != nil {
		return contentitem.SaveDraftCommand{}, e
	}
	if !title.Set && !content.Set && !summary.Set {
		return contentitem.SaveDraftCommand{}, errors.New("no draft fields")
	}
	return contentitem.SaveDraftCommand{ExpectedVersion: expected, Title: title, Content: content, Summary: summary}, nil
}
func saveContentDraftHandler(app contentItemApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := contentItemID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "contentItemId must be a UUID", map[string]any{})
			return
		}
		command, err := decodeSaveContentDraft(r)
		if err != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		command.ContentItemID = id
		detail, err := app.SaveDraft(r.Context(), command)
		if err != nil {
			contentItemServiceError(w, r, err)
			return
		}
		writeJSON(w, r, 200, contentDetailResponse(detail))
	}
}

type mockGenerateContentRequest struct {
	ExpectedVersion json.RawMessage `json:"expected_version"`
	Parameters      json.RawMessage `json:"parameters"`
}
type mockGenerationParametersRequest struct {
	ChapterGoal       json.RawMessage `json:"chapter_goal"`
	StorylineRefs     json.RawMessage `json:"storyline_refs_json"`
	MaterialRefs      json.RawMessage `json:"material_refs_json"`
	ForeshadowingRefs json.RawMessage `json:"foreshadowing_refs_json"`
	CreationNotes     json.RawMessage `json:"creation_notes"`
}

func optionalUUIDs(raw json.RawMessage) (contentitem.OptionalUUIDs, error) {
	if raw == nil || string(raw) == "null" {
		return contentitem.OptionalUUIDs{}, errors.New("required array")
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return contentitem.OptionalUUIDs{}, err
	}
	out := make([]uuid.UUID, len(values))
	for i, v := range values {
		id, e := uuid.Parse(v)
		if e != nil {
			return contentitem.OptionalUUIDs{}, e
		}
		out[i] = id
	}
	return contentitem.OptionalUUIDs{Set: true, Value: out}, nil
}

func decodeStrictRaw(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}
func decodeMockGenerateContent(r *http.Request) (contentitem.MockGenerateCommand, error) {
	var body mockGenerateContentRequest
	if err := decodeBody(r, &body); err != nil || body.ExpectedVersion == nil || body.Parameters == nil {
		return contentitem.MockGenerateCommand{}, errors.New("invalid request")
	}
	var expected int
	if err := json.Unmarshal(body.ExpectedVersion, &expected); err != nil || expected < 1 {
		return contentitem.MockGenerateCommand{}, errors.New("invalid expected")
	}
	var p mockGenerationParametersRequest
	if err := decodeStrictRaw(body.Parameters, &p); err != nil {
		return contentitem.MockGenerateCommand{}, err
	}
	goal, e := optionalString(p.ChapterGoal, true)
	if e != nil || !goal.Set {
		return contentitem.MockGenerateCommand{}, errors.New("invalid goal")
	}
	notes, e := optionalString(p.CreationNotes, true)
	if e != nil || !notes.Set {
		return contentitem.MockGenerateCommand{}, errors.New("invalid notes")
	}
	story, e := optionalUUIDs(p.StorylineRefs)
	if e != nil {
		return contentitem.MockGenerateCommand{}, e
	}
	material, e := optionalUUIDs(p.MaterialRefs)
	if e != nil {
		return contentitem.MockGenerateCommand{}, e
	}
	foreshadowing, e := optionalUUIDs(p.ForeshadowingRefs)
	if e != nil {
		return contentitem.MockGenerateCommand{}, e
	}
	return contentitem.MockGenerateCommand{ExpectedVersion: expected, Parameters: contentitem.MockGenerationParameters{ChapterGoal: goal, CreationNotes: notes, StorylineRefs: story, MaterialRefs: material, ForeshadowingRefs: foreshadowing}}, nil
}
func mockGenerateContentHandler(app contentItemApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := contentItemID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "contentItemId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeError(w, r, 400, "idempotency_key_required", "idempotency key required", map[string]any{})
			return
		}
		command, err := decodeMockGenerateContent(r)
		if err != nil {
			writeError(w, r, 400, "validation_error", "invalid request body", map[string]any{})
			return
		}
		command.ContentItemID = id
		command.IdempotencyKey = key
		result, err := app.MockGenerate(r.Context(), command)
		if err != nil {
			contentItemServiceError(w, r, err)
			return
		}
		run := result.WorkflowRun
		finished := (*string)(nil)
		if run.FinishedAt != nil {
			s := run.FinishedAt.UTC().Format(time.RFC3339Nano)
			finished = &s
		}
		writeJSON(w, r, 200, map[string]any{"content_item": contentDetailResponse(result.Detail).ContentItem, "current_version": contentDetailResponse(result.Detail).CurrentVersion, "workflow_run": workflowRunResponse{ID: run.ID, ProviderKey: run.ProviderKey, WorkflowKey: run.WorkflowKey, Status: run.Status, StartedAt: run.StartedAt.UTC().Format(time.RFC3339Nano), FinishedAt: finished}})
	}
}

type mockReviewRequest struct {
	ContentVersionID string `json:"content_version_id"`
	ExpectedVersion  int    `json:"expected_version"`
}

func decodeMockReview(r *http.Request) (contentitem.MockReviewCommand, error) {
	var body mockReviewRequest
	if err := decodeBody(r, &body); err != nil || body.ExpectedVersion < 1 {
		return contentitem.MockReviewCommand{}, errors.New("invalid request")
	}
	versionID, err := uuid.Parse(body.ContentVersionID)
	if err != nil {
		return contentitem.MockReviewCommand{}, err
	}
	return contentitem.MockReviewCommand{ContentVersionID: versionID, ExpectedVersion: body.ExpectedVersion}, nil
}

func mockReviewContentHandler(app contentItemApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemID, ok := contentItemID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "contentItemId must be a UUID", map[string]any{})
			return
		}
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" {
			writeError(w, r, 400, "idempotency_key_required", "idempotency key required", map[string]any{})
			return
		}
		command, err := decodeMockReview(r)
		if err != nil {
			writeError(w, r, 422, "invalid_review_parameters", "invalid review parameters", map[string]any{})
			return
		}
		command.ContentItemID, command.IdempotencyKey = itemID, key
		out, err := app.MockReview(r.Context(), command)
		if err != nil {
			contentItemServiceError(w, r, err)
			return
		}
		writeJSON(w, r, 200, map[string]any{"content_item": contentDetailResponse(out.Detail).ContentItem, "review": reviewResponse(out.Review), "findings": findingsResponse(out.Findings), "recommendations": recommendationsResponse(out.Recommendations), "workflow_run": workflowResponse(out.WorkflowRun)})
	}
}

func listReviewsHandler(app contentItemApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		itemID, ok := contentItemID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "contentItemId must be a UUID", map[string]any{})
			return
		}
		limit, offset := 20, 0
		var err error
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.Atoi(raw)
			if err != nil || limit < 1 || limit > 100 {
				writeError(w, r, 400, "invalid_pagination", "invalid pagination", map[string]any{})
				return
			}
		}
		if raw := r.URL.Query().Get("offset"); raw != "" {
			offset, err = strconv.Atoi(raw)
			if err != nil || offset < 0 {
				writeError(w, r, 400, "invalid_pagination", "invalid pagination", map[string]any{})
				return
			}
		}
		out, err := app.ListReviews(r.Context(), contentitem.ListReviewsCommand{ContentItemID: itemID, Limit: limit, Offset: offset})
		if err != nil {
			contentItemServiceError(w, r, err)
			return
		}
		items := make([]reviewReportResponse, len(out.Items))
		for i, v := range out.Items {
			items[i] = reviewResponse(v)
		}
		writeJSON(w, r, 200, map[string]any{"items": items, "total": out.Total, "limit": out.Limit, "offset": out.Offset})
	}
}

func reviewID(r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("reviewId"))
	return id, err == nil
}

func getReviewHandler(app contentItemApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := reviewID(r)
		if !ok {
			writeError(w, r, 400, "invalid_uuid", "reviewId must be a UUID", map[string]any{})
			return
		}
		out, err := app.GetReview(r.Context(), contentitem.GetReviewCommand{ReviewID: id})
		if err != nil {
			contentItemServiceError(w, r, err)
			return
		}
		if out.ContentVersion.FrozenAt == nil {
			writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
			return
		}
		version := contentVersionSummaryResponse{out.ContentVersion.ID, out.ContentVersion.VersionNo, out.ContentVersion.Version, out.ContentVersion.Title, out.ContentVersion.WordCount, out.ContentVersion.Source, out.ContentVersion.FrozenAt.UTC().Format(time.RFC3339Nano)}
		writeJSON(w, r, 200, map[string]any{"review": reviewResponse(out.Review), "content_version": version, "findings": findingsResponse(out.Findings), "recommendations": recommendationsResponse(out.Recommendations), "workflow_run": workflowResponse(out.WorkflowRun)})
	}
}

func contentItemServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, contentitem.ErrChapterPlanNotFound):
		writeError(w, r, 404, "chapter_plan_not_found", "chapter plan not found", map[string]any{})
	case errors.Is(err, contentitem.ErrChapterPlanNotConfirmed):
		writeError(w, r, 409, "chapter_plan_not_confirmed", "chapter plan not confirmed", map[string]any{})
	case errors.Is(err, contentitem.ErrContentItemNotFound):
		writeError(w, r, 404, "content_item_not_found", "content item not found", map[string]any{})
	case errors.Is(err, contentitem.ErrContentVersionNotFound):
		writeError(w, r, 404, "content_version_not_found", "content version not found", map[string]any{})
	case errors.Is(err, contentitem.ErrContentVersionLocked):
		writeError(w, r, 409, "content_version_locked", "content version locked", map[string]any{})
	case errors.Is(err, contentitem.ErrContentVersionReviewed):
		writeError(w, r, 409, "content_version_already_reviewed", "content version already reviewed", map[string]any{})
	case errors.Is(err, contentitem.ErrVersionConflict):
		writeError(w, r, 409, "version_conflict", "content version conflict", map[string]any{})
	case errors.Is(err, contentitem.ErrIdempotencyKeyRequired):
		writeError(w, r, 400, "idempotency_key_required", "idempotency key required", map[string]any{})
	case errors.Is(err, contentitem.ErrIdempotencyConflict):
		writeError(w, r, 409, "idempotency_key_reused_with_different_payload", "idempotency key reused with different payload", map[string]any{})
	case errors.Is(err, contentitem.ErrInvalidGenerationParameters):
		writeError(w, r, 422, "invalid_generation_parameters", "invalid generation parameters", map[string]any{})
	case errors.Is(err, contentitem.ErrMockGenerationFailed):
		writeError(w, r, 422, "mock_generation_failed", "mock generation failed", map[string]any{})
	case errors.Is(err, contentitem.ErrInvalidReviewParameters):
		writeError(w, r, 422, "invalid_review_parameters", "invalid review parameters", map[string]any{})
	case errors.Is(err, contentitem.ErrMockReviewFailed):
		writeError(w, r, 422, "mock_review_failed", "mock review failed", map[string]any{})
	case errors.Is(err, contentitem.ErrReviewNotFound):
		writeError(w, r, 404, "review_not_found", "review not found", map[string]any{})
	case errors.Is(err, contentitem.ErrInvalidPagination):
		writeError(w, r, 400, "invalid_pagination", "invalid pagination", map[string]any{})
	case errors.Is(err, contentitem.ErrValidation):
		writeError(w, r, 400, "validation_error", "invalid content request", map[string]any{})
	default:
		writeError(w, r, 500, "internal_error", "internal server error", map[string]any{})
	}
}

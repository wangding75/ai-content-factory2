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
	"github.com/local/ai-content-factory/apps/api/internal/chapterplan"
	"github.com/local/ai-content-factory/apps/api/internal/project"
)

type fakeChapterPlanApplication struct {
	items             []chapterplan.Plan
	plan              chapterplan.Plan
	generated         chapterplan.MockGenerateResult
	err               error
	listProjectID     uuid.UUID
	getID             uuid.UUID
	generateProjectID uuid.UUID
	command           chapterplan.MockGenerateCommand
	listCalls         int
	getCalls          int
	generateCalls     int
	updateID          uuid.UUID
	updateCommand     chapterplan.UpdateCommand
	updateCalls       int
	deleteID          uuid.UUID
	deleteVersion     int
	deleteCalls       int
	confirmProjectID  uuid.UUID
	selections        []chapterplan.Selection
	confirmCalls      int
}

func (f *fakeChapterPlanApplication) List(_ context.Context, id uuid.UUID) ([]chapterplan.Plan, error) {
	f.listCalls++
	f.listProjectID = id
	return f.items, f.err
}
func (f *fakeChapterPlanApplication) Get(_ context.Context, id uuid.UUID) (chapterplan.Plan, error) {
	f.getCalls++
	f.getID = id
	return f.plan, f.err
}
func (f *fakeChapterPlanApplication) GenerateMock(_ context.Context, id uuid.UUID, command chapterplan.MockGenerateCommand) (chapterplan.MockGenerateResult, error) {
	f.generateCalls++
	f.generateProjectID = id
	f.command = command
	return f.generated, f.err
}

func chapterPlanHTTPValue(projectID uuid.UUID) chapterplan.Plan {
	now := time.Date(2026, 7, 15, 1, 2, 3, 456000000, time.UTC)
	goal := "goal"
	return chapterplan.Plan{ID: uuid.New(), ProjectID: projectID, ChapterNo: 2, Title: "Title", Summary: "Summary", Status: "pending_confirmation", Source: "mock_generated", Goal: &goal, Version: 1, CreatedAt: now, UpdatedAt: now, Storylines: []chapterplan.StorylineRef{{ID: uuid.New(), Relation: "primary"}}, Materials: []uuid.UUID{}, Foreshadowings: []uuid.UUID{}}
}
func chapterPlanRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	parts := strings.Split(strings.Trim(strings.Split(path, "?")[0], "/"), "/")
	for i := range parts {
		if parts[i] == "projects" && i+1 < len(parts) {
			request.SetPathValue("projectId", parts[i+1])
		}
		if parts[i] == "chapter-plans" && i == 2 && i+1 < len(parts) {
			request.SetPathValue("chapterPlanId", parts[i+1])
		}
	}
	withRequestID(handler).ServeHTTP(response, request)
	return response
}
func mockGenerateBody(targetID uuid.UUID) string {
	return `{"target_storyline_id":"` + targetID.String() + `","start_chapter_no":1,"end_chapter_no":2,"chapter_count":2,"include_main_storyline":true,"include_child_storylines":false,"include_project_materials":true,"include_unpaid_foreshadowings":false,"include_prior_chapter_summaries":true,"summary_length":"medium","chapter_pace":"balanced","generation_notes":null}`
}

func TestChapterPlanListHandler(t *testing.T) {
	projectID := uuid.New()
	first, second := chapterPlanHTTPValue(projectID), chapterPlanHTTPValue(projectID)
	first.ChapterNo, second.ChapterNo = 2, 1
	fake := &fakeChapterPlanApplication{items: []chapterplan.Plan{first, second}}
	response := chapterPlanRequest(listChapterPlansHandler(fake), http.MethodGet, "/api/v1/projects/"+projectID.String()+"/chapter-plans?limit=1&offset=0", "")
	if response.Code != 200 || fake.listCalls != 1 || fake.listProjectID != projectID || !strings.Contains(response.Body.String(), `"chapter_no":1`) || !strings.Contains(response.Body.String(), `"total":2`) {
		t.Fatalf("list=%d body=%s fake=%#v", response.Code, response.Body.String(), fake)
	}
	if !strings.Contains(response.Body.String(), `"request_id":"req_`) {
		t.Fatalf("missing envelope/request ID: %s", response.Body.String())
	}
	fake.items = nil
	response = chapterPlanRequest(listChapterPlansHandler(fake), http.MethodGet, "/api/v1/projects/"+projectID.String()+"/chapter-plans", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"items":[]`) {
		t.Fatalf("empty=%d %s", response.Code, response.Body.String())
	}
	fake.err = chapterplan.ErrProjectNotFound
	response = chapterPlanRequest(listChapterPlansHandler(fake), http.MethodGet, "/api/v1/projects/"+projectID.String()+"/chapter-plans", "")
	requireErrorEnvelope(t, response, 404)
	response = chapterPlanRequest(listChapterPlansHandler(fake), http.MethodGet, "/api/v1/projects/bad/chapter-plans", "")
	requireErrorEnvelope(t, response, 400)
}

func TestChapterPlanGetHandler(t *testing.T) {
	projectID, id := uuid.New(), uuid.New()
	value := chapterPlanHTTPValue(projectID)
	value.ID = id
	confirmed := time.Date(2026, 7, 15, 4, 5, 6, 0, time.UTC)
	value.ConfirmedAt = &confirmed
	value.Notes = nil
	fake := &fakeChapterPlanApplication{plan: value}
	response := chapterPlanRequest(getChapterPlanHandler(fake), http.MethodGet, "/api/v1/chapter-plans/"+id.String(), "")
	if response.Code != 200 || fake.getCalls != 1 || fake.getID != id || !strings.Contains(response.Body.String(), `"confirmed_at":"2026-07-15T04:05:06Z"`) || !strings.Contains(response.Body.String(), `"creation_notes":null`) || !strings.Contains(response.Body.String(), `"storyline_refs_json"`) {
		t.Fatalf("get=%d body=%s fake=%#v", response.Code, response.Body.String(), fake)
	}
	fake.err = chapterplan.ErrChapterPlanNotFound
	response = chapterPlanRequest(getChapterPlanHandler(fake), http.MethodGet, "/api/v1/chapter-plans/"+id.String(), "")
	requireErrorEnvelope(t, response, 404)
	response = chapterPlanRequest(getChapterPlanHandler(fake), http.MethodGet, "/api/v1/chapter-plans/nope", "")
	requireErrorEnvelope(t, response, 400)
}

func TestChapterPlanMockGenerateHandler(t *testing.T) {
	projectID, targetID := uuid.New(), uuid.New()
	value := chapterPlanHTTPValue(projectID)
	run := chapterplan.Run{ID: uuid.New(), ProjectID: projectID, CreatedAt: time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 15, 1, 0, 1, 0, time.UTC)}
	fake := &fakeChapterPlanApplication{generated: chapterplan.MockGenerateResult{Run: run, Items: []chapterplan.Plan{value}}}
	body := mockGenerateBody(targetID)
	response := chapterPlanRequest(generateMockChapterPlansHandler(fake), http.MethodPost, "/api/v1/projects/"+projectID.String()+"/chapter-plans/mock-generate", body)
	if response.Code != 201 || fake.generateCalls != 1 || fake.generateProjectID != projectID || fake.command.TargetStorylineID != targetID || fake.command.StartChapterNo != 1 || fake.command.EndChapterNo != 2 || fake.command.ChapterCount != 2 || !fake.command.IncludeMainStoryline || fake.command.IncludeChildStorylines || !fake.command.IncludeProjectMaterials || fake.command.GenerationNotes != nil || !strings.Contains(response.Body.String(), `"provider_key":"mock"`) {
		t.Fatalf("generate=%d body=%s command=%#v", response.Code, response.Body.String(), fake.command)
	}
	for _, bad := range []string{"{", `{}`, strings.Replace(body, `"generation_notes":null`, `"generation_notes":null,"id":"server"`, 1)} {
		response = chapterPlanRequest(generateMockChapterPlansHandler(fake), http.MethodPost, "/api/v1/projects/"+projectID.String()+"/chapter-plans/mock-generate", bad)
		requireErrorEnvelope(t, response, 400)
	}
	response = chapterPlanRequest(generateMockChapterPlansHandler(fake), http.MethodPost, "/api/v1/projects/bad/chapter-plans/mock-generate", body)
	requireErrorEnvelope(t, response, 400)
	for _, pair := range []struct {
		err    error
		status int
		code   string
	}{{chapterplan.ErrValidation, 400, "validation_error"}, {chapterplan.ErrStorylineReferenceInvalid, 404, "storyline_not_found"}, {chapterplan.ErrMaterialReferenceInvalid, 404, "material_not_found"}, {chapterplan.ErrForeshadowingReferenceInvalid, 404, "foreshadowing_not_found"}, {chapterplan.ErrChapterNoConflict, 409, "chapter_no_conflict"}, {chapterplan.ErrVersionConflict, 409, "version_conflict"}, {errors.New("driver detail"), 500, "internal_error"}} {
		fake.err = pair.err
		response = chapterPlanRequest(generateMockChapterPlansHandler(fake), http.MethodPost, "/api/v1/projects/"+projectID.String()+"/chapter-plans/mock-generate", body)
		requireErrorEnvelope(t, response, pair.status)
		if !strings.Contains(response.Body.String(), pair.code) || strings.Contains(response.Body.String(), "driver detail") {
			t.Fatalf("error=%s", response.Body.String())
		}
	}
}

func TestChapterPlanRoutesRegisteredOnlyForThisIteration(t *testing.T) {
	projectID, planID, targetID := uuid.New(), uuid.New(), uuid.New()
	value := chapterPlanHTTPValue(projectID)
	value.ID = planID
	fake := &fakeChapterPlanApplication{items: []chapterplan.Plan{}, plan: value, generated: chapterplan.MockGenerateResult{Run: chapterplan.Run{ID: uuid.New(), ProjectID: projectID, CreatedAt: time.Now(), UpdatedAt: time.Now()}, Items: []chapterplan.Plan{value}}}
	handler := New(":0", project.NewService(newMemoryRepository()), fake).httpServer.Handler
	for _, item := range []struct{ method, path, body string }{{http.MethodGet, "/api/v1/projects/" + projectID.String() + "/chapter-plans", ""}, {http.MethodPost, "/api/v1/projects/" + projectID.String() + "/chapter-plans/mock-generate", mockGenerateBody(targetID)}, {http.MethodGet, "/api/v1/chapter-plans/" + planID.String(), ""}} {
		response := doRequest(handler, item.method, item.path, item.body)
		if response.Code == 404 || response.Code == 405 {
			t.Fatalf("route %s %s=%d", item.method, item.path, response.Code)
		}
	}
	for _, item := range []struct{ method, path, body string }{{http.MethodPatch, "/api/v1/chapter-plans/" + planID.String(), `{"expected_version":1,"title":"x"}`}, {http.MethodDelete, "/api/v1/chapter-plans/" + planID.String() + "?expected_version=1", ""}, {http.MethodPost, "/api/v1/projects/" + projectID.String() + "/chapter-plans/confirm", `{"selections":[{"chapter_plan_id":"` + planID.String() + `","expected_version":1}]}`}} {
		response := doRequest(handler, item.method, item.path, item.body)
		if response.Code == http.StatusMethodNotAllowed || response.Code == http.StatusNotFound {
			t.Fatalf("route %s %s=%d", item.method, item.path, response.Code)
		}
	}
	for _, path := range []string{"/api/v1/projects/" + projectID.String() + "/content-items", "/api/v1/chapter-plans/" + planID.String() + "/content"} {
		if response := doRequest(handler, http.MethodPost, path, "{}"); response.Code != http.StatusMethodNotAllowed && response.Code != http.StatusNotFound {
			t.Fatalf("unexpected extra route %s=%d", path, response.Code)
		}
	}
}

func TestChapterPlanResponseShapeIsJSON(t *testing.T) { // protects the envelope's UUID/time fields from accidental non-JSON values.
	value := chapterPlanHTTPValue(uuid.New())
	raw, err := json.Marshal(chapterPlanResponseFrom(value))
	if err != nil || !strings.Contains(string(raw), `"material_refs_json":[]`) {
		t.Fatalf("marshal=%v %s", err, raw)
	}
}

func (f *fakeChapterPlanApplication) Update(_ context.Context, id uuid.UUID, command chapterplan.UpdateCommand) (chapterplan.Plan, error) {
	f.updateCalls++
	f.updateID = id
	f.updateCommand = command
	return f.plan, f.err
}
func (f *fakeChapterPlanApplication) Delete(_ context.Context, id uuid.UUID, version int) error {
	f.deleteCalls++
	f.deleteID = id
	f.deleteVersion = version
	return f.err
}
func (f *fakeChapterPlanApplication) Confirm(_ context.Context, projectID uuid.UUID, selections []chapterplan.Selection) ([]chapterplan.Plan, error) {
	f.confirmCalls++
	f.confirmProjectID = projectID
	f.selections = selections
	return f.items, f.err
}

func TestChapterPlanUpdateDeleteConfirmHandlers(t *testing.T) {
	projectID, planID, storylineID, materialID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	value := chapterPlanHTTPValue(projectID)
	value.ID = planID
	fake := &fakeChapterPlanApplication{plan: value, items: []chapterplan.Plan{value}}
	body := `{"expected_version":2,"title":"updated","chapter_goal":null,"creation_notes":"","storyline_refs_json":[{"storyline_id":"` + storylineID.String() + `","relation":"primary"}],"material_refs_json":["` + materialID.String() + `"],"foreshadowing_refs_json":[]}`
	response := chapterPlanRequest(updateChapterPlanHandler(fake), http.MethodPatch, "/api/v1/chapter-plans/"+planID.String(), body)
	if response.Code != 200 || fake.updateCalls != 1 || fake.updateID != planID || fake.updateCommand.ExpectedVersion != 2 || fake.updateCommand.Title == nil || *fake.updateCommand.Title != "updated" || !fake.updateCommand.Goal.Set || fake.updateCommand.Goal.Value != nil || !fake.updateCommand.Notes.Set || fake.updateCommand.Notes.Value == nil || *fake.updateCommand.Notes.Value != "" || !fake.updateCommand.Storylines.Set || !fake.updateCommand.Materials.Set || !fake.updateCommand.Foreshadowings.Set || len(fake.updateCommand.Foreshadowings.Value) != 0 {
		t.Fatalf("update=%d fake=%#v", response.Code, fake)
	}
	for _, bad := range []string{`{`, `{"expected_version":2}`, `{"expected_version":0,"title":"x"}`, `{"expected_version":2,"id":"bad","title":"x"}`} {
		response = chapterPlanRequest(updateChapterPlanHandler(fake), http.MethodPatch, "/api/v1/chapter-plans/"+planID.String(), bad)
		requireErrorEnvelope(t, response, 400)
	}
	response = chapterPlanRequest(updateChapterPlanHandler(fake), http.MethodPatch, "/api/v1/chapter-plans/bad", body)
	requireErrorEnvelope(t, response, 400)
	for _, pair := range []struct {
		err    error
		status int
	}{{chapterplan.ErrChapterPlanNotFound, 404}, {chapterplan.ErrStorylineReferenceInvalid, 404}, {chapterplan.ErrChapterNoConflict, 409}, {chapterplan.ErrVersionConflict, 409}, {chapterplan.ErrInvalidState, 409}, {errors.New("private"), 500}} {
		fake.err = pair.err
		response = chapterPlanRequest(updateChapterPlanHandler(fake), http.MethodPatch, "/api/v1/chapter-plans/"+planID.String(), body)
		requireErrorEnvelope(t, response, pair.status)
		if strings.Contains(response.Body.String(), "private") {
			t.Fatal("internal error leaked")
		}
	}
	fake.err = nil
	response = chapterPlanRequest(deleteChapterPlanHandler(fake), http.MethodDelete, "/api/v1/chapter-plans/"+planID.String()+"?expected_version=2", "")
	if response.Code != 204 || response.Body.Len() != 0 || fake.deleteCalls != 1 || fake.deleteID != planID || fake.deleteVersion != 2 {
		t.Fatalf("delete=%d body=%q fake=%#v", response.Code, response.Body.String(), fake)
	}
	for _, path := range []string{"/api/v1/chapter-plans/bad?expected_version=2", "/api/v1/chapter-plans/" + planID.String(), "/api/v1/chapter-plans/" + planID.String() + "?expected_version=0"} {
		response = chapterPlanRequest(deleteChapterPlanHandler(fake), http.MethodDelete, path, "")
		requireErrorEnvelope(t, response, 400)
	}
	for _, pair := range []struct {
		err    error
		status int
	}{{chapterplan.ErrChapterPlanNotFound, 404}, {chapterplan.ErrVersionConflict, 409}, {chapterplan.ErrInvalidState, 409}, {errors.New("private"), 500}} {
		fake.err = pair.err
		response = chapterPlanRequest(deleteChapterPlanHandler(fake), http.MethodDelete, "/api/v1/chapter-plans/"+planID.String()+"?expected_version=2", "")
		requireErrorEnvelope(t, response, pair.status)
	}
	fake.err = nil
	confirm := `{"selections":[{"chapter_plan_id":"` + planID.String() + `","expected_version":2}]}`
	response = chapterPlanRequest(confirmChapterPlansHandler(fake), http.MethodPost, "/api/v1/projects/"+projectID.String()+"/chapter-plans/confirm", confirm)
	if response.Code != 200 || fake.confirmCalls != 1 || fake.confirmProjectID != projectID || len(fake.selections) != 1 || fake.selections[0].ID != planID || fake.selections[0].ExpectedVersion != 2 {
		t.Fatalf("confirm=%d fake=%#v", response.Code, fake)
	}
	for _, bad := range []string{`{`, `{"selections":[]}`, `{"selections":[{"chapter_plan_id":"bad","expected_version":2}]}`, `{"selections":[{"chapter_plan_id":"` + planID.String() + `","expected_version":2},{"chapter_plan_id":"` + planID.String() + `","expected_version":2}]}`} {
		response = chapterPlanRequest(confirmChapterPlansHandler(fake), http.MethodPost, "/api/v1/projects/"+projectID.String()+"/chapter-plans/confirm", bad)
		requireErrorEnvelope(t, response, 400)
	}
	response = chapterPlanRequest(confirmChapterPlansHandler(fake), http.MethodPost, "/api/v1/projects/bad/chapter-plans/confirm", confirm)
	requireErrorEnvelope(t, response, 400)
	for _, pair := range []struct {
		err    error
		status int
	}{{chapterplan.ErrProjectNotFound, 404}, {chapterplan.ErrChapterPlanNotFound, 404}, {chapterplan.ErrProjectMismatch, 400}, {chapterplan.ErrVersionConflict, 409}, {chapterplan.ErrInvalidState, 409}, {chapterplan.ErrMaterialReferenceInvalid, 404}, {errors.New("private"), 500}} {
		fake.err = pair.err
		response = chapterPlanRequest(confirmChapterPlansHandler(fake), http.MethodPost, "/api/v1/projects/"+projectID.String()+"/chapter-plans/confirm", confirm)
		requireErrorEnvelope(t, response, pair.status)
	}
}

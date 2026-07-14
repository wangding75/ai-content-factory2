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
	"github.com/local/ai-content-factory/apps/api/internal/foreshadowing"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
)

type fakeForeshadowingApplication struct {
	list             foreshadowing.ListResult
	created, updated foreshadowing.Foreshadowing
	err              error
	create           foreshadowing.CreateCommand
	update           foreshadowing.UpdateCommand
}

func (f *fakeForeshadowingApplication) List(context.Context, uuid.UUID) (foreshadowing.ListResult, error) {
	return f.list, f.err
}
func (f *fakeForeshadowingApplication) Create(_ context.Context, _ uuid.UUID, c foreshadowing.CreateCommand) (foreshadowing.Foreshadowing, error) {
	f.create = c
	return f.created, f.err
}
func (f *fakeForeshadowingApplication) Update(_ context.Context, _ uuid.UUID, c foreshadowing.UpdateCommand) (foreshadowing.Foreshadowing, error) {
	f.update = c
	return f.updated, f.err
}

func foreshadowingHTTPValue(projectID uuid.UUID) foreshadowing.Foreshadowing {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return foreshadowing.Foreshadowing{ID: uuid.New(), ProjectID: projectID, Title: "setup", Description: "description", Priority: "high", Status: "planned", Version: 1, CreatedAt: now, UpdatedAt: now}
}
func foreshadowingHTTPRequest(handler http.Handler, method, path, body, projectID, id string) *httptest.ResponseRecorder {
	if path == "" {
		path = "/"
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if projectID != "" {
		req.SetPathValue("projectId", projectID)
	}
	if id != "" {
		req.SetPathValue("foreshadowingId", id)
	}
	res := httptest.NewRecorder()
	withRequestID(handler).ServeHTTP(res, req)
	return res
}
func foreshadowingBody(planted, payoff, plant, pay string) string {
	return `{"title":"setup","description":"description","priority":"high","planted_plot_line_id":` + planted + `,"payoff_plot_line_id":` + payoff + `,"planned_plant_chapter":` + plant + `,"planned_payoff_chapter":` + pay + `,"status":"planned"}`
}

func TestForeshadowingListHandler(t *testing.T) {
	projectID := uuid.New()
	value := foreshadowingHTTPValue(projectID)
	fake := &fakeForeshadowingApplication{list: foreshadowing.ListResult{Items: []foreshadowing.Foreshadowing{value}}}
	res := foreshadowingHTTPRequest(listForeshadowingsHandler(fake), http.MethodGet, "/?limit=1&offset=0", "", projectID.String(), "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"planted_plot_line_id":null`) || !strings.Contains(res.Body.String(), `"request_id"`) {
		t.Fatalf("list=%d %s", res.Code, res.Body.String())
	}
	var body struct {
		Data struct {
			Items                []json.RawMessage `json:"items"`
			Total, Limit, Offset int
		}
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || len(body.Data.Items) != 1 || body.Data.Total != 1 || body.Data.Limit != 1 || body.RequestID == "" {
		t.Fatalf("body=%s err=%v", res.Body.String(), err)
	}
	fake.list = foreshadowing.ListResult{Items: []foreshadowing.Foreshadowing{}}
	res = foreshadowingHTTPRequest(listForeshadowingsHandler(fake), http.MethodGet, "", "", projectID.String(), "")
	if res.Code != 200 || !strings.Contains(res.Body.String(), `"items":[]`) {
		t.Fatal(res.Body.String())
	}
	res = foreshadowingHTTPRequest(listForeshadowingsHandler(fake), http.MethodGet, "/?limit=0", "", projectID.String(), "")
	requireErrorEnvelope(t, res, 400)
	res = foreshadowingHTTPRequest(listForeshadowingsHandler(fake), http.MethodGet, "", "", "bad", "")
	requireErrorEnvelope(t, res, 400)
	fake.err = project.ErrNotFound
	res = foreshadowingHTTPRequest(listForeshadowingsHandler(fake), http.MethodGet, "", "", projectID.String(), "")
	requireErrorEnvelope(t, res, 404)
	fake.err = errors.New("storage failure")
	res = foreshadowingHTTPRequest(listForeshadowingsHandler(fake), http.MethodGet, "", "", projectID.String(), "")
	requireErrorEnvelope(t, res, 500)
}

func TestForeshadowingCreateHandlerMappingAndErrors(t *testing.T) {
	projectID, planted, payoff := uuid.New(), uuid.New(), uuid.New()
	value := foreshadowingHTTPValue(projectID)
	fake := &fakeForeshadowingApplication{created: value}
	res := foreshadowingHTTPRequest(createForeshadowingHandler(fake), http.MethodPost, "", foreshadowingBody("null", "null", "null", "null"), projectID.String(), "")
	if res.Code != 201 || fake.create.PlantedPlotLineID != nil || fake.create.PlannedPlantChapter != nil || !strings.Contains(res.Body.String(), `"data"`) {
		t.Fatalf("create=%d command=%#v %s", res.Code, fake.create, res.Body.String())
	}
	res = foreshadowingHTTPRequest(createForeshadowingHandler(fake), http.MethodPost, "", foreshadowingBody(`"`+planted.String()+`"`, `"`+payoff.String()+`"`, "2", "3"), projectID.String(), "")
	if res.Code != 201 || fake.create.PlantedPlotLineID == nil || *fake.create.PlantedPlotLineID != planted || fake.create.PayoffPlotLineID == nil || *fake.create.PayoffPlotLineID != payoff {
		t.Fatalf("command=%#v", fake.create)
	}
	for _, body := range []string{"{", `{}`, foreshadowingBody(`"bad"`, "null", "null", "null")} {
		res = foreshadowingHTTPRequest(createForeshadowingHandler(fake), http.MethodPost, "", body, projectID.String(), "")
		requireErrorEnvelope(t, res, 400)
	}
	for _, pair := range []struct {
		err    error
		status int
	}{{project.ErrNotFound, 404}, {foreshadowing.ErrStorylineNotFound, 404}, {foreshadowing.ErrProjectMismatch, 400}, {foreshadowing.ErrInvalidPriority, 400}, {foreshadowing.ErrChapterRange, 400}, {errors.New("failure"), 500}} {
		fake.err = pair.err
		res = foreshadowingHTTPRequest(createForeshadowingHandler(fake), http.MethodPost, "", foreshadowingBody("null", "null", "null", "null"), projectID.String(), "")
		requireErrorEnvelope(t, res, pair.status)
	}
}

func TestForeshadowingUpdateHandlerTriStateAndErrors(t *testing.T) {
	projectID, id, ref := uuid.New(), uuid.New(), uuid.New()
	value := foreshadowingHTTPValue(projectID)
	value.ID = id
	fake := &fakeForeshadowingApplication{updated: value}
	res := foreshadowingHTTPRequest(updateForeshadowingHandler(fake), http.MethodPatch, "", `{"expected_version":1,"title":"changed"}`, "", id.String())
	if res.Code != 200 || fake.update.PlantedPlotLineID.Set || fake.update.PlannedPlantChapter.Set {
		t.Fatalf("update=%d %#v", res.Code, fake.update)
	}
	res = foreshadowingHTTPRequest(updateForeshadowingHandler(fake), http.MethodPatch, "", `{"expected_version":1,"planted_plot_line_id":null,"planned_plant_chapter":null}`, "", id.String())
	if res.Code != 200 || !fake.update.PlantedPlotLineID.Set || fake.update.PlantedPlotLineID.Value != nil || !fake.update.PlannedPlantChapter.Set || fake.update.PlannedPlantChapter.Value != nil {
		t.Fatalf("clear=%#v", fake.update)
	}
	res = foreshadowingHTTPRequest(updateForeshadowingHandler(fake), http.MethodPatch, "", `{"expected_version":1,"payoff_plot_line_id":"`+ref.String()+`","planned_payoff_chapter":9}`, "", id.String())
	if res.Code != 200 || !fake.update.PayoffPlotLineID.Set || fake.update.PayoffPlotLineID.Value == nil || *fake.update.PayoffPlotLineID.Value != ref || fake.update.PlannedPayoffChapter.Value == nil || *fake.update.PlannedPayoffChapter.Value != 9 {
		t.Fatalf("set=%#v", fake.update)
	}
	for _, body := range []string{"{", `{}`, `{"expected_version":"x","title":"x"}`, `{"expected_version":0,"title":"x"}`, `{"expected_version":1,"planted_plot_line_id":"bad"}`} {
		res = foreshadowingHTTPRequest(updateForeshadowingHandler(fake), http.MethodPatch, "", body, "", id.String())
		requireErrorEnvelope(t, res, 400)
	}
	res = foreshadowingHTTPRequest(updateForeshadowingHandler(fake), http.MethodPatch, "", `{"expected_version":1,"title":"x"}`, "", "bad")
	requireErrorEnvelope(t, res, 400)
	for _, pair := range []struct {
		err    error
		status int
	}{{foreshadowing.ErrNotFound, 404}, {foreshadowing.ErrStorylineNotFound, 404}, {foreshadowing.ErrProjectMismatch, 400}, {foreshadowing.ErrInvalidTransition, 400}, {foreshadowing.ErrChapterRange, 400}, {foreshadowing.ErrVersionConflict, 409}, {errors.New("failure"), 500}} {
		fake.err = pair.err
		res = foreshadowingHTTPRequest(updateForeshadowingHandler(fake), http.MethodPatch, "", `{"expected_version":1,"title":"x"}`, "", id.String())
		requireErrorEnvelope(t, res, pair.status)
	}
}

func TestForeshadowingRoutesRegistered(t *testing.T) {
	repo := newMemoryRepository()
	projects := project.NewService(repo)
	projectID, id := uuid.New(), uuid.New()
	fake := &fakeForeshadowingApplication{created: foreshadowingHTTPValue(projectID), updated: foreshadowingHTTPValue(projectID), list: foreshadowing.ListResult{Items: []foreshadowing.Foreshadowing{}}}
	story := &fakeStorylineApplication{tree: storyline.GetTreeResult{Items: []*storyline.StorylineTreeNode{}}}
	handler := New(":0", projects, story, fake).httpServer.Handler
	for _, item := range []struct{ method, path, body string }{{http.MethodGet, "/api/v1/projects/" + projectID.String() + "/foreshadowings", ""}, {http.MethodPost, "/api/v1/projects/" + projectID.String() + "/foreshadowings", foreshadowingBody("null", "null", "null", "null")}, {http.MethodPatch, "/api/v1/foreshadowings/" + id.String(), `{"expected_version":1,"title":"x"}`}, {http.MethodGet, "/api/v1/projects/" + projectID.String() + "/storylines", ""}} {
		res := doRequest(handler, item.method, item.path, item.body)
		if res.Code == 404 || res.Code == 405 {
			t.Fatalf("route %s %s=%d", item.method, item.path, res.Code)
		}
	}
	if res := doRequest(handler, http.MethodDelete, "/api/v1/foreshadowings/"+id.String(), ""); res.Code != 405 {
		t.Fatalf("wrong method=%d", res.Code)
	}
	if res := doRequest(handler, http.MethodGet, "/api/v1/content-items", ""); res.Code != 404 {
		t.Fatalf("content route=%d", res.Code)
	}
}

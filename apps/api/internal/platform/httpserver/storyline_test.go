package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/local/ai-content-factory/apps/api/internal/project"
	"github.com/local/ai-content-factory/apps/api/internal/storyline"
)

type fakeStorylineApplication struct {
	tree         storyline.GetTreeResult
	root         storyline.PlotLine
	child        storyline.PlotLine
	updated      storyline.PlotLine
	err          error
	rootCommand  storyline.CreateRootCommand
	childID      uuid.UUID
	childCommand storyline.CreateChildCommand
	updateID     uuid.UUID
	update       storyline.UpdateCommand
}

func (f *fakeStorylineApplication) GetTree(context.Context, uuid.UUID) (storyline.GetTreeResult, error) {
	return f.tree, f.err
}
func (f *fakeStorylineApplication) CreateRoot(_ context.Context, _ uuid.UUID, command storyline.CreateRootCommand) (storyline.PlotLine, error) {
	f.rootCommand = command
	return f.root, f.err
}
func (f *fakeStorylineApplication) CreateChildForParent(_ context.Context, id uuid.UUID, command storyline.CreateChildCommand) (storyline.PlotLine, error) {
	f.childID = id
	f.childCommand = command
	return f.child, f.err
}
func (f *fakeStorylineApplication) Update(_ context.Context, id uuid.UUID, command storyline.UpdateCommand) (storyline.PlotLine, error) {
	f.updateID = id
	f.update = command
	return f.updated, f.err
}

func storylineHTTPValue(projectID uuid.UUID) storyline.PlotLine {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return storyline.PlotLine{ID: uuid.New(), ProjectID: projectID, Type: "main", Relation: "root", Name: "line", Summary: "summary", Status: "active", SortOrder: 1, Version: 1, CreatedAt: now, UpdatedAt: now}
}
func storylineHTTPRequest(handler http.Handler, method, path, body, projectID, storylineID string) *httptest.ResponseRecorder {
	if path == "" {
		path = "/"
	}
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if projectID != "" {
		request.SetPathValue("projectId", projectID)
	}
	if storylineID != "" {
		request.SetPathValue("storylineId", storylineID)
	}
	response := httptest.NewRecorder()
	withRequestID(handler).ServeHTTP(response, request)
	return response
}

func TestStorylineTreeHandlerResponses(t *testing.T) {
	projectID := uuid.New()
	root := storylineHTTPValue(projectID)
	child := storylineHTTPValue(projectID)
	child.ID = uuid.New()
	child.Type, child.Relation, child.ParentID = "child", "child", &root.ID
	fake := &fakeStorylineApplication{tree: storyline.GetTreeResult{Items: []*storyline.StorylineTreeNode{{PlotLine: root, Children: []*storyline.StorylineTreeNode{{PlotLine: child, Children: []*storyline.StorylineTreeNode{}}}}}}}
	response := storylineHTTPRequest(getStorylineTreeHandler(fake), http.MethodGet, "/api/v1/projects/"+projectID.String()+"/storylines", "", projectID.String(), "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"project_id"`) || !strings.Contains(response.Body.String(), `"children"`) || !strings.Contains(response.Body.String(), `"request_id"`) {
		t.Fatalf("tree=%d %s", response.Code, response.Body.String())
	}
	fake.tree = storyline.GetTreeResult{Items: []*storyline.StorylineTreeNode{}}
	response = storylineHTTPRequest(getStorylineTreeHandler(fake), http.MethodGet, "", "", projectID.String(), "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"items":[]`) {
		t.Fatal(response.Body.String())
	}
	response = storylineHTTPRequest(getStorylineTreeHandler(fake), http.MethodGet, "", "", "bad", "")
	requireErrorEnvelope(t, response, 400)
	fake.err = project.ErrNotFound
	response = storylineHTTPRequest(getStorylineTreeHandler(fake), http.MethodGet, "", "", projectID.String(), "")
	requireErrorEnvelope(t, response, 404)
	if !strings.Contains(response.Body.String(), "project_not_found") {
		t.Fatal(response.Body.String())
	}
	fake.err = errors.New("database failed")
	response = storylineHTTPRequest(getStorylineTreeHandler(fake), http.MethodGet, "", "", projectID.String(), "")
	requireErrorEnvelope(t, response, 500)
}

func TestStorylineCreateRootHandler(t *testing.T) {
	projectID := uuid.New()
	value := storylineHTTPValue(projectID)
	fake := &fakeStorylineApplication{root: value}
	body := `{"name":"root","summary":"s","start_chapter":null,"end_chapter":10,"status":"active","sort_order":0}`
	response := storylineHTTPRequest(createRootStorylineHandler(fake), http.MethodPost, "", body, projectID.String(), "")
	if response.Code != 201 || fake.rootCommand.Name != "root" || fake.rootCommand.StartChapter != nil || !strings.Contains(response.Body.String(), `"data"`) {
		t.Fatalf("root=%d %s command=%#v", response.Code, response.Body.String(), fake.rootCommand)
	}
	for _, bad := range []string{"{", "{}", `{"name":"x","summary":"s","start_chapter":null,"end_chapter":1,"status":"active","sort_order":0,"extra":true}`} {
		response = storylineHTTPRequest(createRootStorylineHandler(fake), http.MethodPost, "", bad, projectID.String(), "")
		requireErrorEnvelope(t, response, 400)
	}
	response = storylineHTTPRequest(createRootStorylineHandler(fake), http.MethodPost, "", body, "bad", "")
	requireErrorEnvelope(t, response, 400)
	fake.err = project.ErrNotFound
	response = storylineHTTPRequest(createRootStorylineHandler(fake), http.MethodPost, "", body, projectID.String(), "")
	requireErrorEnvelope(t, response, 404)
	fake.err = storyline.ErrValidation
	response = storylineHTTPRequest(createRootStorylineHandler(fake), http.MethodPost, "", body, projectID.String(), "")
	requireErrorEnvelope(t, response, 400)
	fake.err = errors.New("failure")
	response = storylineHTTPRequest(createRootStorylineHandler(fake), http.MethodPost, "", body, projectID.String(), "")
	requireErrorEnvelope(t, response, 500)
}

func TestStorylineCreateChildHandler(t *testing.T) {
	projectID, parentID := uuid.New(), uuid.New()
	value := storylineHTTPValue(projectID)
	value.ParentID = &parentID
	value.Type, value.Relation = "child", "child"
	fake := &fakeStorylineApplication{child: value}
	body := `{"name":"child","summary":"s","start_chapter":2,"end_chapter":3,"status":"active","sort_order":0}`
	response := storylineHTTPRequest(createChildStorylineHandler(fake), http.MethodPost, "", body, "", parentID.String())
	if response.Code != 201 || fake.childID != parentID || fake.childCommand.Name != "child" || !strings.Contains(response.Body.String(), `"parent_id"`) {
		t.Fatalf("child=%d %s command=%#v", response.Code, response.Body.String(), fake.childCommand)
	}
	response = storylineHTTPRequest(createChildStorylineHandler(fake), http.MethodPost, "", body, "", "bad")
	requireErrorEnvelope(t, response, 400)
	fake.err = storyline.ErrParentNotFound
	response = storylineHTTPRequest(createChildStorylineHandler(fake), http.MethodPost, "", body, "", parentID.String())
	requireErrorEnvelope(t, response, 404)
	fake.err = storyline.ErrProjectMismatch
	response = storylineHTTPRequest(createChildStorylineHandler(fake), http.MethodPost, "", body, "", parentID.String())
	requireErrorEnvelope(t, response, 400)
	fake.err = storyline.ErrChildOutOfRange
	response = storylineHTTPRequest(createChildStorylineHandler(fake), http.MethodPost, "", body, "", parentID.String())
	requireErrorEnvelope(t, response, 400)
	response = storylineHTTPRequest(createChildStorylineHandler(fake), http.MethodPost, "", "{}", "", parentID.String())
	requireErrorEnvelope(t, response, 400)
}

func TestStorylineUpdateHandler(t *testing.T) {
	projectID, id := uuid.New(), uuid.New()
	value := storylineHTTPValue(projectID)
	value.ID = id
	fake := &fakeStorylineApplication{updated: value}
	body := `{"expected_version":1,"name":"updated","start_chapter":null,"end_chapter":3}`
	response := storylineHTTPRequest(updateStorylineHandler(fake), http.MethodPatch, "", body, "", id.String())
	if response.Code != 200 || fake.updateID != id || !fake.update.StartChapter.Set || fake.update.StartChapter.Value != nil || !strings.Contains(response.Body.String(), `"version"`) {
		t.Fatalf("update=%d %s command=%#v", response.Code, response.Body.String(), fake.update)
	}
	for _, bad := range []string{"{}", `{"expected_version":1}`, `{"expected_version":"one","name":"x"}`} {
		response = storylineHTTPRequest(updateStorylineHandler(fake), http.MethodPatch, "", bad, "", id.String())
		requireErrorEnvelope(t, response, 400)
	}
	response = storylineHTTPRequest(updateStorylineHandler(fake), http.MethodPatch, "", body, "", "bad")
	requireErrorEnvelope(t, response, 400)
	for _, pair := range []struct {
		err    error
		status int
	}{{storyline.ErrNotFound, 404}, {storyline.ErrVersionConflict, 409}, {storyline.ErrChapterRange, 400}, {storyline.ErrDescendantOutOfRange, 400}, {errors.New("failure"), 500}} {
		fake.err = pair.err
		response = storylineHTTPRequest(updateStorylineHandler(fake), http.MethodPatch, "", body, "", id.String())
		requireErrorEnvelope(t, response, pair.status)
	}
}

func TestStorylineRoutesRegistered(t *testing.T) {
	repo := newMemoryRepository()
	projects := project.NewService(repo)
	projectID, parentID := uuid.New(), uuid.New()
	fake := &fakeStorylineApplication{root: storylineHTTPValue(projectID), child: storylineHTTPValue(projectID), updated: storylineHTTPValue(projectID), tree: storyline.GetTreeResult{Items: []*storyline.StorylineTreeNode{}}}
	handler := New(":0", projects, fake).httpServer.Handler
	for _, item := range []struct{ method, path, body string }{{http.MethodGet, "/api/v1/projects/" + projectID.String() + "/storylines", ""}, {http.MethodPost, "/api/v1/projects/" + projectID.String() + "/storylines", `{"name":"x","summary":"s","start_chapter":null,"end_chapter":null,"status":"active","sort_order":0}`}, {http.MethodPost, "/api/v1/storylines/" + parentID.String() + "/children", `{"name":"x","summary":"s","start_chapter":null,"end_chapter":null,"status":"active","sort_order":0}`}, {http.MethodPatch, "/api/v1/storylines/" + parentID.String(), `{"expected_version":1,"name":"x"}`}} {
		response := doRequest(handler, item.method, item.path, item.body)
		if response.Code == 404 || response.Code == 405 {
			t.Fatalf("route %s %s=%d %s", item.method, item.path, response.Code, response.Body.String())
		}
	}
	wrong := doRequest(handler, http.MethodDelete, "/api/v1/storylines/"+parentID.String(), "")
	if wrong.Code != 405 {
		t.Fatalf("wrong method=%d", wrong.Code)
	}
	content := doRequest(handler, http.MethodGet, "/api/v1/content-items", "")
	if content.Code != 404 {
		t.Fatalf("content route=%d", content.Code)
	}
}

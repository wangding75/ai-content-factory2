package workflowrun

import "testing"

func TestRuntimeBridgeExposesTheSharedStageBoundary(t *testing.T) {
	var bridge RuntimeBridge = NewRuntimeBridge(nil)
	var chapterPlanning ChapterPlanningRuntime = NewRuntimeBridge(nil)
	if bridge == nil {
		t.Fatal("runtime bridge must be available for every stage")
	}
	if chapterPlanning == nil {
		t.Fatal("runtime bridge must satisfy the chapter-planning boundary")
	}
}

package workflowrun

import "testing"

func TestRuntimeBridgeExposesTheSharedStageBoundary(t *testing.T) {
	var bridge RuntimeBridge = NewRuntimeBridge(nil)
	if bridge == nil {
		t.Fatal("runtime bridge must be available for every stage")
	}
}

import assert from "node:assert/strict";
import test from "node:test";
import { toWorkflowPreflightReasons } from "./workflow-preflight-reason.ts";

test("preserves code, message, action, and every repair target field", () => {
  assert.deepEqual(toWorkflowPreflightReasons([{ code: "missing", message: "Needs setup", repairAction: "provider:edit", repairTarget: {
    providerId: "provider", connectionId: "connection", workflowConfigurationId: "workflow", projectId: "project", stage: "content_review",
  } }]), [{ code: "missing", message: "Needs setup", repairAction: "provider:edit", providerId: "provider", connectionId: "connection", workflowConfigurationId: "workflow", projectId: "project", stage: "content_review" }]);
});

test("does not invent identifiers for incomplete targets", () => {
  assert.deepEqual(toWorkflowPreflightReasons([{ code: "missing", message: "Needs setup", repairAction: "connection:edit", repairTarget: null }]), [{ code: "missing", message: "Needs setup", repairAction: "connection:edit", providerId: undefined, connectionId: undefined, workflowConfigurationId: undefined, projectId: undefined, stage: undefined }]);
});

import assert from "node:assert/strict";
import test from "node:test";
import { workflowPreflightRepairLink } from "./workflow-preflight-repair-route.ts";

const target = {
  providerId: "provider/id",
  connectionId: "connection/id",
  workflowConfigurationId: "workflow/id",
  projectId: "project/id",
  stage: "review",
};

test("maps every current provider action to the selected Provider", () => {
  for (const action of ["provider:verify", "provider:enable", "provider:models", "provider:edit"]) {
    assert.deepEqual(workflowPreflightRepairLink(action, target), { href: "/settings?providerId=provider%2Fid", known: true });
  }
});

test("maps every current Connection action to the selected Connection", () => {
  for (const action of ["connection:verify", "connection:enable", "connection:edit"]) {
    assert.deepEqual(workflowPreflightRepairLink(action, target), { href: "/settings?tab=connections&connectionId=connection%2Fid", known: true });
  }
});

test("maps every current Workflow Configuration action to the selected configuration", () => {
  for (const action of ["workflow_configuration:verify", "workflow_configuration:enable", "workflow_configuration:edit"]) {
    assert.deepEqual(workflowPreflightRepairLink(action, target), { href: "/settings?tab=workflows&workflowConfigurationId=workflow%2Fid", known: true });
  }
});

test("maps every binding action to the selected project stage", () => {
  for (const action of ["workflow_binding:configure", "workflow_binding:repair"]) {
    assert.deepEqual(workflowPreflightRepairLink(action, target), { href: "/projects/project%2Fid/settings?tab=workflow-bindings&stage=review", known: true });
  }
});

test("uses a safe configuration link when an action or binding target is unknown", () => {
  assert.deepEqual(workflowPreflightRepairLink("future:repair", target), { href: "/settings", known: false });
  assert.deepEqual(workflowPreflightRepairLink("workflow_binding:configure", { projectId: "project" }), { href: "/settings", known: false });
});

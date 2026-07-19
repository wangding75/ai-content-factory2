import assert from "node:assert/strict";
import test from "node:test";
import { createWorkflowPayload, updateWorkflowPayload, validateWorkflow, type WorkflowForm } from "./workflow-api.ts";

const form:WorkflowForm={name:"w",connectionId:"id",applicableStages:["review"],referenceType:"workflow_id",referenceValue:"chapter-plan",inputContractVersion:"v1",outputContractVersion:"v1",defaultParametersJson:"{}",note:""};
test("workflow payload keeps derived fields out of writes",()=>{assert.equal("workflowType" in createWorkflowPayload(form),false);assert.deepEqual(createWorkflowPayload(form).typeConfig,{referenceType:"workflow_id",referenceValue:"chapter-plan"});assert.equal(updateWorkflowPayload(form,2).expectedVersion,2);});
test("workflow only accepts default parameter objects",()=>{assert.match(validateWorkflow({...form,defaultParametersJson:"[]"})??"",/JSON 对象/);assert.match(validateWorkflow({...form,defaultParametersJson:"{"})??"",/JSON 对象/);});

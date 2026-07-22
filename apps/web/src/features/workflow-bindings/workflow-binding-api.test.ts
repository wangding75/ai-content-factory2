import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
const source=readFileSync(new URL("./workflow-binding-api.ts",import.meta.url),"utf8");
test("workflow bindings keep frozen stage order and request concurrency fields",()=>{assert.match(source,/stageOrder:WorkflowStage\[\]=\["chapter_planning","content_generation","review","rewrite"\]/);assert.match(source,/expectedVersion/);assert.match(source,/expected_version/);assert.match(source,/Idempotency-Key/);});

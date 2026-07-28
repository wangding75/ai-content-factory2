import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

test("正文生成 API 使用冻结预检、专用创建端点和幂等键", () => {
  const source = readFileSync(join(process.cwd(), "src", "features", "content-items", "content-item-http-api.ts"), "utf8");
  assert.match(source, /content-generation-summary/);
  assert.match(source, /content-generation-runs\/preflight/);
  assert.match(source, /content-generation-runs`,/);
  assert.match(source, /"Idempotency-Key": idempotencyKey/);
  assert.match(source, /preflightToken: string/);
  assert.doesNotMatch(source, /createWorkflowRun/);
});

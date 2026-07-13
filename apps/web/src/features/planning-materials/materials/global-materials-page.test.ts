import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("./global-materials-page.tsx", import.meta.url), "utf8");

test("global material list delegates loading, filters, pagination, and retry to the real API", () => {
  assert.match(source, /listMaterialsFromApi\(\{ q, type, sort, limit, offset \}/);
  assert.match(source, /setOffset\(offset \+ limit\)/);
  assert.match(source, /onClick=\{\(\) => void load\(\)\}/);
  assert.match(source, /new AbortController\(\)/);
  assert.doesNotMatch(source, /material-repository|mockScenario/);
});

test("global material list keeps empty and error states without fabricating totals", () => {
  assert.match(source, /setTotal\(result.total\)/);
  assert.match(source, /items\.length/);
  assert.match(source, /setError\(/);
});

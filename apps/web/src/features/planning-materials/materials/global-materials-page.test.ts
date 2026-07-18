import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("./global-materials-page.tsx", import.meta.url), "utf8");

test("global material list delegates loading, filters, pagination, and retry to the real API", () => {
  assert.match(source, /listMaterialsFromApi\(\{ scope: "global", q, type, sort, limit, offset \}/);
  assert.match(source, /if \(signal\?\.aborted\) return/);
  assert.match(source, /setOffset\(offset \+ limit\)/);
  assert.match(source, /onClick=\{\(\) => void load\(\)\}/);
  assert.match(source, /new AbortController\(\)/);
  assert.doesNotMatch(source, /material-repository|mockScenario/);
});

test("Chinese global materials copy uses mapped types, filter defaults, and the project empty-state route", () => {
  for (const text of ["全部类型", "最近更新", "暂无素材", "在项目中创建或绑定素材后，会同步显示在这里。", "前往项目", "人物", "世界观", "地点", "组织", "道具", "参考资料"]) assert.match(source, new RegExp(text));
  assert.match(source, /document\.documentElement\.lang/);
  assert.match(source, /<Link href="\/projects">\{t\.goProjects\}<\/Link>/);
  assert.doesNotMatch(source, /\/materials\/new/);
});

test("global material list keeps real error, retry, and filtered-empty states without fabricating totals", () => {
  assert.match(source, /setTotal\(result\.total\)/);
  assert.match(source, /items\.length/);
  assert.match(source, /setError\(/);
  assert.match(source, /loadErrorTitle/);
  assert.match(source, /filteredEmptyTitle/);
});

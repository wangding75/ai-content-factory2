import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import test from "node:test";

const source = readFileSync(new URL("./create-material-page.tsx", import.meta.url), "utf8");

test("create material renders a dedicated modal with fixed header, body, and footer regions", () => {
  for (const region of ["create-material-modal__header", "create-material-modal__body", "create-material-modal__footer"]) assert.match(source, new RegExp(`className=\\"${region}`));
  assert.match(source, /新建素材/);
  assert.match(source, /创建素材并自动绑定到当前项目/);
  assert.match(source, /aria-label="关闭"/);
  assert.match(source, /当前项目：/);
  assert.match(source, /项目类型：/);
  assert.match(source, /getPlanningProject\(projectId, scenario\)/);
});

test("create material maps prototype usage controls without chapter inputs", () => {
  assert.match(source, /<select value=\{form\.usage\.usage_type\}/);
  assert.match(source, /具体角色/);
  assert.doesNotMatch(source, /角色名称/);
  assert.doesNotMatch(source, /起始章节/);
  assert.doesNotMatch(source, /结束章节/);
  assert.match(source, /start_chapter: null, end_chapter: null/);
  assert.match(source, /create-material-modal__submit/);
  assert.doesNotMatch(source, /danger/);
});

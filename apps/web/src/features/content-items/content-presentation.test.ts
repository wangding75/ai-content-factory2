import assert from "node:assert/strict";
import test from "node:test";
import { contentVersionSourceLabel, contentVersionStatusLabel, reviewCategoryLabel, reviewConclusionLabel } from "./content-presentation.ts";
import type { ContentVersionSource } from "./content-item-http-api.ts";

const expectedLabels: Record<ContentVersionSource, string> = {
  manual_created: "手动创建",
  mock_generated: "模拟生成",
  mock_rewrite: "模拟重写",
  workflow_generated: "工作流生成",
  workflow_rewrite: "工作流重写",
};

test("content version source labels map every legal enum", () => {
  const sources = Object.keys(expectedLabels) as ContentVersionSource[];
  assert.equal(sources.length, 5);
  for (const source of sources) {
    assert.equal(contentVersionSourceLabel(source), expectedLabels[source]);
  }
});

test("workflow_rewrite and mock_rewrite are distinct from their generated counterparts", () => {
  assert.equal(contentVersionSourceLabel("workflow_rewrite"), "工作流重写");
  assert.notEqual(contentVersionSourceLabel("workflow_rewrite"), contentVersionSourceLabel("workflow_generated"));
  assert.equal(contentVersionSourceLabel("mock_rewrite"), "模拟重写");
  assert.notEqual(contentVersionSourceLabel("mock_rewrite"), contentVersionSourceLabel("mock_generated"));
});

test("content version metadata is localized with safe fallbacks", () => {
  assert.equal(contentVersionSourceLabel("mock_rewrite"), "模拟重写");
  assert.equal(contentVersionSourceLabel("provider_debug"), "其他来源");
  assert.equal(contentVersionStatusLabel("editable_draft"), "可编辑草稿");
});

test("review labels never expose raw enums", () => {
  assert.equal(reviewConclusionLabel("pass"), "审核通过");
  assert.equal(reviewCategoryLabel("character_consistency"), "人物一致性");
  assert.equal(reviewCategoryLabel("provider_rule"), "其他问题");
});

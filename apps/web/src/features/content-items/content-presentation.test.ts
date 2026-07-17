import assert from "node:assert/strict";
import test from "node:test";
import { contentVersionSourceLabel, contentVersionStatusLabel, reviewCategoryLabel, reviewConclusionLabel } from "./content-presentation.ts";

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

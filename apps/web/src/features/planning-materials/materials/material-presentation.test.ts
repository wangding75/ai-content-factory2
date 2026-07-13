import assert from "node:assert/strict";
import test from "node:test";
import { materialFields, materialTypeLabels, roleNameForUsage, usageShowsRole } from "./material-presentation.ts";

test("presentation maps material types and schema fields without raw keys", () => {
  assert.equal(materialTypeLabels.character, "人物");
  assert.deepEqual(materialFields("character", { age: 26, appearance: "短发", background: "边境", personality: "冷静" }), [
    { label: "年龄", value: "26" }, { label: "外貌特征", value: "短发" }, { label: "背景", value: "边境" }, { label: "性格", value: "冷静" },
  ]);
  assert.deepEqual(materialFields("location", { atmosphere: "压抑", age: "" }), [{ label: "氛围", value: "压抑" }]);
});

test("only character usage keeps a role name for create bind and usage updates", () => {
  assert.equal(usageShowsRole("人物角色"), true);
  assert.equal(usageShowsRole("环境场景"), false);
  assert.equal(roleNameForUsage("人物角色", "主角"), "主角");
  assert.equal(roleNameForUsage("环境场景", "主角"), "");
  assert.equal(roleNameForUsage("背景设定", "历史错误角色"), "");
});
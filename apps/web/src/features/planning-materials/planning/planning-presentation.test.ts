import assert from "node:assert/strict";
import test from "node:test";
import { planningCopy, planningSaveStatus } from "./planning-presentation.ts";

test("planning presentation has explicit safe empty states", () => {
  assert.equal(planningCopy.emptyPremise, "尚未填写核心主题");
  assert.equal(planningCopy.emptySellingPoints, "暂无核心卖点");
  assert.equal(planningSaveStatus({ version: 0 } as never), "尚未保存策划内容");
  assert.equal(planningSaveStatus({ version: 3 } as never), "已保存（版本 3）");
});

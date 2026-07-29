import assert from "node:assert/strict";
import test from "node:test";
import {
  locateIssueInSource,
  realReviewConclusionLabel,
  realReviewSeverityLabel,
  reviewDimensionLabel,
  reviewLocationLabel,
  reviewStateLabel,
  safeReviewError,
} from "./content-review-presentation.ts";

const issue = {
  evidence: { quote: "第二段需要高亮", sourceRefs: ["正文"] },
  location: {
    paragraphStart: 2,
    paragraphEnd: 2,
    sentenceStart: 1,
    sentenceEnd: 1,
  },
};

test("review presentation localizes all eight states and formal enums", () => {
  assert.deepEqual(
    [
      "idle",
      "not_configured",
      "queued",
      "running",
      "review_ready",
      "runtime_failed",
      "output_validation_failed",
      "result_consumption_failed",
    ].map((state) => reviewStateLabel(state as Parameters<typeof reviewStateLabel>[0])),
    [
      "尚未审核",
      "未配置",
      "已排队",
      "运行中",
      "审核完成",
      "运行失败",
      "输出校验失败",
      "结果保存失败",
    ],
  );
  assert.equal(reviewDimensionLabel("factual_consistency"), "事实一致性");
  assert.equal(realReviewSeverityLabel("critical"), "严重");
  assert.equal(realReviewConclusionLabel("needs_changes"), "建议修改");
  assert.equal(reviewLocationLabel(issue.location), "第 2 段");
});

test("source location uses the frozen paragraph and exact evidence quote", () => {
  const located = locateIssueInSource(
    "第一段。\n\n这里是第二段需要高亮的内容。\n\n第三段。",
    issue,
  );
  assert.equal(located.exact, true);
  assert.equal(located.paragraphs.length, 3);
  assert.equal(located.paragraphs[1].highlighted, true);
  assert.equal(located.paragraphs[1].exactQuote, "第二段需要高亮");
});

test("source location safely falls back to evidence when location cannot match", () => {
  const located = locateIssueInSource("只有一段。", {
    evidence: { quote: "审核证据片段", sourceRefs: [] },
    location: {
      paragraphStart: 9,
      paragraphEnd: 9,
      sentenceStart: 1,
      sentenceEnd: 1,
    },
  });
  assert.equal(located.exact, false);
  assert.equal(located.paragraphs.length, 1);
  assert.equal(located.paragraphs[0].text, "审核证据片段");
  assert.equal(located.paragraphs[0].highlighted, true);
});

test("error mapper exposes safe business copy rather than raw server details", () => {
  assert.equal(
    safeReviewError(
      { code: "review_not_configured", message: "internal repository error" },
      "fallback",
    ),
    "审核工作流尚未配置，请前往项目设置完成绑定。",
  );
  assert.equal(
    safeReviewError(
      { code: "preflight_token_expired", message: "raw token" },
      "fallback",
    ),
    "审核条件已变化，请重新预检。",
  );
});

test("review resource and optimistic-lock errors have precise safe copy", () => {
  const cases = [
    ["content_version_not_found", "固定来源正文版本不存在或已不可用。"],
    ["review_not_found", "所请求的审核报告不存在或已不可用。"],
    ["review_issue_not_found", "所请求的审核问题不存在或已不可用。"],
    ["workflow_run_not_found", "所请求的审核任务不存在或已不可用。"],
    ["review_issue_version_conflict", "审核问题状态已变化，请重新加载后再操作。"],
    ["workflow_run_version_conflict", "审核任务状态已变化，请刷新后再重试。"],
  ] as const;
  for (const [code, expected] of cases) {
    assert.equal(
      safeReviewError(
        { code, status: code.includes("not_found") ? 404 : 409 },
        "fallback",
      ),
      expected,
    );
  }
});

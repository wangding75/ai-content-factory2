"use client";

import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import type { ChapterPlan } from "./chapter-plan-http-api";
import {
  chapterPlanSummary,
  createConfirmationViewModel,
} from "./chapter-plan-presentation";

export function ConfirmChapterPlansDialog({
  plans,
  allPlans,
  onClose,
  onConfirm,
  submitting,
  error,
}: {
  plans: ChapterPlan[];
  allPlans: ChapterPlan[];
  onClose: () => void;
  onConfirm: () => void;
  submitting: boolean;
  error: ApiError | null;
}) {
  const model = createConfirmationViewModel(plans, allPlans);
  const message =
    error?.status === 409
      ? "确认数据已发生变化。请返回检查并刷新列表。"
      : error
        ? "暂时无法确认章节规划，请检查网络后重试。"
        : null;
  return (
    <div className="chapter-confirm-backdrop" role="presentation">
      <section
        className="chapter-confirm-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="chapter-confirm-title"
      >
        <header>
          <div className="chapter-confirm-heading">
            <div className="chapter-confirm-icon">
              <Icon name="timeline" size={24} />
            </div>
            <div>
              <h2 id="chapter-confirm-title">确认章节规划</h2>
              <p>
                确认后，所选候选章节将进入可生产状态；不会自动生成正文或覆盖已有内容。
              </p>
            </div>
          </div>
          <button
            type="button"
            aria-label="关闭确认章节规划"
            onClick={onClose}
            disabled={submitting}
          >
            ×
          </button>
        </header>
        <div className="chapter-confirm-body">
          <p className="chapter-confirm-summary">
            <b>{model.selectedLabel}</b>
            <i>|</i>
            <span>{model.rangeLabel}</span>
            <i>|</i>
            <span>{model.sourceLabel}</span>
          </p>
          <section className="chapter-confirm-selected" aria-label="已选择章节">
            {plans.map((plan) => (
              <article key={plan.id}>
                <div>
                  <b>{plan.chapter_no}</b>
                  <strong>{plan.title}</strong>
                </div>
                <p>{chapterPlanSummary(plan.summary)}</p>
                {plan.storyline_refs_json.length > 0 && (
                  <small>
                    已关联 {plan.storyline_refs_json.length} 条故事线
                  </small>
                )}
              </article>
            ))}
          </section>
          <section className="chapter-confirm-validation" aria-label="系统校验">
            <h3>系统校验</h3>
            {model.checks.map((check) => (
              <p className={check.status} key={check.label}>
                <Icon
                  name={check.status === "error" ? "info" : "timeline"}
                  size={16}
                />
                <span>
                  <b>{check.label}</b>
                  <small>{check.detail}</small>
                </span>
              </p>
            ))}
          </section>
          {model.warnings.length > 0 && (
            <section className="chapter-confirm-warning" role="status">
              <h3>请注意</h3>
              {model.warnings.map((warning) => (
                <p key={warning}>{warning}</p>
              ))}
            </section>
          )}
          <section className="chapter-confirm-after">
            <div>
              <h3>
                <Icon name="info" size={16} />
                确认后
              </h3>
              <ul>
                <li>状态转为“已确认”</li>
                <li>可进入正文生产</li>
                <li>来源记录保留</li>
                <li>不会自动生成正文或覆盖现有内容</li>
              </ul>
            </div>
            <div>
              <h3>仍可执行</h3>
              <ul>
                <li>查看章节规划</li>
                <li>调整关联关系</li>
                <li>手动进入正文生产</li>
              </ul>
            </div>
          </section>
          {message && (
            <p className="chapter-confirm-error" role="alert">
              {message}
            </p>
          )}
        </div>
        <footer>
          <button type="button" onClick={onClose} disabled={submitting}>
            返回检查
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={submitting || !model.canConfirm}
          >
            {submitting ? "确认中…" : "确认章节规划"}
          </button>
        </footer>
      </section>
    </div>
  );
}

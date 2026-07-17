"use client";
/* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { Icon } from "@/components/ui/icons";
import {
  listChapterPlans,
  type ChapterPlan,
} from "@/features/chapter-plans/chapter-plan-http-api";
import { ApiError } from "@/lib/api";
import {
  createOrGetContentItem,
  getContentItem,
  getReviewDetail,
  listContentReviews,
  mockReviewContent,
  type ContentItemDetail,
  type ContentReviewList,
  type ReviewDetail,
} from "./content-item-http-api";
import { formatChineseDate, reviewCategoryLabel, reviewConclusionLabel, reviewSeverityLabel, workflowStatusLabel } from "./content-presentation";

const PAGE_SIZE = 10;
const idKey = () => crypto.randomUUID();

function safeError(cause: unknown, fallback: string) {
  const error = cause as ApiError;
  if (error?.status === 404) return "所请求的审核记录不存在或已不可用。";
  if (error?.code === "version_conflict")
    return "正文版本已发生变化，请返回编辑器刷新后重试。";
  if (error?.code === "content_version_already_reviewed")
    return "该正文版本已完成审核，请查看审核结果。";
  if (error?.code === "idempotency_key_reused_with_different_payload")
    return "本次提交参数与原请求不一致，请取消后重新发起审核。";
  if (error?.status === 400 || error?.status === 422)
    return "审核参数无效，请返回编辑器确认正文后重试。";
  return fallback;
}

export function ContentReviewWorkspace({
  projectId,
  chapterPlanId,
}: {
  projectId: string;
  chapterPlanId: string;
}) {
  const [content, setContent] = useState<ContentItemDetail | null>(null);
  const [plan, setPlan] = useState<ChapterPlan | null>(null);
  const [history, setHistory] = useState<ContentReviewList | null>(null);
  const [offset, setOffset] = useState(0);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selected, setSelected] = useState<ReviewDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyError, setHistoryError] = useState<string | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [dialog, setDialog] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const itemRef = useRef("");
  const sequence = useRef(0);
  const detailSequence = useRef(0);
  const controllers = useRef<AbortController[]>([]);
  const reviewKey = useRef<string | null>(null);

  const addController = () => {
    const controller = new AbortController();
    controllers.current.push(controller);
    return controller;
  };
  const abortAll = () => {
    controllers.current.forEach((controller) => controller.abort());
    controllers.current = [];
  };

  const loadHistory = useCallback(
    async (
      contentItemId: string,
      nextOffset: number,
      request: number,
      chooseFirst: boolean,
    ) => {
      const controller = addController();
      setHistoryLoading(true);
      setHistoryError(null);
      try {
        const page = await listContentReviews(
          contentItemId,
          { limit: PAGE_SIZE, offset: nextOffset },
          { signal: controller.signal },
        );
        if (
          controller.signal.aborted ||
          request !== sequence.current ||
          itemRef.current !== contentItemId
        )
          return;
        setHistory(page);
        if (chooseFirst && page.items[0]) setSelectedId(page.items[0].id);
      } catch (cause) {
        if (
          !controller.signal.aborted &&
          request === sequence.current &&
          itemRef.current === contentItemId
        )
          setHistoryError(
            safeError(cause, "审核历史加载失败，请检查网络后重试。"),
          );
      } finally {
        if (
          !controller.signal.aborted &&
          request === sequence.current &&
          itemRef.current === contentItemId
        )
          setHistoryLoading(false);
      }
    },
    [],
  );

  const load = useCallback(async () => {
    const request = ++sequence.current;
    abortAll();
    itemRef.current = "";
    reviewKey.current = null;
    setLoading(true);
    setContent(null);
    setPlan(null);
    setHistory(null);
    setOffset(0);
    setSelectedId(null);
    setSelected(null);
    setDetailError(null);
    setDialog(false);
    setSubmitError(null);
    try {
      const controller = addController();
      const [created, plans] = await Promise.all([
        createOrGetContentItem(chapterPlanId, { signal: controller.signal }),
        listChapterPlans(
          projectId,
          { limit: 100, offset: 0 },
          { signal: controller.signal },
        ),
      ]);
      if (controller.signal.aborted || request !== sequence.current) return;
      itemRef.current = created.content_item.id;
      setPlan(plans.items.find((item) => item.id === chapterPlanId) ?? null);
      const fresh = await getContentItem(created.content_item.id, {
        signal: addController().signal,
      });
      if (
        request !== sequence.current ||
        itemRef.current !== fresh.content_item.id
      )
        return;
      setContent(fresh);
      await loadHistory(fresh.content_item.id, 0, request, true);
    } catch (cause) {
      if (request === sequence.current)
        setHistoryError(
          safeError(cause, "审核页面加载失败，请检查网络后重试。"),
        );
    } finally {
      if (request === sequence.current) setLoading(false);
    }
  }, [chapterPlanId, loadHistory, projectId]);

  useEffect(() => {
    void load();
    return () => {
      sequence.current++;
      detailSequence.current++;
      abortAll();
    };
  }, [load]);

  const loadDetail = useCallback(async (reviewId: string) => {
    const request = ++detailSequence.current;
    const expectedItem = itemRef.current;
    const controller = addController();
    setDetailLoading(true);
    setDetailError(null);
    setSelected(null);
    try {
      const detail = await getReviewDetail(reviewId, {
        signal: controller.signal,
      });
      if (
        controller.signal.aborted ||
        request !== detailSequence.current ||
        itemRef.current !== expectedItem ||
        detail.review.content_item_id !== expectedItem ||
        detail.review.id !== reviewId
      )
        return;
      setSelected(detail);
    } catch (cause) {
      if (
        !controller.signal.aborted &&
        request === detailSequence.current &&
        itemRef.current === expectedItem
      )
        setDetailError(safeError(cause, "审核详情加载失败，请稍后重试。"));
    } finally {
      if (
        !controller.signal.aborted &&
        request === detailSequence.current &&
        itemRef.current === expectedItem
      )
        setDetailLoading(false);
    }
  }, []);

  useEffect(() => {
    if (selectedId) void loadDetail(selectedId);
  }, [loadDetail, selectedId]);

  const changePage = (nextOffset: number) => {
    if (!content || historyLoading) return;
    setOffset(nextOffset);
    setSelectedId(null);
    setSelected(null);
    setDetailError(null);
    void loadHistory(
      content.content_item.id,
      nextOffset,
      sequence.current,
      true,
    );
  };

  const submit = async () => {
    if (
      !content ||
      submitting ||
      content.content_item.status !== "draft" ||
      content.current_version.status !== "editable_draft"
    )
      return;
    const contentItemId = content.content_item.id;
    const versionId = content.current_version.id;
    const expectedVersion = content.current_version.version;
    const request = sequence.current;
    const controller = addController();
    const key = reviewKey.current ?? idKey();
    reviewKey.current = key;
    setSubmitting(true);
    setSubmitError(null);
    try {
      const result = await mockReviewContent(
        contentItemId,
        { content_version_id: versionId, expected_version: expectedVersion },
        key,
        { signal: controller.signal },
      );
      if (
        controller.signal.aborted ||
        request !== sequence.current ||
        itemRef.current !== contentItemId ||
        result.content_item.id !== contentItemId ||
        result.review.content_version_id !== versionId
      )
        return;
      const refreshed = await getContentItem(contentItemId, {
        signal: addController().signal,
      });
      if (
        request !== sequence.current ||
        itemRef.current !== contentItemId ||
        refreshed.current_version.id !== versionId
      )
        return;
      setContent(refreshed);
      setDialog(false);
      reviewKey.current = null;
      setOffset(0);
      setSelectedId(result.review.id);
      setSelected(null);
      await loadHistory(contentItemId, 0, request, false);
    } catch (cause) {
      if (
        !controller.signal.aborted &&
        request === sequence.current &&
        itemRef.current === contentItemId
      )
        setSubmitError(
          safeError(cause, "审核提交失败，正文没有被前端修改。请重试。"),
        );
    } finally {
      if (
        !controller.signal.aborted &&
        request === sequence.current &&
        itemRef.current === contentItemId
      )
        setSubmitting(false);
    }
  };

  if (loading)
    return (
      <State
        title="正在加载审核"
        description="正在读取正文与审核历史…"
        loading
      />
    );
  if (!content)
    return (
      <State
        title="无法打开审核"
        description={historyError ?? "正文或审核记录不存在。"}
        retry={() => void load()}
      />
    );
  const canSubmit =
    content.content_item.status === "draft" &&
    content.current_version.status === "editable_draft";
  const canPrev = offset > 0,
    canNext = !!history && offset + history.limit < history.total;
  const reviewCount = history?.total ?? 0;
  return (
    <main className="content-review">
      <header className="content-review-header">
        <div>
          <h1>
            审核｜第 {plan?.chapter_no ?? "—"} 章{" "}
            {content.current_version.title}
          </h1>
          <p>
            当前版本：v{content.current_version.version_no} ｜正文字数：
            {content.current_version.word_count} 字｜内容状态：
            {content.content_item.status === "draft" ? "待审核" : "已审核"}
          </p>
        </div>
        <div className="content-review-actions">
          <Link
            href={`/projects/${projectId}/chapter-plans/${chapterPlanId}/content`}
          >
            <Icon name="arrowLeft" size={17} />
            返回编辑
          </Link>
          {canSubmit && (
            <button
              onClick={() => {
                setDialog(true);
                setSubmitError(null);
              }}
            >
              发起模拟审核
            </button>
          )}
          <button disabled title="Iteration 07">
            创建重写版本
          </button>
        </div>
      </header>
      <section className="content-review-grid">
        <article className="content-review-text">
          <header>
            <span>原始章节文本（只读）</span>
            <small>固定版本 v{content.current_version.version_no}</small>
          </header>
          <div>
            {content.current_version.content ? (
              content.current_version.content
                .split(/\n{2,}/)
                .map((paragraph, index) => <p key={index}>{paragraph}</p>)
            ) : (
              <p>正文为空。</p>
            )}
          </div>
        </article>
        <aside className="content-review-side">
          <section className="content-review-history">
            <header>
              <h2>审核历史{history ? `（${history.total}）` : ""}</h2>
              <button
                onClick={() =>
                  content &&
                  void loadHistory(
                    content.content_item.id,
                    offset,
                    sequence.current,
                    false,
                  )
                }
                disabled={historyLoading}
              >
                刷新
              </button>
            </header>
            {historyLoading && !history ? (
              <p className="content-review-muted">加载中…</p>
            ) : historyError ? (
              <State
                title="审核历史加载失败"
                description={historyError}
                retry={() =>
                  content &&
                  void loadHistory(
                    content.content_item.id,
                    offset,
                    sequence.current,
                    false,
                  )
                }
                compact
              />
            ) : history?.total === 0 ? (
              <p className="content-review-muted">
                尚无审核记录。请发起模拟审核。
              </p>
            ) : (
              <>
                <div className="content-review-list">
                  {history?.items.map((report) => (
                    <button
                      key={report.id}
                      className={selectedId === report.id ? "active" : ""}
                      onClick={() => setSelectedId(report.id)}
                    >
                      <b>{reviewConclusionLabel(report.conclusion)}</b>
                      <span>
                        评分 {report.score} · {formatChineseDate(report.created_at)}
                      </span>
                    </button>
                  ))}
                </div>
                <footer>
                  <span>共 {reviewCount} 条</span>
                  <div>
                    <button
                      disabled={!canPrev || historyLoading}
                      onClick={() =>
                        changePage(Math.max(0, offset - PAGE_SIZE))
                      }
                    >
                      上一页
                    </button>
                    <button
                      disabled={!canNext || historyLoading}
                      onClick={() => changePage(offset + PAGE_SIZE)}
                    >
                      下一页
                    </button>
                  </div>
                </footer>
              </>
            )}
          </section>
          {detailLoading ? (
            <p className="content-review-muted">正在加载审核详情…</p>
          ) : detailError ? (
            <State
              title="审核详情不可用"
              description={detailError}
              retry={() => selectedId && void loadDetail(selectedId)}
              compact
            />
          ) : selected ? (
            <ReviewDetailPanel detail={selected} />
          ) : (
            <section className="content-review-empty-detail">
              <Icon name="info" size={22} />
              <p>选择一条审核历史查看详情。</p>
            </section>
          )}
        </aside>
      </section>
      {dialog && (
        <ReviewDialog
          submitting={submitting}
          error={submitError}
          onClose={() => {
            if (!submitting) {
              setDialog(false);
              setSubmitError(null);
              reviewKey.current = null;
            }
          }}
          onSubmit={() => void submit()}
        />
      )}
    </main>
  );
}

function ReviewDetailPanel({ detail }: { detail: ReviewDetail }) {
  return (
    <section className="content-review-detail">
      <section className={`content-review-score ${detail.review.conclusion}`}>
        <div>
          <span>模拟评审</span>
          <span>已完成</span>
        </div>
        <h2>{reviewConclusionLabel(detail.review.conclusion)}</h2>
        <p>
          评分 {detail.review.score}｜{detail.review.summary || "暂无审核摘要"}
        </p>
        <small>完成时间：{formatChineseDate(detail.workflow_run.finished_at)}</small>
      </section>
      <section>
        <h3>审核时固定正文</h3>
        <dl>
          <div>
            <dt>版本</dt>
            <dd>
              v{detail.content_version.version_no}（锁定版本{" "}
              {detail.content_version.version}）
            </dd>
          </div>
          <div>
            <dt>标题</dt>
            <dd>{detail.content_version.title}</dd>
          </div>
          <div>
            <dt>字数</dt>
            <dd>{detail.content_version.word_count} 字</dd>
          </div>
          <div>
            <dt>冻结时间</dt>
            <dd>{formatChineseDate(detail.content_version.frozen_at)}</dd>
          </div>
        </dl>
      </section>
      <section>
        <h3>问题清单（{detail.findings.length}）</h3>
        {detail.findings.length ? (
          <div className="content-review-findings">
            {detail.findings.map((finding) => (
              <article className={finding.severity} key={finding.id}>
                <p>
                  <b>
                    {reviewCategoryLabel(finding.category)}｜{reviewSeverityLabel(finding.severity)}
                  </b>
                  {finding.location && (
                    <span>
                      正文位置 {finding.location.start_offset ?? 0}–
                      {finding.location.end_offset ?? 0}
                    </span>
                  )}
                </p>
                <h4>{finding.title}</h4>
                <small>{finding.description}</small>
              </article>
            ))}
          </div>
        ) : (
          <p className="content-review-muted">未发现问题。</p>
        )}
      </section>
      <section className="content-review-recommendations">
        <h3>
          <Icon name="lightbulb" size={18} />
          修改建议
        </h3>
        {detail.recommendations.length ? (
          <ul>
            {detail.recommendations.map((item) => (
              <li key={item.id}>
                <b>{item.title}</b>
                <span>{item.description}</span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="content-review-muted">暂无修改建议。</p>
        )}
      </section>
      <section className="content-review-run">
        <h3>工作流摘要</h3>
        <p>
          审核任务｜{workflowStatusLabel(detail.workflow_run.status)}｜开始：{formatChineseDate(detail.workflow_run.started_at)}
        </p>
      </section>
    </section>
  );
}
function ReviewDialog({
  submitting,
  error,
  onClose,
  onSubmit,
}: {
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: () => void;
}) {
  return (
    <div className="content-review-backdrop">
      <section
        className="content-review-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="content-review-dialog-title"
      >
        <header>
          <div>
            <h2 id="content-review-dialog-title">发起模拟审核</h2>
            <p>将审核当前 v1，并在成功后冻结该版本。正文不会被修改。</p>
          </div>
          <button onClick={onClose} disabled={submitting} aria-label="关闭">
            <Icon name="close" size={20} />
          </button>
        </header>
        <div>
          <p>
            审核使用当前 ContentVersion 与乐观锁版本。提交期间不可重复操作。
          </p>
          {error && (
            <p className="content-review-error" role="alert">
              {error}
            </p>
          )}
        </div>
        <footer>
          <button onClick={onClose} disabled={submitting}>
            取消
          </button>
          <button onClick={onSubmit} disabled={submitting}>
            {submitting ? "提交中…" : "确认发起审核"}
          </button>
        </footer>
      </section>
    </div>
  );
}
function State({
  title,
  description,
  retry,
  loading,
  compact,
}: {
  title: string;
  description: string;
  retry?: () => void;
  loading?: boolean;
  compact?: boolean;
}) {
  return (
    <section
      className={
        compact ? "content-review-state compact" : "content-review-state"
      }
    >
      <Icon name="info" size={compact ? 22 : 34} />
      <h2>{title}</h2>
      <p>{description}</p>
      {loading ? (
        <span>加载中…</span>
      ) : (
        retry && <button onClick={retry}>重试</button>
      )}
    </section>
  );
}

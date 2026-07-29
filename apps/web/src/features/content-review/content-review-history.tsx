"use client";
/* eslint-disable react-hooks/set-state-in-effect */

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { Icon } from "@/components/ui/icons";
import {
  getReview,
  isRealReviewDetail,
  isRealReviewHistoryItem,
  listContentReviewHistory,
  type ReviewDetail,
  type ReviewHistoryEntry,
  type ReviewHistoryPage,
} from "./content-review-api";
import { reviewCopy as copy } from "./content-review-locale";
import {
  formatReviewTime,
  realReviewConclusionLabel,
  reviewStateLabel,
  safeReviewError,
} from "./content-review-presentation";
import { reviewConclusionLabel } from "@/features/content-items/content-presentation";

const PAGE_SIZE = 20;

export function ContentReviewHistory({
  projectId,
  workId,
}: {
  projectId: string;
  workId: string;
}) {
  const [page, setPage] = useState<ReviewHistoryPage | null>(null);
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<ReviewHistoryEntry | null>(null);
  const [detail, setDetail] = useState<ReviewDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true);
      setError(null);
      try {
        const next = await listContentReviewHistory(
          workId,
          { limit: PAGE_SIZE, offset },
          { signal },
        );
        if (!signal?.aborted) {
          setPage(next);
          setSelected(next.items[0] ?? null);
        }
      } catch (cause) {
        if (!signal?.aborted)
          setError(safeReviewError(cause, copy.history.loadFailed));
      } finally {
        if (!signal?.aborted) setLoading(false);
      }
    },
    [offset, workId],
  );

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const selectedReportId = selected
    ? isRealReviewHistoryItem(selected)
      ? selected.reportSummary?.id
      : selected.id
    : null;

  useEffect(() => {
    if (!selectedReportId) {
      setDetail(null);
      return;
    }
    const controller = new AbortController();
    setDetail(null);
    setDetailLoading(true);
    void getReview(selectedReportId, { signal: controller.signal })
      .then((next) => {
        if (!controller.signal.aborted) setDetail(next);
      })
      .catch(() => undefined)
      .finally(() => {
        if (!controller.signal.aborted) setDetailLoading(false);
      });
    return () => controller.abort();
  }, [selectedReportId]);

  return (
    <main className="review-history-page">
      <header>
        <div>
          <h2>{copy.history.title}</h2>
          <p>{copy.history.subtitle}</p>
        </div>
        <nav>
          <Link href={`/projects/${projectId}/works/${workId}/review`}>
            {copy.history.back}
          </Link>
        </nav>
      </header>
      <section className="review-history-summary">
        <span>
          {copy.history.allRuns} <b>{page?.total ?? 0}</b>
        </span>
        <span>
          {copy.history.currentPage} <b>{page?.items.length ?? 0}</b>
        </span>
      </section>
      {loading ? (
        <HistoryState title={copy.common.loading} loading />
      ) : error ? (
        <HistoryState
          title={copy.history.loadFailed}
          description={error}
          retry={() => void load()}
        />
      ) : !page?.items.length ? (
        <HistoryState title={copy.history.empty} />
      ) : (
        <div className="review-history-layout">
          <section className="review-history-table-wrap">
            <table>
              <thead>
                <tr>
                  <th>{copy.history.time}</th>
                  <th>{copy.history.version}</th>
                  <th>{copy.history.status}</th>
                  <th>{copy.history.conclusion}</th>
                  <th>{copy.history.workflow}</th>
                  <th>{copy.history.action}</th>
                </tr>
              </thead>
              <tbody>
                {page.items.map((item) => (
                  <HistoryRow
                    key={historyKey(item)}
                    item={item}
                    projectId={projectId}
                    workId={workId}
                    selected={selected === item}
                    onSelect={() => setSelected(item)}
                  />
                ))}
              </tbody>
            </table>
            <footer>
              <button
                type="button"
                disabled={offset === 0}
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              >
                {copy.history.previous}
              </button>
              <button
                type="button"
                disabled={offset + PAGE_SIZE >= page.total}
                onClick={() => setOffset(offset + PAGE_SIZE)}
              >
                {copy.history.next}
              </button>
            </footer>
          </section>
          <HistoryDetail
            item={selected}
            detail={detail}
            loading={detailLoading}
            projectId={projectId}
            workId={workId}
          />
        </div>
      )}
    </main>
  );
}

function HistoryRow({
  item,
  projectId,
  workId,
  selected,
  onSelect,
}: {
  item: ReviewHistoryEntry;
  projectId: string;
  workId: string;
  selected: boolean;
  onSelect: () => void;
}) {
  if (!isRealReviewHistoryItem(item))
    return (
      <tr className={selected ? "selected" : ""} onClick={onSelect}>
        <td>{formatReviewTime(item.created_at)}</td>
        <td>{copy.history.mock}</td>
        <td><span className="history-status succeeded">{copy.history.completed}</span></td>
        <td>{reviewConclusionLabel(item.conclusion)}</td>
        <td>{copy.history.mock}</td>
        <td>
          <Link
            href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(item.id)}`}
          >
            {copy.history.viewReport}
          </Link>
        </td>
      </tr>
    );
  const run = item.workflowRun;
  const state =
    run.status === "queued"
      ? "queued"
      : run.status === "running"
        ? "running"
        : item.reportSummary
          ? "review_ready"
          : run.status === "succeeded"
            ? "result_consumption_failed"
            : "runtime_failed";
  return (
    <tr className={selected ? "selected" : ""} onClick={onSelect}>
      <td>{formatReviewTime(run.createdAt)}</td>
      <td>V{item.sourceContentVersionSummary.versionNo}</td>
      <td>
        <span className={`history-status ${state}`}>
          {reviewStateLabel(state)}
        </span>
      </td>
      <td>
        {item.reportSummary
          ? realReviewConclusionLabel(item.reportSummary.conclusion)
          : copy.history.noReport}
      </td>
      <td>{item.reportSummary ? copy.history.reviewWorkflow : reviewStateLabel(state)}</td>
      <td>
        {item.reportSummary ? (
          <Link
            href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(item.reportSummary.id)}`}
          >
            {copy.history.viewReport}
          </Link>
        ) : (
          <Link href={`/workflow-runs/${run.id}`}>
            {state === "queued" || state === "running"
              ? copy.history.viewProgress
              : copy.history.viewFailure}
          </Link>
        )}
      </td>
    </tr>
  );
}

function HistoryDetail({
  item,
  detail,
  loading,
  projectId,
  workId,
}: {
  item: ReviewHistoryEntry | null;
  detail: ReviewDetail | null;
  loading: boolean;
  projectId: string;
  workId: string;
}) {
  if (!item)
    return <aside className="review-history-detail">{copy.history.empty}</aside>;
  const real = isRealReviewHistoryItem(item);
  const run = real ? item.workflowRun : null;
  const counts =
    detail && isRealReviewDetail(detail)
      ? detail.issues.reduce(
          (result, issue) => {
            result[issue.severity] += 1;
            return result;
          },
          { critical: 0, warning: 0, suggestion: 0 },
        )
      : null;
  return (
    <aside className="review-history-detail">
      <h3>{copy.history.runDetail}</h3>
      <dl>
        <div>
          <dt>{copy.history.version}</dt>
          <dd>
            {real
              ? `V${item.sourceContentVersionSummary.versionNo}`
              : copy.history.mock}
          </dd>
        </div>
        <div>
          <dt>{copy.history.status}</dt>
          <dd>
            {run?.status === "running"
              ? copy.labels.states.running
              : run?.status === "queued"
                ? copy.labels.states.queued
                : copy.history.ended}
          </dd>
        </div>
        <div>
          <dt>{copy.history.time}</dt>
          <dd>
            {formatReviewTime(run?.createdAt ?? (!real ? item.created_at : null))}
          </dd>
        </div>
      </dl>
      {loading ? (
        <p>{copy.common.loading}</p>
      ) : counts ? (
        <section className="review-history-counts">
          <span><b>{counts.critical}</b>{copy.report.critical}</span>
          <span><b>{counts.warning}</b>{copy.report.warning}</span>
          <span><b>{counts.suggestion}</b>{copy.report.suggestion}</span>
        </section>
      ) : (
        <p>{real && !item.reportSummary ? copy.history.noReport : copy.report.legacy}</p>
      )}
      {real && item.reportSummary && (
        <Link
          href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(item.reportSummary.id)}`}
        >
          {copy.history.viewReport}
        </Link>
      )}
      {run && <Link href={`/workflow-runs/${run.id}`}>{copy.common.runDetail}</Link>}
    </aside>
  );
}

const historyKey = (item: ReviewHistoryEntry) =>
  isRealReviewHistoryItem(item) ? item.workflowRun.id : item.id;

function HistoryState({
  title,
  description,
  loading,
  retry,
}: {
  title: string;
  description?: string;
  loading?: boolean;
  retry?: () => void;
}) {
  return (
    <section className="review-history-state" aria-busy={loading}>
      <Icon name={loading ? "timeline" : "timeline"} size={30} />
      <h3>{title}</h3>
      {description && <p>{description}</p>}
      {retry && <button onClick={retry}>{copy.common.retry}</button>}
    </section>
  );
}

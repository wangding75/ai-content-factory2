"use client";
/* eslint-disable react-hooks/set-state-in-effect */

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Icon } from "@/components/ui/icons";
import {
  getReview,
  isRealReviewDetail,
  isRealReviewHistoryItem,
  listContentReviewHistory,
  type ReviewDetail,
  type ReviewHistoryEntry,
  type ReviewHistoryPage,
  type ReviewState,
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
  const [statusFilter, setStatusFilter] = useState<ReviewState | "all">("all");
  const [versionFilter, setVersionFilter] = useState("all");
  const [searchQuery, setSearchQuery] = useState("");

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
          setSelected(sortHistoryItems(next.items)[0] ?? null);
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

  const visibleItems = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return sortHistoryItems(page?.items ?? []).filter((item) => {
      if (statusFilter !== "all" && (!isRealReviewHistoryItem(item) || item.state !== statusFilter))
        return false;
      if (versionFilter !== "all" && (!isRealReviewHistoryItem(item) || String(item.sourceContentVersionSummary.versionNo) !== versionFilter))
        return false;
      if (!query) return true;
      if (!isRealReviewHistoryItem(item)) return item.id.toLowerCase().includes(query);
      return [item.workflowRun.runNumber, item.reportSummary?.id ?? ""]
        .join(" ")
        .toLowerCase()
        .includes(query);
    });
  }, [page, searchQuery, statusFilter, versionFilter]);

  const filterOptions = useMemo(
    () =>
      Array.from(
        new Set(
          (page?.items ?? [])
            .filter(isRealReviewHistoryItem)
            .map((item) => String(item.sourceContentVersionSummary.versionNo)),
        ),
      ).sort((left, right) => Number(right) - Number(left)),
    [page],
  );

  const summaryCounts = useMemo(() => {
    const items = page?.items ?? [];
    return {
      success: items.filter((item) =>
        isRealReviewHistoryItem(item) ? item.state === "review_ready" : true,
      ).length,
      failure: items.filter(
        (item) =>
          isRealReviewHistoryItem(item) &&
          ["runtime_failed", "output_validation_failed", "result_consumption_failed"].includes(item.state),
      ).length,
      running: items.filter(
        (item) =>
          isRealReviewHistoryItem(item) &&
          (item.state === "queued" || item.state === "running"),
      ).length,
    };
  }, [page]);

  const resetFilters = () => {
    setStatusFilter("all");
    setVersionFilter("all");
    setSearchQuery("");
  };

  useEffect(() => {
    if (!visibleItems.length) {
      setSelected(null);
      return;
    }
    if (!selected || !visibleItems.some((item) => historyKey(item) === historyKey(selected)))
      setSelected(visibleItems[0]);
  }, [selected, visibleItems]);

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
      {page?.items.length ? (
        <section className="review-history-filters" aria-label={copy.history.filters}>
          <div className="review-history-filter-object">
            <span>{copy.history.object}</span>
            <strong>
              {page.items.find(isRealReviewHistoryItem)?.sourceContentVersionSummary.title ??
                copy.common.noValue}
            </strong>
          </div>
          <label>
            <span>{copy.history.status}</span>
            <select
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value as ReviewState | "all")}
            >
              <option value="all">{copy.history.allStatus}</option>
              <option value="queued">{reviewStateLabel("queued")}</option>
              <option value="running">{reviewStateLabel("running")}</option>
              <option value="review_ready">{reviewStateLabel("review_ready")}</option>
              <option value="runtime_failed">{reviewStateLabel("runtime_failed")}</option>
              <option value="output_validation_failed">{reviewStateLabel("output_validation_failed")}</option>
              <option value="result_consumption_failed">{reviewStateLabel("result_consumption_failed")}</option>
            </select>
          </label>
          <label>
            <span>{copy.history.version}</span>
            <select value={versionFilter} onChange={(event) => setVersionFilter(event.target.value)}>
              <option value="all">{copy.history.allVersions}</option>
              {filterOptions.map((version) => (
                <option key={version} value={version}>V{version}</option>
              ))}
            </select>
          </label>
          <label className="review-history-filter-search">
            <span>{copy.history.searchPlaceholder}</span>
            <input
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder={copy.history.searchPlaceholder}
            />
          </label>
          <button type="button" onClick={resetFilters}>
            {copy.history.resetFilters}
          </button>
        </section>
      ) : null}
      <section className="review-history-summary">
        <span>
          {copy.history.allRuns} <b>{page?.total ?? 0}</b>
        </span>
        <span>
          {copy.history.success} <b>{summaryCounts.success}</b>
        </span>
        <span>
          {copy.history.failure} <b>{summaryCounts.failure}</b>
        </span>
        <span>
          {copy.history.runningCount} <b>{summaryCounts.running}</b>
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
      ) : !visibleItems.length ? (
        <HistoryState title={copy.history.noMatches} retry={resetFilters} retryLabel={copy.history.resetFilters} />
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
                  <th>{copy.history.runNumber}</th>
                  <th>{copy.history.action}</th>
                </tr>
              </thead>
              <tbody>
                {visibleItems.map((item) => (
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
        <td>—</td>
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
  const state = item.state;
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
      <td><code>{run.runNumber}</code></td>
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
          <dt>{copy.history.runNumber}</dt>
          <dd>{real ? run?.runNumber ?? copy.common.noValue : copy.common.noValue}</dd>
        </div>
        <div>
          <dt>{copy.history.reportId}</dt>
          <dd>{real ? item.reportSummary?.id ?? copy.common.noValue : copy.common.noValue}</dd>
        </div>
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
            {real ? reviewStateLabel(item.state) : copy.history.ended}
          </dd>
        </div>
        <div>
          <dt>{copy.history.time}</dt>
          <dd>
            {formatReviewTime(run?.createdAt ?? (!real ? item.created_at : null))}
          </dd>
        </div>
        <div>
          <dt>{copy.report.completedAt}</dt>
          <dd>{real ? formatReviewTime(item.reportSummary?.completedAt) : copy.common.noValue}</dd>
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
      {real && item.latestError && <p>{item.latestError.message}</p>}
      {run && <Link href={`/workflow-runs/${run.id}`}>{copy.common.runDetail}</Link>}
    </aside>
  );
}

const sortHistoryItems = (items: ReviewHistoryEntry[]) =>
  [...items].sort(
    (left, right) =>
      historyTimestamp(right).getTime() - historyTimestamp(left).getTime(),
  );

const historyTimestamp = (item: ReviewHistoryEntry) =>
  new Date(
    isRealReviewHistoryItem(item)
      ? item.workflowRun.createdAt
      : item.created_at,
  );

const historyKey = (item: ReviewHistoryEntry) =>
  isRealReviewHistoryItem(item) ? item.workflowRun.id : item.id;

function HistoryState({
  title,
  description,
  loading,
  retry,
  retryLabel,
}: {
  title: string;
  description?: string;
  loading?: boolean;
  retry?: () => void;
  retryLabel?: string;
}) {
  return (
    <section className="review-history-state" aria-busy={loading}>
      <Icon name={loading ? "timeline" : "timeline"} size={30} />
      <h3>{title}</h3>
      {description && <p>{description}</p>}
      {retry && <button onClick={retry}>{retryLabel ?? copy.common.retry}</button>}
    </section>
  );
}

"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import {
  listChapterPlanCandidateBatches,
  type ChapterPlanCandidateBatch,
  type ChapterPlanCandidateBatchStatus,
  type ChapterPlanningGenerationMode,
  type ListCandidateBatchesQuery,
} from "./chapter-plan-http-api";
import {
  candidateBatchModeLabel,
  candidateBatchStatusLabel,
  chapterPlanningErrorMessage,
} from "./chapter-plan-presentation";
import { chapterPlanCacheEvent } from "./chapter-plan-cache";


export function CandidateBatchListPage({ projectId }: { projectId: string }) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  // Read initial query params from URL
  const statusParam = (searchParams.get("status") as ChapterPlanCandidateBatchStatus) || "";
  const modeParam = (searchParams.get("generationMode") as ChapterPlanningGenerationMode) || "";
  const sourceRunIdParam = searchParams.get("sourceWorkflowRunId") || "";
  const fromParam = searchParams.get("createdAtFrom") || "";
  const toParam = searchParams.get("createdAtTo") || "";
  const limitParam = Number(searchParams.get("limit")) || 20;
  const offsetParam = Number(searchParams.get("offset")) || 0;

  const [batches, setBatches] = useState<ChapterPlanCandidateBatch[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const hasFilters = Boolean(statusParam || modeParam || sourceRunIdParam || fromParam || toParam);

  const syncUrl = (newQuery: Record<string, string | number | undefined>) => {
    const params = new URLSearchParams(searchParams.toString());
    Object.entries(newQuery).forEach(([key, value]) => {
      if (value !== undefined && value !== "") {
        params.set(key, String(value));
      } else {
        params.delete(key);
      }
    });
    router.replace(`${pathname}?${params.toString()}`);
  };

  const loadBatches = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true);
      setError(null);

      const query: ListCandidateBatchesQuery = {
        status: statusParam || undefined,
        generationMode: modeParam || undefined,
        sourceWorkflowRunId: sourceRunIdParam || undefined,
        createdAtFrom: fromParam || undefined,
        createdAtTo: toParam || undefined,
        limit: limitParam,
        offset: offsetParam,
      };

      try {
        const envelope = await listChapterPlanCandidateBatches(projectId, query, { signal });
        setBatches(envelope.items);
        setTotal(envelope.total);
      } catch (cause) {
        if (!signal?.aborted) {
          setError(
            cause instanceof ApiError
              ? cause
              : new ApiError("加载候选批次列表失败", 500),
          );
        }
      } finally {
        if (!signal?.aborted) {
          setLoading(false);
        }
      }
    },
    [projectId, statusParam, modeParam, sourceRunIdParam, fromParam, toParam, limitParam, offsetParam],
  );

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    const query: ListCandidateBatchesQuery = {
      status: statusParam || undefined,
      generationMode: modeParam || undefined,
      sourceWorkflowRunId: sourceRunIdParam || undefined,
      createdAtFrom: fromParam || undefined,
      createdAtTo: toParam || undefined,
      limit: limitParam,
      offset: offsetParam,
    };

    listChapterPlanCandidateBatches(projectId, query, { signal: controller.signal })
      .then((envelope) => {
        if (!cancelled) {
          setBatches(envelope.items);
          setTotal(envelope.total);
          setError(null);
          setLoading(false);
        }
      })
      .catch((cause) => {
        if (!cancelled && !controller.signal.aborted) {
          setError(
            cause instanceof ApiError
              ? cause
              : new ApiError("加载候选批次列表失败", 500),
          );
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [projectId, statusParam, modeParam, sourceRunIdParam, fromParam, toParam, limitParam, offsetParam]);

  useEffect(() => {
    const refreshAfterMutation = (event: Event) => {
      if ((event as CustomEvent<{ projectId?: string }>).detail?.projectId === projectId) {
        void loadBatches();
      }
    };
    window.addEventListener(chapterPlanCacheEvent, refreshAfterMutation);
    return () => window.removeEventListener(chapterPlanCacheEvent, refreshAfterMutation);
  }, [loadBatches, projectId]);

  return (
    <div className="chapter-plan-batch-list-page">
      <header className="chapter-plans-heading">
        <div>
          <h2>候选批次列表</h2>
          <p>按批次查看与对比AI生成的章节规划候选组合。</p>
        </div>
        <div className="chapter-plans-actions">
          <Link
            className="chapter-plan-button secondary"
            href={`/projects/${projectId}/chapter-plans`}
          >
            ← 返回章节工作区
          </Link>
        </div>
      </header>

      {/* 7 项筛选器 */}
      <section className="chapter-plans-toolbar" aria-label="候选批次筛选">
        <select
          aria-label="批次状态筛选"
          value={statusParam}
          onChange={(e) => syncUrl({ status: e.target.value, offset: 0 })}
        >
          <option value="">全部状态</option>
          <option value="ready">待处理</option>
          <option value="partially_adopted">部分采用</option>
          <option value="adopted">全量采用</option>
          <option value="abandoned">已放弃</option>
        </select>

        <select
          aria-label="生成模式筛选"
          value={modeParam}
          onChange={(e) => syncUrl({ generationMode: e.target.value, offset: 0 })}
        >
          <option value="">全部模式</option>
          <option value="range">局部范围</option>
          <option value="full">完整大纲</option>
          <option value="append">追加后续</option>
        </select>

        <select
          aria-label="来源任务筛选"
          value={sourceRunIdParam}
          onChange={(e) => syncUrl({ sourceWorkflowRunId: e.target.value, offset: 0 })}
        >
          <option value="">全部来源任务</option>
          {batches.map((batch, index) => (
            <option key={batch.sourceWorkflowRunId} value={batch.sourceWorkflowRunId}>
              来源任务 {offsetParam + index + 1} · {new Date(batch.createdAt).toLocaleDateString("zh-CN")}
            </option>
          ))}
        </select>

        <input
          type="date"
          aria-label="创建起始时间"
          value={fromParam}
          onChange={(e) => syncUrl({ createdAtFrom: e.target.value, offset: 0 })}
        />

        <input
          type="date"
          aria-label="创建截止时间"
          value={toParam}
          onChange={(e) => syncUrl({ createdAtTo: e.target.value, offset: 0 })}
        />

        <button
          type="button"
          onClick={() =>
            syncUrl({
              status: "",
              generationMode: "",
              sourceWorkflowRunId: "",
              createdAtFrom: "",
              createdAtTo: "",
              offset: 0,
            })
          }
        >
          重置筛选
        </button>
      </section>

      {error && (
        <div className="chapter-plans-form-error chapter-plan-batch-error" role="alert">
          <strong>候选批次加载失败</strong>
          <span>{chapterPlanningErrorMessage(error, "候选批次暂时无法加载，请稍后重试。")}</span>
          <button type="button" onClick={() => void loadBatches()} style={{ marginLeft: 12 }}>
            重试
          </button>
        </div>
      )}

      {loading ? (
        <div className="chapter-plans-skeleton card" />
      ) : batches.length === 0 ? (
        <section className="chapter-plans-empty chapter-plan-batch-empty" role="status">
          <Icon name="archive" size={36} />
          <h3>{hasFilters ? "暂无匹配候选批次" : "暂无候选批次"}</h3>
          <p>{hasFilters ? "当前筛选条件没有对应批次，请调整筛选后重试。" : "完成章节规划预检并创建任务后，候选批次会显示在这里。"}</p>
        </section>
      ) : (
        <div className="chapter-plans-table" aria-live="polite">
          <div className="chapter-plan-row header">
            <span>批次 / 来源</span>
            <span>生成模式</span>
            <span>目标范围</span>
            <span>状态</span>
            <span>候选汇总</span>
            <span>创建时间</span>
            <span>操作</span>
          </div>

          {batches.map((batch, index) => (
            <article key={batch.id} className="chapter-plan-row">
              <div className="chapter-plan-batch-identity">
                <strong>批次 {offsetParam + index + 1}</strong>
                <small>Run ID：{batch.sourceWorkflowRunId}</small>
              </div>
              <span>{candidateBatchModeLabel(batch.generationMode)}</span>
              <span>
                {batch.generationMode === "range"
                  ? `第 ${batch.target.startChapterNo}–${batch.target.endChapterNo} 章`
                  : `${batch.target.requestedChapterCount} 个章节`}
              </span>
              <span className={`chapter-plan-status ${batch.status}`}>
                {candidateBatchStatusLabel(batch.status)}
              </span>
              <div className="batch-candidate-counts">
                <span title="总候选数">共 {batch.candidateCount} 项</span>
                {batch.pendingCount > 0 && <i>待处理: {batch.pendingCount}</i>}
                {batch.staleCount > 0 && <i className="warning">过期: {batch.staleCount}</i>}
                {batch.adoptedCount > 0 && <i className="success">已采用: {batch.adoptedCount}</i>}
                {batch.discardedCount > 0 && <i className="muted">已丢弃: {batch.discardedCount}</i>}
              </div>
              <div className="chapter-plan-batch-created">
                <span>创建时间</span>
                <small>{new Date(batch.createdAt).toLocaleString("zh-CN")}</small>
              </div>
              <div>
                <Link
                  className="chapter-plan-edit-button"
                  href={`/projects/${batch.projectId}/chapter-plan-candidate-batches/${batch.id}`}
                >
                  查看详情
                </Link>
              </div>
            </article>
          ))}
        </div>
      )}

      {/* Pagination */}
      {total > limitParam && (
        <footer className="chapter-plan-pagination">
          <button
            type="button"
            disabled={offsetParam === 0}
            onClick={() => syncUrl({ offset: Math.max(0, offsetParam - limitParam) })}
          >
            上一页
          </button>
          <span>
            {Math.floor(offsetParam / limitParam) + 1} / {Math.ceil(total / limitParam)}
          </span>
          <button
            type="button"
            disabled={offsetParam + limitParam >= total}
            onClick={() => syncUrl({ offset: offsetParam + limitParam })}
          >
            下一页
          </button>
        </footer>
      )}
    </div>
  );
}

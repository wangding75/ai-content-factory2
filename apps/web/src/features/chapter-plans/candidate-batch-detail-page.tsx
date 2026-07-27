"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import {
  getChapterPlanCandidateBatch,
  listChapterPlanCandidates,
  type ChapterPlanCandidate,
  type ChapterPlanCandidateBatch,
  type ChapterPlanCandidateDiffType,
  type ChapterPlanCandidateStatus,
  type ListCandidatesQuery,
} from "./chapter-plan-http-api";
import {
  candidateBatchModeLabel,
  candidateBatchStatusLabel,
  candidateDiffTypeLabel,
  candidateStatusLabel,
} from "./chapter-plan-presentation";
import { CandidateEditDrawer } from "./candidate-edit-drawer";
import { CandidateCompareDialog } from "./candidate-compare-dialog";



export function CandidateBatchDetailPage({ batchId }: { batchId: string }) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  // Read initial query parameters from URL
  const statusParam = (searchParams.get("status") as ChapterPlanCandidateStatus) || "";
  const diffTypeParam = (searchParams.get("diffType") as ChapterPlanCandidateDiffType) || "";
  const storylineIdParam = searchParams.get("storylineId") || "";
  const searchParam = searchParams.get("q") || "";
  const limitParam = Number(searchParams.get("limit")) || 20;
  const offsetParam = Number(searchParams.get("offset")) || 0;

  const [batch, setBatch] = useState<ChapterPlanCandidateBatch | null>(null);
  const [candidates, setCandidates] = useState<ChapterPlanCandidate[]>([]);
  const [total, setTotal] = useState(0);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);

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

  const loadData = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true);
      setError(null);

      const query: ListCandidatesQuery = {
        status: statusParam || undefined,
        diffType: diffTypeParam || undefined,
        storylineId: storylineIdParam || undefined,
        q: searchParam || undefined,
        limit: limitParam,
        offset: offsetParam,
      };

      try {
        const [batchEnvelope, candidatesEnvelope] = await Promise.all([
          getChapterPlanCandidateBatch(batchId, { signal }),
          listChapterPlanCandidates(batchId, query, { signal }),
        ]);

        setBatch(batchEnvelope.data);
        setCandidates(candidatesEnvelope.data.items);
        setTotal(candidatesEnvelope.data.total);
      } catch (cause) {
        if (!signal?.aborted) {
          setError(
            cause instanceof ApiError
              ? cause
              : new ApiError("加载候选批次详情失败", 500),
          );
        }
      } finally {
        if (!signal?.aborted) {
          setLoading(false);
        }
      }
    },
    [batchId, statusParam, diffTypeParam, storylineIdParam, searchParam, limitParam, offsetParam],
  );

  useEffect(() => {
    const controller = new AbortController();
    void loadData(controller.signal);
    return () => controller.abort();
  }, [loadData]);

  if (loading && !batch) {
    return <div className="chapter-plans-skeleton card" />;
  }

  if (error && !batch) {
    return (
      <div className="chapter-plans-state">
        <Icon name="info" size={34} />
        <h1>批次详情加载失败</h1>
        <p>{error.message}</p>
        <button type="button" onClick={() => void loadData()}>
          重试
        </button>
      </div>
    );
  }

  const [editingCandidate, setEditingCandidate] = useState<ChapterPlanCandidate | null>(null);
  const [comparingCandidateId, setComparingCandidateId] = useState<string | null>(null);

  return (
    <div className="chapter-plan-batch-detail-page">
      <header className="chapter-plans-heading">
        <div>
          <h2>候选批次详情</h2>
          <p>
            批次 ID: {batch?.id} (模式:{" "}
            {batch ? candidateBatchModeLabel(batch.generationMode) : "—"})
          </p>
        </div>
        <div className="chapter-plans-actions">
          {batch?.projectId ? (
            <Link
              className="chapter-plan-button secondary"
              href={`/projects/${batch.projectId}/chapter-plan-candidate-batches`}
            >
              ← 返回批次列表
            </Link>
          ) : (
            <button
              type="button"
              className="chapter-plan-button secondary"
              onClick={() => router.back()}
            >
              ← 返回
            </button>
          )}
        </div>
      </header>

      {/* Batch Header Stats Card */}
      {batch && (
        <section className="chapter-plan-stats" aria-label="批次统计与状态">
          <article>
            <span>批次状态</span>
            <b className={`chapter-plan-status ${batch.status}`}>
              {candidateBatchStatusLabel(batch.status)}
            </b>
          </article>
          <article>
            <span>总候选数</span>
            <b>{batch.candidateCount}</b>
          </article>
          <article>
            <span>待处理</span>
            <b>{batch.pendingCount}</b>
          </article>
          <article>
            <span>已过期</span>
            <b className={batch.staleCount > 0 ? "warning" : ""}>{batch.staleCount}</b>
          </article>
          <article>
            <span>已采用</span>
            <b className="success">{batch.adoptedCount}</b>
          </article>
          <article>
            <span>已丢弃</span>
            <b className="muted">{batch.discardedCount}</b>
          </article>
        </section>
      )}

      {/* 6 项 Candidate 筛选与搜索 */}
      <section className="chapter-plans-toolbar" aria-label="候选筛选与搜索">
        <input
          aria-label="搜索候选标题或摘要"
          placeholder="搜索候选标题、摘要或目的..."
          value={searchParam}
          onChange={(e) => syncUrl({ q: e.target.value, offset: 0 })}
        />

        <select
          aria-label="候选状态筛选"
          value={statusParam}
          onChange={(e) => syncUrl({ status: e.target.value, offset: 0 })}
        >
          <option value="">全部候选状态</option>
          <option value="pending">待处理</option>
          <option value="stale">已过期</option>
          <option value="adopted">已采用</option>
          <option value="discarded">已丢弃</option>
        </select>

        <select
          aria-label="差异类型筛选"
          value={diffTypeParam}
          onChange={(e) => syncUrl({ diffType: e.target.value, offset: 0 })}
        >
          <option value="">全部差异类型</option>
          <option value="new">新设章节</option>
          <option value="replace">替换候选</option>
          <option value="no_change">无变化</option>
          <option value="stale_conflict">基线冲突</option>
        </select>

        <input
          aria-label="故事线标识筛选"
          placeholder="故事线 ID"
          value={storylineIdParam}
          onChange={(e) => syncUrl({ storylineId: e.target.value, offset: 0 })}
        />

        <button
          type="button"
          onClick={() =>
            syncUrl({
              q: "",
              status: "",
              diffType: "",
              storylineId: "",
              offset: 0,
            })
          }
        >
          清除筛选
        </button>
      </section>

      {error && (
        <div className="chapter-plans-form-error" role="alert">
          {error.message}
        </div>
      )}

      {/* Candidates List Table */}
      {!candidates.length ? (
        <section className="chapter-plans-empty">
          <Icon name="book" size={36} />
          <h3>暂无匹配候选</h3>
          <p>请调整筛选条件。</p>
        </section>
      ) : (
        <div className="chapter-plans-table" aria-live="polite">
          <div className="chapter-plan-row header">
            <span>章节</span>
            <span>候选标题与摘要</span>
            <span>差异类型</span>
            <span>状态</span>
            <span>版本</span>
            <span>关联故事线</span>
            <span>操作</span>
          </div>

          {candidates.map((cand) => {
            const snap = cand.currentSnapshot;
            const storylineNames = snap.storylineRefs.map((r) => r.label).join("、") || "—";

            return (
              <article key={cand.id} className="chapter-plan-row">
                <b>第 {cand.chapterNo} 章</b>
                <div>
                  <strong>{snap.title}</strong>
                  <p className="candidate-summary-text">{snap.summary}</p>
                  <small>章节目的: {snap.chapterPurpose || "未描述"}</small>
                </div>
                <span className={`diff-tag ${cand.diffType}`}>
                  {candidateDiffTypeLabel(cand.diffType)}
                </span>
                <span className={`chapter-plan-status ${cand.status}`}>
                  {candidateStatusLabel(cand.status)}
                </span>
                <span>v{cand.version}</span>
                <span>{storylineNames}</span>
                <div className="candidate-action-buttons">
                  <button
                    type="button"
                    className="chapter-plan-edit-button"
                    onClick={() => setEditingCandidate(cand)}
                  >
                    编辑候选
                  </button>
                  <button
                    type="button"
                    className="chapter-plan-edit-button"
                    onClick={() => setComparingCandidateId(cand.id)}
                  >
                    差异对比
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      )}

      {editingCandidate && (
        <CandidateEditDrawer
          candidate={editingCandidate}
          onClose={() => setEditingCandidate(null)}
          onSaved={async () => {
            setEditingCandidate(null);
            await loadData();
          }}
          onRefreshCandidate={() => void loadData()}
        />
      )}

      {comparingCandidateId && (
        <CandidateCompareDialog
          candidateId={comparingCandidateId}
          onClose={() => setComparingCandidateId(null)}
        />
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

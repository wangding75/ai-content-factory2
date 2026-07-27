"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import {
  adoptChapterPlanCandidate,
  discardChapterPlanCandidate,
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
import {
  BatchAbandonDialog,
  BatchAdoptDialog,
  StaleConflictDialog,
} from "./candidate-action-dialogs";

import { useIdempotency } from "./use-idempotency";
import { invalidateChapterPlanViews } from "./chapter-plan-cache";

export function CandidateBatchDetailPage({ batchId }: { batchId: string }) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const { getOrCreateKey, clearKey, markUnknown } = useIdempotency();

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
  const [selected, setSelected] = useState<Record<string, ChapterPlanCandidate>>({});

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const [actionNotice, setActionNotice] = useState<string | null>(null);

  // Modals & Drawers
  const [editingCandidate, setEditingCandidate] = useState<ChapterPlanCandidate | null>(null);
  const [comparingCandidateId, setComparingCandidateId] = useState<string | null>(null);
  const [staleCandidate, setStaleCandidate] = useState<ChapterPlanCandidate | null>(null);
  const [batchAdoptOpen, setBatchAdoptOpen] = useState(false);
  const [batchAbandonOpen, setBatchAbandonOpen] = useState(false);

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
    let cancelled = false;
    const controller = new AbortController();

    const query: ListCandidatesQuery = {
      status: statusParam || undefined,
      diffType: diffTypeParam || undefined,
      storylineId: storylineIdParam || undefined,
      q: searchParam || undefined,
      limit: limitParam,
      offset: offsetParam,
    };

    Promise.all([
      getChapterPlanCandidateBatch(batchId, { signal: controller.signal }),
      listChapterPlanCandidates(batchId, query, { signal: controller.signal }),
    ])
      .then(([batchEnvelope, candidatesEnvelope]) => {
        if (!cancelled) {
          setBatch(batchEnvelope.data);
          setCandidates(candidatesEnvelope.data.items);
          setTotal(candidatesEnvelope.data.total);
          setError(null);
          setLoading(false);
        }
      })
      .catch((cause) => {
        if (!cancelled && !controller.signal.aborted) {
          setError(
            cause instanceof ApiError
              ? cause
              : new ApiError("加载候选批次详情失败", 500),
          );
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [batchId, statusParam, diffTypeParam, storylineIdParam, searchParam, limitParam, offsetParam]);

  const toggleSelect = (cand: ChapterPlanCandidate) => {
    setSelected((prev) => {
      const next = { ...prev };
      if (next[cand.id]) {
        delete next[cand.id];
      } else {
        next[cand.id] = cand;
      }
      return next;
    });
  };

  // Single Candidate Adopt
  const handleAdoptCandidate = async (cand: ChapterPlanCandidate) => {
    setError(null);
    setActionNotice(null);
    const scope = `candidate:adopt:${cand.id}`;
    const payload = {
      expectedCandidateVersion: cand.version,
      expectedChapterPlanVersion: cand.baseChapterPlanVersion ?? null,
    };
    try {
      const idempotencyKey = getOrCreateKey(scope, payload);
      const envelope = await adoptChapterPlanCandidate(cand.id, payload, idempotencyKey);

      clearKey(scope);
      invalidateChapterPlanViews(cand.projectId);

      if (envelope.data.outcome === "no_change") {
        setActionNotice(`第 ${cand.chapterNo} 章候选与线上内容一致 (no_change)，未产生新 Revision。`);
      } else {
        setActionNotice(`第 ${cand.chapterNo} 章候选采用成功 (Revision r${envelope.data.revision.revisionNo})。`);
      }

      await loadData();
    } catch (cause) {
      if (
        cause instanceof ApiError &&
        (cause.status === 0 || cause.status >= 500 || cause.code === "timeout")
      ) {
        markUnknown(scope, payload);
      }
      if (cause instanceof ApiError) {
        if (
          cause.status === 409 ||
          cause.message?.includes("stale") ||
          cause.message?.includes("conflict")
        ) {
          setStaleCandidate(cand);
        } else {
          setError(cause);
        }
      } else {
        setError(new ApiError("采用候选失败，请重试。", 500));
      }
    }
  };

  // Single Candidate Discard
  const handleDiscardCandidate = async (cand: ChapterPlanCandidate) => {
    setError(null);
    setActionNotice(null);
    const scope = `candidate:discard:${cand.id}`;
    const payload = { expectedCandidateVersion: cand.version };
    try {
      const idempotencyKey = getOrCreateKey(scope, payload);
      await discardChapterPlanCandidate(cand.id, payload, idempotencyKey);

      clearKey(scope);
      invalidateChapterPlanViews(cand.projectId);

      setActionNotice(`第 ${cand.chapterNo} 章候选已丢弃。`);
      await loadData();
    } catch (cause) {
      if (
        cause instanceof ApiError &&
        (cause.status === 0 || cause.status >= 500 || cause.code === "timeout")
      ) {
        markUnknown(scope, payload);
      }
      if (cause instanceof ApiError) {
        setError(cause);
      } else {
        setError(new ApiError("丢弃候选失败，请重试。", 500));
      }
    }
  };


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

  const isBatchFinalized = batch?.status === "abandoned" || batch?.status === "adopted";
  const selectedList = Object.values(selected).filter(
    (cand) => cand.status === "pending" || cand.status === "stale",
  );

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
          {batch && !isBatchFinalized && (
            <>
              <button
                type="button"
                className="chapter-plan-button primary"
                onClick={() => setBatchAdoptOpen(true)}
                disabled={selectedList.length === 0}
              >
                批量采用已选候选 ({selectedList.length})
              </button>
              <button
                type="button"
                className="chapter-plan-button secondary"
                onClick={() => setBatchAbandonOpen(true)}
              >
                放弃本批次
              </button>
            </>
          )}

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

      {/* Action Feedback Notice */}
      {actionNotice && (
        <div className="chapter-plan-status-banner success" style={{ marginBottom: 16 }}>
          <Icon name="sparkles" size={18} />
          <div>{actionNotice}</div>
        </div>
      )}

      {error && (
        <div className="chapter-plans-form-error" role="alert" style={{ marginBottom: 16 }}>
          {error.message}
        </div>
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
            <span>选择</span>
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
            const isSelectable =
              !isBatchFinalized && (cand.status === "pending" || cand.status === "stale");
            const isFinalized = cand.status === "adopted" || cand.status === "discarded";

            return (
              <article key={cand.id} className="chapter-plan-row">
                <span>
                  <input
                    type="checkbox"
                    aria-label={`选择第 ${cand.chapterNo} 章候选`}
                    checked={Boolean(selected[cand.id])}
                    onChange={() => toggleSelect(cand)}
                    disabled={!isSelectable}
                  />
                </span>
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
                  {!isFinalized && !isBatchFinalized && (
                    <>
                      <button
                        type="button"
                        className="chapter-plan-edit-button primary"
                        onClick={() => void handleAdoptCandidate(cand)}
                      >
                        采用
                      </button>
                      <button
                        type="button"
                        className="chapter-plan-edit-button secondary"
                        onClick={() => void handleDiscardCandidate(cand)}
                      >
                        丢弃
                      </button>
                    </>
                  )}
                  <button
                    type="button"
                    className="chapter-plan-edit-button"
                    onClick={() => setEditingCandidate(cand)}
                    disabled={isFinalized}
                  >
                    编辑
                  </button>
                  <button
                    type="button"
                    className="chapter-plan-edit-button"
                    onClick={() => setComparingCandidateId(cand.id)}
                  >
                    对比
                  </button>
                </div>
              </article>
            );
          })}
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

      {/* Modals & Drawers */}
      {editingCandidate && (
        <CandidateEditDrawer
          candidate={editingCandidate}
          onClose={() => setEditingCandidate(null)}
          onSaved={async () => {
            setEditingCandidate(null);
            invalidateChapterPlanViews(editingCandidate.projectId);
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

      {batchAdoptOpen && batch && (
        <BatchAdoptDialog
          batch={batch}
          selectedCandidates={selectedList}
          onClose={() => setBatchAdoptOpen(false)}
          onCompleted={() => {
            setSelected({});
            invalidateChapterPlanViews(batch.projectId);
            void loadData();
          }}
        />
      )}

      {batchAbandonOpen && batch && (
        <BatchAbandonDialog
          batch={batch}
          onClose={() => setBatchAbandonOpen(false)}
          onAbandoned={() => {
            invalidateChapterPlanViews(batch.projectId);
            void loadData();
          }}
        />
      )}

      {staleCandidate && (
        <StaleConflictDialog
          candidate={staleCandidate}
          onClose={() => setStaleCandidate(null)}
          onRecompare={() => {
            const candId = staleCandidate.id;
            setStaleCandidate(null);
            setComparingCandidateId(candId);
          }}
          onRefresh={() => {
            setStaleCandidate(null);
            void loadData();
          }}
        />
      )}
    </div>
  );
}

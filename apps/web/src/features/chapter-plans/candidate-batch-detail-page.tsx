"use client";

import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
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
  candidatePurposeLabel,
  candidateStatusLabel,
  chapterPlanningErrorMessage,
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

        setBatch(batchEnvelope);
        setCandidates(candidatesEnvelope.items);
        setTotal(candidatesEnvelope.total);
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
          setBatch(batchEnvelope);
          setCandidates(candidatesEnvelope.items);
          setTotal(candidatesEnvelope.total);
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
      const idempotencyKey = await getOrCreateKey(scope, payload);
      const envelope = await adoptChapterPlanCandidate(cand.id, payload, idempotencyKey);

      clearKey(scope);
      invalidateChapterPlanViews(cand.projectId);

      if (envelope.outcome === "no_change") {
        setActionNotice(`第 ${cand.chapterNo} 章候选与线上内容一致，未产生新的修订记录。`);
      } else {
        setActionNotice(`第 ${cand.chapterNo} 章候选采用成功，已生成第 ${envelope.revision.revisionNo} 版修订记录。`);
      }

      await loadData();
    } catch (cause) {
      if (
        cause instanceof ApiError &&
        (cause.status === 0 || cause.status >= 500 || cause.code === "timeout")
      ) {
        await markUnknown(scope, payload);
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
      const idempotencyKey = await getOrCreateKey(scope, payload);
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
        await markUnknown(scope, payload);
      }
      if (cause instanceof ApiError) {
        setError(cause);
      } else {
        setError(new ApiError("丢弃候选失败，请重试。", 500));
      }
    }
  };


  const storylineOptions = useMemo(() => {
    const values = new Map<string, string>();
    for (const candidate of candidates) {
      for (const ref of candidate.currentSnapshot.storylineRefs) {
        values.set(ref.id, ref.label || "未命名故事线");
      }
    }
    return [...values.entries()];
  }, [candidates]);

  if (loading && !batch) {
    return <div className="chapter-plans-skeleton card" />;
  }

  if (error && !batch) {
    return (
      <div className="chapter-plans-state">
        <Icon name="info" size={34} />
        <h1>批次详情加载失败</h1>
        <p>{chapterPlanningErrorMessage(error, "批次详情暂时无法加载，请稍后重试。")}</p>
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
  const selectableCandidates = candidates.filter(
    (cand) => !isBatchFinalized && (cand.status === "pending" || cand.status === "stale"),
  );
  const allSelectableSelected =
    selectableCandidates.length > 0 &&
    selectableCandidates.every((candidate) => Boolean(selected[candidate.id]));

  const toggleAllSelectable = () => {
    setSelected((prev) => {
      const next = { ...prev };
      if (allSelectableSelected) {
        selectableCandidates.forEach((candidate) => delete next[candidate.id]);
      } else {
        selectableCandidates.forEach((candidate) => {
          next[candidate.id] = candidate;
        });
      }
      return next;
    });
  };

  return (
    <div className="chapter-plan-batch-detail-page">
      <header className="chapter-plans-heading">
        <div>
          <h2>候选批次详情</h2>
          <p>
            候选批次 / {batch ? batch.sourceWorkflowRunId : "—"} ·{" "}
            {batch
              ? `第 ${batch.target.startChapterNo}–${batch.target.endChapterNo} 章`
              : "正在加载目标范围"}
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
                title={selectedList.length === 0 ? "先选择可处理的候选章节" : undefined}
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
        <section className="chapter-plan-batch-detail-summary" aria-label="批次摘要">
          <div className="chapter-plan-batch-detail-summary-main">
            <div className="chapter-plan-batch-detail-summary-icon" aria-hidden="true">
              <Icon name="book" size={24} />
            </div>
            <div>
              <span className="chapter-plan-batch-detail-summary-kicker">候选批次</span>
              <h3>
                {candidateBatchModeLabel(batch.generationMode)} · 第 {batch.target.startChapterNo}—
                {batch.target.endChapterNo} 章
              </h3>
              <p>
                来源 Run ID：<code>{batch.sourceWorkflowRunId}</code> · 共 {batch.candidateCount} 项 ·
                创建于 {new Date(batch.createdAt).toLocaleString("zh-CN")}
              </p>
            </div>
          </div>
          <div className="chapter-plan-batch-detail-summary-status">
            <b className={`chapter-plan-status ${batch.status}`}>
              {candidateBatchStatusLabel(batch.status)}
            </b>
            <span>
              待处理 {batch.pendingCount} · 已采用 {batch.adoptedCount} · 已过期 {batch.staleCount} ·
              已丢弃 {batch.discardedCount}
            </span>
          </div>
          <p className="chapter-plan-batch-detail-summary-note">
            采用候选将生成版本化修订记录，不直接覆盖当前章节。
          </p>
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
          {chapterPlanningErrorMessage(error, "候选操作未完成，请刷新后重试。")}
        </div>
      )}

      <div className="chapter-plan-batch-detail-tabs" role="tablist" aria-label="候选状态快捷筛选">
        {[
          ["", `全部候选 ${batch?.candidateCount ?? total}`],
          ["pending", `待处理 ${batch?.pendingCount ?? 0}`],
          ["stale", `已过期 ${batch?.staleCount ?? 0}`],
          ["adopted", `已采用 ${batch?.adoptedCount ?? 0}`],
          ["discarded", `已丢弃 ${batch?.discardedCount ?? 0}`],
        ].map(([value, label]) => (
          <button
            key={value || "all"}
            type="button"
            role="tab"
            aria-selected={statusParam === value}
            className={statusParam === value ? "active" : ""}
            onClick={() => syncUrl({ status: value, offset: 0 })}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Candidate filters and search */}
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

        <select
          aria-label="故事线筛选"
          value={storylineIdParam}
          onChange={(e) => syncUrl({ storylineId: e.target.value, offset: 0 })}
        >
          <option value="">全部故事线</option>
          {storylineOptions.map(([id, label]) => (
            <option key={id} value={id}>{label}</option>
          ))}
        </select>

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
            <span>
              <input
                type="checkbox"
                aria-label="选择全部可处理候选"
                checked={allSelectableSelected}
                onChange={toggleAllSelectable}
                disabled={selectableCandidates.length === 0}
              />
            </span>
            <span>章节</span>
            <span>当前章节</span>
            <span>新候选</span>
            <span>关联故事线</span>
            <span>状态</span>
            <span>差异</span>
            <span>操作</span>
          </div>

          {candidates.map((cand) => {
            const snap = cand.currentSnapshot;
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
                <div className="candidate-current-chapter">
                  <strong className={cand.baseSnapshot ? "" : "muted"}>
                    {cand.baseSnapshot?.title || "—"}
                  </strong>
                  {cand.baseSnapshot && (
                    <small>当前版本 · 第 {cand.baseChapterPlanVersion ?? "—"} 版</small>
                  )}
                </div>
                <div className="candidate-generated-chapter">
                  <strong>{snap.title}</strong>
                  <p className="candidate-summary-text">{snap.summary}</p>
                  <small>章节目的：{candidatePurposeLabel(snap.chapterPurpose)}</small>
                </div>
                <div className="candidate-storyline-tags">
                  {snap.storylineRefs.length > 0 ? (
                    snap.storylineRefs.map((ref) => <span key={ref.id}>{ref.label}</span>)
                  ) : (
                    <span className="muted">—</span>
                  )}
                </div>
                <span className={`chapter-plan-status ${cand.status}`}>
                  {candidateStatusLabel(cand.status)}
                </span>
                <span className={`diff-tag ${cand.diffType}`}>
                  {candidateDiffTypeLabel(cand.diffType)}
                </span>
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

      {selectedList.length > 0 && (
        <div className="chapter-plan-batch-selection-bar" role="status">
          <div>
            <strong>{selectedList.length}</strong>
            <span>已选择 {selectedList.length} 个可处理候选</span>
          </div>
          <button type="button" onClick={() => setSelected({})}>
            清空选择
          </button>
          <button
            type="button"
            className="primary"
            onClick={() => setBatchAdoptOpen(true)}
            disabled={isBatchFinalized}
          >
            批量采用
          </button>
        </div>
      )}
    </div>
  );
}

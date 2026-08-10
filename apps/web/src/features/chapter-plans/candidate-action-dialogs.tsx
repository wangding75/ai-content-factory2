"use client";

import { useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import {
  abandonChapterPlanCandidateBatch,
  adoptChapterPlanCandidates,
  type BulkAdoptChapterPlanCandidateResult,
  type BulkAdoptChapterPlanCandidatesResult,
  type ChapterPlanCandidate,
  type ChapterPlanCandidateBatch,
} from "./chapter-plan-http-api";

import { useIdempotency } from "./use-idempotency";
import {
  candidateBatchModeLabel,
  chapterPlanningErrorMessage,
} from "./chapter-plan-presentation";

// --- P15_C8_BATCH_ADOPT_DIALOG ---

export interface BatchAdoptDialogProps {
  batch: ChapterPlanCandidateBatch;
  selectedCandidates: ChapterPlanCandidate[];
  onClose: () => void;
  onCompleted: (result: BulkAdoptChapterPlanCandidatesResult) => void;
}

export function BatchAdoptDialog({
  batch,
  selectedCandidates,
  onClose,
  onCompleted,
}: BatchAdoptDialogProps) {
  const { getOrCreateKey, clearKey, markUnknown } = useIdempotency();
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const [result, setResult] = useState<BulkAdoptChapterPlanCandidatesResult | null>(null);

  const impact = selectedCandidates.reduce(
    (summary, candidate) => {
      summary[candidate.diffType] += 1;
      return summary;
    },
    { new: 0, replace: 0, no_change: 0, stale_conflict: 0 },
  );
  const conflictCount = selectedCandidates.filter(
    (candidate) => candidate.status === "stale" || candidate.diffType === "stale_conflict",
  ).length;

  const handleSubmit = async () => {
    setSubmitting(true);
    setError(null);
    const scope = `batch:bulk-adopt:${batch.id}`;
    const payloadCandidates = selectedCandidates.map((cand) => ({
      candidateId: cand.id,
      expectedCandidateVersion: cand.version,
      expectedChapterPlanVersion: cand.baseChapterPlanVersion ?? null,
    }));
    const payload = {
      expectedBatchVersion: batch.version,
      candidates: payloadCandidates,
    };

    try {
      const idempotencyKey = await getOrCreateKey(scope, payload);
      const envelope = await adoptChapterPlanCandidates(
        batch.id,
        payload,
        idempotencyKey,
      );

      clearKey(scope);
      setResult(envelope);
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
        setError(new ApiError("批量采用动作执行失败，请重试。", 500));
      }
    } finally {
      setSubmitting(false);
    }
  };



  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="batch-adopt-title"
    >
      <div className="chapter-plan-dialog-content batch-adopt-dialog">
        <header className="chapter-plan-dialog-header">
          <h3 id="batch-adopt-title">确认批量采用候选</h3>
          <button
            type="button"
            className="chapter-plan-dialog-close"
            onClick={onClose}
            aria-label="关闭"
          >
            <Icon name="close" size={18} />
          </button>
        </header>

        <div className="chapter-plan-dialog-body">
          {error && (
            <div className="chapter-plans-form-error" role="alert">
              {chapterPlanningErrorMessage(error, "批量采用未完成，请刷新后重试。")}
            </div>
          )}

          {!result ? (
            <>
              <section className="batch-adopt-summary" aria-label="批量采用摘要">
                <div>
                  <strong>已选 {selectedCandidates.length} 个候选章节</strong>
                  <p>
                    批次：{candidateBatchModeLabel(batch.generationMode)} · 第 {batch.target.startChapterNo}—
                    {batch.target.endChapterNo} 章 · 来源 Run ID：<code>{batch.sourceWorkflowRunId}</code>
                  </p>
                </div>
              </section>

              <section className="batch-adopt-impact" aria-label="批量采用影响">
                <h4>本次采用影响</h4>
                <div className="batch-adopt-impact-grid">
                  <div className="positive">
                    <strong>{impact.new}</strong>
                    <span>新设章节</span>
                  </div>
                  <div className="positive">
                    <strong>{impact.replace}</strong>
                    <span>替换候选</span>
                  </div>
                  <div className="neutral">
                    <strong>{impact.no_change}</strong>
                    <span>无变化</span>
                  </div>
                  <div className={conflictCount > 0 ? "warning" : "neutral"}>
                    <strong>{conflictCount}</strong>
                    <span>存在冲突</span>
                  </div>
                </div>
              </section>

              {conflictCount > 0 && (
                <div className="batch-adopt-conflict-warning" role="alert">
                  <Icon name="info" size={20} />
                  <div>
                    <strong>{conflictCount} 个候选存在基线冲突</strong>
                    <p>冲突候选不会被强制覆盖，提交后会在结果中单独标记，需要重新比较或刷新后再处理。</p>
                  </div>
                </div>
              )}

              <section className="batch-adopt-explanation" aria-label="采用说明">
                <h4>采用说明</h4>
                <ul>
                  <li>采用后进入章节规划的待确认流程，并生成版本化修订记录。</li>
                  <li>本操作不会直接确认章节，也不会触发正文生产。</li>
                  <li>被替换章节保留历史版本和来源记录。</li>
                </ul>
              </section>

              <section className="batch-adopt-candidate-list" aria-label="待采用候选列表">
                <h4>待采用候选列表</h4>
                <ul className="chapter-plan-preflight-list">
                  {selectedCandidates.map((cand) => (
                    <li key={cand.id} className="preflight-item info">
                      <div>
                        <strong>第 {cand.chapterNo} 章：{cand.currentSnapshot.title}</strong>
                        <p>{cand.diffType === "stale_conflict" ? "基线冲突，提交后将单独返回处理结果" : "可进入采用处理"}</p>
                      </div>
                      <span className={`badge ${cand.diffType === "stale_conflict" ? "blocker" : "info"}`}>
                        第 {cand.version} 版
                      </span>
                    </li>
                  ))}
                </ul>
              </section>
            </>
          ) : (
            <div className="batch-adopt-result-section">
              <div className="chapter-plan-status-banner success">
                <Icon name="sparkles" size={20} />
                <div>
                  <strong>批量采用处理完成</strong>
                  <p>共处理 {result.items.length} 项候选，独立事务执行结果如下：</p>
                </div>
              </div>

              <ul className="chapter-plan-preflight-list" style={{ marginTop: 16 }}>
                {result.items.map((item, idx) => (
                  <ItemizedOutcomeRow key={idx} item={item} />
                ))}
              </ul>
            </div>
          )}
        </div>

        <footer className="chapter-plan-dialog-footer">
          {!result ? (
            <>
              <button
                type="button"
                className="chapter-plan-button secondary"
                onClick={onClose}
                disabled={submitting}
              >
                取消
              </button>
              <button
                type="button"
                className="chapter-plan-button primary"
                onClick={() => void handleSubmit()}
                disabled={submitting || selectedCandidates.length === 0}
              >
                {submitting ? "正在执行采用..." : "确认批量采用"}
              </button>
            </>
          ) : (
            <button
              type="button"
              className="chapter-plan-button primary"
              onClick={() => {
                onCompleted(result);
                onClose();
              }}
            >
              完成并刷新
            </button>
          )}
        </footer>
      </div>
    </div>
  );
}

function ItemizedOutcomeRow({ item }: { item: BulkAdoptChapterPlanCandidateResult }) {
  const isSuccess = item.outcome === "adopted";
  const isNoChange = item.outcome === "no_change";
  const isStale = item.outcome === "stale";
  const isConflict = item.outcome === "conflict";

  return (
    <li
      className={`preflight-item ${
        isSuccess ? "info" : isNoChange ? "info" : "blocker"
      }`}
    >
      <div className="preflight-item-header">
        <span
          className={`badge ${
            isSuccess ? "success" : isNoChange ? "warning" : "blocker"
          }`}
        >
          {isSuccess
            ? "已采用"
            : isNoChange
              ? "无变化"
              : isStale
                ? "基线过期"
                : isConflict
                  ? "版本冲突"
                  : "处理失败"}
        </span>
        <strong>候选处理结果</strong>
      </div>
      {isNoChange && (
        <p className="preflight-item-detail">
          该候选内容与线上章节无差异，未创建新的修订记录。
        </p>
      )}
      {(isStale || isConflict) && (
        <p className="preflight-item-detail warning">
          采用已被拦截，请先重新比较或刷新最新版本。
        </p>
      )}
    </li>
  );
}

// --- P15_C9_BATCH_ABANDON_DIALOG ---

export interface BatchAbandonDialogProps {
  batch: ChapterPlanCandidateBatch;
  onClose: () => void;
  onAbandoned: (updatedBatch: ChapterPlanCandidateBatch) => void;
}

export function BatchAbandonDialog({
  batch,
  onClose,
  onAbandoned,
}: BatchAbandonDialogProps) {
  const { getOrCreateKey, clearKey, markUnknown } = useIdempotency();
  const [reason, setReason] = useState("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!acknowledged) return;
    setSubmitting(true);
    setError(null);

    const scope = `batch:abandon:${batch.id}`;
    const payload = {
      expectedBatchVersion: batch.version,
      reason: reason.trim() || null,
      acknowledgeAdoptedChaptersRemain: true as const,
    };

    try {
      const idempotencyKey = await getOrCreateKey(scope, payload);
      const envelope = await abandonChapterPlanCandidateBatch(
        batch.id,
        payload,
        idempotencyKey,
      );

      clearKey(scope);
      onAbandoned(envelope);
      onClose();
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
        setError(new ApiError("放弃批次执行失败，请重试。", 500));
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="batch-abandon-title"
    >
      <div className="chapter-plan-dialog-content">
        <header className="chapter-plan-dialog-header">
          <h3 id="batch-abandon-title">放弃候选批次确认</h3>
          <button
            type="button"
            className="chapter-plan-dialog-close"
            onClick={onClose}
            aria-label="关闭"
          >
            <Icon name="close" size={18} />
          </button>
        </header>

        <form onSubmit={handleSubmit} className="chapter-plan-dialog-body">
          {error && (
            <div className="chapter-plans-form-error" role="alert">
              {chapterPlanningErrorMessage(error, "批次放弃未完成，请刷新后重试。")}
            </div>
          )}

          <div className="chapter-plan-status-banner warning">
            <Icon name="info" size={20} />
            <div>
              <strong>提示：已采用章节将被保留</strong>
              <p>
                放弃本批次后，未采用的候选将无法再修改或采用。但本批次中<b>已采用的章节及其修订记录将被完整保留</b>，不会发生回滚。
              </p>
            </div>
          </div>

          <div className="chapter-plan-form-group" style={{ marginTop: 16 }}>
            <label className="chapter-plan-form-label">
              <input
                type="checkbox"
                checked={acknowledged}
                onChange={(e) => setAcknowledged(e.target.checked)}
                style={{ marginRight: 8 }}
              />
              我已知晓并确认：已采用的章节将保留线上修订记录，不会发生回滚。
            </label>
          </div>

          <div className="chapter-plan-form-group" style={{ marginTop: 16 }}>
            <label htmlFor="abandonReason" className="chapter-plan-form-label">
              放弃原因 (可选)
            </label>
            <textarea
              id="abandonReason"
              rows={3}
              maxLength={500}
              placeholder="请输入放弃本批次的原因说明..."
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </div>

          <footer className="chapter-plan-dialog-footer">
            <button
              type="button"
              className="chapter-plan-button secondary"
              onClick={onClose}
              disabled={submitting}
            >
              取消
            </button>
            <button
              type="submit"
              className="chapter-plan-button primary"
              disabled={submitting || !acknowledged}
            >
              {submitting ? "正在放弃..." : "确认放弃本批次"}
            </button>
          </footer>
        </form>
      </div>
    </div>
  );
}

// --- P15_C10_STALE_CONFLICT_DIALOG ---

export interface StaleConflictDialogProps {
  candidate: ChapterPlanCandidate;
  onClose: () => void;
  onRecompare: () => void;
  onRefresh: () => void;
}

export function StaleConflictDialog({
  candidate,
  onClose,
  onRecompare,
  onRefresh,
}: StaleConflictDialogProps) {
  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="stale-conflict-title"
    >
      <div className="chapter-plan-dialog-content">
        <header className="chapter-plan-dialog-header">
          <h3 id="stale-conflict-title">基线与版本冲突处理</h3>
          <button
            type="button"
            className="chapter-plan-dialog-close"
            onClick={onClose}
            aria-label="关闭"
          >
            <Icon name="close" size={18} />
          </button>
        </header>

        <div className="chapter-plan-dialog-body">
          <div className="chapter-plan-status-banner warning">
            <Icon name="info" size={20} />
            <div>
              <strong>候选基线已变更或存在版本冲突</strong>
              <p>
                第 {candidate.chapterNo} 章候选在生成后，线上对应的正式章节或服务端版本已更新。
                系统已禁止强制覆盖操作。
              </p>
            </div>
          </div>

          <div className="chapter-plan-preflight-section" style={{ marginTop: 16 }}>
            <p className="chapter-plan-help-text">
              建议您先执行<b>“重新比较”</b>以获取最新的字段级差异，或<b>“刷新”</b>获取最新的服务端版本。
            </p>
          </div>
        </div>

        <footer className="chapter-plan-dialog-footer">
          <button
            type="button"
            className="chapter-plan-button secondary"
            onClick={onClose}
          >
            取消
          </button>
          <button
            type="button"
            className="chapter-plan-button secondary"
            onClick={onRefresh}
          >
            刷新最新版本
          </button>
          <button
            type="button"
            className="chapter-plan-button primary"
            onClick={onRecompare}
          >
            重新比较
          </button>
        </footer>
      </div>
    </div>
  );
}

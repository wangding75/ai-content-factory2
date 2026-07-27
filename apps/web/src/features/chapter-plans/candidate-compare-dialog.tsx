"use client";

import { useEffect, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import {
  compareChapterPlanCandidate,
  recompareChapterPlanCandidate,
  type ChapterPlanCandidateComparison,
} from "./chapter-plan-http-api";

import { useIdempotency } from "./use-idempotency";

export interface CandidateCompareDialogProps {
  candidateId: string;
  onClose: () => void;
}

export function CandidateCompareDialog({
  candidateId,
  onClose,
}: CandidateCompareDialogProps) {
  const { getOrCreateKey, clearKey, markUnknown } = useIdempotency();
  const [comparison, setComparison] =
    useState<ChapterPlanCandidateComparison | null>(null);
  const [loading, setLoading] = useState(true);
  const [recomparing, setRecomparing] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    compareChapterPlanCandidate(candidateId, { signal: controller.signal })
      .then((envelope) => {
        if (!cancelled) {
          setComparison(envelope.data);
          setError(null);
          setLoading(false);
        }
      })
      .catch((cause) => {
        if (!cancelled && !controller.signal.aborted) {
          setError(
            cause instanceof ApiError
              ? cause
              : new ApiError("加载差异对比失败", 500),
          );
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [candidateId]);

  const handleRecompare = async () => {
    if (!comparison) return;
    setRecomparing(true);
    setError(null);
    const scope = `candidate:recompare:${candidateId}`;
    const payload = { expectedCandidateVersion: comparison.candidate.version };
    try {
      const idempotencyKey = getOrCreateKey(scope, payload);
      const envelope = await recompareChapterPlanCandidate(
        candidateId,
        payload,
        idempotencyKey,
      );
      clearKey(scope);
      setComparison(envelope.data);
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
        setError(new ApiError("重新比较失败，请重试。", 500));
      }
    } finally {
      setRecomparing(false);
    }
  };


  const candidate = comparison?.candidate;
  const currentChapter = comparison?.currentChapter;
  const diff = comparison?.diff;

  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="candidate-compare-title"
    >
      <div className="chapter-plan-dialog-content max-w-3xl">
        <header className="chapter-plan-dialog-header">
          <h3 id="candidate-compare-title">
            章节候选差异对比 {candidate ? `(第 ${candidate.chapterNo} 章)` : ""}
          </h3>
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
              {error.message}
            </div>
          )}

          {loading ? (
            <div className="chapter-plan-spinner" />
          ) : !comparison ? (
            <p>暂无对比数据。</p>
          ) : (
            <>
              {/* Stale Baseline Banner */}
              {diff?.stale && (
                <div className="chapter-plan-status-banner warning">
                  <Icon name="info" size={20} />
                  <div>
                    <strong>候选基线已过期 (Stale Baseline)</strong>
                    <p>当前线上章节在候选生成后已发生修改。请执行“重新比较”更新差异。</p>
                  </div>
                  <button
                    type="button"
                    className="chapter-plan-button secondary"
                    onClick={() => void handleRecompare()}
                    disabled={recomparing}
                  >
                    {recomparing ? "比较中..." : "重新比较"}
                  </button>
                </div>
              )}

              {/* Side-by-side Overview */}
              <div className="compare-grid">
                <div className="compare-column">
                  <h4>当前线上章节 {currentChapter ? `(v${currentChapter.version})` : "(无)"}</h4>
                  {currentChapter ? (
                    <div className="compare-box">
                      <strong>{currentChapter.title}</strong>
                      <p>{currentChapter.summary}</p>
                    </div>
                  ) : (
                    <p className="chapter-plan-help-text">当前无确认章节（新设章节）</p>
                  )}
                </div>

                <div className="compare-column">
                  <h4>候选章节 (v{candidate?.version})</h4>
                  <div className="compare-box highlight">
                    <strong>{candidate?.currentSnapshot.title}</strong>
                    <p>{candidate?.currentSnapshot.summary}</p>
                  </div>
                </div>
              </div>

              {/* Field-level Diff Table */}
              <div className="compare-diff-section">
                <h4>字段级差异列表 ({diff?.entries.length || 0} 项)</h4>
                {diff?.entries.length === 0 ? (
                  <p className="chapter-plan-help-text">两版本无任何内容差异 (no_change)。</p>
                ) : (
                  <table className="chapter-plan-diff-table">
                    <thead>
                      <tr>
                        <th>字段路径</th>
                        <th>变更类型</th>
                        <th>变更前 (Before)</th>
                        <th>变更后 (After)</th>
                      </tr>
                    </thead>
                    <tbody>
                      {diff?.entries.map((entry, index) => (
                        <tr key={index} className={`diff-row ${entry.changeType}`}>
                          <td><code>{entry.path}</code></td>
                          <td>
                            <span className={`diff-tag ${entry.changeType}`}>
                              {entry.changeType === "added"
                                ? "新增"
                                : entry.changeType === "removed"
                                  ? "删除"
                                  : entry.changeType === "changed"
                                    ? "修改"
                                    : "未变"}
                            </span>
                          </td>
                          <td>{entry.before || "—"}</td>
                          <td>{entry.after || "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </>
          )}
        </div>

        <footer className="chapter-plan-dialog-footer">
          <button
            type="button"
            className="chapter-plan-button secondary"
            onClick={onClose}
          >
            关闭
          </button>
          {comparison && (
            <button
              type="button"
              className="chapter-plan-button primary"
              onClick={() => void handleRecompare()}
              disabled={recomparing}
            >
              {recomparing ? "重新比较中..." : "重新比较 (Recompare)"}
            </button>
          )}
        </footer>
      </div>
    </div>
  );
}

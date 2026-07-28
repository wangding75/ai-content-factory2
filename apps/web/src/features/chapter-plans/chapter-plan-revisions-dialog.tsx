"use client";

import { useEffect, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import {
  listChapterPlanRevisions,
  type ChapterPlanRevision,
} from "./chapter-plan-http-api";
import { chapterPlanningErrorMessage } from "./chapter-plan-presentation";

export interface ChapterPlanRevisionsDialogProps {
  chapterPlanId: string;
  chapterNo: number;
  onClose: () => void;
}

export function revisionChangeTypeLabel(changeType: string): string {
  switch (changeType) {
    case "candidate_adopt":
      return "候选采用";
    case "manual_edit":
      return "手动编辑";
    case "manual_create":
      return "手动新建";
    case "confirm":
      return "章节确认";
    case "legacy_backfill":
      return "历史补录";
    default:
      return changeType;
  }
}

export function ChapterPlanRevisionsDialog({
  chapterPlanId,
  chapterNo,
  onClose,
}: ChapterPlanRevisionsDialogProps) {
  const [revisions, setRevisions] = useState<ChapterPlanRevision[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    listChapterPlanRevisions(chapterPlanId, { limit: 50 }, { signal: controller.signal })
      .then((envelope) => {
        if (!cancelled) {
          setRevisions(envelope.items);
          setError(null);
          setLoading(false);
        }
      })
      .catch((cause) => {
        if (!cancelled && !controller.signal.aborted) {
          setError(
            cause instanceof ApiError
              ? cause
              : new ApiError("加载修订历史失败", 500),
          );
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [chapterPlanId]);


  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="revisions-title"
    >
      <div className="chapter-plan-dialog-content max-w-3xl">
        <header className="chapter-plan-dialog-header">
          <h3 id="revisions-title">
            第 {chapterNo} 章 - 修订历史记录
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
              {chapterPlanningErrorMessage(error, "版本记录暂时无法加载，请稍后重试。")}
            </div>
          )}

          {loading ? (
            <div className="chapter-plan-spinner" />
          ) : revisions.length === 0 ? (
            <p className="chapter-plan-help-text">暂无修订历史。</p>
          ) : (
            <table className="chapter-plan-diff-table">
              <thead>
                <tr>
                  <th>修订版本</th>
                  <th>变更类型</th>
                  <th>标题与摘要快照</th>
                  <th>来源追溯</th>
                  <th>时间</th>
                </tr>
              </thead>
              <tbody>
                {revisions.map((rev) => (
                  <tr key={rev.id}>
                    <td><b>r{rev.revisionNo}</b></td>
                    <td>
                      <span className="badge info">
                        {revisionChangeTypeLabel(rev.changeType)}
                      </span>
                    </td>
                    <td>
                      <strong>{rev.snapshot.title}</strong>
                      <p className="candidate-summary-text">{rev.snapshot.summary}</p>
                    </td>
                    <td>
                      <div className="chapter-plan-help-text">
                        {rev.sourceCandidateId && <div>候选采用</div>}
                        {rev.sourceCandidateBatchId && <div>批次生成</div>}
                        {rev.sourceWorkflowRunId && <div>工作流任务</div>}
                        {!rev.sourceCandidateId && !rev.sourceCandidateBatchId && !rev.sourceWorkflowRunId && (
                          <div>—</div>
                        )}
                      </div>
                    </td>
                    <td>{new Date(rev.createdAt).toLocaleString("zh-CN")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
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
        </footer>
      </div>
    </div>
  );
}

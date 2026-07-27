"use client";

import { useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import {
  updateChapterPlanCandidate,
  type ChapterPlanCandidate,
  type ChapterPlanCandidateSnapshot,
} from "./chapter-plan-http-api";

export interface CandidateEditDrawerProps {
  candidate: ChapterPlanCandidate;
  onClose: () => void;
  onSaved: (updated: ChapterPlanCandidate) => void;
  onRefreshCandidate?: () => void;
}

export function CandidateEditDrawer({
  candidate,
  onClose,
  onSaved,
  onRefreshCandidate,
}: CandidateEditDrawerProps) {
  const [activeTab, setActiveTab] = useState<"current" | "generated" | "base">(
    "current",
  );

  // Editable state for currentSnapshot
  const [currentSnapshot, setCurrentSnapshot] = useState<ChapterPlanCandidateSnapshot>(
    { ...candidate.currentSnapshot },
  );

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const [versionConflict, setVersionConflict] = useState(false);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setVersionConflict(false);

    try {
      const idempotencyKey = `edit-candidate-${candidate.id}-${Date.now()}`;
      const envelope = await updateChapterPlanCandidate(
        candidate.id,
        {
          expectedCandidateVersion: candidate.version,
          currentSnapshot,
        },
        idempotencyKey,
      );
      onSaved(envelope.data);
    } catch (cause) {
      if (cause instanceof ApiError) {
        setError(cause);
        if (
          cause.status === 409 ||
          cause.message?.includes("version_conflict") ||
          cause.message?.includes("invalid_candidate_state")
        ) {
          setVersionConflict(true);
        }
      } else {
        setError(new ApiError("保存候选修改失败，请重试。", 500));
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <div
      className="chapter-plan-drawer-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="candidate-edit-title"
    >
      <div className="chapter-plan-drawer-content">
        <header className="chapter-plan-drawer-header">
          <h2 id="candidate-edit-title">
            编辑章节候选 (第 {candidate.chapterNo} 章 - v{candidate.version})
          </h2>
          <button
            type="button"
            className="chapter-plan-drawer-close"
            onClick={onClose}
            aria-label="关闭抽屉"
          >
            <Icon name="close" size={20} />
          </button>
        </header>

        {/* Snapshot Tabs */}
        <nav className="chapter-plans-filters" aria-label="快照选择">
          <button
            type="button"
            className={activeTab === "current" ? "active" : ""}
            onClick={() => setActiveTab("current")}
          >
            当前快照 (可编辑)
          </button>
          <button
            type="button"
            className={activeTab === "generated" ? "active" : ""}
            onClick={() => setActiveTab("generated")}
          >
            生成初始快照 (只读)
          </button>
          <button
            type="button"
            className={activeTab === "base" ? "active" : ""}
            onClick={() => setActiveTab("base")}
          >
            对比基线快照 (只读)
          </button>
        </nav>

        {error && (
          <div className="chapter-plans-form-error" role="alert">
            {error.message}
            {versionConflict && onRefreshCandidate && (
              <button
                type="button"
                className="chapter-plan-button secondary"
                onClick={onRefreshCandidate}
                style={{ marginLeft: 12 }}
              >
                刷新最新版本
              </button>
            )}
          </div>
        )}

        {activeTab === "current" ? (
          <form onSubmit={handleSave} className="chapter-plan-drawer-body">
            <div className="chapter-plan-form-group">
              <label htmlFor="candTitle" className="chapter-plan-form-label">
                章节标题
              </label>
              <input
                id="candTitle"
                type="text"
                required
                maxLength={120}
                value={currentSnapshot.title}
                onChange={(e) =>
                  setCurrentSnapshot((prev) => ({ ...prev, title: e.target.value }))
                }
              />
            </div>

            <div className="chapter-plan-form-group">
              <label htmlFor="candPurpose" className="chapter-plan-form-label">
                章节目的
              </label>
              <select
                id="candPurpose"
                value={currentSnapshot.chapterPurpose}
                onChange={(e) =>
                  setCurrentSnapshot((prev) => ({
                    ...prev,
                    chapterPurpose: e.target.value,
                  }))
                }
              >
                <option value="plot_advance">剧情推进 (Plot Advance)</option>
                <option value="information_reveal">信息揭露 (Info Reveal)</option>
                <option value="conflict_escalation">冲突升级 (Conflict Escalation)</option>
                <option value="transition">过渡衔接 (Transition)</option>
                <option value="atmosphere">氛围渲染 (Atmosphere)</option>
                <option value="other">其他 (Other)</option>
              </select>
            </div>

            <div className="chapter-plan-form-group">
              <label htmlFor="candSummary" className="chapter-plan-form-label">
                章节摘要
              </label>
              <textarea
                id="candSummary"
                rows={5}
                required
                maxLength={5000}
                value={currentSnapshot.summary}
                onChange={(e) =>
                  setCurrentSnapshot((prev) => ({ ...prev, summary: e.target.value }))
                }
              />
            </div>

            <div className="chapter-plan-form-group">
              <label htmlFor="contextSummary" className="chapter-plan-form-label">
                生成背景摘要 (只读说明)
              </label>
              <textarea
                id="contextSummary"
                rows={3}
                value={currentSnapshot.generationBasis?.contextSummary || ""}
                onChange={(e) =>
                  setCurrentSnapshot((prev) => ({
                    ...prev,
                    generationBasis: {
                      ...prev.generationBasis,
                      contextSummary: e.target.value,
                    },
                  }))
                }
              />
            </div>

            <footer className="chapter-plan-drawer-footer">
              <button
                type="button"
                className="chapter-plan-button secondary"
                onClick={onClose}
                disabled={saving}
              >
                取消
              </button>
              <button
                type="submit"
                className="chapter-plan-button primary"
                disabled={saving}
              >
                {saving ? "正在保存..." : "保存修改"}
              </button>
            </footer>
          </form>
        ) : (
          <div className="chapter-plan-drawer-body readonly-snapshot">
            {activeTab === "generated" ? (
              <SnapshotViewer snapshot={candidate.generatedSnapshot} title="生成初始快照" />
            ) : (
              <SnapshotViewer snapshot={candidate.baseSnapshot} title="对比基线快照" />
            )}
            <footer className="chapter-plan-drawer-footer">
              <button
                type="button"
                className="chapter-plan-button secondary"
                onClick={onClose}
              >
                关闭
              </button>
            </footer>
          </div>
        )}
      </div>
    </div>
  );
}

function SnapshotViewer({
  snapshot,
  title,
}: {
  snapshot: ChapterPlanCandidateSnapshot | null;
  title: string;
}) {
  if (!snapshot) {
    return (
      <div className="chapter-plans-empty">
        <p>暂无{title}</p>
      </div>
    );
  }

  return (
    <div className="snapshot-view">
      <h3>{snapshot.title}</h3>
      <p className="chapter-plan-help-text">章节目的: {snapshot.chapterPurpose}</p>
      <div className="snapshot-summary-box">
        <strong>摘要内容：</strong>
        <p>{snapshot.summary}</p>
      </div>
      {snapshot.storylineRefs.length > 0 && (
        <div>
          <strong>关联故事线：</strong>
          <ul>
            {snapshot.storylineRefs.map((ref) => (
              <li key={ref.id}>{ref.label}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

"use client";

import { useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import {
  updateChapterPlanCandidate,
  type ChapterPlanCandidate,
  type ChapterPlanCandidateSnapshot,
} from "./chapter-plan-http-api";
import {
  candidateDiffTypeLabel,
  candidatePurposeLabel,
  candidateStatusLabel,
  chapterPlanningErrorMessage,
} from "./chapter-plan-presentation";

import { useIdempotency } from "./use-idempotency";

const candidatePurposeOptions = [
  ["information_reveal", "信息揭露"],
  ["plot_advance", "情节推进"],
  ["conflict_escalation", "冲突升级"],
  ["transition", "转折过渡"],
  ["atmosphere", "氛围渲染"],
  ["other", "其他"],
] as const;

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
  const { getOrCreateKey, clearKey, markUnknown } = useIdempotency();
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

    const scope = `candidate:edit:${candidate.id}`;
    const payload = {
      expectedCandidateVersion: candidate.version,
      currentSnapshot,
    };

    try {
      const idempotencyKey = await getOrCreateKey(scope, payload);
      const envelope = await updateChapterPlanCandidate(
        candidate.id,
        payload,
        idempotencyKey,
      );
      clearKey(scope);
      onSaved(envelope);
    } catch (cause) {
      if (
        cause instanceof ApiError &&
        (cause.status === 0 || cause.status >= 500 || cause.code === "timeout")
      ) {
        await markUnknown(scope, payload);
      }
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
          <div>
            <span className="candidate-edit-drawer-kicker">候选版本 · 仅编辑候选快照</span>
            <h2 id="candidate-edit-title">编辑章节候选</h2>
            <p>第 {candidate.chapterNo} 章 · 第 {candidate.version} 版</p>
          </div>
          <button
            type="button"
            className="chapter-plan-drawer-close"
            onClick={onClose}
            aria-label="关闭抽屉"
          >
            <Icon name="close" size={20} />
          </button>
        </header>

        <section className="candidate-edit-drawer-summary" aria-label="候选摘要">
          <div className="candidate-edit-drawer-summary-heading">
            <strong>第 {candidate.chapterNo} 章候选</strong>
            <span className={`chapter-plan-status ${candidate.status}`}>
              状态：{candidateStatusLabel(candidate.status)}
            </span>
          </div>
          <dl>
            <div>
              <dt>批次 ID</dt>
              <dd><code>{candidate.batchId}</code></dd>
            </div>
            <div>
              <dt>差异类型</dt>
              <dd>{candidateDiffTypeLabel(candidate.diffType)}</dd>
            </div>
            <div>
              <dt>基线版本</dt>
              <dd>{candidate.baseChapterPlanVersion ? `第 ${candidate.baseChapterPlanVersion} 版` : "新设章节"}</dd>
            </div>
          </dl>
        </section>

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
            {chapterPlanningErrorMessage(error, "候选保存失败，请稍后重试。")}
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
          <form onSubmit={handleSave} className="chapter-plan-drawer-body candidate-edit-drawer-body">
            <section className="candidate-edit-drawer-section" aria-labelledby="candidate-fields-title">
              <div className="candidate-edit-drawer-section-heading">
                <div>
                  <h3 id="candidate-fields-title">候选章节字段</h3>
                  <p>保存修改只更新这个候选版本，不直接编辑当前已采用章节。</p>
                </div>
                <span className="candidate-edit-required-note">* 必填</span>
              </div>

              <div className="candidate-edit-drawer-field-row">
                <div className="chapter-plan-form-group">
                  <label htmlFor="candChapterNo" className="chapter-plan-form-label">章节号</label>
                  <input id="candChapterNo" type="number" value={candidate.chapterNo} readOnly aria-readonly="true" />
                </div>
                <div className="chapter-plan-form-group">
                  <label htmlFor="candTitle" className="chapter-plan-form-label">章节标题 <em>*</em></label>
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
              </div>

              <div className="chapter-plan-form-group">
                <label htmlFor="candSummary" className="chapter-plan-form-label">
                  章节概要 <em>*</em><span>建议 100–300 字</span>
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

              <fieldset className="chapter-plan-form-group candidate-edit-purpose-fieldset">
                <legend className="chapter-plan-form-label">章节目的</legend>
                <div className="candidate-edit-purpose-options" role="radiogroup" aria-label="章节目的">
                  {candidatePurposeOptions.map(([value, label]) => (
                    <button
                      key={value}
                      type="button"
                      role="radio"
                      aria-checked={currentSnapshot.chapterPurpose === value}
                      className={currentSnapshot.chapterPurpose === value ? "active" : ""}
                      onClick={() =>
                        setCurrentSnapshot((prev) => ({ ...prev, chapterPurpose: value }))
                      }
                    >
                      {currentSnapshot.chapterPurpose === value ? "✓ " : ""}{label}
                    </button>
                  ))}
                </div>
              </fieldset>
            </section>

            <section className="candidate-edit-drawer-section" aria-labelledby="candidate-relations-title">
              <div className="candidate-edit-drawer-section-heading">
                <div>
                  <h3 id="candidate-relations-title">候选关联信息</h3>
                  <p>关联信息随候选快照展示，当前抽屉不修改项目中的原始内容。</p>
                </div>
              </div>
              <ReferenceGroup label="关联故事线" refs={currentSnapshot.storylineRefs} />
              <ReferenceGroup label="出场素材" refs={currentSnapshot.materialRefs} />
              <ReferenceGroup label="伏笔关联" refs={currentSnapshot.foreshadowingRefs} />
            </section>

            <section className="candidate-edit-drawer-section" aria-labelledby="candidate-generation-title">
              <div className="candidate-edit-drawer-section-heading">
                <div>
                  <h3 id="candidate-generation-title">生成背景</h3>
                  <p>以下内容用于理解候选来源，保存修改时保持不变。</p>
                </div>
                <span className="candidate-edit-readonly-badge">只读</span>
              </div>
              <div className="candidate-edit-readonly-box">
                <strong>背景摘要</strong>
                <p>{currentSnapshot.generationBasis?.contextSummary || "暂无生成背景摘要"}</p>
                {currentSnapshot.generationBasis?.additionalInstructions && (
                  <>
                    <strong>补充指令</strong>
                    <p>{currentSnapshot.generationBasis.additionalInstructions}</p>
                  </>
                )}
              </div>
            </section>

            <footer className="chapter-plan-drawer-footer candidate-edit-drawer-footer">
              <div className="chapter-plan-drawer-footer-summary">
                <Icon name="info" size={18} />
                <div>
                  <strong>仅保存候选版本</strong>
                  <small>不会覆盖当前章节内容</small>
                </div>
              </div>
              <div className="chapter-plan-drawer-footer-actions">
                <button
                  type="button"
                  className="chapter-plan-button secondary"
                  onClick={onClose}
                  disabled={saving}
                >
                  取消
                </button>
                <button type="submit" className="chapter-plan-button primary" disabled={saving}>
                  {saving ? "正在保存..." : "保存修改"}
                </button>
              </div>
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

function ReferenceGroup({
  label,
  refs,
}: {
  label: string;
  refs: ChapterPlanCandidateSnapshot["storylineRefs"];
}) {
  return (
    <div className="candidate-edit-reference-group">
      <strong>{label}</strong>
      <div className="candidate-edit-reference-list">
        {refs.length > 0 ? (
          refs.map((ref) => <span key={ref.id}>{ref.label}</span>)
        ) : (
          <span className="empty">暂无关联</span>
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
      <p className="chapter-plan-help-text">
        章节目的: {candidatePurposeLabel(snapshot.chapterPurpose)}
      </p>
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

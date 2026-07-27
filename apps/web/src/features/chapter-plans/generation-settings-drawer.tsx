"use client";

import { useState } from "react";
import { Icon } from "@/components/ui/icons";
import type { StorylineNode } from "@/lib/api";
import type {
  ChapterPlanningContextOptions,
  ChapterPlanningGenerationMode,
  ChapterPlanningPreflightRequest,
  ChapterPlanningStorylineSelection,
} from "./chapter-plan-http-api";
import { flattenStorylines } from "./chapter-plan-presentation";

export interface GenerationSettingsDrawerProps {
  storylines: StorylineNode[];
  onClose: () => void;
  onSubmit: (request: ChapterPlanningPreflightRequest) => void;
  submitting?: boolean;
}

export function GenerationSettingsDrawer({
  storylines,
  onClose,
  onSubmit,
  submitting = false,
}: GenerationSettingsDrawerProps) {
  const [generationMode, setGenerationMode] =
    useState<ChapterPlanningGenerationMode>("range");

  // Targets
  const [chapterCount, setChapterCount] = useState<number>(5);
  const [startChapterNo, setStartChapterNo] = useState<number>(1);
  const [endChapterNo, setEndChapterNo] = useState<number>(5);

  // Storylines
  const flatStorylines = flattenStorylines(storylines);
  const [storylineMode, setStorylineMode] = useState<"auto_balanced" | "specified">(
    "auto_balanced",
  );
  const [selectedStorylineIds, setSelectedStorylineIds] = useState<string[]>([]);

  // Context options
  const [contextOptions, setContextOptions] = useState<ChapterPlanningContextOptions>({
    includeProjectMaterials: true,
    includeUnpaidForeshadowings: true,
    includePriorChapterSummaries: true,
    coreSettingsOnly: false,
  });

  // Additional instructions
  const [additionalInstructions, setAdditionalInstructions] = useState("");
  const [validationError, setValidationError] = useState<string | null>(null);

  const toggleStoryline = (id: string) => {
    setSelectedStorylineIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id],
    );
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);

    let targetPayload:
      | { requestedChapterCount: number }
      | { startChapterNo: number; endChapterNo: number };

    if (generationMode === "full" || generationMode === "append") {
      if (!chapterCount || chapterCount < 1 || chapterCount > 100) {
        setValidationError("生成章节数量必须在 1 至 100 之间");
        return;
      }
      targetPayload = { requestedChapterCount: Math.round(chapterCount) };
    } else {
      if (!startChapterNo || startChapterNo < 1) {
        setValidationError("起始章节必须大于等于 1");
        return;
      }
      if (!endChapterNo || endChapterNo < startChapterNo) {
        setValidationError("结束章节不能小于起始章节");
        return;
      }
      const count = endChapterNo - startChapterNo + 1;
      if (count > 100) {
        setValidationError("单次生成章节数量不能超过 100 章");
        return;
      }
      targetPayload = {
        startChapterNo: Math.round(startChapterNo),
        endChapterNo: Math.round(endChapterNo),
      };
    }

    let storylineSelectionPayload: ChapterPlanningStorylineSelection;
    if (storylineMode === "specified") {
      if (selectedStorylineIds.length === 0) {
        setValidationError("请至少选择一条指定故事线");
        return;
      }
      storylineSelectionPayload = {
        mode: "specified",
        storylineIds: selectedStorylineIds,
      };
    } else {
      storylineSelectionPayload = { mode: "auto_balanced" };
    }

    const payload: ChapterPlanningPreflightRequest = {
      generationMode,
      target: targetPayload,
      storylineSelection: storylineSelectionPayload,
      contextOptions,
      additionalInstructions: additionalInstructions.trim() || null,
    };

    onSubmit(payload);
  };

  return (
    <div
      className="chapter-plan-drawer-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="generation-settings-title"
    >
      <div className="chapter-plan-drawer-content">
        <header className="chapter-plan-drawer-header">
          <h2 id="generation-settings-title">生成章节规划设置</h2>
          <button
            type="button"
            className="chapter-plan-drawer-close"
            onClick={onClose}
            aria-label="关闭抽屉"
          >
            <Icon name="close" size={20} />
          </button>
        </header>

        <form onSubmit={handleSubmit} className="chapter-plan-drawer-body">
          {validationError && (
            <div className="chapter-plans-form-error" role="alert">
              {validationError}
            </div>
          )}

          {/* 1. 生成模式 */}
          <fieldset className="chapter-plan-form-group">
            <legend className="chapter-plan-form-label">生成模式</legend>
            <div className="chapter-plan-radio-group">
              <label className="chapter-plan-radio-item">
                <input
                  type="radio"
                  name="generationMode"
                  value="range"
                  checked={generationMode === "range"}
                  onChange={() => setGenerationMode("range")}
                />
                <div>
                  <strong>局部范围生成</strong>
                  <p className="chapter-plan-help-text">
                    按指定章节编号范围（如 1–5 章）重新规划或调整。
                  </p>
                </div>
              </label>

              <label className="chapter-plan-radio-item">
                <input
                  type="radio"
                  name="generationMode"
                  value="full"
                  checked={generationMode === "full"}
                  onChange={() => setGenerationMode("full")}
                />
                <div>
                  <strong>完整大纲生成</strong>
                  <p className="chapter-plan-help-text">
                    从第 1 章开始生成完整大纲章节序列。
                  </p>
                </div>
              </label>

              <label className="chapter-plan-radio-item">
                <input
                  type="radio"
                  name="generationMode"
                  value="append"
                  checked={generationMode === "append"}
                  onChange={() => setGenerationMode("append")}
                />
                <div>
                  <strong>追加后续章节</strong>
                  <p className="chapter-plan-help-text">
                    紧接当前已存在最高章节序号顺延生成。
                  </p>
                </div>
              </label>
            </div>
          </fieldset>

          {/* 2. 目标范围 / 数量 */}
          {generationMode === "range" ? (
            <div className="chapter-plan-form-row">
              <div className="chapter-plan-form-group">
                <label htmlFor="startChapterNo" className="chapter-plan-form-label">
                  起始章节
                </label>
                <input
                  id="startChapterNo"
                  type="number"
                  min={1}
                  value={startChapterNo}
                  onChange={(e) => setStartChapterNo(Number(e.target.value))}
                  required
                />
              </div>
              <div className="chapter-plan-form-group">
                <label htmlFor="endChapterNo" className="chapter-plan-form-label">
                  结束章节
                </label>
                <input
                  id="endChapterNo"
                  type="number"
                  min={1}
                  value={endChapterNo}
                  onChange={(e) => setEndChapterNo(Number(e.target.value))}
                  required
                />
              </div>
            </div>
          ) : (
            <div className="chapter-plan-form-group">
              <label htmlFor="chapterCount" className="chapter-plan-form-label">
                生成章节数量
              </label>
              <input
                id="chapterCount"
                type="number"
                min={1}
                max={100}
                value={chapterCount}
                onChange={(e) => setChapterCount(Number(e.target.value))}
                required
              />
            </div>
          )}

          {/* 3. 故事线选择 */}
          <fieldset className="chapter-plan-form-group">
            <legend className="chapter-plan-form-label">故事线范围</legend>
            <div className="chapter-plan-radio-group">
              <label className="chapter-plan-radio-item">
                <input
                  type="radio"
                  name="storylineMode"
                  value="auto_balanced"
                  checked={storylineMode === "auto_balanced"}
                  onChange={() => setStorylineMode("auto_balanced")}
                />
                <span>自动平衡主支线</span>
              </label>
              <label className="chapter-plan-radio-item">
                <input
                  type="radio"
                  name="storylineMode"
                  value="specified"
                  checked={storylineMode === "specified"}
                  onChange={() => setStorylineMode("specified")}
                />
                <span>指定故事线</span>
              </label>
            </div>

            {storylineMode === "specified" && (
              <div className="chapter-plan-checkbox-list">
                {flatStorylines.map((line) => (
                  <label key={line.id} className="chapter-plan-checkbox-item">
                    <input
                      type="checkbox"
                      checked={selectedStorylineIds.includes(line.id)}
                      onChange={() => toggleStoryline(line.id)}
                    />
                    <span>{line.name}</span>
                  </label>
                ))}
                {flatStorylines.length === 0 && (
                  <p className="chapter-plan-help-text">暂无可选故事线</p>
                )}
              </div>
            )}
          </fieldset>

          {/* 4. 上下文与素材参考选项 */}
          <fieldset className="chapter-plan-form-group">
            <legend className="chapter-plan-form-label">上下文参考选项</legend>
            <div className="chapter-plan-checkbox-list">
              <label className="chapter-plan-checkbox-item">
                <input
                  type="checkbox"
                  checked={contextOptions.includeProjectMaterials}
                  onChange={(e) =>
                    setContextOptions((prev) => ({
                      ...prev,
                      includeProjectMaterials: e.target.checked,
                    }))
                  }
                />
                <span>包含项目素材</span>
              </label>
              <label className="chapter-plan-checkbox-item">
                <input
                  type="checkbox"
                  checked={contextOptions.includeUnpaidForeshadowings}
                  onChange={(e) =>
                    setContextOptions((prev) => ({
                      ...prev,
                      includeUnpaidForeshadowings: e.target.checked,
                    }))
                  }
                />
                <span>包含未回收伏笔</span>
              </label>
              <label className="chapter-plan-checkbox-item">
                <input
                  type="checkbox"
                  checked={contextOptions.includePriorChapterSummaries}
                  onChange={(e) =>
                    setContextOptions((prev) => ({
                      ...prev,
                      includePriorChapterSummaries: e.target.checked,
                    }))
                  }
                />
                <span>包含前文章节摘要</span>
              </label>
              <label className="chapter-plan-checkbox-item">
                <input
                  type="checkbox"
                  checked={contextOptions.coreSettingsOnly}
                  onChange={(e) =>
                    setContextOptions((prev) => ({
                      ...prev,
                      coreSettingsOnly: e.target.checked,
                    }))
                  }
                />
                <span>仅使用核心设定</span>
              </label>
            </div>
          </fieldset>

          {/* 5. 补充说明 */}
          <div className="chapter-plan-form-group">
            <label htmlFor="additionalInstructions" className="chapter-plan-form-label">
              补充生成要求 (可选)
            </label>
            <textarea
              id="additionalInstructions"
              rows={3}
              maxLength={2000}
              placeholder="请输入任何针对本次生成的额外约束或提示说明..."
              value={additionalInstructions}
              onChange={(e) => setAdditionalInstructions(e.target.value)}
            />
          </div>

          <footer className="chapter-plan-drawer-footer">
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
              disabled={submitting}
            >
              {submitting ? "正在生成预检..." : "开始预检"}
            </button>
          </footer>
        </form>
      </div>
    </div>
  );
}

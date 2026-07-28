"use client";

import { Icon } from "@/components/ui/icons";
import type {
  ChapterPlanningBlockerItem,
  ChapterPlanningPreflightBlocked,
  ChapterPlanningPreflightPassed,
  ChapterPlanningPreflightReport,
} from "./chapter-plan-http-api";

function blockerReasonLabel(item: ChapterPlanningBlockerItem): string {
  switch (item.code) {
    case "project_binding_missing":
      return "项目尚未配置章节规划工作流。";
    case "execution_integration_unavailable":
      return "章节规划执行配置暂不可用。";
    case "active_run_conflict":
      return "当前已有章节规划任务正在运行。";
    case "storyline_reference_invalid":
      return "所选故事线已失效或不属于当前项目。";
    case "generation_input_invalid":
      return "生成范围或生成参数不符合要求。";
  }
}

function blockerTitleLabel(item: ChapterPlanningBlockerItem): string {
  switch (item.code) {
    case "project_binding_missing":
      return "项目工作流尚未配置";
    case "execution_integration_unavailable":
      return "工作流执行服务暂不可用";
    case "active_run_conflict":
      return "已有生成任务正在运行";
    case "storyline_reference_invalid":
      return "所选故事线不可用";
    case "generation_input_invalid":
      return "生成设置需要调整";
    default:
      return "预检发现阻断项";
  }
}

function retryActionLabel(action: string): string {
  switch (action) {
    case "configure_project_binding":
    case "open_project_settings":
    case "configure_workflow":
      return "前往项目设置完成工作流配置后重试。";
    case "retry_after_integration_recovers":
    case "retry_preflight":
      return "待执行服务恢复后重新预检。";
    case "wait_for_active_run":
      return "等待当前任务结束后重试。";
    case "refresh_storylines":
    case "review_storyline_selection":
      return "刷新故事线并重新选择。";
    case "fix_generation_input":
    case "review_generation_target":
      return "返回生成设置并修正范围或参数。";
    case "restore_execution_integration":
    case "verify_execution_integration":
      return "恢复并复验工作流执行配置后重试。";
    default:
      return "修正阻断项后重新预检。";
  }
}

export interface PreflightProgressDialogProps {
  onClose?: () => void;
}

export function PreflightProgressDialog({ onClose }: PreflightProgressDialogProps) {
  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="preflight-progress-title"
    >
      <div className="chapter-plan-dialog-content">
        <header className="chapter-plan-dialog-header">
          <h3 id="preflight-progress-title">生成预检进行中</h3>
          {onClose && (
            <button
              type="button"
              className="chapter-plan-dialog-close"
              onClick={onClose}
              aria-label="关闭"
            >
              <Icon name="close" size={18} />
            </button>
          )}
        </header>
        <div className="chapter-plan-dialog-body preflight-progress-body">
          <div className="chapter-plan-spinner" aria-hidden="true" />
          <p>正在对工作流配置、故事线关联及运行状态进行检查...</p>
        </div>
      </div>
    </div>
  );
}

export interface PreflightReportDialogProps {
  report: ChapterPlanningPreflightReport;
  onClose: () => void;
  onCreateRun?: (token: string) => void;
  creatingRun?: boolean;
}

export function PreflightReportDialog({
  report,
  onClose,
  onCreateRun,
  creatingRun = false,
}: PreflightReportDialogProps) {
  const isPassed = report.result === "passed";
  const passedReport = isPassed ? (report as ChapterPlanningPreflightPassed) : null;
  const blockedReport = !isPassed ? (report as ChapterPlanningPreflightBlocked) : null;

  const inputSummary = report.inputSummary;
  const modeLabel = inputSummary
    ? inputSummary.generationMode === "full"
      ? "完整大纲"
      : inputSummary.generationMode === "append"
        ? "追加后续"
        : "局部范围"
    : null;

  const targetLabel = inputSummary
    ? inputSummary.generationMode === "range"
      ? `第 ${inputSummary.target.startChapterNo}–${inputSummary.target.endChapterNo} 章`
      : `${inputSummary.target.requestedChapterCount} 个章节`
    : null;

  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="preflight-report-title"
    >
      <div className="chapter-plan-dialog-content">
        <header className="chapter-plan-dialog-header">
          <h3 id="preflight-report-title">
            {isPassed ? "预检完成报告" : "预检阻断报告"}
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
          <div
            className={`chapter-plan-status-banner ${
              isPassed ? "success" : "blocked"
            }`}
          >
            <Icon name={isPassed ? "sparkles" : "info"} size={20} />
            <div>
              <strong>
                {isPassed
                  ? report.warnings.length > 0
                    ? "预检通过 (含提示警告)"
                    : "预检通过，具备生成条件"
                  : "预检阻断，暂时无法生成"}
              </strong>
              {modeLabel && targetLabel && (
                <p>
                  模式：{modeLabel}（{targetLabel}）
                </p>
              )}
            </div>
          </div>

          {/* Blockers list */}
          {blockedReport && (
            <div className="chapter-plan-preflight-section">
              <h4>阻断原因列表</h4>
              <ul className="chapter-plan-preflight-list">
                {blockedReport.blockers.map((item, index) => (
                  <li key={index} className="preflight-item blocker">
                    <div className="preflight-item-header">
                      <span className="badge blocker">阻断</span>
                      <strong>{blockerTitleLabel(item)}</strong>
                    </div>
                    <p className="preflight-item-detail">{blockerReasonLabel(item)}</p>
                    <p className="preflight-item-action">
                      建议操作：
                      {retryActionLabel(
                        item.retryAction ||
                          item.details?.retryAction ||
                          item.details?.action ||
                          "",
                      )}
                    </p>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Warnings */}
          {report.warnings && report.warnings.length > 0 && (
            <div className="chapter-plan-preflight-section">
              <h4>注意事项与警告</h4>
              <ul className="chapter-plan-preflight-list">
                {report.warnings.map((item, index) => (
                  <li key={index} className="preflight-item warning">
                    <div className="preflight-item-header">
                      <span className="badge warning">警告</span>
                      <span>生成上下文存在需要留意的事项。</span>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Passed info */}
          {passedReport && (
            <div className="chapter-plan-preflight-section">
              <p className="chapter-plan-help-text">
                预检 Token 有效期为 10 分钟。确认发起后将自动创建章节规划运行任务。
              </p>
            </div>
          )}
        </div>

        <footer className="chapter-plan-dialog-footer">
          <button
            type="button"
            className="chapter-plan-button secondary"
            onClick={onClose}
            disabled={creatingRun}
          >
            {isPassed ? "取消" : "关闭"}
          </button>

          {isPassed && passedReport && onCreateRun && (
            <button
              type="button"
              className="chapter-plan-button primary"
              onClick={() => onCreateRun(passedReport.preflightToken)}
              disabled={creatingRun}
            >
              {creatingRun ? "正在发起任务..." : "确认发起生成"}
            </button>
          )}
        </footer>
      </div>
    </div>
  );
}

export interface RunCreatedDialogProps {
  runId: string;
  onClose: () => void;
}

export function RunCreatedDialog({ onClose }: RunCreatedDialogProps) {
  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="run-created-title"
    >
      <div className="chapter-plan-dialog-content">
        <header className="chapter-plan-dialog-header">
          <h3 id="run-created-title">任务创建成功</h3>
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
          <div className="chapter-plan-status-banner success">
            <Icon name="sparkles" size={20} />
            <div>
              <strong>章节规划生成任务已进入队列</strong>
              <p>系统已创建独立运行记录，可在工作区查看最新状态。</p>
            </div>
          </div>
          <p className="chapter-plan-help-text">
            生成过程在后台持续运行。您可以关注工作区顶部的状态更新条，或在完成后查看候选批次。
          </p>
        </div>
        <footer className="chapter-plan-dialog-footer">
          <button
            type="button"
            className="chapter-plan-button primary"
            onClick={onClose}
          >
            确定
          </button>
        </footer>
      </div>
    </div>
  );
}

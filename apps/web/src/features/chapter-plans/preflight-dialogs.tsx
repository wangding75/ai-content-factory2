"use client";

import Link from "next/link";
import { Icon } from "@/components/ui/icons";
import { WorkflowPreflightBlocker } from "@/components/workflow-preflight-blocker";
import { toWorkflowPreflightReasons } from "@/components/workflow-preflight-reason";
import { workflowPreflightRepairLink } from "@/components/workflow-preflight-repair-route";
import type {
  ChapterPlanningBlockerItem,
  ChapterPlanningPreflightItem,
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

function preflightCheckLabel(item: ChapterPlanningPreflightItem): string {
  switch (item.code) {
    case "workflow_binding_valid":
    case "workflow_binding_available":
      return "工作流配置连通性正常";
    case "execution_integration_available":
      return "工作流执行连接可用";
    case "storyline_selection_valid":
      return "故事线关联校验通过";
    case "generation_input_valid":
    case "input_contract_valid":
      return "必填输入项与契约校验通过";
    case "chapter_target_valid":
      return "目标章节数量合理";
    default:
      return item.safeReason || "检查项已通过";
  }
}

export interface PreflightProgressDialogProps {
  onClose?: () => void;
}

export function PreflightProgressDialog({ onClose }: PreflightProgressDialogProps) {
  const completedSteps = [
    "读取项目策划与设定",
    "加载故事线树并校验结构",
    "整理角色、素材与伏笔",
  ];
  const pendingSteps = [
    "检查运行冲突与任务锁定",
    "校验输入契约与版本",
    "生成预检摘要",
  ];

  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="preflight-progress-title"
      aria-busy="true"
    >
      <div className="chapter-plan-dialog-content">
        <header className="chapter-plan-dialog-header">
          <h3 id="preflight-progress-title">正在执行章节规划预检</h3>
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
        <div className="chapter-plan-dialog-body preflight-progress-body" aria-live="polite">
          <div className="preflight-progress-heading">
            <div className="chapter-plan-spinner" aria-hidden="true" />
            <div>
              <strong>预检进行中</strong>
              <p>正在确认配置与输入是否满足真实生成条件。</p>
            </div>
          </div>
          <ol className="preflight-progress-steps" aria-label="预检检查项">
            {completedSteps.map((label) => (
              <li key={label} className="complete">
                <span className="preflight-step-marker" aria-hidden="true">✓</span>
                <span>{label}</span>
              </li>
            ))}
            <li className="current" aria-current="step">
              <span className="preflight-step-marker" aria-hidden="true" />
              <span>检查工作流配置与连接</span>
            </li>
            {pendingSteps.map((label) => (
              <li key={label} className="pending">
                <span className="preflight-step-marker" aria-hidden="true" />
                <span>{label}</span>
              </li>
            ))}
          </ol>
          <div className="preflight-progress-notice">
            <Icon name="info" size={18} />
            <p>预检阶段不会创建任务，也不会调用 n8n 或 LLM；通常耗时 5–15 秒。</p>
          </div>
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
  const primaryRepair = blockedReport?.blockers[0]
    ? workflowPreflightRepairLink(
        blockedReport.blockers[0].repairAction,
        blockedReport.blockers[0].repairTarget ?? undefined,
      )
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
          <div>
            <h3 id="preflight-report-title">
              {isPassed ? "确认章节规划任务" : "预检阻断报告"}
            </h3>
            {isPassed && <p className="chapter-plan-dialog-subtitle">确认后将使用本次预检结果创建任务。</p>}
          </div>
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
                  : "预检阻断，暂时无法创建任务"}
              </strong>
              {modeLabel && targetLabel && (
                <p>
                  模式：{modeLabel}（{targetLabel}）
                </p>
              )}
            </div>
          </div>

          {isPassed && passedReport && (
            <div className="chapter-plan-preflight-pass-content">
              <div className="chapter-plan-preflight-pass-grid">
                <section className="chapter-plan-preflight-pass-card">
                  <h4>任务概览</h4>
                  <dl>
                    <div>
                      <dt>生成模式</dt>
                      <dd>{modeLabel}</dd>
                    </div>
                    <div>
                      <dt>目标章节</dt>
                      <dd>{targetLabel}</dd>
                    </div>
                    <div>
                      <dt>故事线</dt>
                      <dd>{inputSummary?.storylineSelection.mode === "specified" ? `指定 ${inputSummary.storylineSelection.storylineIds.length} 条故事线` : "自动平衡全部故事线"}</dd>
                    </div>
                  </dl>
                </section>
                <section className="chapter-plan-preflight-pass-card">
                  <h4>执行配置</h4>
                  <dl>
                    <div>
                      <dt>工作流阶段</dt>
                      <dd>章节规划</dd>
                    </div>
                    <div>
                      <dt>绑定版本</dt>
                      <dd>v{passedReport.executionConfigurationSummary.workflowBindingVersion}</dd>
                    </div>
                    <div>
                      <dt>预期结果</dt>
                      <dd>新的待确认章节候选批次</dd>
                    </div>
                  </dl>
                </section>
              </div>
              <section className="chapter-plan-preflight-pass-card">
                <h4>预检结果详情</h4>
                <ul className="chapter-plan-preflight-checks">
                  {passedReport.checks.map((item, index) => (
                    <li key={`${item.code}-${index}`}>
                      <span aria-hidden="true">✓</span>
                      <div>
                        <strong>{preflightCheckLabel(item)}</strong>
                        {item.safeReason && <small>{item.safeReason}</small>}
                      </div>
                    </li>
                  ))}
                </ul>
              </section>
            </div>
          )}

          {!isPassed && blockedReport && inputSummary && (
            <section className="chapter-plan-preflight-blocked-summary">
              <h4>任务概要</h4>
              <dl>
                <div>
                  <dt>生成模式</dt>
                  <dd>{modeLabel}</dd>
                </div>
                <div>
                  <dt>目标章节</dt>
                  <dd>{targetLabel}</dd>
                </div>
                <div>
                  <dt>预期结果</dt>
                  <dd>新的待确认章节候选批次</dd>
                </div>
              </dl>
            </section>
          )}

          {/* Blockers list */}
          {blockedReport && (
            <div className="chapter-plan-preflight-section">
              <WorkflowPreflightBlocker reasons={toWorkflowPreflightReasons(blockedReport.blockers)} />
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
                预检 Token 有效期为 10 分钟。创建任务后将进入运行队列，已确认章节不会被覆盖。
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
            {isPassed ? "返回修改" : "返回修改"}
          </button>

          {!isPassed && primaryRepair && (
            <Link
              href={primaryRepair.href}
              className="chapter-plan-button primary"
            >
              {primaryRepair.known ? "前往项目配置" : "查看配置"}
            </Link>
          )}

          {isPassed && passedReport && onCreateRun && (
            <button
              type="button"
              className="chapter-plan-button primary"
              onClick={() => onCreateRun(passedReport.preflightToken)}
              disabled={creatingRun}
            >
              {creatingRun ? "正在创建任务..." : "创建任务"}
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

export function RunCreatedDialog({ runId, onClose }: RunCreatedDialogProps) {
  return (
    <div
      className="chapter-plan-dialog-overlay"
      role="dialog"
      aria-modal="true"
      aria-labelledby="run-created-title"
    >
      <div className="chapter-plan-dialog-content">
        <header className="chapter-plan-dialog-header">
          <div>
            <h3 id="run-created-title">任务创建成功</h3>
            <p className="chapter-plan-dialog-subtitle">章节规划任务已提交，正在排队执行。</p>
          </div>
          <button
            type="button"
            className="chapter-plan-dialog-close"
            onClick={onClose}
            aria-label="关闭"
          >
            <Icon name="close" size={18} />
          </button>
        </header>
        <div className="chapter-plan-dialog-body chapter-plan-run-created-body">
          <div className="chapter-plan-status-banner success">
            <Icon name="sparkles" size={20} />
            <div>
              <strong>Run 已创建，当前等待执行</strong>
              <p>任务创建成功不代表章节规划结果已经生成。</p>
            </div>
          </div>
          <dl className="chapter-plan-run-created-details">
            <div>
              <dt>Run ID</dt>
              <dd>{runId}</dd>
            </div>
            <div>
              <dt>当前状态</dt>
              <dd><span className="chapter-plan-run-created-status">已提交 / 排队中</span></dd>
            </div>
          </dl>
          <div className="chapter-plan-run-created-notice">
            <Icon name="info" size={18} />
            <p>可以返回章节规划工作区查看运行状态；任务完成后结果进入候选批次，不直接覆盖当前章节。</p>
          </div>
        </div>
        <footer className="chapter-plan-dialog-footer">
          <button
            type="button"
            className="chapter-plan-button primary"
            onClick={onClose}
          >
            返回工作区
          </button>
          <Link href={`/workflow-runs/${encodeURIComponent(runId)}`} className="chapter-plan-run-created-detail-link">
            查看运行详情
          </Link>
        </footer>
      </div>
    </div>
  );
}

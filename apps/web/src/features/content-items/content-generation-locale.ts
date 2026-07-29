export const contentGenerationCopy = {
  titles: { queued: "排队中", running: "运行中", candidate_ready: "候选版本已创建", not_configured: "尚未配置正文生成工作流", failed: "正文生成未完成" },
  queuedDetail: "正文生成任务已进入队列，等待执行。", runningDetail: "正文生成正在执行，进度以运行事件为准。", candidateDetail: (version: number | undefined) => `候选版本 v${version ?? "—"} 已创建，当前正文尚未被替换。`, notConfiguredDetail: "请在项目设置中绑定正文生成工作流，并确认执行连接可用。",
  safeFailure: "任务未能完成，当前正文已保留。", retryFailure: "重试未能完成，当前正文已保留，请稍后再试。", eventFallback: "运行状态已更新", eventEmpty: "暂无可公开的运行事件。",
  events: { queued: "已进入队列", worker_started: "开始执行", request_sent: "已发送生成请求", response_received: "已收到生成结果", output_validated: "输出已校验", result_consumed: "候选版本已创建", result_consumption_failed: "候选版本创建失败", succeeded: "任务完成", failed: "任务失败", cancelled: "任务已取消", retry_created: "已创建重试任务" } as Record<string, string>,
  viewCandidate: "查看候选", viewDetails: "查看详情", hideDetails: "收起详情", workflowCenter: "前往流程中心", configureWorkflow: "配置工作流", retryRuntime: "重新执行 Runtime", retryConsumption: "重试结果消费", retrying: "正在重试…",
  candidateCompareAria: "候选版本比较", candidateViewing: (version: number) => `当前查看：v${version}（候选）`, sourceRun: (run: string, version: number) => `来源 Run：${run}；基线版本：v${version}`,
  viewCurrent: (version: number) => `查看当前 v${version}`, viewCandidateVersion: (version: number) => `查看候选 v${version}`, closeCandidate: "关闭候选", setCurrent: "设为当前版本", settingCurrent: "正在设为当前…",
  stale: "当前正文已变化；候选仍可比较，但不能强制覆盖。", currentReadFailed: "无法读取当前版本，请稍后重试。", currentReading: "正在读取当前版本…", switchFailed: "切换版本失败，当前正文未被覆盖，请稍后重试。", confirmSetCurrent: "确认将候选版本设为当前正文吗？当前版本会保留在版本历史中。",
  candidateTitle: (version: number) => `候选版本 v${version}`, currentTitle: (version: number | undefined) => `当前版本 v${version ?? "—"}`,
};

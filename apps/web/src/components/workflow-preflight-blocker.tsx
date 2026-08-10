import Link from "next/link";
import { workflowPreflightRepairLink, type WorkflowPreflightRepairTarget } from "./workflow-preflight-repair-route";

export type WorkflowPreflightBlockerReason = { code: string; message: string; repairAction?: string; retryAction?: string; safeReason?: string } & WorkflowPreflightRepairTarget;

function blockerTitle(code: string) {
  return ({
    project_binding_missing: "项目工作流绑定缺失",
    execution_integration_unavailable: "执行连接不可用",
    workflow_configuration_stale: "工作流配置需要重新验证",
    provider_unavailable: "Provider 或模型不可执行",
    input_contract_invalid: "输入契约校验未通过",
  } as Record<string, string>)[code] ?? "预检阻断项";
}

function repairDirection(action?: string) {
  return ({
    "workflow_binding:configure": "前往项目设置完成工作流绑定",
    "workflow_binding:repair": "修复项目工作流绑定后重新预检",
    "workflow_configuration:verify": "重新验证工作流配置",
    "connection:verify": "重新验证执行连接",
    "connection:enable": "启用执行连接后重新预检",
    "provider:verify": "验证 Provider 与模型可执行性",
    retry_after_integration_recovers: "待执行服务恢复后重新预检",
    retry_preflight: "修复阻断项后重新预检",
  } as Record<string, string>)[action ?? ""] ?? "修正阻断项后重新预检";
}

export function WorkflowPreflightBlocker({ reasons }: { reasons: WorkflowPreflightBlockerReason[] }) {
  if (!reasons.length) return null;
  return <section className="workflow-preflight-blocker" role="alert" aria-labelledby="workflow-preflight-blocker-title"><header><span>预检阻断</span><div><h2 id="workflow-preflight-blocker-title">当前无法启动工作流</h2><p>请先处理以下阻断项；页面不会发送执行请求。</p></div></header><ul>{reasons.map((reason, index) => { const repair = workflowPreflightRepairLink(reason.repairAction, reason); return <li key={`${reason.message}-${index}`}><div><strong>{blockerTitle(reason.code)}</strong><p>{reason.message}</p>{reason.safeReason && <small>{reason.safeReason}</small>}<small>建议操作：{repairDirection(reason.retryAction ?? reason.repairAction)}</small></div><Link href={repair.href}>{repair.known ? "前往修复" : "查看配置"}</Link></li>; })}</ul></section>;
}

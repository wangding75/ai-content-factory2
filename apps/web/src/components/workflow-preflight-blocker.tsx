import Link from "next/link";
import { workflowPreflightRepairLink, type WorkflowPreflightRepairTarget } from "./workflow-preflight-repair-route";

export type WorkflowPreflightBlockerReason = { code: string; message: string; repairAction?: string } & WorkflowPreflightRepairTarget;

export function WorkflowPreflightBlocker({ reasons }: { reasons: WorkflowPreflightBlockerReason[] }) {
  if (!reasons.length) return null;
  return <section className="workflow-preflight-blocker" role="alert"><h2>当前无法启动工作流</h2><p>请先处理以下配置问题；页面不会发送执行请求。</p><ul>{reasons.map((reason, index) => { const repair = workflowPreflightRepairLink(reason.repairAction, reason); return <li key={`${reason.message}-${index}`}>{reason.message}<Link href={repair.href}>{repair.known ? "前往修复" : "查看配置"}</Link></li>; })}</ul></section>;
}

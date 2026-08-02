import Link from "next/link";

export type WorkflowPreflightBlockerReason = { message: string; repairAction?: string };

const repairHref = (action?: string) => {
  if (action === "configure_provider") return "/settings";
  if (action === "configure_connection" || action === "configure_workflow") return "/settings?tab=connections";
  return undefined;
};

export function WorkflowPreflightBlocker({ reasons }: { reasons: WorkflowPreflightBlockerReason[] }) {
  if (!reasons.length) return null;
  return <section className="workflow-preflight-blocker" role="alert"><h2>当前无法启动工作流</h2><p>请先处理以下配置问题；页面不会发送执行请求。</p><ul>{reasons.map((reason, index) => { const href = repairHref(reason.repairAction); return <li key={`${reason.message}-${index}`}>{reason.message}{href && <Link href={href}>前往修复</Link>}</li>; })}</ul></section>;
}

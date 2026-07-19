import Link from "next/link";
import type { ReactNode } from "react";
import "./global-settings-tabs.css";

export type SettingsSection = "llm" | "workflow" | "distribution";
export type WorkflowSection = "connections" | "workflows";

export function GlobalSettingsWorkspace({ section, workflowSection, children }: { section: SettingsSection; workflowSection?: WorkflowSection; children: ReactNode }) {
  return <div className="global-settings-workspace"><header className="global-settings-heading"><p>设置</p><h1>全局设置</h1></header><nav className="global-settings-tabs" aria-label="全局配置分类"><Link className={section === "llm" ? "active" : ""} href="/settings">LLM 配置</Link><Link className={section === "workflow" ? "active" : ""} href="/settings?tab=connections">工作流配置</Link><Link className={section === "distribution" ? "active" : ""} href="/settings?tab=distribution">分发平台配置</Link></nav>{section === "workflow" && <nav className="workflow-subtabs" aria-label="工作流配置分类"><Link className={workflowSection === "connections" ? "active" : ""} href="/settings?tab=connections">连接</Link><Link className={workflowSection === "workflows" ? "active" : ""} href="/settings?tab=workflows">工作流</Link></nav>}<div className="global-settings-body">{children}</div></div>;
}

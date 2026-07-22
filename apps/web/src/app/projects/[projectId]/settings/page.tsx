import Link from "next/link";
import { AppShell } from "@/components/ui/app-shell";
import { getProjectWorkspace } from "@/lib/api";
import { ProjectWorkspaceFrame } from "@/features/planning-materials/components/project-workspace-frame";
import { WorkflowBindingsPage } from "@/features/workflow-bindings/workflow-bindings-page";

export default async function ProjectSettingsPage({ params, searchParams }: { params: Promise<{ projectId: string }>; searchParams: Promise<{ tab?: string }> }) {
  const { projectId } = await params;
  const { tab } = await searchParams;
  const project = await getProjectWorkspace(projectId).then(({ project }) => project).catch(() => null);
  if (!project) return <AppShell active="projects"><main className="project-works-state error"><h1>暂时无法加载项目</h1><p>请稍后重试。</p><Link href="/projects">返回项目列表</Link></main></AppShell>;
  const workflowTab = tab === "workflow-bindings";
  return <AppShell active="projects"><ProjectWorkspaceFrame project={project} active="settings"><main className="project-settings-page"><nav className="project-settings-tabs" aria-label="项目设置"><Link href={`/projects/${projectId}/settings`}>基本信息</Link><Link className={workflowTab ? "active" : ""} href={`/projects/${projectId}/settings?tab=workflow-bindings`}>工作流绑定</Link><button type="button" disabled>其他设置</button></nav>{workflowTab ? <WorkflowBindingsPage projectId={projectId} /> : <section className="project-settings-placeholder"><h2>基本信息</h2><p>项目基础信息将在后续版本开放编辑。</p><Link href={`/projects/${projectId}`}>返回项目概览</Link></section>}</main></ProjectWorkspaceFrame></AppShell>;
}

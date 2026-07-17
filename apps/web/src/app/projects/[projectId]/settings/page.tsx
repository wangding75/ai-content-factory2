import Link from "next/link";
import { AppShell } from "@/components/ui/app-shell";
import { getProjectWorkspace } from "@/lib/api";
import { ProjectWorkspaceFrame } from "@/features/planning-materials/components/project-workspace-frame";

export default async function ProjectSettingsPage({ params }: { params: Promise<{ projectId: string }> }) {
  const { projectId } = await params;
  const project = await getProjectWorkspace(projectId).then(({ project }) => project).catch(() => null);
  if (!project) return <AppShell active="projects"><main className="project-works-state error"><h1>暂时无法加载项目</h1><p>请稍后重试。</p><Link href="/projects">返回项目列表</Link></main></AppShell>;
  return <AppShell active="projects"><ProjectWorkspaceFrame project={project} active="settings"><main className="project-settings-page"><section><h2>项目设置</h2><span>暂未开放</span><p>项目级生成规则、成员与高级配置将在后续版本开放。</p><div><Link href={`/projects/${projectId}`}>返回项目概览</Link><Link href="/settings">前往全局设置</Link></div></section></main></ProjectWorkspaceFrame></AppShell>;
}

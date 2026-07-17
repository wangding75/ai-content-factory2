import Link from "next/link";
import type { Project, ProjectStatus, ProjectType } from "@/lib/api";
import { ProjectWorkspaceNav, type ProjectWorkspaceTab } from "./project-workspace-nav";

const typeLabels: Record<ProjectType, string> = { novel: "小说" };
const statusLabels: Record<ProjectStatus, string> = { planning: "策划中", producing: "制作中", archived: "已归档" };

export function ProjectWorkspaceFrame({ project, active, children }: { project: Project; active: ProjectWorkspaceTab; children: React.ReactNode }) {
  return <div className="project-workspace-frame"><header className="project-workspace-header"><div className="project-workspace-canvas"><nav className="project-workspace-breadcrumb"><Link href="/projects">项目</Link><span aria-hidden="true">/</span><span>{project.name}</span></nav><div className="project-workspace-title"><div><h1>{project.name}</h1><span>{typeLabels[project.type]}</span><b>{statusLabels[project.status]}</b></div><time>更新于 {new Date(project.updated_at).toLocaleString("zh-CN", { dateStyle: "medium", timeStyle: "short" })}</time></div><ProjectWorkspaceNav projectId={project.id} active={active} /></div></header>{children}</div>;
}

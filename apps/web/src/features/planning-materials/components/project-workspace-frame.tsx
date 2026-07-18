import Link from "next/link";
import type { Project } from "@/lib/api";
import { projectStatusLabel, projectTypeLabel } from "@/components/projects/project-presentation";
import { ProjectWorkspaceNav, type ProjectWorkspaceTab } from "./project-workspace-nav";


export function ProjectWorkspaceFrame({ project, active, children, variant = "standard" }: { project: Project; active: ProjectWorkspaceTab; children: React.ReactNode; variant?: "standard" | "wide" }) {
  return <div className={`project-workspace-frame ${variant}`}><header className="project-workspace-header"><div className="project-workspace-canvas"><nav className="project-workspace-breadcrumb"><Link href="/projects">项目</Link><span aria-hidden="true">/</span><span>{project.name}</span></nav><div className="project-workspace-title"><div><h1>{project.name}</h1><span>{projectTypeLabel(project.type)}</span><b>{projectStatusLabel(project.status)}</b></div><time>更新于 {new Date(project.updated_at).toLocaleString("zh-CN", { dateStyle: "medium", timeStyle: "short" })}</time></div><p className="project-workspace-description">{project.description || " "}</p><ProjectWorkspaceNav projectId={project.id} active={active} /></div></header><div className="project-workspace-content">{children}</div></div>;
}

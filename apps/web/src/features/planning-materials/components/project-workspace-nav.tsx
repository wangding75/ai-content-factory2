import Link from "next/link";

export type ProjectWorkspaceTab = "overview" | "planning" | "materials" | "storylines" | "chapter-plans" | "review" | "works" | "settings";

const items: Array<{ key: ProjectWorkspaceTab; label: string; href: (id: string) => string }> = [
  { key: "overview", label: "概览", href: (id) => `/projects/${id}` },
  { key: "planning", label: "策划", href: (id) => `/projects/${id}/planning` },
  { key: "materials", label: "素材", href: (id) => `/projects/${id}/materials` },
  { key: "storylines", label: "故事线", href: (id) => `/projects/${id}/storylines` },
  { key: "chapter-plans", label: "章节规划", href: (id) => `/projects/${id}/chapter-plans` },
  { key: "review", label: "审核", href: (id) => `/projects/${id}/works?view=review` },
  { key: "works", label: "作品", href: (id) => `/projects/${id}/works` },
  { key: "settings", label: "设置", href: (id) => `/projects/${id}/settings` },
];

export function ProjectWorkspaceNav({ projectId, active }: { projectId: string; active: ProjectWorkspaceTab }) {
  return <nav className="workspace-tabs" aria-label="项目工作区导航">{items.map((item) => <Link className={active === item.key ? "is-active" : ""} href={item.href(projectId)} key={item.key}>{item.label}</Link>)}</nav>;
}

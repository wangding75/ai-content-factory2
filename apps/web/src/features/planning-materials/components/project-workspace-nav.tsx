import Link from "next/link";

const futureItems = ["故事线", "章节", "审核", "作品", "设置"];

export function ProjectWorkspaceNav({ projectId, active }: { projectId: string; active: "overview" | "planning" | "materials" }) {
  return <nav className="workspace-tabs" aria-label="项目工作区导航">
    <Link className={active === "overview" ? "is-active" : ""} href={`/projects/${projectId}`}>概览</Link>
    <Link className={active === "planning" ? "is-active" : ""} href={`/projects/${projectId}/planning`}>策划</Link>
    <Link className={active === "materials" ? "is-active" : ""} href={`/projects/${projectId}/materials`}>素材</Link>
    {futureItems.map((item) => <span className="is-disabled" aria-disabled="true" key={item}>{item}</span>)}
  </nav>;
}

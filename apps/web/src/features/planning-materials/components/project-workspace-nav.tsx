import Link from "next/link";

const futureItems = ["Review", "Settings"];

export function ProjectWorkspaceNav({ projectId, active }: { projectId: string; active: "overview" | "planning" | "materials" | "storylines" | "chapter-plans" | "works" }) {
  return <nav className="workspace-tabs" aria-label="Project workspace navigation">
    <Link className={active === "overview" ? "is-active" : ""} href={`/projects/${projectId}`}>Overview</Link>
    <Link className={active === "planning" ? "is-active" : ""} href={`/projects/${projectId}/planning`}>Planning</Link>
    <Link className={active === "materials" ? "is-active" : ""} href={`/projects/${projectId}/materials`}>Materials</Link>
    <Link className={active === "storylines" ? "is-active" : ""} href={`/projects/${projectId}/storylines`}>Storylines</Link>
    <Link className={active === "chapter-plans" ? "is-active" : ""} href={`/projects/${projectId}/chapter-plans`}>章节规划</Link>
    <Link className={active === "works" ? "is-active" : ""} href={`/projects/${projectId}/works`}>作品</Link>
    {futureItems.map((item) => <span className="is-disabled" aria-disabled="true" key={item}>{item}</span>)}
  </nav>;
}

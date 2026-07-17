"use client";
/* eslint-disable react-hooks/set-state-in-effect */
import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ApiError, type Project } from "@/lib/api";
import { ProjectWorkspaceFrame } from "@/features/planning-materials/components/project-workspace-frame";
import { listProjectWorks, type ProjectWorkListDto } from "./project-work-api";
import {
  projectWorksRoutes,
  toProjectWorkView,
  type ProjectWorkView,
} from "./project-work-presentation";

const limit = 20;
export function ProjectWorksWorkspace({
  projectId,
  project,
  view = "works",
}: {
  projectId: string;
  project: Project;
  view?: "works" | "review";
}) {
  const [data, setData] = useState<ProjectWorkListDto | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [reload, setReload] = useState(0);
  const [offset, setOffset] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setData(null);
    setError(null);
    void listProjectWorks(projectId, { limit, offset, signal: controller.signal })
      .then((works) => {
        setData(works);
      })
      .catch((cause) => {
        if (!controller.signal.aborted) setError(cause as ApiError);
      });
    return () => controller.abort();
  }, [projectId, reload, offset]);
  const works = useMemo(() => data?.items.map(toProjectWorkView) ?? [], [data]);
  const retry = useCallback(() => setReload((value) => value + 1), []);
  if (error)
    return (
      <ProjectWorkspaceFrame project={project} active={view}>
        <WorksState
          kind={error.status === 404 ? "not-found" : "error"}
          projectId={projectId}
          retry={retry}
        />
      </ProjectWorkspaceFrame>
    );
  if (!data)
    return (
      <ProjectWorkspaceFrame project={project} active={view}>
        <WorksState kind="loading" projectId={projectId} />
      </ProjectWorkspaceFrame>
    );
  if (view === "review")
    return (
      <ProjectWorkspaceFrame project={project} active="review">
        <ReviewList projectId={projectId} works={works} />
      </ProjectWorkspaceFrame>
    );
  if (!data.total)
    return (
      <ProjectWorkspaceFrame project={project} active="works">
        <WorksState kind="empty" projectId={projectId} />
      </ProjectWorkspaceFrame>
    );
  return (
    <ProjectWorkspaceFrame project={project} active="works">
      <main className="project-works">
        <p className="project-works-intro">
          查看项目中已保存的章节正文、版本和审核结果。
        </p>
        <section className="project-works-summary">
          <Stat label="章节作品" value={data.total} />
          <Stat
            label="正文版本"
            value={works.reduce((sum, work) => sum + work.version_count, 0)}
          />
          <Stat
            label="已有审核"
            value={works.filter((work) => work.latest_review).length}
          />
          <Stat
            label="已完成"
            value={
              works.filter(
                (work) => work.latest_workflow_run?.status === "succeeded",
              ).length
            }
          />
        </section>
        <section className="project-works-list">
          <header>
            <h2>章节列表</h2>
            <Link href={`/projects/${projectId}/chapter-plans`}>
              返回章节规划
            </Link>
          </header>
          {works.map((work) => (
            <article className="project-work-card" key={work.work_id}>
              <span className="project-work-badge">{work.reviewLabel}</span>
              <strong>{work.chapterLabel}</strong>
              <span>当前版本：v{work.current_version.version_no}</span>
              <span>版本数量：{work.version_count}</span>
              <WorkLinks projectId={projectId} work={work} />
            </article>
          ))}
          <footer className="project-works-pagination">
            <button
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - limit))}
            >
              上一页
            </button>
            <button
              disabled={offset + limit >= data.total}
              onClick={() => setOffset(offset + limit)}
            >
              下一页
            </button>
          </footer>
        </section>
      </main>
    </ProjectWorkspaceFrame>
  );
}
function Stat({ label, value }: { label: string; value: number }) {
  return (
    <article>
      <span>{label}</span>
      <strong>{value}</strong>
    </article>
  );
}
function WorkLinks({
  projectId,
  work,
}: {
  projectId: string;
  work: ProjectWorkView;
}) {
  const routes = projectWorksRoutes(projectId, work);
  return (
    <div className="project-work-actions">
      <Link href={routes.editor}>打开正文</Link>
      {routes.review && <Link href={routes.review}>查看审核</Link>}
      {routes.rewrite && <Link href={routes.rewrite}>创建重写版本</Link>}
    </div>
  );
}
function ReviewList({
  projectId,
  works,
}: {
  projectId: string;
  works: ProjectWorkView[];
}) {
  const reviewed = works.filter((work) => work.latest_review);
  if (!reviewed.length)
    return (
      <main className="project-works-state review-empty">
        <h1>暂无审核记录</h1>
        <p>完成章节正文并发起审核后，审核结果将在这里显示。</p>
        <div>
          <Link href={`/projects/${projectId}/works`}>前往作品</Link>
          <Link href={`/projects/${projectId}/chapter-plans`}>
            前往章节规划
          </Link>
        </div>
      </main>
    );
  return (
    <main className="project-review-list">
      <h2>审核</h2>
      {reviewed.map((work) => {
        const route = projectWorksRoutes(projectId, work);
        return (
          <article key={work.work_id}>
            <h3>{work.chapterLabel}</h3>
            <p>
              审核结论：
              {work.latest_review?.conclusion === "pass" ? "通过" : "需修改"} ·
              评分：{work.latest_review?.score}
            </p>
            <p>{work.latest_review?.summary}</p>
            <time>
              {new Date(work.latest_review!.created_at).toLocaleString("zh-CN")}
            </time>
            {route.review && <Link href={route.review}>查看审核</Link>}
          </article>
        );
      })}
    </main>
  );
}
function WorksState({
  kind,
  retry,
  projectId,
}: {
  kind: "loading" | "empty" | "error" | "not-found";
  retry?: () => void;
  projectId: string;
}) {
  const text = {
    loading: ["正在加载作品", "正在读取项目作品和版本信息。"],
    empty: ["暂无项目作品", "项目中尚未保存章节正文。"],
    error: ["作品列表加载失败", "请检查网络后重试。"],
    "not-found": ["项目不存在或无权限", "请确认项目仍存在，或返回项目列表。"],
  }[kind];
  return (
    <main
      className={`project-works-state ${kind}`}
      aria-busy={kind === "loading"}
    >
      <h1>{text[0]}</h1>
      <p>{text[1]}</p>
      {retry && <button onClick={retry}>重试</button>}
      {kind === "empty" && (
        <div>
          <Link href={`/projects/${projectId}/chapter-plans`}>
            前往章节规划
          </Link>
          <Link href={`/projects/${projectId}`}>返回项目概览</Link>
        </div>
      )}
    </main>
  );
}

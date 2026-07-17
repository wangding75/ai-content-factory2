/* eslint-disable react-hooks/error-boundaries */
import Link from "next/link";
import { AppShell } from "@/components/ui/app-shell";
import { Icon } from "@/components/ui/icons";
import {
  ApiError,
  getProjectWorkspace,
  type ProjectStage,
  type ProjectStatus,
} from "@/lib/api";
import { ProjectWorkspaceFrame } from "@/features/planning-materials/components/project-workspace-frame";
const stages: Record<ProjectStage, string> = {
  project_setup: "项目准备",
  project_planning: "项目策划",
  materials: "项目素材",
  storylines: "故事线",
  chapter_planning: "章节规划",
  content_production: "内容生产",
  review: "审核",
  completed: "已完成",
};
const statuses: Record<ProjectStatus, string> = {
  planning: "策划中",
  producing: "制作中",
  archived: "已归档",
};
const date = (v: string) =>
  new Date(v).toLocaleString("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  });
function ErrorState({
  id,
  title,
  desc,
  retry,
}: {
  id: string;
  title: string;
  desc: string;
  retry?: boolean;
}) {
  return (
    <AppShell active="projects">
      <main className="overview-error-main">
        <section className="overview-error" role="alert">
          <span>
            <Icon name="info" size={30} />
          </span>
          <h1>{title}</h1>
          <p>{desc}</p>
          <div>
            <Link href="/projects">返回项目列表</Link>
            {retry && <Link href={`/projects/${id}`}>重试</Link>}
          </div>
        </section>
      </main>
    </AppShell>
  );
}
export default async function ProjectOverviewPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId: id } = await params;
  if (
    !/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(
      id,
    )
  )
    return (
      <ErrorState
        id={id}
        title="项目地址无效"
        desc="请从项目列表中选择一个有效项目。"
      />
    );
  try {
    const { project, progress } = await getProjectWorkspace(id);
    const cards = [
      ["archive", progress.material_count, "项目素材"],
      ["timeline", progress.storyline_count, "故事线节点"],
      ["book", progress.confirmed_chapter_count, "已确认章节"],
      ["movie", progress.work_count, "发布作品"],
    ] as const;
    return (
      <AppShell active="projects"><ProjectWorkspaceFrame project={project} active="overview">
        <main className="overview-main">
          <div className="overview-canvas overview-grid">
            <div className="overview-left">
              <section className="overview-summary">
                <div className="overview-summary-heading">
                  <div>
                    <h2>项目摘要</h2>
                    <p>
                      {project.description ||
                        "尚未填写项目简介。完成项目策划后，这里会展示项目主题与创作方向。"}
                    </p>
                  </div>
                  <Link
                    className="overview-planning-link"
                    href={"/projects/" + id + "/planning"}
                  >
                    <Icon name="edit" size={18} />
                    完善项目策划
                  </Link>
                </div>
                <dl className="overview-info-grid">
                  <div>
                    <dt>项目类型</dt>
                    <dd>小说</dd>
                  </div>
                  <div>
                    <dt>当前状态</dt>
                    <dd>{statuses[project.status]}</dd>
                  </div>
                  <div>
                    <dt>当前阶段</dt>
                    <dd>{stages[project.current_stage]}</dd>
                  </div>
                  <div>
                    <dt>创建时间</dt>
                    <dd>{date(project.created_at)}</dd>
                  </div>
                  <div>
                    <dt>更新时间</dt>
                    <dd>{date(project.updated_at)}</dd>
                  </div>
                </dl>
              </section>
              <section>
                <h2 className="overview-section-title">当前进度</h2>
                <div className="overview-progress-grid">
                  {cards.map(([icon, value, label]) => (
                    <div className="overview-progress-card" key={label}>
                      <Icon name={icon} size={24} />
                      <strong>{value}</strong>
                      <span>{label}</span>
                    </div>
                  ))}
                </div>
              </section>
              <section className="overview-visual-empty">
                <span>
                  <Icon name="image" size={32} />
                </span>
                <h2>概念视觉暂无数据</h2>
                <p>添加真实项目素材后，这里会展示项目世界观参考图。</p>
              </section>
            </div>
            <aside className="overview-right">
              <section className="overview-next">
                <Icon
                  name="lightbulb"
                  size={104}
                  className="overview-next-mark"
                />
                <div>
                  <h2>下一步建议</h2>
                  <p>
                    项目处于初始阶段。完成详细的策划方案后，将解锁更多创作功能。
                  </p>
                  <Link href={"/projects/" + id + "/planning"}>
                    去完善策划 <Icon name="arrowRight" size={18} />
                  </Link>
                  <Link
                    className="overview-materials-link"
                    href={"/projects/" + id + "/materials"}
                  >
                    添加项目素材
                  </Link>
                </div>
              </section>
              <section className="overview-activity">
                <h2>最近活动</h2>
                <div className="overview-activity-empty">
                  <span />
                  <div>
                    <p>暂无活动记录</p>
                    <small>项目创建后的真实活动将在这里显示</small>
                  </div>
                </div>
              </section>
              <section className="overview-meta">
                <div>
                  <span>最后编辑于</span>
                  <strong>{date(project.updated_at)}</strong>
                </div>
                <div>
                  <span>项目成员</span>
                  <strong>暂无成员数据</strong>
                </div>
              </section>
            </aside>
          </div>
        </main>
      </ProjectWorkspaceFrame></AppShell>
    );
  } catch (error) {
    if (error instanceof ApiError && error.status === 404)
      return (
        <ErrorState
          id={id}
          title="项目不存在"
          desc="该项目可能已被删除，或项目地址不正确。"
        />
      );
    return (
      <ErrorState
        id={id}
        title="暂时无法加载项目"
        desc="网络或服务暂时不可用，请稍后重试。"
        retry
      />
    );
  }
}

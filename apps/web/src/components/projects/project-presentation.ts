import type { ApiError, Project, ProjectStage, ProjectStatus, ProjectType, ProjectTypeDescriptor } from "@/lib/api";

export type ProjectCardVm = { id: string; href: string; name: string; typeLabel: string; statusLabel: string; stageLabel: string; updatedLabel: string; description: string };
export type ProjectTypeVm = { code: ProjectType; name: string; description: string };

const typeLabels: Record<ProjectType, string> = { novel: "小说", short_film: "短片", series: "剧集", graphic_text: "图文", image: "图片" };
const statusLabels: Record<ProjectStatus, string> = { planning: "策划中", producing: "生产中", archived: "已归档" };
const stageLabels: Record<ProjectStage, string> = { project_setup: "项目准备", project_planning: "项目策划", materials: "项目素材", storylines: "故事线", chapter_planning: "章节规划", content_production: "内容生产", review: "审核", completed: "已完成" };
export const projectTypeLabel = (value: ProjectType) => typeLabels[value] ?? "其他项目";
export const projectStatusLabel = (value: ProjectStatus) => statusLabels[value] ?? "状态待确认";

const formatTime = (value: string) => {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "最近更新" : new Intl.DateTimeFormat("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit", timeZone: "Asia/Shanghai" }).format(date);
};

export const mapProjectCard = (project: Project): ProjectCardVm => ({ id: project.id, href: `/projects/${project.id}`, name: project.name, typeLabel: projectTypeLabel(project.type), statusLabel: projectStatusLabel(project.status), stageLabel: stageLabels[project.current_stage] ?? "准备中", updatedLabel: formatTime(project.updated_at), description: project.description.trim() || "暂无项目简介" });
export const mapProjectTypes = (items: ProjectTypeDescriptor[]): ProjectTypeVm[] => items.filter((item) => item.enabled).sort((a, b) => a.sort_order - b.sort_order).map((item) => ({ code: item.code, name: item.name || "项目类型", description: item.description || "开始新的内容创作项目。" }));
export const projectErrorMessage = (error: ApiError | undefined) => {
  if (!error) return "暂时无法完成请求，请稍后重试。";
  return ({ network_error: "网络连接失败，请检查后重试。", timeout: "请求超时，请稍后重试。", validation_error: "填写内容不符合要求，请修改后重试。", cancelled: "请求已取消，请重新尝试。" } as Record<string, string>)[error.code] ?? "暂时无法完成请求，请稍后重试。";
};

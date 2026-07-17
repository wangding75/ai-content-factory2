import type { ChapterPlan, ChapterPlanStatus } from "./chapter-plan-http-api";
import type { Foreshadowing, StorylineNode } from "@/lib/api";
import type { ProjectMaterialItem } from "@/features/planning-materials/contracts/materials";

export const chapterPlanStatusLabels: Record<ChapterPlanStatus, string> = {
  pending_confirmation: "待确认",
  confirmed: "已确认",
};

const additionalStatusLabels: Record<string, string> = {
  generated: "已生成",
  generating: "生成中",
  failed: "生成失败",
  draft: "草稿",
  mock_generated: "模拟生成",
  manual: "手动创建",
};

export function chapterPlanStatusLabel(status: string): string {
  return chapterPlanStatusLabels[status as ChapterPlanStatus] ?? additionalStatusLabels[status] ?? "未知状态";
}

export function chapterPlanSourceLabel(source: string): string {
  return source === "mock_generated" ? "模拟生成" : "手动创建";
}

export function chapterPlanSummary(value: string | null | undefined): string {
  return value?.trim() || "暂无章节规划摘要";
}

export function chapterPlanGenerationSummary(plan: ChapterPlan): string {
  return chapterPlanSourceLabel(plan.source);
}

type RelationNames = { storylines: Map<string, string>; materials: Map<string, string>; foreshadowings: Map<string, string> };
const flatten = (nodes: StorylineNode[]): StorylineNode[] => nodes.flatMap((node) => [node, ...flatten(node.children)]);

export function createRelationNames(
  storylines: StorylineNode[],
  materials: ProjectMaterialItem[],
  foreshadowings: Foreshadowing[],
): RelationNames {
  return {
    storylines: new Map(flatten(storylines).map((item) => [item.id, item.name])),
    materials: new Map(materials.map((item) => [item.material.id, item.material.name])),
    foreshadowings: new Map(foreshadowings.map((item) => [item.id, item.title])),
  };
}

export function relationValues(ids: string[], names: Map<string, string>, empty: string): string[] {
  const values = ids.map((id) => names.get(id)).filter((value): value is string => Boolean(value));
  return values.length ? values : [empty];
}

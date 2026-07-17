import type { ChapterPlan, ChapterPlanStatus } from "./chapter-plan-http-api";
import type { Foreshadowing, StorylineNode } from "@/lib/api";
import type { ProjectMaterialItem } from "@/features/planning-materials/contracts/materials";

export type ChapterPlanFilterStatus =
  | "all"
  | ChapterPlanStatus
  | "draft_generated";
export const chapterPlanStatusLabels: Record<ChapterPlanStatus, string> = {
  pending_confirmation: "待确认",
  confirmed: "已确认",
};
export function chapterPlanStatusLabel(status: string): string {
  return (
    chapterPlanStatusLabels[status as ChapterPlanStatus] ??
    (status === "generated" || status === "draft_generated"
      ? "已生成草稿"
      : "未知状态")
  );
}
export function chapterPlanSourceLabel(source: string): string {
  return source === "mock_generated" ? "模拟生成" : "手动创建";
}
export function chapterPlanSummary(value: string | null | undefined): string {
  const summary = value?.trim();
  if (!summary || summary.includes("???")) return "暂无章节规划摘要";
  if (
    /\b(main|children|materials|unpaid_foreshadowings|prior_summaries)=/.test(
      summary,
    )
  ) {
    const pace = summary.match(/\b(slow|medium|fast) pace/)?.[1];
    const label =
      pace === "slow" ? "慢节奏" : pace === "fast" ? "快节奏" : "中等节奏";
    const includes = [
      ["main=true", "主线"],
      ["children=true", "支线"],
      ["materials=true", "项目素材"],
      ["unpaid_foreshadowings=true", "未回收伏笔"],
      ["prior_summaries=true", "前文摘要"],
    ]
      .filter(([key]) => summary.includes(key))
      .map(([, text]) => text);
    return includes.length
      ? `${label}推进，并参考${includes.join("、")}。`
      : `${label}推进章节内容。`;
  }
  return summary;
}
export function chapterPlanDetail(
  value: string | null | undefined,
  fallback: string,
): string {
  const detail = value?.trim();
  return detail && !detail.includes("???") ? detail : fallback;
}
export function flattenStorylines(nodes: StorylineNode[]): StorylineNode[] {
  return nodes.flatMap((node) => [node, ...flattenStorylines(node.children)]);
}
export function createRelationNames(
  storylines: StorylineNode[],
  materials: ProjectMaterialItem[],
  foreshadowings: Foreshadowing[],
) {
  return {
    storylines: new Map(
      flattenStorylines(storylines).map((item) => [item.id, item.name]),
    ),
    materials: new Map(
      materials.map((item) => [item.material.id, item.material.name]),
    ),
    foreshadowings: new Map(
      foreshadowings.map((item) => [item.id, item.title]),
    ),
  };
}
export function relationValues(
  ids: string[],
  names: Map<string, string>,
  empty: string,
): string[] {
  const values = ids
    .map((id) => names.get(id))
    .filter((value): value is string => Boolean(value));
  return values.length ? values : [empty];
}
export function createChapterPlanStats(plans: ChapterPlan[]) {
  return {
    all: plans.length,
    pending: plans.filter((plan) => plan.status === "pending_confirmation")
      .length,
    confirmed: plans.filter((plan) => plan.status === "confirmed").length,
    draftGenerated: plans.filter(
      (plan) =>
        (plan.status as string) === "generated" ||
        (plan.status as string) === "draft_generated",
    ).length,
  };
}

export type ConfirmationCheck = {
  label: string;
  status: "success" | "error" | "warning";
  detail: string;
};
export type ConfirmationViewModel = {
  selectedLabel: string;
  rangeLabel: string;
  sourceLabel: string;
  checks: ConfirmationCheck[];
  warnings: string[];
  canConfirm: boolean;
};

function chapterNumbersLabel(numbers: number[]) {
  const sorted = [...numbers].sort((a, b) => a - b);
  if (sorted.length === 1) return `第 ${sorted[0]} 章`;
  const contiguous = sorted.every(
    (value, index) => index === 0 || value === sorted[index - 1] + 1,
  );
  return contiguous
    ? `第 ${sorted[0]}–${sorted.at(-1)} 章`
    : `第 ${sorted.join("、")} 章`;
}

/** Derives every confirmation message from the current selection instead of JSX literals. */
export function createConfirmationViewModel(
  selected: ChapterPlan[],
  allPlans: ChapterPlan[],
): ConfirmationViewModel {
  const numbers = selected.map((plan) => plan.chapter_no);
  const duplicate = new Set(numbers).size !== numbers.length;
  const hasPrimaryStoryline = selected.every((plan) =>
    plan.storyline_refs_json.some((ref) => ref.relation === "primary"),
  );
  const validForeshadowings = selected.every((plan) =>
    Array.isArray(plan.foreshadowing_refs_json),
  );
  const overlapsConfirmed = selected.some((plan) =>
    allPlans.some(
      (other) =>
        other.id !== plan.id &&
        other.status === "confirmed" &&
        other.chapter_no === plan.chapter_no,
    ),
  );
  const complexRelations = selected.filter(
    (plan) =>
      plan.storyline_refs_json.length + plan.foreshadowing_refs_json.length >=
      5,
  );
  const checks: ConfirmationCheck[] = [
    {
      label: "章节序号无重复",
      status: duplicate ? "error" : "success",
      detail: duplicate
        ? "所选章节存在重复序号，请返回检查。"
        : "所选章节序号互不重复。",
    },
    {
      label: "章节均位于故事线范围内",
      status: hasPrimaryStoryline ? "success" : "error",
      detail: hasPrimaryStoryline
        ? "每章均关联了可用故事线。"
        : "存在未关联主故事线的章节。",
    },
    {
      label: "每章已设置主故事线",
      status: hasPrimaryStoryline ? "success" : "error",
      detail: hasPrimaryStoryline
        ? "每章均已设置主故事线。"
        : "请先为每章设置主故事线。",
    },
    {
      label: "关联伏笔章节范围有效",
      status: validForeshadowings ? "success" : "error",
      detail: validForeshadowings
        ? "关联伏笔可用于本次确认。"
        : "存在无效的伏笔关联。",
    },
    {
      label: "未覆盖现有已确认章节",
      status: overlapsConfirmed ? "error" : "success",
      detail: overlapsConfirmed
        ? "所选章节与已确认章节的序号重复。"
        : "不会覆盖现有已确认章节。",
    },
  ];
  const warnings = complexRelations.map(
    (plan) => `第 ${plan.chapter_no} 章关联内容较多，正文生产时可能更复杂。`,
  );
  return {
    selectedLabel: `已选择：${selected.length} 个候选章节`,
    rangeLabel: `章节范围：${chapterNumbersLabel(numbers)}`,
    sourceLabel: `来源：${[...new Set(selected.map((plan) => chapterPlanSourceLabel(plan.source)))].join("、") || "未记录"}`,
    checks,
    warnings,
    canConfirm:
      selected.length > 0 && checks.every((check) => check.status !== "error"),
  };
}

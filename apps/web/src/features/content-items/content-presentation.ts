import type {
  ContentVersionSource,
  ContentVersionStatus,
  ReviewFinding,
  ReviewReport,
} from "./content-item-http-api";

const sources: Record<ContentVersionSource, string> = {
  manual_created: "手动创建",
  mock_generated: "模拟生成",
  manual: "手动编辑",
  generated: "生成内容",
  mock_rewrite: "模拟重写",
};
const statuses: Record<ContentVersionStatus, string> = {
  editable_draft: "可编辑草稿",
  frozen: "已冻结",
};
const conclusions: Record<ReviewReport["conclusion"], string> = {
  pass: "审核通过",
  revise: "建议修改",
};
const categories: Record<ReviewFinding["category"], string> = {
  pacing: "节奏问题",
  foreshadowing: "伏笔表达",
  character_consistency: "人物一致性",
  world_consistency: "世界观一致性",
};
const severities: Record<ReviewFinding["severity"], string> = {
  low: "低",
  medium: "中",
  high: "高",
};

export const contentVersionSourceLabel = (value: string) =>
  sources[value as ContentVersionSource] ?? "其他来源";
export const contentVersionStatusLabel = (value: string) =>
  statuses[value as ContentVersionStatus] ?? "状态未知";
export const reviewConclusionLabel = (value: string) =>
  conclusions[value as ReviewReport["conclusion"]] ?? "审核结果待定";
export const reviewCategoryLabel = (value: string) =>
  categories[value as ReviewFinding["category"]] ?? "其他问题";
export const reviewSeverityLabel = (value: string) =>
  severities[value as ReviewFinding["severity"]] ?? "未分级";
export const formatChineseDate = (value: string | null | undefined) =>
  value
    ? new Intl.DateTimeFormat("zh-CN", {
        dateStyle: "medium",
        timeStyle: "short",
        hour12: false,
      }).format(new Date(value))
    : "未记录";

import type { StorylineNode } from "@/lib/api";

const typeLabels: Record<string, string> = { main: "主故事线", branch: "支线", child: "子故事线" };
const relationLabels: Record<string, string> = { root: "根节点", child: "子故事线" };
const statusLabels: Record<string, string> = { active: "进行中", planned: "规划中", completed: "已完成", paused: "已暂停", archived: "已归档" };

export function isPlaceholderText(value: string | null | undefined): boolean {
  return /[?？]{2,}/.test(value ?? "");
}
export function storylineName(value: string | null | undefined): string { return !value || isPlaceholderText(value) ? "故事线名称待完善" : value; }
export function storylineSummary(value: string | null | undefined): string { return !value || isPlaceholderText(value) ? "暂无故事线摘要" : value; }
export function storylineType(value: string): string { return typeLabels[value] ?? "未知状态"; }
export function storylineRelation(value: string): string { return relationLabels[value] ?? "未知状态"; }
export function storylineStatus(value: string): string { return statusLabels[value] ?? "未知状态"; }
export function chapterRange(node: Pick<StorylineNode, "start_chapter" | "end_chapter">): string {
  if (node.start_chapter == null && node.end_chapter == null) return "未设置";
  if (node.start_chapter != null && node.start_chapter === node.end_chapter) return `第 ${node.start_chapter} 章`;
  return `第 ${node.start_chapter ?? "?"}～${node.end_chapter ?? "?"} 章`;
}
export function childCount(count: number): string { return count === 0 ? "暂无子故事线" : `${count} 条子故事线`; }
export function formatChineseDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "时间待确认";
  const parts = new Intl.DateTimeFormat("zh-CN", { timeZone: "Asia/Shanghai", year: "numeric", month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit", hour12: false }).formatToParts(date);
  const pick = (kind: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === kind)?.value ?? "";
  return `${pick("year")}年${pick("month")}月${pick("day")}日 ${pick("hour")}:${pick("minute")}`;
}
export function storylineViewModel(node: StorylineNode, parentName?: string) {
  return { displayName: storylineName(node.name), displaySummary: storylineSummary(node.summary), displayChapterRange: chapterRange(node), displayType: storylineType(node.type), displayStatus: storylineStatus(node.status), displayParentName: parentName ?? "主故事线", displayChildCount: childCount(node.children.length), displayCreatedAt: formatChineseDate(node.created_at), displayUpdatedAt: formatChineseDate(node.updated_at) };
}

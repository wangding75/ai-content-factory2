import type { ProjectPlanning } from "../contracts/planning";

export const planningCopy = {
  emptyPremise: "尚未填写核心主题",
  emptyAudience: "尚未设置目标受众",
  emptySellingPoints: "暂无核心卖点",
  emptyStyle: "尚未设置文学风格",
  emptyTone: "尚未设置情感基调",
  emptyPlot: "尚未填写核心剧情描述",
  unsaved: "尚未保存策划内容",
} as const;

export function planningSaveStatus(planning: ProjectPlanning): string {
  return planning.version > 0 ? `已保存（版本 ${planning.version}）` : planningCopy.unsaved;
}

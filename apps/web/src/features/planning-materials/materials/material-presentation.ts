import type { MaterialType } from "../contracts/materials";

export const materialTypeLabels: Record<MaterialType, string> = {
  character: "人物",
  worldview: "世界观",
  location: "地点",
  organization: "组织",
  item: "道具",
  reference: "参考资料",
};

const fieldLabels: Partial<Record<MaterialType, Record<string, string>>> = {
  character: { age: "年龄", appearance: "外貌特征", background: "背景", personality: "性格" },
  worldview: { era: "时代背景", rules: "世界规则", history: "历史沿革" },
  location: { location: "地点位置", appearance: "场景特征", atmosphere: "氛围" },
  organization: { purpose: "组织宗旨", members: "成员构成", background: "背景" },
  item: { function: "用途", appearance: "外观", origin: "来源" },
  reference: { source: "来源", author: "作者", notes: "参考说明" },
};

export function materialFields(type: MaterialType, content: Record<string, unknown>) {
  const labels = fieldLabels[type] ?? {};
  return Object.entries(content)
    .filter(([, value]) => value !== null && value !== undefined && String(value).trim() !== "")
    .map(([key, value]) => ({ label: labels[key] ?? "补充信息", value: String(value) }));
}

export function usageShowsRole(usageType: string) {
  return usageType.trim() === "人物角色";
}

export function roleNameForUsage(usageType: string, roleName: string) {
  return usageShowsRole(usageType) ? roleName.trim() : "";
}
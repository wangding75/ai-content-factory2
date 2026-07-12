import type { ProjectMaterialItem } from "../contracts/materials.ts";
import { planningFixtureProject } from "./fixtures.ts";

const projectId = planningFixtureProject.id;
const item = (id: string, type: ProjectMaterialItem["material"]["type"], name: string, summary: string, tags: string[], usage_type: string, role_name: string, notes: string, start: number | null, end: number | null, updated: string): ProjectMaterialItem => ({ material: { id, type, name, summary, content_json: {}, tags_json: tags, version: 1, created_at: "2026-06-16T08:00:00Z", updated_at: updated }, usage: { id: `10000000-0000-4000-8000-${id.slice(-12)}`, project_id: projectId, material_id: id, usage_type, role_name, notes, start_chapter: start, end_chapter: end, status: "active", version: 1, created_at: "2026-06-16T08:00:00Z", updated_at: updated }, last_updated_at: updated });

export const projectMaterialFixtures: ProjectMaterialItem[] = [
  item("20000000-0000-4000-8000-000000000001", "character", "林野", "冷静谨慎，在末世爆发后带领小队寻找安全区。", ["主角", "幸存者"], "人物角色", "主角", "承担关键决策", 1, null, "2026-07-11T06:30:00Z"),
  item("20000000-0000-4000-8000-000000000002", "character", "许晴", "医生，在生存压力和道德选择之间不断挣扎。", ["医生", "同伴"], "人物角色", "配角", "推动医疗线冲突", 1, 24, "2026-07-11T05:10:00Z"),
  item("20000000-0000-4000-8000-000000000003", "worldview", "红雾末世设定", "红雾会造成感染和认知异常，是所有章节必须遵守的世界规则。", ["红雾", "感染规则"], "规则设定", "世界观", "全篇一致性约束", 1, null, "2026-07-10T16:00:00Z"),
  item("20000000-0000-4000-8000-000000000004", "location", "临江安全区", "由旧体育馆改建的幸存者聚居地，资源与秩序都在临界点。", ["安全区", "临江"], "环境场景", "主要场景", "第一卷核心地点", 1, 30, "2026-07-10T10:00:00Z"),
  item("20000000-0000-4000-8000-000000000005", "organization", "曙光互助会", "掌握配给权的民间组织，与主角小队维持脆弱合作。", ["组织", "配给"], "势力关系", "盟友", "中期转为矛盾焦点", 8, null, "2026-07-09T15:00:00Z"),
  item("20000000-0000-4000-8000-000000000006", "item", "禁区通行证", "进入红雾禁区的唯一许可，也是各方争夺的剧情推动物。", ["道具", "禁区"], "剧情推动", "关键道具", "第十二章首次出现", 12, 20, "2026-07-09T09:00:00Z"),
  item("20000000-0000-4000-8000-000000000007", "reference", "灾后气候研究摘录", "用于校准红雾扩散与资源衰减的参考资料。", ["资料", "气候"], "创作参考", "参考资料", "供世界观设定查阅", null, null, "2026-07-08T09:00:00Z"),
];

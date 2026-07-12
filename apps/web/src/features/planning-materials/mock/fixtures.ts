import type { Project } from "../../../lib/api.ts";
import type { ProjectPlanning } from "../contracts/planning.ts";

export const planningFixtureProject: Project = {
  id: "00000000-0000-4000-8000-000000000001",
  name: "末世求生",
  type: "novel",
  status: "planning",
  description: "一部关于红雾降临后，人类在资源枯竭中寻找希望的科幻末日小说。",
  current_stage: "project_planning",
  created_at: "2026-06-16T08:00:00Z",
  updated_at: "2026-07-11T06:20:00Z",
};

export const planningFixture: ProjectPlanning = {
  project_id: planningFixtureProject.id,
  premise: "红雾末世求生",
  audience: "20-35 岁硬核科幻与末日题材爱好者",
  style: "紧张、克制",
  goals_json: { selling_points: ["沉浸式克苏鲁元素", "资源枯竭压力"], plot_summary: "红雾笼罩城市后，幸存者必须在有限资源与未知污染中寻找离开的道路。" },
  constraints_json: { emotional_tone: "压抑、绝望中寻找微光" },
  version: 3,
  created_at: "2026-06-16T08:00:00Z",
  updated_at: "2026-07-11T06:20:00Z",
};

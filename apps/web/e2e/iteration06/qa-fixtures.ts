import { readFileSync } from "node:fs";

export type Iteration06Fixtures = {
  project_a_id: string;
  confirmed_chapter_plan_id: string;
  pending_chapter_plan_id: string;
  project_b_id?: string;
  confirmed_chapter_plan_b_id?: string;
};

export function loadIteration06Fixtures(): Iteration06Fixtures {
  const fixturePath = process.env.I06_QA_FIXTURES;
  if (!fixturePath) throw new Error("I06_QA_FIXTURES must point to reset-fixtures.json.");
  try {
    const value: unknown = JSON.parse(readFileSync(fixturePath, "utf8"));
    if (!value || typeof value !== "object") throw new Error("fixture JSON must be an object.");
    const fixture = value as Partial<Iteration06Fixtures>;
    for (const key of ["project_a_id", "confirmed_chapter_plan_id", "pending_chapter_plan_id"] as const) {
      if (typeof fixture[key] !== "string" || fixture[key].trim() === "") throw new Error(`fixture field ${key} must be a non-empty string.`);
    }
    return fixture as Iteration06Fixtures;
  } catch (error) {
    throw new Error(`Unable to load I06_QA_FIXTURES (${fixturePath}): ${error instanceof Error ? error.message : String(error)}`);
  }
}

export function requireProjectBFixtures(fixtures: Iteration06Fixtures): Required<Pick<Iteration06Fixtures, "project_b_id" | "confirmed_chapter_plan_b_id">> {
  if (!fixtures.project_b_id || !fixtures.confirmed_chapter_plan_b_id) throw new Error("I06_QA_FIXTURES must include project_b_id and confirmed_chapter_plan_b_id for race-condition tests.");
  return { project_b_id: fixtures.project_b_id, confirmed_chapter_plan_b_id: fixtures.confirmed_chapter_plan_b_id };
}

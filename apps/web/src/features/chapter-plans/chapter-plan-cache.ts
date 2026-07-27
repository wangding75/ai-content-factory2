export const chapterPlanCacheEvent = "acf:chapter-plan-cache-updated";

export function invalidateChapterPlanViews(projectId: string) {
  window.dispatchEvent(
    new CustomEvent(chapterPlanCacheEvent, { detail: { projectId } }),
  );
}

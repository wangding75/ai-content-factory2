"use client";

import { useEffect, useState } from "react";
import { getProjectWorkspace, type Project } from "@/lib/api";
import { CandidateBatchListPage } from "@/features/chapter-plans/candidate-batch-list-page";
import { CandidateBatchDetailPage } from "@/features/chapter-plans/candidate-batch-detail-page";
import { ChapterPlansWorkspace } from "@/features/chapter-plans/chapter-plans-workspace";
import { StorylinesWorkspace } from "@/features/storylines-workspace";
import { ProjectWorkspaceFrame } from "./project-workspace-frame";

type WorkspacePage = "chapters" | "candidate-batches" | "candidate-batch-detail" | "storylines";

export function ProjectWorkspaceLoader({ projectId, page, batchId }: { projectId: string; page: WorkspacePage; batchId?: string }) {
  const [project, setProject] = useState<Project | null>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    void getProjectWorkspace(projectId)
      .then((workspace) => setProject(workspace.project))
      .catch(() => setProject(null))
      .finally(() => setLoaded(true));
  }, [projectId]);

  if (!loaded) return <main className="project-works-state"><h1>正在加载项目</h1></main>;
  if (!project) return <main className="project-works-state error"><h1>暂时无法加载项目</h1></main>;

  const content = page === "chapters"
    ? <ChapterPlansWorkspace projectId={projectId} project={project} />
    : page === "candidate-batches"
      ? <CandidateBatchListPage projectId={projectId} />
      : page === "candidate-batch-detail" && batchId
        ? <CandidateBatchDetailPage batchId={batchId} />
        : <StorylinesWorkspace projectId={projectId} />;
  return <ProjectWorkspaceFrame project={project} active={page === "storylines" ? "storylines" : "chapter-plans"} variant={page === "storylines" ? "wide" : "standard"}>{content}</ProjectWorkspaceFrame>;
}

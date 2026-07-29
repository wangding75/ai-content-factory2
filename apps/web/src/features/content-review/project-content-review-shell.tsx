"use client";
/* eslint-disable react-hooks/set-state-in-effect */

import { useEffect, useState } from "react";
import { getProjectWorkspace, type Project } from "@/lib/api";
import { ProjectWorkspaceFrame } from "@/features/planning-materials/components/project-workspace-frame";
import { ContentEditorWorkspace } from "@/features/content-items/content-editor-workspace";
import { ContentReviewWorkspace } from "./content-review-workspace";
import { ContentReviewHistory } from "./content-review-history";
import { reviewCopy as copy } from "./content-review-locale";

function useProject(projectId: string) {
  const [project, setProject] = useState<Project | null>(null);
  const [error, setError] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    setProject(null);
    setError(false);
    void getProjectWorkspace(projectId, controller.signal)
      .then(({ project: next }) => {
        if (!controller.signal.aborted) setProject(next);
      })
      .catch(() => {
        if (!controller.signal.aborted) setError(true);
      });
    return () => controller.abort();
  }, [projectId]);
  return { project, error };
}

function ProjectState({ error }: { error: boolean }) {
  return (
    <main className="project-works-state error">
      <h1>{error ? copy.common.projectLoadFailed : copy.common.loading}</h1>
      <p>
        {error
          ? copy.common.projectLoadFailedHint
          : copy.common.projectLoading}
      </p>
    </main>
  );
}

export function ProjectWorkEditorShell({
  projectId,
  workId,
}: {
  projectId: string;
  workId: string;
}) {
  const { project, error } = useProject(projectId);
  if (!project) return <ProjectState error={error} />;
  return (
    <ProjectWorkspaceFrame project={project} active="works" variant="wide">
      <ContentEditorWorkspace projectId={projectId} workId={workId} />
    </ProjectWorkspaceFrame>
  );
}

export function ProjectReviewShell({
  projectId,
  workId,
  reportId,
  issueId,
  sourceView,
}: {
  projectId: string;
  workId: string;
  reportId?: string;
  issueId?: string;
  sourceView?: boolean;
}) {
  const { project, error } = useProject(projectId);
  if (!project) return <ProjectState error={error} />;
  return (
    <ProjectWorkspaceFrame project={project} active="review" variant="wide">
      <ContentReviewWorkspace
        projectId={projectId}
        workId={workId}
        reportId={reportId}
        issueId={issueId}
        sourceView={sourceView}
      />
    </ProjectWorkspaceFrame>
  );
}

export function ProjectReviewHistoryShell({
  projectId,
  workId,
}: {
  projectId: string;
  workId: string;
}) {
  const { project, error } = useProject(projectId);
  if (!project) return <ProjectState error={error} />;
  return (
    <ProjectWorkspaceFrame project={project} active="review" variant="wide">
      <ContentReviewHistory projectId={projectId} workId={workId} />
    </ProjectWorkspaceFrame>
  );
}

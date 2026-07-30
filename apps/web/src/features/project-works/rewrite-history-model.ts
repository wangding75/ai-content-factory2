export type RewriteHistoryRun = {
  id: string;
  runNumber: string;
  retryOfRunId?: string | null;
};

export const rewriteHistoryRunLabel = (run: RewriteHistoryRun) =>
  `${run.runNumber}${run.retryOfRunId ? "（重试）" : ""}`;

export const rewriteHistoryRunHref = (base: string, runId: string) =>
  `${base}?workflowRunId=${encodeURIComponent(runId)}`;

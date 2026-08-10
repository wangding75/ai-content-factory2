"use client";
/* eslint-disable react-hooks/set-state-in-effect */

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Icon } from "@/components/ui/icons";
import {
  getContentItem,
  type ContentItemDetail,
  type ContentVersion,
} from "@/features/content-items/content-item-http-api";
import {
  retryWorkflowRun,
  type WorkflowRunDto,
} from "@/features/workflow-runs/workflow-run-api";
import { ContentReviewDrawer } from "./content-review-drawer";
import {
  getContentReviewSummary,
  getReview,
  getReviewRunEvents,
  getReviewSourceVersion,
  isRealReviewDetail,
  retryReviewResultConsumption,
  updateReviewIssue,
  type ContentReviewSummary,
  type RealReviewDetail,
  type RealReviewIssue,
  type ReviewDetail,
  type ReviewDisposition,
  type WorkflowRunEventDto,
} from "./content-review-api";
import { reviewCopy as copy } from "./content-review-locale";
import {
  formatReviewTime,
  locateIssueInSource,
  realReviewConclusionLabel,
  realReviewSeverityLabel,
  reviewDispositionLabel,
  reviewLocationLabel,
  safeReviewError,
} from "./content-review-presentation";
import {
  reviewCategoryLabel,
  reviewConclusionLabel,
  reviewSeverityLabel,
} from "@/features/content-items/content-presentation";
import {
  isUncertainReviewCommandError,
  reviewCommandKey,
  type ReviewCommandKey,
} from "./content-review-command-key";

const activeStates = new Set(["queued", "running"]);

export function ContentReviewWorkspace({
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
  const router = useRouter();
  const [content, setContent] = useState<ContentItemDetail | null>(null);
  const [summary, setSummary] = useState<ContentReviewSummary | null>(null);
  const [detail, setDetail] = useState<ReviewDetail | null>(null);
  const [events, setEvents] = useState<WorkflowRunEventDto[]>([]);
  const [runSource, setRunSource] = useState<ContentVersion | null>(null);
  const [runSourceError, setRunSourceError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [drawer, setDrawer] = useState(false);
  const [retrying, setRetrying] = useState(false);
  const controllers = useRef<AbortController[]>([]);
  const runSourceCache = useRef<{ id: string; value: ContentVersion } | null>(null);
  const retryCommand = useRef<ReviewCommandKey | null>(null);

  const addController = useCallback(() => {
    const controller = new AbortController();
    controllers.current.push(controller);
    return controller;
  }, []);

  const loadSummary = useCallback(
    async (withContent = false) => {
      const controller = addController();
      try {
        const [nextSummary, nextContent] = await Promise.all([
          getContentReviewSummary(workId, { signal: controller.signal }),
          withContent
            ? getContentItem(workId, { signal: controller.signal })
            : Promise.resolve(null),
        ]);
        if (controller.signal.aborted) return;
        setSummary(nextSummary);
        if (nextContent) setContent(nextContent);
        setError(null);
        const run = nextSummary.activeRun ?? nextSummary.latestRun;
        if (run && activeStates.has(nextSummary.state)) {
          if (!run.subjectId) {
            setEvents([]);
            setRunSource(null);
            setRunSourceError(copy.errors.source);
            return;
          }
          const cachedSource = runSourceCache.current;
          const sourceRequest =
            cachedSource?.id === run.subjectId
              ? Promise.resolve({ content_version: cachedSource.value })
              : getReviewSourceVersion(run.subjectId, {
                  signal: controller.signal,
                });
          try {
            const [eventPage, sourceResult] = await Promise.all([
              getReviewRunEvents(run.id, { signal: controller.signal }),
              sourceRequest,
            ]);
            if (!controller.signal.aborted) {
              setEvents(eventPage.items);
              setRunSource(sourceResult.content_version);
              setRunSourceError(null);
              runSourceCache.current = {
                id: run.subjectId,
                value: sourceResult.content_version,
              };
            }
          } catch {
            if (!controller.signal.aborted) {
              setEvents([]);
              setRunSource(null);
              setRunSourceError(copy.errors.source);
            }
          }
        } else {
          setEvents([]);
          setRunSource(null);
          setRunSourceError(null);
        }
        if (
          !reportId &&
          nextSummary.state === "review_ready" &&
          nextSummary.latestReport
        ) {
          router.replace(
            `/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(nextSummary.latestReport.id)}`,
          );
        }
      } catch (cause) {
        if (!controller.signal.aborted)
          setError(safeReviewError(cause, copy.errors.load));
      }
    },
    [addController, projectId, reportId, router, workId],
  );

  useEffect(() => {
    setLoading(true);
    void loadSummary(true).finally(() => setLoading(false));
    return () => {
      controllers.current.forEach((controller) => controller.abort());
      controllers.current = [];
    };
  }, [loadSummary]);

  useEffect(() => {
    if (!summary || !activeStates.has(summary.state)) return;
    const timer = window.setInterval(
      () => void loadSummary(false),
      5000,
    );
    return () => window.clearInterval(timer);
  }, [loadSummary, summary]);

  const loadDetail = useCallback(
    async (id: string) => {
      const controller = addController();
      setDetailLoading(true);
      setDetailError(null);
      try {
        const next = await getReview(id, { signal: controller.signal });
        if (!controller.signal.aborted) setDetail(next);
      } catch (cause) {
        if (!controller.signal.aborted)
          setDetailError(safeReviewError(cause, copy.errors.detail));
      } finally {
        if (!controller.signal.aborted) setDetailLoading(false);
      }
    },
    [addController],
  );

  useEffect(() => {
    setDetail(null);
    if (reportId) void loadDetail(reportId);
  }, [loadDetail, reportId]);

  const retry = async () => {
    const run = summary?.latestRun;
    if (!run || retrying) return;
    setRetrying(true);
    setError(null);
    const operation =
      summary.state === "result_consumption_failed"
        ? "consumption-retry"
        : "runtime-retry";
    const command = reviewCommandKey(
      retryCommand.current,
      `${operation}:${run.id}:${run.version}`,
    );
    retryCommand.current = command;
    try {
      if (summary.state === "result_consumption_failed") {
        const result = await retryReviewResultConsumption(
          run.id,
          run.version,
          command.key,
        );
        router.replace(
          `/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(result.report.id)}`,
        );
      } else {
        await retryWorkflowRun(run.id, run.version, command.key, "original_configuration");
        setDetail(null);
        await loadSummary();
      }
      retryCommand.current = null;
    } catch (cause) {
      if (!isUncertainReviewCommandError(cause)) retryCommand.current = null;
      setError(
        safeReviewError(
          cause,
          summary.state === "result_consumption_failed"
            ? copy.errors.consumptionRetry
            : copy.errors.runtimeRetry,
        ),
      );
      await loadSummary();
    } finally {
      setRetrying(false);
    }
  };

  if (loading)
    return <ReviewState title={copy.common.loading} loading />;
  if (!content || !summary)
    return (
      <ReviewState
        title={copy.errors.load}
        description={error ?? copy.errors.load}
        retry={() => void loadSummary(true)}
      />
    );

  return (
    <main className="real-review">
      <ReviewHeader
        projectId={projectId}
        workId={workId}
        canStart={summary.canStartReview}
        onStart={() => setDrawer(true)}
      />
      {error && (
        <p className="real-review-page-error" role="alert">
          {error}
        </p>
      )}
      {reportId ? (
        detailLoading ? (
          <ReviewState title={copy.common.loading} loading compact />
        ) : detailError || !detail ? (
          <ReviewState
            title={copy.errors.detail}
            description={detailError ?? copy.errors.detail}
            retry={() => reportId && void loadDetail(reportId)}
            compact
          />
        ) : isRealReviewDetail(detail) ? (
          <RealReportView
            projectId={projectId}
            workId={workId}
            detail={detail}
            issueId={issueId}
            sourceView={sourceView}
            onReload={() => loadDetail(detail.report.id)}
            onDetailChange={setDetail}
          />
        ) : (
          <MockReportView detail={detail} />
        )
      ) : (
        <SummaryState
          projectId={projectId}
          content={content}
          summary={summary}
          events={events}
          runSource={runSource}
          runSourceError={runSourceError}
          retrying={retrying}
          onRetry={() => void retry()}
          onStart={() => setDrawer(true)}
        />
      )}
      {drawer && (
        <ContentReviewDrawer
          projectId={projectId}
          version={content.current_version}
          onClose={() => setDrawer(false)}
          onCreated={() => {
            setDrawer(false);
            router.replace(`/projects/${projectId}/works/${workId}/review`);
            void loadSummary();
          }}
        />
      )}
    </main>
  );
}

function ReviewHeader({
  projectId,
  workId,
  canStart,
  onStart,
}: {
  projectId: string;
  workId: string;
  canStart: boolean;
  onStart: () => void;
}) {
  return (
    <header className="real-review-header">
      <div>
        <h2>{copy.common.title}</h2>
        <p>{copy.common.subtitle}</p>
      </div>
      <nav>
        <Link href={`/projects/${projectId}/works/${workId}`}>
          {copy.report.openEditor}
        </Link>
        <Link href={`/projects/${projectId}/works/${workId}/review/history`}>
          <Icon name="timeline" size={17} />
          {copy.common.history}
        </Link>
        <button type="button" onClick={onStart} disabled={!canStart}>
          {copy.common.start}
        </button>
      </nav>
    </header>
  );
}

function SummaryState({
  projectId,
  content,
  summary,
  events,
  runSource,
  runSourceError,
  retrying,
  onRetry,
  onStart,
}: {
  projectId: string;
  content: ContentItemDetail;
  summary: ContentReviewSummary;
  events: WorkflowRunEventDto[];
  runSource: ContentVersion | null;
  runSourceError: string | null;
  retrying: boolean;
  onRetry: () => void;
  onStart: () => void;
}) {
  if (summary.state === "idle")
    return (
      <section className="review-empty-state">
        <Icon name="info" size={34} />
        <h3>{copy.states.idleTitle}</h3>
        <p>{copy.states.idleDescription}</p>
        <button type="button" onClick={onStart}>
          {copy.common.start}
        </button>
      </section>
    );
  if (summary.state === "not_configured")
    return (
      <div className="review-not-configured-grid">
        <section className="review-empty-state warning">
          <Icon name="workflow" size={36} />
          <h3>{copy.states.notConfiguredTitle}</h3>
          <p>{copy.states.notConfiguredDescription}</p>
          <Link
            href={`/projects/${projectId}/settings?tab=workflow-bindings`}
          >
            {copy.common.settings}
          </Link>
        </section>
        <aside>
          <h3>{copy.common.currentSubject}</h3>
          <dl>
            <div>
              <dt>{copy.common.chapter}</dt>
              <dd>{content.current_version.title}</dd>
            </div>
            <div>
              <dt>{copy.common.version}</dt>
              <dd>V{content.current_version.version_no}</dd>
            </div>
            <div>
              <dt>{copy.common.wordCount}</dt>
              <dd>
                {content.current_version.word_count.toLocaleString("zh-CN")}{" "}
                {copy.common.words}
              </dd>
            </div>
          </dl>
          <p>{copy.states.notConfiguredHint}</p>
        </aside>
      </div>
    );
  if (summary.state === "queued" || summary.state === "running") {
    const run = summary.activeRun ?? summary.latestRun!;
    if (runSourceError)
      return (
        <ReviewState
          title={copy.errors.source}
          description={runSourceError}
          compact
        />
      );
    if (!runSource)
      return <ReviewState title={copy.common.loading} loading compact />;
    return (
      <RunningState
        source={runSource}
        run={run}
        state={summary.state}
        events={events}
      />
    );
  }
  if (
    summary.state === "runtime_failed" ||
    summary.state === "output_validation_failed" ||
    summary.state === "result_consumption_failed"
  )
    return (
      <FailureState
        projectId={projectId}
        summary={summary}
        retrying={retrying}
        onRetry={onRetry}
      />
    );
  return (
    <ReviewState
      title={copy.common.loading}
      description={copy.states.refreshHint}
      loading
      compact
    />
  );
}

function RunningState({
  source,
  run,
  state,
  events,
}: {
  source: ContentVersion;
  run: WorkflowRunDto;
  state: "queued" | "running";
  events: WorkflowRunEventDto[];
}) {
  const completedEvents = new Set(events.map((event) => event.eventType));
  const steps = [
    ["created", "queued"],
    ["runtime", "worker_started"],
    ["validation", "output_validated"],
    ["consume", "result_consumed"],
  ] as const;
  return (
    <>
      <section className="review-running-banner">
        <span className="review-running-pulse" />
        <div>
          <h3>
            {source.title} {copy.states.runningTitle}
          </h3>
          <p>
            {copy.common.versionPrefix} V{source.version_no} ·{" "}
            {state === "queued" ? copy.states.queued : copy.states.running}
          </p>
          <small>Run ID: {run.runNumber}</small>
          <small>{copy.states.runningHint}</small>
        </div>
        <Link href={`/workflow-runs/${run.id}`}>
          {copy.common.runDetail}
        </Link>
      </section>
      <section className="review-run-summary">
        <span>
          {copy.common.contentVersion} <b>V{source.version_no}</b>
        </span>
        <span>
          {copy.common.wordCount}{" "}
          <b>{source.word_count.toLocaleString("zh-CN")} {copy.common.words}</b>
        </span>
        <span>
          {copy.common.startedAt} <b>{formatReviewTime(run.startedAt ?? run.createdAt)}</b>
        </span>
        <span>
          {copy.common.currentStage}{" "}
          <b>{state === "queued" ? copy.states.queued : copy.states.running}</b>
        </span>
      </section>
      <section className="review-progress-card">
        {steps.map(([key, event], index) => {
          const complete = completedEvents.has(event);
          const current =
            !complete &&
            (index === 0 ||
              steps.slice(0, index).every(([, previous]) =>
                completedEvents.has(previous),
              ));
          return (
            <article
              key={key}
              className={complete ? "complete" : current ? "current" : ""}
            >
              <i>{complete ? "✓" : index + 1}</i>
              <div>
                <h4>{copy.progress[key]}</h4>
                <p>{copy.progress[`${key}Hint` as keyof typeof copy.progress]}</p>
              </div>
            </article>
          );
        })}
      </section>
      <p className="review-refresh-hint">{copy.states.refreshHint}</p>
    </>
  );
}

function FailureState({
  projectId,
  summary,
  retrying,
  onRetry,
}: {
  projectId: string;
  summary: ContentReviewSummary;
  retrying: boolean;
  onRetry: () => void;
}) {
  const error = summary.latestError;
  const run = summary.latestRun;
  const consumption = summary.state === "result_consumption_failed";
  const title =
    summary.state === "runtime_failed"
      ? copy.states.runtimeFailed
      : summary.state === "output_validation_failed"
        ? copy.states.validationFailed
        : copy.states.consumptionFailed;
  return (
    <>
      <section className="review-failure-banner">
        <Icon name="info" size={28} />
        <div>
          <h3>{title}</h3>
          <p>{error?.message ?? title}</p>
          <small>
            {copy.states.occurredAt}：{formatReviewTime(error?.occurredAt)}
          </small>
        </div>
        <div>
          <button type="button" onClick={onRetry} disabled={retrying || !run}>
            {retrying
              ? copy.states.retrying
              : consumption
                ? copy.states.consumptionRetry
                : copy.states.runtimeRetry}
          </button>
          {run && (
            <Link href={`/workflow-runs/${run.id}`}>
              {copy.common.runDetail}
            </Link>
          )}
        </div>
      </section>
      <div className="review-failure-grid">
        <section>
          <h3>{copy.states.failureAdvice}</h3>
          <ol>
            <li>{copy.states.adviceConnection}</li>
            <li>
              {consumption
                ? copy.states.adviceConsumption
                : copy.states.adviceRuntime}
            </li>
            <li>{copy.states.adviceRecovery}</li>
          </ol>
          <Link
            href={`/projects/${projectId}/settings?tab=workflow-bindings`}
          >
            {copy.common.settings}
          </Link>
        </section>
        <aside>
          <h3>{copy.states.safeDetails}</h3>
          <dl>
            <div>
              <dt>{copy.states.attempts}</dt>
              <dd>{error?.attemptCount ?? 1}</dd>
            </div>
            <div>
              <dt>{copy.states.correlation}</dt>
              <dd>{error?.correlationId ?? copy.common.noValue}</dd>
            </div>
            <div>
              <dt>{copy.states.occurredAt}</dt>
              <dd>{formatReviewTime(error?.occurredAt)}</dd>
            </div>
          </dl>
        </aside>
      </div>
    </>
  );
}

function RealReportView({
  projectId,
  workId,
  detail,
  issueId,
  sourceView,
  onReload,
  onDetailChange,
}: {
  projectId: string;
  workId: string;
  detail: RealReviewDetail;
  issueId?: string;
  sourceView?: boolean;
  onReload: () => Promise<void>;
  onDetailChange: (detail: ReviewDetail) => void;
}) {
  const router = useRouter();
  const selected =
    detail.issues.find((issue) => issue.id === issueId) ??
    (issueId ? null : detail.issues[0] ?? null);
  const [savingId, setSavingId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [source, setSource] = useState<ContentVersion | null>(null);
  const [sourceError, setSourceError] = useState<string | null>(null);
  const dispositionCommand = useRef<ReviewCommandKey | null>(null);

  useEffect(() => {
    if (!sourceView || !issueId) {
      setSource(null);
      setSourceError(null);
      return;
    }
    const controller = new AbortController();
    setSource(null);
    setSourceError(null);
    void getReviewSourceVersion(detail.report.sourceContentVersionId, {
      signal: controller.signal,
    })
      .then((result) => {
        if (!controller.signal.aborted)
          setSource(result.content_version);
      })
      .catch(() => {
        if (!controller.signal.aborted) setSourceError(copy.errors.source);
      });
    return () => controller.abort();
  }, [detail.report.sourceContentVersionId, issueId, sourceView]);

  const changeDisposition = async (
    issue: RealReviewIssue,
    disposition: ReviewDisposition,
  ) => {
    if (savingId) return;
    setSavingId(issue.id);
    setActionError(null);
    const command = reviewCommandKey(
      dispositionCommand.current,
      `${issue.id}:${disposition}:${issue.version}`,
    );
    dispositionCommand.current = command;
    try {
      const updated = await updateReviewIssue(
        detail.report.id,
        issue.id,
        { disposition, expectedVersion: issue.version },
        command.key,
      );
      dispositionCommand.current = null;
      onDetailChange({
        ...detail,
        issues: detail.issues.map((item) =>
          item.id === updated.id ? updated : item,
        ),
      });
    } catch (cause) {
      const error = cause as { status?: number };
      if (!isUncertainReviewCommandError(cause))
        dispositionCommand.current = null;
      if (error?.status === 409) {
        setActionError(copy.report.conflict);
        await onReload();
      } else setActionError(copy.report.updateFailed);
    } finally {
      setSavingId(null);
    }
  };

  if (sourceView && issueId && selected && source)
    return (
      <IssueSourceView
        projectId={projectId}
        workId={workId}
        detail={detail}
        issue={selected}
        source={source}
        saving={savingId === selected.id}
        error={actionError}
        onDisposition={(value) => void changeDisposition(selected, value)}
      />
    );
  if (sourceView && issueId && sourceError)
    return (
      <ReviewState
        title={copy.errors.source}
        description={sourceError}
        compact
      />
    );
  if (issueId && !selected)
    return (
      <ReviewState
        title={copy.errors.detail}
        description={copy.errors.detail}
        compact
      />
    );
  if (sourceView && issueId)
    return <ReviewState title={copy.common.loading} loading compact />;

  const counts = detail.issues.reduce(
    (result, issue) => {
      result[issue.severity] += 1;
      return result;
    },
    { critical: 0, warning: 0, suggestion: 0 },
  );
  const categoryCounts = detail.issues.reduce((result, issue) => {
    result.set(issue.categoryLabel, (result.get(issue.categoryLabel) ?? 0) + 1);
    return result;
  }, new Map<string, number>());
  return (
    <>
      <section className="review-report-banner">
        <Icon name="info" size={23} />
        <div>
          <h3>
            {detail.sourceContentVersionSummary.title} {copy.report.completed}
          </h3>
          <p>
            {copy.common.versionPrefix} V
            {detail.sourceContentVersionSummary.versionNo} ·{" "}
            {formatReviewTime(detail.report.completedAt)}
          </p>
        </div>
        <Link href={`/workflow-runs/${detail.report.workflowRunId}`}>
          {copy.common.runDetail}
        </Link>
      </section>
      <section className="review-report-summary">
        <div>
          <h3>
            {realReviewConclusionLabel(detail.report.conclusion)}
          </h3>
          <p>{detail.report.summary}</p>
        </div>
        <Stat label={copy.report.critical} value={counts.critical} tone="critical" />
        <Stat label={copy.report.warning} value={counts.warning} tone="warning" />
        <Stat label={copy.report.suggestion} value={counts.suggestion} />
        <Stat label={copy.report.passedRules} value={detail.report.passedRuleCount} />
      </section>
      <section className="review-category-summary">
        <header><h3>{copy.report.categorySummary}</h3><span>{copy.report.categorySummaryHint}</span></header>
        {categoryCounts.size ? <div>{Array.from(categoryCounts.entries()).map(([label, count]) => <article key={label}><span>{label}</span><b>{count}</b></article>)}</div> : <p>{copy.report.noIssues}</p>}
      </section>
      <section className="review-report-layout">
        <div className="review-issue-list">
          <header>
            <h3>
              {copy.report.issueList}（{detail.issues.length}）
            </h3>
          </header>
          {detail.issues.length ? (
            detail.issues.map((issue) => (
              <button
                type="button"
                key={issue.id}
                className={selected?.id === issue.id ? "active" : ""}
                onClick={() =>
                  router.push(
                    `/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(detail.report.id)}&issueId=${encodeURIComponent(issue.id)}`,
                  )
                }
              >
                <span className={`severity ${issue.severity}`}>
                  {realReviewSeverityLabel(issue.severity)}
                </span>
                <span>{issue.categoryLabel}</span>
                <b>{issue.title}</b>
                <small>{reviewLocationLabel(issue.location)}</small>
              </button>
            ))
          ) : (
            <p className="review-muted">{copy.report.noIssues}</p>
          )}
        </div>
        <div className="review-issue-panel">
          {selected ? (
            <IssuePanel
              projectId={projectId}
              workId={workId}
              reportId={detail.report.id}
              issue={selected}
              saving={savingId === selected.id}
              error={actionError}
              onDisposition={(value) =>
                void changeDisposition(selected, value)
              }
            />
          ) : (
            <p className="review-muted">{copy.report.noIssues}</p>
          )}
        </div>
      </section>
      <section className="review-report-footer">
        <div>
          <b>{copy.report.source}</b>
          <span>V{detail.sourceContentVersionSummary.versionNo}</span>
        </div>
        <div>
          <b>{copy.report.workflow}</b>
          <span>
            {formatReviewTime(detail.workflowRunSummary.createdAt)}
          </span>
        </div>
        {detail.issues.some((issue) => issue.disposition === "open") ? (
          <Link href={`/projects/${projectId}/works/${workId}/rewrite?reportId=${encodeURIComponent(detail.report.id)}`}>
            创建重写
          </Link>
        ) : (
          <button type="button" disabled title="当前审核结果没有可处理的问题">创建重写</button>
        )}
      </section>
    </>
  );
}

function IssuePanel({
  projectId,
  workId,
  reportId,
  issue,
  saving,
  error,
  onDisposition,
}: {
  projectId: string;
  workId: string;
  reportId: string;
  issue: RealReviewIssue;
  saving: boolean;
  error: string | null;
  onDisposition: (value: ReviewDisposition) => void;
}) {
  return (
    <article>
      <header>
        <span className={`severity ${issue.severity}`}>
          {realReviewSeverityLabel(issue.severity)}
        </span>
        <span>{issue.categoryLabel}</span>
        <i>{reviewDispositionLabel(issue.disposition)}</i>
      </header>
      <h3>{issue.title}</h3>
      <Section title={copy.report.issueDescription}>
        <p>{issue.description}</p>
      </Section>
      <Section title={copy.report.evidence}>
        <blockquote>{issue.evidence.quote ?? copy.common.noValue}</blockquote>
      </Section>
      <Section title={copy.report.sourceRefs}>
        <p>
          {issue.evidence.sourceRefs.length
            ? issue.evidence.sourceRefs.join("、")
            : copy.common.noValue}
        </p>
      </Section>
      <Section title={copy.report.suggestionTitle}>
        <p>{issue.suggestion ?? copy.common.noValue}</p>
      </Section>
      {error && (
        <p className="review-error" role="alert">
          {error}
        </p>
      )}
      <footer>
        <button
          type="button"
          onClick={() =>
            onDisposition(issue.disposition === "ignored" ? "open" : "ignored")
          }
          disabled={saving}
        >
          {saving
            ? copy.report.saving
            : issue.disposition === "ignored"
              ? copy.report.reopen
              : copy.report.ignore}
        </button>
        <Link
          href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(reportId)}&issueId=${encodeURIComponent(issue.id)}&view=source`}
        >
          {copy.report.locate}
        </Link>
        <button type="button" disabled title={copy.report.disabledRewriteHint}>
          {copy.report.disabledRewrite}
        </button>
      </footer>
    </article>
  );
}

function IssueSourceView({
  projectId,
  workId,
  detail,
  issue,
  source,
  saving,
  error,
  onDisposition,
}: {
  projectId: string;
  workId: string;
  detail: RealReviewDetail;
  issue: RealReviewIssue;
  source: ContentVersion;
  saving: boolean;
  error: string | null;
  onDisposition: (value: ReviewDisposition) => void;
}) {
  const located = useMemo(
    () => locateIssueInSource(source.content, issue),
    [issue, source.content],
  );
  return (
    <section className="review-source">
      <header>
        <div>
          <b>
            {copy.common.sourceSnapshot} · V{source.version_no}
          </b>
          <span>{copy.report.editorDrift}</span>
        </div>
        <Link
          href={`/projects/${projectId}/works/${workId}/review?reportId=${encodeURIComponent(detail.report.id)}`}
        >
          {copy.report.backToIssues}
        </Link>
        <Link href={`/projects/${projectId}/works/${workId}`}>
          {copy.report.openEditor}
        </Link>
      </header>
      <div className="review-source-layout">
        <article className="review-source-document">
          <h2>{source.title}</h2>
          {!located.exact && (
            <p className="review-location-fallback">
              {copy.report.locationFallback}
            </p>
          )}
          {located.paragraphs.map((paragraph) => (
            <p
              key={`${paragraph.paragraph}-${paragraph.text.slice(0, 12)}`}
              className={paragraph.highlighted ? "highlighted" : ""}
              data-paragraph={paragraph.paragraph}
            >
              <HighlightedText
                text={paragraph.text}
                quote={paragraph.exactQuote}
              />
            </p>
          ))}
        </article>
        <aside>
          <IssuePanel
            projectId={projectId}
            workId={workId}
            reportId={detail.report.id}
            issue={issue}
            saving={saving}
            error={error}
            onDisposition={onDisposition}
          />
        </aside>
      </div>
    </section>
  );
}

function HighlightedText({
  text,
  quote,
}: {
  text: string;
  quote: string | null;
}) {
  if (!quote || !text.includes(quote)) return text;
  const [before, ...after] = text.split(quote);
  return (
    <>
      {before}
      <mark>{quote}</mark>
      {after.join(quote)}
    </>
  );
}

function MockReportView({
  detail,
}: {
  detail: Exclude<ReviewDetail, RealReviewDetail>;
}) {
  return (
    <section className="review-legacy">
      <header>
        <span>{copy.report.legacy}</span>
        <h3>{reviewConclusionLabel(detail.review.conclusion)}</h3>
        <p>{detail.review.summary}</p>
      </header>
      <dl>
        <div>
          <dt>{copy.report.source}</dt>
          <dd>V{detail.content_version.version_no}</dd>
        </div>
        <div>
          <dt>{copy.report.completedAt}</dt>
          <dd>{formatReviewTime(detail.workflow_run.finished_at)}</dd>
        </div>
      </dl>
      <section>
        <h3>
          {copy.report.issueList}（{detail.findings.length}）
        </h3>
        {detail.findings.length ? (
          detail.findings.map((finding) => (
            <article key={finding.id}>
              <b>{finding.title}</b>
              <span>
                {reviewCategoryLabel(finding.category)} ·{" "}
                {reviewSeverityLabel(finding.severity)}
              </span>
              <p>{finding.description}</p>
            </article>
          ))
        ) : (
          <p>{copy.report.noIssues}</p>
        )}
      </section>
      <button type="button" disabled title={copy.report.disabledRewriteHint}>
        {copy.report.disabledRewrite}
      </button>
    </section>
  );
}

function Stat({
  label,
  value,
  tone = "",
}: {
  label: string;
  value: number;
  tone?: string;
}) {
  return (
    <article className={tone}>
      <strong>{value}</strong>
      <span>{label}</span>
    </article>
  );
}

function Section({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <h4>{title}</h4>
      {children}
    </section>
  );
}

function ReviewState({
  title,
  description,
  retry,
  loading,
  compact,
}: {
  title: string;
  description?: string;
  retry?: () => void;
  loading?: boolean;
  compact?: boolean;
}) {
  return (
    <section
      className={`real-review-state${compact ? " compact" : ""}`}
      aria-busy={loading}
    >
      <Icon name={loading ? "timeline" : "info"} size={28} />
      <h2>{title}</h2>
      {description && <p>{description}</p>}
      {retry && (
        <button type="button" onClick={retry}>
          {copy.common.retry}
        </button>
      )}
    </section>
  );
}

"use client";
/* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */
import { useEffect, useRef, useState } from "react";
import { WorkflowPreflightBlocker } from "@/components/workflow-preflight-blocker";
import { toWorkflowPreflightReasons } from "@/components/workflow-preflight-reason";
import { ApiError } from "@/lib/api";
import { clearOperation, getOrCreateOperation } from "@/features/chapter-plans/use-idempotency";
import { createContentGenerationRun, preflightContentGenerationRun, type ContentGenerationContextOptions, type ContentGenerationPreflightReport, type ContentVersion } from "./content-item-http-api";
import { contentGenerationCopy as copy } from "./content-generation-locale";

const initialOptions: ContentGenerationContextOptions = { includePriorChapterSummaries: true, includeProjectMaterials: true, includeStoryContext: true, includeForeshadowings: true };
const errorText = (error: unknown) => { const api = error as ApiError; return api?.code === "preflight_token_expired" || api?.code === "preflight_input_changed" || api?.code === "current_version_changed" || api?.code === "active_run_conflict" ? copy.drawer.conditionsChanged : copy.drawer.createFailed; };
export const contentGenerationPreflightBlockerReasons = (report: ContentGenerationPreflightReport) => toWorkflowPreflightReasons(report.checks.filter((check) => check.status === "blocked"));

export function ContentGenerationDrawer({ contentItemId, version, chapterLabel = "当前章节", onClose, onCreated }: { contentItemId: string; version: ContentVersion; chapterLabel?: string; onClose: () => void; onCreated: () => Promise<void> }) {
  const [instructions, setInstructions] = useState("");
  const [options, setOptions] = useState(initialOptions);
  const [report, setReport] = useState<ContentGenerationPreflightReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const submitted = useRef(false);
  const request = () => ({ expectedCurrentVersionId: version.id, expectedCurrentVersion: version.version, contextOptions: options, additionalInstructions: instructions.trim() || null });
  const invalidatePreflight = () => { setReport(null); clearOperation(`content-generation:${contentItemId}`); };
  const preflight = async () => { setLoading(true); setError(null); setReport(null); clearOperation(`content-generation:${contentItemId}`); try { setReport(await preflightContentGenerationRun(contentItemId, request())); } catch { setError(copy.drawer.preflightFailed); } finally { setLoading(false); } };
  useEffect(() => { void preflight(); }, []);
  const submit = async () => { if (!report || report.status !== "passed" || !report.preflightToken || submitting || submitted.current) return; submitted.current = true; setSubmitting(true); setError(null); const payload = { preflightToken: report.preflightToken }; const scope = `content-generation:${contentItemId}`; try { const key = await getOrCreateOperation(scope, payload); await createContentGenerationRun(contentItemId, payload, key); clearOperation(scope); await onCreated(); onClose(); } catch (cause) { submitted.current = false; setError(errorText(cause)); } finally { setSubmitting(false); } };
  const reasons = report ? contentGenerationPreflightBlockerReasons(report) : [];
  return (
    <div className="content-generate-backdrop">
      <section className="content-generate-dialog content-generation-drawer content-generation-confirmation" role="dialog" aria-modal="true" aria-labelledby="content-generation-title">
        <header>
          <div>
            <span className="content-generation-eyebrow">运行前确认</span>
            <h2 id="content-generation-title">{copy.drawer.title}</h2>
            <p>{copy.drawer.baseline(version.version_no)}</p>
          </div>
          <button aria-label={copy.drawer.close} onClick={onClose} disabled={submitting}>×</button>
        </header>
        <div className="content-generation-confirmation-scroll">
          <section className="content-generation-overview">
            <h3>工作流概览</h3>
            <dl>
              <div><dt>目标章节</dt><dd>{chapterLabel}</dd></div>
              <div><dt>当前版本</dt><dd>v{version.version_no}（当前）</dd></div>
              <div><dt>候选版本</dt><dd className="candidate">v{report?.targetVersionNo ?? version.version_no + 1}（候选）</dd></div>
              <div><dt>生成工作流</dt><dd>{report?.workflow?.name ?? copy.drawer.workflowConfigured}{report?.workflow?.configurationVersion ? ` · 配置 v${report.workflow.configurationVersion}` : ""}</dd></div>
            </dl>
          </section>
          {loading && <p>{copy.drawer.preflighting}</p>}
          {!loading && (
            <>
              {report && (
                <section className="content-generation-checks" aria-live="polite">
                  <h3>前置检查</h3>
                  {report.status === "blocked" ? <div className="content-generation-blocked-state">
                    <div className="content-generation-check-blocked-summary">
                      <strong>{copy.drawer.preflightBlocked}</strong>
                      <p>当前正文、生成要求和上下文选项均已保留；处理阻断项后才能创建正文生成任务。</p>
                    </div>
                    <WorkflowPreflightBlocker reasons={reasons} />
                    <div className="content-generation-check-list" aria-label="正文生成预检项目">
                      {report.checks.map((check) => <p key={check.code}>{check.status === "blocked" ? copy.drawer.checkBlocked : copy.drawer.checkPassed}{check.message}</p>)}
                    </div>
                    <button type="button" className="content-generation-retry-preflight" onClick={() => void preflight()} disabled={submitting}>{copy.drawer.retryPreflight}</button>
                  </div> : <div className="content-generation-check-passed">
                    <strong>{copy.drawer.preflightPassed}</strong>
                    {report.checks.map((check) => <p key={check.code}>{check.status === "blocked" ? copy.drawer.checkBlocked : copy.drawer.checkPassed}{check.message}</p>)}
                  </div>}
                </section>
              )}
              {report && (
                <section className="content-generation-context-summary">
                  <h3>注入上下文</h3>
                  <div>
                    <span>章节规划<strong>{report.contextSummary.chapterGoalCount} 项</strong></span>
                    <span>前序章节<strong>{report.contextSummary.priorChapterCount} 段</strong></span>
                    <span>项目素材<strong>{report.contextSummary.materialCount} 项</strong></span>
                    <span>故事线<strong>{report.contextSummary.storylineCount} 条</strong></span>
                    <span>伏笔<strong>{report.contextSummary.foreshadowingCount} 条</strong></span>
                  </div>
                </section>
              )}
              {report && report.status === "passed" && (
                <p className="content-generation-candidate-hint">{copy.drawer.preflightCandidate(report.sourceVersion.version_no, report.targetVersionNo)}</p>
              )}
              {!report && <button type="button" onClick={() => void preflight()} disabled={submitting}>{copy.drawer.retryPreflight}</button>}
              <section className={`content-generation-requirements-state${instructions.trim() ? " filled" : " empty"}`} aria-live="polite">
                <div><strong>{instructions.trim() ? "已填写生成要求" : "尚未填写生成要求"}</strong><span>{instructions.trim() ? "内容已保留，可在下方继续编辑。" : "可选；可以在下方补充本次正文创作要求。"}</span></div>
                {instructions.trim() && <p>{instructions}</p>}
              </section>
              <label className="content-generation-instructions">{copy.drawer.additionalInstructions}<span>（可选）</span><textarea value={instructions} maxLength={2000} onChange={(e) => { setInstructions(e.target.value); invalidatePreflight(); }} disabled={submitting} placeholder={copy.drawer.additionalInstructionsPlaceholder} /></label>
              <fieldset className="content-generation-context-options"><legend>{copy.drawer.contextOptions}</legend>{Object.entries(copy.drawer.contextOptionLabels).map(([key, label]) => <label key={key}><input type="checkbox" checked={options[key as keyof ContentGenerationContextOptions]} onChange={(e) => { setOptions((v) => ({ ...v, [key]: e.target.checked })); invalidatePreflight(); }} disabled={submitting} /> {label}</label>)}</fieldset>
              {error && <p className="content-inline-error" role="alert">{error}</p>}
            </>
          )}
        </div>
        <footer><button onClick={onClose} disabled={submitting}>{copy.drawer.cancel}</button>{report?.status === "passed" && <button className="content-generation-confirm" onClick={() => void submit()} disabled={submitting}>{submitting ? copy.drawer.creating : copy.drawer.confirmCreate}</button>}</footer>
      </section>
    </div>
  );
}

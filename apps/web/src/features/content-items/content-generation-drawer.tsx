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

export function ContentGenerationDrawer({ contentItemId, version, onClose, onCreated }: { contentItemId: string; version: ContentVersion; onClose: () => void; onCreated: () => Promise<void> }) {
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
  return <div className="content-generate-backdrop"><section className="content-generate-dialog content-generation-drawer" role="dialog" aria-modal="true" aria-labelledby="content-generation-title"><header><div><h2 id="content-generation-title">{copy.drawer.title}</h2><p>{copy.drawer.baseline(version.version_no)}</p></div><button aria-label={copy.drawer.close} onClick={onClose} disabled={submitting}>×</button></header><div>{loading && <p>{copy.drawer.preflighting}</p>}{!loading && <><label>{copy.drawer.additionalInstructions}<textarea value={instructions} maxLength={2000} onChange={(e) => { setInstructions(e.target.value); invalidatePreflight(); }} disabled={submitting} placeholder={copy.drawer.additionalInstructionsPlaceholder} /></label><fieldset><legend>{copy.drawer.contextOptions}</legend>{Object.entries(copy.drawer.contextOptionLabels).map(([key, label]) => <label key={key}><input type="checkbox" checked={options[key as keyof ContentGenerationContextOptions]} onChange={(e) => { setOptions((v) => ({ ...v, [key]: e.target.checked })); invalidatePreflight(); }} disabled={submitting} /> {label}</label>)}</fieldset>{report ? <section className={report.status === "passed" ? "content-preflight-passed" : "content-preflight-blocked"}><h3>{report.status === "passed" ? copy.drawer.preflightPassed : copy.drawer.preflightBlocked}</h3>{report.status === "blocked" && <WorkflowPreflightBlocker reasons={reasons} />}{report.checks.map((check) => <p key={check.code}>{check.status === "blocked" ? copy.drawer.checkBlocked : copy.drawer.checkPassed}{check.message}</p>)}{report.status === "passed" && <><p>{copy.drawer.preflightCandidate(report.sourceVersion.version_no, report.targetVersionNo)}</p><p>{copy.drawer.workflow(report.workflow?.name ?? copy.drawer.workflowConfigured, report.workflow?.configurationVersion ?? copy.drawer.unknown)}</p></>}</section> : <button type="button" onClick={() => void preflight()} disabled={submitting}>{copy.drawer.retryPreflight}</button>}</>}{error && <p className="content-inline-error" role="alert">{error}</p>}</div><footer><button onClick={onClose} disabled={submitting}>{copy.drawer.cancel}</button>{report?.status === "passed" && <button onClick={() => void submit()} disabled={submitting}>{submitting ? copy.drawer.creating : copy.drawer.confirmCreate}</button>}</footer></section></div>;
}

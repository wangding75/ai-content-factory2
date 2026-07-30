"use client";
/* eslint-disable react-hooks/set-state-in-effect */

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { ApiError } from "@/lib/api";
import { listContentRewriteHistory, type RewriteHistoryPage, type RewriteState } from "./rewrite-api";
import { rewriteHistoryRunHref, rewriteHistoryRunLabel } from "./rewrite-history-model";

const PAGE_SIZE = 20;
const labels: Record<RewriteState, string> = { idle: "空闲", not_configured: "未配置", queued: "排队中", running: "运行中", candidate_ready: "候选版本已就绪", runtime_failed: "运行失败", output_validation_failed: "输出校验失败", result_consumption_failed: "结果提交失败" };
export function RewriteHistory({ projectId, workId, contentItemId, selectedRunId }: { projectId: string; workId: string; contentItemId?: string; selectedRunId?: string }) {
  const [page, setPage] = useState<RewriteHistoryPage | null>(null); const [offset, setOffset] = useState(0); const [error, setError] = useState<ApiError | null>(null);
  const load = useCallback(async (signal?: AbortSignal) => { if (!contentItemId) return; setError(null); try { const next = await listContentRewriteHistory(contentItemId, { limit: PAGE_SIZE, offset }, { signal }); if (!signal?.aborted) setPage(next); } catch (cause) { if (!signal?.aborted) setError(cause as ApiError); } }, [contentItemId, offset]);
  useEffect(() => { const controller = new AbortController(); void load(controller.signal); return () => controller.abort(); }, [load]);
  const base = `/projects/${projectId}/works/${workId}/rewrite`;
  if (!contentItemId) return <HistoryState title="无法打开重写历史" detail="链接缺少内容标识。" />;
  if (error) return <HistoryState title="重写历史加载失败" detail="请检查网络后重试。" retry={() => void load()} />;
  if (!page) return <HistoryState title="正在加载重写历史" />;
  return <main className="rewrite-page"><section className="rewrite-state"><header><h1>重写历史</h1><Link href={base}>返回重写</Link></header>{page.items.length === 0 ? <p>暂无重写记录。</p> : <table><thead><tr><th>运行</th><th>状态</th><th>固定来源</th><th>Candidate</th><th>操作</th></tr></thead><tbody>{page.items.map((entry) => <tr key={entry.workflowRun.id} aria-current={entry.workflowRun.id === selectedRunId ? "true" : undefined}><td>{rewriteHistoryRunLabel(entry.workflowRun)}</td><td>{labels[entry.state]}</td><td>V{entry.sourceContentVersionSummary.versionNo}</td><td>{entry.candidateVersion ? `V${entry.candidateVersion.version_no}${entry.candidateIsCurrent ? "（当前）" : ""}` : "—"}</td><td>{entry.candidateVersion ? <Link href={rewriteHistoryRunHref(base, entry.workflowRun.id)}>查看 Candidate</Link> : <Link href={rewriteHistoryRunHref(base, entry.workflowRun.id)}>查看运行</Link>}{entry.latestError && <p>{entry.latestError.message}</p>}</td></tr>)}</tbody></table>}<footer><button type="button" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>上一页</button><button type="button" disabled={offset + page.limit >= page.total} onClick={() => setOffset(offset + page.limit)}>下一页</button></footer></section></main>;
}
function HistoryState({ title, detail, retry }: { title: string; detail?: string; retry?: () => void }) { return <main className="rewrite-page"><section className="rewrite-state"><h1>{title}</h1>{detail && <p role="alert">{detail}</p>}{retry && <button type="button" onClick={retry}>重试</button>}</section></main>; }

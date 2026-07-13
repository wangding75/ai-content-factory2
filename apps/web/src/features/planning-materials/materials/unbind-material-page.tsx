"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { ApiError } from "../../../lib/api.ts";
import { listProjectMaterialsFromApi, unbindProjectMaterialFromApi } from "../api/project-material-http-api";
import { useLayerInteractions } from "../components/layer-interactions";
import { closeMaterialLayer } from "../components/material-layer-routes";
import type { ProjectMaterialItem } from "../contracts/materials";
import type { PlanningMockScenario } from "../contracts/planning";

const types = { character: "人物", worldview: "世界观", location: "地点", organization: "组织", item: "道具", reference: "参考资料" };
function messageFor(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

export function UnbindMaterialPage({ projectId, materialId, scenario }: { projectId: string; materialId: string; scenario: PlanningMockScenario }) {
  void scenario;
  const router = useRouter();
  const close = useCallback(() => router.push(closeMaterialLayer("unbind", projectId, materialId)), [router, projectId, materialId]);
  const layerRef = useLayerInteractions<HTMLDivElement>(close);
  const alive = useRef(true);
  const [item, setItem] = useState<ProjectMaterialItem | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const load = useCallback(async (signal?: AbortSignal) => {
    setLoading(true); setError("");
    try {
      const result = await listProjectMaterialsFromApi(projectId, {}, { signal });
      if (signal?.aborted || !alive.current) return;
      const next = result.items.find((candidate) => candidate.material.id === materialId) ?? null;
      setItem(next);
      if (!next) setError("当前项目未绑定该素材。");
    } catch (caught) {
      if (!signal?.aborted && alive.current) setError(messageFor(caught, "加载失败"));
    } finally {
      if (!signal?.aborted && alive.current) setLoading(false);
    }
  }, [projectId, materialId]);
  useEffect(() => { alive.current = true; const controller = new AbortController(); const timer = setTimeout(() => { void load(controller.signal); }, 0); return () => { alive.current = false; clearTimeout(timer); controller.abort(); }; }, [load]);
  if (loading) return <div className="detail-skeleton"/>;
  if (error || !item) return <main className="materials-state"><h1>{error === "当前项目未绑定该素材。" ? "当前项目未绑定该素材" : "解除绑定加载失败"}</h1><p>{error || "当前项目未绑定该素材。"}</p><div><button onClick={() => void load()}>重试</button><Link href={`/projects/${projectId}/materials/${materialId}`}>返回素材详情</Link></div></main>;
  const go = async () => {
    if (busy) return;
    setBusy(true); setError("");
    try {
      await unbindProjectMaterialFromApi(projectId, materialId, item.usage.version);
      if (alive.current) router.push(`/projects/${projectId}/materials`);
    } catch (caught) {
      if (!alive.current) return;
      const apiError = caught instanceof ApiError ? caught : null;
      setError(apiError?.code === "VERSION_CONFLICT" ? "素材用途已在其他位置更新，请重新加载后再解除绑定。" : messageFor(caught, "解除绑定失败"));
    } finally { if (alive.current) setBusy(false); }
  };
  const range = item.usage.start_chapter ? `第 ${item.usage.start_chapter}–${item.usage.end_chapter ?? "后续"} 章` : "全局";
  return <div ref={layerRef} className="unbind-prototype" role="dialog" aria-modal="true" aria-label="确认解除项目素材绑定" tabIndex={-1}><div className="unbind-dim"/><div className="unbind-dialog"><header><span className="unbind-icon">!</span><h1>确认解除项目绑定？</h1><p>解除后，素材“{item.material.name}”将不再作为当前项目的创作素材。</p></header><section className="unbind-context"><p><span>素材</span><b>{item.material.name} · {types[item.material.type]}</b></p><p><span>项目</span><b>当前项目 · 小说</b></p><p><span>用途</span><b>{item.usage.usage_type} · {item.usage.role_name || "未填写"} · {range}</b></p></section><section className="unbind-effects"><div><h2>将会发生</h2><p>• 从项目素材列表中移除“{item.material.name}”</p><p>• 删除该素材在当前项目中的用途配置</p><p>• 后续章节规划不再自动读取该素材</p></div><div><h2>不会发生</h2><p>✓ 不会删除全局素材“{item.material.name}”</p><p>✓ 不会影响其他引用该素材的项目</p><p>✓ 不会修改素材的基础属性和详细信息</p></div><small>解除绑定后，素材仍会保留在全局素材中，之后可以重新绑定到当前项目。</small></section>{error && <p className="unbind-error">{error} <button onClick={() => void load()}>重新加载</button></p>}<footer><Link href={`/projects/${projectId}/materials/${materialId}`}>取消</Link><button disabled={busy} onClick={go}>{busy ? "解除中…" : "确认解除"}</button></footer></div></div>;
}
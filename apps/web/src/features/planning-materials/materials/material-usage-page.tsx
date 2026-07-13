"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "../../../lib/api.ts";
import { listProjectMaterialsFromApi, updateProjectMaterialUsageFromApi } from "../api/project-material-http-api";
import { ProjectWorkspaceNav } from "../components/project-workspace-nav";
import { useLayerInteractions } from "../components/layer-interactions";
import { closeMaterialLayer } from "../components/material-layer-routes";
import type { ProjectMaterialItem } from "../contracts/materials";

const labels = { character: "人物", worldview: "世界观", location: "地点", organization: "组织", item: "道具", reference: "参考资料" };

function messageFor(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback;
}

export function MaterialUsagePage({ projectId, materialId }: { projectId: string; materialId: string }) {
  const router = useRouter();
  const close = useCallback(() => router.push(closeMaterialLayer("usage", projectId, materialId)), [router, projectId, materialId]);
  const layerRef = useLayerInteractions<HTMLDivElement>(close);
  const alive = useRef(true);
  const [item, setItem] = useState<ProjectMaterialItem | null>(null);
  const [error, setError] = useState("");
  const [form, setForm] = useState<ProjectMaterialItem["usage"] | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError("");
    try {
      const result = await listProjectMaterialsFromApi(projectId, {}, { signal });
      if (signal?.aborted || !alive.current) return;
      const next = result.items.find((candidate) => candidate.material.id === materialId) ?? null;
      if (!next) {
        setItem(null);
        setForm(null);
        setError("当前项目未绑定该素材。");
        return;
      }
      setItem(next);
      setForm(next.usage);
    } catch (caught) {
      if (!signal?.aborted && alive.current) setError(messageFor(caught, "加载失败"));
    } finally {
      if (!signal?.aborted && alive.current) setLoading(false);
    }
  }, [projectId, materialId]);

  useEffect(() => {
    alive.current = true;
    const controller = new AbortController();
    const timer = setTimeout(() => { void load(controller.signal); }, 0);
    return () => { alive.current = false; clearTimeout(timer); controller.abort(); };
  }, [load]);

  const retry = () => { void load(); };
  if (loading) return <div className="detail-skeleton"/>;
  if (error || !item || !form) return <main className="materials-state"><h1>{error === "当前项目未绑定该素材。" ? "当前项目未绑定该素材" : "项目素材用途加载失败"}</h1><p>{error || "当前项目未绑定该素材。"}</p><div><button onClick={retry}>重试</button><Link href={`/projects/${projectId}/materials/${materialId}`}>返回素材详情</Link></div></main>;

  const { material, usage } = item;
  const dirty = JSON.stringify(form) !== JSON.stringify(usage);
  const save = async () => {
    if (saving || !dirty) return;
    setSaving(true);
    setError("");
    const controller = new AbortController();
    try {
      const updated = await updateProjectMaterialUsageFromApi(projectId, materialId, {
        expected_version: usage.version, usage_type: form.usage_type, role_name: form.role_name, notes: form.notes, start_chapter: form.start_chapter, end_chapter: form.end_chapter,
      }, { signal: controller.signal });
      if (!alive.current) return;
      setItem(updated);
      setForm(updated.usage);
      router.push(closeMaterialLayer("usage", projectId, materialId));
    } catch (caught) {
      if (!alive.current) return;
      const apiError = caught instanceof ApiError ? caught : null;
      setError(apiError?.code === "VERSION_CONFLICT" ? "素材用途已在其他位置更新，请重新加载后再保存。" : messageFor(caught, "保存失败"));
    } finally {
      if (alive.current) setSaving(false);
    }
  };

  return <div ref={layerRef} className="create-material" role="dialog" aria-modal="true" aria-label="编辑项目素材用途" tabIndex={-1}><header><p>项目 / 编辑素材用途</p><h1>编辑项目素材用途</h1></header><ProjectWorkspaceNav projectId={projectId} active="materials"/><main><div className="create-modal"><section className="create-head"><div><h2>{material.name}</h2><p>{labels[material.type]} · {material.summary || "暂无简介"}</p></div></section><div className="create-body"><p className="create-tip">当前页面只影响本项目用途，不会修改全局素材，也不会影响其他项目。</p><label className="create-field"><span>项目用途</span><input value={form.usage_type} onChange={event => setForm({ ...form, usage_type: event.target.value })}/></label><label className="create-field"><span>角色名称</span><input value={form.role_name} onChange={event => setForm({ ...form, role_name: event.target.value })}/></label><label className="create-field"><span>使用说明</span><textarea value={form.notes} onChange={event => setForm({ ...form, notes: event.target.value })}/></label><div className="create-grid"><label className="create-field"><span>起始章节</span><input type="number" value={form.start_chapter ?? ""} onChange={event => setForm({ ...form, start_chapter: event.target.value ? Number(event.target.value) : null })}/></label><label className="create-field"><span>结束章节</span><input type="number" value={form.end_chapter ?? ""} onChange={event => setForm({ ...form, end_chapter: event.target.value ? Number(event.target.value) : null })}/></label></div><p className="detail-label">当前 Usage 版本：v{usage.version}</p>{error && <p className="unbind-error">{error} <button onClick={retry}>重新加载</button></p>}</div><footer><Link href={`/projects/${projectId}/materials/${materialId}`}>取消</Link><button disabled={!dirty || saving} onClick={save}>{saving ? "保存中…" : "保存用途"}</button></footer></div></main></div>;
}
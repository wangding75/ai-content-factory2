"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useLayerInteractions } from "../components/layer-interactions";
import { closeMaterialLayer } from "../components/material-layer-routes";
import { getMaterialFromApi, type GlobalMaterialDetail } from "../api/material-http-api";
import { listProjectMaterialsFromApi } from "../api/project-material-http-api";
import type { ProjectMaterialUsage } from "../contracts/materials";
import { materialFields, materialTypeLabels, usageShowsRole } from "./material-presentation";

export function MaterialDetailDrawer({ projectId, materialId }: { projectId: string; materialId: string }) {
  const router = useRouter();
  const [detail, setDetail] = useState<GlobalMaterialDetail | null>(null);
  const [usage, setUsage] = useState<ProjectMaterialUsage | null>(null);
  const [error, setError] = useState("");
  const close = useCallback(() => router.push(closeMaterialLayer("detail", projectId)), [router, projectId]);

  const load = useCallback(async (signal?: AbortSignal) => {
    try {
      const [nextDetail, projectMaterials] = await Promise.all([getMaterialFromApi(materialId, { signal }), listProjectMaterialsFromApi(projectId, {}, { signal })]);
      if (signal?.aborted) return;
      setDetail(nextDetail);
      setUsage(projectMaterials.items.find((item) => item.material.id === materialId)?.usage ?? null);
      setError("");
    } catch (reason) {
      if (signal?.aborted) return;
      setError(reason instanceof Error ? reason.message : "加载失败");
    }
  }, [materialId, projectId]);

  useEffect(() => { const controller = new AbortController(); const timer = window.setTimeout(() => { void load(controller.signal); }, 0); return () => { window.clearTimeout(timer); controller.abort(); }; }, [load]);
  const layerRef = useLayerInteractions(close);
  if (error) return <><button className="drawer-backdrop" aria-label="关闭详情" onClick={close}/><aside className="material-drawer drawer-state" role="dialog" aria-modal="true"><h2>素材详情加载失败</h2><p>{error}</p><button onClick={() => void load()}>重试</button><button onClick={close}>关闭</button></aside></>;
  if (!detail) return <div className="drawer-loading"/>;

  const { material } = detail;
  const fields = materialFields(material.type, material.content_json);
  return <><button className="drawer-backdrop" aria-label="关闭详情" onClick={close}/><aside ref={layerRef} className="material-drawer" role="dialog" aria-modal="true" aria-label="素材详情" tabIndex={-1}>
    <header className="drawer-header"><div><p>素材详情</p><small>已绑定到当前项目</small></div><button onClick={close} aria-label="关闭">×</button></header>
    <div className="drawer-scroll">
      <section className="drawer-identity"><div className="drawer-avatar">{material.name.slice(0, 1)}</div><div><h1>{material.name}</h1><span>{materialTypeLabels[material.type]}</span>{material.tags_json.map((tag) => <i key={tag}>{tag}</i>)}</div></section>
      <section className="drawer-section"><div className="drawer-section-title"><h2>素材信息</h2><Link href={`/projects/${projectId}/materials/${materialId}/edit`}>编辑全局素材</Link></div><dl><div><dt>名称</dt><dd>{material.name}</dd></div><div><dt>类型</dt><dd>{materialTypeLabels[material.type]}</dd></div>{fields.map((field) => <div key={field.label}><dt>{field.label}</dt><dd>{field.value}</dd></div>)}<div><dt>摘要</dt><dd>{material.summary || "暂无描述"}</dd></div><div><dt>最近更新</dt><dd>{new Date(material.updated_at).toLocaleString("zh-CN")}</dd></div></dl><p className="drawer-note">编辑素材会更新全局素材内容，并影响所有引用该素材的项目。</p></section>
      <section className="drawer-references"><h2>引用项目</h2><p>当前共有 {detail.reference_count} 个项目引用此素材</p>{detail.references.length ? detail.references.map((reference) => <div key={reference.usage_id}><b>{reference.project_name}</b><small>类型：小说</small></div>) : <p>该素材当前没有有效项目引用。</p>}</section>
      <section className="drawer-section drawer-usage"><h2>当前项目用途</h2>{usage ? <dl><div><dt>用途</dt><dd>{usage.usage_type}</dd></div>{usageShowsRole(usage.usage_type) && <div><dt>具体角色</dt><dd>{usage.role_name || "未填写"}</dd></div>}<div><dt>使用说明</dt><dd>{usage.notes || "未填写"}</dd></div></dl> : <p>当前项目未绑定此素材。</p>}<div className="drawer-actions"><Link href={`/projects/${projectId}/materials/${materialId}/usage`}>编辑项目用途</Link><Link href={`/projects/${projectId}/materials/${materialId}/unbind`}>解除项目绑定</Link></div></section>
    </div>
    <footer><button onClick={close}>关闭</button><Link href={`/projects/${projectId}/materials/${materialId}/usage`}>编辑项目用途</Link></footer>
  </aside></>;
}
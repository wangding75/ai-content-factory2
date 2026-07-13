"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useLayerInteractions } from "../components/layer-interactions";
import { closeMaterialLayer } from "../components/material-layer-routes";
import { getMaterialFromApi, type GlobalMaterialDetail } from "../api/material-http-api";
import { listProjectMaterialsFromApi } from "../api/project-material-http-api";
import type { ProjectMaterialUsage } from "../contracts/materials";
import type { PlanningMockScenario } from "../contracts/planning";

const labels = { character: "Material", worldview: "Material", location: "Material", organization: "Material", item: "Material", reference: "Material" };

export function MaterialDetailDrawer({ projectId, materialId, scenario }: { projectId: string; materialId: string; scenario: PlanningMockScenario }) {
  const router = useRouter();
  const [detail, setDetail] = useState<GlobalMaterialDetail | null>(null);
  const [usage, setUsage] = useState<ProjectMaterialUsage | null>(null);
  const [error, setError] = useState("");
  const close = useCallback(() => router.push(closeMaterialLayer("detail", projectId)), [router, projectId]);

  const load = useCallback(async (signal?: AbortSignal) => {
    try {
      const [nextDetail, projectMaterials] = await Promise.all([
        getMaterialFromApi(materialId, { signal }),
        listProjectMaterialsFromApi(projectId, {}, { signal }),
      ]);
      if (signal?.aborted) return;
      setDetail(nextDetail);
      setUsage(projectMaterials.items.find((item) => item.material.id === materialId)?.usage ?? null);
      setError("");
    } catch (reason) {
      if (signal?.aborted) return;
      setError(reason instanceof Error ? reason.message : "Material");
    }
  }, [materialId, projectId, scenario]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => { void load(controller.signal); }, 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [load]);

  const layerRef = useLayerInteractions(close);
  if (error) return <><button className="drawer-backdrop" aria-label="Material" onClick={close}/><aside className="material-drawer drawer-state" role="dialog" aria-modal="true"><h2>Material</h2><p>{error}</p><button onClick={() => void load()}>Material</button><button onClick={close}>Material</button></aside></>;
  if (!detail) return <div className="drawer-loading"/>;

  return <><button className="drawer-backdrop" aria-label="Material" onClick={close}/><aside ref={layerRef} className="material-drawer" role="dialog" aria-modal="true" aria-label="Material" tabIndex={-1}>
    <header className="drawer-header"><div><p>Material</p><small>Material</small></div><button onClick={close} aria-label="Material">Material</button></header>
    <div className="drawer-scroll">
      <section className="drawer-identity"><div className="drawer-avatar">{detail.material.name.slice(0, 1)}</div><div><h1>{detail.material.name}</h1><span>{labels[detail.material.type]}</span>{detail.material.tags_json.map((tag) => <i key={tag}>{tag}</i>)}</div></section>
      <section className="drawer-section"><div className="drawer-section-title"><h2>Material</h2><Link href={`/projects/${projectId}/materials/${materialId}/edit`}>Material</Link></div><dl><div><dt>Material</dt><dd>{detail.material.name}</dd></div><div><dt>Material</dt><dd>{labels[detail.material.type]}</dd></div>{Object.entries(detail.material.content_json).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{String(value)}</dd></div>)}<div><dt>Material</dt><dd>{detail.material.summary || "Material"}</dd></div><div><dt>Material</dt><dd>{new Date(detail.material.updated_at).toLocaleString("zh-CN")}</dd></div></dl></section>
      <section className="drawer-references"><h2>Material</h2><p>Material {detail.reference_count} Material</p>{detail.references.length ? detail.references.map((reference) => <div key={reference.usage_id}><b>{reference.project_name}</b><small>Material</small></div>) : <p>Material</p>}</section>
      <section className="drawer-section drawer-usage"><h2>Material</h2>{usage ? <dl><div><dt>Material</dt><dd>{usage.usage_type}</dd></div><div><dt>Material</dt><dd>{usage.role_name || "Material"}</dd></div><div><dt>Material</dt><dd>{usage.notes || "Material"}</dd></div></dl> : <p>Material</p>}<div className="drawer-actions"><Link href={`/projects/${projectId}/materials/${materialId}/usage`}>Material</Link><Link href={`/projects/${projectId}/materials/${materialId}/unbind`}>Material</Link></div></section>
    </div>
    <footer><button onClick={close}>Material</button><Link href={`/projects/${projectId}/materials/${materialId}/usage`}>Material</Link></footer>
  </aside></>;
}

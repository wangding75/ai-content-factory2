"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { ApiError } from "../../../lib/api";
import { getMaterialFromApi, updateMaterialFromApi } from "../api/material-http-api";
import { useLayerInteractions } from "../components/layer-interactions";
import { closeMaterialLayer } from "../components/material-layer-routes";
import { ProjectWorkspaceNav } from "../components/project-workspace-nav";
import type { Material } from "../contracts/materials";

const labels = { character: "Material", worldview: "Material", location: "Material", organization: "Material", item: "Material", reference: "Material" };

function errorMessage(error: unknown) {
  if (error instanceof ApiError && error.code === "VERSION_CONFLICT") return "Material";
  if (error instanceof ApiError && error.code === "MATERIAL_NOT_FOUND") return "Material";
  if (error instanceof ApiError && error.code === "VALIDATION_ERROR") return "Material";
  return "Material";
}

export function EditMaterialPage({ projectId, materialId }: { projectId: string; materialId: string }) {
  const router = useRouter();
  const close = useCallback(() => router.push(closeMaterialLayer("edit", projectId, materialId)), [router, projectId, materialId]);
  const layerRef = useLayerInteractions<HTMLDivElement>(close);
  const [saved, setSaved] = useState<Material | null>(null);
  const [form, setForm] = useState<Material | null>(null);
  const [tag, setTag] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    try {
      const detail = await getMaterialFromApi(materialId, { signal });
      if (signal?.aborted) return;
      setSaved(detail.material);
      setForm(detail.material);
      setError("");
    } catch (reason) {
      if (!signal?.aborted) setError(errorMessage(reason));
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [materialId]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => { void load(controller.signal); }, 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [load]);

  if (loading && !form) return <div className="detail-skeleton"/>;
  if (error && !form) return <main className="materials-state"><h1>Material</h1><p>{error}</p><button onClick={() => void load()}>Material</button></main>;
  if (!form || !saved) return null;

  const dirty = JSON.stringify(form) !== JSON.stringify(saved);
  const save = async () => {
    if (!dirty || saving) return;
    setSaving(true);
    setError("");
    try {
      const updated = await updateMaterialFromApi(materialId, {
        expected_version: saved.version,
        name: form.name.trim(),
        summary: form.summary.trim(),
        content_json: form.content_json,
        tags_json: form.tags_json,
      });
      setSaved(updated);
      setForm(updated);
      router.push(closeMaterialLayer("edit", projectId, materialId));
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setSaving(false);
    }
  };

  const reloadLatest = () => void load();
  const addTag = () => {
    const value = tag.trim();
    if (!value || value.length > 50 || form.tags_json.includes(value) || form.tags_json.length >= 20) return;
    setForm({ ...form, tags_json: [...form.tags_json, value] });
    setTag("");
  };

  return <div ref={layerRef} className="create-material" role="dialog" aria-modal="true" aria-label="Material" tabIndex={-1}>
    <header><p>Material / Material</p><h1>Material <span>{labels[form.type]}</span></h1></header>
    <ProjectWorkspaceNav projectId={projectId} active="materials"/>
    <main><div className="create-modal"><section className="create-head"><div><h2>{form.name}</h2><p>Material</p></div></section>
      <div className="create-body"><p className="create-tip">Material{labels[form.type]}Material</p>
        <label className="create-field"><span>Material</span><input value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })}/></label>
        <label className="create-field"><span>Material</span><textarea value={form.summary} onChange={(event) => setForm({ ...form, summary: event.target.value })}/></label>
        <label className="create-field"><span>Material</span><div className="create-tags">{form.tags_json.map((item) => <span key={item}>{item}<button type="button" onClick={() => setForm({ ...form, tags_json: form.tags_json.filter((value) => value !== item) })}>Material</button></span>)}<input value={tag} onChange={(event) => setTag(event.target.value)}/><button type="button" onClick={addTag}>Material</button></div></label>
        {form.type === "character" && Object.entries({ age: "Material", personality: "Material", background: "Material", appearance: "Material" }).map(([key, label]) => <label className="create-field" key={key}><span>{label}</span><textarea value={String(form.content_json[key] ?? "")} onChange={(event) => setForm({ ...form, content_json: { ...form.content_json, [key]: event.target.value } })}/></label>)}
        {error && <p className="create-error" role="alert">{error}</p>}
        {error.includes("Material") && <button type="button" onClick={reloadLatest}>Material</button>}
      </div><footer><Link href={`/projects/${projectId}/materials/${materialId}`}>Material</Link><button disabled={!dirty || saving} onClick={() => void save()}>{saving ? "Material" : "Material"}</button></footer>
    </div></main>
  </div>;
}
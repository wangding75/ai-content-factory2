"use client";

import { useCallback, useEffect, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import { listMaterialsFromApi } from "../api/material-http-api";
import type { Material, MaterialType, ProjectMaterialSort } from "../contracts/materials";

const labels: Record<MaterialType, string> = {
  character: "Material",
  worldview: "Material",
  location: "Material",
  organization: "Material",
  item: "Material",
  reference: "Material",
};

const limit = 12;

export function GlobalMaterialsPage() {
  const [items, setItems] = useState<Material[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [searchDraft, setSearchDraft] = useState("");
  const [q, setQ] = useState("");
  const [type, setType] = useState<MaterialType | undefined>();
  const [sort, setSort] = useState<ProjectMaterialSort>("updated_at_desc");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);

  const load = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      const result = await listMaterialsFromApi({ q, type, sort, limit, offset }, { signal });
      setItems(result.items);
      setTotal(result.total);
      setOffset(result.offset);
    } catch (reason) {
      if (reason instanceof ApiError && reason.code === "cancelled") return;
      setError(reason instanceof ApiError ? reason : new ApiError("Material", 0));
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [offset, q, sort, type]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => { void load(controller.signal); }, 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [load]);

  const clear = () => {
    setSearchDraft("");
    setQ("");
    setType(undefined);
    setOffset(0);
  };
  const updateQuery = () => {
    setQ(searchDraft.trim());
    setOffset(0);
  };
  const page = Math.floor(offset / limit) + 1;
  const pages = Math.max(1, Math.ceil(total / limit));

  return <main className="materials-main global-materials">
    <header className="materials-heading"><div><h1>Material</h1><p>Material</p></div></header>
    <section className="materials-filters">
      <label><Icon name="search" size={17}/><input value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} onBlur={updateQuery} onKeyDown={(event) => { if (event.key === "Enter") updateQuery(); }} placeholder="Material"/></label>
      <select value={type ?? ""} onChange={(event) => { setType((event.target.value as MaterialType) || undefined); setOffset(0); }}><option value="">Material</option>{Object.entries(labels).map(([key, value]) => <option value={key} key={key}>{value}</option>)}</select>
      <select value={sort} onChange={(event) => { setSort(event.target.value as ProjectMaterialSort); setOffset(0); }}><option value="updated_at_desc">Material</option><option value="updated_at_asc">Material</option><option value="name_asc">Material A-Z</option><option value="name_desc">Material Z-A</option></select>
    </section>
    {loading ? <div className="materials-grid">{[1, 2, 3, 4, 5, 6].map((key) => <div className="materials-skeleton card" key={key}/>)}</div>
      : error ? <section className="materials-empty"><h3>Material</h3><p>{error.code === "network_error" ? "Material" : "Material"}</p><button onClick={() => void load()}>Material</button></section>
      : items.length ? <><p className="materials-count">Material {total} Material</p><section className="materials-grid">{items.map((material) => <article className="material-card" key={material.id}><header><span className="material-icon"><Icon name="archive" size={22}/></span><div><h3>{material.name}</h3><p><b>{labels[material.type]}</b></p></div></header><p className="material-summary">{material.summary}</p><div className="material-tags">{material.tags_json.map((tag) => <span key={tag}>{tag}</span>)}</div><footer>Material {new Date(material.updated_at).toLocaleString("zh-CN")}</footer></article>)}</section><footer className="materials-pagination"><span>Material {page} / {pages} Material</span><button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>Material</button><button disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}>Material</button></footer></>
      : <section className="materials-empty"><Icon name="archive" size={32}/><h3>{q || type ? "Material" : "Material"}</h3><p>{q || type ? "Material" : "Material"}</p>{(q || type) && <button onClick={clear}>Material</button>}</section>}
  </main>;
}

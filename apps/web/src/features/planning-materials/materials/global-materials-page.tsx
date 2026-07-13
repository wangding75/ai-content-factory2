"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError } from "@/lib/api";
import { listMaterialsFromApi } from "../api/material-http-api";
import type { Material, MaterialType, ProjectMaterialSort } from "../contracts/materials";

type Locale = "zh" | "en";

const copy = {
  zh: {
    title: "素材库", description: "管理所有项目共用的素材", search: "搜索素材", allTypes: "全部类型", recent: "最近更新", oldest: "最早更新", nameAscending: "名称 A–Z", nameDescending: "名称 Z–A",
    loadErrorTitle: "暂时无法加载素材", loadErrorDescription: "请检查网络或服务后重试。", retry: "重试", count: (total: number) => `共 ${total} 个素材`, page: (current: number, total: number) => `第 ${current} / ${total} 页`, previous: "上一页", next: "下一页",
    emptyTitle: "暂无素材", emptyDescription: "在项目中创建或绑定素材后，会同步显示在这里。", goProjects: "前往项目", filteredEmptyTitle: "没有符合条件的素材", filteredEmptyDescription: "调整筛选或清空搜索条件后重试。", clearFilters: "清除筛选",
    types: { character: "人物", worldview: "世界观", location: "地点", organization: "组织", item: "道具", reference: "参考资料" },
  },
  en: {
    title: "Materials", description: "Manage materials shared across projects", search: "Search materials", allTypes: "All types", recent: "Recently updated", oldest: "Oldest updated", nameAscending: "Name A–Z", nameDescending: "Name Z–A",
    loadErrorTitle: "Unable to load materials", loadErrorDescription: "Check your connection or service and try again.", retry: "Retry", count: (total: number) => `${total} materials`, page: (current: number, total: number) => `Page ${current} of ${total}`, previous: "Previous", next: "Next",
    emptyTitle: "No materials yet", emptyDescription: "Materials created or bound in a project will appear here.", goProjects: "Go to projects", filteredEmptyTitle: "No matching materials", filteredEmptyDescription: "Adjust the filters or clear your search and try again.", clearFilters: "Clear filters",
    types: { character: "Character", worldview: "Worldview", location: "Location", organization: "Organization", item: "Item", reference: "Reference" },
  },
} as const;

const limit = 12;

export function GlobalMaterialsPage() {
  const locale: Locale = typeof document === "undefined" || document.documentElement.lang.toLowerCase().startsWith("zh") ? "zh" : "en";
  const [items, setItems] = useState<Material[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [searchDraft, setSearchDraft] = useState("");
  const [q, setQ] = useState("");
  const [type, setType] = useState<MaterialType | undefined>();
  const [sort, setSort] = useState<ProjectMaterialSort>("updated_at_desc");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);
  const t = copy[locale];

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
      setError(reason instanceof ApiError ? reason : new ApiError("Unable to load materials.", 0));
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [offset, q, sort, type]);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => { void load(controller.signal); }, 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [load]);

  const clear = () => { setSearchDraft(""); setQ(""); setType(undefined); setOffset(0); };
  const updateQuery = () => { setQ(searchDraft.trim()); setOffset(0); };
  const page = Math.floor(offset / limit) + 1;
  const pages = Math.max(1, Math.ceil(total / limit));
  const filtered = Boolean(q || type);

  return <main className="materials-main global-materials">
    <header className="materials-heading"><div><h1>{t.title}</h1><p>{t.description}</p></div></header>
    <section className="materials-filters">
      <label><Icon name="search" size={17}/><input value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} onBlur={updateQuery} onKeyDown={(event) => { if (event.key === "Enter") updateQuery(); }} placeholder={t.search}/></label>
      <select aria-label={t.allTypes} value={type ?? ""} onChange={(event) => { setType((event.target.value as MaterialType) || undefined); setOffset(0); }}><option value="">{t.allTypes}</option>{(Object.keys(t.types) as MaterialType[]).map((key) => <option value={key} key={key}>{t.types[key]}</option>)}</select>
      <select aria-label={t.recent} value={sort} onChange={(event) => { setSort(event.target.value as ProjectMaterialSort); setOffset(0); }}><option value="updated_at_desc">{t.recent}</option><option value="updated_at_asc">{t.oldest}</option><option value="name_asc">{t.nameAscending}</option><option value="name_desc">{t.nameDescending}</option></select>
    </section>
    {loading ? <div className="materials-grid">{[1, 2, 3, 4, 5, 6].map((key) => <div className="materials-skeleton card" key={key}/>)}</div>
      : error ? <section className="materials-empty"><h3>{t.loadErrorTitle}</h3><p>{t.loadErrorDescription}</p><button onClick={() => void load()}>{t.retry}</button></section>
      : items.length ? <><p className="materials-count">{t.count(total)}</p><section className="materials-grid">{items.map((material) => <article className="material-card" key={material.id}><header><span className="material-icon"><Icon name="archive" size={22}/></span><div><h3>{material.name}</h3><p><b>{t.types[material.type]}</b></p></div></header><p className="material-summary">{material.summary}</p><div className="material-tags">{material.tags_json.map((tag) => <span key={tag}>{tag}</span>)}</div><footer>{new Date(material.updated_at).toLocaleString(locale === "zh" ? "zh-CN" : "en-US")}</footer></article>)}</section><footer className="materials-pagination"><span>{t.page(page, pages)}</span><button disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - limit))}>{t.previous}</button><button disabled={offset + limit >= total} onClick={() => setOffset(offset + limit)}>{t.next}</button></footer></>
      : <section className="materials-empty"><Icon name="archive" size={32}/><h3>{filtered ? t.filteredEmptyTitle : t.emptyTitle}</h3><p>{filtered ? t.filteredEmptyDescription : t.emptyDescription}</p>{filtered ? <button onClick={clear}>{t.clearFilters}</button> : <Link href="/projects">{t.goProjects}</Link>}</section>}
  </main>;
}
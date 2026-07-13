"use client";

import {useCallback, useEffect, useRef, useState} from "react";
import {useRouter} from "next/navigation";
import {Icon} from "@/components/ui/icons";
import {ApiError} from "@/lib/api";
import {listMaterialsFromApi} from "../api/material-http-api";
import {bindProjectMaterialFromApi, listProjectMaterialsFromApi} from "../api/project-material-http-api";
import {useLayerInteractions} from "../components/layer-interactions";
import {closeMaterialLayer} from "../components/material-layer-routes";
import type {Material, MaterialType, ProjectMaterialUsageInput} from "../contracts/materials";
import type {PlanningMockScenario} from "../contracts/planning";

const labels: Record<MaterialType, string> = {character: "人物", worldview: "世界观", location: "地点", organization: "组织", item: "道具", reference: "参考资料"};
export const pickMaterialModalRegions = ["pick-material-modal__header", "pick-material-modal__notice", "pick-material-modal__body", "pick-material-modal__footer"] as const;

export function PickMaterialPage({projectId}: {projectId: string; scenario: PlanningMockScenario}) {
  const router = useRouter();
  const close = useCallback(() => router.push(closeMaterialLayer("pick", projectId)), [router, projectId]);
  const layerRef = useLayerInteractions<HTMLDivElement>(close);
  const [items, setItems] = useState<Material[]>([]);
  const [bound, setBound] = useState(new Set<string>());
  const [selected, setSelected] = useState<Material | null>(null);
  const [searchDraft, setSearchDraft] = useState("");
  const [q, setQ] = useState("");
  const [type, setType] = useState<MaterialType | undefined>();
  const [usage, setUsage] = useState<ProjectMaterialUsageInput>({usage_type: "", role_name: "", notes: "", start_chapter: null, end_chapter: null});
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [key, setKey] = useState(crypto.randomUUID());
  const [submitting, setSubmitting] = useState(false);
  const requestController = useRef<AbortController | null>(null);

  const load = useCallback(async () => {
    requestController.current?.abort();
    const controller = new AbortController();
    requestController.current = controller;
    setLoading(true);
    try {
      const [materials, projectMaterials] = await Promise.all([listMaterialsFromApi({q, type}, {signal: controller.signal}), listProjectMaterialsFromApi(projectId, {}, {signal: controller.signal})]);
      if (controller.signal.aborted) return;
      setItems(materials.items);
      setBound(new Set(projectMaterials.items.map((item) => item.material.id)));
      setError("");
    } catch (cause) {
      if (controller.signal.aborted) return;
      setError(cause instanceof Error ? cause.message : "读取失败");
    } finally { if (!controller.signal.aborted) setLoading(false); }
  }, [projectId, q, type]);

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => { window.clearTimeout(timer); requestController.current?.abort(); };
  }, [load]);

  const submit = async () => {
    if (submitting || !selected || bound.has(selected.id) || !usage.usage_type.trim()) {
      if (!selected || !usage.usage_type.trim()) setError("请选择素材并填写项目用途。");
      return;
    }
    const controller = new AbortController();
    requestController.current = controller;
    setSubmitting(true); setError("");
    try {
      await bindProjectMaterialFromApi(projectId, selected.id, {...usage, usage_type: usage.usage_type.trim(), role_name: usage.role_name.trim(), notes: usage.notes.trim()}, key, {signal: controller.signal});
      if (!controller.signal.aborted) router.push(closeMaterialLayer("pick", projectId));
    } catch (cause) {
      if (controller.signal.aborted) return;
      const code = cause instanceof ApiError ? cause.code : "";
      if (code === "MATERIAL_ALREADY_BOUND") { setBound((current) => new Set(current).add(selected.id)); setSelected(null); }
      setError(code === "MATERIAL_ALREADY_BOUND" ? "该素材已添加到当前项目。" : code === "PROJECT_NOT_FOUND" ? "项目不存在。" : code === "MATERIAL_NOT_FOUND" ? "素材不存在。" : code === "VALIDATION_ERROR" ? "请检查项目用途。" : code === "IDEMPOTENCY_KEY_REUSED" ? "本次绑定与先前请求冲突，请修改用途后重试。" : "绑定失败，已保留当前选择和输入。");
    } finally { if (!controller.signal.aborted) setSubmitting(false); }
  };

  return <div ref={layerRef} className="pick-material" role="dialog" aria-modal="true" aria-labelledby="pick-material-modal-title" tabIndex={-1}>
    <section className="pick-material-modal">
      <header className="pick-material-modal__header">
        <div>
          <h1 id="pick-material-modal-title">选择已有素材</h1>
          <p>从全局素材中选择内容并绑定到当前项目</p>
          <small>当前项目：{projectId}</small>
        </div>
        <button type="button" className="pick-material-modal__close" onClick={close} aria-label="关闭"><Icon name="close" size={22}/></button>
      </header>
      <div className="pick-material-modal__notice"><Icon name="info" size={20}/><p>选择已有素材只会建立项目引用关系，不会复制或修改素材的全局内容。</p></div>
      <div className="pick-material-modal__body">
        <section className="pick-material-modal__library" aria-label="全局素材列表">
          <div className="pick-material-modal__filters">
            <label className="pick-material-modal__search"><Icon name="search" size={18}/><input placeholder="搜索全局素材" value={searchDraft} onChange={(event) => setSearchDraft(event.target.value)} onBlur={() => setQ(searchDraft.trim())} onKeyDown={(event) => {if (event.key === "Enter") setQ(searchDraft.trim());}}/></label>
            <div className="pick-material-modal__type-tabs" role="group" aria-label="素材类型筛选">
              <button type="button" className={!type ? "active" : ""} onClick={() => setType(undefined)}>全部</button>
              {(Object.keys(labels) as MaterialType[]).map((value) => <button type="button" className={type === value ? "active" : ""} onClick={() => setType(value)} key={value}>{labels[value]}</button>)}
            </div>
          </div>
          <div className="pick-material-modal__list">
            {loading ? <p className="pick-material-modal__state">正在加载素材库…</p> : error ? <div className="pick-material-modal__state"><p>{error}</p><button type="button" onClick={load}>重试</button></div> : items.length === 0 ? <div className="pick-material-modal__state"><p>{q || type ? "没有符合条件的素材" : "素材库为空"}</p><button type="button" onClick={() => {setSearchDraft(""); setQ(""); setType(undefined);}}>清除筛选</button></div> : items.map((material) => {
              const added = bound.has(material.id);
              const isSelected = selected?.id === material.id;
              return <button type="button" key={material.id} disabled={added} className={`pick-material-modal__item${isSelected ? " is-selected" : ""}`} onClick={() => !added && setSelected(material)}>
                <span className="pick-material-modal__item-copy"><strong>{material.name}</strong><small>{labels[material.type]}</small><em>{material.summary || "暂无简介"}</em></span>
                {added ? <i>已添加</i> : <span className="pick-material-modal__radio" aria-hidden="true"/>}
              </button>;
            })}
          </div>
        </section>
        <aside className="pick-material-modal__usage" aria-label="当前项目用途设置">
          {selected ? <>
            <section className="pick-material-modal__preview"><div><h2>{selected.name}</h2><span>{labels[selected.type]}</span></div><p>{selected.summary || "暂无简介"}</p><div className="pick-material-modal__tags">{selected.tags_json.map((tag) => <i key={tag}>{tag}</i>)}</div></section>
            <section className="pick-material-modal__usage-form"><div className="pick-material-modal__usage-title"><h2>当前项目用途设置</h2><small>项目：{projectId}</small></div><label>项目内用途<input value={usage.usage_type} maxLength={120} onChange={(event) => {setUsage((current) => ({...current, usage_type: event.target.value})); setKey(crypto.randomUUID());}}/></label><label>角色名称<input value={usage.role_name} maxLength={120} onChange={(event) => setUsage((current) => ({...current, role_name: event.target.value}))}/></label><label>使用说明<textarea value={usage.notes} maxLength={300} onChange={(event) => setUsage((current) => ({...current, notes: event.target.value}))}/></label><div className="pick-material-modal__chapters"><label>起始章节<input type="number" min="1" value={usage.start_chapter ?? ""} onChange={(event) => setUsage((current) => ({...current, start_chapter: event.target.value ? Number(event.target.value) : null}))}/></label><label>结束章节<input type="number" min="1" value={usage.end_chapter ?? ""} onChange={(event) => setUsage((current) => ({...current, end_chapter: event.target.value ? Number(event.target.value) : null}))}/></label></div><p>该用途只在当前项目中生效，不会影响素材的全局定义。</p></section>
          </> : <div className="pick-material-modal__empty"><Icon name="archive" size={28}/><h2>添加到项目</h2><p>从左侧选择一条尚未添加的素材后，填写当前项目用途。</p></div>}
        </aside>
      </div>
      <footer className="pick-material-modal__footer">
        {error && <p role="alert">{error}</p>}
        <button type="button" className="pick-material-modal__cancel" onClick={close}>取消</button>
        <button type="button" className="pick-material-modal__submit" disabled={!selected || loading || submitting || (selected !== null && bound.has(selected.id))} onClick={submit}>绑定到项目</button>
      </footer>
    </section>
  </div>;
}

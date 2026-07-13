"use client";

import {useCallback, useEffect, useRef, useState} from "react";
import {useRouter} from "next/navigation";
import {Icon} from "@/components/ui/icons";
import {ApiError, getProjectWorkspace} from "@/lib/api";
import {createProjectMaterialFromApi} from "../api/project-material-http-api";
import {useLayerInteractions} from "../components/layer-interactions";
import {closeMaterialLayer} from "../components/material-layer-routes";
import type {CreateProjectMaterialRequest, MaterialType} from "../contracts/materials";
import {roleNameForUsage, usageShowsRole} from "./material-presentation";

const labels: Record<MaterialType, string> = {character: "人物", worldview: "世界观", location: "地点", organization: "组织", item: "道具", reference: "参考资料"};
const types = Object.keys(labels) as MaterialType[];
const usageTypes = ["人物角色", "环境场景", "背景设定", "关键线索", "剧情推动"];
const roles = ["", "主角", "配角", "反派", "次要人物"];
const empty = (): CreateProjectMaterialRequest => ({material: {type: "character", name: "", summary: "", content_json: {}, tags_json: []}, usage: {usage_type: "人物角色", role_name: "", notes: "", start_chapter: null, end_chapter: null}});
export const createMaterialModalRegions = ["create-material-modal__header", "create-material-modal__body", "create-material-modal__footer"] as const;

export function CreateMaterialPage({projectId}: {projectId: string}) {
  const router = useRouter();
  const close = useCallback(() => router.push(closeMaterialLayer("create", projectId)), [router, projectId]);
  const layerRef = useLayerInteractions<HTMLDivElement>(close);
  const bodyRef = useRef<HTMLDivElement>(null);
  const requestController = useRef<AbortController | null>(null);
  const [form, setForm] = useState(empty);
  const [tag, setTag] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [key, setKey] = useState(crypto.randomUUID());
  const [projectName, setProjectName] = useState("当前项目");

  useEffect(() => { if (bodyRef.current) bodyRef.current.scrollTop = 0; }, []);
  useEffect(() => () => { requestController.current?.abort(); }, []);
  useEffect(() => { void getProjectWorkspace(projectId).then(({project}) => setProjectName(project.name)).catch(() => {}); }, [projectId]);
  const set = (path: string, value: string) => {
    setForm((current) => {const next = structuredClone(current); const [group, field] = path.split(".") as ["material" | "usage", string]; Object.assign(next[group], {[field]: value}); if (path === "usage.usage_type" && !usageShowsRole(value)) next.usage.role_name = ""; return next;});
    setKey(crypto.randomUUID());
  };
  const add = () => {
    const value = tag.trim();
    if (!value) return;
    if (value.length > 50 || form.material.tags_json.includes(value) || form.material.tags_json.length >= 20) {setErrors({tags: "标签不能为空、不能重复，且最多 20 项。"}); return;}
    setForm((current) => ({...current, material: {...current.material, tags_json: [...current.material.tags_json, value]}}));
    setTag("");
  };
  const submit = async () => {
    if (saving) return;
    const nextErrors: Record<string, string> = {};
    if (!form.material.name.trim()) nextErrors.name = "请填写素材名称。";
    if (!form.usage.usage_type.trim()) nextErrors.usage = "请选择项目用途。";
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length) return;
    const controller = new AbortController();
    requestController.current = controller;
    setSaving(true); setError("");
    try {
      await createProjectMaterialFromApi(projectId, {...form, material: {...form.material, name: form.material.name.trim(), summary: form.material.summary.trim()}, usage: {...form.usage, usage_type: form.usage.usage_type.trim(), role_name: roleNameForUsage(form.usage.usage_type, form.usage.role_name), notes: form.usage.notes.trim(), start_chapter: null, end_chapter: null}}, key, {signal: controller.signal});
      if (!controller.signal.aborted) router.push(closeMaterialLayer("create", projectId));
    } catch (cause) {
      if (controller.signal.aborted) return;
      const code = cause instanceof ApiError ? cause.code : "";
      setError(code === "PROJECT_NOT_FOUND" ? "项目不存在。" : code === "VALIDATION_ERROR" ? "请检查填写内容。" : code === "IDEMPOTENCY_KEY_REUSED" ? "本次提交与先前请求冲突，请修改内容后重试。" : "创建失败，已保留当前输入。请重试。");
    } finally { if (!controller.signal.aborted) setSaving(false); }
  };
  const content = form.material.content_json as Record<string, string>;

  return <div ref={layerRef} className="create-material" role="dialog" aria-modal="true" aria-labelledby="create-material-modal-title" tabIndex={-1}>
    <section className="create-material-modal">
      <header className="create-material-modal__header">
        <div><h1 id="create-material-modal-title">新建素材</h1><p>创建素材并自动绑定到当前项目</p></div>
        <button type="button" className="create-material-modal__close" onClick={close} aria-label="关闭"><Icon name="close" size={22}/></button>
      </header>
      <div ref={bodyRef} className="create-material-modal__body">
        <p className="create-material-modal__notice"><Icon name="info" size={20}/>在项目内创建的素材会自动保存到全局素材，并绑定当前项目。</p>
        <section className="create-material-modal__section"><h2>基础信息</h2><Field label="素材类型"><div className="create-material-modal__types">{types.map((type) => <button type="button" className={form.material.type === type ? "active" : ""} onClick={() => setForm((current) => ({...current, material: {...current.material, type, content_json: type === "character" ? current.material.content_json : {}}}))} key={type}>{labels[type]}</button>)}</div></Field><Field label="素材名称" error={errors.name}><input value={form.material.name} maxLength={120} onChange={(event) => set("material.name", event.target.value)}/></Field><Field label="素材简介"><textarea value={form.material.summary} maxLength={5000} onChange={(event) => set("material.summary", event.target.value)}/></Field><Field label="标签" error={errors.tags}><div className="create-material-modal__tags">{form.material.tags_json.map((value) => <span key={value}>{value}<button type="button" onClick={() => setForm((current) => ({...current, material: {...current.material, tags_json: current.material.tags_json.filter((tagValue) => tagValue !== value)}}))}><Icon name="close" size={13}/></button></span>)}<input value={tag} onChange={(event) => setTag(event.target.value)} onKeyDown={(event) => {if (event.key === "Enter") {event.preventDefault(); add();}}}/><button type="button" onClick={add}>添加</button></div></Field></section>
        {form.material.type === "character" && <section className="create-material-modal__section"><div className="create-material-modal__section-heading"><h2>人物详细信息</h2><small>详细字段会根据素材类型动态变化。</small></div><div className="create-material-modal__two-columns">{([['age', '年龄'], ['personality', '性格']] as const).map(([field, label]) => <Field key={field} label={label}><input type={field === "age" ? "number" : "text"} value={content[field] ?? ""} onChange={(event) => setForm((current) => ({...current, material: {...current.material, content_json: {...current.material.content_json, [field]: event.target.value}}}))}/></Field>)}</div>{([['background', '背景'], ['appearance', '外貌特征']] as const).map(([field, label]) => <Field key={field} label={label}><textarea value={content[field] ?? ""} onChange={(event) => setForm((current) => ({...current, material: {...current.material, content_json: {...current.material.content_json, [field]: event.target.value}}}))}/></Field>)}</section>}
        <section className="create-material-modal__section"><h2>当前项目用途</h2><div className="create-material-modal__context"><span>当前项目：<b>{projectName}</b></span><span>项目类型：<b>小说</b></span></div><div className="create-material-modal__two-columns"><Field label="项目用途" error={errors.usage}><select value={form.usage.usage_type} onChange={(event) => set("usage.usage_type", event.target.value)}>{usageTypes.map((value) => <option value={value} key={value}>{value}</option>)}</select></Field>{usageShowsRole(form.usage.usage_type) && <Field label="具体角色"><select value={form.usage.role_name} onChange={(event) => set("usage.role_name", event.target.value)}>{roles.map((value) => <option value={value} key={value}>{value || "未选择"}</option>)}</select></Field>}</div><Field label="项目使用说明"><textarea value={form.usage.notes} maxLength={300} onChange={(event) => set("usage.notes", event.target.value)}/></Field><p className="create-material-modal__hint">该用途仅在当前项目中生效，不会修改素材的全局内容。</p></section>
        {error && <p className="create-material-modal__error" role="alert">{error}</p>}
      </div>
      <footer className="create-material-modal__footer"><button type="button" className="create-material-modal__cancel" onClick={close}>取消</button><button type="button" className="create-material-modal__submit" disabled={saving} onClick={submit}>{saving ? "创建中…" : "创建并绑定"}</button></footer>
    </section>
  </div>;
}

function Field({label, error, children}: {label: string; error?: string; children: React.ReactNode}) {return <label className="create-material-modal__field"><span>{label}</span>{children}{error && <em>{error}</em>}</label>;}

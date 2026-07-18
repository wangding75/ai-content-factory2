"use client";

import Link from "next/link";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Icon } from "@/components/ui/icons";
import { ApiError, createProject, listProjectTypes, validateProjectInput, type ProjectType } from "@/lib/api";
import { mapProjectTypes, projectErrorMessage, type ProjectTypeVm } from "./project-presentation";

type TypesState = { kind: "loading" } | { kind: "error"; message: string } | { kind: "success"; items: ProjectTypeVm[] };

export function NewProjectForm() {
  const router = useRouter();
  const [typesState, setTypesState] = useState<TypesState>({ kind: "loading" });
  const [typesReload, setTypesReload] = useState(0);
  const [type, setType] = useState<ProjectType>();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [fieldErrors, setFieldErrors] = useState<{ name?: string; description?: string; type?: string }>({});
  const [apiError, setApiError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);
  const submitController = useRef<AbortController | undefined>(undefined);

  useEffect(() => {
    const controller = new AbortController();
    void listProjectTypes(controller.signal).then(({ items }) => {
      if (controller.signal.aborted) return;
      const mapped = mapProjectTypes(items);
      setTypesState({ kind: "success", items: mapped });
      setType((current) => current && mapped.some((item) => item.code === current) ? current : mapped[0]?.code);
    }).catch((error: unknown) => {
      if (!controller.signal.aborted) setTypesState({ kind: "error", message: projectErrorMessage(error instanceof ApiError ? error : undefined) });
    });
    return () => controller.abort();
  }, [typesReload]);

  useEffect(() => () => submitController.current?.abort(), []);

  function validate() {
    const next: { name?: string; description?: string; type?: string } = {};
    const error = validateProjectInput(name, description, type);
    if (error?.startsWith("Project name")) next.name = error;
    else if (error?.startsWith("Description")) next.description = error;
    else if (error?.startsWith("Project type")) next.type = error;
    setFieldErrors(next);
    return !error;
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting || !validate() || !type) return;
    const controller = new AbortController();
    submitController.current = controller;
    setSubmitting(true); setApiError(undefined);
    try {
      const project = await createProject({ name: name.trim(), description, type }, controller.signal);
      router.push(`/projects/${project.id}`);
    } catch (error) {
      if (!controller.signal.aborted) setApiError(projectErrorMessage(error instanceof ApiError ? error : undefined));
      setSubmitting(false);
    }
  }

  const types = typesState.kind === "success" ? typesState.items : [];
  return <form className="create-form" onSubmit={submit} noValidate>
    <header className="create-form-header"><div><h1>新建项目</h1><p>创建一个新的内容项目，项目类型创建后不可直接变更。</p></div><Link className="create-close" href="/projects" aria-label="返回项目列表"><Icon name="close" size={20} /></Link></header>
    <div className="create-form-content"><section className="create-section"><h2>项目类型</h2>
      {typesState.kind === "loading" && <div className="create-type-state" aria-busy="true">正在加载项目类型…</div>}
      {typesState.kind === "error" && <div className="create-type-state" role="alert"><p>{typesState.message}</p><button type="button" onClick={() => { setTypesState({ kind: "loading" }); setTypesReload((value) => value + 1); }}>重试</button></div>}
      {typesState.kind === "success" && (types.length ? <div className="create-type-grid" role="radiogroup" aria-label="项目类型">{types.map((item) => <label className={`create-type-card ${type === item.code ? "selected" : ""}`} key={item.code}><input type="radio" name="type" value={item.code} checked={type === item.code} onChange={() => { setType(item.code); setFieldErrors((errors) => ({ ...errors, type: undefined })); }} /><Icon name="book" size={30} strokeWidth={1.7} /><strong>{item.name}</strong><small>{item.description}</small></label>)}</div> : <div className="create-type-state"><p>暂无可创建的项目类型。</p><button type="button" onClick={() => { setTypesState({ kind: "loading" }); setTypesReload((value) => value + 1); }}>重新加载</button></div>)}
      {fieldErrors.type && <p className="create-field-error" role="alert">请选择一个项目类型。</p>}
    </section><section className="create-section create-fields"><h2>基础信息</h2><div className="create-field"><label htmlFor="name">项目名称 <em>*</em></label><input id="name" name="name" aria-label="项目名称" aria-invalid={Boolean(fieldErrors.name)} aria-describedby="name-help name-error" required maxLength={120} value={name} onChange={(event) => { setName(event.target.value); setFieldErrors((errors) => ({ ...errors, name: undefined })); }} placeholder="例如：末日求生" /><div className="create-help"><span id="name-help">为项目取一个清晰、容易识别的名称</span><span>{name.length}/120</span></div>{fieldErrors.name && <p id="name-error" className="create-field-error" role="alert">{fieldErrors.name === "Project name is required." ? "请输入项目名称。" : "项目名称不能超过 120 个字符。"}</p>}</div><div className="create-field"><label htmlFor="description">项目简介</label><textarea id="description" name="description" aria-label="项目简介" aria-invalid={Boolean(fieldErrors.description)} aria-describedby="description-help description-error" maxLength={5000} value={description} onChange={(event) => { setDescription(event.target.value); setFieldErrors((errors) => ({ ...errors, description: undefined })); }} placeholder="简要描述项目主题和创作方向" rows={4} /><div className="create-help"><span id="description-help">可选，用于记录项目的创作方向</span><span>{description.length}/5000</span></div>{fieldErrors.description && <p id="description-error" className="create-field-error" role="alert">项目简介不能超过 5000 个字符。</p>}</div>{apiError && <p className="create-api-error" role="alert">{apiError}</p>}</section></div>
    <footer className="create-form-footer"><p><Icon name="info" size={16} />确认后将自动进入项目工作区</p><div><Link className="create-cancel" href="/projects"> <Icon name="arrowLeft" size={18} />返回项目</Link><button type="submit" disabled={submitting || typesState.kind !== "success" || types.length === 0} aria-label={submitting ? "正在创建项目" : "创建项目"}>{submitting ? "创建中…" : <><Icon name="plus" size={18} />创建项目</>}</button></div></footer>
  </form>;
}

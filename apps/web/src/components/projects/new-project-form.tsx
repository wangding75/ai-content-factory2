"use client";

import Link from "next/link";
import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Icon } from "@/components/ui/icons";
import { ApiError, createProject, validateProjectInput } from "@/lib/api";

const unavailableTypes = [
  { label: "剧集", icon: "archive" as const },
  { label: "短片", icon: "filePlus" as const },
  { label: "图文", icon: "image" as const },
  { label: "图片", icon: "image" as const },
];

export function NewProjectForm() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<string>();
  const [submitting, setSubmitting] = useState(false);
  const nameError = error?.startsWith("Project name") ? error : undefined;
  const descriptionError = error?.startsWith("Description") ? error : undefined;
  const apiError = error && !nameError && !descriptionError ? error : undefined;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const validationError = validateProjectInput(name, description);
    if (validationError) {
      setError(validationError);
      return;
    }

    setSubmitting(true);
    setError(undefined);

    try {
      const project = await createProject({
        name: name.trim(),
        description,
        type: "novel",
      });
      router.push(`/projects/${project.id}`);
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Unable to create project.");
      setSubmitting(false);
    }
  }

  return (
    <form className="create-form" onSubmit={submit} noValidate>
      <header className="create-form-header">
        <div>
          <h1>新建项目</h1>
          <p>创建一个新的内容项目，项目类型创建后不可直接变更</p>
        </div>
        <Link className="create-close" href="/projects" aria-label="返回项目列表">
          <Icon name="close" size={20} />
        </Link>
      </header>
      <div className="create-form-content">
        <section className="create-section">
          <h2>项目类型</h2>
          <div className="create-type-grid">
            <label className="create-type-card selected"><input type="radio" name="type" checked readOnly /><Icon name="book" size={30} strokeWidth={1.7} /><strong>小说</strong></label>
            {unavailableTypes.map((item) => <div className="create-type-card disabled" key={item.label}><small>暂未开放</small><Icon name={item.icon} size={28} /><strong>{item.label}</strong></div>)}
          </div>
        </section>
        <section className="create-section create-fields">
          <h2>基础信息</h2>
          <div className="create-field">
            <label htmlFor="name">项目名称 <em>*</em></label>
            <input id="name" name="name" aria-label="Project name" aria-invalid={Boolean(nameError)} aria-describedby="name-help name-error" required maxLength={120} value={name} onChange={(event) => { setName(event.target.value); if (nameError) setError(undefined); }} placeholder="例如：末日求生" />
            <div className="create-help"><span id="name-help">为项目取一个清晰、容易识别的名称</span><span>{name.length}/120</span></div>
            {nameError && <p id="name-error" className="create-field-error" role="alert">{nameError}</p>}
          </div>
          <div className="create-field">
            <label htmlFor="description">项目简介</label>
            <textarea id="description" name="description" aria-label="Description" aria-invalid={Boolean(descriptionError)} aria-describedby="description-help description-error" maxLength={5000} value={description} onChange={(event) => { setDescription(event.target.value); if (descriptionError) setError(undefined); }} placeholder="简要描述项目主题和创作方向" rows={4} />
            <div className="create-help"><span id="description-help">可选，用于记录项目的创作方向</span><span>{description.length}/5000</span></div>
            {descriptionError && <p id="description-error" className="create-field-error" role="alert">{descriptionError}</p>}
          </div>
          {apiError && <p className="create-api-error" role="alert">{apiError}</p>}
        </section>
      </div>
      <footer className="create-form-footer">
        <p><Icon name="info" size={16} />确认后将自动进入项目工作台</p>
        <div><Link className="create-cancel" href="/projects"><Icon name="arrowLeft" size={18} />返回项目</Link><button type="submit" disabled={submitting} aria-label={submitting ? "Creating project…" : "Create project"}>{submitting ? "创建中…" : <><Icon name="plus" size={18} />创建项目</>}</button></div>
      </footer>
    </form>
  );
}

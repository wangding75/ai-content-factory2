"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError, type Project } from "@/lib/api";
import {
  getProjectPlanningFromApi,
  saveProjectPlanningToApi,
} from "../api/planning-http-api";
import type {
  ProjectPlanning,
  SaveProjectPlanningRequest,
} from "../contracts/planning";
import { planningCopy, planningSaveStatus } from "./planning-presentation";

type FormState = Omit<SaveProjectPlanningRequest, "expected_version">;
const blankForm = (): FormState => ({
  premise: "",
  audience: "",
  style: "",
  goals_json: { selling_points: [], plot_summary: "" },
  constraints_json: { emotional_tone: "" },
});
const toForm = (data: ProjectPlanning): FormState => ({
  premise: data.premise,
  audience: data.audience,
  style: data.style,
  goals_json: {
    ...data.goals_json,
    selling_points: [...data.goals_json.selling_points],
  },
  constraints_json: { ...data.constraints_json },
});
const same = (a: FormState, b: FormState) =>
  JSON.stringify(a) === JSON.stringify(b);
const isPlanningCompleted = (data: ProjectPlanning) =>
  data.version > 0 &&
  Boolean(
    data.premise.trim() ||
      data.audience.trim() ||
      data.style.trim() ||
      data.goals_json.plot_summary.trim() ||
      data.goals_json.selling_points.length ||
      data.constraints_json.emotional_tone.trim(),
  );

function validate(form: FormState) {
  const errors: Record<string, string> = {};
  if (form.premise.length > 500) errors.premise = "核心主题最多 500 个字符。";
  if (form.audience.length > 500) errors.audience = "目标受众最多 500 个字符。";
  if (form.style.length > 120) errors.style = "文学风格最多 120 个字符。";
  if (form.constraints_json.emotional_tone.length > 500)
    errors.emotional_tone = "情感基调最多 500 个字符。";
  if (form.goals_json.plot_summary.length > 10000)
    errors.plot_summary = "核心剧情描述最多 10000 个字符。";
  if (form.goals_json.selling_points.length > 20)
    errors.selling_points = "核心卖点最多添加 20 项。";
  return errors;
}

export function PlanningPage({ projectId, project }: { projectId: string; project: Project }) {
  const [saved, setSaved] = useState<ProjectPlanning | null>(null);
  const [form, setForm] = useState<FormState>(blankForm);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<ApiError | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const [tagText, setTagText] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [editing, setEditing] = useState(true);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true);
      setLoadError(null);
      setSaveError(null);
      setTagText("");
      try {
        const planning = await getProjectPlanningFromApi(projectId, { signal });
        setSaved(planning);
        setForm(toForm(planning));
        setEditing(!isPlanningCompleted(planning));
      } catch (error) {
        setLoadError(
          error instanceof ApiError
            ? error
            : new ApiError("暂时无法加载项目策划。", 500),
        );
      } finally {
        setLoading(false);
      }
    },
    [projectId],
  );
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      void load(controller.signal);
    }, 0);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [load]);

  const dirty = useMemo(
    () => saved !== null && !same(form, toForm(saved)),
    [form, saved],
  );
  const planningCompleted = saved !== null && isPlanningCompleted(saved);
  const isEditing = !planningCompleted || editing;
  const update = (key: "premise" | "audience" | "style", value: string) =>
    setForm((current) => ({ ...current, [key]: value }));
  const addTag = () => {
    const tag = tagText.trim();
    if (!tag) return;
    if (form.goals_json.selling_points.includes(tag)) {
      setErrors((current) => ({
        ...current,
        selling_points: "核心卖点不能重复。",
      }));
      return;
    }
    if (tag.length > 100 || form.goals_json.selling_points.length >= 20) {
      setErrors((current) => ({
        ...current,
        selling_points:
          tag.length > 100
            ? "每个核心卖点最多 100 个字符。"
            : "核心卖点最多添加 20 项。",
      }));
      return;
    }
    setForm((current) => ({
      ...current,
      goals_json: {
        ...current.goals_json,
        selling_points: [...current.goals_json.selling_points, tag],
      },
    }));
    setTagText("");
    setErrors((current) => ({ ...current, selling_points: "" }));
  };
  const save = async () => {
    if (!saved || saving || !isEditing) return;
    const pendingTag = tagText.trim();
    if (pendingTag && form.goals_json.selling_points.includes(pendingTag)) {
      setErrors((current) => ({ ...current, selling_points: "\u6838\u5fc3\u5356\u70b9\u4e0d\u80fd\u91cd\u590d\u3002" }));
      return;
    }
    const candidate = pendingTag && !form.goals_json.selling_points.includes(pendingTag)
      ? { ...form, goals_json: { ...form.goals_json, selling_points: [...form.goals_json.selling_points, pendingTag] } }
      : form;
    const nextErrors = validate(candidate);
    setErrors(nextErrors);
    if (Object.values(nextErrors).some(Boolean)) return;
    setSaving(true);
    setSaveError(null);
    setNotice(null);
    try {
      const result = await saveProjectPlanningToApi(projectId, {
        ...candidate,
        expected_version: saved.version,
      });
      setSaved(result);
      setForm(toForm(result));
      setEditing(false);
      setTagText("");
      setNotice("策划方案已保存。");
    } catch (error) {
      const apiError = error instanceof ApiError ? error : null;
      setSaveError(
        apiError?.code === "VERSION_CONFLICT"
          ? "当前数据已被更新，未覆盖你的输入。请重新加载后再保存。"
          : "保存失败，已保留当前输入。请稍后重试。",
      );
    } finally {
      setSaving(false);
    }
  };
  const cancel = () => {
    if (saved) {
      setForm(toForm(saved));
      setEditing(!isPlanningCompleted(saved));
      setErrors({});
      setSaveError(null);
      setNotice(null);
    }
  };

  if (loading) return <PlanningSkeleton />;
  if (loadError?.code === "PROJECT_NOT_FOUND")
    return <NotFound projectId={projectId} />;
  if (loadError || !saved)
    return <PlanningLoadError projectId={projectId} retry={load} />;
  return (
    <div className="planning-workspace">
      <main className="planning-main">
        <section className="planning-content">
          <div className="planning-intro">
            <h2>项目策划</h2>
            <p>明确创作方向，让后续的素材与故事创作保持一致。</p>
          </div>
          {planningCompleted && <Link className="planning-workflow-shortcut" href={`/projects/${projectId}/settings?tab=workflow-bindings`}>配置工作流</Link>}
          {dirty && (
            <div className="planning-dirty">
              <span>
                <Icon name="info" size={17} />
                有未保存的更改
              </span>
              <button onClick={save} disabled={saving || !isEditing}>
                立即保存
              </button>
            </div>
          )}
          {notice && <div className="planning-notice success">{notice}</div>}
          {saveError && (
            <div className="planning-notice error" role="alert">
              <span>{saveError}</span>
              {saveError.includes("重新加载") && (
                <button onClick={() => void load()}>重新加载</button>
              )}
            </div>
          )}
          <section className="planning-card">
            <h3>
              <Icon name="chart" size={20} />
              项目定位
            </h3>
            <div className="planning-grid">
              <Field label="核心主题" error={errors.premise}>
                <input
                  disabled={!isEditing}
                  value={form.premise}
                  onChange={(event) => update("premise", event.target.value)}
                  maxLength={500}
                />
              </Field>
              <Field label="目标受众" error={errors.audience}>
                <input
                  disabled={!isEditing}
                  value={form.audience}
                  onChange={(event) => update("audience", event.target.value)}
                  maxLength={500}
                  placeholder="例如：20-35 岁硬核科幻爱好者"
                />
              </Field>
            </div>
            <Field
              label="核心卖点"
              error={errors.selling_points}
            >
              <div className="tag-editor">
                {form.goals_json.selling_points.map((tag) => (
                  <span className="planning-tag" key={tag}>
                    {tag}
                    <button
                      aria-label={`删除 ${tag}`}
                      onClick={() =>
                        setForm((current) => ({
                          ...current,
                          goals_json: {
                            ...current.goals_json,
                            selling_points:
                              current.goals_json.selling_points.filter(
                                (value) => value !== tag,
                              ),
                          },
                        }))
                      }
                    >
                      <Icon name="close" size={14} />
                    </button>
                  </span>
                ))}
                <div className="tag-add">
                  <input
                    disabled={!isEditing}
                    value={tagText}
                    maxLength={100}
                    onChange={(event) => setTagText(event.target.value)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter") {
                        event.preventDefault();
                        addTag();
                      }
                    }}
                    placeholder="添加卖点"
                  />
                  <button type="button" onClick={addTag}>
                    <Icon name="plus" size={15} />
                    添加标签
                  </button>
                </div>
              </div>
            </Field>
          </section>
          <section className="planning-card">
            <h3>
              <Icon name="edit" size={20} />
              创作方向
            </h3>
            <div className="planning-grid">
              <Field label="文学风格" error={errors.style}>
                <input
                  disabled={!isEditing}
                  value={form.style}
                  onChange={(event) => update("style", event.target.value)}
                  maxLength={120}
                  placeholder="例如：紧张、克制"
                />
              </Field>
              <Field label="情感基调" error={errors.emotional_tone}>
                <input
                  disabled={!isEditing}
                  value={form.constraints_json.emotional_tone}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      constraints_json: { emotional_tone: event.target.value },
                    }))
                  }
                  maxLength={500}
                />
              </Field>
            </div>
            <Field label="核心剧情描述" error={errors.plot_summary}>
              <textarea
                disabled={!isEditing}
                value={form.goals_json.plot_summary}
                onChange={(event) =>
                  setForm((current) => ({
                    ...current,
                    goals_json: {
                      ...current.goals_json,
                      plot_summary: event.target.value,
                    },
                  }))
                }
                maxLength={10000}
                rows={5}
                placeholder="描述故事的起始冲突与世界观核心机制…"
              />
            </Field>
          </section>
          <footer className="planning-actions">
            {isEditing ? (
              <>
                <button
                  className="secondary"
                  onClick={cancel}
                  disabled={saving}
                >
                  取消
                </button>
                <button
                  className="primary"
                  onClick={save}
                  disabled={!dirty || saving}
                >
                  {saving
                    ? "保存中…"
                    : planningCompleted
                      ? "保存修改"
                      : "保存策划方案"}
                </button>
              </>
            ) : (
              <button className="primary" onClick={() => setEditing(true)}>
                编辑策划方案
              </button>
            )}
          </footer>
        </section>
        <aside className="planning-preview">
          <p>项目摘要预览</p>
          <article>
            <div className="planning-cover">
              <Icon name="book" size={42} />
              <div>
                <h3>{project.name}</h3>
              </div>
            </div>
            <div className="planning-preview-body">
              <label>定位摘要</label>
              <h2>{form.premise || "尚未填写核心主题"}</h2>
              <p>
                {form.audience || planningCopy.emptyAudience}
              </p>
              <dl>
                <div>
                  <dt>文学风格</dt>
                  <dd>{form.style || planningCopy.emptyStyle}</dd>
                </div>
                <div>
                  <dt>保存状态</dt>
                  <dd>{planningSaveStatus(saved)}</dd>
                </div>
              </dl>
              <blockquote>
                {form.constraints_json.emotional_tone
                  ? `“${form.constraints_json.emotional_tone}”`
                  : planningCopy.emptyTone}
              </blockquote>
              <small>{planningSaveStatus(saved)}</small>
            </div>
          </article>
        </aside>
      </main>
    </div>
  );
}

function Field({
  label,
  error,
  children,
}: {
  label: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="planning-field">
      <span>{label}</span>
      {children}
      {error && <em>{error}</em>}
    </label>
  );
}
function PlanningSkeleton() {
  return (
    <div className="planning-workspace">
      <div className="planning-skeleton header" />
      <div className="planning-skeleton tabs" />
      <main className="planning-main">
        <div className="planning-content">
          {[1, 2, 3].map((item) => (
            <div className="planning-skeleton card" key={item} />
          ))}
        </div>
        <div className="planning-skeleton preview" />
      </main>
    </div>
  );
}
function NotFound({ projectId }: { projectId: string }) {
  return (
    <main className="planning-state">
      <Icon name="folder" size={32} />
      <h1>项目不存在</h1>
      <p>该项目可能已被删除，或项目地址不正确。</p>
      <div>
        <Link href="/projects">返回项目列表</Link>
        <Link href={`/projects/${projectId}`}>查看项目概览</Link>
      </div>
    </main>
  );
}
function PlanningLoadError({
  projectId,
  retry,
}: {
  projectId: string;
  retry: () => void;
}) {
  return (
    <main className="planning-state">
      <Icon name="info" size={32} />
      <h1>暂时无法加载项目策划</h1>
      <p>请检查网络后重试。</p>
      <div>
        <button onClick={retry}>重试</button>
        <Link href={`/projects/${projectId}`}>返回项目概览</Link>
      </div>
    </main>
  );
}

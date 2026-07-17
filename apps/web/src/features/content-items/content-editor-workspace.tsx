"use client";
/* eslint-disable react-hooks/set-state-in-effect, react-hooks/refs, react-hooks/exhaustive-deps */
import Link from "next/link";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { ApiError } from "@/lib/api";
import {
  listChapterPlans,
  type ChapterPlan,
} from "@/features/chapter-plans/chapter-plan-http-api";
import { Icon } from "@/components/ui/icons";
import {
  createOrGetContentItem,
  getContentItem,
  mockGenerateContent,
  saveContentDraft,
  type ContentItemDetail,
  type MockGenerationParameters,
  type WorkflowRunSummary,
} from "./content-item-http-api";
import {
  contentVersionSourceLabel,
  contentVersionStatusLabel,
  formatChineseDate,
} from "./content-presentation";
type Draft = { title: string; content: string; summary: string };
const draftOf = (x: ContentItemDetail): Draft => ({
  title: x.current_version.title,
  content: x.current_version.content,
  summary: x.current_version.summary ?? "",
});
const idKey = () => crypto.randomUUID();
export function ContentEditorWorkspace({
  projectId,
  chapterPlanId,
}: {
  projectId: string;
  chapterPlanId: string;
}) {
  const [detail, setDetail] = useState<ContentItemDetail | null>(null),
    [plan, setPlan] = useState<ChapterPlan | null>(null),
    [draft, setDraft] = useState<Draft | null>(null),
    [loading, setLoading] = useState(true),
    [error, setError] = useState<string | null>(null),
    [saving, setSaving] = useState(false),
    [saveState, setSaveState] = useState<"idle" | "success" | "error">("idle"),
    [saveError, setSaveError] = useState<string | null>(null),
    [clearSummary, setClearSummary] = useState(false),
    [dialog, setDialog] = useState(false),
    [generating, setGenerating] = useState(false),
    [generateError, setGenerateError] = useState<string | null>(null),
    [run, setRun] = useState<WorkflowRunSummary | null>(null);
  const initial = useRef<Draft | null>(null),
    current = useRef(""),
    sequence = useRef(0),
    controllers = useRef<AbortController[]>([]),
    generateKey = useRef<string | null>(null);
  const addController = () => {
    const c = new AbortController();
    controllers.current.push(c);
    return c;
  };
  const apply = useCallback((next: ContentItemDetail) => {
    if (current.current && current.current !== next.content_item.id) return;
    current.current = next.content_item.id;
    const nextDraft = draftOf(next);
    initial.current = nextDraft;
    setDetail(next);
    setDraft(nextDraft);
    setClearSummary(false);
  }, []);
  const load = useCallback(async () => {
    const request = ++sequence.current;
    controllers.current.forEach((c) => c.abort());
    controllers.current = [];
    current.current = "";
    setLoading(true);
    setError(null);
    setDetail(null);
    setDraft(null);
    setRun(null);
    try {
      const c = addController();
      const [created, plans] = await Promise.all([
        createOrGetContentItem(chapterPlanId, { signal: c.signal }),
        listChapterPlans(
          projectId,
          { limit: 100, offset: 0 },
          { signal: c.signal },
        ),
      ]);
      if (c.signal.aborted || request !== sequence.current) return;
      current.current = created.content_item.id;
      setPlan(plans.items.find((x) => x.id === chapterPlanId) ?? null);
      apply(created);
      const get = addController(),
        fresh = await getContentItem(created.content_item.id, {
          signal: get.signal,
        });
      if (
        get.signal.aborted ||
        request !== sequence.current ||
        current.current !== fresh.content_item.id
      )
        return;
      apply(fresh);
    } catch (cause) {
      if (request !== sequence.current) return;
      const api = cause as ApiError;
      setError(
        api?.status === 409
          ? "此章节规划尚未确认，暂不能创建正文。"
          : api?.status === 404
            ? "正文或章节规划不存在。"
            : "正文加载失败，请检查网络后重试。",
      );
    } finally {
      if (request === sequence.current) setLoading(false);
    }
  }, [apply, chapterPlanId, projectId]);
  useEffect(() => {
    void load();
    return () => {
      sequence.current++;
      controllers.current.forEach((c) => c.abort());
    };
  }, [load]);
  const dirty =
    !!draft &&
    !!initial.current &&
    (draft.title !== initial.current.title ||
      draft.content !== initial.current.content ||
      draft.summary !== initial.current.summary ||
      clearSummary);
  const set = (field: keyof Draft, value: string) => {
    setDraft((x) => (x ? { ...x, [field]: value } : x));
    setSaveState("idle");
    setSaveError(null);
    if (field === "summary") setClearSummary(false);
  };
  const save = async () => {
    if (!detail || !draft || saving || !dirty) return;
    const c = addController();
    setSaving(true);
    setSaveState("idle");
    const before = initial.current!,
      body: {
        expected_version: number;
        title?: string;
        content?: string;
        summary?: string | null;
      } = { expected_version: detail.current_version.version };
    if (draft.title !== before.title) body.title = draft.title;
    if (draft.content !== before.content) body.content = draft.content;
    if (clearSummary) body.summary = null;
    else if (draft.summary !== before.summary) body.summary = draft.summary;
    try {
      const updated = await saveContentDraft(detail.content_item.id, body, {
        signal: c.signal,
      });
      if (!c.signal.aborted && current.current === updated.content_item.id) {
        apply(updated);
        setSaveState("success");
      }
    } catch (cause) {
      if (!c.signal.aborted) {
        const api = cause as ApiError;
        setSaveState("error");
        setSaveError(
          api?.status === 409
            ? "版本已发生变化，请刷新后合并并重试。"
            : "保存失败，请稍后重试。",
        );
      }
    } finally {
      if (!c.signal.aborted && current.current === detail.content_item.id)
        setSaving(false);
    }
  };
  const generate = async (parameters: MockGenerationParameters) => {
    if (!detail || generating) return;
    const c = addController(),
      k = generateKey.current ?? idKey();
    generateKey.current = k;
    setGenerating(true);
    setGenerateError(null);
    try {
      const result = await mockGenerateContent(
        detail.content_item.id,
        { expected_version: detail.current_version.version, parameters },
        k,
        { signal: c.signal },
      );
      if (!c.signal.aborted && current.current === result.content_item.id) {
        apply(result);
        setRun(result.workflow_run);
        setDialog(false);
        generateKey.current = null;
        setSaveState("success");
      }
    } catch (cause) {
      if (!c.signal.aborted) {
        const api = cause as ApiError;
        setGenerateError(
          api?.status === 409
            ? "版本或幂等请求发生冲突。请检查后重试。"
            : api?.status === 422
              ? "生成参数无效，请检查后重试。"
              : "生成失败，原草稿未改变，可重试。",
        );
      }
    } finally {
      if (!c.signal.aborted && current.current === detail.content_item.id)
        setGenerating(false);
    }
  };
  if (loading)
    return (
      <State
        title="正在打开正文"
        description="正在创建或读取本章唯一正文…"
        loading
      />
    );
  if (error)
    return (
      <State
        title="无法打开正文"
        description={error}
        retry={() => void load()}
      />
    );
  if (!detail || !draft) return null;
  const v = detail.current_version,
    readOnly =
      v.status !== "editable_draft" || detail.content_item.status !== "draft";
  return (
    <main className="content-editor">
      <header className="content-editor-project">
        <Link href={`/projects/${projectId}/chapter-plans`}>
          <Icon name="arrowLeft" size={17} />
          返回章节规划
        </Link>
        <span>项目正文 / 第 {plan?.chapter_no ?? "—"} 章</span>
      </header>
      <section className="content-editor-grid">
        <aside className="content-editor-left">
          <b>章节导航</b>
          <p className="content-current">
            第 {plan?.chapter_no ?? "—"} 章<br />
            <strong>{detail.content_item.title}</strong>
          </p>
          <Link href={`/projects/${projectId}/chapter-plans`}>
            查看章节规划
          </Link>
        </aside>
        <section className="content-editor-center">
          <header className="content-editor-header">
            <div>
              <h1>
                第 {plan?.chapter_no ?? "—"} 章《{draft.title}》
              </h1>
              <p>
                <span>v{v.version_no}</span>
                <span>{contentVersionSourceLabel(v.source)}</span>
                <span>{contentVersionStatusLabel(v.status)}</span>
                <span>
                  {detail.content_item.status === "draft" ? "待审核" : "已审核"}
                </span>
                <small>{v.word_count} 字</small>
              </p>
            </div>
            <div className="content-actions">
              <button
                onClick={() => void save()}
                disabled={saving || readOnly || !dirty}
              >
                {saving ? "保存中…" : "保存草稿"}
              </button>
              <button
                onClick={() => setDialog(true)}
                disabled={generating || readOnly}
              >
                模拟生成正文
              </button>
              {readOnly ? (
                <Link
                  className="content-review-link"
                  href={`/projects/${projectId}/chapter-plans/${chapterPlanId}/content/review`}
                >
                  查看审核结果
                </Link>
              ) : (
                <Link
                  className="content-review-link"
                  href={`/projects/${projectId}/chapter-plans/${chapterPlanId}/content/review`}
                >
                  提交审核
                </Link>
              )}
            </div>
          </header>
          <div className="content-toolbar" aria-label="编辑器工具栏">
            <button disabled>¶</button>
            <button disabled>
              <b>B</b>
            </button>
            <button disabled>❝</button>
            <button disabled>—</button>
            <i />
            <button disabled>↶</button>
            <button disabled>↷</button>
          </div>
          <div className="content-editor-body">
            <div className="content-editor-paper">
              {v.source === "mock_generated" && (
                <p className="content-notice">
                  当前正文由模拟生成产生。您可以直接修改或通过左侧章节规划重新生成。
                </p>
              )}
              <label>
                标题
                <input
                  value={draft.title}
                  maxLength={120}
                  onChange={(e) => set("title", e.target.value)}
                  disabled={readOnly || saving}
                />
              </label>
              <label>
                章节摘要
                <textarea
                  value={draft.summary}
                  maxLength={5000}
                  onChange={(e) => set("summary", e.target.value)}
                  disabled={readOnly || saving || clearSummary}
                />
              </label>
              <label className="content-clear">
                <input
                  type="checkbox"
                  checked={clearSummary}
                  onChange={(e) => {
                    setClearSummary(e.target.checked);
                    setSaveState("idle");
                  }}
                  disabled={readOnly || saving}
                />{" "}
                保存时清空摘要
              </label>
              <label>
                正文
                <textarea
                  className="content-body-input"
                  value={draft.content}
                  maxLength={200000}
                  onChange={(e) => set("content", e.target.value)}
                  disabled={readOnly || saving}
                />
              </label>
            </div>
          </div>
          <footer className="content-editor-footer">
            <span>
              {saving
                ? "正在保存…"
                : saveState === "success"
                  ? "已保存"
                  : saveState === "error"
                    ? "保存失败"
                    : "未保存"}
            </span>
            <span>{v.word_count} 字</span>
            <span>v{v.version_no}</span>
            <span>最后修改：{formatChineseDate(v.updated_at)}</span>
          </footer>
          {saveError && (
            <p className="content-inline-error" role="alert">
              {saveError}
            </p>
          )}
        </section>
        <aside className="content-editor-right">
          <Info title="章节目标" value={plan?.chapter_goal ?? "未设置"} />
          <Info
            title="故事线"
            value={
              plan?.storyline_refs_json.length
                ? `${plan.storyline_refs_json.length} 条关联`
                : "未加载"
            }
          />
          <Info
            title="关联素材"
            value={
              plan?.material_refs_json.length
                ? `${plan.material_refs_json.length} 项关联`
                : "无"
            }
          />
          <Info
            title="关联伏笔"
            value={
              plan?.foreshadowing_refs_json.length
                ? `${plan.foreshadowing_refs_json.length} 项关联`
                : "无"
            }
          />
          <Info title="创作提醒" value={plan?.creation_notes ?? "未设置"} />
          <Info title="版本记录" value={`v${v.version_no}${detail.content_item.current_version_id === v.id ? "（当前）" : ""} · ${contentVersionSourceLabel(v.source)}`} />
          {run && <Info title="最近工作流" value="最近工作流已完成" />}
        </aside>
      </section>
      {dialog && (
        <GenerateDialog
          plan={plan}
          submitting={generating}
          error={generateError}
          onClose={() => {
            if (!generating) {
              setDialog(false);
              setGenerateError(null);
              generateKey.current = null;
            }
          }}
          onSubmit={generate}
        />
      )}
    </main>
  );
}
function Info({ title, value }: { title: string; value: string }) {
  return (
    <section className="content-info">
      <h2>{title}</h2>
      <p>{value}</p>
    </section>
  );
}
function State({
  title,
  description,
  retry,
  loading,
}: {
  title: string;
  description: string;
  retry?: () => void;
  loading?: boolean;
}) {
  return (
    <main className="content-editor-state">
      <Icon name="book" size={34} />
      <h1>{title}</h1>
      <p>{description}</p>
      {loading ? <span>加载中…</span> : <button onClick={retry}>重试</button>}
    </main>
  );
}
function GenerateDialog({
  plan,
  submitting,
  error,
  onClose,
  onSubmit,
}: {
  plan: ChapterPlan | null;
  submitting: boolean;
  error: string | null;
  onClose: () => void;
  onSubmit: (x: MockGenerationParameters) => void;
}) {
  const [goal, setGoal] = useState(plan?.chapter_goal ?? ""),
    [notes, setNotes] = useState(plan?.creation_notes ?? "");
  const submit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit({
      chapter_goal: goal || null,
      storyline_refs_json:
        plan?.storyline_refs_json.map((x) => x.storyline_id) ?? [],
      material_refs_json: plan?.material_refs_json ?? [],
      foreshadowing_refs_json: plan?.foreshadowing_refs_json ?? [],
      creation_notes: notes || null,
    });
  };
  return (
    <div className="content-generate-backdrop">
      <form
        className="content-generate-dialog"
        onSubmit={submit}
        role="dialog"
        aria-modal="true"
        aria-labelledby="content-generate-title"
      >
        <header>
          <div>
            <h2 id="content-generate-title">模拟生成正文</h2>
            <p>使用当前章节规划的正式参数，生成将只更新可编辑的 v1。</p>
          </div>
          <button
            type="button"
            aria-label="关闭生成对话框"
            onClick={onClose}
            disabled={submitting}
          >
            ×
          </button>
        </header>
        <div>
          <label>
            章节目标
            <textarea
              value={goal}
              maxLength={2000}
              onChange={(e) => setGoal(e.target.value)}
              disabled={submitting}
            />
          </label>
          <label>
            创作备注
            <textarea
              value={notes}
              maxLength={2000}
              onChange={(e) => setNotes(e.target.value)}
              disabled={submitting}
            />
          </label>
          <p className="content-param-summary">
            故事线 {plan?.storyline_refs_json.length ?? 0} 条 · 素材{" "}
            {plan?.material_refs_json.length ?? 0} 项 · 伏笔{" "}
            {plan?.foreshadowing_refs_json.length ?? 0} 项
          </p>
          {error && (
            <p className="content-inline-error" role="alert">
              {error}
            </p>
          )}
        </div>
        <footer>
          <button type="button" onClick={onClose} disabled={submitting}>
            取消
          </button>
          <button
            type="submit"
            disabled={submitting || !plan?.storyline_refs_json.length}
          >
            {submitting ? "生成中…" : "开始生成"}
          </button>
        </footer>
      </form>
    </div>
  );
}

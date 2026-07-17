"use client";
/* eslint-disable react-hooks/set-state-in-effect */
import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import { Icon } from "@/components/ui/icons";
import { ApiError, getForeshadowings, getStorylines, type Foreshadowing, type Project, type StorylineNode } from "@/lib/api";
import { listProjectMaterialsFromApi } from "@/features/planning-materials/api/project-material-http-api";
import type { ProjectMaterialItem } from "@/features/planning-materials/contracts/materials";
import {
  confirmChapterPlans,
  listChapterPlans,
  type ChapterPlan,
  type ChapterPlanList,
  type ChapterPlanStatus,
} from "./chapter-plan-http-api";
import { MockGenerateDialog } from "./mock-generate-dialog";
import { EditChapterPlanDrawer } from "./edit-chapter-plan-drawer";
import { ConfirmChapterPlansDialog } from "./confirm-chapter-plans-dialog";
import { chapterPlanGenerationSummary, chapterPlanStatusLabel, createRelationNames, relationValues } from "./chapter-plan-presentation";

type RelationContext = { storylines: StorylineNode[]; materials: ProjectMaterialItem[]; foreshadowings: Foreshadowing[] };
const limit = 10;
export function ChapterPlansWorkspace({ projectId, project }: { projectId: string; project: Project }) {
  void project;
  const [data, setData] = useState<ChapterPlanList | null>(null),
    [status, setStatus] = useState<ChapterPlanStatus | undefined>(),
    [offset, setOffset] = useState(0),
    [loading, setLoading] = useState(true),
    [error, setError] = useState<ApiError | null>(null),
    [mockOpen, setMockOpen] = useState(false),
    [editing, setEditing] = useState<ChapterPlan | null>(null),
    [selected, setSelected] = useState<Record<string, ChapterPlan>>({}),
    [confirmOpen, setConfirmOpen] = useState(false),
    [confirming, setConfirming] = useState(false),
    [confirmError, setConfirmError] = useState<ApiError | null>(null),
    [relations, setRelations] = useState<RelationContext | null>(null);
  const projectRef = useRef(projectId),
    requestRef = useRef(0),
    confirmControllerRef = useRef<AbortController | null>(null);
  const loadPlans = useCallback(
    async (
      query: { status?: ChapterPlanStatus; offset: number },
      { signal }: { signal?: AbortSignal } = {},
    ) => {
      const request = ++requestRef.current;
      try {
        const plans = await listChapterPlans(
          projectId,
          { status: query.status, limit, offset: query.offset },
          { signal },
        );
        if (
          signal?.aborted ||
          projectRef.current !== projectId ||
          request !== requestRef.current
        )
          return;
        setData(plans);
      } catch (cause) {
        if (
          signal?.aborted ||
          projectRef.current !== projectId ||
          request !== requestRef.current
        )
          return;
        throw cause;
      }
    },
    [projectId],
  );
  const loadInitial = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true);
      setError(null);
      try {
        await loadPlans({ status, offset }, { signal });
        if (signal?.aborted || projectRef.current !== projectId) return;
      } catch (cause) {
        if (signal?.aborted || projectRef.current !== projectId) return;
        setError(
          cause instanceof ApiError
            ? cause
            : new ApiError("Unable to load chapter plans.", 500),
        );
      } finally {
        if (!signal?.aborted && projectRef.current === projectId)
          setLoading(false);
      }
    },
    [loadPlans, offset, projectId, status],
  );
  const loadRelations = useCallback(async (signal?: AbortSignal) => {
    const [storylines, materials, foreshadowings] = await Promise.all([
      getStorylines(projectId, signal),
      listProjectMaterialsFromApi(projectId, { limit: 100 }, { signal }),
      getForeshadowings(projectId, signal),
    ]);
    if (!signal?.aborted && projectRef.current === projectId) {
      setRelations({ storylines: storylines.items, materials: materials.items, foreshadowings: foreshadowings.items });
    }
  }, [projectId]);
  useEffect(() => {
    projectRef.current = projectId;
    setMockOpen(false);
    setEditing(null);
    setSelected({});
    setConfirmOpen(false);
    setConfirmError(null);
    setRelations(null);
    confirmControllerRef.current?.abort();
    const controller = new AbortController();
    void loadInitial(controller.signal);
    void loadRelations(controller.signal).catch(() => {
      if (!controller.signal.aborted && projectRef.current === projectId) setRelations({ storylines: [], materials: [], foreshadowings: [] });
    });
    return () => {
      controller.abort();
      confirmControllerRef.current?.abort();
    };
  }, [loadInitial, loadRelations, projectId]);
  const clearSelection = () => {
    setSelected({});
    setConfirmOpen(false);
    setConfirmError(null);
  };
  const choose = (next?: ChapterPlanStatus) => {
    clearSelection();
    setStatus(next);
    setOffset(0);
  };
  const changePage = (nextOffset: number) => {
    clearSelection();
    setOffset(nextOffset);
  };
  const refreshAfterGeneration = useCallback(async () => {
    clearSelection();
    setStatus(undefined);
    setOffset(0);
    try {
      await loadPlans({ offset: 0 });
    } catch (cause) {
      setError(
        cause instanceof ApiError
          ? cause
          : new ApiError("Unable to reload chapter plans.", 500),
      );
      throw cause;
    }
  }, [loadPlans]);
  if (loading && !data) return <Loading />;
  if (error)
    return (
      <State
        title={error.status === 404 ? "项目不存在" : "章节规划加载失败"}
        description={
          error.status === 404
            ? "请确认项目仍存在，或返回项目列表。"
            : "请检查网络连接后重试。"
        }
        retry={() => void loadInitial()}
      />
    );
  if (!data) return null;
  const page = Math.floor(data.offset / data.limit) + 1,
    pages = Math.max(1, Math.ceil(data.total / data.limit)),
    pending = data.items.filter(
      (plan) => plan.status === "pending_confirmation",
    ),
    selectedPlans = Object.values(selected),
    allPendingSelected =
      pending.length > 0 && pending.every((plan) => selected[plan.id]);
  const toggle = (plan: ChapterPlan) =>
    setSelected((current) => {
      const next = { ...current };
      if (next[plan.id]) delete next[plan.id];
      else next[plan.id] = plan;
      return next;
    });
  const toggleAll = () =>
    setSelected((current) => {
      const next = { ...current };
      const all = pending.every((plan) => next[plan.id]);
      pending.forEach((plan) => {
        if (all) delete next[plan.id];
        else next[plan.id] = plan;
      });
      return next;
    });
  const submitConfirm = async () => {
    const plans = Object.values(selected).filter(
      (plan) =>
        plan.status === "pending_confirmation" && plan.project_id === projectId,
    );
    if (!plans.length) {
      setConfirmError(new ApiError("请至少选择一个待确认章节。", 400));
      return;
    }
    const controller = new AbortController();
    confirmControllerRef.current = controller;
    setConfirming(true);
    setConfirmError(null);
    try {
      await confirmChapterPlans(
        projectId,
        {
          selections: plans.map((plan) => ({
            chapter_plan_id: plan.id,
            expected_version: plan.version,
          })),
        },
        { signal: controller.signal },
      );
      if (controller.signal.aborted || projectRef.current !== projectId) return;
      clearSelection();
      await loadPlans({ status, offset });
    } catch (cause) {
      if (controller.signal.aborted || projectRef.current !== projectId) return;
      setConfirmError(
        cause instanceof ApiError
          ? cause
          : new ApiError("暂时无法确认章节规划。", 500),
      );
    } finally {
      if (!controller.signal.aborted && projectRef.current === projectId)
        setConfirming(false);
      if (confirmControllerRef.current === controller)
        confirmControllerRef.current = null;
    }
  };
  return (
    <div className="chapter-plans-workspace">
      <main className="chapter-plans-main">
        <section className="chapter-plans-heading">
          <div>
            <h2>章节规划</h2>
            <p>基于故事线、素材和伏笔生成并管理章节候选。</p>
          </div>
          <div className="chapter-plans-actions">
            <button onClick={() => setMockOpen(true)}>
              <Icon name="wand" size={17} />
              模拟生成
            </button>
          </div>
        </section>
        <p className="chapter-plans-notice">
          <Icon name="info" size={18} />
          候选章节可编辑或删除；确认后将锁定，并可进入正文生产。
        </p>
        <nav className="chapter-plans-filters" aria-label="章节状态筛选">
          <button className={!status ? "active" : ""} onClick={() => choose()}>
            全部
          </button>
          {(["pending_confirmation", "confirmed"] as ChapterPlanStatus[]).map((value) => (
            <button
              className={status === value ? "active" : ""}
              onClick={() => choose(value)}
              key={value}
            >
              {chapterPlanStatusLabel(value)}
            </button>
          ))}
        </nav>
        {loading ? (
          <LoadingList />
        ) : data.total === 0 ? (
          <section className="chapter-plans-empty">
            <Icon name="book" size={34} />
            <h3>暂无章节规划</h3>
            <p>
              {status
                ? "当前筛选条件下没有章节规划。"
                : "请先模拟生成章节规划候选。"}
            </p>
            {status && <button onClick={() => choose()}>查看全部</button>}
          </section>
        ) : (
          <>
            <div className="chapter-plan-select-all">
              <label>
                <input
                  type="checkbox"
                  checked={allPendingSelected}
                  onChange={toggleAll}
                  disabled={!pending.length}
                />
                <span>全选待确认章节</span>
              </label>
              <span>{pending.length} 条待确认</span>
            </div>
            <section className="chapter-plans-list" aria-live="polite">
              {data.items.map((plan) => (
                <Card
                  key={plan.id}
                  plan={plan}
                  relations={relations}
                  selected={Boolean(selected[plan.id])}
                  onToggle={() => toggle(plan)}
                  onEdit={() => setEditing(plan)}
                />
              ))}
            </section>
            <footer className="chapter-plans-pagination">
              <span>共 {data.total} 条记录</span>
              <div>
                <button
                  disabled={page === 1}
                  onClick={() => changePage(Math.max(0, offset - limit))}
                >
                  上一页
                </button>
                <b>
                  {page} / {pages}
                </b>
                <button
                  disabled={page === pages}
                  onClick={() => changePage(offset + limit)}
                >
                  下一页
                </button>
              </div>
            </footer>
          </>
        )}
      </main>
      {selectedPlans.length > 0 && (
        <footer className="chapter-plan-batch-bar">
          <div>
            <p>
              已选择 <b>{selectedPlans.length}</b> 个候选章节
            </p>
            <button type="button" onClick={clearSelection}>
              取消选择
            </button>
          </div>
          <div>
            <p>
              确认后，所选候选章节将变为已确认，并允许用户手动进入正文生产。
            </p>
            <button
              type="button"
              onClick={() => {
                setConfirmError(null);
                setConfirmOpen(true);
              }}
            >
              确认章节规划
            </button>
          </div>
        </footer>
      )}
      {mockOpen && (
        <MockGenerateDialog
          projectId={projectId}
          onClose={() => setMockOpen(false)}
          onGenerated={refreshAfterGeneration}
        />
      )}{" "}
      {editing && (
        <EditChapterPlanDrawer
          projectId={projectId}
          plan={editing}
          onClose={() => setEditing(null)}
          onSaved={refreshAfterGeneration}
        />
      )}{" "}
      {confirmOpen && (
        <ConfirmChapterPlansDialog
          plans={selectedPlans}
          onClose={() => {
            if (!confirming) {
              setConfirmOpen(false);
              setConfirmError(null);
            }
          }}
          onConfirm={() => void submitConfirm()}
          submitting={confirming}
          error={confirmError}
        />
      )}
    </div>
  );
}
function Card({
  plan,
  relations,
  selected,
  onToggle,
  onEdit,
}: {
  plan: ChapterPlan;
  relations: RelationContext | null;
  selected: boolean;
  onToggle: () => void;
  onEdit: () => void;
}) {
  const selectable = plan.status === "pending_confirmation";
  const names = relations ? createRelationNames(relations.storylines, relations.materials, relations.foreshadowings) : null;
  return (
    <article className={`chapter-plan-card ${selected ? "selected" : ""}`}>
      <header>
        <label className="chapter-plan-select">
          <input
            type="checkbox"
            checked={selected}
            onChange={onToggle}
            disabled={!selectable}
            aria-label={`选择第 ${plan.chapter_no} 章`}
          />
        </label>
        <div className="chapter-plan-number">第 {plan.chapter_no} 章</div>
        <div className="chapter-plan-title">
          <h3>{plan.title}</h3>
          <span className={`chapter-plan-status ${plan.status}`}>
            {chapterPlanStatusLabel(plan.status)}
          </span>
          <small>{chapterPlanGenerationSummary(plan)}</small>
        </div>
      </header>
      <p className="chapter-plan-summary">{plan.summary || "暂无章节摘要"}</p>
      <dl className="chapter-plan-details">
        <div>
          <dt>章节目标</dt>
          <dd>{plan.chapter_goal || "未设置"}</dd>
        </div>
        <div>
          <dt>创作备注</dt>
          <dd>{plan.creation_notes || "未设置"}</dd>
        </div>
      </dl>
      <section
        className="chapter-plan-relations"
        aria-label={`第 ${plan.chapter_no} 章关联`}
      >
        <Relation
          icon="workflow"
          label="关联故事线"
          values={names ? relationValues(plan.storyline_refs_json.map((ref) => ref.storyline_id), names.storylines, "暂无关联故事线") : ["正在加载关联故事线"]}
        />
        <Relation
          icon="archive"
          label="关联素材"
          values={names ? relationValues(plan.material_refs_json, names.materials, "暂无关联素材") : ["正在加载关联素材"]}
        />
        <Relation
          icon="sparkles"
          label="关联伏笔"
          values={names ? relationValues(plan.foreshadowing_refs_json, names.foreshadowings, "暂无关联伏笔") : ["正在加载关联伏笔"]}
        />
      </section>
      {selectable ? (
        <button
          type="button"
          className="chapter-plan-edit-button"
          onClick={onEdit}
        >
          编辑
        </button>
      ) : (
        <Link
          className="chapter-plan-edit-button"
          href={`/projects/${plan.project_id}/chapter-plans/${plan.id}/content`}
        >
          进入正文生产
        </Link>
      )}
    </article>
  );
}
function Relation({
  icon,
  label,
  values,
}: {
  icon: "workflow" | "archive" | "sparkles";
  label: string;
  values: string[];
}) {
  return (
    <div>
      <dt>
        <Icon name={icon} size={15} />
        {label}
      </dt>
      <dd>
        {values.length ? (
          values.map((value) => <span key={value}>{value}</span>)
        ) : (
          <em>无</em>
        )}
      </dd>
    </div>
  );
}
function State({
  title,
  description,
  retry,
}: {
  title: string;
  description: string;
  retry: () => void;
}) {
  return (
    <main className="chapter-plans-state">
      <Icon name="info" size={34} />
      <h1>{title}</h1>
      <p>{description}</p>
      <button onClick={retry}>重试</button>
    </main>
  );
}
function Loading() {
  return (
    <div className="chapter-plans-workspace">
      <div className="chapter-plans-skeleton header" />
      <div className="chapter-plans-skeleton tabs" />
      <main className="chapter-plans-main">
        <div className="chapter-plans-skeleton heading" />
        <LoadingList />
      </main>
    </div>
  );
}
function LoadingList() {
  return (
    <section className="chapter-plans-list">
      {[1, 2, 3].map((item) => (
        <div className="chapter-plans-skeleton card" key={item} />
      ))}
    </section>
  );
}

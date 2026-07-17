"use client";
/* eslint-disable react-hooks/set-state-in-effect */

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Icon } from "@/components/ui/icons";
import {
  ApiError,
  getForeshadowings,
  getStorylines,
  type Foreshadowing,
  type Project,
  type StorylineNode,
} from "@/lib/api";
import { listProjectMaterialsFromApi } from "@/features/planning-materials/api/project-material-http-api";
import type { ProjectMaterialItem } from "@/features/planning-materials/contracts/materials";
import {
  confirmChapterPlans,
  listChapterPlans,
  type ChapterPlan,
} from "./chapter-plan-http-api";
import { ConfirmChapterPlansDialog } from "./confirm-chapter-plans-dialog";
import { EditChapterPlanDrawer } from "./edit-chapter-plan-drawer";
import { MockGenerateDialog } from "./mock-generate-dialog";
import {
  chapterPlanDetail,
  chapterPlanSourceLabel,
  chapterPlanStatusLabel,
  chapterPlanSummary,
  createChapterPlanStats,
  createRelationNames,
  flattenStorylines,
  relationValues,
  type ChapterPlanFilterStatus,
} from "./chapter-plan-presentation";

type Relations = {
  storylines: StorylineNode[];
  materials: ProjectMaterialItem[];
  foreshadowings: Foreshadowing[];
};
const statuses: { value: ChapterPlanFilterStatus; label: string }[] = [
  { value: "all", label: "全部" },
  { value: "pending_confirmation", label: "待确认" },
  { value: "confirmed", label: "已确认" },
  { value: "draft_generated", label: "已生成草稿" },
];

export function ChapterPlansWorkspace({
  projectId,
}: {
  projectId: string;
  project: Project;
}) {
  const [plans, setPlans] = useState<ChapterPlan[] | null>(null);
  const [relations, setRelations] = useState<Relations | null>(null);
  const [status, setStatus] = useState<ChapterPlanFilterStatus>("all");
  const [search, setSearch] = useState("");
  const [storylineId, setStorylineId] = useState("");
  const [foreshadowingId, setForeshadowingId] = useState("");
  const [selected, setSelected] = useState<Record<string, ChapterPlan>>({});
  const [loading, setLoading] = useState(true),
    [error, setError] = useState<ApiError | null>(null);
  const [mockOpen, setMockOpen] = useState(false),
    [editing, setEditing] = useState<ChapterPlan | null>(null),
    [confirmOpen, setConfirmOpen] = useState(false),
    [confirming, setConfirming] = useState(false),
    [confirmError, setConfirmError] = useState<ApiError | null>(null);
  const requestRef = useRef(0);
  const load = useCallback(
    async (signal?: AbortSignal) => {
      const request = ++requestRef.current;
      setLoading(true);
      setError(null);
      try {
        const [response, storylines, materials, foreshadowings] =
          await Promise.all([
            listChapterPlans(projectId, { limit: 100 }, { signal }),
            getStorylines(projectId, signal),
            listProjectMaterialsFromApi(projectId, { limit: 100 }, { signal }),
            getForeshadowings(projectId, signal),
          ]);
        if (signal?.aborted || request !== requestRef.current) return;
        setPlans(response.items);
        setRelations({
          storylines: storylines.items,
          materials: materials.items,
          foreshadowings: foreshadowings.items,
        });
      } catch (cause) {
        if (!signal?.aborted && request === requestRef.current)
          setError(
            cause instanceof ApiError
              ? cause
              : new ApiError("Unable to load chapter plans.", 500),
          );
      } finally {
        if (!signal?.aborted && request === requestRef.current)
          setLoading(false);
      }
    },
    [projectId],
  );
  useEffect(() => {
    const controller = new AbortController();
    setSelected({});
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);
  const relationNames = useMemo(
    () =>
      relations &&
      createRelationNames(
        relations.storylines,
        relations.materials,
        relations.foreshadowings,
      ),
    [relations],
  );
  const visible = useMemo(
    () =>
      (plans ?? []).filter((plan) => {
        const needle = search.trim().toLowerCase();
        return (
          (status === "all" || plan.status === status) &&
          (!needle ||
            plan.title.toLowerCase().includes(needle) ||
            String(plan.chapter_no).includes(needle)) &&
          (!storylineId ||
            plan.storyline_refs_json.some(
              (ref) => ref.storyline_id === storylineId,
            )) &&
          (!foreshadowingId ||
            plan.foreshadowing_refs_json.includes(foreshadowingId))
        );
      }),
    [plans, status, search, storylineId, foreshadowingId],
  );
  const stats = useMemo(() => createChapterPlanStats(plans ?? []), [plans]);
  const pendingVisible = visible.filter(
    (plan) => plan.status === "pending_confirmation",
  );
  const selectedPlans = Object.values(selected);
  const clearSelection = () => {
    setSelected({});
    setConfirmOpen(false);
    setConfirmError(null);
  };
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
      const all =
        pendingVisible.length > 0 &&
        pendingVisible.every((plan) => next[plan.id]);
      pendingVisible.forEach((plan) => {
        if (all) delete next[plan.id];
        else next[plan.id] = plan;
      });
      return next;
    });
  const refresh = useCallback(async () => {
    clearSelection();
    await load();
  }, [load]);
  const submitConfirm = async () => {
    const candidates = Object.values(selected).filter(
      (plan) => plan.status === "pending_confirmation",
    );
    if (!candidates.length || confirming) return;
    setConfirming(true);
    setConfirmError(null);
    try {
      await confirmChapterPlans(projectId, {
        selections: candidates.map((plan) => ({
          chapter_plan_id: plan.id,
          expected_version: plan.version,
        })),
      });
      await refresh();
    } catch (cause) {
      setConfirmError(
        cause instanceof ApiError
          ? cause
          : new ApiError("暂时无法确认章节规划。", 500),
      );
    } finally {
      setConfirming(false);
    }
  };
  if (loading && !plans) return <Loading />;
  if (error && !plans)
    return (
      <State
        title={error.status === 404 ? "项目不存在" : "章节规划加载失败"}
        description="请检查网络连接后重试。"
        retry={() => void load()}
      />
    );
  const storylines = relations ? flattenStorylines(relations.storylines) : [];
  return (
    <div className="chapter-plans-workspace">
      <main className="chapter-plans-main">
        <section className="chapter-plans-heading">
          <div>
            <h2>章节规划</h2>
            <p>基于故事线、素材和伏笔生成并管理章节候选。</p>
          </div>
          <div className="chapter-plans-actions">
            <button type="button" onClick={() => setMockOpen(true)}>
              <Icon name="wand" size={17} />
              模拟生成章节规划
            </button>
          </div>
        </section>
        <section className="chapter-plan-stats" aria-label="章节规划统计">
          {[
            ["全部章节", stats.all],
            ["待确认", stats.pending],
            ["已确认", stats.confirmed],
            ["已生成草稿", stats.draftGenerated],
          ].map(([label, value]) => (
            <article key={String(label)}>
              <span>{label}</span>
              <b>{value}</b>
            </article>
          ))}
        </section>
        <nav className="chapter-plans-filters" aria-label="章节状态筛选">
          {statuses.map((item) => (
            <button
              type="button"
              className={status === item.value ? "active" : ""}
              onClick={() => {
                setStatus(item.value);
                clearSelection();
              }}
              key={item.value}
            >
              {item.label}{" "}
              <small>
                {item.value === "all"
                  ? stats.all
                  : item.value === "pending_confirmation"
                    ? stats.pending
                    : item.value === "confirmed"
                      ? stats.confirmed
                      : stats.draftGenerated}
              </small>
            </button>
          ))}
        </nav>
        <section
          className="chapter-plans-toolbar"
          aria-label="章节规划搜索与筛选"
        >
          <input
            aria-label="搜索章节标题或章节编号"
            placeholder="搜索章节标题或章节编号"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
          <select
            aria-label="故事线筛选"
            value={storylineId}
            onChange={(event) => setStorylineId(event.target.value)}
          >
            <option value="">全部故事线</option>
            {storylines.map((line) => (
              <option key={line.id} value={line.id}>
                {line.name}
              </option>
            ))}
          </select>
          <select
            aria-label="伏笔筛选"
            value={foreshadowingId}
            onChange={(event) => setForeshadowingId(event.target.value)}
          >
            <option value="">全部伏笔</option>
            {relations?.foreshadowings.map((item) => (
              <option key={item.id} value={item.id}>
                {item.title}
              </option>
            ))}
          </select>
          <button
            type="button"
            onClick={() => {
              setSearch("");
              setStorylineId("");
              setForeshadowingId("");
              setStatus("all");
              clearSelection();
            }}
          >
            清除筛选
          </button>
        </section>
        {error && (
          <p className="chapter-plans-form-error" role="alert">
            数据刷新失败，请重试。
          </p>
        )}
        <div className="chapter-plan-select-all">
          <label>
            <input
              type="checkbox"
              checked={
                pendingVisible.length > 0 &&
                pendingVisible.every((plan) => selected[plan.id])
              }
              onChange={toggleAll}
              disabled={!pendingVisible.length}
            />
            全选待确认章节
          </label>
          <span>已选 {selectedPlans.length} 项</span>
        </div>
        {!visible.length ? (
          <section className="chapter-plans-empty">
            <Icon name="book" size={34} />
            <h3>{plans?.length ? "未找到匹配章节" : "暂无章节规划"}</h3>
            <p>
              {plans?.length
                ? "请调整搜索或筛选条件。"
                : "请先模拟生成章节规划候选。"}
            </p>
          </section>
        ) : (
          <section className="chapter-plans-table" aria-live="polite">
            <div className="chapter-plan-row header">
              <span>选择</span>
              <span>章节</span>
              <span>标题与摘要</span>
              <span>关联故事线</span>
              <span>关联子故事线</span>
              <span>关联素材</span>
              <span>关联伏笔</span>
              <span>状态</span>
              <span>来源</span>
              <span>操作</span>
            </div>
            {visible.map((plan) => (
              <PlanRow
                key={plan.id}
                plan={plan}
                selected={Boolean(selected[plan.id])}
                names={relationNames}
                onToggle={() => toggle(plan)}
                onEdit={() => setEditing(plan)}
              />
            ))}
          </section>
        )}
      </main>
      {selectedPlans.length > 0 && (
        <footer className="chapter-plan-batch-bar">
          <div>
            <p>
              已选择 <b>{selectedPlans.length}</b> 个待确认章节
            </p>
            <button type="button" onClick={clearSelection}>
              取消选择
            </button>
          </div>
          <button
            type="button"
            onClick={() => setConfirmOpen(true)}
            disabled={confirming}
          >
            批量确认章节规划
          </button>
        </footer>
      )}
      {mockOpen && (
        <MockGenerateDialog
          projectId={projectId}
          onClose={() => setMockOpen(false)}
          onGenerated={async () => {
            setMockOpen(false);
            await refresh();
          }}
        />
      )}
      {editing && (
        <EditChapterPlanDrawer
          projectId={projectId}
          plan={editing}
          onClose={() => setEditing(null)}
          onSaved={refresh}
        />
      )}
      {confirmOpen && (
        <ConfirmChapterPlansDialog
          plans={selectedPlans}
          allPlans={plans ?? []}
          onClose={() => !confirming && setConfirmOpen(false)}
          onConfirm={() => void submitConfirm()}
          submitting={confirming}
          error={confirmError}
        />
      )}
    </div>
  );
}
function PlanRow({
  plan,
  selected,
  names,
  onToggle,
  onEdit,
}: {
  plan: ChapterPlan;
  selected: boolean;
  names: ReturnType<typeof createRelationNames> | null;
  onToggle: () => void;
  onEdit: () => void;
}) {
  const refs = plan.storyline_refs_json;
  const main = refs
    .filter((ref) => ref.relation === "primary")
    .map((ref) => ref.storyline_id);
  const children = refs
    .filter((ref) => ref.relation === "secondary")
    .map((ref) => ref.storyline_id);
  return (
    <article className="chapter-plan-row">
      <span>
        <input
          type="checkbox"
          aria-label={`选择第 ${plan.chapter_no} 章`}
          checked={selected}
          onChange={onToggle}
          disabled={plan.status !== "pending_confirmation"}
        />
      </span>
      <b>第 {plan.chapter_no} 章</b>
      <div>
        <strong>{plan.title}</strong>
        <p>{chapterPlanSummary(plan.summary)}</p>
        <small>{chapterPlanDetail(plan.chapter_goal, "未设置章节目标")}</small>
      </div>
      <Badges
        values={
          names ? relationValues(main, names.storylines, "—") : ["加载中"]
        }
      />
      <Badges
        values={
          names ? relationValues(children, names.storylines, "—") : ["加载中"]
        }
      />
      <Badges
        values={
          names
            ? relationValues(plan.material_refs_json, names.materials, "—")
            : ["加载中"]
        }
      />
      <Badges
        values={
          names
            ? relationValues(
                plan.foreshadowing_refs_json,
                names.foreshadowings,
                "—",
              )
            : ["加载中"]
        }
      />
      <span className={`chapter-plan-status ${plan.status}`}>
        {chapterPlanStatusLabel(plan.status)}
      </span>
      <span>{chapterPlanSourceLabel(plan.source)}</span>
      {plan.status === "pending_confirmation" ? (
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
function Badges({ values }: { values: string[] }) {
  return (
    <span className="chapter-plan-badges">
      {values.map((value) => (
        <i key={value}>{value}</i>
      ))}
    </span>
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
      <button type="button" onClick={retry}>
        重试
      </button>
    </main>
  );
}
function Loading() {
  return (
    <div className="chapter-plans-workspace">
      <main className="chapter-plans-main">
        <div className="chapter-plans-skeleton heading" />
        <div className="chapter-plans-skeleton card" />
      </main>
    </div>
  );
}

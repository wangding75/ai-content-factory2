"use client";
/* eslint-disable react-hooks/set-state-in-effect */

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useIdempotency } from "./use-idempotency";
import { chapterPlanCacheEvent, invalidateChapterPlanViews } from "./chapter-plan-cache";
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
  createChapterPlanRun,
  getProjectChapterPlanningSummary,
  listChapterPlans,
  preflightChapterPlanRun,
  type ChapterPlan,
  type ChapterPlanningPreflightReport,
  type ChapterPlanningPreflightRequest,
  type ChapterPlanningSummary,
} from "./chapter-plan-http-api";
import { ConfirmChapterPlansDialog } from "./confirm-chapter-plans-dialog";
import { EditChapterPlanDrawer } from "./edit-chapter-plan-drawer";
import { GenerationSettingsDrawer } from "./generation-settings-drawer";
import {
  PreflightProgressDialog,
  PreflightReportDialog,
  RunCreatedDialog,
} from "./preflight-dialogs";
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
  const { getOrCreateKey, clearKey, markUnknown } = useIdempotency();
  const [plans, setPlans] = useState<ChapterPlan[] | null>(null);
  const [summary, setSummary] = useState<ChapterPlanningSummary | null>(null);
  const [summaryError, setSummaryError] = useState<ApiError | null>(null);
  const [relations, setRelations] = useState<Relations | null>(null);

  const [status, setStatus] = useState<ChapterPlanFilterStatus>("all");
  const [search, setSearch] = useState("");
  const [storylineId, setStorylineId] = useState("");
  const [foreshadowingId, setForeshadowingId] = useState("");
  const [selected, setSelected] = useState<Record<string, ChapterPlan>>({});

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<ApiError | null>(null);

  // Modals & Drawers
  const [editing, setEditing] = useState<ChapterPlan | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [confirmError, setConfirmError] = useState<ApiError | null>(null);

  // Preflight & Run Generation State
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [preflighting, setPreflighting] = useState(false);
  const [preflightReport, setPreflightReport] =
    useState<ChapterPlanningPreflightReport | null>(null);
  const [creatingRun, setCreatingRun] = useState(false);
  const [createdRunId, setCreatedRunId] = useState<string | null>(null);

  const requestRef = useRef(0);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      const request = ++requestRef.current;
      setLoading(true);
      setError(null);
      setSummaryError(null);

      try {
        const [plansRes, summaryRes, storylinesRes, materialsRes, foreshadowingsRes] =
          await Promise.allSettled([
            listChapterPlans(projectId, { limit: 100 }, { signal }),
            getProjectChapterPlanningSummary(projectId, { signal }),
            getStorylines(projectId, signal),
            listProjectMaterialsFromApi(projectId, { limit: 100 }, { signal }),
            getForeshadowings(projectId, signal),
          ]);

        if (signal?.aborted || request !== requestRef.current) return;

        if (plansRes.status === "fulfilled") {
          setPlans(plansRes.value.items);
        } else {
          setError(
            plansRes.reason instanceof ApiError
              ? plansRes.reason
              : new ApiError("Unable to load chapter plans.", 500),
          );
        }

        if (summaryRes.status === "fulfilled") {
          setSummary(summaryRes.value);
        } else if (summaryRes.reason instanceof ApiError) {
          setSummaryError(summaryRes.reason);
        }

        setRelations({
          storylines: storylinesRes.status === "fulfilled" ? storylinesRes.value.items : [],
          materials: materialsRes.status === "fulfilled" ? materialsRes.value.items : [],
          foreshadowings: foreshadowingsRes.status === "fulfilled" ? foreshadowingsRes.value.items : [],
        });
      } catch (cause) {
        if (!signal?.aborted && request === requestRef.current) {
          setError(
            cause instanceof ApiError
              ? cause
              : new ApiError("Unable to load chapter plans.", 500),
          );
        }
      } finally {
        if (!signal?.aborted && request === requestRef.current) {
          setLoading(false);
        }
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

  useEffect(() => {
    const refreshAfterMutation = (event: Event) => {
      if ((event as CustomEvent<{ projectId?: string }>).detail?.projectId === projectId) {
        void load();
      }
    };
    window.addEventListener(chapterPlanCacheEvent, refreshAfterMutation);
    return () => window.removeEventListener(chapterPlanCacheEvent, refreshAfterMutation);
  }, [load, projectId]);

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

  // Preflight Flow Handler
  const handlePreflightSubmit = async (requestPayload: ChapterPlanningPreflightRequest) => {
    setPreflighting(true);
    setError(null);
    try {
      const report = await preflightChapterPlanRun(projectId, requestPayload);
      setSettingsOpen(false);
      setPreflightReport(report);
    } catch (cause) {
      if (cause instanceof ApiError) {
        setError(cause);
      } else {
        setError(new ApiError("预检发起失败，请稍后重试。", 500));
      }
    } finally {
      setPreflighting(false);
    }
  };

  // Create Run Handler
  const handleCreateRun = async (preflightToken: string) => {
    setCreatingRun(true);
    const scope = `chapter-plan-run:create:${projectId}`;
    const payload = { preflightToken };
    try {
      const idempotencyKey = await getOrCreateKey(scope, payload);
      const result = await createChapterPlanRun(
        projectId,
        payload,
        idempotencyKey,
      );

      clearKey(scope);
      invalidateChapterPlanViews(projectId);
      const runId = result?.id || "run-created";
      setPreflightReport(null);
      setCreatedRunId(runId);
      await refresh();
    } catch (cause) {
      if (
        cause instanceof ApiError &&
        (cause.status === 0 || cause.status >= 500 || cause.code === "timeout")
      ) {
        await markUnknown(scope, payload);
      }
      if (cause instanceof ApiError) {
        setError(cause);
      } else {
        setError(new ApiError("创建生成任务失败，请重试。", 500));
      }
    } finally {
      setCreatingRun(false);
    }
  };

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
      invalidateChapterPlanViews(projectId);
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
        {/* Active Run / Summary Banner */}
        <SummaryRunBanner
          summary={summary}
          summaryError={summaryError}
          onConfigure={() => setSettingsOpen(true)}
          onRetry={() => void load()}
        />

        <section className="chapter-plans-heading">
          <div>
            <h2>章节规划</h2>
            <p>基于故事线、素材和伏笔生成并管理章节候选。</p>
          </div>
          <div className="chapter-plans-actions">
            <button
              type="button"
              className="chapter-plan-button primary"
              onClick={() => setSettingsOpen(true)}
            >
              <Icon name="wand" size={17} />
              生成章节规划
            </button>
          </div>
        </section>

        {/* Batch Counts & Summary Cards */}
        {summary?.candidateBatchCounts && (
          <section className="chapter-plan-batch-summary" aria-label="候选批次概览">
            <div className="batch-summary-item">
              <span>待处理批次</span>
              <b>{summary.candidateBatchCounts.ready}</b>
            </div>
            <div className="batch-summary-item">
              <span>部分采用</span>
              <b>{summary.candidateBatchCounts.partiallyAdopted}</b>
            </div>
            <div className="batch-summary-item">
              <span>已全部采用</span>
              <b>{summary.candidateBatchCounts.adopted}</b>
            </div>
            <div className="batch-summary-item">
              <span>已放弃批次</span>
              <b>{summary.candidateBatchCounts.abandoned}</b>
            </div>
            <div className="batch-summary-link">
              <Link href={`/projects/${projectId}/chapter-plan-candidate-batches`}>
                查看全量候选批次 →
              </Link>
            </div>
          </section>
        )}

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
                : "请先生成章节规划候选。"}
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

      {/* Generation Settings Drawer */}
      {settingsOpen && (
        <GenerationSettingsDrawer
          storylines={relations?.storylines ?? []}
          onClose={() => setSettingsOpen(false)}
          onSubmit={(payload) => void handlePreflightSubmit(payload)}
          submitting={preflighting}
        />
      )}

      {/* Preflight Progress Modal */}
      {preflighting && <PreflightProgressDialog />}

      {/* Preflight Report Modal (Passed / Blocked) */}
      {preflightReport && (
        <PreflightReportDialog
          report={preflightReport}
          onClose={() => setPreflightReport(null)}
          onCreateRun={(token) => void handleCreateRun(token)}
          creatingRun={creatingRun}
        />
      )}

      {/* Run Created Dialog */}
      {createdRunId && (
        <RunCreatedDialog
          runId={createdRunId}
          onClose={() => setCreatedRunId(null)}
        />
      )}

      {editing && (
        <EditChapterPlanDrawer
          projectId={projectId}
          plan={editing}
          onClose={() => setEditing(null)}
          onSaved={async () => {
            invalidateChapterPlanViews(projectId);
            await refresh();
          }}
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

function SummaryRunBanner({
  summary,
  summaryError,
  onConfigure,
  onRetry,
}: {
  summary: ChapterPlanningSummary | null;
  summaryError: ApiError | null;
  onConfigure: () => void;
  onRetry: () => void;
}) {
  if (summaryError) {
    const isNotConfigured =
      summaryError.status === 422 ||
      summaryError.message?.includes("workflow_not_configured");
    const isAtomicFailed =
      summaryError.status === 500 ||
      summaryError.message?.includes("output_validation_failed") ||
      summaryError.message?.includes("result_consumption_failed");

    if (isNotConfigured) {
      return (
        <div className="chapter-plan-run-banner warning">
          <Icon name="info" size={20} />
          <div className="banner-content">
            <strong>未配置章节规划工作流</strong>
            <p>请先选择或配置可用的章节规划工作流绑定再发起生成。</p>
          </div>
          <button type="button" onClick={onConfigure}>
            配置并生成
          </button>
        </div>
      );
    }

    if (isAtomicFailed) {
      return (
        <div className="chapter-plan-run-banner error">
          <Icon name="info" size={20} />
          <div className="banner-content">
            <strong>生成失败且零候选写入</strong>
            <p>
              章节规划生成输出校验失败或数据入库失败，尚未写入候选。
            </p>
          </div>
          <button type="button" onClick={onRetry}>
            重试
          </button>
        </div>
      );
    }

    return null;
  }

  if (!summary?.activeRun) return null;

  const run = summary.activeRun;
  const status = run.status;

  const isRunning = status === "running" || status === "queued";
  const isValidating = status === "validating";

  return (
    <div className={`chapter-plan-run-banner ${isRunning ? "info" : "warning"}`}>
      <div className="chapter-plan-spinner" />
      <div className="banner-content">
        <strong>
          {isValidating
            ? "局部范围生成中（正在校验输出结果...）"
            : `章节规划运行中 (Run: ${run.runNumber || run.id.slice(0, 8)})`}
        </strong>
        <p>
          当前状态：{status === "queued" ? "等待执行" : status === "running" ? "运行中" : status}
          {run.createdAt ? ` · 开始于 ${new Date(run.createdAt).toLocaleTimeString("zh-CN")}` : ""}
        </p>
      </div>
      <button type="button" onClick={onRetry}>
        刷新状态
      </button>
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

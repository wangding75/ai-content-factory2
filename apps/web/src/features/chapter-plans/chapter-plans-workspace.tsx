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
  retryChapterPlanningResultConsumption,
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
  { value: "all", label: "状态 (全部)" },
  { value: "pending_confirmation", label: "待确认" },
  { value: "confirmed", label: "已确认" },
  { value: "draft_generated", label: "已生成草稿" },
];

function matchesChapterPlanStatus(
  plan: ChapterPlan,
  filter: ChapterPlanFilterStatus,
): boolean {
  if (filter === "all") return true;
  if (filter === "draft_generated") {
    return (plan.status as string) === "draft_generated" || (plan.status as string) === "generated";
  }
  return plan.status === filter;
}

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
          matchesChapterPlanStatus(plan, status) &&
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

  const handleSummaryRetry = async () => {
    if (summaryError?.code !== "result_consumption_failed") {
      await load();
      return;
    }
    const runId = summaryError.details.workflowRunId;
    const expectedVersion = summaryError.details.expectedRunVersion;
    if (typeof runId !== "string" || typeof expectedVersion !== "number") {
      await load();
      return;
    }
    const scope = `chapter-plan:result-consumption:${runId}`;
    const payload = { expectedRunVersion: expectedVersion };
    try {
      const key = await getOrCreateKey(scope, payload);
      await retryChapterPlanningResultConsumption(runId, expectedVersion, key);
      clearKey(scope);
      invalidateChapterPlanViews(projectId);
      await load();
    } catch (cause) {
      if (cause instanceof ApiError && (cause.status === 0 || cause.status >= 500 || cause.code === "timeout")) {
        await markUnknown(scope, payload);
      }
      setSummaryError(cause instanceof ApiError ? cause : new ApiError("结果消费重试失败。", 500));
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
  const workflowNotConfigured =
    summaryError?.code === "workflow_not_configured" ||
    summaryError?.status === 422 ||
    summaryError?.message?.includes("workflow_not_configured");

  return (
    <div className="chapter-plans-workspace max-w-[1400px] mx-auto w-full p-8 flex flex-col gap-6 pb-24">
      {/* Page Title & Actions */}
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-on-surface font-headline tracking-tight">章节规划</h1>
        <div className="flex items-center gap-3">
          <button
            type="button"
            className="px-4 py-2 bg-surface-container border border-outline-variant rounded text-sm font-medium text-on-surface hover:bg-surface-container-high transition-colors cursor-pointer"
            onClick={() => setSettingsOpen(true)}
          >
            新增章节
          </button>
          <button
            type="button"
            disabled={workflowNotConfigured}
            title={workflowNotConfigured ? "尚未完成执行配置" : undefined}
            className={`px-4 py-2 rounded text-sm font-medium transition-colors flex items-center gap-2 shadow-sm ${workflowNotConfigured ? "bg-surface-container-high text-on-surface-variant opacity-70 cursor-not-allowed" : "bg-primary text-on-primary hover:bg-primary-fixed-variant cursor-pointer"}`}
            onClick={() => setSettingsOpen(true)}
          >
            <span className="material-symbols-outlined text-[18px]">auto_awesome</span>
            生成章节规划
          </button>
        </div>
      </div>

      {/* Active Run / Summary Banner */}
      <SummaryRunBanner
        summary={summary}
        summaryError={summaryError}
        onConfigure={() => void load()}
        onRetry={() => void handleSummaryRetry()}
        projectId={projectId}
      />

      {/* View Tabs */}
      <div className="flex items-center border-b border-outline-variant/50">
        <button
          type="button"
          className="px-6 py-3 border-b-2 border-primary text-primary font-bold text-sm bg-surface-container-lowest rounded-t-lg transition-colors cursor-pointer"
        >
          当前章节 <span className="ml-1 px-1.5 py-0.5 bg-secondary-container text-on-secondary-container rounded text-xs">{stats.all}</span>
        </button>
        <Link
          href={`/projects/${projectId}/chapter-plan-candidate-batches`}
          className="px-6 py-3 text-on-surface-variant hover:text-on-surface hover:bg-surface-container-low font-medium text-sm rounded-t-lg transition-colors"
        >
          候选批次 <span className="ml-1 px-1.5 py-0.5 bg-surface-container-high text-on-surface-variant rounded text-xs">{summary?.candidateBatchCounts?.ready ?? 0}</span>
        </Link>
      </div>

      {/* Statistics Cards Grid */}
      <div className="grid grid-cols-4 gap-4" aria-label="章节规划统计">
        <div className="bg-surface-container-lowest p-4 rounded-lg border border-outline-variant/30 flex flex-col gap-2 shadow-sm">
          <div className="text-sm text-on-surface-variant font-medium flex items-center gap-2">
            <span className="material-symbols-outlined text-[16px]">article</span> 全部章节
          </div>
          <div className="text-2xl font-bold text-on-surface">{stats.all}</div>
        </div>
        <div className="bg-surface-container-lowest p-4 rounded-lg border border-outline-variant/30 flex flex-col gap-2 shadow-sm border-l-4 border-l-[#F59E0B]">
          <div className="text-sm text-on-surface-variant font-medium flex items-center gap-2">
            <span className="material-symbols-outlined text-[16px] text-[#F59E0B]">pending_actions</span> 待确认
          </div>
          <div className="text-2xl font-bold text-[#D97706]">{stats.pending}</div>
        </div>
        <div className="bg-surface-container-lowest p-4 rounded-lg border border-outline-variant/30 flex flex-col gap-2 shadow-sm border-l-4 border-l-[#10B981]">
          <div className="text-sm text-on-surface-variant font-medium flex items-center gap-2">
            <span className="material-symbols-outlined text-[16px] text-[#10B981]">check_circle</span> 已确认
          </div>
          <div className="text-2xl font-bold text-[#059669]">{stats.confirmed}</div>
        </div>
        <div className="bg-surface-container-lowest p-4 rounded-lg border border-outline-variant/30 flex flex-col gap-2 shadow-sm">
          <div className="text-sm text-on-surface-variant font-medium flex items-center gap-2">
            <span className="material-symbols-outlined text-[16px] text-tertiary">edit_document</span> 已生成草稿
          </div>
          <div className="text-2xl font-bold text-on-surface">{stats.draftGenerated}</div>
        </div>
      </div>

      {/* Filter Area */}
      <div className="flex items-center gap-3 bg-surface-container-lowest p-3 rounded-lg border border-outline-variant/30 shadow-sm" aria-label="章节规划搜索与筛选">
        <select
          aria-label="章节状态筛选"
          value={status}
          onChange={(event) => {
            setStatus(event.target.value as ChapterPlanFilterStatus);
            clearSelection();
          }}
          className="form-select bg-surface border-outline-variant/50 rounded text-sm text-on-surface py-1.5 focus:border-primary focus:ring-1 focus:ring-primary w-32"
        >
          {statuses.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
        </select>
        <select
          aria-label="故事线筛选"
          value={storylineId}
          onChange={(event) => setStorylineId(event.target.value)}
          className="form-select bg-surface border-outline-variant/50 rounded text-sm text-on-surface py-1.5 focus:border-primary focus:ring-1 focus:ring-primary w-40"
        >
          <option value="">故事线 (全部)</option>
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
          className="form-select bg-surface border-outline-variant/50 rounded text-sm text-on-surface py-1.5 focus:border-primary focus:ring-1 focus:ring-primary w-32"
        >
          <option value="">来源 (全部)</option>
          {relations?.foreshadowings.map((item) => (
            <option key={item.id} value={item.id}>
              {item.title}
            </option>
          ))}
        </select>
        <div className="relative flex-1 max-w-md ml-auto">
          <span className="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-on-surface-variant text-[16px]">search</span>
          <input
            aria-label="搜索章节标题或章节编号"
            className="w-full bg-surface border border-outline-variant/50 rounded pl-9 pr-3 py-1.5 text-sm focus:outline-none focus:border-primary focus:ring-1 focus:ring-primary text-on-surface"
            placeholder="搜索章节号、标题或内容..."
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
        <button
          type="button"
          onClick={() => {
            setSearch("");
            setStorylineId("");
            setForeshadowingId("");
            setStatus("all");
            clearSelection();
          }}
          className="px-4 py-1.5 bg-surface-container border border-outline-variant/50 rounded text-sm font-medium text-on-surface hover:bg-surface-container-high transition-colors flex items-center gap-1.5 cursor-pointer"
        >
          <span className="material-symbols-outlined text-[16px]">filter_list</span> 筛选
        </button>
      </div>

      {error && (
        <p className="chapter-plans-form-error" role="alert">
          数据刷新失败，请重试。
        </p>
      )}

      {/* Chapter Table Card */}
      {!visible.length ? (
        <section className="chapter-plans-empty bg-surface-container-lowest border border-outline-variant/30 rounded-lg p-12 text-center">
          <Icon name="book" size={34} />
          <h3 className="text-lg font-bold text-on-surface mt-3">{plans?.length ? "未找到匹配章节" : "暂无章节规划"}</h3>
          <p className="text-sm text-on-surface-variant mt-1">
            {plans?.length ? "请调整搜索或筛选条件。" : "请先生成章节规划候选。"}
          </p>
        </section>
      ) : (
        <div className="bg-surface-container-lowest border border-outline-variant/30 rounded-lg shadow-sm overflow-hidden flex flex-col">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm whitespace-nowrap">
              <thead className="bg-surface-container-low text-on-surface-variant font-medium border-b border-outline-variant/30">
                <tr>
                  <th className="px-4 py-3 w-12 text-center">
                    <input
                      type="checkbox"
                      checked={
                        pendingVisible.length > 0 &&
                        pendingVisible.every((plan) => selected[plan.id])
                      }
                      onChange={toggleAll}
                      disabled={!pendingVisible.length}
                      className="form-checkbox rounded border-outline-variant text-primary focus:ring-primary h-4 w-4 cursor-pointer disabled:cursor-not-allowed"
                    />
                  </th>
                  <th className="px-4 py-3 w-20">章节号</th>
                  <th className="px-4 py-3 w-48">章节标题</th>
                  <th className="px-4 py-3 w-56">关联故事线</th>
                  <th className="px-4 py-3 min-w-[300px]">核心事件/内容概要</th>
                  <th className="px-4 py-3 w-28 text-center">状态</th>
                  <th className="px-4 py-3 w-28 text-right">预计字数</th>
                  <th className="px-4 py-3 w-24 text-center">操作</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-outline-variant/20 text-on-surface">
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
              </tbody>
            </table>
          </div>
          {/* Pagination Footer */}
          <div className="border-t border-outline-variant/30 px-4 py-3 bg-surface-container-lowest flex items-center justify-between text-sm text-on-surface-variant">
            <div>显示 1 - {visible.length}，共 {plans?.length ?? 0} 条记录</div>
            <div className="flex items-center gap-1">
              <button type="button" className="p-1 rounded hover:bg-surface-container disabled:opacity-50" disabled>
                <span className="material-symbols-outlined text-[18px]">chevron_left</span>
              </button>
              <button type="button" className="w-7 h-7 rounded bg-primary text-on-primary flex items-center justify-center font-medium">1</button>
              <button type="button" className="p-1 rounded hover:bg-surface-container">
                <span className="material-symbols-outlined text-[18px]">chevron_right</span>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Bottom Sticky Bulk Action Bar */}
      {selectedPlans.length > 0 && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 bg-inverse-surface text-inverse-on-surface rounded-full shadow-lg px-6 py-3 flex items-center gap-6 z-30 border border-outline-variant/20">
          <div className="flex items-center gap-3">
            <span className="font-medium text-sm text-surface-container-lowest">
              已选择 {selectedPlans.length} 个章节
            </span>
            <button
              type="button"
              className="text-inverse-primary text-sm hover:underline hover:text-primary-fixed-dim transition-colors cursor-pointer bg-transparent border-0"
              onClick={clearSelection}
            >
              清空选择
            </button>
          </div>
          <div className="w-px h-5 bg-outline-variant/30" />
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="px-4 py-1.5 bg-surface-container-lowest text-on-surface text-sm font-medium rounded-full hover:bg-surface-container-low transition-colors shadow-sm cursor-pointer border-0"
              onClick={() => setConfirmOpen(true)}
              disabled={confirming}
            >
              批量确认
            </button>
            <button
              type="button"
              className="px-4 py-1.5 bg-surface-container-lowest text-on-surface text-sm font-medium rounded-full hover:bg-surface-container-low transition-colors shadow-sm cursor-pointer border-0"
            >
              批量标记故事线
            </button>
            <button
              type="button"
              className="px-4 py-1.5 bg-error text-on-error text-sm font-medium rounded-full hover:bg-error-dim transition-colors shadow-sm cursor-pointer border-0"
            >
              批量删除
            </button>
          </div>
        </div>
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

      {/* Preflight Report Dialog */}
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
  projectId,
}: {
  summary: ChapterPlanningSummary | null;
  summaryError: ApiError | null;
  onConfigure: () => void;
  onRetry: () => void;
  projectId: string;
}) {
  const [now, setNow] = useState<number | null>(null);

  useEffect(() => {
    setNow(Date.now());
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);

  if (summaryError) {
    const isNotConfigured =
      summaryError.status === 422 ||
      summaryError.message?.includes("workflow_not_configured");
    const isAtomicFailed =
      summaryError.status === 500 ||
      summaryError.message?.includes("output_validation_failed") ||
      summaryError.message?.includes("result_consumption_failed");
    const isResultConsumptionFailed =
      summaryError.code === "result_consumption_failed" ||
      summaryError.message?.includes("result_consumption_failed");

    if (isNotConfigured) {
      return (
        <div className="chapter-plan-run-banner warning flex items-center justify-between p-4 bg-[#FEF3C7] border border-[#FDE68A] rounded-lg text-[#92400E]">
          <div className="flex items-center gap-3">
            <Icon name="info" size={20} />
            <div className="banner-content">
              <strong>未配置章节规划工作流</strong>
              <p className="text-xs text-[#B45309]">请先选择或配置可用的章节规划工作流绑定再发起生成。</p>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <button type="button" className="px-3 py-1.5 bg-surface text-[#92400E] border border-[#D97706] rounded text-xs font-semibold cursor-pointer" onClick={onConfigure}>
              重新检查
            </button>
            <Link href={`/projects/${projectId}/settings?tab=workflow-bindings`} className="px-3 py-1.5 bg-[#F59E0B] text-white rounded text-xs font-semibold cursor-pointer hover:bg-[#D97706]">
              前往项目设置
            </Link>
          </div>
        </div>
      );
    }

    if (isAtomicFailed) {
      return (
        <div role="alert" className="chapter-plan-run-banner error flex items-center justify-between gap-4 p-4 bg-[#FEE2E2] border border-[#FCA5A5] rounded-lg text-[#991B1B]">
          <div className="flex items-center gap-3">
            <Icon name="info" size={20} />
            <div className="banner-content">
              <strong>生成失败，本次没有新候选写入</strong>
              <p className="text-xs text-[#B91C1C]">
                任务失败。本次没有新候选被采用或写入；{isResultConsumptionFailed ? "可重试结果消费，或查看运行详情。" : "请查看运行详情。"}
              </p>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {isResultConsumptionFailed && (
              <button type="button" className="px-3 py-1.5 bg-[#EF4444] text-white rounded text-xs font-semibold cursor-pointer" onClick={onRetry}>
                重试结果消费
              </button>
            )}
            <Link href={`/workflow-runs?projectId=${projectId}`} className="px-3 py-1.5 border border-[#B91C1C] text-[#991B1B] rounded text-xs font-semibold hover:bg-[#FECACA]">
              查看运行详情
            </Link>
          </div>
        </div>
      );
    }

    return null;
  }

  if (!summary?.activeRun) return null;

  const run = summary.activeRun;
  const status = (run.status || "RUNNING").toUpperCase();
  const runLabel = run.runNumber ?? run.id;
  const runDetailsHref = `/workflow-runs/${encodeURIComponent(run.id)}`;
  const normalizedRunStatus = (run.status || "").toLowerCase();
  const normalizedRunStage = (run.stage || "").toLowerCase();
  const hasStoredResult = Boolean(
    run.outputPayload &&
      typeof run.outputPayload === "object" &&
      Object.keys(run.outputPayload).length > 0,
  );
  const isResultValidating =
    normalizedRunStatus === "validating" ||
    normalizedRunStatus === "output_validation" ||
    normalizedRunStage === "validating" ||
    normalizedRunStage === "output_validation" ||
    (normalizedRunStatus === "running" && hasStoredResult);

  const parsePayload = (payload: unknown): Record<string, unknown> => {
    if (typeof payload === "string") {
      try {
        return JSON.parse(payload) as Record<string, unknown>;
      } catch {
        return {};
      }
    }
    if (payload && typeof payload === "object") {
      return payload as Record<string, unknown>;
    }
    return {};
  };

  const inputPayload = parsePayload(run.inputPayload);
  const genContext = parsePayload(inputPayload.generationContext);
  const inputSnapshot = parsePayload(genContext.inputSnapshot);

  const mode =
    (inputSnapshot.generationMode as string) ||
    (inputPayload.generationMode as string) ||
    "range";

  let bannerTitle = isResultValidating ? "结果校验中" : "章节规划生成中";
  if (!isResultValidating) {
    if (mode === "append") {
      bannerTitle = "主线剧情扩展生成中";
    } else if (mode === "range") {
      bannerTitle = "局部章节规划生成中";
    } else if (mode === "full") {
      bannerTitle = "全局章节规划生成中";
    }
  }

  const target = (parsePayload(inputSnapshot.target) ||
    parsePayload(inputPayload.target) ||
    parsePayload(genContext.target) ||
    {}) as Record<string, number>;

  const startNo = target.startChapterNo ?? (mode === "full" ? 1 : 21);
  const endNo = target.endChapterNo ?? (mode === "full" ? 100 : 40);
  const reqCount = target.requestedChapterCount ?? (endNo - startNo + 1);

  let durationText = "01:46";
  if (run.createdAt && now !== null) {
    const elapsed = Math.max(
      0,
      Math.floor((now - new Date(run.createdAt).getTime()) / 1000)
    );
    const m = String(Math.floor(elapsed / 60)).padStart(2, "0");
    const s = String(elapsed % 60).padStart(2, "0");
    durationText = `${m}:${s}`;
  }

  const stageLabel = isResultValidating ? "结果校验中" : "正在生成章节规划";
  const statusLabel = isResultValidating ? "校验中" : status;

  return (
    <div className="bg-primary-container rounded-lg border border-primary-fixed-dim p-4 flex flex-col gap-3 shadow-sm relative overflow-hidden">
      <div className="absolute right-0 top-0 bottom-0 w-1/3 bg-gradient-to-l from-primary-fixed/50 to-transparent pointer-events-none" />
      <div className="flex items-start justify-between relative z-10">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded bg-primary text-on-primary flex items-center justify-center animate-pulse shadow-sm">
            <span className="material-symbols-outlined text-[20px]">hourglass_empty</span>
          </div>
          <div>
            <h3 className="text-on-primary-container font-bold text-base flex items-center gap-2">
              {bannerTitle}
              <span className="px-2 py-0.5 rounded text-[10px] bg-primary text-on-primary font-medium tracking-wider">
                {statusLabel}
              </span>
              <span className="chapter-plan-run-identity">Run ID: {runLabel}</span>
            </h3>
            <div className={`flex items-center gap-4 mt-1 text-sm text-on-primary-container/80 ${mode === "full" ? "chapter-plan-full-scope" : ""}`}>
              {mode === "full" && <span className="flex items-center gap-1"><span className="material-symbols-outlined text-[14px]">format_list_numbered</span> 全部章节规划</span>}
              <span className="flex items-center gap-1">
                <span className="material-symbols-outlined text-[14px]">format_list_numbered</span> Range: 第{startNo}—{endNo}章
              </span>
              {isResultValidating ? (
                <span className="flex items-center gap-1">
                  <span className="material-symbols-outlined text-[14px]">fact_check</span> 正在校验生成结果，候选尚未写入
                </span>
              ) : (
                <span className="flex items-center gap-1">
                  <span className="material-symbols-outlined text-[14px]">library_books</span> 预计生成{reqCount}个章节候选
                </span>
              )}
            </div>
          </div>
        </div>
        <div className="text-right flex flex-col items-end gap-1">
          <div className="text-sm font-medium text-primary flex items-center gap-1 bg-surface-container-lowest/60 px-2 py-1 rounded">
            <span className="material-symbols-outlined text-[16px] animate-spin">sync</span> 已运行 {durationText}
          </div>
          <div className="text-xs text-on-primary-container mt-1 font-medium bg-secondary-container/50 px-2 py-0.5 rounded">
            当前阶段：{stageLabel}
          </div>
        </div>
      </div>
      <div className="border-t border-primary-fixed-dim/50 pt-3 mt-1 flex justify-between items-center text-sm relative z-10">
        <div className="text-on-primary-container/80 flex items-center gap-1.5">
          <span className="material-symbols-outlined text-[16px]">info</span>
          已恢复当前运行 · {statusLabel} · {runLabel}
        </div>
        <div className="flex items-center gap-4">
          <Link href={runDetailsHref} className="text-primary hover:underline font-medium hover:text-primary-dim transition-colors text-sm">
            查看详情
          </Link>
          <Link href={`/workflow-runs?projectId=${projectId}`} className="text-primary hover:underline font-medium hover:text-primary-dim transition-colors text-sm">
            查看全部
          </Link>
        </div>
      </div>
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

  const mainNames = names ? relationValues(main, names.storylines, "主线背景") : ["加载中"];

  return (
    <tr className={`hover:bg-surface-container-lowest/50 transition-colors ${plan.status === "pending_confirmation" ? "bg-surface-bright" : ""} group`}>
      <td className="px-4 py-4 text-center align-top">
        <input
          type="checkbox"
          aria-label={`选择第 ${plan.chapter_no} 章`}
          checked={selected}
          onChange={onToggle}
          disabled={plan.status !== "pending_confirmation"}
          className="form-checkbox rounded border-outline-variant text-primary focus:ring-primary h-4 w-4 mt-1 cursor-pointer disabled:cursor-not-allowed"
        />
      </td>
      <td className="px-4 py-4 align-top font-medium text-on-surface whitespace-nowrap">
        第{String(plan.chapter_no).padStart(2, "0")}章
      </td>
      <td className="px-4 py-4 align-top font-semibold text-on-surface">
        {plan.title}
      </td>
      <td className="px-4 py-4 align-top">
        <div className="flex items-center text-xs text-on-surface-variant truncate max-w-[200px]" title={mainNames.join(" › ")}>
          <span className="bg-surface-container px-1.5 py-0.5 rounded text-on-surface truncate">
            {mainNames[0] || "主线"}
          </span>
        </div>
      </td>
      <td className="px-4 py-4 align-top whitespace-normal break-words text-xs leading-relaxed text-on-surface-variant max-w-[400px]">
        {chapterPlanSummary(plan.summary)}
      </td>
      <td className="px-4 py-4 align-top text-center whitespace-nowrap">
        <span
          className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium ${
            plan.status === "pending_confirmation"
              ? "bg-[#FEF3C7] text-[#92400E] border border-[#FDE68A]"
              : plan.status === "confirmed"
                ? "bg-[#D1FAE5] text-[#065F46] border border-[#A7F3D0]"
                : plan.status === "draft_generated" || (plan.status as string) === "generated"
                  ? "bg-[#E0E7FF] text-[#4338CA] border border-[#C7D2FE]"
                  : "bg-surface-container-high text-on-surface-variant border border-outline-variant"
          }`}
        >
          {chapterPlanStatusLabel(plan.status)}
        </span>
      </td>
      <td className="px-4 py-4 align-top text-right text-on-surface-variant font-mono whitespace-nowrap">
        {plan.chapter_goal ? "2,500" : "3,000"}
      </td>
      <td className="px-4 py-4 align-top text-center whitespace-nowrap">
        {plan.status === "pending_confirmation" ? (
          <button
            type="button"
            className="text-primary hover:text-primary-dim p-1 rounded hover:bg-primary-container transition-colors cursor-pointer bg-transparent border-0"
            title="编辑"
            onClick={onEdit}
          >
            编辑
          </button>
        ) : (
          <Link
            className="text-primary hover:text-primary-dim p-1 rounded hover:bg-primary-container transition-colors"
            href={`/projects/${plan.project_id}/chapter-plans/${plan.id}/content`}
          >
            正文
          </Link>
        )}
      </td>
    </tr>
  );
}

function Loading() {
  return (
    <div className="chapter-plans-state loading">
      <h1>加载章节规划中...</h1>
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
    <div className="chapter-plans-state">
      <h1>{title}</h1>
      <p>{description}</p>
      <button type="button" onClick={retry}>
        重试
      </button>
    </div>
  );
}

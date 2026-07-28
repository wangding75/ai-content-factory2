import { apiRequest, type ApiRequestInit } from "../../lib/api.ts";
export type ChapterPlanStatus = "pending_confirmation" | "confirmed";
export interface ChapterPlanStorylineRef { storyline_id: string; relation: "primary" | "secondary"; }
export type ChapterPlanSource = "mock_generated" | "candidate_adopted" | "legacy_manual";
export type WritableChapterPlanSource = Exclude<ChapterPlanSource, "legacy_manual">;
export interface ChapterPlan {
  id: string;
  project_id: string;
  chapter_no: number;
  title: string;
  summary: string;
  status: ChapterPlanStatus;
  source: ChapterPlanSource;
  storyline_refs_json: ChapterPlanStorylineRef[];
  material_refs_json: string[];
  foreshadowing_refs_json: string[];
  chapter_goal: string | null;
  creation_notes: string | null;
  confirmed_at: string | null;
  currentRevisionId: string | null;
  sourceCandidateId: string | null;
  sourceCandidateBatchId: string | null;
  sourceWorkflowRunId: string | null;
  version: number;
  created_at: string;
  updated_at: string;
}
export interface ChapterPlanList{items:ChapterPlan[];total:number;limit:number;offset:number}
export interface ConfirmChapterPlanSelection{chapter_plan_id:string;expected_version:number}
export interface ConfirmChapterPlansRequest{selections:ConfirmChapterPlanSelection[]}
export interface UpdateChapterPlanRequest{expected_version:number;chapter_no?:number;title?:string;summary?:string;storyline_refs_json?:ChapterPlanStorylineRef[];material_refs_json?:string[];foreshadowing_refs_json?:string[];chapter_goal?:string|null;creation_notes?:string|null}
export type SummaryLength="short"|"medium"|"long";export type ChapterPace="slow"|"balanced"|"fast";export interface MockGenerateChapterPlansRequest{target_storyline_id:string;start_chapter_no:number;end_chapter_no:number;chapter_count:number;include_main_storyline:boolean;include_child_storylines:boolean;include_project_materials:boolean;include_unpaid_foreshadowings:boolean;include_prior_chapter_summaries:boolean;summary_length:SummaryLength;chapter_pace:ChapterPace;generation_notes:string|null}
export interface MockGenerationRun{id:string;project_id:string;provider_key:"mock";workflow_key:"chapter_plan_mock_generate";status:"succeeded";created_at:string;updated_at:string}
export interface MockGenerateChapterPlansResult{run:MockGenerationRun;items:ChapterPlan[]}
export interface ListChapterPlansQuery{status?:ChapterPlanStatus;limit?:number;offset?:number}
export function chapterPlanQuery(query:ListChapterPlansQuery={}){const params=new URLSearchParams();if(query.status)params.set("status",query.status);if(query.limit!==undefined)params.set("limit",String(query.limit));if(query.offset!==undefined)params.set("offset",String(query.offset));const value=params.toString();return value?`?${value}`:""}
export function listChapterPlans(projectId:string,query:ListChapterPlansQuery={},init?:ApiRequestInit){return apiRequest<ChapterPlanList>(`/projects/${encodeURIComponent(projectId)}/chapter-plans${chapterPlanQuery(query)}`,init)}

export function updateChapterPlan(chapterPlanId:string,payload:UpdateChapterPlanRequest,init?:ApiRequestInit){return apiRequest<ChapterPlan>(`/chapter-plans/${encodeURIComponent(chapterPlanId)}`,{...init,method:"PATCH",headers:{...init?.headers,"Content-Type":"application/json"},body:JSON.stringify(payload)})}
export function deleteChapterPlan(chapterPlanId:string,expectedVersion:number,init?:ApiRequestInit):Promise<void>{const query=new URLSearchParams({expected_version:String(expectedVersion)});return apiRequest<void>(`/chapter-plans/${encodeURIComponent(chapterPlanId)}?${query}`,{...init,method:"DELETE"})}
export function mockGenerateChapterPlans(projectId:string,payload:MockGenerateChapterPlansRequest,init?:ApiRequestInit){return apiRequest<MockGenerateChapterPlansResult>(`/projects/${encodeURIComponent(projectId)}/chapter-plans/mock-generate`,{...init,method:"POST",headers:{...init?.headers,"Content-Type":"application/json"},body:JSON.stringify(payload)})}
export function confirmChapterPlans(projectId:string,payload:ConfirmChapterPlansRequest,init?:ApiRequestInit){return apiRequest<ChapterPlanList>(`/projects/${encodeURIComponent(projectId)}/chapter-plans/confirm`,{...init,method:"POST",headers:{...init?.headers,"Content-Type":"application/json"},body:JSON.stringify(payload)})}

// --- Iteration 15 Chapter Planning Contracts ---

export type ChapterPlanningGenerationMode = "full" | "append" | "range";

export type ChapterPlanningStorylineSelection =
  | { mode: "auto_balanced" }
  | { mode: "specified"; storylineIds: string[] };

export interface ChapterPlanningContextOptions {
  includeProjectMaterials: boolean;
  includeUnpaidForeshadowings: boolean;
  includePriorChapterSummaries: boolean;
  coreSettingsOnly: boolean;
}

export interface ChapterPlanningFullTarget {
  targetTotalChapters: number;
}

export interface ChapterPlanningAppendTarget {
  chapterCount: number;
}

export interface ChapterPlanningRangeTarget {
  startChapterNo: number;
  endChapterNo: number;
}

export interface ChapterPlanningPreflightRequest {
  generationMode: ChapterPlanningGenerationMode;
  target: ChapterPlanningFullTarget | ChapterPlanningAppendTarget | ChapterPlanningRangeTarget;
  storylineSelection: ChapterPlanningStorylineSelection;
  contextOptions: ChapterPlanningContextOptions;
  additionalInstructions: string | null;
}

export type ChapterPlanningBlockerCode =
  | "project_binding_missing"
  | "execution_integration_unavailable"
  | "active_run_conflict"
  | "storyline_reference_invalid"
  | "generation_input_invalid";

export type ChapterPlanningItemSeverity = "info" | "warning" | "blocker";

export interface ChapterPlanningItemDetails {
  action?: string;
  field?: string;
  resourceId?: string;
  retryAction?: string;
  safeReason?: string;
  safeSummary?: string;
}

export interface ChapterPlanningPreflightItem {
  code: string;
  message: string;
  severity: ChapterPlanningItemSeverity;
  safeReason?: string;
  retryAction?: string;
  details?: ChapterPlanningItemDetails;
}

export interface ChapterPlanningBlockerItem {
  code: ChapterPlanningBlockerCode;
  message: string;
  severity: "blocker";
  safeReason: string;
  retryAction: string;
  details?: ChapterPlanningItemDetails;
}

export interface ChapterPlanningInputSummary {
  generationMode: ChapterPlanningGenerationMode;
  target: {
    startChapterNo: number;
    endChapterNo: number;
    requestedChapterCount: number;
  };
  storylineSelection: ChapterPlanningStorylineSelection;
  contextOptions: ChapterPlanningContextOptions;
}

export interface ChapterPlanningExecutionConfigurationSummary {
  stage: "chapter_planning";
  workflowBindingId: string;
  workflowBindingVersion: number;
}

export interface ChapterPlanningPreflightPassed {
  result: "passed";
  status: "passed";
  preflightToken: string;
  expiresAt: string;
  inputDigest: string;
  inputSummary: ChapterPlanningInputSummary;
  executionConfigurationSummary: ChapterPlanningExecutionConfigurationSummary;
  checks: ChapterPlanningPreflightItem[];
  blockers: ChapterPlanningBlockerItem[];
  warnings: ChapterPlanningPreflightItem[];
}

export interface ChapterPlanningPreflightBlocked {
  result: "blocked";
  status: "blocked";
  inputDigest: string;
  inputSummary: ChapterPlanningInputSummary | null;
  executionConfigurationSummary: ChapterPlanningExecutionConfigurationSummary | null;
  checks: ChapterPlanningPreflightItem[];
  blockers: ChapterPlanningBlockerItem[];
  warnings: ChapterPlanningPreflightItem[];
}

export type ChapterPlanningPreflightReport =
  | ChapterPlanningPreflightPassed
  | ChapterPlanningPreflightBlocked;

export interface CreateChapterPlanRunRequest {
  preflightToken: string;
}

export interface ChapterPlanningWorkflowRun {
  id: string;
  status: string;
}

export interface ChapterPlanningSummary {
  currentChapterCount: number;
  pendingConfirmationCount: number;
  confirmedChapterCount: number;
  candidateBatchCounts: {
    ready: number;
    partiallyAdopted: number;
    adopted: number;
    abandoned: number;
  };
  activeRun: {
    id: string;
    runNumber?: string;
    status: string;
    stage?: string;
    createdAt?: string;
    finishedAt?: string;
    inputPayload?: Record<string, unknown>;
    outputPayload?: Record<string, unknown> | null;
  } | null;
}

export type ChapterPlanningErrorCode =
  | "workflow_not_configured"
  | "preflight_token_invalid"
  | "preflight_token_expired"
  | "preflight_input_changed"
  | "active_run_conflict"
  | "invalid_candidate_state"
  | "stale_candidate"
  | "version_conflict"
  | "batch_already_finalized"
  | "run_already_consumed"
  | "chapter_no_conflict"
  | "revision_sequence_conflict"
  | "output_validation_failed"
  | "result_consumption_failed"
  | "idempotency_key_reused_with_different_payload";

export function getProjectChapterPlanningSummary(
  projectId: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanningSummary> {
  return apiRequest<ChapterPlanningSummary>(
    `/projects/${encodeURIComponent(projectId)}/chapter-planning-summary`,
    init,
  );
}

export function preflightChapterPlanRun(
  projectId: string,
  payload: ChapterPlanningPreflightRequest,
  init?: ApiRequestInit,
): Promise<ChapterPlanningPreflightReport> {
  return apiRequest<ChapterPlanningPreflightReport>(
    `/projects/${encodeURIComponent(projectId)}/chapter-plan-runs/preflight`,
    {
      ...init,
      method: "POST",
      headers: { ...init?.headers, "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
  );
}

export function createChapterPlanRun(
  projectId: string,
  payload: CreateChapterPlanRunRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanningWorkflowRun> {
  return apiRequest<ChapterPlanningWorkflowRun>(
    `/projects/${encodeURIComponent(projectId)}/chapter-plan-runs`,
    {
      ...init,
      method: "POST",
      headers: {
        ...init?.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(payload),
    },
  );
}

// --- FE-F2 Candidate Batch & Candidate Contracts ---

export type ChapterPlanCandidateBatchStatus =
  | "ready"
  | "partially_adopted"
  | "adopted"
  | "abandoned";

export type ChapterPlanCandidateStatus =
  | "pending"
  | "stale"
  | "adopted"
  | "discarded";

export type ChapterPlanCandidateDiffType =
  | "new"
  | "replace"
  | "no_change"
  | "stale_conflict";

export interface ChapterPlanReferenceSnapshot {
  id: string;
  label: string;
  relation: string;
  position: number;
  version: number;
}

export interface ChapterPlanCandidateSnapshot {
  chapterNo: number;
  title: string;
  summary: string;
  chapterPurpose: string;
  storylineRefs: ChapterPlanReferenceSnapshot[];
  materialRefs: ChapterPlanReferenceSnapshot[];
  foreshadowingRefs: ChapterPlanReferenceSnapshot[];
  generationBasis: {
    contextSummary: string;
    additionalInstructions: string | null;
  };
}

export interface ChapterPlanCandidateBatch {
  id: string;
  projectId: string;
  sourceWorkflowRunId: string;
  generationMode: ChapterPlanningGenerationMode;
  target: {
    startChapterNo: number;
    endChapterNo: number;
    requestedChapterCount: number;
  };
  inputDigest: string;
  inputSummary: ChapterPlanningInputSummary;
  candidateCount: number;
  pendingCount: number;
  staleCount: number;
  adoptedCount: number;
  discardedCount: number;
  status: ChapterPlanCandidateBatchStatus;
  version: number;
  completedAt: string | null;
  abandonedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface ChapterPlanCandidate {
  id: string;
  batchId: string;
  projectId: string;
  chapterNo: number;
  sortOrder: number;
  baseChapterPlanId: string | null;
  baseRevisionId: string | null;
  baseChapterPlanVersion: number | null;
  baseSnapshot: ChapterPlanCandidateSnapshot | null;
  generatedSnapshot: ChapterPlanCandidateSnapshot;
  currentSnapshot: ChapterPlanCandidateSnapshot;
  diffType: ChapterPlanCandidateDiffType;
  status: ChapterPlanCandidateStatus;
  version: number;
  createdAt: string;
  updatedAt: string;
}

export interface ChapterPlanCandidateBatchList {
  items: ChapterPlanCandidateBatch[];
  total: number;
  limit: number;
  offset: number;
}

export interface ChapterPlanCandidateList {
  items: ChapterPlanCandidate[];
  total: number;
  limit: number;
  offset: number;
}

export interface ListCandidateBatchesQuery {
  status?: ChapterPlanCandidateBatchStatus;
  generationMode?: ChapterPlanningGenerationMode;
  sourceWorkflowRunId?: string;
  createdAtFrom?: string;
  createdAtTo?: string;
  limit?: number;
  offset?: number;
}

export interface ListCandidatesQuery {
  status?: ChapterPlanCandidateStatus;
  diffType?: ChapterPlanCandidateDiffType;
  storylineId?: string;
  q?: string;
  limit?: number;
  offset?: number;
}

export function candidateBatchQuery(query: ListCandidateBatchesQuery = {}) {
  const params = new URLSearchParams();
  if (query.status) params.set("status", query.status);
  if (query.generationMode) params.set("generationMode", query.generationMode);
  if (query.sourceWorkflowRunId) params.set("sourceWorkflowRunId", query.sourceWorkflowRunId);
  if (query.createdAtFrom) params.set("createdAtFrom", query.createdAtFrom);
  if (query.createdAtTo) params.set("createdAtTo", query.createdAtTo);
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  if (query.offset !== undefined) params.set("offset", String(query.offset));
  const val = params.toString();
  return val ? `?${val}` : "";
}

export function candidateQuery(query: ListCandidatesQuery = {}) {
  const params = new URLSearchParams();
  if (query.status) params.set("status", query.status);
  if (query.diffType) params.set("diffType", query.diffType);
  if (query.storylineId) params.set("storylineId", query.storylineId);
  if (query.q?.trim()) params.set("q", query.q.trim());
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  if (query.offset !== undefined) params.set("offset", String(query.offset));
  const val = params.toString();
  return val ? `?${val}` : "";
}

export function listChapterPlanCandidateBatches(
  projectId: string,
  query: ListCandidateBatchesQuery = {},
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateBatchList> {
  return apiRequest<ChapterPlanCandidateBatchList>(
    `/projects/${encodeURIComponent(projectId)}/chapter-plan-candidate-batches${candidateBatchQuery(query)}`,
    init,
  );
}

export function getChapterPlanCandidateBatch(
  batchId: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateBatch> {
  return apiRequest<ChapterPlanCandidateBatch>(
    `/chapter-plan-candidate-batches/${encodeURIComponent(batchId)}`,
    init,
  );
}

export function listChapterPlanCandidates(
  batchId: string,
  query: ListCandidatesQuery = {},
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateList> {
  return apiRequest<ChapterPlanCandidateList>(
    `/chapter-plan-candidate-batches/${encodeURIComponent(batchId)}/candidates${candidateQuery(query)}`,
    init,
  );
}

export function getChapterPlanCandidate(
  candidateId: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidate> {
  return apiRequest<ChapterPlanCandidate>(
    `/chapter-plan-candidates/${encodeURIComponent(candidateId)}`,
    init,
  );
}

// --- FE-F3 Candidate Edit, Compare, Recompare & Revision Contracts ---

export interface UpdateChapterPlanCandidateRequest {
  expectedCandidateVersion: number;
  currentSnapshot: ChapterPlanCandidateSnapshot;
}

export interface RecompareChapterPlanCandidateRequest {
  expectedCandidateVersion: number;
}

export interface ChapterPlanDiffEntry {
  path: string;
  changeType: "added" | "removed" | "changed" | "unchanged";
  before: string | null;
  after: string | null;
}

export interface ChapterPlanCandidateDiff {
  baseRevisionId: string | null;
  candidateVersion: number;
  entries: ChapterPlanDiffEntry[];
  stale: boolean;
}

export interface ChapterPlanCandidateComparison {
  candidate: ChapterPlanCandidate;
  currentChapter: ChapterPlan | null;
  diff: ChapterPlanCandidateDiff;
}

export interface ChapterPlanRevision {
  id: string;
  chapterPlanId: string;
  projectId: string;
  revisionNo: number;
  snapshot: ChapterPlanCandidateSnapshot;
  changeType: "manual_create" | "manual_edit" | "candidate_adopt" | "confirm" | "legacy_backfill";
  sourceCandidateId: string | null;
  sourceCandidateBatchId: string | null;
  sourceWorkflowRunId: string | null;
  createdAt: string;
}

export interface ChapterPlanRevisionList {
  items: ChapterPlanRevision[];
  total: number;
  limit: number;
  offset: number;
}

export function updateChapterPlanCandidate(
  candidateId: string,
  payload: UpdateChapterPlanCandidateRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidate> {
  return apiRequest<ChapterPlanCandidate>(
    `/chapter-plan-candidates/${encodeURIComponent(candidateId)}`,
    {
      ...init,
      method: "PATCH",
      headers: {
        ...init?.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(payload),
    },
  );
}

export function compareChapterPlanCandidate(
  candidateId: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateComparison> {
  return apiRequest<ChapterPlanCandidateComparison>(
    `/chapter-plan-candidates/${encodeURIComponent(candidateId)}/compare`,
    init,
  );
}

export function recompareChapterPlanCandidate(
  candidateId: string,
  payload: RecompareChapterPlanCandidateRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateComparison> {
  return apiRequest<ChapterPlanCandidateComparison>(
    `/chapter-plan-candidates/${encodeURIComponent(candidateId)}/recompare`,
    {
      ...init,
      method: "POST",
      headers: {
        ...init?.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(payload),
    },
  );
}

export function listChapterPlanRevisions(
  chapterPlanId: string,
  query: { limit?: number; offset?: number } = {},
  init?: ApiRequestInit,
): Promise<ChapterPlanRevisionList> {
  const params = new URLSearchParams();
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  if (query.offset !== undefined) params.set("offset", String(query.offset));
  const val = params.toString();
  const qStr = val ? `?${val}` : "";

  return apiRequest<ChapterPlanRevisionList>(
    `/chapter-plans/${encodeURIComponent(chapterPlanId)}/revisions${qStr}`,
    init,
  );
}

// --- FE-F4 Candidate Adoption & Lifecycle Contracts ---

export interface AdoptChapterPlanCandidateRequest {
  expectedCandidateVersion: number;
  expectedChapterPlanVersion: number | null;
}

export interface AdoptChapterPlanCandidateAdoptedResult {
  outcome: "adopted";
  candidate: ChapterPlanCandidate;
  chapterPlan: ChapterPlan;
  revision: ChapterPlanRevision;
  batch: ChapterPlanCandidateBatch;
}

export interface AdoptChapterPlanCandidateNoChangeResult {
  outcome: "no_change";
  candidate: ChapterPlanCandidate;
  chapterPlan: ChapterPlan;
  revision: null;
  batch: ChapterPlanCandidateBatch;
}

export type AdoptChapterPlanCandidateResult =
  | AdoptChapterPlanCandidateAdoptedResult
  | AdoptChapterPlanCandidateNoChangeResult;

export interface DiscardChapterPlanCandidateRequest {
  expectedCandidateVersion: number;
  reason?: string | null;
}

export interface BulkAdoptChapterPlanCandidateItem {
  candidateId: string;
  expectedCandidateVersion: number;
  expectedChapterPlanVersion: number | null;
}

export interface BulkAdoptChapterPlanCandidatesRequest {
  expectedBatchVersion: number;
  candidates: BulkAdoptChapterPlanCandidateItem[];
}

export interface BulkAdoptChapterPlanCandidateAdoptedResult {
  candidateId: string;
  outcome: "adopted";
  candidate: ChapterPlanCandidate;
  chapterPlan: ChapterPlan;
  revision: ChapterPlanRevision;
}

export interface BulkAdoptChapterPlanCandidateNoChangeResult {
  candidateId: string;
  outcome: "no_change";
  candidate: ChapterPlanCandidate;
  chapterPlan: ChapterPlan;
  revision: null;
}

export interface BulkAdoptChapterPlanCandidateStaleResult {
  candidateId: string;
  outcome: "stale";
  candidate: ChapterPlanCandidate;
  error: unknown;
}

export interface BulkAdoptChapterPlanCandidateConflictResult {
  candidateId: string;
  outcome: "conflict";
  candidate: ChapterPlanCandidate;
  error: unknown;
}

export interface BulkAdoptChapterPlanCandidateFailedResult {
  candidateId: string;
  outcome: "failed";
  error: unknown;
}

export type BulkAdoptChapterPlanCandidateResult =
  | BulkAdoptChapterPlanCandidateAdoptedResult
  | BulkAdoptChapterPlanCandidateNoChangeResult
  | BulkAdoptChapterPlanCandidateStaleResult
  | BulkAdoptChapterPlanCandidateConflictResult
  | BulkAdoptChapterPlanCandidateFailedResult;

export interface BulkAdoptChapterPlanCandidatesResult {
  items: BulkAdoptChapterPlanCandidateResult[];
  batch: ChapterPlanCandidateBatch;
}

export interface AbandonChapterPlanCandidateBatchRequest {
  expectedBatchVersion: number;
  reason?: string | null;
  acknowledgeAdoptedChaptersRemain: true;
}

export function adoptChapterPlanCandidate(
  candidateId: string,
  payload: AdoptChapterPlanCandidateRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<AdoptChapterPlanCandidateResult> {
  return apiRequest<AdoptChapterPlanCandidateResult>(
    `/chapter-plan-candidates/${encodeURIComponent(candidateId)}/adopt`,
    {
      ...init,
      method: "POST",
      headers: {
        ...init?.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(payload),
    },
  );
}

export function discardChapterPlanCandidate(
  candidateId: string,
  payload: DiscardChapterPlanCandidateRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidate> {
  return apiRequest<ChapterPlanCandidate>(
    `/chapter-plan-candidates/${encodeURIComponent(candidateId)}/discard`,
    {
      ...init,
      method: "POST",
      headers: {
        ...init?.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(payload),
    },
  );
}

export function adoptChapterPlanCandidates(
  batchId: string,
  payload: BulkAdoptChapterPlanCandidatesRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<BulkAdoptChapterPlanCandidatesResult> {
  return apiRequest<BulkAdoptChapterPlanCandidatesResult>(
    `/chapter-plan-candidate-batches/${encodeURIComponent(batchId)}/adoptions`,
    {
      ...init,
      method: "POST",
      headers: {
        ...init?.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(payload),
    },
  );
}

export function abandonChapterPlanCandidateBatch(
  batchId: string,
  payload: AbandonChapterPlanCandidateBatchRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateBatch> {
  return apiRequest<ChapterPlanCandidateBatch>(
    `/chapter-plan-candidate-batches/${encodeURIComponent(batchId)}/abandon`,
    {
      ...init,
      method: "POST",
      headers: {
        ...init?.headers,
        "Content-Type": "application/json",
        "Idempotency-Key": idempotencyKey,
      },
      body: JSON.stringify(payload),
    },
  );
}

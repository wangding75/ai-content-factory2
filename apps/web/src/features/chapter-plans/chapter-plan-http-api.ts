import { apiRequest, type ApiRequestInit } from "@/lib/api";
export type ChapterPlanStatus="pending_confirmation"|"confirmed";
export interface ChapterPlanStorylineRef{storyline_id:string;relation:"primary"|"secondary"}
export interface ChapterPlan{id:string;project_id:string;chapter_no:number;title:string;summary:string;status:ChapterPlanStatus;source:"mock_generated";storyline_refs_json:ChapterPlanStorylineRef[];material_refs_json:string[];foreshadowing_refs_json:string[];chapter_goal:string|null;creation_notes:string|null;confirmed_at:string|null;version:number;created_at:string;updated_at:string}
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
  requestedChapterCount: number;
}

export interface ChapterPlanningAppendTarget {
  requestedChapterCount: number;
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
  safeSummary?: string;
}

export interface ChapterPlanningPreflightItem {
  code: string;
  message: string;
  severity: ChapterPlanningItemSeverity;
  details?: ChapterPlanningItemDetails;
}

export interface ChapterPlanningBlockerItem {
  code: ChapterPlanningBlockerCode;
  message: string;
  severity: "blocker";
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
  inputSummary: ChapterPlanningInputSummary;
  executionConfigurationSummary: ChapterPlanningExecutionConfigurationSummary;
  checks: ChapterPlanningPreflightItem[];
  blockers: ChapterPlanningBlockerItem[];
  warnings: ChapterPlanningPreflightItem[];
}

export type ChapterPlanningPreflightReport =
  | ChapterPlanningPreflightPassed
  | ChapterPlanningPreflightBlocked;

export interface ChapterPlanningPreflightEnvelope {
  data: ChapterPlanningPreflightReport;
  request_id: string;
}

export interface CreateChapterPlanRunRequest {
  preflightToken: string;
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

export interface ChapterPlanningSummaryEnvelope {
  data: ChapterPlanningSummary;
  request_id: string;
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
): Promise<ChapterPlanningSummaryEnvelope> {
  return apiRequest<ChapterPlanningSummaryEnvelope>(
    `/projects/${encodeURIComponent(projectId)}/chapter-planning-summary`,
    init,
  );
}

export function preflightChapterPlanRun(
  projectId: string,
  payload: ChapterPlanningPreflightRequest,
  init?: ApiRequestInit,
): Promise<ChapterPlanningPreflightEnvelope> {
  return apiRequest<ChapterPlanningPreflightEnvelope>(
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
): Promise<unknown> {
  return apiRequest(
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

export interface ChapterPlanCandidateBatchListEnvelope {
  data: ChapterPlanCandidateBatchList;
  request_id: string;
}

export interface ChapterPlanCandidateBatchEnvelope {
  data: ChapterPlanCandidateBatch;
  request_id: string;
}

export interface ChapterPlanCandidateListEnvelope {
  data: ChapterPlanCandidateList;
  request_id: string;
}

export interface ChapterPlanCandidateEnvelope {
  data: ChapterPlanCandidate;
  request_id: string;
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
): Promise<ChapterPlanCandidateBatchListEnvelope> {
  return apiRequest<ChapterPlanCandidateBatchListEnvelope>(
    `/projects/${encodeURIComponent(projectId)}/chapter-plan-candidate-batches${candidateBatchQuery(query)}`,
    init,
  );
}

export function getChapterPlanCandidateBatch(
  batchId: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateBatchEnvelope> {
  return apiRequest<ChapterPlanCandidateBatchEnvelope>(
    `/chapter-plan-candidate-batches/${encodeURIComponent(batchId)}`,
    init,
  );
}

export function listChapterPlanCandidates(
  batchId: string,
  query: ListCandidatesQuery = {},
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateListEnvelope> {
  return apiRequest<ChapterPlanCandidateListEnvelope>(
    `/chapter-plan-candidate-batches/${encodeURIComponent(batchId)}/candidates${candidateQuery(query)}`,
    init,
  );
}

export function getChapterPlanCandidate(
  candidateId: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateEnvelope> {
  return apiRequest<ChapterPlanCandidateEnvelope>(
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

export interface ChapterPlanCandidateComparisonEnvelope {
  data: ChapterPlanCandidateComparison;
  request_id: string;
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

export interface ChapterPlanRevisionListEnvelope {
  data: ChapterPlanRevisionList;
  request_id: string;
}

export function updateChapterPlanCandidate(
  candidateId: string,
  payload: UpdateChapterPlanCandidateRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateEnvelope> {
  return apiRequest<ChapterPlanCandidateEnvelope>(
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
): Promise<ChapterPlanCandidateComparisonEnvelope> {
  return apiRequest<ChapterPlanCandidateComparisonEnvelope>(
    `/chapter-plan-candidates/${encodeURIComponent(candidateId)}/compare`,
    init,
  );
}

export function recompareChapterPlanCandidate(
  candidateId: string,
  payload: RecompareChapterPlanCandidateRequest,
  idempotencyKey: string,
  init?: ApiRequestInit,
): Promise<ChapterPlanCandidateComparisonEnvelope> {
  return apiRequest<ChapterPlanCandidateComparisonEnvelope>(
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
): Promise<ChapterPlanRevisionListEnvelope> {
  const params = new URLSearchParams();
  if (query.limit !== undefined) params.set("limit", String(query.limit));
  if (query.offset !== undefined) params.set("offset", String(query.offset));
  const val = params.toString();
  const qStr = val ? `?${val}` : "";

  return apiRequest<ChapterPlanRevisionListEnvelope>(
    `/chapter-plans/${encodeURIComponent(chapterPlanId)}/revisions${qStr}`,
    init,
  );
}

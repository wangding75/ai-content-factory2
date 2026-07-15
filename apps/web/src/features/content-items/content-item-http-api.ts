import {apiRequest, type ApiRequestInit} from "@/lib/api";
export type ContentItemStatus="draft"|"in_review"|"reviewed";export type ContentVersionStatus="editable_draft"|"frozen";export type ContentVersionSource="manual_created"|"mock_generated";
export interface ContentItem{id:string;chapter_plan_id:string;title:string;status:ContentItemStatus;current_version_id:string;reviewed_at:string|null;created_at:string;updated_at:string}
export interface ContentVersion{id:string;content_item_id:string;version_no:1;version:number;status:ContentVersionStatus;source:ContentVersionSource;title:string;content:string;summary:string|null;word_count:number;frozen_at:string|null;created_at:string;updated_at:string}
export interface ContentItemDetail{content_item:ContentItem;current_version:ContentVersion}
export interface SaveContentDraftRequest{expected_version:number;title?:string;content?:string;summary?:string|null}
export interface MockGenerationParameters{chapter_goal:string|null;storyline_refs_json:string[];material_refs_json:string[];foreshadowing_refs_json:string[];creation_notes:string|null}
export interface WorkflowRunSummary{id:string;provider_key:"mock";workflow_key:"content_mock_generate"|"content_mock_review";status:"running"|"succeeded"|"failed";started_at:string;finished_at:string|null}
export interface MockGenerateContentResult extends ContentItemDetail{workflow_run:WorkflowRunSummary}
export interface MockReviewRequest{content_version_id:string;expected_version:number}
export interface ReviewReport{id:string;content_item_id:string;content_version_id:string;provider_key:"mock";status:"completed";conclusion:"pass"|"revise";score:number;summary:string;created_at:string}
export interface ReviewFinding{id:string;review_id:string;category:"pacing"|"foreshadowing"|"character_consistency"|"world_consistency";severity:"low"|"medium"|"high";title:string;description:string;location:{start_offset?:number;end_offset?:number}|null}
export interface ReviewRecommendation{id:string;review_id:string;priority:"low"|"medium"|"high";title:string;description:string;created_at:string}
export interface ContentVersionSummary{id:string;version_no:number;version:number;title:string;word_count:number;source:ContentVersionSource;frozen_at:string}
export interface MockReviewContentResult{content_item:ContentItem;review:ReviewReport;findings:ReviewFinding[];recommendations:ReviewRecommendation[];workflow_run:WorkflowRunSummary}
export interface ContentReviewList{items:ReviewReport[];total:number;limit:number;offset:number}
export interface ReviewDetail{review:ReviewReport;content_version:ContentVersionSummary;findings:ReviewFinding[];recommendations:ReviewRecommendation[];workflow_run:WorkflowRunSummary}
/** POST accepts both 201 (new singleton) and 200 (existing singleton) envelopes. */
export const createOrGetContentItem=(chapterPlanId:string,init?:ApiRequestInit)=>apiRequest<ContentItemDetail>(`/chapter-plans/${encodeURIComponent(chapterPlanId)}/content`,{...init,method:"POST"});
export const getContentItem=(contentItemId:string,init?:ApiRequestInit)=>apiRequest<ContentItemDetail>(`/content-items/${encodeURIComponent(contentItemId)}`,init);
export const saveContentDraft=(contentItemId:string,payload:SaveContentDraftRequest,init?:ApiRequestInit)=>apiRequest<ContentItemDetail>(`/content-items/${encodeURIComponent(contentItemId)}/draft`,{...init,method:"PUT",headers:{...init?.headers,"Content-Type":"application/json"},body:JSON.stringify(payload)});
export const mockGenerateContent=(contentItemId:string,payload:{expected_version:number;parameters:MockGenerationParameters},idempotencyKey:string,init?:ApiRequestInit)=>apiRequest<MockGenerateContentResult>(`/content-items/${encodeURIComponent(contentItemId)}/mock-generate`,{...init,method:"POST",headers:{...init?.headers,"Content-Type":"application/json","Idempotency-Key":idempotencyKey},body:JSON.stringify(payload)});
export const mockReviewContent=(contentItemId:string,payload:MockReviewRequest,idempotencyKey:string,init?:ApiRequestInit)=>apiRequest<MockReviewContentResult>(`/content-items/${encodeURIComponent(contentItemId)}/reviews/mock`,{...init,method:"POST",headers:{...init?.headers,"Content-Type":"application/json","Idempotency-Key":idempotencyKey},body:JSON.stringify(payload)});
export const listContentReviews=(contentItemId:string,options:{limit:number;offset:number},init?:ApiRequestInit)=>{const query=new URLSearchParams({limit:String(options.limit),offset:String(options.offset)});return apiRequest<ContentReviewList>(`/content-items/${encodeURIComponent(contentItemId)}/reviews?${query}`,init)};
export const getReviewDetail=(reviewId:string,init?:ApiRequestInit)=>apiRequest<ReviewDetail>(`/reviews/${encodeURIComponent(reviewId)}`,init);

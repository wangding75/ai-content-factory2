import { apiRequest, type ApiRequestInit } from "../../lib/api.ts";

export type LlmProviderType = "openai_compatible";
export type IntegrationStatus = "not_connected";
export type LlmProviderDto = { id: string; name: string; providerType: LlmProviderType; baseUrl: string; defaultModel: string; timeoutSeconds: number; hasSecret: boolean; secretFingerprint: string | null; integrationStatus: IntegrationStatus; enabled: boolean; lastVerifiedAt: string | null; lastErrorCode: string | null; lastErrorMessage: string | null; version: number; createdAt: string; updatedAt: string };
export type LlmProviderTypeDto = { providerType: LlmProviderType; displayName: string; supportsSecret: boolean; fieldSchemas: unknown[] };
export type LlmProviderListDto = { items: LlmProviderDto[]; total: number; limit: number; offset: number };
export type LlmProviderVm = { id: string; name: string; providerTypeLabel: string; baseUrl: string; defaultModel: string; timeoutSeconds: number; hasSecret: boolean; secretFingerprint: string | null; integrationStatusLabel: string; version: number };
export type ProviderFormInput = { name: string; providerType: LlmProviderType; baseUrl: string; defaultModel: string; timeoutSeconds: number; secret: string };

const providerTypeLabel: Record<LlmProviderType, string> = { openai_compatible: "OpenAI-compatible" };

export const mapLlmProvider = (item: LlmProviderDto): LlmProviderVm => ({ id: item.id, name: item.name, providerTypeLabel: providerTypeLabel[item.providerType], baseUrl: item.baseUrl, defaultModel: item.defaultModel, timeoutSeconds: item.timeoutSeconds, hasSecret: item.hasSecret, secretFingerprint: item.secretFingerprint, integrationStatusLabel: "未接入", version: item.version });
export const createLlmProviderPayload = (input: ProviderFormInput) => ({ name: input.name.trim(), providerType: input.providerType, baseUrl: input.baseUrl.trim(), defaultModel: input.defaultModel.trim(), timeoutSeconds: input.timeoutSeconds, ...(input.secret.trim() ? { secret: input.secret } : {}) });
export const updateLlmProviderPayload = (input: Omit<ProviderFormInput, "providerType"> & { expectedVersion: number }) => ({ expectedVersion: input.expectedVersion, name: input.name.trim(), baseUrl: input.baseUrl.trim(), defaultModel: input.defaultModel.trim(), timeoutSeconds: input.timeoutSeconds, ...(input.secret.trim() ? { secret: input.secret } : {}) });

const writeHeaders = (idempotencyKey: string) => ({ "Content-Type": "application/json", "Idempotency-Key": idempotencyKey });
export function listLlmProviderTypes(init?: ApiRequestInit) { return apiRequest<{ items: LlmProviderTypeDto[] }>("/llm-provider-types", init); }
export function listLlmProviders(query: { q?: string; limit?: number; offset?: number } = {}, init?: ApiRequestInit) { const params = new URLSearchParams({ limit: String(query.limit ?? 100), offset: String(query.offset ?? 0) }); if (query.q?.trim()) params.set("q", query.q.trim()); return apiRequest<LlmProviderListDto>(`/llm-providers?${params}`, init); }
export function getLlmProvider(providerId: string, init?: ApiRequestInit) { return apiRequest<LlmProviderDto>(`/llm-providers/${encodeURIComponent(providerId)}`, init); }
export function createLlmProvider(input: ProviderFormInput, idempotencyKey: string, init?: ApiRequestInit) { return apiRequest<LlmProviderDto>("/llm-providers", { ...init, method: "POST", headers: writeHeaders(idempotencyKey), body: JSON.stringify(createLlmProviderPayload(input)) }); }
export function updateLlmProvider(providerId: string, input: Omit<ProviderFormInput, "providerType"> & { expectedVersion: number }, idempotencyKey: string, init?: ApiRequestInit) { return apiRequest<LlmProviderDto>(`/llm-providers/${encodeURIComponent(providerId)}`, { ...init, method: "PATCH", headers: writeHeaders(idempotencyKey), body: JSON.stringify(updateLlmProviderPayload(input)) }); }
export function validateProviderForm(input: ProviderFormInput, requiresSecret: boolean, editing: boolean) { if (!input.name.trim()) return "请输入配置名称。"; if (input.name.trim().length > 120) return "配置名称不能超过 120 个字符。"; try { new URL(input.baseUrl.trim()); } catch { return "请输入有效的 Base URL。"; } if (!input.defaultModel.trim()) return "请输入默认模型。"; if (input.defaultModel.trim().length > 160) return "默认模型不能超过 160 个字符。"; if (!Number.isInteger(input.timeoutSeconds) || input.timeoutSeconds < 5 || input.timeoutSeconds > 300) return "请求超时需为 5 至 300 秒之间的整数。"; if (requiresSecret && !editing && !input.secret.trim()) return "请输入 API Key。"; return undefined; }

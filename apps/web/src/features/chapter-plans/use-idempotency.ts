import { useCallback } from "react";

export interface PersistedOperation {
  version: 2;
  scope: string;
  payloadDigest: string;
  idempotencyKey: string;
  status: "pending" | "unknown";
  createdAt: string;
  updatedAt: string;
}

export class IdempotencyStorageError extends Error {
  constructor() {
    super("无法安全保存请求重试标识，请恢复浏览器会话存储后重试。");
    this.name = "IdempotencyStorageError";
  }
}

const STORAGE_KEY = "acf:chapter-planning:idempotency:v2";
const memoryStore: Record<string, PersistedOperation> = {};

function storageOrThrow(): Storage | null {
  if (typeof window === "undefined") return null;
  try {
    return window.sessionStorage;
  } catch {
    throw new IdempotencyStorageError();
  }
}

export function canonicalizePayload(value: unknown): unknown {
  if (value === null || value === undefined) return null;
  if (typeof value === "boolean" || typeof value === "number" || typeof value === "string") return value;
  if (value instanceof Date) return value.toISOString();
  if (typeof value === "function" || typeof value === "symbol") return undefined;
  if (Array.isArray(value)) return value.map(canonicalizePayload);
  if (typeof value === "object") {
    if ("$$typeof" in value || ("nodeType" in value && typeof (value as Record<string, unknown>).nodeType === "number")) return undefined;
    return Object.keys(value as Record<string, unknown>).sort().reduce<Record<string, unknown>>((result, key) => {
      const input = (value as Record<string, unknown>)[key];
      if (input === undefined || typeof input === "function" || typeof input === "symbol") return result;
      const canonical = canonicalizePayload(input);
      if (canonical !== undefined) result[key] = canonical;
      return result;
    }, {});
  }
  return String(value);
}

function fallbackDigest(input: string): string {
  let hash = 2166136261;
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return `fnv1a:${(hash >>> 0).toString(16).padStart(8, "0")}`;
}

export async function hashCanonicalPayload(payload: unknown): Promise<string> {
  const canonicalJson = JSON.stringify(canonicalizePayload(payload));
  const subtle = globalThis.crypto?.subtle;
  if (!subtle) return fallbackDigest(canonicalJson);
  const bytes = await subtle.digest("SHA-256", new TextEncoder().encode(canonicalJson));
  return `sha256:${Array.from(new Uint8Array(bytes), (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
}

function isPersistedOperation(value: unknown): value is PersistedOperation {
  if (!value || typeof value !== "object") return false;
  const item = value as Partial<PersistedOperation>;
  return item.version === 2 && typeof item.scope === "string" && typeof item.payloadDigest === "string" && typeof item.idempotencyKey === "string" && (item.status === "pending" || item.status === "unknown") && typeof item.createdAt === "string" && typeof item.updatedAt === "string";
}

export function loadPersistedOperations(): Record<string, PersistedOperation> {
  const storage = storageOrThrow();
  if (!storage) return { ...memoryStore };
  try {
    const raw = storage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object") return {};
    return Object.fromEntries(Object.entries(parsed).filter(([, value]) => isPersistedOperation(value))) as Record<string, PersistedOperation>;
  } catch {
    throw new IdempotencyStorageError();
  }
}

function savePersistedOperations(operations: Record<string, PersistedOperation>): void {
  const storage = storageOrThrow();
  if (!storage) {
    Object.keys(memoryStore).forEach((key) => delete memoryStore[key]);
    Object.assign(memoryStore, operations);
    return;
  }
  try {
    storage.setItem(STORAGE_KEY, JSON.stringify(operations));
  } catch {
    throw new IdempotencyStorageError();
  }
}

function generateUUID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") return crypto.randomUUID();
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (character) => {
    const random = (Math.random() * 16) | 0;
    return (character === "x" ? random : (random & 0x3) | 0x8).toString(16);
  });
}

export async function getOrCreateOperation(scope: string, payload: unknown): Promise<string> {
  const operations = loadPersistedOperations();
  const payloadDigest = await hashCanonicalPayload(payload);
  const existing = operations[scope];
  if (existing?.payloadDigest === payloadDigest) return existing.idempotencyKey;
  const now = new Date().toISOString();
  const idempotencyKey = generateUUID();
  operations[scope] = { version: 2, scope, payloadDigest, idempotencyKey, status: "pending", createdAt: now, updatedAt: now };
  savePersistedOperations(operations);
  return idempotencyKey;
}

export async function markUnknown(scope: string, payload: unknown): Promise<void> {
  const operations = loadPersistedOperations();
  const payloadDigest = await hashCanonicalPayload(payload);
  const now = new Date().toISOString();
  const existing = operations[scope];
  operations[scope] = existing?.payloadDigest === payloadDigest
    ? { ...existing, status: "unknown", updatedAt: now }
    : { version: 2, scope, payloadDigest, idempotencyKey: generateUUID(), status: "unknown", createdAt: now, updatedAt: now };
  savePersistedOperations(operations);
}

export function clearOperation(scope: string): void {
  const operations = loadPersistedOperations();
  if (operations[scope]) {
    delete operations[scope];
    savePersistedOperations(operations);
  }
}

export function markSucceeded(scope: string, _payload?: unknown): void {
  void _payload;
  clearOperation(scope);
}

export function markDefinitelyNotExecuted(scope: string, _payload?: unknown): void {
  void _payload;
  clearOperation(scope);
}

export class IdempotencyManager {
  getOrCreateKey(scope: string, payload: unknown): Promise<string> { return getOrCreateOperation(scope, payload); }
  markUnknown(scope: string, payload: unknown): Promise<void> { return markUnknown(scope, payload); }
  markSucceeded(scope: string): void { markSucceeded(scope); }
  markDefinitelyNotExecuted(scope: string): void { markDefinitelyNotExecuted(scope); }
  clearKey(scope: string): void { clearOperation(scope); }
  getKey(scope: string): string | undefined { return loadPersistedOperations()[scope]?.idempotencyKey; }
}

export function useIdempotency() {
  const getOrCreateKey = useCallback((scope: string, payload: unknown) => getOrCreateOperation(scope, payload), []);
  const clearKey = useCallback((scope: string) => clearOperation(scope), []);
  const handleMarkUnknown = useCallback((scope: string, payload: unknown) => markUnknown(scope, payload), []);
  return { getOrCreateKey, clearKey, markUnknown: handleMarkUnknown, markSucceeded: clearKey, markDefinitelyNotExecuted: clearKey, getKeyForScope: (scope: string) => loadPersistedOperations()[scope]?.idempotencyKey };
}

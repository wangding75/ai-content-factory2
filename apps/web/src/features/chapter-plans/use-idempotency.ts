import { useCallback } from "react";

export interface PersistedOperation {
  version: 1;
  scope: string;
  payloadHash: string;
  idempotencyKey: string;
  status: "pending" | "unknown";
  createdAt: string;
  updatedAt: string;
}

const STORAGE_KEY = "acf:chapter-planning:idempotency:v1";

// In-memory fallback if sessionStorage is disabled, quota exceeded, or SSR
const memoryStore: Record<string, PersistedOperation> = {};

function safeGetStorage(): Storage | null {
  if (typeof window !== "undefined" && window.sessionStorage) {
    try {
      return window.sessionStorage;
    } catch {
      return null;
    }
  }
  return null;
}

export function canonicalizePayload(val: unknown): unknown {
  if (val === null || val === undefined) {
    return null;
  }
  if (typeof val === "boolean" || typeof val === "number" || typeof val === "string") {
    return val;
  }
  if (val instanceof Date) {
    return val.toISOString();
  }
  if (typeof val === "function" || typeof val === "symbol") {
    return undefined;
  }
  if (Array.isArray(val)) {
    return val.map((item) => canonicalizePayload(item));
  }
  if (typeof val === "object") {
    // Exclude React elements or DOM nodes
    if (
      "$$typeof" in val ||
      ("nodeType" in val && typeof (val as Record<string, unknown>).nodeType === "number")
    ) {
      return undefined;
    }
    const result: Record<string, unknown> = {};
    const keys = Object.keys(val as Record<string, unknown>).sort();
    for (const key of keys) {
      const v = (val as Record<string, unknown>)[key];
      if (v !== undefined && typeof v !== "function" && typeof v !== "symbol") {
        const canonicalV = canonicalizePayload(v);
        if (canonicalV !== undefined) {
          result[key] = canonicalV;
        }
      }
    }
    return result;
  }
  return String(val);
}

export function hashCanonicalPayload(payload: unknown): string {
  const canonical = canonicalizePayload(payload);
  return JSON.stringify(canonical);
}

export function loadPersistedOperations(): Record<string, PersistedOperation> {
  const storage = safeGetStorage();
  if (!storage) {
    return { ...memoryStore };
  }
  try {
    const raw = storage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as Record<string, PersistedOperation>;
    if (typeof parsed !== "object" || parsed === null) return {};
    return parsed;
  } catch {
    return {};
  }
}

function savePersistedOperations(ops: Record<string, PersistedOperation>): void {
  const storage = safeGetStorage();
  if (!storage) {
    Object.keys(memoryStore).forEach((key) => delete memoryStore[key]);
    Object.assign(memoryStore, ops);
    return;
  }
  try {
    storage.setItem(STORAGE_KEY, JSON.stringify(ops));
  } catch {
    Object.keys(memoryStore).forEach((key) => delete memoryStore[key]);
    Object.assign(memoryStore, ops);
  }
}

function generateUUID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  // Fallback UUID v4 generator
  return "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx".replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === "x" ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export function getOrCreateOperation(scope: string, payload: unknown): string {
  const ops = loadPersistedOperations();
  const payloadHash = hashCanonicalPayload(payload);
  const existing = ops[scope];

  if (existing && existing.payloadHash === payloadHash) {
    return existing.idempotencyKey;
  }

  const now = new Date().toISOString();
  const idempotencyKey = generateUUID();

  ops[scope] = {
    version: 1,
    scope,
    payloadHash,
    idempotencyKey,
    status: "pending",
    createdAt: now,
    updatedAt: now,
  };

  savePersistedOperations(ops);
  return idempotencyKey;
}

export function markUnknown(scope: string, payload: unknown): void {
  const ops = loadPersistedOperations();
  const payloadHash = hashCanonicalPayload(payload);
  const existing = ops[scope];

  const now = new Date().toISOString();
  if (existing && existing.payloadHash === payloadHash) {
    ops[scope] = {
      ...existing,
      status: "unknown",
      updatedAt: now,
    };
  } else {
    ops[scope] = {
      version: 1,
      scope,
      payloadHash,
      idempotencyKey: generateUUID(),
      status: "unknown",
      createdAt: now,
      updatedAt: now,
    };
  }
  savePersistedOperations(ops);
}

export function markSucceeded(scope: string, payload?: unknown): void {
  void payload;
  clearOperation(scope);
}

export function markDefinitelyNotExecuted(scope: string, payload?: unknown): void {
  void payload;
  clearOperation(scope);
}

export function clearOperation(scope: string): void {
  const ops = loadPersistedOperations();
  if (ops[scope]) {
    delete ops[scope];
    savePersistedOperations(ops);
  }
}

export class IdempotencyManager {
  getOrCreateKey(scope: string, payload: unknown): string {
    return getOrCreateOperation(scope, payload);
  }

  markUnknown(scope: string, payload: unknown): void {
    markUnknown(scope, payload);
  }

  markSucceeded(scope: string, ...args: unknown[]): void {
    markSucceeded(scope, ...args);
  }

  markDefinitelyNotExecuted(scope: string, ...args: unknown[]): void {
    markDefinitelyNotExecuted(scope, ...args);
  }

  clearKey(scope: string): void {
    clearOperation(scope);
  }

  getKey(scope: string): string | undefined {
    const ops = loadPersistedOperations();
    return ops[scope]?.idempotencyKey;
  }
}

export function useIdempotency() {
  const getOrCreateKey = useCallback((scope: string, payload: unknown): string => {
    return getOrCreateOperation(scope, payload);
  }, []);

  const clearKey = useCallback((scope: string) => {
    clearOperation(scope);
  }, []);

  const handleMarkUnknown = useCallback((scope: string, payload: unknown) => {
    markUnknown(scope, payload);
  }, []);

  const handleMarkSucceeded = useCallback((scope: string, ...args: unknown[]) => {
    markSucceeded(scope, ...args);
  }, []);

  const handleMarkDefinitelyNotExecuted = useCallback((scope: string, ...args: unknown[]) => {
    markDefinitelyNotExecuted(scope, ...args);
  }, []);

  const getKeyForScope = useCallback((scope: string): string | undefined => {
    const ops = loadPersistedOperations();
    return ops[scope]?.idempotencyKey;
  }, []);

  return {
    getOrCreateKey,
    clearKey,
    markUnknown: handleMarkUnknown,
    markSucceeded: handleMarkSucceeded,
    markDefinitelyNotExecuted: handleMarkDefinitelyNotExecuted,
    getKeyForScope,
  };
}

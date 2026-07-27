import { useCallback, useRef } from "react";

export interface IdempotencyKeyEntry {
  fingerprint: string;
  key: string;
}

function generateKey(scope: string): string {
  const rand = Math.random().toString(36).slice(2, 9);
  return `${scope}-${Date.now()}-${rand}`;
}

export function useIdempotency() {
  const activeKeys = useRef<Map<string, IdempotencyKeyEntry>>(new Map());

  const getOrCreateKey = useCallback((scope: string, payload: unknown): string => {
    const fingerprint = JSON.stringify(payload ?? null);
    const existing = activeKeys.current.get(scope);

    if (existing && existing.fingerprint === fingerprint) {
      return existing.key;
    }

    const newKey = generateKey(scope);
    activeKeys.current.set(scope, { fingerprint, key: newKey });
    return newKey;
  }, []);

  const clearKey = useCallback((scope: string) => {
    activeKeys.current.delete(scope);
  }, []);

  const resetKey = useCallback((scope: string) => {
    activeKeys.current.delete(scope);
  }, []);

  const getKeyForScope = useCallback((scope: string): string | undefined => {
    return activeKeys.current.get(scope)?.key;
  }, []);

  return {
    getOrCreateKey,
    clearKey,
    resetKey,
    getKeyForScope,
  };
}

// Pure helper class for non-hook / standalone API handlers if needed
export class IdempotencyManager {
  private activeKeys = new Map<string, IdempotencyKeyEntry>();

  getOrCreateKey(scope: string, payload: unknown): string {
    const fingerprint = JSON.stringify(payload ?? null);
    const existing = this.activeKeys.get(scope);

    if (existing && existing.fingerprint === fingerprint) {
      return existing.key;
    }

    const newKey = generateKey(scope);
    this.activeKeys.set(scope, { fingerprint, key: newKey });
    return newKey;
  }

  clearKey(scope: string): void {
    this.activeKeys.delete(scope);
  }

  resetKey(scope: string): void {
    this.activeKeys.delete(scope);
  }

  getKey(scope: string): string | undefined {
    return this.activeKeys.get(scope)?.key;
  }
}

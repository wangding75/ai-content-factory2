import assert from "node:assert/strict";
import test from "node:test";
import {
  canonicalizePayload,
  hashCanonicalPayload,
  getOrCreateOperation,
  markUnknown,
  markSucceeded,
  clearOperation,
  loadPersistedOperations,
  IdempotencyManager,
  IdempotencyCryptoUnavailableError,
} from "./use-idempotency.ts";

test("canonicalizePayload sorts object keys lexicographically and preserves array order", async () => {
  const objA = { z: 1, a: "hello", b: [10, 20] };
  const objB = { a: "hello", z: 1, b: [10, 20] };
  const objC = { a: "hello", z: 1, b: [20, 10] };

  assert.ok(canonicalizePayload(objA) !== null);
  assert.equal(await hashCanonicalPayload(objA), await hashCanonicalPayload(objB), "Different key order must produce identical canonical hash");
  assert.notEqual(await hashCanonicalPayload(objA), await hashCanonicalPayload(objC), "Different array order must produce different canonical hash");
});

test("canonicalizePayload omits undefined fields and formats Date objects to ISO strings", async () => {
  const d = new Date("2026-07-27T12:00:00.000Z");
  const payload1 = { a: 1, b: undefined, c: d };
  const payload2 = { c: "2026-07-27T12:00:00.000Z", a: 1 };

  assert.equal(await hashCanonicalPayload(payload1), await hashCanonicalPayload(payload2));
});

test("digest is not canonical JSON and persisted records exclude request content", async () => {
  const payload = { candidateBody: "private chapter text", additionalInstructions: "private instruction" };
  const digest = await hashCanonicalPayload(payload);
  assert.notEqual(digest, JSON.stringify(canonicalizePayload(payload)));
  assert.match(digest, /^sha256:[0-9a-f]{64}$/);

  const storage = new Map<string, string>();
  const originalWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  Object.defineProperty(globalThis, "window", { configurable: true, value: { sessionStorage: { getItem: (key: string) => storage.get(key) ?? null, setItem: (key: string, value: string) => storage.set(key, value) } } });
  try {
    await getOrCreateOperation("private-payload", payload);
    const persisted = storage.get("acf:chapter-planning:idempotency:v2") ?? "";
    assert.doesNotMatch(persisted, /private chapter text|private instruction/);
    assert.match(persisted, /payloadDigest/);
    assert.doesNotMatch(persisted, /payloadHash|candidateBody|additionalInstructions/);
  } finally {
    if (originalWindow) Object.defineProperty(globalThis, "window", originalWindow);
    else Reflect.deleteProperty(globalThis, "window");
  }
});

test("legacy v1 storage is ignored instead of being treated as a request payload", () => {
  const storage = new Map<string, string>();
  const originalWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  Object.defineProperty(globalThis, "window", { configurable: true, value: { sessionStorage: { getItem: () => JSON.stringify({ old: { version: 1, payloadHash: "{\\\"secret\\\":true}" } }), setItem: (key: string, value: string) => storage.set(key, value) } } });
  try {
    assert.deepEqual(loadPersistedOperations(), {});
  } finally {
    if (originalWindow) Object.defineProperty(globalThis, "window", originalWindow);
    else Reflect.deleteProperty(globalThis, "window");
  }
});

test("persisted operations accept only lowercase SHA-256 digests", () => {
  const storage = new Map<string, string>();
  const originalWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  Object.defineProperty(globalThis, "window", { configurable: true, value: { sessionStorage: { getItem: () => JSON.stringify({ legacy: { version: 2, scope: "write", payloadDigest: "fnv1a:deadbeef", idempotencyKey: "key", status: "pending", createdAt: "2026-01-01T00:00:00.000Z", updatedAt: "2026-01-01T00:00:00.000Z" } }), setItem: (key: string, value: string) => storage.set(key, value) } } });
  try {
    assert.deepEqual(loadPersistedOperations(), {});
  } finally {
    if (originalWindow) Object.defineProperty(globalThis, "window", originalWindow);
    else Reflect.deleteProperty(globalThis, "window");
  }
});

test("WebCrypto absence fails closed before a write request can be issued", async () => {
  const originalCrypto = Object.getOwnPropertyDescriptor(globalThis, "crypto");
  const originalFetch = globalThis.fetch;
  let writeRequests = 0;
  Object.defineProperty(globalThis, "crypto", { configurable: true, value: undefined });
  globalThis.fetch = (async () => {
    writeRequests += 1;
    return new Response();
  }) as typeof fetch;
  try {
    await assert.rejects(
      async () => {
        const key = await getOrCreateOperation("candidate:edit:blocked", { title: "private chapter text" });
        await fetch("/write", { method: "POST", headers: { "Idempotency-Key": key } });
      },
      IdempotencyCryptoUnavailableError,
    );
    assert.equal(writeRequests, 0);
  } finally {
    globalThis.fetch = originalFetch;
    if (originalCrypto) Object.defineProperty(globalThis, "crypto", originalCrypto);
    else Reflect.deleteProperty(globalThis, "crypto");
  }
});

test("idempotency manager generates valid UUID keys", async () => {
  const scope = "chapter-plan-run:create:p1";
  const payload = { preflightToken: "tok_123" };

  const key1 = await getOrCreateOperation(scope, payload);
  assert.ok(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(key1),
    "Idempotency Key must be a valid UUID v4",
  );
});

test("idempotency manager reuses key for exact same scope and payload (retry / timeout)", async () => {
  const scope = "candidate:adopt:cand-1";
  const payload = { expectedCandidateVersion: 2, expectedChapterPlanVersion: 5 };

  const key1 = await getOrCreateOperation(scope, payload);
  const key2 = await getOrCreateOperation(scope, payload);

  assert.equal(key1, key2, "Retrying identical operation must reuse the original key");
});

test("idempotency manager generates a new key when request payload changes", async () => {
  const scope = "candidate:adopt:cand-1";
  const payloadA = { expectedCandidateVersion: 2, expectedChapterPlanVersion: 5 };
  const payloadB = { expectedCandidateVersion: 3, expectedChapterPlanVersion: 5 };

  const key1 = await getOrCreateOperation(scope, payloadA);
  const key2 = await getOrCreateOperation(scope, payloadB);

  assert.notEqual(key1, key2, "Changing payload must produce a new idempotency key");
});

test("idempotency manager clears key on explicit success so next action gets a fresh key", async () => {
  const scope = "candidate:edit:c100";
  const payload = { expectedCandidateVersion: 1, currentSnapshot: { title: "Test" } };

  const key1 = await getOrCreateOperation(scope, payload);
  markSucceeded(scope, payload);

  const ops = loadPersistedOperations();
  assert.equal(ops[scope], undefined, "Completed operation must be cleared");

  const key2 = await getOrCreateOperation(scope, payload);
  assert.notEqual(key1, key2, "New action after success must get a fresh idempotency key");
});

test("idempotency manager preserves key when status is marked unknown after network error", async () => {
  const scope = "batch:abandon:b1";
  const payload = { expectedBatchVersion: 1, acknowledgeAdoptedChaptersRemain: true };

  const key1 = await getOrCreateOperation(scope, payload);
  await markUnknown(scope, payload);

  const ops = loadPersistedOperations();
  assert.equal(ops[scope]?.status, "unknown");

  const keyRetry = await getOrCreateOperation(scope, payload);
  assert.equal(key1, keyRetry, "Preserves idempotency key when operation result is unknown");

  clearOperation(scope);
});

test("IdempotencyManager class provides consistent interface", async () => {
  const mgr = new IdempotencyManager();
  const scope = "candidate:recompare:cand-99";
  const payload = { expectedCandidateVersion: 4 };

  const k1 = await mgr.getOrCreateKey(scope, payload);
  assert.equal(mgr.getKey(scope), k1);

  await mgr.markUnknown(scope, payload);
  assert.equal(mgr.getKey(scope), k1);

  mgr.clearKey(scope);
  assert.equal(mgr.getKey(scope), undefined);
});

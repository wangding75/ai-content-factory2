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
} from "./use-idempotency.ts";

test("canonicalizePayload sorts object keys lexicographically and preserves array order", () => {
  const objA = { z: 1, a: "hello", b: [10, 20] };
  const objB = { a: "hello", z: 1, b: [10, 20] };
  const objC = { a: "hello", z: 1, b: [20, 10] };

  assert.ok(canonicalizePayload(objA) !== null);
  assert.equal(hashCanonicalPayload(objA), hashCanonicalPayload(objB), "Different key order must produce identical canonical hash");
  assert.notEqual(hashCanonicalPayload(objA), hashCanonicalPayload(objC), "Different array order must produce different canonical hash");
});

test("canonicalizePayload omits undefined fields and formats Date objects to ISO strings", () => {
  const d = new Date("2026-07-27T12:00:00.000Z");
  const payload1 = { a: 1, b: undefined, c: d };
  const payload2 = { c: "2026-07-27T12:00:00.000Z", a: 1 };

  assert.equal(hashCanonicalPayload(payload1), hashCanonicalPayload(payload2));
});

test("idempotency manager generates valid UUID keys", () => {
  const scope = "chapter-plan-run:create:p1";
  const payload = { preflightToken: "tok_123" };

  const key1 = getOrCreateOperation(scope, payload);
  assert.ok(
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(key1),
    "Idempotency Key must be a valid UUID v4",
  );
});

test("idempotency manager reuses key for exact same scope and payload (retry / timeout)", () => {
  const scope = "candidate:adopt:cand-1";
  const payload = { expectedCandidateVersion: 2, expectedChapterPlanVersion: 5 };

  const key1 = getOrCreateOperation(scope, payload);
  const key2 = getOrCreateOperation(scope, payload);

  assert.equal(key1, key2, "Retrying identical operation must reuse the original key");
});

test("idempotency manager generates a new key when request payload changes", () => {
  const scope = "candidate:adopt:cand-1";
  const payloadA = { expectedCandidateVersion: 2, expectedChapterPlanVersion: 5 };
  const payloadB = { expectedCandidateVersion: 3, expectedChapterPlanVersion: 5 };

  const key1 = getOrCreateOperation(scope, payloadA);
  const key2 = getOrCreateOperation(scope, payloadB);

  assert.notEqual(key1, key2, "Changing payload must produce a new idempotency key");
});

test("idempotency manager clears key on explicit success so next action gets a fresh key", () => {
  const scope = "candidate:edit:c100";
  const payload = { expectedCandidateVersion: 1, currentSnapshot: { title: "Test" } };

  const key1 = getOrCreateOperation(scope, payload);
  markSucceeded(scope, payload);

  const ops = loadPersistedOperations();
  assert.equal(ops[scope], undefined, "Completed operation must be cleared");

  const key2 = getOrCreateOperation(scope, payload);
  assert.notEqual(key1, key2, "New action after success must get a fresh idempotency key");
});

test("idempotency manager preserves key when status is marked unknown after network error", () => {
  const scope = "batch:abandon:b1";
  const payload = { expectedBatchVersion: 1, acknowledgeAdoptedChaptersRemain: true };

  const key1 = getOrCreateOperation(scope, payload);
  markUnknown(scope, payload);

  const ops = loadPersistedOperations();
  assert.equal(ops[scope]?.status, "unknown");

  const keyRetry = getOrCreateOperation(scope, payload);
  assert.equal(key1, keyRetry, "Preserves idempotency key when operation result is unknown");

  clearOperation(scope);
});

test("IdempotencyManager class provides consistent interface", () => {
  const mgr = new IdempotencyManager();
  const scope = "candidate:recompare:cand-99";
  const payload = { expectedCandidateVersion: 4 };

  const k1 = mgr.getOrCreateKey(scope, payload);
  assert.equal(mgr.getKey(scope), k1);

  mgr.markUnknown(scope, payload);
  assert.equal(mgr.getKey(scope), k1);

  mgr.clearKey(scope);
  assert.equal(mgr.getKey(scope), undefined);
});

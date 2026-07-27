import assert from "node:assert/strict";
import test from "node:test";
import { IdempotencyManager } from "./use-idempotency.ts";

test("idempotency manager reuses key for exact same scope and payload (retry / timeout)", () => {
  const mgr = new IdempotencyManager();
  const scope = "single-adopt-cand-1";
  const payload = { expectedCandidateVersion: 2, expectedChapterPlanVersion: 5 };

  const key1 = mgr.getOrCreateKey(scope, payload);
  const key2 = mgr.getOrCreateKey(scope, payload);

  assert.ok(key1.length > 0);
  assert.equal(key1, key2, "Retrying identical operation must reuse the original key");
});

test("idempotency manager generates a new key when request payload changes", () => {
  const mgr = new IdempotencyManager();
  const scope = "single-adopt-cand-1";
  const payloadA = { expectedCandidateVersion: 2, expectedChapterPlanVersion: 5 };
  const payloadB = { expectedCandidateVersion: 3, expectedChapterPlanVersion: 5 };

  const key1 = mgr.getOrCreateKey(scope, payloadA);
  const key2 = mgr.getOrCreateKey(scope, payloadB);

  assert.notEqual(key1, key2, "Changing payload must produce a new idempotency key");
});

test("idempotency manager clears key on explicit success so next action gets a fresh key", () => {
  const mgr = new IdempotencyManager();
  const scope = "edit-candidate-c100";
  const payload = { expectedCandidateVersion: 1, currentSnapshot: { title: "Test" } };

  const key1 = mgr.getOrCreateKey(scope, payload);
  assert.equal(mgr.getKey(scope), key1);

  // Clear key on explicit success
  mgr.clearKey(scope);
  assert.equal(mgr.getKey(scope), undefined);

  const key2 = mgr.getOrCreateKey(scope, payload);
  assert.notEqual(key1, key2, "New action after success must get a fresh idempotency key");
});

test("idempotency manager preserves key when result is unknown or fails without explicit cleanup", () => {
  const mgr = new IdempotencyManager();
  const scope = "abandon-batch-b1";
  const payload = { expectedBatchVersion: 1, acknowledgeAdoptedChaptersRemain: true };

  const key1 = mgr.getOrCreateKey(scope, payload);
  // Simulating network error / timeout without clearKey
  const keyRetry = mgr.getOrCreateKey(scope, payload);

  assert.equal(key1, keyRetry, "Preserves idempotency key when operation result is unknown");
});

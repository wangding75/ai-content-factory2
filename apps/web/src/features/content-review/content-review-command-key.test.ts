import assert from "node:assert/strict";
import test from "node:test";
import {
  isUncertainReviewCommandError,
  reviewCommandKey,
} from "./content-review-command-key.ts";

test("review command keys survive uncertain results and change with command input", () => {
  let sequence = 0;
  const create = () => `key-${++sequence}`;
  const first = reviewCommandKey(null, "run:1", create);
  const uncertainRetry = reviewCommandKey(first, "run:1", create);
  const changed = reviewCommandKey(uncertainRetry, "run:2", create);

  assert.equal(uncertainRetry.key, first.key);
  assert.notEqual(changed.key, first.key);
  assert.equal(isUncertainReviewCommandError({ status: 0 }), true);
  assert.equal(isUncertainReviewCommandError({ status: 500 }), true);
  assert.equal(isUncertainReviewCommandError({ status: 409 }), false);
});

test("all four review command signatures reuse their original key after a lost response", () => {
  const signatures = [
    "create:version:token",
    "runtime-retry:run:3",
    "consumption-retry:run:3",
    "issue:issue:ignored:2",
  ];
  for (const signature of signatures) {
    const first = reviewCommandKey(null, signature, () => `${signature}:key`);
    const retry = reviewCommandKey(first, signature, () => "unexpected-new-key");
    assert.equal(retry.key, first.key);
  }
});

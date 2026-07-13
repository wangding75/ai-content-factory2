import assert from "node:assert/strict";
import test from "node:test";
import { ApiError, apiRequest } from "./api.ts";

const originalFetch = global.fetch;
test.after(() => { global.fetch = originalFetch; });

test("maps network failures", async () => {
  global.fetch = async () => { throw new TypeError("offline"); };
  await assert.rejects(apiRequest("/planning"), (error: unknown) => error instanceof ApiError && error.code === "network_error");
});

test("rejects non-JSON responses", async () => {
  global.fetch = async () => new Response("gateway", { status: 502 });
  await assert.rejects(apiRequest("/planning"), (error: unknown) => error instanceof ApiError && error.code === "invalid_json");
});

test("maps timeout and cancellation", async () => {
  global.fetch = async (_input, init) => new Promise<Response>((_resolve, reject) => init?.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError"))));
  await assert.rejects(apiRequest("/planning", { timeoutMs: 1 }), (error: unknown) => error instanceof ApiError && error.code === "timeout");
  const controller = new AbortController();
  const pending = apiRequest("/planning", { signal: controller.signal });
  controller.abort();
  await assert.rejects(pending, (error: unknown) => error instanceof ApiError && error.code === "cancelled");
});

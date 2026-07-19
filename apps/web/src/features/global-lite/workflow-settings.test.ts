import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const settings = readFileSync(new URL("./settings-page.tsx", import.meta.url), "utf8");
const llmApi = readFileSync(new URL("../global-config/llm-provider-api.ts", import.meta.url), "utf8");
test("Iteration 12 LLM settings use mapped real CRUD APIs with loading, empty, error and conflict states", () => {
  for (const text of ["listLlmProviders", "listLlmProviderTypes", "getLlmProvider", "createLlmProvider", "updateLlmProvider", "mapLlmProvider", "AbortController", "version_conflict", "onSaved"]) assert.match(settings, new RegExp(text));
  assert.match(llmApi, /Idempotency-Key/);
  assert.doesNotMatch(`${settings}\n${llmApi}`, /\/verify|\/enable|\/disable|\/models/);
});
test("secret data is input-only and never becomes a rendered provider view model", () => {
  assert.match(settings, /type="password"/);
  assert.doesNotMatch(llmApi, /secret:\s*string.*LlmProviderDto/);
});

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const settings = readFileSync(new URL("./settings-page.tsx", import.meta.url), "utf8");
const llmApi = readFileSync(new URL("../global-config/llm-provider-api.ts", import.meta.url), "utf8");
test("LLM settings use mapped CRUD plus independent verification, discovery and enablement commands", () => {
  for (const text of ["listLlmProviders", "listLlmProviderTypes", "getLlmProvider", "createLlmProvider", "updateLlmProvider", "verifyLlmProvider", "discoverLlmProviderModels", "setLlmProviderEnabled", "mapLlmProvider", "AbortController", "version_conflict", "onSaved"]) assert.match(settings, new RegExp(text));
  assert.match(llmApi, /Idempotency-Key/);
  for (const text of ["/verify", "/models/discover", "setLlmProviderEnabled"]) assert.match(llmApi, new RegExp(text.replaceAll("/", "\\/")));
});
test("secret data is input-only and never becomes a rendered provider view model", () => {
  assert.match(settings, /type="password"/);
  assert.doesNotMatch(llmApi, /secret:\s*string.*LlmProviderDto/);
});

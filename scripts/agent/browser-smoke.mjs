import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import process from "node:process";

function readArg(name) {
  const index = process.argv.indexOf(name);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

const configPath = readArg("--config");
if (!configPath) {
  throw new Error("Missing --config <json-file>.");
}

const config = JSON.parse(fs.readFileSync(path.resolve(configPath), "utf8"));

let chromium;
try {
  if (!config.playwrightPackageJson) {
    throw new Error("Missing playwrightPackageJson in smoke configuration.");
  }
  const requireFromWeb = createRequire(path.resolve(config.playwrightPackageJson));
  ({ chromium } = requireFromWeb("@playwright/test"));
} catch (error) {
  console.error("Unable to load @playwright/test from apps/web.");
  console.error(error);
  process.exit(2);
}

const browser = await chromium.launch({ headless: true });
const results = [];

try {
  for (const route of config.routes) {
    const page = await browser.newPage({
      viewport: {
        width: config.viewport?.width ?? 1440,
        height: config.viewport?.height ?? 1000,
      },
    });

    const consoleErrors = [];
    const pageErrors = [];
    const failedResponses = [];
    const failedRequests = [];

    page.on("console", (message) => {
      if (message.type() === "error") consoleErrors.push(message.text());
    });
    page.on("pageerror", (error) => pageErrors.push(String(error)));
    page.on("response", (response) => {
      if (response.status() >= 400) {
        failedResponses.push(`${response.status()} ${response.url()}`);
      }
    });
    page.on("requestfailed", (request) => {
      failedRequests.push(`${request.failure()?.errorText ?? "failed"} ${request.url()}`);
    });

    const url = new URL(route, config.baseUrl).toString();
    const response = await page.goto(url, {
      waitUntil: "networkidle",
      timeout: config.timeoutMs ?? 45000,
    });

    if (!response || response.status() >= 400) {
      throw new Error(`Route failed: ${url}, status=${response?.status() ?? "none"}`);
    }

    await page.waitForTimeout(config.settleMs ?? 500);

    const pageState = await page.evaluate(() => ({
      text: document.body?.innerText ?? "",
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
      title: document.title,
    }));

    const forbiddenMatches = [];
    for (const patternText of config.forbiddenPatterns ?? []) {
      const pattern = new RegExp(patternText, "i");
      if (pattern.test(pageState.text)) forbiddenMatches.push(patternText);
    }

    const allowedFailurePatterns = (config.allowedFailurePatterns ?? []).map(
      (value) => new RegExp(value, "i"),
    );

    const filterAllowed = (items) =>
      items.filter(
        (item) => !allowedFailurePatterns.some((pattern) => pattern.test(item)),
      );

    const routeResult = {
      route,
      url,
      title: pageState.title,
      horizontalOverflow: pageState.scrollWidth > pageState.clientWidth + 1,
      consoleErrors,
      pageErrors,
      failedResponses: filterAllowed(failedResponses),
      failedRequests: filterAllowed(failedRequests),
      forbiddenMatches,
    };

    results.push(routeResult);
    await page.close();
  }
} finally {
  await browser.close();
}

const failures = results.filter(
  (item) =>
    item.horizontalOverflow ||
    item.consoleErrors.length ||
    item.pageErrors.length ||
    item.failedResponses.length ||
    item.failedRequests.length ||
    item.forbiddenMatches.length,
);

const output = {
  result: failures.length === 0 ? "passed" : "failed",
  checkedAt: new Date().toISOString(),
  results,
};

if (config.outputPath) {
  fs.mkdirSync(path.dirname(path.resolve(config.outputPath)), { recursive: true });
  fs.writeFileSync(
    path.resolve(config.outputPath),
    `${JSON.stringify(output, null, 2)}\n`,
    "utf8",
  );
}

console.log(JSON.stringify(output, null, 2));
if (failures.length > 0) process.exit(1);

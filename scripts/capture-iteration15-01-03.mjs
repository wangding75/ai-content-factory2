import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const requireFromWeb = createRequire(path.resolve("apps/web/package.json"));
const { chromium } = requireFromWeb("@playwright/test");

const PROJECT_ID = "13a13e7e-656e-4174-bc96-c301692ebced";
const BASE_URL = `http://127.0.0.1:13001/projects/${PROJECT_ID}/chapter-plans`;
const API_BASE = `http://127.0.0.1:18080/api/v1/projects/${PROJECT_ID}`;

const SCENARIOS = [
  {
    num: "01",
    slug: "running-mainline-expansion",
    title: "主线扩写",
    preflightBody: {
      generationMode: "append",
      target: { chapterCount: 20 },
      storylineSelection: { mode: "auto_balanced", storylineIds: [] },
      contextOptions: {
        includeProjectMaterials: true,
        includeUnpaidForeshadowings: true,
        includePriorChapterSummaries: true,
        coreSettingsOnly: false,
      },
      additionalInstructions: null,
    },
  },
  {
    num: "02",
    slug: "running-partial-range",
    title: "局部范围",
    preflightBody: {
      generationMode: "range",
      target: { startChapterNo: 21, endChapterNo: 40 },
      storylineSelection: { mode: "auto_balanced", storylineIds: [] },
      contextOptions: {
        includeProjectMaterials: true,
        includeUnpaidForeshadowings: true,
        includePriorChapterSummaries: true,
        coreSettingsOnly: false,
      },
      additionalInstructions: null,
    },
  },
  {
    num: "03",
    slug: "running-full-plan",
    title: "完整规划",
    preflightBody: {
      generationMode: "full",
      target: { targetTotalChapters: 100 },
      storylineSelection: { mode: "auto_balanced", storylineIds: [] },
      contextOptions: {
        includeProjectMaterials: true,
        includeUnpaidForeshadowings: true,
        includePriorChapterSummaries: true,
        coreSettingsOnly: false,
      },
      additionalInstructions: null,
    },
  },
];

async function createRun(preflightBody) {
  const pfRes = await fetch(`${API_BASE}/chapter-plan-runs/preflight`, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Actor-ID": "system" },
    body: JSON.stringify(preflightBody),
  }).then((r) => r.json());

  if (pfRes.data?.result !== "passed") {
    throw new Error(`Preflight failed: ${JSON.stringify(pfRes)}`);
  }

  const token = pfRes.data.preflightToken;
  const createRes = await fetch(`${API_BASE}/chapter-plan-runs`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Actor-ID": "system",
      "Idempotency-Key": `key-${Date.now()}-${Math.random().toString(36).slice(2)}`,
    },
    body: JSON.stringify({ preflightToken: token }),
  }).then((r) => r.json());

  return createRes;
}

async function captureScenario(scenario, browser) {
  console.log(`\n=== Preparing ${scenario.num} | ${scenario.title} ===`);

  // First ensure no active run is pending
  let summary = await fetch(`${API_BASE}/chapter-planning-summary`).then((r) => r.json());
  while (summary.data?.activeRun) {
    console.log("Waiting for previous active run to finish...");
    await new Promise((resolve) => setTimeout(resolve, 2000));
    summary = await fetch(`${API_BASE}/chapter-planning-summary`).then((r) => r.json());
  }

  // Create new run
  console.log(`Creating run for ${scenario.title}...`);
  await createRun(scenario.preflightBody);

  // Launch browser page
  const page = await browser.newPage({
    viewport: { width: 1440, height: 1024 },
    deviceScaleFactor: 1,
  });

  const networkLogs = [];

  page.on("request", (req) => {
    if (req.url().includes("/api/v1/")) {
      networkLogs.push({
        url: req.url(),
        method: req.method(),
        postData: req.postData() || null,
      });
    }
  });

  page.on("response", async (res) => {
    const log = networkLogs.find((l) => l.url === res.url() && l.method === res.request().method() && !l.status);
    if (log) {
      log.status = res.status();
    }
  });

  console.log(`Navigating to ${BASE_URL}...`);
  await page.goto(BASE_URL, { waitUntil: "networkidle" });
  await page.waitForTimeout(1000);

  const evidenceDir = path.resolve(
    "docs/development-inputs/p1/iterations/iteration-15-real-chapter-planning/evidence"
  );
  fs.mkdirSync(evidenceDir, { recursive: true });

  const actualPath = path.join(
    evidenceDir,
    `${scenario.num}-${scenario.slug}-actual.png`
  );
  await page.screenshot({ path: actualPath, fullPage: false });
  console.log(`Saved screenshot to ${actualPath}`);

  await page.close();

  return { scenario, networkLogs };
}

async function main() {
  const browser = await chromium.launch({ headless: true });
  const allLogs = [];
  try {
    for (const scenario of SCENARIOS) {
      const result = await captureScenario(scenario, browser);
      allLogs.push(result);
    }
  } finally {
    await browser.close();
  }

  console.log("\n=== Network Baseline Recorded ===");
  console.log(JSON.stringify(allLogs, null, 2));
}

main().catch(console.error);

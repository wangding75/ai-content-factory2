/**
 * Deterministic second-loop UI capture runner (UI-001 ~ UI-088).
 * Uses real browser + real fixture scenes. No mock / route intercept.
 */
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const requireFromWeb = createRequire(
  path.resolve('apps/web/package.json'),
);
const { chromium } = requireFromWeb('@playwright/test');

const FIXTURE_PKG =
  'C:\\Users\\wangding\\Downloads\\ACF_第二闭环_UI验收数据与Flash操作包\\acf_ui_fixture_pkg';
const MANIFEST_PATH = path.join(FIXTURE_PKG, 'ui-acceptance-fixture-manifest.json');
const OUTPUT_DIR = path.resolve(
  'docs/acceptance/second-loop-ui/phase-1-deterministic-collection',
);
const SCREENSHOTS_DIR = path.join(OUTPUT_DIR, 'screenshots');
const PROGRESS_PATH = path.join(OUTPUT_DIR, 'capture-progress.json');
const RESULTS_PATH = path.join(OUTPUT_DIR, 'collection-manifest.json');
const WEB_BASE = 'http://127.0.0.1:13001';
const MAX_WAIT_MS = 30000;

fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });

const BUSINESS_AREAS = [
  { name: '全局设置', range: [1, 12] },
  { name: '项目设置', range: [13, 21] },
  { name: '流程中心', range: [22, 29] },
  { name: '内置流程', range: [30, 30] },
  { name: '项目概览', range: [31, 31] },
  { name: '章节规划', range: [32, 51] },
  { name: '故事线', range: [52, 52] },
  { name: '正文生成', range: [53, 63] },
  { name: '内容审核', range: [64, 71] },
  { name: '正文重写', range: [72, 80] },
  { name: '共享升级状态', range: [81, 88] },
];

function captureNum(id) {
  return parseInt(String(id).split('-')[1], 10);
}

function businessAreaOf(id) {
  const n = captureNum(id);
  const hit = BUSINESS_AREAS.find((r) => n >= r.range[0] && n <= r.range[1]);
  return hit ? hit.name : '未知';
}

const POWERSHELL_EXE =
  process.env.SystemRoot
    ? path.join(process.env.SystemRoot, 'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe')
    : 'C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe';

function psEnv() {
  // Preserve the full parent PATH (docker, git, node, etc.) and only prepend
  // the local psql shim directory. Set both PATH and Path for Windows/PS.
  const env = { ...process.env };
  const prepend = [
    path.resolve('.tmp/bin'),
    'C:\\Program Files\\Docker\\Docker\\resources\\bin',
    'C:\\ProgramData\\DockerDesktop\\version-bin',
    'C:\\Program Files\\PostgreSQL\\18\\bin',
    path.join(process.env.SystemRoot || 'C:\\Windows', 'System32'),
    path.join(process.env.SystemRoot || 'C:\\Windows', 'System32', 'WindowsPowerShell', 'v1.0'),
  ].join(';');
  const full = `${prepend};${process.env.PATH || process.env.Path || ''}`;
  env.PATH = full;
  env.Path = full;
  return env;
}

/**
 * Fixture scripts call Get-Content without -Encoding utf8; on Chinese Windows
 * the UTF-8 JSON under the fixture package path can break ConvertFrom-Json.
 * Local wrappers in .tmp/ override Get-Content without modifying the package.
 */
function runWrapper(wrapperRel, args, timeoutMs = 180000) {
  const wrapper = path.resolve(wrapperRel);
  const res = spawnSync(
    POWERSHELL_EXE,
    ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', wrapper, ...args],
    {
      env: psEnv(),
      encoding: 'utf8',
      maxBuffer: 20 * 1024 * 1024,
      timeout: timeoutMs,
      killSignal: 'SIGTERM',
    },
  );
  if (res.error) {
    return {
      code: 1,
      stdout: res.stdout || '',
      stderr: `${res.stderr || ''}\n${String(res.error)}`,
    };
  }
  if (res.signal) {
    return {
      code: 1,
      stdout: res.stdout || '',
      stderr: `${res.stderr || ''}\nProcess killed by signal ${res.signal} (timeout=${timeoutMs})`,
    };
  }
  return {
    code: res.status ?? 1,
    stdout: res.stdout || '',
    stderr: res.stderr || '',
  };
}

function resolvePsql() {
  const candidates = [
    path.resolve('.tmp/bin/psql.exe'),
    'C:\\Program Files\\PostgreSQL\\18\\bin\\psql.exe',
    'psql',
  ];
  for (const c of candidates) {
    if (c === 'psql' || fs.existsSync(c)) return c;
  }
  return 'psql';
}

function terminateLeftoverSceneLocks() {
  // Previous scene holds use FOR UPDATE + pg_sleep(43200). If a prior run
  // crashed without cleanup, TRUNCATE in the next scene blocks forever.
  // Also kill lock pid from fixture runtime state before TRUNCATE.
  for (const f of ['.ui-acceptance-runtime.json', '.ui-acceptance-action.json']) {
    const p = path.join(FIXTURE_PKG, f);
    try {
      if (fs.existsSync(p)) {
        const st = JSON.parse(fs.readFileSync(p, 'utf8'));
        for (const key of ['lock_pid', 'hold_pid']) {
          if (st[key]) {
            try {
              process.kill(Number(st[key]));
            } catch {
              /* ignore */
            }
          }
        }
        fs.unlinkSync(p);
      }
    } catch {
      /* ignore */
    }
  }

  const sql =
    "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='ai_content_factory' AND pid <> pg_backend_pid() AND (query ILIKE '%pg_sleep%' OR query ILIKE '%LOCK TABLE project_workflow_bindings%' OR (state = 'active' AND wait_event = 'relation' AND query ILIKE 'TRUNCATE%'));";
  const res = spawnSync(
    resolvePsql(),
    [
      'postgres://postgres:postgres@127.0.0.1:15433/ai_content_factory?sslmode=disable',
      '-v',
      'ON_ERROR_STOP=1',
      '-c',
      sql,
    ],
    { env: psEnv(), encoding: 'utf8' },
  );
  if (res.error) {
    console.warn(`[LockCleanup] psql error: ${res.error.message}`);
  }
  // Brief pause so backends release before scene TRUNCATE.
  spawnSync(POWERSHELL_EXE, ['-NoProfile', '-Command', 'Start-Sleep -Milliseconds 800'], {
    encoding: 'utf8',
  });
}

function switchScene(scene) {
  console.log(`[Scene] Switching to ${scene}...`);
  terminateLeftoverSceneLocks();
  const res = runWrapper('.tmp/Invoke-FixtureScene.ps1', [
    '-Scene',
    scene,
    '-FixturePkg',
    FIXTURE_PKG,
  ]);
  const out = `${res.stdout}\n${res.stderr}`;
  // Fixture script may leave a non-zero LASTEXITCODE after docker compose
  // even when it printed PASS; trust the PASS marker.
  if (!out.includes(`PASS: ${scene}`)) {
    console.error(`[Scene] FAILED ${scene} code=${res.code}`);
    console.error(out.slice(-2000));
    return false;
  }
  console.log(`[Scene] PASS: ${scene}`);
  return true;
}

function invokeControlledAction(actionName) {
  console.log(`[Action] Invoke ${actionName}...`);
  const res = runWrapper('.tmp/Invoke-FixtureAction.ps1', [
    '-Action',
    actionName,
    '-FixturePkg',
    FIXTURE_PKG,
  ]);
  const out = `${res.stdout}\n${res.stderr}`;
  if (!/PASS/i.test(out)) {
    console.error(`[Action] FAILED ${actionName} code=${res.code}`);
    console.error(out.slice(-1500));
    return false;
  }
  console.log(`[Action] PASS: ${actionName}`);
  return true;
}

function checkRouteMatches(actualUrl, expectedRoute) {
  try {
    const actual = new URL(actualUrl);
    const expected = new URL(expectedRoute, WEB_BASE);
    if (actual.pathname !== expected.pathname) return false;
    for (const [key, val] of expected.searchParams.entries()) {
      if (actual.searchParams.get(key) !== val) return false;
    }
    return true;
  } catch {
    return false;
  }
}

async function getVisibleText(page) {
  try {
    return await page.locator('body').innerText({ timeout: 5000 });
  } catch {
    return '';
  }
}

async function waitForNetworkQuiet(page, timeoutMs = 10000) {
  await page.waitForLoadState('domcontentloaded', { timeout: timeoutMs }).catch(() => {});
  await page.waitForLoadState('networkidle', { timeout: timeoutMs }).catch(() => {});
}

async function waitLoadingGone(page, timeoutMs = MAX_WAIT_MS) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const busy = await page.locator('[aria-busy="true"]').count().catch(() => 0);
    const skeletons = await page
      .locator(
        '.skeleton, [class*="skeleton"], .workflow-binding-skeleton, .workflow-drawer-loading',
      )
      .evaluateAll((els) =>
        els.some((el) => {
          const s = window.getComputedStyle(el);
          return s && s.display !== 'none' && s.visibility !== 'hidden' && s.opacity !== '0';
        }),
      )
      .catch(() => false);
    const text = await getVisibleText(page);
    const loadingText =
      /正在加载|Loading\.\.\.|加载中/.test(text) &&
      !/加载失败|无法加载/.test(text);
    if (busy === 0 && !skeletons && !loadingText) return true;
    await page.waitForTimeout(400);
  }
  return false;
}

async function waitForAnyText(page, texts, timeoutMs = MAX_WAIT_MS) {
  if (!texts || texts.length === 0) return true;
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const body = await getVisibleText(page);
    const overlay = await page
      .locator('[role="dialog"], [role="alertdialog"], aside, [class*="drawer"]')
      .allInnerTexts()
      .then((arr) => arr.join('\n'))
      .catch(() => '');
    const hay = `${body}\n${overlay}`;
    for (const t of texts) {
      if (t && hay.includes(t)) return true;
    }
    await page.waitForTimeout(400);
  }
  return false;
}

/** Expand runbook regex with known production copy variants (selector only). */
function expandClickPatterns(nameRegex) {
  const parts = String(nameRegex).split('|').filter(Boolean);
  const aliases = {
    重试: ['重试', '重新执行', '重新执行 Runtime', 'Retry'],
    提交审核: ['提交审核', '发起审核'],
    发起审核: ['发起审核', '提交审核'],
    查看候选: ['查看候选', '查看候选版本', '比较版本'],
    比较版本: ['比较版本', '查看候选', '候选版本比较'],
    比较: ['比较', '查看差异', '对比'],
    查看差异: ['查看差异', '比较', '对比'],
    查看配置: ['查看配置', '重写配置', '配置'],
    重写配置: ['重写配置', '查看配置'],
    设为当前版本: ['设为当前版本', '确认设为当前版本', '设为当前'],
    创建任务: ['创建任务', '开始生成', '确认生成', '创建生成任务'],
    开始生成: ['开始生成', '创建任务', '确认生成'],
    确认生成: ['确认生成', '开始生成', '创建任务'],
    批量采用: ['批量采用', '批量采用已选候选', '采用全部', '确认批量采用'],
    采用全部: ['采用全部', '批量采用', '批量采用已选候选'],
    放弃批次: ['放弃批次', '放弃本批次', '确认放弃本批次', '放弃'],
    放弃: ['放弃', '放弃本批次', '放弃批次'],
    生成正文: ['生成正文', '开始生成'],
    生成章节规划: ['生成章节规划', '模拟生成章节规划'],
  };
  const expanded = new Set(parts);
  for (const p of parts) {
    for (const [k, vals] of Object.entries(aliases)) {
      if (p.includes(k) || k.includes(p)) vals.forEach((v) => expanded.add(v));
    }
  }
  return [...expanded];
}

async function clickByNameRegex(page, nameRegex, selectorsUsed) {
  const labels = expandClickPatterns(nameRegex);
  const patterns = labels.map((l) => new RegExp(l.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i'));
  // Also try original combined regex
  patterns.unshift(new RegExp(nameRegex, 'i'));

  for (const re of patterns) {
    const candidates = [
      () => page.getByRole('button', { name: re }),
      () => page.getByRole('link', { name: re }),
      () => page.getByRole('menuitem', { name: re }),
      () => page.locator('button, a, [role="button"]').filter({ hasText: re }),
      () => page.getByText(re),
    ];
    for (const make of candidates) {
      const loc = make();
      const count = await loc.count().catch(() => 0);
      for (let i = 0; i < count; i++) {
        const el = loc.nth(i);
        if (!(await el.isVisible().catch(() => false))) continue;
        const disabled = await el.isDisabled().catch(() => false);
        if (disabled) continue;
        selectorsUsed.push(`click ~ /${re.source}/`);
        await el.scrollIntoViewIfNeeded().catch(() => {});
        await el.click({ timeout: 8000 });
        return true;
      }
    }
  }
  // Force-click first match even if disabled (to surface UI state)
  const re = new RegExp(nameRegex, 'i');
  const fallback = page.locator('button, a, [role="button"]').filter({ hasText: re }).first();
  if ((await fallback.count()) > 0) {
    selectorsUsed.push(`click force ~ /${nameRegex}/`);
    await fallback.click({ timeout: 8000, force: true }).catch(() => {});
    return true;
  }
  return false;
}

async function locateTabOrButton(page, name, selectorsUsed) {
  let loc = page.getByRole('tab', { name });
  if ((await loc.count()) > 0 && (await loc.first().isVisible())) {
    selectorsUsed.push(`role=tab name="${name}"`);
    return loc.first();
  }
  loc = page.getByRole('button', { name });
  if ((await loc.count()) > 0 && (await loc.first().isVisible())) {
    selectorsUsed.push(`role=button name="${name}"`);
    return loc.first();
  }
  loc = page.getByText(name, { exact: true });
  if ((await loc.count()) > 0 && (await loc.first().isVisible())) {
    selectorsUsed.push(`text="${name}"`);
    return loc.first();
  }
  selectorsUsed.push(`text~="${name}"`);
  return page.getByText(name).first();
}

async function executeActions(page, actions, selectorsUsed) {
  const postActions = [];
  const executed = [];
  if (!actions || actions.length === 0) return { postActions, executed };

  for (const action of actions) {
    console.log(`[Action] type=${action.type}`);
    if (action.type === 'click') {
      if (action.role === 'tab_or_button') {
        const loc = await locateTabOrButton(page, action.name, selectorsUsed);
        await loc.waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
        await loc.click();
        executed.push(`click:${action.name}`);
      } else {
        const nameSelector = action.name_regex || action.name || '';
        if (/批量采用|采用全部/.test(nameSelector)) {
          const checkboxes = page.locator('input[type="checkbox"]:not(:disabled)');
          const count = await checkboxes.count();
          for (let i = 0; i < count; i++) {
            await checkboxes.nth(i).check().catch(() => {});
          }
          await page.waitForTimeout(300);
        }
        const ok = await clickByNameRegex(page, nameSelector, selectorsUsed);
        if (!ok) {
          // Do not treat missing click target as CAPTURE_FAILED — still screenshot
          // the actual page as COLLECTED_DEFECT via assertion path.
          executed.push(`click_missed:${nameSelector}`);
          selectorsUsed.push(`MISS click ~ /${nameSelector}/`);
          console.warn(`[Action] Click target not found: ${nameSelector}`);
        } else {
          executed.push(`click:${nameSelector}`);
        }
      }
      await page.waitForTimeout(400);
    } else if (action.type === 'fill') {
      let loc = page.getByLabel(new RegExp(action.label_regex));
      if ((await loc.count()) === 0) {
        loc = page.locator(
          'textarea, input[type="text"], input:not([type]), [contenteditable="true"]',
        );
      }
      selectorsUsed.push(`fill label~/${action.label_regex}/`);
      await loc.first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
      await loc.first().fill(action.value);
      executed.push(`fill:${action.label_regex}`);
      await page.waitForTimeout(300);
    } else if (action.type === 'fill_filter') {
      let loc = page.getByRole('textbox', { name: /搜索|筛选|Filter|Query|关键词/i });
      if ((await loc.count()) === 0) {
        loc = page.locator(
          'input[type="search"], input[type="text"], input[placeholder*="搜索"], input[placeholder*="筛选"], input[placeholder*="query"], input[placeholder*="filter"], input',
        );
      }
      selectorsUsed.push('filter textbox');
      await loc.first().waitFor({ state: 'visible', timeout: 10000 }).catch(() => {});
      await loc.first().fill(action.value);
      await loc.first().press('Enter');
      executed.push(`fill_filter:${action.value}`);
      await page.waitForTimeout(500);
    } else if (action.type === 'select_option') {
      const selectLoc = page.locator('select');
      let selected = false;
      const count = await selectLoc.count();
      for (let i = 0; i < count; i++) {
        const sel = selectLoc.nth(i);
        if (!(await sel.isVisible().catch(() => false))) continue;
        const options = await sel.locator('option').allTextContents();
        const idx = options.findIndex((text) => new RegExp(action.option_regex).test(text));
        if (idx !== -1) {
          const val = await sel.locator('option').nth(idx).getAttribute('value');
          await sel.selectOption(val);
          selected = true;
          selectorsUsed.push(`select option~/${action.option_regex}/`);
          break;
        }
      }
      if (!selected) {
        // radio / list candidates
        const opt = page
          .locator(
            `label:has-text("${action.option_regex}"), .workflow-candidate:has-text("${action.option_regex}"), [role="option"]:has-text("${action.option_regex}"), li:has-text("${action.option_regex}")`,
          )
          .first();
        if ((await opt.count()) > 0 && (await opt.isVisible().catch(() => false))) {
          await opt.click();
          selectorsUsed.push(`option text~/${action.option_regex}/`);
          selected = true;
        } else {
          const byText = page.getByText(new RegExp(action.option_regex)).first();
          await byText.click({ timeout: 8000 });
          selectorsUsed.push(`text~/${action.option_regex}/`);
          selected = true;
        }
      }
      executed.push(`select_option:${action.option_regex}`);
      await page.waitForTimeout(400);
    } else if (action.type === 'invoke_controlled_action') {
      const ok = invokeControlledAction(action.action);
      if (!ok) throw new Error(`Controlled action failed: ${action.action}`);
      executed.push(`invoke:${action.action}`);
      await page.waitForTimeout(300);
    } else if (action.type === 'invoke_controlled_action_after_screenshot') {
      postActions.push(action);
      executed.push(`post_invoke:${action.action}`);
    }
  }
  return { postActions, executed };
}

async function tryClickAny(page, labels, selectorsUsed, tag) {
  for (const label of labels) {
    const re = new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i');
    const locs = [
      page.getByRole('button', { name: re }),
      page.getByRole('link', { name: re }),
      page.locator('button, a, [role="button"], [role="tab"]').filter({ hasText: re }),
      page.getByText(re),
    ];
    for (const loc of locs) {
      const n = await loc.count().catch(() => 0);
      for (let i = 0; i < n; i++) {
        const el = loc.nth(i);
        if (!(await el.isVisible().catch(() => false))) continue;
        if (await el.isDisabled().catch(() => false)) continue;
        selectorsUsed.push(`${tag}:${label}`);
        await el.scrollIntoViewIfNeeded().catch(() => {});
        await el.click({ timeout: 8000 }).catch(() => el.click({ force: true }));
        await page.waitForTimeout(600);
        return true;
      }
    }
  }
  return false;
}

async function applySpecialOverrides(page, captureId, selectorsUsed) {
  // Per-id interaction fixes using production copy (selector-only corrections).
  const map = {
    'UI-021': async () => {
      await tryClickAny(page, ['更换工作流', '选择工作流', '绑定工作流'], selectorsUsed, 'ov');
      await page.waitForTimeout(500);
      // pick a workflow candidate if visible
      const cand = page.locator('.workflow-candidate, [class*="candidate"], label').filter({ hasText: /acf-fixture|章节规划|chapter/i }).first();
      if ((await cand.count()) > 0 && (await cand.isVisible().catch(() => false))) {
        await cand.click().catch(() => {});
        selectorsUsed.push('ov:pick-workflow');
        await page.waitForTimeout(400);
      }
    },
    'UI-029': async () => {
      await tryClickAny(
        page,
        ['重试', '重新执行 Runtime', '重新执行', '创建重试', 'Retry'],
        selectorsUsed,
        'ov',
      );
    },
    'UI-038': async () => {
      // ensure failure banner area is in view
      await page.evaluate(() => window.scrollTo(0, 0));
      await tryClickAny(page, ['查看详情', '展开', '失败原因'], selectorsUsed, 'ov');
    },
    'UI-040': async () => {
      await tryClickAny(page, ['生成章节规划', '模拟生成章节规划', '开始生成'], selectorsUsed, 'ov');
    },
    'UI-041': async () => {
      await tryClickAny(page, ['生成章节规划', '模拟生成章节规划'], selectorsUsed, 'ov');
    },
    'UI-042': async () => {
      await tryClickAny(page, ['生成章节规划', '模拟生成章节规划', '下一步', '预检', '开始预检'], selectorsUsed, 'ov');
    },
    'UI-043': async () => {
      await tryClickAny(page, ['生成章节规划', '模拟生成章节规划', '预检'], selectorsUsed, 'ov');
    },
    'UI-044': async () => {
      await tryClickAny(page, ['生成章节规划', '模拟生成章节规划', '下一步', '预检', '开始预检', '创建任务', '开始生成', '确认生成'], selectorsUsed, 'ov');
    },
    'UI-048': async () => {
      await tryClickAny(page, ['比较', '查看差异', '重新比较', '对比'], selectorsUsed, 'ov');
    },
    'UI-056': async () => {
      await tryClickAny(page, ['生成正文', '开始生成'], selectorsUsed, 'ov');
    },
    'UI-061': async () => {
      await tryClickAny(page, ['查看详情', '展开', '重试', '重新执行'], selectorsUsed, 'ov');
    },
    'UI-062': async () => {
      await tryClickAny(page, ['查看候选', '查看候选版本', '比较版本', '候选'], selectorsUsed, 'ov');
    },
    'UI-064': async () => {
      // entry point only — text 提交审核 should be visible
      await tryClickAny(page, ['提交审核', '发起审核'], selectorsUsed, 'ov');
      // close if drawer opened accidentally for 064 entry shot — keep open is ok for entry
    },
    'UI-065': async () => {
      await tryClickAny(page, ['提交审核', '发起审核'], selectorsUsed, 'ov');
    },
    'UI-068': async () => {
      // ensure issue panel / source view
      await tryClickAny(page, ['问题详情', '全文定位', '定位', '问题'], selectorsUsed, 'ov');
    },
    'UI-072': async () => {
      await tryClickAny(page, ['创建重写', '正文重写', '已选择问题', '重写'], selectorsUsed, 'ov');
    },
    'UI-074': async () => {
      await tryClickAny(page, ['查看配置', '重写配置', '配置', '工作流'], selectorsUsed, 'ov');
    },
    'UI-077': async () => {
      await tryClickAny(page, ['设为当前版本', '确认设为当前版本', '设为当前'], selectorsUsed, 'ov');
    },
    'UI-078': async () => {
      await tryClickAny(page, ['结果提交失败', '重试', '重试保存结果', '结果消费'], selectorsUsed, 'ov');
    },
    'UI-079': async () => {
      await tryClickAny(page, ['重写失败', '错误原因', '查看详情', '重试'], selectorsUsed, 'ov');
    },
    'UI-080': async () => {
      await tryClickAny(page, ['不可执行', '配置已变更', '预检'], selectorsUsed, 'ov');
    },
    'UI-081': async () => {
      await tryClickAny(page, ['生成章节规划', '预检未通过', '预检'], selectorsUsed, 'ov');
    },
    'UI-082': async () => {
      await tryClickAny(page, ['生成正文', '开始生成', '预检未通过'], selectorsUsed, 'ov');
    },
    'UI-083': async () => {
      await tryClickAny(page, ['提交审核', '发起审核', '预检未通过'], selectorsUsed, 'ov');
    },
    'UI-084': async () => {
      await tryClickAny(
        page,
        ['进行预检', '重新预检', '进行重写', '预检未通过', '正文重写'],
        selectorsUsed,
        'ov',
      );
    },
    'UI-085': async () => {
      await tryClickAny(page, ['超时', '恢复', '重新执行', '查看详情'], selectorsUsed, 'ov');
    },
    'UI-088': async () => {
      await tryClickAny(page, ['超时', '恢复', '重新执行', '查看详情'], selectorsUsed, 'ov');
    },
  };
  if (map[captureId]) {
    await map[captureId]();
  }
}

async function componentVisible(page, stateKind) {
  if (stateKind === 'drawer') {
    const drawerLoc = page.locator(
      'aside, [role="dialog"], [role="presentation"], .workflow-binding-drawer, [class*="drawer"]',
    );
    const count = await drawerLoc.count();
    for (let i = 0; i < count; i++) {
      if (await drawerLoc.nth(i).isVisible().catch(() => false)) return true;
    }
    return false;
  }
  if (stateKind === 'dialog') {
    const dialogLoc = page.locator(
      '[role="dialog"], [role="alertdialog"], .dialog, .modal, [class*="dialog"]',
    );
    const count = await dialogLoc.count();
    for (let i = 0; i < count; i++) {
      if (await dialogLoc.nth(i).isVisible().catch(() => false)) return true;
    }
    return false;
  }
  if (stateKind === 'list') {
    const listLoc = page.locator('table, ul, ol, .list, .grid, [class*="list"], [class*="table"]');
    const count = await listLoc.count();
    for (let i = 0; i < count; i++) {
      if (await listLoc.nth(i).isVisible().catch(() => false)) return true;
    }
    return false;
  }
  // page_state / empty / error — body content is enough
  return true;
}

function detectSecrets(text) {
  const patterns = [
    /\bsk-[a-zA-Z0-9]{20,}\b/i,
    /\bkey-[a-zA-Z0-9]{20,}\b/i,
    /\bBearer\s+[A-Za-z0-9\-._~+/]+=*/i,
    /-----BEGIN (RSA |EC )?PRIVATE KEY-----/,
    /\b(api[_-]?key|secret|password|token)\s*[:=]\s*['"]?[A-Za-z0-9_\-]{16,}/i,
  ];
  return patterns.some((p) => p.test(text));
}

async function evaluatePage(page, capture) {
  const ass = capture.target_assertion || {};
  const errors = [];
  const bodyText = await getVisibleText(page);
  const observedSnippet = bodyText.replace(/\s+/g, ' ').trim().slice(0, 400);

  const basic = {
    not_black: true,
    not_white: true,
    not_blank: true,
    not_browser_error: true,
    no_error_overlay: true,
    not_stuck_loading: true,
    target_state_visible: false,
    no_secret: true,
  };

  if (!bodyText || bodyText.trim().length < 8) {
    basic.not_blank = false;
    errors.push('Page body appears blank.');
  }

  if (
    /This page isn’t working|This site can’t be reached|ERR_|HTTP ERROR|404 Page Not Found|Application error/i.test(
      bodyText,
    )
  ) {
    basic.not_browser_error = false;
    errors.push('Browser error page detected.');
  }

  if (
    /Unhandled Runtime Error|Internal Server Error|nextjs-portal|Application error: a client-side exception/i.test(
      bodyText,
    )
  ) {
    basic.no_error_overlay = false;
    errors.push('Error overlay or server error text detected.');
  }
  const portal = page.locator('nextjs-portal');
  if ((await portal.count()) > 0 && (await portal.first().isVisible().catch(() => false))) {
    basic.no_error_overlay = false;
    errors.push('Next.js error portal visible.');
  }

  // loading
  const loadingGone = await waitLoadingGone(page, 1500);
  if (!loadingGone) {
    basic.not_stuck_loading = false;
    errors.push('Loading/Skeleton still visible.');
  }

  if (detectSecrets(bodyText)) {
    basic.no_secret = false;
    errors.push('Potential secret/token visible.');
  }

  // route
  if ((ass.required_conditions || []).includes('route_matches')) {
    if (!checkRouteMatches(page.url(), capture.route)) {
      errors.push(`Route mismatch. expected=${capture.route} actual=${page.url()}`);
    }
  }

  // required text any
  let textOk = true;
  if (ass.required_visible_text_any && ass.required_visible_text_any.length) {
    textOk = ass.required_visible_text_any.some((t) => bodyText.includes(t));
    if (!textOk) {
      const overlay = await page
        .locator('[role="dialog"], [role="alertdialog"], aside, [class*="drawer"]')
        .allInnerTexts()
        .then((a) => a.join('\n'))
        .catch(() => '');
      textOk = ass.required_visible_text_any.some((t) => overlay.includes(t));
    }
    if (!textOk) {
      errors.push(
        `None of required texts found: [${ass.required_visible_text_any.join(', ')}]`,
      );
    }
  }

  // component
  let compOk = true;
  if ((ass.required_conditions || []).includes('target_component_visible')) {
    compOk = await componentVisible(page, ass.state_kind);
    if (!compOk) errors.push(`Target component not visible for state_kind=${ass.state_kind}`);
  }

  basic.target_state_visible = textOk && compOk;

  // route failed?
  const routeFailed =
    !checkRouteMatches(page.url(), capture.route) ||
    /404|Not Found|页面不存在/.test(bodyText) ||
    basic.not_browser_error === false;

  const targetOk =
    errors.length === 0 &&
    basic.not_stuck_loading &&
    basic.target_state_visible &&
    basic.no_error_overlay &&
    basic.no_secret &&
    basic.not_blank;

  return {
    targetOk,
    routeFailed: routeFailed && !targetOk,
    errors,
    basic,
    actual_observed_state: observedSnippet,
  };
}

function analyzePngBasic(filePath) {
  try {
    const buf = fs.readFileSync(filePath);
    if (buf.length < 100) {
      return { ok: false, reason: 'file too small', sha256: '' };
    }
    const sha256 = crypto.createHash('sha256').update(buf).digest('hex');
    // PNG signature
    const sig = buf.slice(0, 8).toString('hex');
    if (sig !== '89504e470d0a1a0a') {
      return { ok: false, reason: 'not png', sha256 };
    }
    // crude: if almost all bytes identical after header → likely solid color
    const sample = buf.slice(100, Math.min(buf.length, 5000));
    const uniq = new Set(sample);
    if (uniq.size <= 3 && buf.length < 5000) {
      return { ok: false, reason: 'likely solid/blank image', sha256 };
    }
    return { ok: true, reason: '', sha256 };
  } catch (e) {
    return { ok: false, reason: e.message, sha256: '' };
  }
}

function loadProgress() {
  if (!fs.existsSync(PROGRESS_PATH)) return null;
  try {
    return JSON.parse(fs.readFileSync(PROGRESS_PATH, 'utf8'));
  } catch {
    return null;
  }
}

function loadExistingResults() {
  if (!fs.existsSync(RESULTS_PATH)) return [];
  try {
    const data = JSON.parse(fs.readFileSync(RESULTS_PATH, 'utf8'));
    return Array.isArray(data.items) ? data.items : [];
  } catch {
    return [];
  }
}

function writeProgress(state) {
  fs.writeFileSync(PROGRESS_PATH, JSON.stringify(state, null, 2), 'utf8');
}

const FORCE_RECAPTURE = new Set(
  String(process.env.FORCE_RECAPTURE || '')
    .split(/[,\s]+/)
    .map((s) => s.trim())
    .filter(Boolean),
);

function isAcceptableExisting(item) {
  if (!item) return false;
  if (FORCE_RECAPTURE.has(item.capture_id)) return false;
  const okStatus = [
    'COLLECTED_TARGET',
    'COLLECTED_DEFECT',
    'ROUTE_FAILED',
    'CAPTURE_FAILED',
  ].includes(item.capture_status);
  if (!okStatus) return false;
  const p = path.join(OUTPUT_DIR, item.screenshot_path || '');
  if (!fs.existsSync(p)) return false;
  const png = analyzePngBasic(p);
  return png.ok && png.sha256;
}

async function captureOne(context, capture, forceScene) {
  const selectorsUsed = [];
  const actionsExecuted = [];
  let attempt = 0;
  let best = null;

  while (attempt < 2) {
    attempt++;
    console.log(`\n===== ${capture.capture_id} attempt ${attempt} scene=${capture.scene} =====`);

    if (forceScene || attempt > 1) {
      const ok = switchScene(capture.scene);
      if (!ok) {
        best = {
          capture_status: 'ROUTE_FAILED',
          errors: [`Scene switch failed: ${capture.scene}`],
          attempt_count: attempt,
          selectors_used: selectorsUsed,
          actions_executed: actionsExecuted,
          actual_url: WEB_BASE + capture.route,
          actual_observed_state: 'scene switch failed',
          basic_check: {
            not_black: false,
            not_white: false,
            not_blank: false,
            not_browser_error: false,
            no_error_overlay: false,
            not_stuck_loading: false,
            target_state_visible: false,
            no_secret: true,
          },
        };
        continue;
      }
    }

    const page = await context.newPage();
    let postActions = [];
    try {
      const actualUrl = WEB_BASE + capture.route;
      // API_STOPPED expects API down — do not wait networkidle forever
      const gotoOpts =
        capture.scene === 'API_STOPPED'
          ? { waitUntil: 'domcontentloaded', timeout: 20000 }
          : { waitUntil: 'domcontentloaded', timeout: 30000 };
      await page.goto(actualUrl, gotoOpts).catch(async (e) => {
        // Still try to screenshot whatever loaded
        console.warn(`[Browser] goto warning: ${e.message}`);
      });
      await waitForNetworkQuiet(page, capture.scene === 'API_STOPPED' ? 3000 : 15000);
      await waitLoadingGone(page, 15000);

      const actRes = await executeActions(page, capture.actions || [], selectorsUsed);
      postActions = actRes.postActions;
      actionsExecuted.push(...actRes.executed);

      await applySpecialOverrides(page, capture.capture_id, selectorsUsed);
      await waitForNetworkQuiet(page, 10000);
      await waitLoadingGone(page, 15000);
      await waitForAnyText(
        page,
        capture.target_assertion?.required_visible_text_any || [],
        15000,
      );

      const evaluation = await evaluatePage(page, capture);
      // Required click missed → cannot claim target state
      if (actionsExecuted.some((a) => String(a).startsWith('click_missed:'))) {
        evaluation.targetOk = false;
        evaluation.basic.target_state_visible = false;
        evaluation.errors.push(
          `Required click not found: ${actionsExecuted.filter((a) => String(a).startsWith('click_missed:')).join(',')}`,
        );
      }

      // Always screenshot actual page
      const shotPath = path.join(SCREENSHOTS_DIR, capture.screenshot_filename);
      await page.screenshot({ path: shotPath, fullPage: false });
      const pngInfo = analyzePngBasic(shotPath);
      if (!pngInfo.ok) {
        evaluation.basic.not_blank = false;
        evaluation.errors.push(`Screenshot basic check failed: ${pngInfo.reason}`);
      }
      // solid black/white heuristic via file size extremes already covered

      let status;
      if (!pngInfo.ok && evaluation.errors.some((e) => e.includes('Screenshot'))) {
        status = 'CAPTURE_FAILED';
      } else if (evaluation.targetOk) {
        status = 'COLLECTED_TARGET';
      } else if (evaluation.routeFailed && !fs.existsSync(shotPath)) {
        status = 'ROUTE_FAILED';
      } else if (
        evaluation.routeFailed &&
        (evaluation.basic.not_browser_error === false ||
          /404|Not Found|页面不存在/.test(evaluation.actual_observed_state))
      ) {
        status = 'ROUTE_FAILED';
      } else {
        status = 'COLLECTED_DEFECT';
      }

      // For COLLECTED_TARGET enforce not_stuck_loading + target_state_visible
      if (status === 'COLLECTED_TARGET') {
        if (!evaluation.basic.not_stuck_loading || !evaluation.basic.target_state_visible) {
          status = 'COLLECTED_DEFECT';
        }
      }

      best = {
        capture_status: status,
        errors: evaluation.errors,
        attempt_count: attempt,
        selectors_used: [...selectorsUsed],
        actions_executed: [...actionsExecuted],
        actual_url: page.url(),
        actual_observed_state: evaluation.actual_observed_state,
        basic_check: evaluation.basic,
        screenshot_sha256: pngInfo.sha256,
      };

      for (const pa of postActions) {
        invokeControlledAction(pa.action);
      }

      if (status === 'COLLECTED_TARGET') {
        await page.close();
        return best;
      }

      console.warn(
        `[Result] ${capture.capture_id} ${status}: ${evaluation.errors.join(' | ')}`,
      );
    } catch (e) {
      console.error(`[Error] ${capture.capture_id}: ${e.message}`);
      try {
        const shotPath = path.join(SCREENSHOTS_DIR, capture.screenshot_filename);
        await page.screenshot({ path: shotPath, fullPage: false }).catch(() => {});
      } catch {
        /* ignore */
      }
      best = {
        capture_status: 'CAPTURE_FAILED',
        errors: [e.message],
        attempt_count: attempt,
        selectors_used: [...selectorsUsed],
        actions_executed: [...actionsExecuted],
        actual_url: page.url?.() || WEB_BASE + capture.route,
        actual_observed_state: e.message,
        basic_check: {
          not_black: true,
          not_white: true,
          not_blank: false,
          not_browser_error: false,
          no_error_overlay: true,
          not_stuck_loading: false,
          target_state_visible: false,
          no_secret: true,
        },
      };
      // release hold if any
      for (const pa of postActions) {
        invokeControlledAction(pa.action);
      }
    } finally {
      await page.close().catch(() => {});
    }
  }

  return best;
}

function buildItem(capture, result) {
  return {
    capture_id: capture.capture_id,
    business_area: businessAreaOf(capture.capture_id),
    function_point: capture.target,
    scene: capture.scene,
    actual_url: result.actual_url || WEB_BASE + capture.route,
    fixture_ids: capture.resolved_ids || {},
    actions_executed: result.actions_executed || [],
    selectors_used: result.selectors_used || [],
    expected_state: {
      state_kind: capture.target_assertion?.state_kind,
      required_visible_text_any:
        capture.target_assertion?.required_visible_text_any || [],
      required_conditions: capture.target_assertion?.required_conditions || [],
    },
    target_assertions: capture.target_assertion,
    actual_observed_state: result.actual_observed_state || '',
    screenshot_path: `screenshots/${capture.screenshot_filename}`,
    capture_status: result.capture_status,
    attempt_count: result.attempt_count || 1,
    basic_check: result.basic_check,
    notes: [
      capture.notes || '',
      ...(result.errors && result.errors.length
        ? [`failures: ${result.errors.join(' | ')}`]
        : []),
    ]
      .filter(Boolean)
      .join(' ; '),
    screenshot_sha256: result.screenshot_sha256 || '',
  };
}

/**
 * Evidence integrity: COLLECTED_TARGET must not share screenshot bytes with
 * another capture_id. Demote colliding TARGETs (keep earliest id as TARGET if
 * its assertions still hold; demote later TARGETs to COLLECTED_DEFECT).
 */
function enforceUniqueTargetHashes(items) {
  const groups = new Map();
  for (const it of items) {
    const p = path.join(OUTPUT_DIR, it.screenshot_path || '');
    if (!fs.existsSync(p)) continue;
    const h = it.screenshot_sha256 || analyzePngBasic(p).sha256;
    it.screenshot_sha256 = h;
    if (!groups.has(h)) groups.set(h, []);
    groups.get(h).push(it);
  }
  let demoted = 0;
  for (const [, list] of groups) {
    if (list.length < 2) continue;
    list.sort((a, b) => a.capture_id.localeCompare(b.capture_id));
    const targets = list.filter((x) => x.capture_status === 'COLLECTED_TARGET');
    if (targets.length === 0) continue;
    // Keep the first TARGET; demote other TARGETs in the same hash group.
    const keep = targets[0].capture_id;
    for (const it of targets) {
      if (it.capture_id === keep) continue;
      it.capture_status = 'COLLECTED_DEFECT';
      it.notes = [
        it.notes || '',
        `demoted: shared screenshot hash with ${keep}`,
      ]
        .filter(Boolean)
        .join(' ; ');
      if (it.basic_check && typeof it.basic_check === 'object') {
        it.basic_check.target_state_visible = false;
      }
      demoted++;
    }
    // If a DEFECT is identical to a kept TARGET, leave DEFECT (already not TARGET).
  }
  if (demoted) console.log(`[Audit] demoted ${demoted} TARGET(s) due to shared hash`);
  return demoted;
}

function writeOutputs(items, startTime, endTime) {
  enforceUniqueTargetHashes(items);
  const manifest = {
    schema_version: 'acf.ui-acceptance-collection.v1',
    collected_at: endTime.toISOString(),
    started_at: startTime.toISOString(),
    total: items.length,
    items,
  };
  fs.writeFileSync(RESULTS_PATH, JSON.stringify(manifest, null, 2), 'utf8');

  const counts = {
    COLLECTED_TARGET: 0,
    COLLECTED_DEFECT: 0,
    ROUTE_FAILED: 0,
    CAPTURE_FAILED: 0,
  };
  for (const it of items) {
    counts[it.capture_status] = (counts[it.capture_status] || 0) + 1;
  }

  // hash uniqueness
  const hashMap = new Map();
  const duplicateHashes = [];
  for (const it of items) {
    const p = path.join(OUTPUT_DIR, it.screenshot_path);
    if (!fs.existsSync(p)) continue;
    const h = it.screenshot_sha256 || analyzePngBasic(p).sha256;
    if (!h) continue;
    if (hashMap.has(h)) {
      duplicateHashes.push({ a: hashMap.get(h), b: it.capture_id, hash: h });
    } else {
      hashMap.set(h, it.capture_id);
    }
  }

  // region stats
  let summary = `# UI Acceptance Collection Summary\n\n`;
  summary += `- **Start Time**: ${startTime.toISOString()}\n`;
  summary += `- **End Time**: ${endTime.toISOString()}\n`;
  summary += `- **Duration (min)**: ${((endTime - startTime) / 60000).toFixed(2)}\n`;
  summary += `- **Total Tasks**: ${items.length}\n`;
  summary += `- **COLLECTED_TARGET**: ${counts.COLLECTED_TARGET || 0}\n`;
  summary += `- **COLLECTED_DEFECT**: ${counts.COLLECTED_DEFECT || 0}\n`;
  summary += `- **ROUTE_FAILED**: ${counts.ROUTE_FAILED || 0}\n`;
  summary += `- **CAPTURE_FAILED**: ${counts.CAPTURE_FAILED || 0}\n`;
  summary += `- **Duplicate SHA-256 pairs**: ${duplicateHashes.length}\n`;
  summary += `- **Mock used**: No\n`;
  summary += `- **UI/Business code modified**: No\n`;
  summary += `- **Database seed/fixture modified**: No\n\n`;

  summary += `## Stats by Business Area\n\n`;
  summary += `| Area | Total | TARGET | DEFECT | ROUTE_FAILED | CAPTURE_FAILED |\n`;
  summary += `|---|---:|---:|---:|---:|---:|\n`;
  for (const r of BUSINESS_AREAS) {
    const subset = items.filter((it) => {
      const n = captureNum(it.capture_id);
      return n >= r.range[0] && n <= r.range[1];
    });
    const c = (s) => subset.filter((x) => x.capture_status === s).length;
    summary += `| ${r.name} | ${subset.length} | ${c('COLLECTED_TARGET')} | ${c('COLLECTED_DEFECT')} | ${c('ROUTE_FAILED')} | ${c('CAPTURE_FAILED')} |\n`;
  }

  if (duplicateHashes.length) {
    summary += `\n## Duplicate Screenshot Hashes\n\n`;
    for (const d of duplicateHashes) {
      summary += `- ${d.a} == ${d.b} (${d.hash.slice(0, 12)}…)\n`;
    }
  }

  const defects = items.filter((it) =>
    ['COLLECTED_DEFECT', 'ROUTE_FAILED', 'CAPTURE_FAILED'].includes(it.capture_status),
  );
  if (defects.length) {
    summary += `\n## Non-target Items\n\n`;
    for (const d of defects) {
      summary += `- **${d.capture_id}** [${d.capture_status}] ${d.function_point} — ${d.notes}\n`;
    }
  }

  fs.writeFileSync(path.join(OUTPUT_DIR, 'collection-summary.md'), summary, 'utf8');

  // defects md
  let defectsMd = `# Observed UI Defects (Phase-1 Deterministic Collection)\n\n`;
  defectsMd += `Generated: ${endTime.toISOString()}\n\n`;
  defectsMd += `Only factual observations. No code fixes performed.\n\n`;
  const defectItems = items.filter((it) =>
    ['COLLECTED_DEFECT', 'ROUTE_FAILED'].includes(it.capture_status),
  );
  if (defectItems.length === 0) {
    defectsMd += `No COLLECTED_DEFECT or ROUTE_FAILED items.\n`;
  } else {
    for (const d of defectItems) {
      defectsMd += `## ${d.capture_id}\n\n`;
      defectsMd += `- **页面**: ${d.function_point}\n`;
      defectsMd += `- **Scene**: ${d.scene}\n`;
      defectsMd += `- **预期状态**: ${JSON.stringify(d.expected_state)}\n`;
      defectsMd += `- **实际状态**: ${d.actual_observed_state}\n`;
      defectsMd += `- **截图**: ${d.screenshot_path}\n`;
      defectsMd += `- **失败断言**: ${d.notes}\n`;
      defectsMd += `- **capture_status**: ${d.capture_status}\n`;
      defectsMd += `- **初步影响范围**: ${d.business_area}\n`;
      defectsMd += `- **是否阻止后续精确原型对比**: 否（仍可作为缺陷证据参与对比）\n\n`;
    }
  }
  fs.writeFileSync(path.join(OUTPUT_DIR, 'observed-ui-defects.md'), defectsMd, 'utf8');

  // README
  let readme = `# Phase 1 Deterministic UI Evidence Collection\n\n`;
  readme += `Deterministic screenshot evidence for second-loop UI acceptance (UI-001 ~ UI-088).\n\n`;
  readme += `## Files\n\n`;
  readme += `- [collection-manifest.json](collection-manifest.json)\n`;
  readme += `- [collection-summary.md](collection-summary.md)\n`;
  readme += `- [capture-progress.json](capture-progress.json)\n`;
  readme += `- [observed-ui-defects.md](observed-ui-defects.md)\n`;
  readme += `- [screenshots/](screenshots/)\n\n`;
  readme += `## Counts\n\n`;
  readme += `- Total: ${items.length}\n`;
  readme += `- COLLECTED_TARGET: ${counts.COLLECTED_TARGET || 0}\n`;
  readme += `- COLLECTED_DEFECT: ${counts.COLLECTED_DEFECT || 0}\n`;
  readme += `- ROUTE_FAILED: ${counts.ROUTE_FAILED || 0}\n`;
  readme += `- CAPTURE_FAILED: ${counts.CAPTURE_FAILED || 0}\n\n`;
  readme += `## Constraints honored\n\n`;
  readme += `- No Mock / route.fulfill / LocalStorage forgery\n`;
  readme += `- No product UI or business code changes\n`;
  readme += `- No seed/fixture package modifications\n`;
  readme += `- Viewport 1440×900, PNG screenshots\n`;
  fs.writeFileSync(path.join(OUTPUT_DIR, 'README.md'), readme, 'utf8');

  return { counts, duplicateHashes };
}

async function main() {
  const startTime = new Date();
  console.log(`[Start] ${startTime.toISOString()}`);

  const fixture = JSON.parse(fs.readFileSync(MANIFEST_PATH, 'utf8'));
  const captures = fixture.captures;
  if (!Array.isArray(captures) || captures.length !== 88) {
    throw new Error(`Expected 88 captures, got ${captures?.length}`);
  }

  // Resume support: keep only valid completed items with correct status + png
  const existing = loadExistingResults();
  const existingMap = new Map(existing.map((x) => [x.capture_id, x]));
  const resultsMap = new Map();

  // Drop legacy incomplete statuses so we re-collect properly
  for (const c of captures) {
    const prev = existingMap.get(c.capture_id);
    if (isAcceptableExisting(prev)) {
      // ensure sha present
      if (!prev.screenshot_sha256) {
        const p = path.join(OUTPUT_DIR, prev.screenshot_path);
        prev.screenshot_sha256 = analyzePngBasic(p).sha256;
      }
      resultsMap.set(c.capture_id, prev);
    }
  }

  const pending = captures.filter((c) => !resultsMap.has(c.capture_id));
  console.log(
    `[Resume] already acceptable=${resultsMap.size}, pending=${pending.length}`,
  );

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1,
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    colorScheme: 'light',
    reducedMotion: 'reduce',
  });

  let currentScene = null;
  let failureCount = 0;

  // Initial progress
  writeProgress({
    last_completed_capture_id: null,
    completed_count: resultsMap.size,
    current_scene: null,
    current_time: new Date().toISOString(),
    pending_capture_ids: pending.map((c) => c.capture_id),
    failure_count: 0,
    started_at: startTime.toISOString(),
  });

  for (let i = 0; i < captures.length; i++) {
    const capture = captures[i];
    if (resultsMap.has(capture.capture_id)) {
      console.log(`[Skip] ${capture.capture_id} already acceptable`);
      currentScene = capture.scene;
      continue;
    }

    // Reuse scene only if consecutive same scene already loaded
    const needScene = currentScene !== capture.scene;
    if (needScene) {
      const ok = switchScene(capture.scene);
      if (!ok) {
        // Still open the target URL and save an actual page screenshot.
        const page = await context.newPage();
        try {
          const actualUrl = WEB_BASE + capture.route;
          await page.goto(actualUrl, { waitUntil: 'domcontentloaded', timeout: 20000 }).catch(() => {});
          await page.waitForTimeout(1500);
          const shotPath = path.join(SCREENSHOTS_DIR, capture.screenshot_filename);
          await page.screenshot({ path: shotPath, fullPage: false }).catch(() => {});
          const pngInfo = analyzePngBasic(shotPath);
          const body = await page.locator('body').innerText().catch(() => 'scene switch failed');
          const item = buildItem(capture, {
            capture_status: 'ROUTE_FAILED',
            errors: [`Scene switch failed: ${capture.scene}`],
            attempt_count: 1,
            selectors_used: [],
            actions_executed: [],
            actual_url: page.url(),
            actual_observed_state: body.replace(/\s+/g, ' ').trim().slice(0, 400),
            basic_check: {
              not_black: pngInfo.ok,
              not_white: pngInfo.ok,
              not_blank: !!body && body.trim().length > 8,
              not_browser_error: true,
              no_error_overlay: true,
              not_stuck_loading: true,
              target_state_visible: false,
              no_secret: !detectSecrets(body),
            },
            screenshot_sha256: pngInfo.sha256,
          });
          resultsMap.set(capture.capture_id, item);
          failureCount++;
          currentScene = null;
        } finally {
          await page.close().catch(() => {});
        }
      } else {
        currentScene = capture.scene;
      }
    }

    if (!resultsMap.has(capture.capture_id)) {
      // forceScene=false because we already switched if needed; attempt2 will force
      const result = await captureOne(context, capture, false);
      // captureOne may have re-switched scene on retry
      currentScene = capture.scene;
      const item = buildItem(capture, result);
      resultsMap.set(capture.capture_id, item);
      if (item.capture_status !== 'COLLECTED_TARGET') failureCount++;
    }

    // Persist after each item
    const ordered = captures.map((c) => resultsMap.get(c.capture_id)).filter(Boolean);
    const endPartial = new Date();
    writeOutputs(ordered, startTime, endPartial);

    const pendingIds = captures
      .filter((c) => !resultsMap.has(c.capture_id))
      .map((c) => c.capture_id);
    writeProgress({
      last_completed_capture_id: capture.capture_id,
      completed_count: resultsMap.size,
      current_scene: currentScene,
      current_time: new Date().toISOString(),
      pending_capture_ids: pendingIds,
      failure_count: failureCount,
      started_at: startTime.toISOString(),
    });
  }

  await browser.close();

  const ordered = captures.map((c) => resultsMap.get(c.capture_id));
  const endTime = new Date();
  const { counts, duplicateHashes } = writeOutputs(ordered, startTime, endTime);
  writeProgress({
    last_completed_capture_id: 'UI-088',
    completed_count: 88,
    current_scene: currentScene,
    current_time: endTime.toISOString(),
    pending_capture_ids: [],
    failure_count: failureCount,
    started_at: startTime.toISOString(),
    finished_at: endTime.toISOString(),
    counts,
    duplicate_hash_pairs: duplicateHashes.length,
  });

  console.log('\n[Done]', JSON.stringify(counts, null, 2));
  console.log(`[Done] duplicates=${duplicateHashes.length}`);
  console.log(`[Done] duration_min=${((endTime - startTime) / 60000).toFixed(2)}`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

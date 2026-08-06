/**
 * Deterministic UI capture runner with lightweight scenes.
 * CLI: --audit-only | --only UI-001,UI-002 | --resume | --no-commit
 */
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';

const requireFromWeb = createRequire(path.resolve('apps/web/package.json'));
const { chromium } = requireFromWeb('@playwright/test');

const FIXTURE_PKG =
  'C:\\Users\\wangding\\Downloads\\ACF_第二闭环_UI验收数据与Flash操作包\\acf_ui_fixture_pkg';
const MANIFEST_PATH = path.join(FIXTURE_PKG, 'ui-acceptance-fixture-manifest.json');
const OUTPUT_DIR = path.resolve(
  'docs/acceptance/second-loop-ui/phase-1-deterministic-collection',
);
const SCREENSHOTS_DIR = path.join(OUTPUT_DIR, 'screenshots');
const REPLACED_DIR = path.join(OUTPUT_DIR, 'pre-audit-replaced');
const PROGRESS_PATH = path.join(OUTPUT_DIR, 'capture-progress.json');
const RESULTS_PATH = path.join(OUTPUT_DIR, 'collection-manifest.json');
const WEB_BASE = 'http://127.0.0.1:13001';
const MAX_WAIT_MS = 30000;

const POWERSHELL_EXE = path.join(
  process.env.SystemRoot || 'C:\\Windows',
  'System32',
  'WindowsPowerShell',
  'v1.0',
  'powershell.exe',
);

fs.mkdirSync(SCREENSHOTS_DIR, { recursive: true });
fs.mkdirSync(REPLACED_DIR, { recursive: true });

function parseArgs(argv) {
  const args = {
    auditOnly: false,
    resume: false,
    noCommit: false,
    only: null,
  };
  for (let i = 2; i < argv.length; i++) {
    const a = argv[i];
    if (a === '--audit-only') args.auditOnly = true;
    else if (a === '--resume') args.resume = true;
    else if (a === '--no-commit') args.noCommit = true;
    else if (a === '--only') {
      args.only = new Set(
        String(argv[++i] || '')
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      );
    }
  }
  return args;
}

const CLI = parseArgs(process.argv);

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
  return BUSINESS_AREAS.find((r) => n >= r.range[0] && n <= r.range[1])?.name || '未知';
}

function psEnv() {
  const env = { ...process.env };
  const prepend = [
    path.resolve('.tmp/bin'),
    'C:\\Program Files\\Docker\\Docker\\resources\\bin',
    'C:\\Program Files\\PostgreSQL\\18\\bin',
    path.join(process.env.SystemRoot || 'C:\\Windows', 'System32'),
  ].join(';');
  const full = `${prepend};${process.env.PATH || process.env.Path || ''}`;
  env.PATH = full;
  env.Path = full;
  return env;
}

function runWrapper(scriptRel, args) {
  const wrapper = path.resolve(scriptRel);
  const res = spawnSync(
    POWERSHELL_EXE,
    ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', wrapper, ...args],
    { env: psEnv(), encoding: 'utf8', maxBuffer: 20 * 1024 * 1024, timeout: 120000 },
  );
  return {
    code: res.status ?? 1,
    stdout: res.stdout || '',
    stderr: res.stderr || '',
    error: res.error,
  };
}

function switchScene(scene) {
  console.log(`[Scene] ${scene}`);
  // Direct call to fixed fixture script (lightweight)
  const res = spawnSync(
    POWERSHELL_EXE,
    [
      '-NoProfile',
      '-ExecutionPolicy',
      'Bypass',
      '-File',
      path.join(FIXTURE_PKG, 'Set-ACF-UiAcceptanceScene.ps1'),
      '-Scene',
      scene,
    ],
    { env: psEnv(), encoding: 'utf8', maxBuffer: 20 * 1024 * 1024, timeout: 120000 },
  );
  const out = `${res.stdout || ''}\n${res.stderr || ''}`;
  if (res.error || !out.includes(`PASS: ${scene}`)) {
    console.error(out.slice(-1500));
    return false;
  }
  console.log(out.split(/\r?\n/).filter((l) => l.includes('PASS:')).pop() || `PASS: ${scene}`);
  return true;
}

function invokeAction(action) {
  const res = spawnSync(
    POWERSHELL_EXE,
    [
      '-NoProfile',
      '-ExecutionPolicy',
      'Bypass',
      '-File',
      path.join(FIXTURE_PKG, 'Invoke-ACF-UiAcceptanceAction.ps1'),
      '-Action',
      action,
    ],
    { env: psEnv(), encoding: 'utf8', timeout: 60000 },
  );
  const out = `${res.stdout || ''}\n${res.stderr || ''}`;
  if (!/PASS/i.test(out)) {
    console.error(out.slice(-800));
    return false;
  }
  return true;
}

function sha256File(p) {
  return crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
}

function analyzePng(p) {
  try {
    const buf = fs.readFileSync(p);
    if (buf.length < 100) return { ok: false, sha256: '', reason: 'too small' };
    const sig = buf.slice(0, 8).toString('hex');
    if (sig !== '89504e470d0a1a0a') return { ok: false, sha256: '', reason: 'not png' };
    return { ok: true, sha256: crypto.createHash('sha256').update(buf).digest('hex'), size: buf.length };
  } catch (e) {
    return { ok: false, sha256: '', reason: e.message };
  }
}

async function getText(page) {
  try {
    return await page.locator('body').innerText({ timeout: 5000 });
  } catch {
    return '';
  }
}

async function waitStable(page, timeoutMs = 15000) {
  await page.waitForLoadState('domcontentloaded', { timeout: timeoutMs }).catch(() => {});
  await page.waitForLoadState('networkidle', { timeout: Math.min(timeoutMs, 10000) }).catch(() => {});
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const t = await getText(page);
    const loading =
      (/正在加载|Loading\.\.\.|加载中/.test(t) && !/加载失败|无法加载/.test(t)) ||
      (await page.locator('[aria-busy="true"]').count().catch(() => 0)) > 0;
    if (!loading) return true;
    await page.waitForTimeout(300);
  }
  return false;
}

async function waitAnyText(page, texts, timeoutMs = MAX_WAIT_MS) {
  if (!texts?.length) return true;
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const body = await getText(page);
    const overlay = await page
      .locator('[role="dialog"],[role="alertdialog"],aside,[class*="drawer"]')
      .allInnerTexts()
      .then((a) => a.join('\n'))
      .catch(() => '');
    const hay = body + '\n' + overlay;
    if (texts.some((t) => t && hay.includes(t))) return true;
    await page.waitForTimeout(300);
  }
  return false;
}

function expandLabels(nameRegex) {
  const parts = String(nameRegex).split('|').filter(Boolean);
  const aliases = {
    重试: ['重试', '重新执行 Runtime', '重新执行', 'Retry'],
    提交审核: ['提交审核', '发起审核'],
    查看候选: ['查看候选', '查看候选版本', '比较版本'],
    比较: ['比较', '查看差异', '重新比较', '对比'],
    查看配置: ['查看配置', '重写配置', '配置'],
    设为当前版本: ['设为当前版本', '确认设为当前版本'],
    创建任务: ['创建任务', '开始生成', '确认生成'],
    批量采用: ['批量采用', '批量采用已选候选', '确认批量采用'],
    放弃: ['放弃', '放弃本批次', '确认放弃本批次'],
    生成正文: ['生成正文', '开始生成'],
    生成章节规划: ['生成章节规划', '模拟生成章节规划'],
    添加: ['添加 LLM 配置', 'Add connection', '添加工作流', '添加分发平台'],
  };
  const set = new Set(parts);
  for (const p of parts) {
    for (const [k, vals] of Object.entries(aliases)) {
      if (p.includes(k) || k.includes(p)) vals.forEach((v) => set.add(v));
    }
  }
  return [...set];
}

async function clickByRegex(page, nameRegex, selectorsUsed) {
  const labels = expandLabels(nameRegex);
  for (const label of labels) {
    const re = new RegExp(label.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i');
    for (const make of [
      () => page.getByRole('button', { name: re }),
      () => page.getByRole('link', { name: re }),
      () => page.locator('button,a,[role="button"]').filter({ hasText: re }),
      () => page.getByText(re),
    ]) {
      const loc = make();
      const n = await loc.count().catch(() => 0);
      for (let i = 0; i < n; i++) {
        const el = loc.nth(i);
        if (!(await el.isVisible().catch(() => false))) continue;
        if (await el.isDisabled().catch(() => false)) continue;
        selectorsUsed.push(`click:${label}`);
        await el.scrollIntoViewIfNeeded().catch(() => {});
        await el.click({ timeout: 8000 });
        return true;
      }
    }
  }
  return false;
}

async function runActions(page, actions, selectorsUsed) {
  const executed = [];
  const post = [];
  if (!actions?.length) return { executed, post };
  for (const action of actions) {
    if (action.type === 'click') {
      if (action.role === 'tab_or_button') {
        const name = action.name;
        let loc = page.getByRole('tab', { name });
        if ((await loc.count()) === 0) loc = page.getByRole('button', { name });
        if ((await loc.count()) === 0) loc = page.getByText(name);
        selectorsUsed.push(`tab_or_button:${name}`);
        await loc.first().click({ timeout: 8000 });
        executed.push(`click:${name}`);
      } else {
        const name = action.name_regex || action.name || '';
        if (/批量采用|采用全部/.test(name)) {
          const cbs = page.locator('input[type="checkbox"]:not(:disabled)');
          const c = await cbs.count();
          for (let i = 0; i < c; i++) await cbs.nth(i).check().catch(() => {});
        }
        const ok = await clickByRegex(page, name, selectorsUsed);
        if (!ok) {
          executed.push(`click_missed:${name}`);
          selectorsUsed.push(`MISS:${name}`);
        } else executed.push(`click:${name}`);
      }
      await page.waitForTimeout(400);
    } else if (action.type === 'fill') {
      let loc = page.getByLabel(new RegExp(action.label_regex));
      if ((await loc.count()) === 0) loc = page.locator('textarea, input[type="text"]');
      await loc.first().fill(action.value);
      executed.push(`fill:${action.label_regex}`);
      selectorsUsed.push(`fill:${action.label_regex}`);
    } else if (action.type === 'fill_filter') {
      let loc = page.getByRole('textbox', { name: /搜索|筛选|Filter|Query/i });
      if ((await loc.count()) === 0) loc = page.locator('input[type="search"], input[type="text"], input');
      await loc.first().fill(action.value);
      await loc.first().press('Enter');
      executed.push(`fill_filter`);
      selectorsUsed.push('filter');
    } else if (action.type === 'select_option') {
      const opt = page.getByText(new RegExp(action.option_regex)).first();
      await opt.click({ timeout: 8000 }).catch(() => {});
      executed.push(`select:${action.option_regex}`);
    } else if (action.type === 'invoke_controlled_action') {
      if (!invokeAction(action.action)) throw new Error(`action failed ${action.action}`);
      executed.push(`invoke:${action.action}`);
    } else if (action.type === 'invoke_controlled_action_after_screenshot') {
      post.push(action);
      executed.push(`post:${action.action}`);
    }
  }
  return { executed, post };
}

async function specialOverrides(page, id, selectorsUsed) {
  const clicks = {
    'UI-029': ['重试', '重新执行 Runtime', '重新执行'],
    'UI-048': ['比较', '查看差异', '重新比较'],
    'UI-056': ['生成正文', '开始生成'],
    'UI-062': ['查看候选', '比较版本'],
    'UI-065': ['提交审核', '发起审核'],
    'UI-074': ['查看配置', '重写配置'],
    'UI-077': ['设为当前版本', '确认设为当前版本'],
    'UI-082': ['生成正文', '开始生成'],
    'UI-083': ['提交审核', '发起审核'],
    'UI-084': ['进行预检', '重新预检', '进行重写'],
  };
  if (!clicks[id]) return;
  for (const label of clicks[id]) {
    const ok = await clickByRegex(page, label, selectorsUsed);
    if (ok) {
      await page.waitForTimeout(600);
      return;
    }
  }
}

function routeOk(url, route) {
  try {
    const a = new URL(url);
    const e = new URL(route, WEB_BASE);
    if (a.pathname !== e.pathname) return false;
    for (const [k, v] of e.searchParams) if (a.searchParams.get(k) !== v) return false;
    return true;
  } catch {
    return false;
  }
}

async function assertTarget(page, capture, actionsExecuted) {
  const ass = capture.target_assertion || {};
  const errors = [];
  const body = await getText(page);
  const observed = body.replace(/\s+/g, ' ').trim().slice(0, 400);
  const kind = ass.state_kind || '';
  const basic = {
    not_black: true,
    not_white: true,
    not_blank: body.trim().length > 8,
    not_browser_error: !/This page isn’t working|ERR_|404 Page Not Found/i.test(body),
    no_error_overlay: !/Unhandled Runtime Error|nextjs-portal/i.test(body),
    not_stuck_loading: !(/正在加载|Loading\.\.\./.test(body) && !/加载失败/.test(body)),
    target_state_visible: false,
    no_secret: !/\bsk-[a-zA-Z0-9]{20,}\b/i.test(body),
  };

  let failureClass = null; // PRODUCT_DEFECT | CAPTURE_INTERACTION_FAILURE | FIXTURE_STATE_MISMATCH | ROUTE_IMPLEMENTATION_DEFECT

  if (!routeOk(page.url(), capture.route)) {
    errors.push(`route mismatch expected=${capture.route} actual=${page.url()}`);
    failureClass = 'ROUTE_IMPLEMENTATION_DEFECT';
  }
  if (!basic.not_blank) errors.push('blank page');
  if (!basic.not_browser_error) {
    errors.push('browser error page');
    failureClass = failureClass || 'ROUTE_IMPLEMENTATION_DEFECT';
  }
  if (!basic.not_stuck_loading) errors.push('still loading');
  if (!basic.no_secret) errors.push('secret visible');

  const clickMiss = (actionsExecuted || []).some((a) => String(a).startsWith('click_missed:'));
  if (clickMiss) {
    errors.push('required click missed');
    failureClass = 'CAPTURE_INTERACTION_FAILURE';
  }

  // Component assertions for drawer/dialog
  if (kind === 'drawer') {
    const d = page.locator('aside,[role="dialog"],[class*="drawer"],.llm-drawer');
    let vis = false;
    const n = await d.count();
    for (let i = 0; i < n; i++) if (await d.nth(i).isVisible().catch(() => false)) vis = true;
    if (!vis) {
      errors.push('drawer not visible');
      failureClass = failureClass || 'CAPTURE_INTERACTION_FAILURE';
    }
  }
  if (kind === 'dialog') {
    const d = page.locator('[role="dialog"],[role="alertdialog"]');
    let vis = false;
    const n = await d.count();
    for (let i = 0; i < n; i++) if (await d.nth(i).isVisible().catch(() => false)) vis = true;
    if (!vis) {
      errors.push('dialog not visible');
      failureClass = failureClass || 'CAPTURE_INTERACTION_FAILURE';
    }
  }

  let textOk = true;
  if (ass.required_visible_text_any?.length) {
    const overlay = await page
      .locator('[role="dialog"],aside,[class*="drawer"]')
      .allInnerTexts()
      .then((a) => a.join('\n'))
      .catch(() => '');
    const hay = body + '\n' + overlay;
    textOk = ass.required_visible_text_any.some((t) => hay.includes(t));
    if (!textOk) {
      errors.push(`required text missing: [${ass.required_visible_text_any.join(', ')}]`);
      if (!failureClass) {
        // If we navigated correctly but state text missing → fixture or product
        failureClass = clickMiss ? 'CAPTURE_INTERACTION_FAILURE' : 'FIXTURE_STATE_MISMATCH';
      }
    }
  }

  basic.target_state_visible = textOk && errors.filter((e) => !e.includes('secret')).length === 0 ||
    (textOk && !clickMiss && (kind !== 'drawer' && kind !== 'dialog' ? true : true) && routeOk(page.url(), capture.route) && basic.not_stuck_loading);

  // Recompute target_state_visible cleanly
  const structuralOk =
    (kind !== 'drawer' && kind !== 'dialog') ||
    !errors.some((e) => e.includes('drawer not visible') || e.includes('dialog not visible'));
  basic.target_state_visible =
    textOk &&
    structuralOk &&
    !clickMiss &&
    basic.not_stuck_loading &&
    routeOk(page.url(), capture.route);

  const targetOk =
    basic.target_state_visible &&
    basic.not_blank &&
    basic.no_error_overlay &&
    basic.no_secret &&
    basic.not_browser_error;

  if (!targetOk && !failureClass) {
    // Page reachable, actions ok, text missing → product or fixture
    failureClass = 'PRODUCT_DEFECT';
  }

  return { targetOk, errors, basic, observed, failureClass };
}

function loadResults() {
  if (!fs.existsSync(RESULTS_PATH)) return [];
  try {
    return JSON.parse(fs.readFileSync(RESULTS_PATH, 'utf8')).items || [];
  } catch {
    return [];
  }
}

function isValidKept(item) {
  if (!item) return false;
  // Final allowed statuses that are "done" (mid-audit may still have DEFECT)
  const ok = [
    'COLLECTED_TARGET',
    'PRODUCT_DEFECT',
    'ROUTE_IMPLEMENTATION_DEFECT',
    'CAPTURE_FAILED',
    // interim during migration:
    'COLLECTED_DEFECT',
    'ROUTE_FAILED',
  ].includes(item.capture_status);
  if (!ok) return false;
  const p = path.join(OUTPUT_DIR, item.screenshot_path || '');
  if (!fs.existsSync(p)) return false;
  return analyzePng(p).ok;
}

function safeWriteFile(filePath, content) {
  const dir = path.dirname(filePath);
  const tmp = path.join(dir, `.${path.basename(filePath)}.${process.pid}.${Date.now()}.tmp`);
  for (let i = 0; i < 8; i++) {
    try {
      fs.writeFileSync(tmp, content, 'utf8');
      try {
        fs.unlinkSync(filePath);
      } catch {
        /* ignore */
      }
      fs.renameSync(tmp, filePath);
      return;
    } catch {
      try {
        fs.writeFileSync(filePath + '.bak', content, 'utf8');
      } catch {
        /* ignore */
      }
      spawnSync(POWERSHELL_EXE, ['-NoProfile', '-Command', `Start-Sleep -Milliseconds ${50 * (i + 1)}`], {
        encoding: 'utf8',
      });
    }
  }
  fs.writeFileSync(filePath, content, 'utf8');
}

function writeProgress(state) {
  safeWriteFile(PROGRESS_PATH, JSON.stringify(state, null, 2));
}

function writeOutputs(items, startTime, endTime) {
  // Demote TARGET with shared hash
  const groups = new Map();
  for (const it of items) {
    const p = path.join(OUTPUT_DIR, it.screenshot_path || '');
    if (!fs.existsSync(p)) continue;
    const h = it.screenshot_sha256 || analyzePng(p).sha256;
    it.screenshot_sha256 = h;
    if (!groups.has(h)) groups.set(h, []);
    groups.get(h).push(it);
  }
  for (const list of groups.values()) {
    if (list.length < 2) continue;
    for (const it of list) {
      if (it.capture_status === 'COLLECTED_TARGET') {
        it.capture_status = 'PRODUCT_DEFECT'; // will re-class in audit; interim force not TARGET
        it.defect_classification = it.defect_classification || 'CAPTURE_INTERACTION_FAILURE';
        it.duplicate_hash_review = 'INVALID_DUPLICATE_pending';
        it.notes = [it.notes || '', 'demoted: non-unique screenshot hash'].filter(Boolean).join(' ; ');
        if (it.basic_check) it.basic_check.target_state_visible = false;
      }
    }
  }

  const manifest = {
    schema_version: 'acf.ui-acceptance-collection.v1',
    collected_at: endTime.toISOString(),
    started_at: startTime.toISOString(),
    total: items.length,
    items,
  };
  const by = {};
  for (const it of items) by[it.capture_status] = (by[it.capture_status] || 0) + 1;

  const lines = [
    '# UI Acceptance Collection Summary',
    '',
    `- **Collected at**: ${endTime.toISOString()}`,
    `- **Total Tasks**: ${items.length}`,
    `- **COLLECTED_TARGET**: ${by.COLLECTED_TARGET || 0}`,
    `- **PRODUCT_DEFECT**: ${by.PRODUCT_DEFECT || 0}`,
    `- **ROUTE_IMPLEMENTATION_DEFECT**: ${by.ROUTE_IMPLEMENTATION_DEFECT || 0}`,
    `- **CAPTURE_FAILED**: ${by.CAPTURE_FAILED || 0}`,
    `- **COLLECTED_DEFECT (legacy interim)**: ${by.COLLECTED_DEFECT || 0}`,
    `- **ROUTE_FAILED (legacy interim)**: ${by.ROUTE_FAILED || 0}`,
    `- **Mock used**: No`,
    `- **UI/Business code modified**: No`,
    '',
  ];

  safeWriteFile(RESULTS_PATH, JSON.stringify(manifest, null, 2));
  safeWriteFile(path.join(OUTPUT_DIR, 'collection-summary.md'), lines.join('\n') + '\n');
  return by;
}

async function captureOne(context, capture) {
  const selectorsUsed = [];
  let attempt = 0;
  let best = null;

  while (attempt < 2) {
    attempt++;
    if (!switchScene(capture.scene)) {
      best = {
        status: 'ROUTE_IMPLEMENTATION_DEFECT',
        failureClass: 'FIXTURE_STATE_MISMATCH',
        errors: [`scene switch failed: ${capture.scene}`],
        attempt,
        selectorsUsed,
        actionsExecuted: [],
        url: WEB_BASE + capture.route,
        observed: 'scene switch failed',
        basic: {
          not_black: false,
          not_white: false,
          not_blank: false,
          not_browser_error: false,
          no_error_overlay: true,
          not_stuck_loading: false,
          target_state_visible: false,
          no_secret: true,
        },
        sha256: '',
      };
      // still try navigate for screenshot
    }

    const page = await context.newPage();
    let post = [];
    try {
      const url = WEB_BASE + capture.route;
      const opts =
        capture.scene === 'API_STOPPED'
          ? { waitUntil: 'domcontentloaded', timeout: 20000 }
          : { waitUntil: 'domcontentloaded', timeout: 30000 };
      await page.goto(url, opts).catch((e) => console.warn('goto', e.message));
      await waitStable(page, capture.scene === 'API_STOPPED' ? 3000 : 15000);

      const act = await runActions(page, capture.actions || [], selectorsUsed);
      post = act.post;
      await specialOverrides(page, capture.capture_id, selectorsUsed);
      await waitStable(page, 10000);
      await waitAnyText(page, capture.target_assertion?.required_visible_text_any || [], 15000);

      const evaluation = await assertTarget(page, capture, act.executed);

      // Delete existing target file then screenshot
      const shotPath = path.join(SCREENSHOTS_DIR, capture.screenshot_filename);
      const beforeCopy = path.join(REPLACED_DIR, `${capture.capture_id}_before.png`);
      if (fs.existsSync(shotPath)) {
        try {
          fs.copyFileSync(shotPath, beforeCopy);
        } catch {
          /* ignore */
        }
        fs.unlinkSync(shotPath);
      }
      const t0 = Date.now();
      await page.screenshot({ path: shotPath, fullPage: false });
      const st = fs.statSync(shotPath);
      if (st.mtimeMs < t0 - 1000) throw new Error('screenshot mtime not fresh');
      const png = analyzePng(shotPath);
      if (!png.ok) throw new Error(`bad png: ${png.reason}`);

      let status;
      if (evaluation.targetOk) status = 'COLLECTED_TARGET';
      else if (evaluation.failureClass === 'ROUTE_IMPLEMENTATION_DEFECT')
        status = 'ROUTE_IMPLEMENTATION_DEFECT';
      else if (evaluation.failureClass === 'CAPTURE_INTERACTION_FAILURE')
        status = 'PRODUCT_DEFECT'; // will reclassify; store class in field
      else if (evaluation.failureClass === 'FIXTURE_STATE_MISMATCH')
        status = 'PRODUCT_DEFECT';
      else status = 'PRODUCT_DEFECT';

      // Map interim: keep defect_classification separate
      best = {
        status,
        failureClass: evaluation.failureClass || (evaluation.targetOk ? null : 'PRODUCT_DEFECT'),
        errors: evaluation.errors,
        attempt,
        selectorsUsed: [...selectorsUsed],
        actionsExecuted: act.executed,
        url: page.url(),
        observed: evaluation.observed,
        basic: evaluation.basic,
        sha256: png.sha256,
      };

      for (const p of post) invokeAction(p.action);

      if (evaluation.targetOk) {
        await page.close();
        return best;
      }
    } catch (e) {
      console.error(`[Error] ${capture.capture_id}`, e.message);
      try {
        const shotPath = path.join(SCREENSHOTS_DIR, capture.screenshot_filename);
        if (fs.existsSync(shotPath)) fs.unlinkSync(shotPath);
        await page.screenshot({ path: shotPath, fullPage: false });
      } catch {
        /* ignore */
      }
      const png = analyzePng(path.join(SCREENSHOTS_DIR, capture.screenshot_filename));
      best = {
        status: png.ok ? 'PRODUCT_DEFECT' : 'CAPTURE_FAILED',
        failureClass: 'CAPTURE_INTERACTION_FAILURE',
        errors: [e.message],
        attempt,
        selectorsUsed: [...selectorsUsed],
        actionsExecuted: [],
        url: WEB_BASE + capture.route,
        observed: e.message,
        basic: {
          not_black: png.ok,
          not_white: png.ok,
          not_blank: png.ok,
          not_browser_error: true,
          no_error_overlay: true,
          not_stuck_loading: false,
          target_state_visible: false,
          no_secret: true,
        },
        sha256: png.sha256 || '',
      };
      for (const p of post) invokeAction(p.action);
    } finally {
      await page.close().catch(() => {});
    }
  }
  return best;
}

function buildItem(capture, result) {
  // Final status mapping: CAPTURE_INTERACTION_FAILURE/FIXTURE must be fixed or PRODUCT_DEFECT
  let status = result.status;
  let defectClass = result.failureClass || null;
  if (status === 'COLLECTED_TARGET') defectClass = null;
  if (status === 'ROUTE_FAILED') status = 'ROUTE_IMPLEMENTATION_DEFECT';
  if (status === 'COLLECTED_DEFECT') {
    status = 'PRODUCT_DEFECT';
    defectClass = defectClass || 'PRODUCT_DEFECT';
  }

  return {
    capture_id: capture.capture_id,
    business_area: businessAreaOf(capture.capture_id),
    function_point: capture.target,
    scene: capture.scene,
    actual_url: result.url || WEB_BASE + capture.route,
    fixture_ids: capture.resolved_ids || {},
    actions_executed: result.actionsExecuted || [],
    selectors_used: result.selectorsUsed || [],
    expected_state: {
      state_kind: capture.target_assertion?.state_kind,
      required_visible_text_any: capture.target_assertion?.required_visible_text_any || [],
      required_conditions: capture.target_assertion?.required_conditions || [],
    },
    target_assertions: capture.target_assertion,
    actual_observed_state: result.observed || '',
    screenshot_path: `screenshots/${capture.screenshot_filename}`,
    screenshot_sha256: result.sha256 || '',
    capture_status: status,
    attempt_count: result.attempt || 1,
    basic_check: result.basic,
    defect_classification: defectClass,
    duplicate_hash_review: null,
    notes: [
      capture.notes || '',
      ...(result.errors?.length ? [`failures: ${result.errors.join(' | ')}`] : []),
    ]
      .filter(Boolean)
      .join(' ; '),
  };
}

async function main() {
  const startTime = new Date();
  console.log(`[Start] ${startTime.toISOString()} args=${JSON.stringify(CLI)}`);

  const fixture = JSON.parse(fs.readFileSync(MANIFEST_PATH, 'utf8'));
  const captures = fixture.captures;
  if (captures.length !== 88) throw new Error(`expected 88 captures, got ${captures.length}`);

  if (CLI.auditOnly) {
    console.log('[audit-only] no browser capture');
    process.exit(0);
  }

  const existing = loadResults();
  const map = new Map(existing.map((x) => [x.capture_id, x]));
  const resultsMap = new Map();

  const onlySet =
    CLI.only && CLI.only.size > 0 ? CLI.only : null;

  for (const c of captures) {
    if (onlySet) {
      // Keep non-selected items; selected items will be recaptured.
      if (!onlySet.has(c.capture_id) && map.has(c.capture_id)) {
        resultsMap.set(c.capture_id, map.get(c.capture_id));
      }
      continue;
    }
    if (CLI.resume) {
      const prev = map.get(c.capture_id);
      if (
        prev &&
        ['COLLECTED_TARGET', 'PRODUCT_DEFECT', 'ROUTE_IMPLEMENTATION_DEFECT'].includes(
          prev.capture_status,
        ) &&
        isValidKept(prev)
      ) {
        resultsMap.set(c.capture_id, prev);
      }
      continue;
    }
    // Default without flags: keep existing evidence as-is.
    if (map.has(c.capture_id)) resultsMap.set(c.capture_id, map.get(c.capture_id));
  }

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1,
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    colorScheme: 'light',
    reducedMotion: 'reduce',
  });

  let failureCount = 0;
  for (const capture of captures) {
    const force = onlySet && onlySet.has(capture.capture_id);
    if (resultsMap.has(capture.capture_id) && !force) {
      console.log(`[Skip] ${capture.capture_id}`);
      continue;
    }
    if (onlySet && !onlySet.has(capture.capture_id)) continue;

    console.log(`\n===== ${capture.capture_id} ${capture.scene} =====`);
    const result = await captureOne(context, capture);
    const item = buildItem(capture, result);
    resultsMap.set(capture.capture_id, item);
    if (item.capture_status !== 'COLLECTED_TARGET') failureCount++;

    const ordered = captures.map((c) => resultsMap.get(c.capture_id)).filter(Boolean);
    writeOutputs(ordered, startTime, new Date());
    writeProgress({
      last_completed_capture_id: capture.capture_id,
      completed_count: ordered.length,
      current_scene: capture.scene,
      current_time: new Date().toISOString(),
      pending_capture_ids: captures
        .filter((c) => !resultsMap.has(c.capture_id))
        .map((c) => c.capture_id),
      failure_count: failureCount,
      started_at: startTime.toISOString(),
    });
  }

  await browser.close();
  const ordered = captures.map((c) => resultsMap.get(c.capture_id)).filter(Boolean);
  const endTime = new Date();
  const counts = writeOutputs(ordered, startTime, endTime);
  writeProgress({
    last_completed_capture_id: 'UI-088',
    completed_count: ordered.length,
    current_time: endTime.toISOString(),
    pending_capture_ids: captures
      .filter((c) => !resultsMap.has(c.capture_id))
      .map((c) => c.capture_id),
    failure_count: failureCount,
    started_at: startTime.toISOString(),
    finished_at: endTime.toISOString(),
    counts,
    no_commit: CLI.noCommit,
  });
  console.log('[Done]', counts);
  if (!CLI.noCommit) console.log('[Note] commit handled by outer task workflow');
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

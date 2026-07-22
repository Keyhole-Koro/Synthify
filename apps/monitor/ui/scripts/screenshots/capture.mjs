// monitor のダッシュボード画面を Playwright でスクリーンショットする。
//
// - 認証は AuthGate の dev 限定プレビューフラグ (NEXT_PUBLIC_MONITOR_PREVIEW=1)
//   で素通りさせる (本番ビルドでは効かない)。
// - 画面データは実 DB ではなく fixtures/*.json をネットワークインターセプトで返す。
//
// 使い方: `bun run screenshots` (next dev を別で起動しておくか、runner が起動する)。
// 単体実行: BASE_URL=http://127.0.0.1:5174 node scripts/screenshots/capture.mjs

import { chromium } from 'playwright-core';
import { readFileSync } from 'node:fs';
import { mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(__dirname, '..', '..', '..', '..', '..');
const fixturesDir = join(__dirname, 'fixtures');
const outDir = join(repoRoot, 'docs', 'monitor-screenshots');

const BASE_URL = process.env.BASE_URL ?? 'http://127.0.0.1:5174';
const EXECUTABLE_PATH = process.env.CHROMIUM_PATH ?? chromium.executablePath();

function fixture(name) {
  return JSON.parse(readFileSync(join(fixturesDir, name), 'utf8'));
}

const JOB_HEALTH = fixture('job-health.json');
const COST = fixture('cost.json');
const WORKSPACE = fixture('workspace.json');
const ERRORS = fixture('errors.json');
const EVAL = fixture('eval.json');
const EVAL_TRACE = fixture('eval-trace.json');
const JOBS = fixture('jobs.json');
const LOGS = fixture('logs.json');

async function installMocks(page) {
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    const json = (body) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });

    if (path.endsWith('/api/dashboards/job-health')) return json(JOB_HEALTH);
    if (path.endsWith('/api/dashboards/cost')) return json(COST);
    if (path.endsWith('/api/dashboards/workspace')) return json(WORKSPACE);
    if (path.endsWith('/api/dashboards/errors')) return json(ERRORS);
    if (path.endsWith('/api/dashboards/eval/trace')) return json(EVAL_TRACE);
    if (path.endsWith('/api/dashboards/eval')) return json(EVAL);
    if (path.endsWith('/api/jobs')) return json(JOBS);
    if (/\/api\/jobs\/[^/]+\/logs$/.test(path)) return json(LOGS);
    if (path.endsWith('/api/jobs/search')) return json(LOGS);
    if (path.endsWith('/api/jobs/related')) return json({ groups: [], nextPageToken: '' });
    if (path.endsWith('/api/jobs/trace')) return json({ jobId: '', toolCalls: [] });
    if (path.endsWith('/api/auth/me')) return json({ uid: 'preview', email: 'preview@example.com', admin: true });
    return json({});
  });
}

async function hideDevTools(page) {
  await page.addStyleTag({
    content: `nextjs-portal, [data-nextjs-dev-tools-button], #__next-dev-tools-indicator { display: none !important; }`,
  });
}

async function shot(page, name, mode = 'clip') {
  mkdirSync(outDir, { recursive: true });
  const width = page.viewportSize()?.width ?? 1440;

  if (mode === 'viewport') {
    await page.screenshot({ path: join(outDir, `${name}.png`) });
    console.log(`  ✓ ${name}.png`);
    return;
  }

  await page.waitForTimeout(1600);

  const clipHeight = await page.evaluate(() => {
    // Clip to the LAST child's bottom, not the first: the eval page renders its
    // sections as a fragment, so firstElementChild is only the first section
    // (too short) while the container's flex-stretched scrollHeight is the full
    // viewport (too tall, trailing whitespace). lastElementChild.bottom is the
    // real content end for both the fragment page and the single-wrapper tabs.
    const el = document.querySelector('div.overflow-y-auto');
    const last = el?.lastElementChild;
    if (!last) return null;
    return Math.ceil(last.getBoundingClientRect().bottom + 24);
  });
  if (clipHeight) {
    await page.screenshot({
      path: join(outDir, `${name}.png`),
      clip: { x: 0, y: 0, width, height: clipHeight },
    });
  } else {
    await page.screenshot({ path: join(outDir, `${name}.png`), fullPage: true });
  }
  console.log(`  ✓ ${name}.png`);
}

async function clickTab(page, label) {
  await page.getByRole('button', { name: label, exact: true }).first().click();
}

async function main() {
  const browser = await chromium.launch({ executablePath: EXECUTABLE_PATH });
  const page = await browser.newPage({ viewport: { width: 1440, height: 2200 } });
  await installMocks(page);

  console.log(`→ ${BASE_URL}`);
  await page.goto(BASE_URL, { waitUntil: 'networkidle' });
  await hideDevTools(page);

  await page.getByText('Overview').first().waitFor({ timeout: 15000 });
  await shot(page, 'job-health');

  await clickTab(page, 'Cost & Usage');
  await page.getByText('Daily Cost Trend').first().waitFor({ timeout: 15000 });
  await shot(page, 'cost');

  await clickTab(page, 'Workspace Activity');
  await page.getByText('Documents Added').first().waitFor({ timeout: 15000 });
  await shot(page, 'workspace');

  await clickTab(page, 'Errors & Alerts');
  await page.getByText('Errors by Day').first().waitFor({ timeout: 15000 });
  await shot(page, 'errors');

  await clickTab(page, 'Logs');
  await page.setViewportSize({ width: 1440, height: 940 });
  await page.locator('button:has-text("job_")').first().click().catch(() => {});
  await page.waitForTimeout(1000);
  await shot(page, 'logs', 'viewport');

  await page.setViewportSize({ width: 1440, height: 4200 });
  await page.goto(`${BASE_URL}/dashboards/eval`, { waitUntil: 'networkidle' });
  await hideDevTools(page);
  await page.getByText('LLM Eval Execution Trace').waitFor({ timeout: 15000 });
  await page.getByText('Execution trace', { exact: true }).waitFor({ timeout: 15000 });
  await shot(page, 'eval');

  await browser.close();
  console.log(`\nSaved to ${outDir}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});

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
const EXECUTABLE_PATH = process.env.CHROMIUM_PATH ?? '/opt/pw-browsers/chromium';

function fixture(name) {
  return JSON.parse(readFileSync(join(fixturesDir, name), 'utf8'));
}

const JOB_HEALTH = fixture('job-health.json');
const COST = fixture('cost.json');
const WORKSPACE = fixture('workspace.json');
const ERRORS = fixture('errors.json');
const EVAL = fixture('eval.json');
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

// mode 'clip': ダッシュボードタブ。背の高いビューポートで全セクションを描画し、
//   最上位コンテンツの実高さに合わせてクリップ (余白なしのタイトな画像)。
// mode 'viewport': Logs タブのような固定 master-detail レイアウト。ビューポート
//   そのままを撮る。
async function shot(page, name, mode = 'clip') {
  mkdirSync(outDir, { recursive: true });
  const width = page.viewportSize()?.width ?? 1440;

  if (mode === 'viewport') {
    await page.screenshot({ path: join(outDir, `${name}.png`) });
    console.log(`  ✓ ${name}.png`);
    return;
  }

  // recharts の ResponsiveContainer の再計測とマウントアニメーションが終わるのを待つ。
  // (待たずに撮ると line/bar が空で写ることがある)
  await page.waitForTimeout(1600);

  const clipHeight = await page.evaluate(() => {
    const el = document.querySelector('div.overflow-y-auto');
    const inner = el?.firstElementChild;
    if (!inner) return null;
    const rect = inner.getBoundingClientRect();
    return Math.ceil(rect.bottom + 24);
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
  // ダッシュボードは h-screen + 内部スクロールなので、背の高いビューポートにして
  // 各タブの全セクションが 1 枚に収まるようにする。
  const page = await browser.newPage({ viewport: { width: 1440, height: 2200 } });
  await installMocks(page);

  console.log(`→ ${BASE_URL}`);
  await page.goto(BASE_URL, { waitUntil: 'networkidle' });
  await hideDevTools(page);

  // 既定は Job Health タブ。
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
  // Logs は固定 master-detail レイアウトなのでビューポートを実サイズに戻して撮る。
  await page.setViewportSize({ width: 1440, height: 940 });
  // 最初のジョブを選択してログビューを表示。
  await page.locator('button:has-text("job_")').first().click().catch(() => {});
  await page.waitForTimeout(1000);
  await shot(page, 'logs', 'viewport');

  // Eval は独立ルート。既存の operations タブを壊さず、同じ screenshot harness で撮る。
  await page.setViewportSize({ width: 1440, height: 4200 });
  await page.goto(`${BASE_URL}/dashboards/eval`, { waitUntil: 'networkidle' });
  await hideDevTools(page);
  await page.getByText('LLM Eval Monitor').waitFor({ timeout: 15000 });
  await page.getByText('Pass Rate Trend').waitFor({ timeout: 15000 });
  await shot(page, 'eval');

  await browser.close();
  console.log(`\nSaved to ${outDir}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});

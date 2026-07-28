import { expect, test } from './fixtures/test';
import type { Page } from '@playwright/test';
import {
  attachPerfReport,
  diffRuntimeStats,
  formatPerfTable,
  installPerfObservers,
  readLongTasks,
  readRuntimeStats,
  resetLongTasks,
  startCdp,
  type PerfSample,
} from './helpers/perf';

// L1 client load test: hand the client N items and measure what the browser
// does with them. Tagged @perf so it is excluded from the normal e2e run —
// these tests are slow by construction and their numbers are only meaningful
// on a quiet machine. Run with `bun run e2e:perf`.
//
// The tree is injected through __synthifyDebug.createMockWorkspace, which
// builds it entirely in the frontend. That is deliberate: it isolates the
// client cost of holding many items from API latency and worker throughput.
// The API-side cost of actually producing such a tree is L2 — see
// docs/improvements/client-item-load-testing.md.

interface Scenario {
  label: string;
  totalItems: number;
  depth: number;
  branching: number;
  contentBytes: number;
}

// SCALES sweeps item count at a fixed, realistic content size and a single
// open level. This is the "how far does it stretch" axis.
const SCALES: Scenario[] = [
  { label: 'scale', totalItems: 100, depth: 3, branching: 5, contentBytes: 2048 },
  { label: 'scale', totalItems: 500, depth: 4, branching: 5, contentBytes: 2048 },
  { label: 'scale', totalItems: 2_000, depth: 5, branching: 6, contentBytes: 2048 },
  { label: 'scale', totalItems: 5_000, depth: 5, branching: 7, contentBytes: 2048 },
];

// OPEN_COUNTS holds the item count fixed and varies how many papers are
// actually opened. Item count alone does not predict render cost — a closed
// paper is a header, an open one carries a content iframe.
//
// Papers are opened by clicking the child links in the workspace's cover
// report, because that is the only thing that opens them: paper-in-paper owns
// the canvas's open state and it changes through OPEN_NODE dispatches. Writing
// the app's own ExpansionMap does not open anything after the canvas has
// mounted, so an "openDepth" knob on the injection call would have measured
// nothing.
const OPEN_COUNTS = [0, 1, 3, 6];
const OPEN_COUNT_SCENARIO: Scenario = {
  label: 'open-count', totalItems: 2_000, depth: 5, branching: 6, contentBytes: 2048,
};

const REPROJECT_ITERATIONS = 7;

declare global {
  interface Window {
    __synthifyDebug?: {
      createMockWorkspace: (args: Record<string, unknown>) => {
        workspace: { workspaceId: string };
        result: { paperCount: number; injectMs: number; projectMs: number };
      };
      reprojectWorkspace: (workspaceId: string, iterations?: number) => { medianMs: number };
    };
  }
}

async function waitForApp(page: Page) {
  // The debug API is registered by useLandingPageController's effect, so its
  // presence also means the landing page has hydrated.
  await page.waitForFunction(() => window.__synthifyDebug !== undefined, null, { timeout: 60_000 });
}

async function openApp(page: Page) {
  // addInitScript accumulates, so the observers are installed once per page —
  // not on every reload.
  await installPerfObservers(page);
  await page.goto('/');
  await waitForApp(page);
}

// resetApp gives the next scenario a clean page. Without it each scenario would
// stack another mock workspace onto the canvas: the heap and DOM deltas would
// include every earlier scale, and the root-content locator would match one
// iframe per workspace. Mock trees live only in memory, so a reload clears
// them; localStorage is cleared too because the persisted expansion state
// would otherwise carry over. Firebase Auth persists in IndexedDB, so the
// session survives.
async function resetApp(page: Page) {
  await page.evaluate(() => {
    localStorage.clear();
    sessionStorage.clear();
  });
  await page.reload();
  await waitForApp(page);
}

// openPapers opens `count` child papers by clicking their links inside the
// workspace cover report, and waits until each one's content frame exists.
// This is the real expansion path: each click runs an OPEN_NODE dispatch and,
// in the app, a loadSubtree + full re-projection.
async function openPapers(page: Page, count: number): Promise<void> {
  if (count === 0) return;
  const links = page.frameLocator('iframe[title="workspace-root-content"]').locator('a[data-paper-id]');
  const available = await links.count();
  const target = Math.min(count, available);
  for (let index = 0; index < target; index++) {
    await links.nth(index).click();
    await expect(page.locator('iframe[title^="paper-content-"]')).toHaveCount(index + 1, { timeout: 60_000 });
  }
}

async function measure(page: Page, scenario: Scenario, openCount = 0): Promise<PerfSample> {
  await resetApp(page);
  const cdp = await startCdp(page);
  const before = await readRuntimeStats(cdp);
  await resetLongTasks(page);

  const startedAt = Date.now();
  const injected = await page.evaluate((spec) => {
    const { result, workspace } = window.__synthifyDebug!.createMockWorkspace(spec);
    return { workspaceId: workspace.workspaceId, ...result };
  }, {
    workspaceName: `perf ${scenario.totalItems} items`,
    totalItems: scenario.totalItems,
    depth: scenario.depth,
    branching: scenario.branching,
    contentBytes: scenario.contentBytes,
    seed: 7,
  });

  // Wait for the tree to be on screen, not merely in state: the root node's
  // content iframe is the last thing to appear, so it marks the end of the
  // commit + layout + paint the user actually waits through.
  await expect(page.locator('iframe[title="workspace-root-content"]')).toBeVisible({ timeout: 120_000 });
  const renderMs = Date.now() - startedAt;

  // Opening happens after renderMs is taken: the click loop's wall clock is
  // dominated by driver round-trips and locator polling, so folding it into
  // renderMs would report Playwright's overhead as the app's. The cost of
  // opening papers shows up in the long-task, DOM, and heap columns instead,
  // which are measured after this returns.
  await openPapers(page, openCount);

  const longTasks = await readLongTasks(page);
  const after = await readRuntimeStats(cdp);
  const openIframes = await page.locator('iframe').count();
  const openPaperFrames = await page.locator('iframe[title^="paper-content-"]').count();
  const { medianMs } = await page.evaluate(
    ({ workspaceId, iterations }) => window.__synthifyDebug!.reprojectWorkspace(workspaceId, iterations),
    { workspaceId: injected.workspaceId, iterations: REPROJECT_ITERATIONS },
  );
  await cdp.detach();

  return {
    label: scenario.label,
    totalItems: scenario.totalItems,
    openedPapers: openCount,
    contentBytes: scenario.contentBytes,
    injectMs: injected.injectMs,
    projectMs: injected.projectMs,
    renderMs,
    reprojectMedianMs: medianMs,
    paperCount: injected.paperCount,
    openIframes,
    openPaperFrames,
    longTasks,
    runtime: diffRuntimeStats(before, after),
  };
}

test.describe('@perf client load with many items', () => {
  // These tests intentionally push the browser; the default 60s budget is for
  // interaction tests, not for parsing a 5000-item tree on a loaded CI box.
  test.setTimeout(300_000);

  test('scales item count at a fixed open depth', async ({ page }, testInfo) => {
    await openApp(page);
    const samples: PerfSample[] = [];
    for (const scenario of SCALES) {
      samples.push(await measure(page, scenario));
    }
    console.info(`\n${formatPerfTable(samples)}\n`);
    await attachPerfReport(testInfo, samples);

    // The only assertion is that every scale completed and produced a paper
    // per item. Wall-clock thresholds would be a flakiness generator on shared
    // CI hardware; the numbers are the deliverable, and regressions are gated
    // on the L0 bench instead.
    for (const [index, sample] of samples.entries()) {
      expect(sample.paperCount).toBe(SCALES[index].totalItems + 1);
    }
  });

  test('scales how many papers are open at a fixed item count', async ({ page }, testInfo) => {
    await openApp(page);
    const samples: PerfSample[] = [];
    for (const openCount of OPEN_COUNTS) {
      samples.push(await measure(page, OPEN_COUNT_SCENARIO, openCount));
    }
    console.info(`\n${formatPerfTable(samples)}\n`);
    await attachPerfReport(testInfo, samples);

    // Each opened paper must have produced a content frame. Without this the
    // axis silently degrades to "the same measurement four times" — which is
    // exactly what an earlier version of this test did.
    for (const [index, sample] of samples.entries()) {
      expect(sample.openPaperFrames).toBe(OPEN_COUNTS[index]);
    }
  });
});

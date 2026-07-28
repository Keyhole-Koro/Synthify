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
  openDepth: number;
}

// SCALES sweeps item count at a fixed, realistic content size and a single
// open level. This is the "how far does it stretch" axis.
const SCALES: Scenario[] = [
  { label: 'scale', totalItems: 100, depth: 3, branching: 5, contentBytes: 2048, openDepth: 1 },
  { label: 'scale', totalItems: 500, depth: 4, branching: 5, contentBytes: 2048, openDepth: 1 },
  { label: 'scale', totalItems: 2_000, depth: 5, branching: 6, contentBytes: 2048, openDepth: 1 },
  { label: 'scale', totalItems: 5_000, depth: 5, branching: 7, contentBytes: 2048, openDepth: 1 },
];

// OPEN_DEPTHS holds the item count fixed and varies how much of the tree is
// actually rendered. Item count alone does not predict render cost — closed
// papers are cheap, open ones each carry a content iframe.
const OPEN_DEPTHS: Scenario[] = [1, 2, 3].map((openDepth) => ({
  label: 'open-depth', totalItems: 2_000, depth: 5, branching: 6, contentBytes: 2048, openDepth,
}));

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

async function openApp(page: Page) {
  await installPerfObservers(page);
  await page.goto('/');
  // The debug API is registered by useLandingPageController's effect, so its
  // presence also means the landing page has hydrated.
  await page.waitForFunction(() => window.__synthifyDebug !== undefined, null, { timeout: 60_000 });
}

async function measure(page: Page, scenario: Scenario): Promise<PerfSample> {
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
    openDepth: scenario.openDepth,
    seed: 7,
  });

  // Wait for the tree to be on screen, not merely in state: the root node's
  // content iframe is the last thing to appear, so it marks the end of the
  // commit + layout + paint the user actually waits through.
  await expect(page.locator('iframe[title="workspace-root-content"]')).toBeVisible({ timeout: 120_000 });
  const renderMs = Date.now() - startedAt;

  const longTasks = await readLongTasks(page);
  const after = await readRuntimeStats(cdp);
  const openIframes = await page.locator('iframe').count();
  const { medianMs } = await page.evaluate(
    ({ workspaceId, iterations }) => window.__synthifyDebug!.reprojectWorkspace(workspaceId, iterations),
    { workspaceId: injected.workspaceId, iterations: REPROJECT_ITERATIONS },
  );
  await cdp.detach();

  return {
    label: scenario.label,
    totalItems: scenario.totalItems,
    openDepth: scenario.openDepth,
    contentBytes: scenario.contentBytes,
    injectMs: injected.injectMs,
    projectMs: injected.projectMs,
    renderMs,
    reprojectMedianMs: medianMs,
    paperCount: injected.paperCount,
    openIframes,
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

  test('scales how much of the tree is open at a fixed item count', async ({ page }, testInfo) => {
    await openApp(page);
    const samples: PerfSample[] = [];
    for (const scenario of OPEN_DEPTHS) {
      samples.push(await measure(page, scenario));
    }
    console.info(`\n${formatPerfTable(samples)}\n`);
    await attachPerfReport(testInfo, samples);

    // Opening more of the tree must cost more DOM than leaving it closed —
    // if it does not, the mock tree is not actually being expanded and the
    // whole open-depth axis is measuring nothing.
    const [shallow, , deepest] = samples;
    expect(deepest.runtime.domNodes).toBeGreaterThan(shallow.runtime.domNodes);
  });
});

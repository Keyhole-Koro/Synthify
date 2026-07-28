import type { CDPSession, Page, TestInfo } from '@playwright/test';

// L1 of the client load test: what actually happens in a browser when the
// client holds many items. See docs/improvements/client-item-load-testing.md.
//
// The L0 bench (src/features/workspaces/tree/treeLoad.bench.ts) measures the
// data layer in isolation. It cannot see the costs that only exist in a real
// browser: React commits, style/layout, the content iframe per open paper, and
// retained heap. That is what these helpers collect.

export interface LongTaskStats {
  count: number;
  totalMs: number;
  maxMs: number;
}

export interface RuntimeStats {
  jsHeapUsedMb: number;
  domNodes: number;
  // scriptDurationMs / taskDurationMs are cumulative process counters, so they
  // are only meaningful as a delta between two reads.
  scriptDurationMs: number;
  taskDurationMs: number;
  layoutCount: number;
}

export interface PerfSample {
  label: string;
  totalItems: number;
  openDepth: number;
  contentBytes: number;
  // injectMs / projectMs are reported by the app itself (useWorkspaceTree),
  // so they attribute the cost to cache build vs. paper projection.
  injectMs: number;
  projectMs: number;
  // renderMs is wall-clock from the injection call to the workspace paper
  // being on screen — it includes React commit, style, layout, and paint.
  renderMs: number;
  reprojectMedianMs: number;
  paperCount: number;
  openIframes: number;
  longTasks: LongTaskStats;
  runtime: RuntimeStats;
}

// installPerfObservers must run before navigation: PerformanceObserver only
// sees entries recorded after it is registered.
export async function installPerfObservers(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const w = window as unknown as { __perfLongTasks?: { duration: number }[] };
    w.__perfLongTasks = [];
    try {
      new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          w.__perfLongTasks!.push({ duration: entry.duration });
        }
      }).observe({ type: 'longtask', buffered: true });
    } catch {
      // longtask is Chromium-only; the perf project pins Chromium, but a
      // missing observer must not fail the test run.
    }
  });
}

export async function resetLongTasks(page: Page): Promise<void> {
  await page.evaluate(() => {
    (window as unknown as { __perfLongTasks: unknown[] }).__perfLongTasks = [];
  });
}

export async function readLongTasks(page: Page): Promise<LongTaskStats> {
  return page.evaluate(() => {
    const entries = (window as unknown as { __perfLongTasks?: { duration: number }[] }).__perfLongTasks ?? [];
    return {
      count: entries.length,
      totalMs: Math.round(entries.reduce((sum, e) => sum + e.duration, 0)),
      maxMs: Math.round(entries.reduce((max, e) => Math.max(max, e.duration), 0)),
    };
  });
}

export async function startCdp(page: Page): Promise<CDPSession> {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Performance.enable');
  await cdp.send('HeapProfiler.enable');
  return cdp;
}

// readRuntimeStats collects a garbage before reading the heap. Without it the
// number is dominated by whatever the collector has not swept yet, and repeated
// runs disagree by tens of megabytes.
export async function readRuntimeStats(cdp: CDPSession): Promise<RuntimeStats> {
  await cdp.send('HeapProfiler.collectGarbage');
  const { metrics } = await cdp.send('Performance.getMetrics');
  const value = (name: string) => metrics.find((m) => m.name === name)?.value ?? 0;
  return {
    jsHeapUsedMb: Math.round((value('JSHeapUsedSize') / 1_048_576) * 10) / 10,
    domNodes: value('Nodes'),
    scriptDurationMs: Math.round(value('ScriptDuration') * 1000),
    taskDurationMs: Math.round(value('TaskDuration') * 1000),
    layoutCount: value('LayoutCount'),
  };
}

export function diffRuntimeStats(before: RuntimeStats, after: RuntimeStats): RuntimeStats {
  return {
    jsHeapUsedMb: Math.round((after.jsHeapUsedMb - before.jsHeapUsedMb) * 10) / 10,
    domNodes: after.domNodes - before.domNodes,
    scriptDurationMs: after.scriptDurationMs - before.scriptDurationMs,
    taskDurationMs: after.taskDurationMs - before.taskDurationMs,
    layoutCount: after.layoutCount - before.layoutCount,
  };
}

const HEADERS = [
  'scenario', 'items', 'openDepth', 'content B', 'inject ms', 'project ms', 'render ms',
  'reproject ms', 'papers', 'iframes', 'longtasks', 'longtask ms', 'max longtask ms',
  'heap MB', 'DOM nodes',
];

function toRow(sample: PerfSample): (string | number)[] {
  return [
    sample.label, sample.totalItems, sample.openDepth, sample.contentBytes,
    sample.injectMs.toFixed(1), sample.projectMs.toFixed(1), sample.renderMs.toFixed(0),
    sample.reprojectMedianMs.toFixed(2), sample.paperCount, sample.openIframes,
    sample.longTasks.count, sample.longTasks.totalMs, sample.longTasks.maxMs,
    sample.runtime.jsHeapUsedMb, sample.runtime.domNodes,
  ];
}

export function formatPerfTable(samples: PerfSample[]): string {
  const rows = samples.map(toRow);
  const lines = [
    `| ${HEADERS.join(' | ')} |`,
    `| ${HEADERS.map(() => '---').join(' | ')} |`,
    ...rows.map((row) => `| ${row.join(' | ')} |`),
  ];
  return lines.join('\n');
}

export async function attachPerfReport(testInfo: TestInfo, samples: PerfSample[]): Promise<void> {
  await testInfo.attach('perf-samples.json', {
    body: Buffer.from(JSON.stringify(samples, null, 2)),
    contentType: 'application/json',
  });
  await testInfo.attach('perf-table.md', {
    body: Buffer.from(formatPerfTable(samples)),
    contentType: 'text/markdown',
  });
}

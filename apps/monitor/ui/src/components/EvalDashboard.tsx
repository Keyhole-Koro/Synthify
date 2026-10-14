/**
 * LLM Evaluation Dashboard component displaying run traces, span latencies,
 * prompt comparisons, and test case pass rates.
 *
 * @module
 */

import { Link } from '@tanstack/react-router';
import { useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { evalDashboardQuery } from '@/lib/api-queries';
import type { Period } from '@/lib/dashboard-queries';
import type { EvalData } from '@/lib/eval-queries';
import { EvalExecutionTrace } from './eval/EvalExecutionTrace';
import { EvalRunTimeline, EvalVariantComparison } from './eval/EvalDiagrams';

/** Selectable evaluation reporting time windows. */
const PERIODS: { id: Period; label: string }[] = [
  { id: 'today', label: 'Today' },
  { id: '7d', label: 'Past 7 days' },
  { id: '30d', label: 'This month' },
];

/** Default blank evaluation metrics state. */
const EMPTY_EVAL: EvalData = {
  totalRuns: 0,
  totalCases: 0,
  passRate: 0,
  avgRunDurationMs: 0,
  p95CaseDurationMs: 0,
  inputTokens: 0,
  outputTokens: 0,
  funnel: { totalCases: 0, executionCompleted: 0, schemaValid: 0, passed: 0, executionErrors: 0, schemaInvalid: 0, assertionFailures: 0 },
  trend: [], byPromptSource: [], byModel: [], recentRuns: [], recentCases: [], recentFailures: [], slowestCases: [],
};

/**
 * Normalizes partial evaluation API responses into complete EvalData objects.
 *
 * @param data - Raw evaluation metrics response.
 * @returns Fully populated EvalData structure.
 */
function normalize(data: Partial<EvalData> | null | undefined): EvalData {
  return {
    ...EMPTY_EVAL,
    ...data,
    funnel: { ...EMPTY_EVAL.funnel, ...(data?.funnel ?? {}) },
    trend: Array.isArray(data?.trend) ? data.trend : [],
    byPromptSource: Array.isArray(data?.byPromptSource) ? data.byPromptSource : [],
    byModel: Array.isArray(data?.byModel) ? data.byModel : [],
    recentRuns: Array.isArray(data?.recentRuns) ? data.recentRuns : [],
    recentCases: Array.isArray(data?.recentCases) ? data.recentCases : [],
    recentFailures: Array.isArray(data?.recentFailures) ? data.recentFailures : [],
    slowestCases: Array.isArray(data?.slowestCases) ? data.slowestCases : [],
  };
}

/**
 * Formats a duration in milliseconds into a concise time unit string.
 *
 * @param ms - Milliseconds duration.
 * @returns Formatted duration string (e.g., "150ms", "4.2s", "1.5m").
 */
function fmtMs(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60_000).toFixed(1)}m`;
}

/**
 * Formats an integer using compact notation for large numbers.
 *
 * @param value - Integer number.
 * @returns Compactly formatted string.
 */
function fmtNumber(value: number): string {
  return new Intl.NumberFormat('en-US', { notation: value >= 10_000 ? 'compact' : 'standard' }).format(value);
}

/**
 * KPI summary card widget.
 */
function Stat({ label, value, detail }: { label: string; value: string; detail?: string }) {
  return <div className="rounded-lg border border-stone-200 bg-white p-4"><p className="text-[9px] font-bold uppercase tracking-wider text-stone-400">{label}</p><p className="mt-1 text-xl font-bold text-stone-800">{value}</p>{detail && <p className="mt-1 text-[9px] text-stone-400">{detail}</p>}</div>;
}

/**
 * Section container with title and subtext detail.
 */
function Section({ title, detail, children }: { title: string; detail?: string; children: ReactNode }) {
  return <section className="mb-8"><div className="mb-3"><h2 className="text-xs font-bold uppercase tracking-wider text-stone-500">{title}</h2>{detail && <p className="mt-1 text-[10px] text-stone-400">{detail}</p>}</div>{children}</section>;
}

/**
 * LLM Evaluation Dashboard rendering test run timelines, case execution traces,
 * prompt variant metrics, and aggregate pass rates.
 *
 * @returns Evaluation dashboard UI with period selection and drill-down panels.
 */
export function EvalDashboard() {
  const [period, setPeriod] = useState<Period>('7d');
  const query = useQuery({ ...evalDashboardQuery(period), select: normalize });
  const loading = query.isPending;
  const error = query.error ? query.error.message || 'Failed to load' : null;
  const data: EvalData | null = query.isError ? EMPTY_EVAL : (query.data ?? null);

  const trend = (data?.trend ?? []).map((point) => ({ ...point, passRatePct: Number((point.passRate * 100).toFixed(1)) }));
  const llmCalls = data?.recentCases.length ?? 0;
  const failedNodes = data?.recentFailures.length ?? 0;

  return (
    <div className="flex h-screen overflow-hidden bg-stone-50">
      <aside className="flex w-56 shrink-0 flex-col border-r border-stone-200 bg-white">
        <div className="border-b border-stone-100 bg-stone-50/50 p-4"><h1 className="text-sm font-bold uppercase tracking-tight text-stone-800">Dashboards</h1><p className="mt-0.5 text-[10px] text-stone-400">Operations BI</p></div>
        <nav className="flex-1 p-2"><Link to="/dashboards" className="mb-0.5 block rounded-md px-3 py-2 text-xs font-medium text-stone-600 hover:bg-stone-100">Logs & Operations</Link><span className="block rounded-md bg-stone-800 px-3 py-2 text-xs font-medium text-white">LLM Eval Trace</span></nav>
        <div className="border-t border-stone-100 p-3 text-[10px] text-stone-400">Inspect LLM calls, tool calls, validation and assertion failures.</div>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <header className="flex shrink-0 items-center justify-between border-b border-stone-200 bg-white px-6 py-3">
          <div><h1 className="text-sm font-bold text-stone-800">LLM Eval Execution Trace</h1><p className="text-[10px] text-stone-400">Select a run and case, then inspect each execution node.</p></div>
          <div className="flex items-center gap-2"><span className="text-xs text-stone-400">Period:</span>{PERIODS.map((item) => <button key={item.id} onClick={() => setPeriod(item.id)} className={`rounded px-3 py-1 text-xs font-medium ${period === item.id ? 'bg-stone-800 text-white' : 'bg-stone-100 text-stone-600 hover:bg-stone-200'}`}>{item.label}</button>)}</div>
        </header>

        <div className="flex-1 overflow-y-auto p-6">
          {error && <p className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p>}
          {loading || !data ? <p className="py-12 text-center text-sm text-stone-400">Loading eval traces…</p> : <>
            <Section title="Trace summary" detail="Counts reflect persisted case telemetry. Each case records real LLM spans nested under its tool span; select a case below to inspect them.">
              <div className="grid grid-cols-2 gap-3 lg:grid-cols-6">
                <Stat label="Runs" value={fmtNumber(data.totalRuns)} />
                <Stat label="Cases" value={fmtNumber(data.totalCases)} />
                <Stat label="Recent cases" value={fmtNumber(llmCalls)} detail="per-case LLM/tool spans in the trace view" />
                <Stat label="Failed nodes" value={fmtNumber(failedNodes)} />
                <Stat label="p95 case" value={fmtMs(data.p95CaseDurationMs)} />
                <Stat label="Tokens" value={fmtNumber(data.inputTokens + data.outputTokens)} />
              </div>
            </Section>

            <Section title="Execution trace" detail="Tool, LLM, schema validation and assertion nodes are ordered per selected eval case.">
              <EvalExecutionTrace runs={data.recentRuns} cases={data.recentCases} />
            </Section>

            <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
              <Section title="Recent run duration lanes" detail="Supporting run-level context."><EvalRunTimeline runs={data.recentRuns} /></Section>
              <Section title="Prompt variant comparison" detail="Supporting aggregate comparison."><EvalVariantComparison rows={data.byPromptSource} /></Section>
            </div>

            <Section title="Pass-rate trend" detail="Aggregate health remains available below the trace inspector.">
              <div className="rounded-xl border border-stone-200 bg-white p-4"><ResponsiveContainer width="100%" height={240}><LineChart data={trend}><CartesianGrid strokeDasharray="3 3" stroke="#e7e5e4" /><XAxis dataKey="date" tick={{ fontSize: 10 }} /><YAxis domain={[0, 100]} tick={{ fontSize: 10 }} unit="%" /><Tooltip /><Line type="monotone" dataKey="passRatePct" name="Pass rate" stroke="#10b981" strokeWidth={2} dot={{ r: 3 }} /></LineChart></ResponsiveContainer></div>
            </Section>
          </>}
        </div>
      </main>
    </div>
  );
}

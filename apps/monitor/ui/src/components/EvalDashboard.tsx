'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { authFetch } from '@/lib/auth-client';
import type { Period } from '@/lib/dashboard-queries';
import type { EvalData } from '@/lib/eval-queries';
import { EvalExecutionTrace } from './eval/EvalExecutionTrace';
import { EvalRunTimeline, EvalVariantComparison } from './eval/EvalDiagrams';

const PERIODS: { id: Period; label: string }[] = [
  { id: 'today', label: 'Today' },
  { id: '7d', label: 'Past 7 days' },
  { id: '30d', label: 'This month' },
];

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

function fmtMs(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60_000).toFixed(1)}m`;
}

function fmtNumber(value: number): string {
  return new Intl.NumberFormat('en-US', { notation: value >= 10_000 ? 'compact' : 'standard' }).format(value);
}

function Stat({ label, value, detail }: { label: string; value: string; detail?: string }) {
  return <div className="rounded-lg border border-stone-200 bg-white p-4"><p className="text-[9px] font-bold uppercase tracking-wider text-stone-400">{label}</p><p className="mt-1 text-xl font-bold text-stone-800">{value}</p>{detail && <p className="mt-1 text-[9px] text-stone-400">{detail}</p>}</div>;
}

function Section({ title, detail, children }: { title: string; detail?: string; children: React.ReactNode }) {
  return <section className="mb-8"><div className="mb-3"><h2 className="text-xs font-bold uppercase tracking-wider text-stone-500">{title}</h2>{detail && <p className="mt-1 text-[10px] text-stone-400">{detail}</p>}</div>{children}</section>;
}

export function EvalDashboard() {
  const [period, setPeriod] = useState<Period>('7d');
  const [data, setData] = useState<EvalData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    authFetch(`/api/dashboards/eval?period=${period}`)
      .then(async (response) => {
        if (!response.ok) throw new Error((await response.text().catch(() => '')) || `Request failed: ${response.status}`);
        return response.json() as Promise<Partial<EvalData>>;
      })
      .then((body) => { if (!cancelled) setData(normalize(body)); })
      .catch((reason) => {
        console.error(reason);
        if (!cancelled) { setData(EMPTY_EVAL); setError(reason instanceof Error ? reason.message : 'Failed to load'); }
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [period]);

  const trend = (data?.trend ?? []).map((point) => ({ ...point, passRatePct: Number((point.passRate * 100).toFixed(1)) }));
  const llmCalls = data?.recentCases.length ?? 0;
  const failedNodes = data?.recentFailures.length ?? 0;

  return (
    <div className="flex h-screen overflow-hidden bg-stone-50">
      <aside className="flex w-56 shrink-0 flex-col border-r border-stone-200 bg-white">
        <div className="border-b border-stone-100 bg-stone-50/50 p-4"><h1 className="text-sm font-bold uppercase tracking-tight text-stone-800">Dashboards</h1><p className="mt-0.5 text-[10px] text-stone-400">Operations BI</p></div>
        <nav className="flex-1 p-2"><Link href="/dashboards" className="mb-0.5 block rounded-md px-3 py-2 text-xs font-medium text-stone-600 hover:bg-stone-100">Logs & Operations</Link><span className="block rounded-md bg-stone-800 px-3 py-2 text-xs font-medium text-white">LLM Eval Trace</span></nav>
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
            <Section title="Trace summary" detail="Counts reflect persisted case telemetry; dedicated nested spans are added as instrumentation becomes available.">
              <div className="grid grid-cols-2 gap-3 lg:grid-cols-6">
                <Stat label="Runs" value={fmtNumber(data.totalRuns)} />
                <Stat label="Cases" value={fmtNumber(data.totalCases)} />
                <Stat label="Visible LLM calls" value={fmtNumber(llmCalls)} detail="one top-level call per persisted case" />
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

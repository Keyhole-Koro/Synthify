'use client';

import Link from 'next/link';
import { useEffect, useState, type ReactNode } from 'react';
import {
  ResponsiveContainer,
  LineChart,
  Line,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
} from 'recharts';
import { authFetch } from '@/lib/auth-client';
import type { Period } from '@/lib/dashboard-queries';
import type { EvalData, EvalCaseRow } from '@/lib/eval-queries';

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
  trend: [],
  byPromptSource: [],
  byModel: [],
  recentRuns: [],
  recentFailures: [],
  slowestCases: [],
};

function normalize(data: Partial<EvalData> | null | undefined): EvalData {
  return {
    ...EMPTY_EVAL,
    ...data,
    trend: Array.isArray(data?.trend) ? data.trend : [],
    byPromptSource: Array.isArray(data?.byPromptSource) ? data.byPromptSource : [],
    byModel: Array.isArray(data?.byModel) ? data.byModel : [],
    recentRuns: Array.isArray(data?.recentRuns) ? data.recentRuns : [],
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

function fmtDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mb-8">
      <h2 className="mb-3 text-xs font-bold uppercase tracking-wider text-stone-500">{title}</h2>
      {children}
    </section>
  );
}

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="rounded-lg border border-stone-200 bg-white p-4">
      <p className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-stone-400">{label}</p>
      <p className="text-2xl font-bold text-stone-800">{value}</p>
      {sub && <p className="mt-1 text-[11px] text-stone-400">{sub}</p>}
    </div>
  );
}

function CaseTable({ rows, empty }: { rows: EvalCaseRow[]; empty: string }) {
  return (
    <div className="overflow-hidden rounded-lg border border-stone-200 bg-white">
      {rows.length === 0 ? (
        <p className="py-6 text-center text-sm text-stone-400">{empty}</p>
      ) : (
        <table className="w-full text-[11px]">
          <thead>
            <tr className="border-b border-stone-200 bg-stone-50">
              <th className="px-3 py-2 text-left font-semibold text-stone-500">Case</th>
              <th className="px-3 py-2 text-left font-semibold text-stone-500">Tool / model</th>
              <th className="px-3 py-2 text-left font-semibold text-stone-500">Prompt</th>
              <th className="px-3 py-2 text-right font-semibold text-stone-500">Duration</th>
              <th className="px-3 py-2 text-left font-semibold text-stone-500">Error</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, index) => (
              <tr key={`${row.runId}-${row.caseName}-${index}`} className="border-b border-stone-50 align-top">
                <td className="px-3 py-2">
                  <p className="font-medium text-stone-700">{row.caseName}</p>
                  <p className="font-mono text-[9px] text-stone-400">{row.runId.slice(0, 12)}… · {fmtDate(row.createdAt)}</p>
                </td>
                <td className="px-3 py-2 text-stone-500">
                  <p>{row.tool}</p>
                  <p className="font-mono text-[9px] text-stone-400">{row.model}</p>
                </td>
                <td className="px-3 py-2 font-mono text-[10px] text-stone-500">{row.promptSource}</td>
                <td className="px-3 py-2 text-right font-semibold text-stone-700">{fmtMs(row.durationMs)}</td>
                <td className="max-w-xs px-3 py-2 text-red-600">
                  <p className="line-clamp-2">{row.error || '—'}</p>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
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
        if (!cancelled) {
          setData(EMPTY_EVAL);
          setError(reason instanceof Error ? reason.message : 'Failed to load');
        }
      })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [period]);

  const trend = (data?.trend ?? []).map((point) => ({
    ...point,
    passRatePct: Number((point.passRate * 100).toFixed(1)),
  }));

  return (
    <div className="flex h-screen overflow-hidden bg-stone-50">
      <aside className="flex w-56 shrink-0 flex-col border-r border-stone-200 bg-white">
        <div className="border-b border-stone-100 bg-stone-50/50 p-4">
          <h1 className="text-sm font-bold uppercase tracking-tight text-stone-800">Dashboards</h1>
          <p className="mt-0.5 text-[10px] font-medium text-stone-400">Operations BI</p>
        </div>
        <nav className="flex-1 p-2">
          <Link href="/dashboards" className="mb-0.5 block rounded-md px-3 py-2 text-xs font-medium text-stone-600 transition-colors hover:bg-stone-100">
            Logs & Operations
          </Link>
          <span className="mb-0.5 block rounded-md bg-stone-800 px-3 py-2 text-xs font-medium text-white">LLM Eval</span>
        </nav>
        <div className="border-t border-stone-100 p-3 text-[10px] text-stone-400">
          Run, case, latency, token and failure telemetry
        </div>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <header className="flex shrink-0 items-center justify-between border-b border-stone-200 bg-white px-6 py-3">
          <div>
            <h1 className="text-sm font-bold text-stone-800">LLM Eval Monitor</h1>
            <p className="text-[10px] text-stone-400">Postgres-backed eval observability</p>
          </div>
          <div className="flex items-center gap-2">
            <span className="mr-1 text-xs text-stone-400">Period:</span>
            {PERIODS.map((item) => (
              <button
                key={item.id}
                onClick={() => setPeriod(item.id)}
                className={`rounded px-3 py-1 text-xs font-medium transition-colors ${period === item.id ? 'bg-stone-800 text-white' : 'bg-stone-100 text-stone-600 hover:bg-stone-200'}`}
              >
                {item.label}
              </button>
            ))}
          </div>
        </header>

        <div className="flex-1 overflow-y-auto p-6">
          <div>
            {error && <p className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p>}
            {loading || !data ? (
              <p className="py-12 text-center text-sm text-stone-400">Loading eval telemetry…</p>
            ) : (
              <>
                <Section title="Overview">
                  <div className="grid grid-cols-2 gap-3 lg:grid-cols-7">
                    <StatCard label="Runs" value={fmtNumber(data.totalRuns)} />
                    <StatCard label="Cases" value={fmtNumber(data.totalCases)} />
                    <StatCard label="Pass Rate" value={`${(data.passRate * 100).toFixed(1)}%`} />
                    <StatCard label="Avg Run" value={fmtMs(data.avgRunDurationMs)} />
                    <StatCard label="p95 Case" value={fmtMs(data.p95CaseDurationMs)} />
                    <StatCard label="Input Tokens" value={fmtNumber(data.inputTokens)} />
                    <StatCard label="Output Tokens" value={fmtNumber(data.outputTokens)} />
                  </div>
                </Section>

                <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
                  <Section title="Pass Rate Trend">
                    <div className="rounded-lg border border-stone-200 bg-white p-4">
                      <ResponsiveContainer width="100%" height={220}>
                        <LineChart data={trend}>
                          <CartesianGrid strokeDasharray="3 3" stroke="#e7e5e4" />
                          <XAxis dataKey="date" tick={{ fontSize: 10 }} />
                          <YAxis domain={[0, 100]} tick={{ fontSize: 10 }} unit="%" />
                          <Tooltip />
                          <Line type="monotone" dataKey="passRatePct" name="Pass rate" stroke="#10b981" strokeWidth={2} dot={{ r: 3 }} />
                        </LineChart>
                      </ResponsiveContainer>
                    </div>
                  </Section>

                  <Section title="Model Latency (p95)">
                    <div className="rounded-lg border border-stone-200 bg-white p-4">
                      <ResponsiveContainer width="100%" height={220}>
                        <BarChart data={data.byModel} layout="vertical" margin={{ left: 55 }}>
                          <XAxis type="number" tick={{ fontSize: 10 }} />
                          <YAxis type="category" dataKey="model" tick={{ fontSize: 10 }} width={110} />
                          <Tooltip formatter={(value) => fmtMs(Number(value))} />
                          <Bar dataKey="p95DurationMs" name="p95 duration" fill="#f59e0b" />
                        </BarChart>
                      </ResponsiveContainer>
                    </div>
                  </Section>
                </div>

                <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
                  <Section title="Prompt Sources">
                    <div className="overflow-hidden rounded-lg border border-stone-200 bg-white">
                      <table className="w-full text-[11px]">
                        <thead><tr className="border-b border-stone-200 bg-stone-50">
                          <th className="px-3 py-2 text-left font-semibold text-stone-500">Source</th>
                          <th className="px-3 py-2 text-right font-semibold text-stone-500">Runs</th>
                          <th className="px-3 py-2 text-right font-semibold text-stone-500">Cases</th>
                          <th className="px-3 py-2 text-right font-semibold text-stone-500">Pass</th>
                          <th className="px-3 py-2 text-right font-semibold text-stone-500">Avg run</th>
                        </tr></thead>
                        <tbody>{data.byPromptSource.map((row) => (
                          <tr key={row.promptSource} className="border-b border-stone-50">
                            <td className="px-3 py-2 font-mono text-stone-600">{row.promptSource}</td>
                            <td className="px-3 py-2 text-right text-stone-500">{row.runs}</td>
                            <td className="px-3 py-2 text-right text-stone-500">{row.cases}</td>
                            <td className="px-3 py-2 text-right font-semibold text-emerald-600">{(row.passRate * 100).toFixed(1)}%</td>
                            <td className="px-3 py-2 text-right text-stone-500">{fmtMs(row.avgDurationMs)}</td>
                          </tr>
                        ))}</tbody>
                      </table>
                    </div>
                  </Section>

                  <Section title="Models & Tokens">
                    <div className="overflow-hidden rounded-lg border border-stone-200 bg-white">
                      <table className="w-full text-[11px]">
                        <thead><tr className="border-b border-stone-200 bg-stone-50">
                          <th className="px-3 py-2 text-left font-semibold text-stone-500">Model</th>
                          <th className="px-3 py-2 text-right font-semibold text-stone-500">Cases</th>
                          <th className="px-3 py-2 text-right font-semibold text-stone-500">Pass</th>
                          <th className="px-3 py-2 text-right font-semibold text-stone-500">Avg / p95</th>
                          <th className="px-3 py-2 text-right font-semibold text-stone-500">Tokens</th>
                        </tr></thead>
                        <tbody>{data.byModel.map((row) => (
                          <tr key={row.model} className="border-b border-stone-50">
                            <td className="px-3 py-2 font-mono text-stone-600">{row.model}</td>
                            <td className="px-3 py-2 text-right text-stone-500">{row.cases}</td>
                            <td className="px-3 py-2 text-right font-semibold text-emerald-600">{(row.passRate * 100).toFixed(1)}%</td>
                            <td className="px-3 py-2 text-right text-stone-500">{fmtMs(row.avgDurationMs)} / {fmtMs(row.p95DurationMs)}</td>
                            <td className="px-3 py-2 text-right text-stone-500">{fmtNumber(row.inputTokens + row.outputTokens)}</td>
                          </tr>
                        ))}</tbody>
                      </table>
                    </div>
                  </Section>
                </div>

                <Section title="Recent Runs">
                  <div className="overflow-hidden rounded-lg border border-stone-200 bg-white">
                    <table className="w-full text-[11px]">
                      <thead><tr className="border-b border-stone-200 bg-stone-50">
                        <th className="px-3 py-2 text-left font-semibold text-stone-500">Run</th>
                        <th className="px-3 py-2 text-left font-semibold text-stone-500">Prompt / model</th>
                        <th className="px-3 py-2 text-right font-semibold text-stone-500">Result</th>
                        <th className="px-3 py-2 text-right font-semibold text-stone-500">Pass rate</th>
                        <th className="px-3 py-2 text-right font-semibold text-stone-500">Duration</th>
                        <th className="px-3 py-2 text-right font-semibold text-stone-500">Tokens</th>
                        <th className="px-3 py-2 text-left font-semibold text-stone-500">Artifact</th>
                      </tr></thead>
                      <tbody>{data.recentRuns.map((run) => (
                        <tr key={run.runId} className="border-b border-stone-50">
                          <td className="px-3 py-2"><p className="font-mono text-stone-600">{run.runId.slice(0, 12)}…</p><p className="text-[9px] text-stone-400">{fmtDate(run.startedAt)}</p></td>
                          <td className="px-3 py-2"><p className="font-mono text-stone-600">{run.promptSource}</p><p className="text-[9px] text-stone-400">{run.model}</p></td>
                          <td className="px-3 py-2 text-right"><span className={`rounded-full px-2 py-0.5 text-[10px] font-semibold ${run.status === 'succeeded' ? 'bg-emerald-50 text-emerald-700' : 'bg-red-50 text-red-700'}`}>{run.passedCount}/{run.caseCount} passed</span></td>
                          <td className="px-3 py-2 text-right font-semibold text-stone-700">{(run.passRate * 100).toFixed(1)}%</td>
                          <td className="px-3 py-2 text-right text-stone-500">{fmtMs(run.durationMs)}</td>
                          <td className="px-3 py-2 text-right text-stone-500">{fmtNumber(run.inputTokens + run.outputTokens)}</td>
                          <td className="max-w-[220px] truncate px-3 py-2 font-mono text-[9px] text-stone-400" title={run.artifactUri}>{run.artifactUri || '—'}</td>
                        </tr>
                      ))}</tbody>
                    </table>
                  </div>
                </Section>

                <Section title="Recent Failures">
                  <CaseTable rows={data.recentFailures} empty="No failed cases in this period" />
                </Section>

                <Section title="Slowest Cases">
                  <CaseTable rows={data.slowestCases} empty="No case data in this period" />
                </Section>
              </>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}

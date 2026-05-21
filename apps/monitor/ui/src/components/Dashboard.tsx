'use client';

import React, { useEffect, useState, useCallback } from 'react';
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
import type {
  JobHealthData,
  CostData,
  WorkspaceData,
  ErrorsData,
  Period,
} from '@/lib/dashboard-queries';
import { AuditPage } from './AuditPage';

// ── Helpers ───────────────────────────────────────────────────────────────────

function fmtCost(minor: number): string {
  return `$${(minor / 100).toFixed(2)}`;
}

function fmtMs(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function shortDate(iso: string): string {
  return iso.slice(5); // MM-DD
}

async function fetchDashboardData<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const message = await res.text().catch(() => '');
    throw new Error(message || `Request failed: ${res.status}`);
  }
  return res.json() as Promise<T>;
}

const EMPTY_JOB_HEALTH: JobHealthData = {
  totalJobs: 0,
  successRate: 0,
  avgProcessingMs: 0,
  p95ProcessingMs: 0,
  byDay: [],
  stageFailures: [],
  topRetryJobs: [],
  errorMessages: [],
  byMimeType: [],
  activeFailures: [],
};

const EMPTY_COST: CostData = {
  totalCostMinor: 0,
  creditCostMinor: 0,
  stripeCostMinor: 0,
  dailyTrend: [],
  byModel: [],
  topJobs: [],
};

const EMPTY_WORKSPACE: WorkspaceData = {
  totalWorkspaces: 0,
  dormantWorkspaces: 0,
  docsByDay: [],
  itemsByDay: [],
};

const EMPTY_ERRORS: ErrorsData = {
  totalErrors: 0,
  recentBurst: false,
  byDay: [],
  topEvents: [],
};

function normalizeJobHealth(data: Partial<JobHealthData> | null | undefined): JobHealthData {
  return {
    ...EMPTY_JOB_HEALTH,
    ...data,
    byDay: Array.isArray(data?.byDay) ? data.byDay : [],
    stageFailures: Array.isArray(data?.stageFailures) ? data.stageFailures : [],
    topRetryJobs: Array.isArray(data?.topRetryJobs) ? data.topRetryJobs : [],
    errorMessages: Array.isArray(data?.errorMessages) ? data.errorMessages : [],
    byMimeType: Array.isArray(data?.byMimeType) ? data.byMimeType : [],
    activeFailures: Array.isArray(data?.activeFailures) ? data.activeFailures : [],
  };
}

function normalizeCost(data: Partial<CostData> | null | undefined): CostData {
  return {
    ...EMPTY_COST,
    ...data,
    dailyTrend: Array.isArray(data?.dailyTrend) ? data.dailyTrend : [],
    byModel: Array.isArray(data?.byModel) ? data.byModel : [],
    topJobs: Array.isArray(data?.topJobs) ? data.topJobs : [],
  };
}

function normalizeWorkspace(data: Partial<WorkspaceData> | null | undefined): WorkspaceData {
  return {
    ...EMPTY_WORKSPACE,
    ...data,
    docsByDay: Array.isArray(data?.docsByDay) ? data.docsByDay : [],
    itemsByDay: Array.isArray(data?.itemsByDay) ? data.itemsByDay : [],
  };
}

function normalizeErrors(data: Partial<ErrorsData> | null | undefined): ErrorsData {
  return {
    ...EMPTY_ERRORS,
    ...data,
    byDay: Array.isArray(data?.byDay) ? data.byDay : [],
    topEvents: Array.isArray(data?.topEvents) ? data.topEvents : [],
  };
}

// ── Stat Card ─────────────────────────────────────────────────────────────────

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div className="bg-white rounded-lg border border-stone-200 p-4">
      <p className="text-[10px] uppercase tracking-wider text-stone-400 font-semibold mb-1">{label}</p>
      <p className="text-2xl font-bold text-stone-800">{value}</p>
      {sub && <p className="text-[11px] text-stone-400 mt-1">{sub}</p>}
    </div>
  );
}

// ── Section wrapper ───────────────────────────────────────────────────────────

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mb-8">
      <h2 className="text-xs font-bold uppercase tracking-wider text-stone-500 mb-3">{title}</h2>
      {children}
    </div>
  );
}

// ── Job Health Tab ────────────────────────────────────────────────────────────

function JobHealthTab({ period, onDrillDown }: { period: Period; onDrillDown?: (jobId: string) => void }) {
  const [data, setData] = useState<JobHealthData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    fetchDashboardData<Partial<JobHealthData>>(`/api/dashboards/job-health?period=${period}`)
      .then((body) => setData(normalizeJobHealth(body)))
      .catch((err) => {
        console.error(err);
        setError(err instanceof Error ? err.message : 'Failed to load');
        setData(EMPTY_JOB_HEALTH);
      })
      .finally(() => setLoading(false));
  }, [period]);

  if (loading) return <p className="text-sm text-stone-400 py-10 text-center">Loading...</p>;
  if (!data) return <p className="text-sm text-red-400 py-10 text-center">Failed to load</p>;

  const chartLabel = period === 'today' ? 'Jobs by Hour' : 'Jobs by Day';

  return (
    <div>
      {error && <p className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p>}

      <Section title="Overview">
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-6">
          <StatCard label="Total Jobs" value={String(data.totalJobs)} />
          <StatCard label="Success Rate" value={`${(data.successRate * 100).toFixed(1)}%`} />
          <StatCard label="Avg Time" value={fmtMs(data.avgProcessingMs)} />
          <StatCard label="p95 Time" value={fmtMs(data.p95ProcessingMs)} />
        </div>
      </Section>

      {/* Active failures — drilldown to logs */}
      {data.activeFailures.length > 0 && (
        <Section title="Recent Failures">
          <div className="bg-white rounded-lg border border-stone-200 overflow-hidden">
            <table className="w-full text-[11px]">
              <thead>
                <tr className="bg-red-50 border-b border-red-100">
                  <th className="px-3 py-2 text-left font-semibold text-stone-500">Job ID</th>
                  <th className="px-3 py-2 text-left font-semibold text-stone-500">Stage</th>
                  <th className="px-3 py-2 text-left font-semibold text-stone-500">Error</th>
                  <th className="px-3 py-2 text-right font-semibold text-stone-500">Time</th>
                  <th className="px-3 py-2" />
                </tr>
              </thead>
              <tbody>
                {data.activeFailures.map((j) => (
                  <tr key={j.jobId} className="border-b border-stone-50 hover:bg-stone-50">
                    <td className="px-3 py-2 font-mono text-stone-600">{j.jobId.slice(0, 10)}…</td>
                    <td className="px-3 py-2 text-stone-500">{j.currentStage || '—'}</td>
                    <td className="px-3 py-2 text-red-600 max-w-xs truncate">{j.errorMessage || '—'}</td>
                    <td className="px-3 py-2 text-right text-stone-400">
                      {new Date(j.updatedAt).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })}
                    </td>
                    <td className="px-3 py-2 text-right">
                      {onDrillDown && (
                        <button
                          onClick={() => onDrillDown(j.jobId)}
                          className="text-[10px] px-2 py-0.5 rounded bg-stone-100 text-stone-600 hover:bg-stone-200 font-medium transition-colors"
                        >
                          Logs →
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Section>
      )}

      <Section title={chartLabel}>
        <div className="bg-white rounded-lg border border-stone-200 p-4">
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={data.byDay}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e7e5e4" />
              <XAxis dataKey="date" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip />
              <Bar dataKey="succeeded" stackId="a" fill="#10b981" name="Succeeded" />
              <Bar dataKey="failed" stackId="a" fill="#ef4444" name="Failed" />
              <Bar dataKey="running" stackId="a" fill="#f59e0b" name="Running" />
              <Bar dataKey="queued" stackId="a" fill="#cbd5e1" name="Queued" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </Section>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Section title="Stage Failures">
          <div className="bg-white rounded-lg border border-stone-200 p-4">
            {data.stageFailures.length === 0 ? (
              <p className="text-sm text-stone-400 py-4 text-center">No failures</p>
            ) : (
              <ResponsiveContainer width="100%" height={180}>
                <BarChart data={data.stageFailures} layout="vertical" margin={{ left: 80 }}>
                  <XAxis type="number" tick={{ fontSize: 10 }} />
                  <YAxis dataKey="stage" type="category" tick={{ fontSize: 10 }} width={80} />
                  <Tooltip />
                  <Bar dataKey="count" fill="#ef4444" name="Failures" />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </Section>

        <Section title="Failure by File Type">
          <div className="bg-white rounded-lg border border-stone-200 overflow-hidden">
            {data.byMimeType.length === 0 ? (
              <p className="text-sm text-stone-400 py-4 text-center">No data</p>
            ) : (
              <table className="w-full text-[11px]">
                <thead>
                  <tr className="bg-stone-50 border-b border-stone-200">
                    <th className="px-3 py-2 text-left font-semibold text-stone-500">MIME Type</th>
                    <th className="px-3 py-2 text-right font-semibold text-stone-500">Failed</th>
                    <th className="px-3 py-2 text-right font-semibold text-stone-500">Total</th>
                    <th className="px-3 py-2 text-right font-semibold text-stone-500">Rate</th>
                  </tr>
                </thead>
                <tbody>
                  {data.byMimeType.map((m) => (
                    <tr key={m.mimeType} className="border-b border-stone-50">
                      <td className="px-3 py-2 font-mono text-stone-600 text-[10px]">{m.mimeType}</td>
                      <td className="px-3 py-2 text-right font-bold text-red-600">{m.failed}</td>
                      <td className="px-3 py-2 text-right text-stone-500">{m.total}</td>
                      <td className="px-3 py-2 text-right text-stone-500">
                        {m.total > 0 ? `${((m.failed / m.total) * 100).toFixed(0)}%` : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </Section>
      </div>

      <Section title="Top Error Messages">
        <div className="bg-white rounded-lg border border-stone-200 overflow-hidden">
          {data.errorMessages.length === 0 ? (
            <p className="text-sm text-stone-400 py-4 text-center">No errors</p>
          ) : (
            <table className="w-full text-[11px]">
              <thead>
                <tr className="bg-stone-50 border-b border-stone-200">
                  <th className="px-3 py-2 text-left font-semibold text-stone-500">Message</th>
                  <th className="px-3 py-2 text-right font-semibold text-stone-500">Count</th>
                </tr>
              </thead>
              <tbody>
                {data.errorMessages.map((e, i) => (
                  <tr key={i} className="border-b border-stone-50">
                    <td className="px-3 py-2 text-stone-600 max-w-lg truncate font-mono text-[10px]">{e.message}</td>
                    <td className="px-3 py-2 text-right font-bold text-red-600">{e.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </Section>
    </div>
  );
}

// ── Cost & Usage Tab ──────────────────────────────────────────────────────────

function CostTab({ period }: { period: Period }) {
  const [data, setData] = useState<CostData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    fetchDashboardData<Partial<CostData>>(`/api/dashboards/cost?period=${period}`)
      .then((body) => setData(normalizeCost(body)))
      .catch((err) => {
        console.error(err);
        setError(err instanceof Error ? err.message : 'Failed to load');
        setData(EMPTY_COST);
      })
      .finally(() => setLoading(false));
  }, [period]);

  if (loading) return <p className="text-sm text-stone-400 py-10 text-center">Loading...</p>;
  if (!data) return <p className="text-sm text-red-400 py-10 text-center">Failed to load</p>;

  return (
    <div>
      {error && <p className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p>}
      <Section title="Overview">
        <div className="grid grid-cols-2 lg:grid-cols-3 gap-3 mb-6">
          <StatCard label="Total Cost" value={fmtCost(data.totalCostMinor)} />
          <StatCard label="Credit" value={fmtCost(data.creditCostMinor)} />
          <StatCard label="Stripe" value={fmtCost(data.stripeCostMinor)} />
        </div>
      </Section>

      <Section title="Daily Cost Trend">
        <div className="bg-white rounded-lg border border-stone-200 p-4">
          <ResponsiveContainer width="100%" height={200}>
            <LineChart data={data.dailyTrend.map((d) => ({ ...d, date: shortDate(d.date), cost: d.costMinor / 100 }))}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e7e5e4" />
              <XAxis dataKey="date" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} tickFormatter={(v) => `$${v}`} />
              <Tooltip formatter={(v) => [`$${Number(v).toFixed(2)}`, 'Cost']} />
              <Line type="monotone" dataKey="cost" stroke="#6366f1" strokeWidth={2} dot={false} name="Cost (USD)" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </Section>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Section title="Cost by Model">
          <div className="bg-white rounded-lg border border-stone-200 p-4">
            {data.byModel.length === 0 ? (
              <p className="text-sm text-stone-400 py-4 text-center">No data</p>
            ) : (
              <ResponsiveContainer width="100%" height={180}>
                <BarChart data={data.byModel.map((m) => ({ ...m, cost: m.costMinor / 100 }))} layout="vertical" margin={{ left: 100 }}>
                  <XAxis type="number" tick={{ fontSize: 10 }} tickFormatter={(v) => `$${v}`} />
                  <YAxis dataKey="model" type="category" tick={{ fontSize: 9 }} width={100} />
                  <Tooltip formatter={(v) => [`$${Number(v).toFixed(2)}`, 'Cost']} />
                  <Bar dataKey="cost" fill="#6366f1" name="Cost (USD)" />
                </BarChart>
              </ResponsiveContainer>
            )}
          </div>
        </Section>

        <Section title="Top Jobs by Cost">
          <div className="bg-white rounded-lg border border-stone-200 overflow-hidden">
            {data.topJobs.length === 0 ? (
              <p className="text-sm text-stone-400 py-4 text-center">No data</p>
            ) : (
              <table className="w-full text-[11px]">
                <thead>
                  <tr className="bg-stone-50 border-b border-stone-200">
                    <th className="px-3 py-2 text-left font-semibold text-stone-500">Job ID</th>
                    <th className="px-3 py-2 text-right font-semibold text-stone-500">Cost</th>
                  </tr>
                </thead>
                <tbody>
                  {data.topJobs.map((j) => (
                    <tr key={j.jobId} className="border-b border-stone-50">
                      <td className="px-3 py-2 font-mono text-stone-600">{j.jobId.slice(0, 12)}…</td>
                      <td className="px-3 py-2 text-right font-bold text-indigo-600">{fmtCost(j.costMinor)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </Section>
      </div>
    </div>
  );
}

// ── Workspace Activity Tab ────────────────────────────────────────────────────

function WorkspaceTab({ period }: { period: Period }) {
  const [data, setData] = useState<WorkspaceData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    fetchDashboardData<Partial<WorkspaceData>>(`/api/dashboards/workspace?period=${period}`)
      .then((body) => setData(normalizeWorkspace(body)))
      .catch((err) => {
        console.error(err);
        setError(err instanceof Error ? err.message : 'Failed to load');
        setData(EMPTY_WORKSPACE);
      })
      .finally(() => setLoading(false));
  }, [period]);

  if (loading) return <p className="text-sm text-stone-400 py-10 text-center">Loading...</p>;
  if (!data) return <p className="text-sm text-red-400 py-10 text-center">Failed to load</p>;

  return (
    <div>
      {error && <p className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p>}
      <Section title="Overview">
        <div className="grid grid-cols-2 gap-3 mb-6">
          <StatCard label="Total Workspaces" value={String(data.totalWorkspaces)} />
          <StatCard
            label="Dormant (30d)"
            value={String(data.dormantWorkspaces)}
            sub="No activity for 30+ days"
          />
        </div>
      </Section>

      <Section title="Documents Added">
        <div className="bg-white rounded-lg border border-stone-200 p-4">
          <ResponsiveContainer width="100%" height={180}>
            <BarChart data={data.docsByDay.map((d) => ({ ...d, date: shortDate(d.date) }))}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e7e5e4" />
              <XAxis dataKey="date" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip />
              <Bar dataKey="count" fill="#0ea5e9" name="Documents" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </Section>

      <Section title="Tree Items Created">
        <div className="bg-white rounded-lg border border-stone-200 p-4">
          <ResponsiveContainer width="100%" height={180}>
            <LineChart data={data.itemsByDay.map((d) => ({ ...d, date: shortDate(d.date) }))}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e7e5e4" />
              <XAxis dataKey="date" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip />
              <Line type="monotone" dataKey="count" stroke="#10b981" strokeWidth={2} dot={false} name="Items" />
            </LineChart>
          </ResponsiveContainer>
        </div>
      </Section>
    </div>
  );
}

// ── Errors & Alerts Tab ───────────────────────────────────────────────────────

function ErrorsTab({ period }: { period: Period }) {
  const [data, setData] = useState<ErrorsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    fetchDashboardData<Partial<ErrorsData>>(`/api/dashboards/errors?period=${period}`)
      .then((body) => setData(normalizeErrors(body)))
      .catch((err) => {
        console.error(err);
        setError(err instanceof Error ? err.message : 'Failed to load');
        setData(EMPTY_ERRORS);
      })
      .finally(() => setLoading(false));
  }, [period]);

  if (loading) return <p className="text-sm text-stone-400 py-10 text-center">Loading...</p>;
  if (!data) return <p className="text-sm text-red-400 py-10 text-center">Failed to load</p>;

  return (
    <div>
      {error && <p className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">{error}</p>}
      {data.recentBurst && (
        <div className="mb-4 bg-red-50 border border-red-200 rounded-lg px-4 py-3 text-sm font-medium text-red-700">
          Error burst detected — 10+ errors in the last 5 minutes
        </div>
      )}

      <Section title="Overview">
        <div className="grid grid-cols-1 gap-3 mb-6">
          <StatCard label="Total Errors" value={String(data.totalErrors)} sub={`in selected period`} />
        </div>
      </Section>

      <Section title="Errors by Day">
        <div className="bg-white rounded-lg border border-stone-200 p-4">
          <ResponsiveContainer width="100%" height={180}>
            <BarChart data={data.byDay.map((d) => ({ ...d, date: shortDate(d.date) }))}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e7e5e4" />
              <XAxis dataKey="date" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip />
              <Bar dataKey="count" fill="#ef4444" name="Errors" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </Section>

      <Section title="Top Error Events">
        <div className="bg-white rounded-lg border border-stone-200 overflow-hidden">
          {data.topEvents.length === 0 ? (
            <p className="text-sm text-stone-400 py-4 text-center">No errors</p>
          ) : (
            <table className="w-full text-[11px]">
              <thead>
                <tr className="bg-stone-50 border-b border-stone-200">
                  <th className="px-3 py-2 text-left font-semibold text-stone-500">Event</th>
                  <th className="px-3 py-2 text-right font-semibold text-stone-500">Count</th>
                </tr>
              </thead>
              <tbody>
                {data.topEvents.map((e) => (
                  <tr key={e.event} className="border-b border-stone-50">
                    <td className="px-3 py-2 font-mono text-stone-600">{e.event}</td>
                    <td className="px-3 py-2 text-right font-bold text-red-600">{e.count}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </Section>
    </div>
  );
}

// ── Main Dashboard ────────────────────────────────────────────────────────────

type Tab = 'logs' | 'job-health' | 'cost' | 'workspace' | 'errors';

const TABS: { id: Tab; label: string }[] = [
  { id: 'logs', label: 'Logs' },
  { id: 'job-health', label: 'Job Health' },
  { id: 'cost', label: 'Cost & Usage' },
  { id: 'workspace', label: 'Workspace Activity' },
  { id: 'errors', label: 'Errors & Alerts' },
];

const PERIODS: { id: Period; label: string }[] = [
  { id: 'today', label: 'Today' },
  { id: '7d', label: 'Past 7 days' },
  { id: '30d', label: 'This month' },
];

export function Dashboard() {
  const [tab, setTab] = useState<Tab>('job-health');
  const [period, setPeriod] = useState<Period>('7d');
  const [focusedJobId, setFocusedJobId] = useState<string | null>(null);

  function drillDownToJob(jobId: string) {
    setFocusedJobId(jobId);
    setTab('logs');
  }

  const renderTab = useCallback(() => {
    switch (tab) {
      case 'logs':       return <AuditPage embedded forcedJobId={focusedJobId} onJobSelect={setFocusedJobId} />;
      case 'job-health': return <JobHealthTab period={period} onDrillDown={drillDownToJob} />;
      case 'cost':       return <CostTab period={period} />;
      case 'workspace':  return <WorkspaceTab period={period} />;
      case 'errors':     return <ErrorsTab period={period} />;
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab, period, focusedJobId]);

  return (
    <div className="flex h-screen bg-stone-50 overflow-hidden">
      {/* Sidebar */}
      <div className="w-56 border-r border-stone-200 bg-white flex flex-col shrink-0">
        <div className="p-4 border-b border-stone-100 bg-stone-50/50">
          <h1 className="text-sm font-bold tracking-tight text-stone-800 uppercase">Dashboards</h1>
          <p className="text-[10px] text-stone-400 font-medium mt-0.5">Operations BI</p>
        </div>

        <nav className="flex-1 p-2">
          {TABS.map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={`w-full text-left px-3 py-2 rounded-md text-xs font-medium mb-0.5 transition-colors ${
                tab === t.id
                  ? 'bg-stone-800 text-white'
                  : 'text-stone-600 hover:bg-stone-100'
              }`}
            >
              {t.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Main */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {tab !== 'logs' && (
          <div className="flex items-center gap-2 px-6 py-3 border-b border-stone-200 bg-white shrink-0">
            <span className="text-xs text-stone-400 mr-2">Period:</span>
            {PERIODS.map((p) => (
              <button
                key={p.id}
                onClick={() => setPeriod(p.id)}
                className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
                  period === p.id
                    ? 'bg-stone-800 text-white'
                    : 'bg-stone-100 text-stone-600 hover:bg-stone-200'
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>
        )}

        {/* Content */}
        <div className={`flex-1 overflow-hidden ${tab === 'logs' ? '' : 'overflow-y-auto p-6'}`}>
          {renderTab()}
        </div>
      </div>
    </div>
  );
}

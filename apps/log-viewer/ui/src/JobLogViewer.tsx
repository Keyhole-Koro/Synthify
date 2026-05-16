'use client';

import React, { useEffect, useMemo, useState } from 'react';
import type { JobLog, JobLogDataSource, JobLogFilters, JobLogGroup } from './types';

interface JobLogViewerProps {
  jobId: string;
  documentId?: string;
  workspaceId?: string;
  dataSource: JobLogDataSource;
}

type ViewMode = 'timeline' | 'flow' | 'related';

const levelClass: Record<string, string> = {
  INFO: 'border-stone-200 bg-white text-stone-700',
  WARN: 'border-amber-200 bg-amber-50 text-amber-800',
  ERROR: 'border-red-200 bg-red-50 text-red-800',
};

export function JobLogViewer({ jobId, documentId, workspaceId, dataSource }: JobLogViewerProps) {
  const [logs, setLogs] = useState<JobLog[]>([]);
  const [related, setRelated] = useState<JobLogGroup[]>([]);
  const [mode, setMode] = useState<ViewMode>('timeline');
  const [filters, setFilters] = useState<JobLogFilters>({ query: '', levels: [], events: [], scope: 'job' });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [nextPageToken, setNextPageToken] = useState('');
  const [loadingMore, setLoadingMore] = useState(false);

  const isFinished = useMemo(() => {
    return logs.some(l => l.event === 'job.completed' || l.event === 'job.failed');
  }, [logs]);

  const isSearch = filters.query.trim().length > 0
    || filters.levels.length > 0
    || filters.events.length > 0
    || !!filters.fromTimestamp
    || !!filters.toTimestamp;

  useEffect(() => {
    let cancelled = false;

    async function loadInitialPage() {
      setLoading(true);
      setError(null);
      try {
        const res = isSearch
          ? await dataSource.searchJobLogs(filters, { jobId, documentId, workspaceId })
          : await dataSource.listJobLogs(jobId);
        if (!cancelled) {
          setLogs(res.logs);
          setNextPageToken(res.nextPageToken);
        }
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load job logs');
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadInitialPage();
    return () => {
      cancelled = true;
    };
  }, [dataSource, documentId, filters, isSearch, jobId, workspaceId]);

  useEffect(() => {
    if (loading || isFinished || logs.length === 0 || filters.toTimestamp) {
      return;
    }

    let cancelled = false;
    const interval = setInterval(() => {
      void (async () => {
        try {
          const lastLog = logs[logs.length - 1];
          const deltaFilters: JobLogFilters = {
            ...filters,
            fromTimestamp: lastLog.timestamp,
          };
          const res = await dataSource.searchJobLogs(deltaFilters, { jobId, documentId, workspaceId });
          if (cancelled || res.logs.length === 0) {
            return;
          }
          setLogs((current) => mergeLogs(current, res.logs));
        } catch (err) {
          if (!cancelled) {
            setError(err instanceof Error ? err.message : 'Failed to refresh job logs');
          }
        }
      })();
    }, 3000);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [dataSource, documentId, filters, isFinished, jobId, loading, logs, workspaceId]);

  useEffect(() => {
    if (mode !== 'related') return;
    let cancelled = false;
    async function loadRelated() {
      try {
        const res = await dataSource.listRelatedJobLogs(filters.scope, { jobId, documentId, workspaceId });
        if (!cancelled) setRelated(res.groups);
      } catch (err) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load related logs');
      }
    }
    void loadRelated();
    return () => { cancelled = true; };
  }, [dataSource, documentId, filters.scope, jobId, mode, workspaceId]);

  async function handleLoadMore() {
    if (!nextPageToken || loadingMore) return;
    setLoadingMore(true);
    try {
      const res = isSearch
        ? await dataSource.searchJobLogs(filters, { jobId, documentId, workspaceId }, nextPageToken)
        : await dataSource.listJobLogs(jobId, nextPageToken);
      setLogs((prev) => mergeLogs(res.logs, prev));
      setNextPageToken(res.nextPageToken);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load more logs');
    } finally {
      setLoadingMore(false);
    }
  }

  const eventOptions = useMemo(() => {
    return Array.from(new Set(logs.map((log) => log.event))).sort();
  }, [logs]);

  return (
    <div className="flex h-full flex-col bg-stone-50 text-stone-800">
      <div className="border-b border-stone-200 bg-white px-4 py-3">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h3 className="text-sm font-bold tracking-tight">Job Log Viewer</h3>
            <p className="text-[10px] text-stone-400">job {jobId}</p>
          </div>
          <Segmented value={mode} values={['timeline', 'flow', 'related']} onChange={(value) => setMode(value as ViewMode)} />
        </div>
        <div className="mt-3 grid gap-2 md:grid-cols-[1fr_auto_auto]">
          <input
            value={filters.query}
            onChange={(event) => setFilters((current) => ({ ...current, query: event.target.value }))}
            placeholder="Search logs, tools, errors, JSON..."
            className="min-w-0 rounded-md border border-stone-200 bg-white px-3 py-2 text-xs outline-none focus:border-stone-400"
          />
          <select
            value={filters.scope}
            onChange={(event) => setFilters((current) => ({ ...current, scope: event.target.value as JobLogFilters['scope'] }))}
            className="rounded-md border border-stone-200 bg-white px-2 py-2 text-xs"
          >
            <option value="job">This job</option>
            <option value="document">This document</option>
            <option value="workspace">This workspace</option>
          </select>
          <select
            value={filters.events[0] ?? ''}
            onChange={(event) => setFilters((current) => ({ ...current, events: event.target.value ? [event.target.value] : [] }))}
            className="rounded-md border border-stone-200 bg-white px-2 py-2 text-xs"
          >
            <option value="">All events</option>
            {eventOptions.map((event) => <option key={event} value={event}>{event}</option>)}
          </select>
          <select
            value={filters.levels[0] ?? ''}
            onChange={(event) => setFilters((current) => ({ ...current, levels: event.target.value ? [event.target.value] : [] }))}
            className="rounded-md border border-stone-200 bg-white px-2 py-2 text-xs"
          >
            <option value="">All levels</option>
            <option value="INFO">INFO</option>
            <option value="WARN">WARN</option>
            <option value="ERROR">ERROR</option>
          </select>
          <div className="flex gap-2 items-center text-xs text-stone-500">
            <input
              type="datetime-local"
              value={formatDateTimeLocal(filters.fromTimestamp)}
              onChange={(e) => setFilters((c) => ({ ...c, fromTimestamp: parseDateTimeLocal(e.target.value) }))}
              className="rounded border border-stone-200 px-2 py-1 bg-white"
              title="From"
            />
            -
            <input
              type="datetime-local"
              value={formatDateTimeLocal(filters.toTimestamp)}
              onChange={(e) => setFilters((c) => ({ ...c, toTimestamp: parseDateTimeLocal(e.target.value) }))}
              className="rounded border border-stone-200 px-2 py-1 bg-white"
              title="To"
            />
          </div>
        </div>
      </div>

      {loading && <div className="p-5 text-xs text-stone-400">Loading logs...</div>}
      {error && <div className="m-4 rounded-md border border-red-200 bg-red-50 p-3 text-xs text-red-700">{error}</div>}

      {!loading && !error && (
        <div className="flex-1 overflow-auto p-4">
          {mode === 'timeline' && <Timeline logs={logs} />}
          {mode === 'flow' && <Flow logs={logs} />}
          {mode === 'related' && <Related groups={related} />}
          
          {nextPageToken && mode !== 'related' && (
            <div className="mt-4 flex justify-center">
              <button
                onClick={() => void handleLoadMore()}
                disabled={loadingMore}
                className="px-4 py-2 text-xs font-bold rounded-full bg-stone-200 text-stone-700 hover:bg-stone-300 disabled:opacity-50"
              >
                {loadingMore ? 'Loading...' : 'Load More'}
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function Segmented({ value, values, onChange }: { value: string; values: string[]; onChange: (value: string) => void }) {
  return (
    <div className="flex rounded-md bg-stone-100 p-1">
      {values.map((item) => (
        <button
          key={item}
          onClick={() => onChange(item)}
          className={`px-3 py-1 text-[10px] font-bold uppercase ${value === item ? 'rounded bg-white text-stone-900 shadow-sm' : 'text-stone-500'}`}
        >
          {item}
        </button>
      ))}
    </div>
  );
}

function Timeline({ logs }: { logs: JobLog[] }) {
  if (logs.length === 0) return <Empty />;
  return (
    <div className="mx-auto flex max-w-4xl flex-col gap-3">
      {logs.map((log) => <LogCard key={`${log.source}:${log.sourceId}`} log={log} />)}
    </div>
  );
}

function Flow({ logs }: { logs: JobLog[] }) {
  const toolLogs = logs.filter((log) => log.source === 'tool');
  if (toolLogs.length === 0) return <Empty />;
  return (
    <div className="flex min-w-max flex-col items-center gap-8 pb-16">
      <div className="rounded-md bg-stone-900 px-5 py-3 text-xs font-bold text-white">Orchestrator</div>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
        {toolLogs.map((log) => <LogCard key={`${log.source}:${log.sourceId}`} log={log} compact />)}
      </div>
    </div>
  );
}

function Related({ groups }: { groups: JobLogGroup[] }) {
  const safeGroups = Array.isArray(groups) ? groups : [];
  if (safeGroups.length === 0) return <Empty />;
  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-5">
      {safeGroups.map((group) => (
        <section key={`${group.workspaceId}:${group.documentId}`} className="border-l border-stone-200 pl-4">
          <div className="mb-3">
            <h4 className="text-xs font-bold text-stone-800">{group.documentId}</h4>
            <p className="text-[10px] text-stone-400">{group.workspaceId}</p>
          </div>
          <div className="flex flex-col gap-4">
            {(Array.isArray(group.jobs) ? group.jobs : []).map((job) => (
              <div key={job.jobId} className="rounded-md border border-stone-200 bg-white p-3">
                <div className="mb-2 flex items-center justify-between">
                  <span className="text-xs font-bold">{job.jobId}</span>
                  <span className="text-[10px] text-stone-400">{new Date(job.createdAt).toLocaleString()}</span>
                </div>
                <Timeline logs={Array.isArray(job.logs) ? job.logs : []} />
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

function LogCard({ log, compact = false }: { log: JobLog; compact?: boolean }) {
  const [open, setOpen] = useState(false);
  const detail = parseDetail(log.detailJson);
  return (
    <article className={`rounded-md border p-3 shadow-sm ${levelClass[log.level] ?? levelClass.INFO}`}>
      <button className="w-full text-left" onClick={() => setOpen((value) => !value)}>
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <span className="rounded bg-stone-100 px-1.5 py-0.5 text-[9px] font-bold">{log.level}</span>
              <span className="truncate text-xs font-bold">{log.event}</span>
            </div>
            <p className="mt-1 truncate text-xs text-stone-600">{log.message}</p>
          </div>
          <span className="shrink-0 text-[10px] text-stone-400">{new Date(log.timestamp).toLocaleTimeString()}</span>
        </div>
      </button>
      {open && !compact && (
        <pre className="mt-3 max-h-80 overflow-auto rounded border border-stone-200 bg-stone-950 p-3 text-[10px] leading-relaxed text-stone-100">
          {JSON.stringify(detail, null, 2)}
        </pre>
      )}
    </article>
  );
}

function Empty() {
  return <div className="flex h-full items-center justify-center p-10 text-xs italic text-stone-400">No logs found.</div>;
}

function parseDetail(value: string) {
  try {
    return JSON.parse(value || '{}');
  } catch {
    return { raw: value };
  }
}

function mergeLogs(older: JobLog[], newer: JobLog[]) {
  const seen = new Set<string>();
  const merged: JobLog[] = [];
  for (const log of [...older, ...newer]) {
    const key = `${log.source}:${log.sourceId}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    merged.push(log);
  }
  merged.sort((left, right) => {
    const timeDiff = new Date(left.timestamp).getTime() - new Date(right.timestamp).getTime();
    if (timeDiff !== 0) {
      return timeDiff;
    }
    return `${left.source}:${left.sourceId}`.localeCompare(`${right.source}:${right.sourceId}`);
  });
  return merged;
}

function formatDateTimeLocal(value?: string) {
  if (!value) {
    return '';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  const pad = (part: number) => String(part).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function parseDateTimeLocal(value: string) {
  if (!value) {
    return undefined;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return date.toISOString();
}

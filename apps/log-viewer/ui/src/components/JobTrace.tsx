'use client';

import React, { useEffect, useState } from 'react';
import type { JobTraceData, ToolCallEntry } from '@/lib/trace-queries';

function fmtDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function fmtTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

function summarize(json: string, maxLen = 200): string {
  try {
    const parsed = JSON.parse(json);
    const str = JSON.stringify(parsed, null, 0);
    return str.length > maxLen ? str.slice(0, maxLen) + '…' : str;
  } catch {
    return json.slice(0, maxLen);
  }
}

function ToolCallRow({ entry, index }: { entry: ToolCallEntry; index: number }) {
  const [expanded, setExpanded] = useState(false);

  return (
    <div className={`border-b border-stone-100 last:border-0 ${entry.isError ? 'bg-red-50/50' : ''}`}>
      <button
        onClick={() => setExpanded((v) => !v)}
        className="w-full text-left px-4 py-3 flex items-center gap-3 hover:bg-stone-50 transition-colors"
      >
        {/* Index bubble */}
        <span className="shrink-0 w-5 h-5 rounded-full bg-stone-200 text-stone-500 text-[9px] font-bold flex items-center justify-center">
          {index + 1}
        </span>

        {/* Status dot */}
        <span className={`shrink-0 w-2 h-2 rounded-full ${entry.isError ? 'bg-red-400' : 'bg-emerald-400'}`} />

        {/* Tool name */}
        <span className="flex-1 text-xs font-mono font-semibold text-stone-700">{entry.toolName}</span>

        {/* Duration */}
        <span className={`text-[10px] font-mono px-1.5 py-0.5 rounded ${
          entry.durationMs > 10000 ? 'bg-amber-100 text-amber-700' : 'bg-stone-100 text-stone-500'
        }`}>
          {fmtDuration(entry.durationMs)}
        </span>

        {/* Time */}
        <span className="text-[10px] text-stone-400 shrink-0">{fmtTime(entry.createdAt)}</span>

        {/* Expand chevron */}
        <svg
          className={`shrink-0 w-3.5 h-3.5 text-stone-400 transition-transform ${expanded ? 'rotate-180' : ''}`}
          fill="none" viewBox="0 0 24 24" stroke="currentColor"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {expanded && (
        <div className="px-4 pb-3 grid grid-cols-2 gap-3">
          <div>
            <p className="text-[9px] uppercase tracking-wider text-stone-400 font-semibold mb-1">Input</p>
            <pre className="text-[10px] font-mono bg-stone-100 rounded p-2 overflow-x-auto whitespace-pre-wrap break-all max-h-40 overflow-y-auto text-stone-600">
              {summarize(entry.inputJson, 1000)}
            </pre>
          </div>
          <div>
            <p className={`text-[9px] uppercase tracking-wider font-semibold mb-1 ${entry.isError ? 'text-red-400' : 'text-stone-400'}`}>
              {entry.isError ? 'Error' : 'Output'}
            </p>
            <pre className={`text-[10px] font-mono rounded p-2 overflow-x-auto whitespace-pre-wrap break-all max-h-40 overflow-y-auto ${
              entry.isError ? 'bg-red-50 text-red-700' : 'bg-stone-100 text-stone-600'
            }`}>
              {summarize(entry.outputJson, 1000)}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}

export function JobTrace({ jobId }: { jobId: string }) {
  const [data, setData] = useState<JobTraceData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    setData(null);
    fetch(`/api/jobs/trace?jobId=${encodeURIComponent(jobId)}`)
      .then((r) => r.json())
      .then((d: JobTraceData) => setData(d))
      .catch(console.error)
      .finally(() => setLoading(false));
  }, [jobId]);

  if (loading) {
    return <div className="p-8 text-sm text-stone-400 text-center">Loading trace...</div>;
  }

  if (!data || data.toolCalls.length === 0) {
    return (
      <div className="p-8 text-sm text-stone-400 text-center">
        No tool calls recorded for this job.
      </div>
    );
  }

  const errorCount = data.toolCalls.filter((t) => t.isError).length;
  const totalMs = data.toolCalls.reduce((sum, t) => sum + t.durationMs, 0);

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Summary bar */}
      <div className="flex items-center gap-4 px-4 py-2.5 border-b border-stone-200 bg-stone-50 shrink-0">
        <span className="text-[11px] text-stone-500">
          <span className="font-bold text-stone-700">{data.toolCalls.length}</span> tool calls
        </span>
        {errorCount > 0 && (
          <span className="text-[11px] text-red-600">
            <span className="font-bold">{errorCount}</span> errors
          </span>
        )}
        <span className="text-[11px] text-stone-500">
          total <span className="font-bold text-stone-700">{fmtDuration(totalMs)}</span>
        </span>
      </div>

      {/* Timeline */}
      <div className="flex-1 overflow-y-auto bg-white">
        {data.toolCalls.map((entry, i) => (
          <ToolCallRow key={entry.mutationId} entry={entry} index={i} />
        ))}
      </div>
    </div>
  );
}

'use client';

import { useMemo, useState } from 'react';
import type { EvalCaseRow, EvalRunRow } from '@/lib/eval-queries';

type TraceKind = 'case' | 'tool' | 'llm' | 'validation' | 'assertion';

type TraceNode = {
  id: string;
  kind: TraceKind;
  title: string;
  subtitle: string;
  status: 'ok' | 'error' | 'unknown';
  durationMs?: number;
  details: Record<string, unknown>;
};

function fmtMs(ms?: number): string {
  if (ms === undefined) return 'not recorded';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function json(value: unknown): string {
  if (value === undefined || value === null || value === '') return 'Not recorded';
  try { return JSON.stringify(value, null, 2); } catch { return String(value); }
}

function buildNodes(row: EvalCaseRow): TraceNode[] {
  const executionFailed = Boolean(row.error) && !row.output;
  return [
    {
      id: 'case', kind: 'case', title: 'Eval case', subtitle: row.caseName, status: 'ok',
      details: { runId: row.runId, caseName: row.caseName, promptSource: row.promptSource, failedInput: row.failedInput },
    },
    {
      id: 'tool', kind: 'tool', title: `Tool · ${row.tool}`, subtitle: 'Top-level eval tool invocation',
      status: executionFailed ? 'error' : 'ok', durationMs: row.durationMs,
      details: { tool: row.tool, input: row.failedInput ?? 'Input payload is only persisted for failed cases', output: row.output, error: row.error || null },
    },
    {
      id: 'llm', kind: 'llm', title: `LLM · ${row.model || 'unknown model'}`, subtitle: row.promptSource,
      status: executionFailed ? 'error' : 'ok', durationMs: row.durationMs,
      details: {
        model: row.model,
        promptSource: row.promptSource,
        inputTokens: row.inputTokens,
        outputTokens: row.outputTokens,
        prompt: 'Not recorded yet',
        rawResponse: row.output ?? 'Not recorded',
        note: 'Duration currently covers the top-level tool execution. Dedicated LLM spans will replace this when instrumentation is available.',
      },
    },
    {
      id: 'validation', kind: 'validation', title: 'Schema validation', subtitle: row.schemaValid ? 'Output contract satisfied' : 'Output contract failed',
      status: row.schemaValid ? 'ok' : 'error',
      details: { schemaValid: row.schemaValid, error: !row.schemaValid ? row.error : null },
    },
    {
      id: 'assertion', kind: 'assertion', title: 'Semantic assertions', subtitle: row.passed ? 'Expectations passed' : 'Expectation failed',
      status: row.passed ? 'ok' : 'error',
      details: { passed: row.passed, error: !row.passed ? row.error : null, output: row.output },
    },
  ];
}

const kindLabel: Record<TraceKind, string> = { case: 'CASE', tool: 'TOOL', llm: 'LLM', validation: 'CHECK', assertion: 'ASSERT' };

export function EvalExecutionTrace({ runs, cases }: { runs: EvalRunRow[]; cases: EvalCaseRow[] }) {
  const [runId, setRunId] = useState(cases[0]?.runId ?? runs[0]?.runId ?? '');
  const visibleCases = useMemo(() => cases.filter((row) => !runId || row.runId === runId), [cases, runId]);
  const [caseKey, setCaseKey] = useState(cases[0] ? `${cases[0].runId}:${cases[0].caseName}` : '');
  const selectedCase = visibleCases.find((row) => `${row.runId}:${row.caseName}` === caseKey) ?? visibleCases[0] ?? cases[0];
  const nodes = selectedCase ? buildNodes(selectedCase) : [];
  const [nodeId, setNodeId] = useState('llm');
  const selectedNode = nodes.find((node) => node.id === nodeId) ?? nodes[0];

  if (!selectedCase) return <div className="rounded-xl border border-stone-200 bg-white p-10 text-center text-sm text-stone-400">No eval cases in this period.</div>;

  return (
    <div className="overflow-hidden rounded-xl border border-stone-200 bg-white">
      <div className="grid min-h-[620px] grid-cols-[240px_minmax(460px,1fr)_360px]">
        <aside className="border-r border-stone-200 bg-stone-50/70 p-3">
          <p className="mb-2 text-[9px] font-bold uppercase tracking-wider text-stone-400">Run</p>
          <select className="mb-4 w-full rounded border border-stone-200 bg-white px-2 py-2 text-[10px] font-mono" value={runId} onChange={(event) => { setRunId(event.target.value); setCaseKey(''); }}>
            {runs.map((run) => <option key={run.runId} value={run.runId}>{run.promptSource} · {run.runId.slice(0, 8)}</option>)}
          </select>
          <p className="mb-2 text-[9px] font-bold uppercase tracking-wider text-stone-400">Cases</p>
          <div className="space-y-1">
            {visibleCases.map((row) => {
              const key = `${row.runId}:${row.caseName}`;
              return <button key={key} onClick={() => { setCaseKey(key); setNodeId('llm'); }} className={`w-full rounded-md border px-3 py-2 text-left ${key === `${selectedCase.runId}:${selectedCase.caseName}` ? 'border-stone-400 bg-white shadow-sm' : 'border-transparent hover:bg-white'}`}>
                <p className="truncate text-[10px] font-semibold text-stone-700">{row.caseName}</p>
                <p className={`mt-1 text-[9px] ${row.passed ? 'text-emerald-600' : 'text-red-600'}`}>{row.passed ? 'passed' : 'failed'} · {fmtMs(row.durationMs)}</p>
              </button>;
            })}
          </div>
        </aside>

        <section className="overflow-auto p-5">
          <div className="mb-5 flex items-center justify-between">
            <div><h3 className="text-sm font-bold text-stone-800">{selectedCase.caseName}</h3><p className="text-[10px] text-stone-400">Ordered execution trace · persisted telemetry only</p></div>
            <div className="flex gap-2 text-[9px]"><span className="rounded bg-stone-100 px-2 py-1">{selectedCase.inputTokens + selectedCase.outputTokens} tokens</span><span className="rounded bg-stone-100 px-2 py-1">{fmtMs(selectedCase.durationMs)}</span></div>
          </div>
          <div className="mx-auto max-w-xl">
            {nodes.map((node, index) => (
              <div key={node.id}>
                <button onClick={() => setNodeId(node.id)} className={`w-full rounded-xl border p-4 text-left transition ${selectedNode?.id === node.id ? 'border-stone-500 ring-2 ring-stone-200' : 'border-stone-200 hover:border-stone-300'} ${node.status === 'error' ? 'bg-red-50/60' : 'bg-white'}`}>
                  <div className="flex items-center gap-3"><span className={`rounded px-2 py-1 text-[9px] font-bold ${node.kind === 'llm' ? 'bg-violet-100 text-violet-700' : node.kind === 'tool' ? 'bg-sky-100 text-sky-700' : 'bg-stone-100 text-stone-600'}`}>{kindLabel[node.kind]}</span><div className="min-w-0 flex-1"><p className="text-xs font-bold text-stone-800">{node.title}</p><p className="truncate text-[10px] text-stone-400">{node.subtitle}</p></div>{node.durationMs !== undefined && <span className="font-mono text-[10px] text-stone-500">{fmtMs(node.durationMs)}</span>}<span className={`h-2.5 w-2.5 rounded-full ${node.status === 'error' ? 'bg-red-500' : node.status === 'ok' ? 'bg-emerald-500' : 'bg-stone-300'}`} /></div>
                </button>
                {index < nodes.length - 1 && <div className="mx-auto h-8 w-px bg-stone-300"><div className="relative top-6 -left-[3px] h-2 w-2 rotate-45 border-b border-r border-stone-400" /></div>}
              </div>
            ))}
          </div>
        </section>

        <aside className="overflow-auto border-l border-stone-200 bg-stone-950 p-4 text-stone-100">
          <p className="text-[9px] font-bold uppercase tracking-wider text-stone-500">Selected node</p>
          <h3 className="mt-1 text-sm font-bold">{selectedNode?.title}</h3>
          <p className="mt-1 text-[10px] text-stone-400">{selectedNode?.subtitle}</p>
          <pre className="mt-4 whitespace-pre-wrap break-words rounded-lg bg-black/30 p-3 text-[10px] leading-5 text-stone-300">{json(selectedNode?.details)}</pre>
        </aside>
      </div>
    </div>
  );
}

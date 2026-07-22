'use client';

import type { EvalFunnelData, EvalPromptSourceRow, EvalRunRow } from '@/lib/eval-queries';

function pct(value: number, total: number): string {
  return total > 0 ? `${((value / total) * 100).toFixed(1)}%` : '0.0%';
}

function fmtMs(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60_000).toFixed(1)}m`;
}

function fmtNumber(value: number): string {
  return new Intl.NumberFormat('en-US', { notation: value >= 10_000 ? 'compact' : 'standard' }).format(value);
}

function toneClasses(tone: 'neutral' | 'blue' | 'amber' | 'emerald') {
  switch (tone) {
    case 'blue': return 'border-sky-200 bg-sky-50 text-sky-800';
    case 'amber': return 'border-amber-200 bg-amber-50 text-amber-800';
    case 'emerald': return 'border-emerald-200 bg-emerald-50 text-emerald-800';
    default: return 'border-stone-200 bg-white text-stone-800';
  }
}

function PipelineStage({
  label,
  count,
  total,
  detail,
  tone,
}: {
  label: string;
  count: number;
  total: number;
  detail: string;
  tone: 'neutral' | 'blue' | 'amber' | 'emerald';
}) {
  return (
    <div className={`min-w-[150px] flex-1 rounded-xl border p-4 shadow-sm ${toneClasses(tone)}`}>
      <p className="text-[10px] font-bold uppercase tracking-wider opacity-60">{label}</p>
      <div className="mt-2 flex items-end justify-between gap-3">
        <p className="text-2xl font-bold">{fmtNumber(count)}</p>
        <p className="text-xs font-semibold opacity-70">{pct(count, total)}</p>
      </div>
      <p className="mt-2 text-[10px] leading-4 opacity-65">{detail}</p>
    </div>
  );
}

function Arrow() {
  return (
    <div className="hidden shrink-0 items-center xl:flex" aria-hidden>
      <div className="h-px w-5 bg-stone-300" />
      <div className="-ml-1 border-y-4 border-l-6 border-y-transparent border-l-stone-300" />
    </div>
  );
}

export function EvalPipelineDiagram({ funnel, artifactRuns, runCount }: { funnel: EvalFunnelData; artifactRuns: number; runCount: number }) {
  const total = funnel.totalCases;
  return (
    <div className="rounded-xl border border-stone-200 bg-white p-5">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-stretch">
        <PipelineStage label="Cases loaded" count={funnel.totalCases} total={total} detail="YAML cases resolved by the eval runner" tone="neutral" />
        <Arrow />
        <PipelineStage label="Model completed" count={funnel.executionCompleted} total={total} detail={`${fmtNumber(funnel.executionErrors)} execution errors branched out`} tone="blue" />
        <Arrow />
        <PipelineStage label="Schema valid" count={funnel.schemaValid} total={total} detail={`${fmtNumber(funnel.schemaInvalid)} outputs failed the response contract`} tone="amber" />
        <Arrow />
        <PipelineStage label="Assertions passed" count={funnel.passed} total={total} detail={`${fmtNumber(funnel.assertionFailures)} semantic expectation failures`} tone="emerald" />
      </div>

      <div className="mt-5 grid gap-3 md:grid-cols-[1fr_auto_1fr_auto_1fr] md:items-center">
        <div className="rounded-lg border border-stone-200 bg-stone-50 px-4 py-3">
          <p className="text-[10px] font-bold uppercase tracking-wider text-stone-400">Result telemetry</p>
          <p className="mt-1 text-sm font-semibold text-stone-700">run + case records</p>
          <p className="mt-1 text-[10px] text-stone-400">latency, tokens, output, failed input</p>
        </div>
        <div className="hidden text-stone-300 md:block">→</div>
        <div className="rounded-lg border border-stone-200 bg-stone-50 px-4 py-3">
          <p className="text-[10px] font-bold uppercase tracking-wider text-stone-400">CockroachDB</p>
          <p className="mt-1 text-sm font-semibold text-stone-700">read-only monitor views</p>
          <p className="mt-1 text-[10px] text-stone-400">v_eval_runs + v_eval_case_results</p>
        </div>
        <div className="hidden text-stone-300 md:block">→</div>
        <div className="rounded-lg border border-stone-200 bg-stone-50 px-4 py-3">
          <p className="text-[10px] font-bold uppercase tracking-wider text-stone-400">Monitor + artifacts</p>
          <p className="mt-1 text-sm font-semibold text-stone-700">dashboard and GCS reports</p>
          <p className="mt-1 text-[10px] text-stone-400">{artifactRuns}/{runCount} recent runs expose artifact URIs</p>
        </div>
      </div>
    </div>
  );
}

export function EvalFailureFunnel({ funnel }: { funnel: EvalFunnelData }) {
  const total = Math.max(funnel.totalCases, 1);
  const stages = [
    { label: 'Cases loaded', value: funnel.totalCases, className: 'bg-stone-700' },
    { label: 'Execution completed', value: funnel.executionCompleted, className: 'bg-sky-500' },
    { label: 'Schema valid', value: funnel.schemaValid, className: 'bg-amber-500' },
    { label: 'Assertions passed', value: funnel.passed, className: 'bg-emerald-500' },
  ];

  return (
    <div className="rounded-xl border border-stone-200 bg-white p-5">
      <div className="space-y-3">
        {stages.map((stage) => (
          <div key={stage.label}>
            <div className="mb-1 flex items-center justify-between text-[10px]">
              <span className="font-semibold text-stone-600">{stage.label}</span>
              <span className="font-mono text-stone-400">{fmtNumber(stage.value)} · {pct(stage.value, funnel.totalCases)}</span>
            </div>
            <div className="h-8 rounded-md bg-stone-100 p-1">
              <div
                className={`flex h-full min-w-[2.5rem] items-center justify-end rounded px-2 text-[9px] font-bold text-white transition-all ${stage.className}`}
                style={{ width: `${Math.max(8, (stage.value / total) * 100)}%` }}
              >
                {fmtNumber(stage.value)}
              </div>
            </div>
          </div>
        ))}
      </div>
      <div className="mt-5 grid grid-cols-3 gap-2">
        <div className="rounded-lg border border-red-100 bg-red-50 p-3 text-center">
          <p className="text-lg font-bold text-red-700">{fmtNumber(funnel.executionErrors)}</p>
          <p className="text-[9px] font-semibold uppercase tracking-wider text-red-500">Execution</p>
        </div>
        <div className="rounded-lg border border-amber-100 bg-amber-50 p-3 text-center">
          <p className="text-lg font-bold text-amber-700">{fmtNumber(funnel.schemaInvalid)}</p>
          <p className="text-[9px] font-semibold uppercase tracking-wider text-amber-500">Schema</p>
        </div>
        <div className="rounded-lg border border-orange-100 bg-orange-50 p-3 text-center">
          <p className="text-lg font-bold text-orange-700">{fmtNumber(funnel.assertionFailures)}</p>
          <p className="text-[9px] font-semibold uppercase tracking-wider text-orange-500">Assertion</p>
        </div>
      </div>
    </div>
  );
}

export function EvalRunTimeline({ runs }: { runs: EvalRunRow[] }) {
  const visible = runs.slice(0, 8);
  const maxDuration = Math.max(...visible.map((run) => run.durationMs), 1);

  return (
    <div className="rounded-xl border border-stone-200 bg-white p-5">
      {visible.length === 0 ? (
        <p className="py-12 text-center text-sm text-stone-400">No runs in this period</p>
      ) : (
        <div className="space-y-3">
          {visible.map((run) => {
            const width = Math.max(10, (run.durationMs / maxDuration) * 100);
            const started = new Date(run.startedAt);
            return (
              <div key={run.runId} className="grid grid-cols-[128px_1fr_58px] items-center gap-3">
                <div className="min-w-0">
                  <p className="truncate font-mono text-[10px] font-semibold text-stone-600" title={run.promptSource}>{run.promptSource}</p>
                  <p className="text-[9px] text-stone-400">
                    {Number.isNaN(started.getTime()) ? run.startedAt : started.toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
                  </p>
                </div>
                <div className="relative h-7 overflow-hidden rounded bg-stone-100">
                  <div
                    className={`flex h-full items-center rounded px-2 text-[9px] font-semibold text-white ${run.status === 'succeeded' ? 'bg-emerald-500' : 'bg-red-500'}`}
                    style={{ width: `${width}%` }}
                    title={`${run.runId}: ${run.passedCount}/${run.caseCount} passed`}
                  >
                    <span className="truncate">{run.model} · {run.passedCount}/{run.caseCount}</span>
                  </div>
                </div>
                <p className="text-right font-mono text-[10px] font-semibold text-stone-600">{fmtMs(run.durationMs)}</p>
              </div>
            );
          })}
        </div>
      )}
      <p className="mt-4 text-[9px] text-stone-400">Bar length is normalized to the slowest visible run. Hover a lane for the run ID and result.</p>
    </div>
  );
}

export function EvalVariantComparison({ rows }: { rows: EvalPromptSourceRow[] }) {
  const maxLatency = Math.max(...rows.map((row) => row.avgDurationMs), 1);
  const perCaseTokens = rows.map((row) => row.cases > 0 ? (row.inputTokens + row.outputTokens) / row.cases : 0);
  const maxTokens = Math.max(...perCaseTokens, 1);

  return (
    <div className="rounded-xl border border-stone-200 bg-white p-5">
      {rows.length === 0 ? (
        <p className="py-12 text-center text-sm text-stone-400">No prompt variants in this period</p>
      ) : (
        <div className="space-y-4">
          {rows.map((row, index) => {
            const tokens = perCaseTokens[index];
            return (
              <div key={row.promptSource} className="rounded-lg border border-stone-100 bg-stone-50/60 p-3">
                <div className="mb-3 flex items-start justify-between gap-3">
                  <div>
                    <p className="font-mono text-[11px] font-semibold text-stone-700">{row.promptSource}</p>
                    <p className="text-[9px] text-stone-400">{row.runs} runs · {row.cases} cases</p>
                  </div>
                  <span className="rounded-full bg-white px-2 py-1 text-[10px] font-bold text-emerald-700 shadow-sm">{(row.passRate * 100).toFixed(1)}%</span>
                </div>
                <div className="grid gap-2 text-[9px] sm:grid-cols-[72px_1fr_64px] sm:items-center">
                  <span className="font-semibold uppercase tracking-wider text-stone-400">Pass rate</span>
                  <div className="h-2 rounded-full bg-stone-200"><div className="h-full rounded-full bg-emerald-500" style={{ width: `${row.passRate * 100}%` }} /></div>
                  <span className="text-right font-mono text-stone-500">{(row.passRate * 100).toFixed(1)}%</span>
                  <span className="font-semibold uppercase tracking-wider text-stone-400">Avg run</span>
                  <div className="h-2 rounded-full bg-stone-200"><div className="h-full rounded-full bg-amber-500" style={{ width: `${(row.avgDurationMs / maxLatency) * 100}%` }} /></div>
                  <span className="text-right font-mono text-stone-500">{fmtMs(row.avgDurationMs)}</span>
                  <span className="font-semibold uppercase tracking-wider text-stone-400">Tokens/case</span>
                  <div className="h-2 rounded-full bg-stone-200"><div className="h-full rounded-full bg-sky-500" style={{ width: `${(tokens / maxTokens) * 100}%` }} /></div>
                  <span className="text-right font-mono text-stone-500">{fmtNumber(Math.round(tokens))}</span>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

'use client';

import { useEffect, useState } from 'react';
import { getBillingAccount, getUsage, type UsageReport } from './api';

interface UsagePaperProps {
  accountId: string;
}

interface UsageState {
  accountId: string;
  report: UsageReport | null;
  budget: number;
  loading: boolean;
  error: string | null;
}

export function UsagePaper({ accountId }: UsagePaperProps) {
  const [state, setState] = useState<UsageState>({
    accountId,
    report: null,
    budget: 0,
    loading: true,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;

    Promise.all([getUsage(accountId), getBillingAccount(accountId)])
      .then(([report, account]) => {
        if (!cancelled) {
          setState({
            accountId,
            report,
            budget: Number(account.budgetLimit) || 0,
            loading: false,
            error: null,
          });
        }
      })
      .catch(() => {
        if (!cancelled) {
          setState({
            accountId,
            report: null,
            budget: 0,
            loading: false,
            error: '使用量を取得できませんでした。',
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountId]);

  if (state.accountId !== accountId || state.loading) {
    return <p style={{ margin: 0, fontSize: '0.8rem', color: 'var(--muted)' }}>読み込み中...</p>;
  }
  if (state.error || !state.report) {
    return <p style={{ margin: 0, fontSize: '0.8rem', color: '#f87171' }}>{state.error ?? '使用量データがありません。'}</p>;
  }

  const total = Number(state.report.totalCost) || 0;
  const percent = state.budget > 0 ? Math.min(100, Math.round((total / state.budget) * 100)) : 0;
  const currencyLabel = (state.report.currency || 'usd').toUpperCase();

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 6 }}>
          <span style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--muted)', fontWeight: 700 }}>
            今月の合計 ({currencyLabel})
          </span>
          <span style={{ fontSize: '1.1rem', fontWeight: 700 }}>
            ${total.toFixed(2)}{' '}
            {state.budget > 0 && (
              <span style={{ fontSize: '0.75rem', color: 'var(--muted)', fontWeight: 400 }}>
                / ${state.budget.toFixed(0)}
              </span>
            )}
          </span>
        </div>
        {state.budget > 0 && (
          <div style={{ height: 8, borderRadius: 4, background: 'var(--surface-alt)', overflow: 'hidden' }}>
            <div style={{ height: '100%', width: `${percent}%`, background: percent >= 80 ? '#f59e0b' : 'var(--accent)' }} />
          </div>
        )}
      </div>

      <div>
        <p style={{ margin: '0 0 6px', fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--muted)', fontWeight: 700 }}>
          モデル別内訳
        </p>
        {state.report.byModel.length === 0 ? (
          <p style={{ margin: 0, fontSize: '0.8rem', color: 'var(--muted)' }}>当月の使用量はまだありません。</p>
        ) : (
          <table style={{ width: '100%', fontSize: '0.8rem', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ background: 'var(--surface-alt)' }}>
                <th style={{ padding: '6px 8px', textAlign: 'left', fontWeight: 600 }}>Model</th>
                <th style={{ padding: '6px 8px', textAlign: 'right', fontWeight: 600 }}>Input</th>
                <th style={{ padding: '6px 8px', textAlign: 'right', fontWeight: 600 }}>Output</th>
                <th style={{ padding: '6px 8px', textAlign: 'right', fontWeight: 600 }}>合計</th>
              </tr>
            </thead>
            <tbody>
              {state.report.byModel.map((row) => (
                <tr key={row.model} style={{ borderBottom: '1px solid var(--link-border)' }}>
                  <td style={{ padding: '6px 8px' }}>{row.model}</td>
                  <td style={{ padding: '6px 8px', textAlign: 'right', color: 'var(--muted)' }}>${Number(row.inputCost).toFixed(2)}</td>
                  <td style={{ padding: '6px 8px', textAlign: 'right', color: 'var(--muted)' }}>${Number(row.outputCost).toFixed(2)}</td>
                  <td style={{ padding: '6px 8px', textAlign: 'right', fontWeight: 600 }}>${Number(row.totalCost).toFixed(2)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

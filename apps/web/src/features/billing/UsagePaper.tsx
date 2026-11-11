
import { useQuery } from '@tanstack/react-query';
import { billingAccountQueryOptions, billingUsageQueryOptions } from './queries';

/**
 * Props for the detailed usage paper.
 */
interface UsagePaperProps {
  /** Target account identifier. */
  accountId: string;
}

/**
 * Paper component presenting aggregated monthly cost breakdown by LLM model and budget progress.
 *
 * @param props - Account identifier used to load usage and budget data.
 * @returns Usage paper showing totals, per-model costs, and budget progress/error states.
 */
export function UsagePaper({ accountId }: UsagePaperProps) {
  const {
    data: report,
    isLoading: isUsageLoading,
    error: usageError,
  } = useQuery(billingUsageQueryOptions(accountId));

  const {
    data: account,
    isLoading: isAccountLoading,
    error: accountError,
  } = useQuery(billingAccountQueryOptions(accountId));

  const loading = isUsageLoading || isAccountLoading;
  const error = usageError || accountError ? '使用量を取得できませんでした。' : null;

  if (loading) {
    return <p style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>読み込み中...</p>;
  }

  if (error || !report) {
    return <p style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>{error ?? 'データがありません'}</p>;
  }

  const budget = Number(account?.budgetLimit) || 0;
  const total = Number(report.totalCost) || 0;
  const percent = budget > 0 ? Math.min(100, Math.round((total / budget) * 100)) : 0;
  const currencyLabel = (report.currency || 'usd').toUpperCase();

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 6 }}>
          <span style={{ fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--muted)', fontWeight: 700 }}>
            今月の合計 ({currencyLabel})
          </span>
          <span style={{ fontSize: '1.1rem', fontWeight: 700 }}>
            ${total.toFixed(2)}{' '}
            {budget > 0 && (
              <span style={{ fontSize: '0.75rem', color: 'var(--muted)', fontWeight: 400 }}>
                / ${budget.toFixed(0)}
              </span>
            )}
          </span>
        </div>
        {budget > 0 && (
          <div style={{ height: 8, borderRadius: 4, background: 'var(--surface-alt)', overflow: 'hidden' }}>
            <div style={{ height: '100%', width: `${percent}%`, background: percent >= 80 ? '#f59e0b' : 'var(--accent)' }} />
          </div>
        )}
      </div>

      <div>
        <p style={{ margin: '0 0 6px', fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--muted)', fontWeight: 700 }}>
          モデル別内訳
        </p>
        {report.byModel.length === 0 ? (
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
              {report.byModel.map((row) => (
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

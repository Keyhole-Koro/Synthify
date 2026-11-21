import { useQuery } from '@tanstack/react-query';
import { billingAccountQueryOptions } from './queries';

/**
 * Props for the current plan paper.
 */
interface CurrentPlanPaperProps {
  /** Target account identifier. */
  accountId: string;
}

/**
 * Paper component detailing the user's active billing plan features, storage quota, and cancel status.
 *
 * @param props - Account identifier whose current plan should be loaded.
 * @returns Current-plan detail paper with loading, error, and populated plan states.
 */
export function CurrentPlanPaper({ accountId }: CurrentPlanPaperProps) {
  const { data: account, isLoading, error } = useQuery(billingAccountQueryOptions(accountId));

  if (isLoading || !account) {
    return (
      <p style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>
        {error ? 'プラン情報を取得できませんでした。' : '読み込み中...'}
      </p>
    );
  }

  const isUsageBased = account.plan === 'usage_based';
  const billingStatus = account.billingStatus;

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <Row label="プラン" value={isUsageBased ? 'Usage-Based' : 'Free'} />
      <Row
        label="月額固定費"
        value={isUsageBased ? '$0 + 従量' : '$0'}
      />
      <Row
        label="1ファイル最大サイズ"
        value={isUsageBased ? '500 MB' : '10 MB'}
      />
      <Row
        label="ストレージ上限"
        value={`${formatBytes(Number(account.storageQuotaBytes))} 中 ${formatBytes(Number(account.storageUsedBytes))} 使用`}
      />
      {billingStatus && billingStatus !== 'free' && billingStatus !== 'active' && (
        <Row label="決済ステータス" value={billingStatus.replace('_', ' ')} accent="warn" />
      )}
      {account.cancelAtPeriodEnd && account.currentPeriodEnd && (
        <Row label="解約予定" value={`${account.currentPeriodEnd} に終了`} />
      )}
    </div>
  );
}

/**
 * Displays a single key-value metric line with border styling.
 */
function Row({ label, value, accent }: { label: string; value: string; accent?: 'warn' }) {
  return (
    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', borderBottom: '1px solid var(--link-border)', paddingBottom: 6 }}>
      <span style={{ fontSize: '0.75rem', color: 'var(--muted)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{label}</span>
      <span style={{ fontSize: '0.9rem', fontWeight: 600, color: accent === 'warn' ? '#f59e0b' : 'inherit' }}>
        {value}
      </span>
    </div>
  );
}

/**
 * Formats a byte quantity into a human-readable string with units (B, KB, MB, GB, TB).
 */
function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value >= 10 || unit === 0 ? 0 : 1)} ${units[unit]}`;
}

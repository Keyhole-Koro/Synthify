'use client';

import { useEffect, useState } from 'react';
import { getBillingAccount, type BillingAccount } from './api';

interface CurrentPlanPaperProps {
  accountId: string;
}

export function CurrentPlanPaper({ accountId }: CurrentPlanPaperProps) {
  const [account, setAccount] = useState<BillingAccount | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getBillingAccount(accountId)
      .then(setAccount)
      .catch(() => setError('プラン情報を取得できませんでした。'));
  }, [accountId]);

  if (!account) {
    return <p style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>{error ?? '読み込み中...'}</p>;
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

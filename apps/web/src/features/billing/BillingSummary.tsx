'use client';

import { useEffect, useState } from 'react';
import { getBillingAccount, type BillingAccount } from './api';

interface BillingSummaryProps {
  accountId: string;
}

export function BillingSummary({ accountId }: BillingSummaryProps) {
  const [account, setAccount] = useState<BillingAccount | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getBillingAccount(accountId)
      .then(setAccount)
      .catch(() => setError('課金情報を取得できませんでした。'));
  }, [accountId]);

  if (!account) {
    return (
      <div style={{ fontSize: '0.8rem', color: 'var(--muted)' }}>
        {error ?? '読み込み中...'}
      </div>
    );
  }

  const quota = Number(account.storageQuotaBytes);
  const used = Number(account.storageUsedBytes);
  const usagePercent = quota > 0 ? Math.min(100, Math.round((used / quota) * 100)) : 0;
  const planLabel = account.plan === 'usage_based' ? 'Usage-Based' : 'Free';

  const monthlyUsage = Number(account.currentPeriodUsage) || 0;
  const budget = Number(account.budgetLimit) || 0;
  const budgetPercent = budget > 0 ? Math.min(100, Math.round((monthlyUsage / budget) * 100)) : 0;
  const usageSub = budget > 0
    ? `予算 $${budget.toFixed(0)} の ${budgetPercent}%${account.budgetExceeded ? ' (超過)' : ''}`
    : '予算未設定';

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.6, color: 'var(--muted)' }}>
        プランと使用量のサマリー。詳細は下の各ペーパーから。
      </p>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10 }}>
        <StatCard label="現在のプラン" value={planLabel} />
        <StatCard label="今月の使用量" value={`$${monthlyUsage.toFixed(2)}`} sub={usageSub} />
      </div>

      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.7rem', color: 'var(--muted)', marginBottom: 4 }}>
          <span>ストレージ</span>
          <span>{formatBytes(used)} / {formatBytes(quota)}</span>
        </div>
        <div style={{ height: 6, borderRadius: 3, background: 'var(--surface-alt)', overflow: 'hidden' }}>
          <div style={{ height: '100%', width: `${usagePercent}%`, background: 'var(--accent)' }} />
        </div>
      </div>

      <div>
        <p style={{ margin: '0 0 6px', fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--muted)', fontWeight: 700 }}>
          詳細
        </p>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8 }}>
          <NavLink paperId="billing:plan" title="現在のプラン" hint="Free / 上限" />
          <NavLink paperId="billing:budget" title="予算設定" hint="月次上限・通知" />
          <NavLink paperId="billing:usage" title="使用量" hint="モデル別内訳" />
          <NavLink paperId="billing:invoice" title="請求・支払い" hint="請求書・カード" />
          <NavLink paperId="billing:upgrade" title="アップグレード" hint="Pro へ移行" />
          <NavLink paperId="billing:manage" title="管理" hint="Stripe ポータル" />
        </div>
      </div>
    </div>
  );
}

function NavLink({ paperId, title, hint }: { paperId: string; title: string; hint: string }) {
  return (
    <a
      data-paper-id={paperId}
      tabIndex={0}
      style={{
        display: 'block',
        padding: '8px 10px',
        border: '1px solid var(--link-border)',
        borderRadius: 6,
        background: 'var(--link-bg)',
        cursor: 'pointer',
        textDecoration: 'none',
        color: 'inherit',
      }}
    >
      <p style={{ margin: 0, fontSize: '0.85rem', fontWeight: 600 }}>{title}</p>
      <p style={{ margin: '2px 0 0', fontSize: '0.7rem', color: 'var(--muted)' }}>{hint}</p>
    </a>
  );
}

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <div style={{ border: '1px solid var(--link-border)', borderRadius: 6, padding: '8px 10px', background: 'var(--surface-alt)' }}>
      <p style={{ margin: 0, fontSize: '0.65rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--muted)', fontWeight: 700 }}>
        {label}
      </p>
      <p style={{ margin: '4px 0 0', fontSize: '1rem', fontWeight: 700 }}>{value}</p>
      {sub && <p style={{ margin: '2px 0 0', fontSize: '0.7rem', color: 'var(--muted)' }}>{sub}</p>}
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

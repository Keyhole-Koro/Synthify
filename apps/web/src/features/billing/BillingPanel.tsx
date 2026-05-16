'use client';

import React, { useEffect, useState } from 'react';
import { getBillingAccount, createCheckoutSession, createPortalSession, type BillingAccount, type BillingCurrency } from '@/features/billing/api';

interface BillingPanelProps {
  accountId: string;
}

export function BillingPanel({ accountId }: BillingPanelProps) {
  const [account, setAccount] = useState<BillingAccount | null>(null);
  const [currency, setCurrency] = useState<BillingCurrency>('jpy');
  const [pendingAction, setPendingAction] = useState<'checkout' | 'portal' | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getBillingAccount(accountId).then(setAccount).catch(() => setError('課金情報を取得できませんでした。'));
  }, [accountId]);

  if (!account) {
    return (
      <div className="rounded-md border border-stone-200 bg-white/80 p-3">
        <p className="text-[11px] text-stone-400">{error ?? '読み込み中...'}</p>
      </div>
    );
  }

  const quota = Number(account.storageQuotaBytes);
  const used = Number(account.storageUsedBytes);
  const usagePercent = quota > 0 ? Math.min(100, Math.round((used / quota) * 100)) : 0;
  const isUsageBased = account.plan === 'usage_based';
  const billingStatus = account.billingStatus;

  async function openCheckout() {
    setPendingAction('checkout');
    setError(null);
    try {
      const url = await createCheckoutSession(accountId, currency);
      window.location.assign(url);
    } catch {
      setError('決済画面を開けませんでした。');
      setPendingAction(null);
    }
  }

  async function openPortal() {
    setPendingAction('portal');
    setError(null);
    try {
      const url = await createPortalSession(accountId);
      window.location.assign(url);
    } catch {
      setError('課金管理画面を開けませんでした。');
      setPendingAction(null);
    }
  }

  return (
    <div className="rounded-md border border-stone-200 bg-white/80 p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-[10px] font-semibold uppercase tracking-[0.16em] text-stone-400">Plan</p>
          <p className="mt-1 text-sm font-semibold text-stone-800">{isUsageBased ? 'Usage-Based' : 'Free'}</p>
          <p className="mt-0.5 text-[11px] text-stone-500">
            {formatBytes(used)} / {formatBytes(quota)} used
          </p>
          {billingStatus && billingStatus !== 'free' && billingStatus !== 'active' && (
            <p className="mt-0.5 text-[11px] font-semibold text-amber-500 capitalize">{billingStatus.replace('_', ' ')}</p>
          )}
          {account.cancelAtPeriodEnd && account.currentPeriodEnd && (
            <p className="mt-0.5 text-[11px] text-stone-400">
              {account.currentPeriodEnd} に終了予定
            </p>
          )}
        </div>
        {!isUsageBased && (
          <div className="flex shrink-0 overflow-hidden rounded-md border border-stone-200">
            {(['jpy', 'usd'] as BillingCurrency[]).map((value) => (
              <button
                key={value}
                type="button"
                onClick={() => setCurrency(value)}
                className={[
                  'px-2.5 py-1 text-[11px] font-semibold uppercase transition-colors',
                  currency === value ? 'bg-stone-800 text-white' : 'bg-white text-stone-500 hover:bg-stone-50',
                ].join(' ')}
              >
                {value}
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-stone-100">
        <div className="h-full rounded-full bg-indigo-500" style={{ width: `${usagePercent}%` }} />
      </div>

      <div className="mt-3 flex gap-2">
        {!isUsageBased && (
          <button
            type="button"
            disabled={pendingAction !== null}
            onClick={openCheckout}
            className="rounded-md bg-stone-900 px-3 py-1.5 text-[11px] font-semibold text-white transition-colors hover:bg-stone-700 disabled:cursor-wait disabled:opacity-50"
          >
            {pendingAction === 'checkout' ? 'Opening...' : 'Upgrade'}
          </button>
        )}
        <button
          type="button"
          disabled={pendingAction !== null}
          onClick={openPortal}
          className="rounded-md border border-stone-200 bg-white px-3 py-1.5 text-[11px] font-semibold text-stone-600 transition-colors hover:bg-stone-50 disabled:cursor-wait disabled:opacity-50"
        >
          {pendingAction === 'portal' ? 'Opening...' : 'Manage'}
        </button>
      </div>

      {error && <p className="mt-2 text-[11px] text-red-400">{error}</p>}
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

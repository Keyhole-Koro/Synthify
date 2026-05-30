'use client';

import React, { useEffect, useState } from 'react';
import { getBillingAccount, createCheckoutSession, createPortalSession, type BillingAccount, type BillingCurrency } from '@/features/billing/api';
import { log } from '@/lib/observability/log';

interface BillingPanelProps {
  accountId: string;
}

export function BillingPanel({ accountId }: BillingPanelProps) {
  const [account, setAccount] = useState<BillingAccount | null>(null);
  const [currency, setCurrency] = useState<BillingCurrency>('jpy');
  const [pendingAction, setPendingAction] = useState<'checkout' | 'portal' | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getBillingAccount(accountId).then(setAccount).catch((err) => {
      log.error('Failed to load billing account', { source: 'billing_load_account', accountId }, err);
      setError('課金情報を取得できませんでした。');
    });
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
    } catch (err) {
      log.error('Checkout failed', { source: 'billing_checkout', accountId }, err);
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
    } catch (err) {
      log.error('Billing portal failed', { source: 'billing_portal', accountId }, err);
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

      <div className="mt-4 flex items-center justify-center gap-1.5 opacity-30 grayscale transition-opacity hover:opacity-100 hover:grayscale-0">
        <span className="text-[9px] font-bold uppercase tracking-widest text-stone-400">Powered by</span>
        <svg className="h-3 w-auto fill-[#635BFF]" viewBox="0 0 60 25" xmlns="http://www.w3.org/2000/svg">
          <path d="M59.64 14.28c0-4.59-2.29-6.87-6.55-6.87-4.23 0-6.87 2.4-6.87 6.87 0 5.1 2.85 7 6.87 7 1.95 0 3.6-.45 4.95-1.11v-3.12c-1.29.6-2.49.84-3.87.84-1.68 0-2.82-.42-3-1.89h10.41c.03-.27.06-.57.06-.85zm-8.4-2.22c0-1.2.69-1.92 1.83-1.92 1.11 0 1.77.72 1.77 1.92h-3.6zM39.18 10.41c0-1.74-1.23-2.73-3.24-2.73-1.68 0-3.3.45-4.59 1.23l1.41 2.46c1.08-.6 2.07-.9 2.91-.9.63 0 1.05.21 1.05.69 0 .48-.45.69-1.41.93-2.31.57-4.71 1.14-4.71 3.72 0 2.22 1.74 3.75 4.14 3.75 1.74 0 2.85-.69 3.63-1.47l.15 1.2h3V10.41c0-3.15-2.22-4.5-5.22-4.5-1.89 0-3.75.54-5.25 1.47l1.32 2.37c1.17-.66 2.37-1.02 3.63-1.02 1.11 0 1.62.36 1.62.96 0 .54-.42.75-1.56 1.05-2.19.57-4.2 1.14-4.2 3.42 0 1.5 1.17 2.49 2.7 2.49.96 0 1.95-.48 2.61-1.17V10.41zM23.16 8.37h3V21h-3v-1.41c-.78.84-1.98 1.41-3.6 1.41-3.21 0-5.61-2.4-5.61-6.87 0-4.59 2.4-6.87 5.61-6.87 1.56 0 2.82.57 3.6 1.41V8.37zm-3.3 9.48c1.71 0 2.67-1.05 2.67-2.61V14.1c0-1.56-.96-2.61-2.67-2.61-1.71 0-2.67 1.05-2.67 2.61v1.14c0 1.56.96 2.61 2.67 2.61zM11.67 5.25c0-1.14-.9-2.1-2.07-2.1-1.14 0-2.07.96-2.07 2.1 0 1.14.93 2.1 2.07 2.1 1.17 0 2.07-.96 2.07-2.1zm-3.54 3.12h3V21h-3V8.37zM0 8.37h3v1.32c.78-.84 1.98-1.32 3.6-1.32 3.21 0 5.61 2.4 5.61 6.87 0 4.59-2.4 6.87-5.61 6.87-1.56 0-2.82-.57-3.6-1.41V21H0V8.37zm3.3 9.48c1.71 0 2.67-1.05 2.67-2.61V14.1c0-1.56-.96-2.61-2.67-2.61-1.71 0-2.67 1.05-2.67 2.61v1.14c0 1.56.96 2.61 2.67 2.61z" />
        </svg>
      </div>
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

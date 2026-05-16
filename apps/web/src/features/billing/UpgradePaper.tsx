'use client';

import { useEffect, useState } from 'react';
import { createCheckoutSession, type BillingCurrency } from './api';

interface UpgradePaperProps {
  accountId: string;
}

export function UpgradePaper({ accountId }: UpgradePaperProps) {
  const [currency, setCurrency] = useState<BillingCurrency>('jpy');
  const [url, setUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    createCheckoutSession(accountId, currency)
      .then((nextUrl) => {
        if (!cancelled) {
          setUrl(nextUrl);
          setError(null);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setUrl(null);
          setError('決済URLを取得できませんでした。');
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [accountId, currency]);

  return (
    <div style={{ display: 'grid', gap: 12 }}>
      <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
        Pro プランにアップグレードするとストレージ上限が拡張され、大規模ドキュメントの処理が可能になります。
      </p>

      <div style={{ display: 'flex', overflow: 'hidden', borderRadius: 6, border: '1px solid var(--link-border)', width: 'fit-content' }}>
        {(['jpy', 'usd'] as BillingCurrency[]).map((c) => (
          <button
            key={c}
            type="button"
            onClick={() => setCurrency(c)}
            style={{
              padding: '4px 12px',
              fontSize: '0.75rem',
              fontWeight: 700,
              textTransform: 'uppercase',
              cursor: 'pointer',
              border: 'none',
              background: currency === c ? 'var(--accent)' : 'transparent',
              color: currency === c ? '#fff' : 'var(--muted)',
              transition: 'background 0.15s',
            }}
          >
            {c}
          </button>
        ))}
      </div>

      {loading && <p style={{ margin: 0, fontSize: '0.8rem', color: 'var(--muted)' }}>読み込み中...</p>}
      {error && <p style={{ margin: 0, fontSize: '0.8rem', color: '#f87171' }}>{error}</p>}
      {url && (
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          style={{
            display: 'inline-block',
            padding: '8px 16px',
            borderRadius: 6,
            background: 'var(--accent)',
            color: '#fff',
            fontSize: '0.85rem',
            fontWeight: 600,
            textDecoration: 'none',
            width: 'fit-content',
          }}
        >
          Stripe で決済する →
        </a>
      )}
    </div>
  );
}

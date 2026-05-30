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
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
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
          <svg className="h-4 w-auto opacity-50 transition-opacity hover:opacity-100" viewBox="0 0 60 25" fill="#635BFF" xmlns="http://www.w3.org/2000/svg" style={{ height: '18px' }}>
            <path d="M59.64 14.28c0-4.59-2.29-6.87-6.55-6.87-4.23 0-6.87 2.4-6.87 6.87 0 5.1 2.85 7 6.87 7 1.95 0 3.6-.45 4.95-1.11v-3.12c-1.29.6-2.49.84-3.87.84-1.68 0-2.82-.42-3-1.89h10.41c.03-.27.06-.57.06-.85zm-8.4-2.22c0-1.2.69-1.92 1.83-1.92 1.11 0 1.77.72 1.77 1.92h-3.6zM39.18 10.41c0-1.74-1.23-2.73-3.24-2.73-1.68 0-3.3.45-4.59 1.23l1.41 2.46c1.08-.6 2.07-.9 2.91-.9.63 0 1.05.21 1.05.69 0 .48-.45.69-1.41.93-2.31.57-4.71 1.14-4.71 3.72 0 2.22 1.74 3.75 4.14 3.75 1.74 0 2.85-.69 3.63-1.47l.15 1.2h3V10.41c0-3.15-2.22-4.5-5.22-4.5-1.89 0-3.75.54-5.25 1.47l1.32 2.37c1.17-.66 2.37-1.02 3.63-1.02 1.11 0 1.62.36 1.62.96 0 .54-.42.75-1.56 1.05-2.19.57-4.2 1.14-4.2 3.42 0 1.5 1.17 2.49 2.7 2.49.96 0 1.95-.48 2.61-1.17V10.41zM23.16 8.37h3V21h-3v-1.41c-.78.84-1.98 1.41-3.6 1.41-3.21 0-5.61-2.4-5.61-6.87 0-4.59 2.4-6.87 5.61-6.87 1.56 0 2.82.57 3.6 1.41V8.37zm-3.3 9.48c1.71 0 2.67-1.05 2.67-2.61V14.1c0-1.56-.96-2.61-2.67-2.61-1.71 0-2.67 1.05-2.67 2.61v1.14c0 1.56.96 2.61 2.67 2.61zM11.67 5.25c0-1.14-.9-2.1-2.07-2.1-1.14 0-2.07.96-2.07 2.1 0 1.14.93 2.1 2.07 2.1 1.17 0 2.07-.96 2.07-2.1zm-3.54 3.12h3V21h-3V8.37zM0 8.37h3v1.32c.78-.84 1.98-1.32 3.6-1.32 3.21 0 5.61-2.4 5.61-6.87 0-4.59-2.4 6.87-5.61 6.87-1.56 0-2.82-.57-3.6-1.41V21H0V8.37zm3.3 9.48c1.71 0 2.67-1.05 2.67-2.61V14.1c0-1.56-.96-2.61-2.67-2.61-1.71 0-2.67 1.05-2.67 2.61v1.14c0 1.56.96 2.61 2.67 2.61z" />
          </svg>
        </div>
      )}
    </div>
  );
}

'use client';

import { useEffect, useState } from 'react';
import { createPortalSession } from './api';

interface ManagePaperProps {
  accountId: string;
}

export function ManagePaper({ accountId }: ManagePaperProps) {
  const [url, setUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    createPortalSession(accountId)
      .then(setUrl)
      .catch(() => setError('管理画面URLを取得できませんでした。'))
      .finally(() => setLoading(false));
  }, [accountId]);

  return (
    <div style={{ display: 'grid', gap: 12 }}>
      <p style={{ margin: 0, fontSize: '0.85rem', lineHeight: 1.65 }}>
        Stripe カスタマーポータルでサブスクリプションの変更・キャンセル・請求書の確認ができます。
      </p>

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
            border: '1px solid var(--link-border)',
            background: 'var(--link-bg)',
            color: 'var(--accent)',
            fontSize: '0.85rem',
            fontWeight: 600,
            textDecoration: 'none',
            width: 'fit-content',
          }}
        >
          Stripe ポータルを開く →
        </a>
      )}
    </div>
  );
}

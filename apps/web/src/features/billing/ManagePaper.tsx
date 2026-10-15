
import { useEffect, useState } from 'react';
import { ConnectError, Code } from '@connectrpc/connect';
import { createPortalSession } from './api';
import { formatConnectErrorMessage, isExpectedConnectError, UserFacingError } from '@/lib/errors';

/**
 * Props for the Stripe customer portal management paper.
 */
interface ManagePaperProps {
  /** Target account identifier. */
  accountId: string;
}

/**
 * Paper component providing a direct link out to the Stripe Customer Portal for subscription management.
 *
 * @param props - Account identifier used to create the customer portal session.
 * @returns Management paper with loading/error handling and the portal link action.
 */
export function ManagePaper({ accountId }: ManagePaperProps) {
  const [url, setUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    createPortalSession(accountId)
      .then(setUrl)
      .catch((err) => {
        if (err instanceof UserFacingError) {
          setError(err.message);
          return;
        }
        if (err instanceof ConnectError) {
          if (err.code === Code.FailedPrecondition) {
            setError(err.rawMessage || '決済プロバイダが設定されていません。');
            return;
          }
          if (err.code === Code.PermissionDenied) {
            setError('管理画面にアクセスする権限がありません。');
            return;
          }
          if (err.code === Code.NotFound) {
            setError('請求アカウントが見つかりませんでした。');
            return;
          }
          if (err.code === Code.Unavailable || err.code === Code.DeadlineExceeded) {
            setError('通信環境を確認して、再度お試しください。');
            return;
          }
          if (isExpectedConnectError(err)) {
            setError(formatConnectErrorMessage(err, '決済管理画面の取得に失敗しました。'));
            return;
          }
        }
        console.error('Unexpected UI error', err);
        setError('決済管理画面の取得に失敗しました。');
      })
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

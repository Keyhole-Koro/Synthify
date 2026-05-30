'use client';

import { useEffect, useState } from 'react';
import { createPortalSession, listInvoices, listPaymentMethods, type InvoiceList, type PaymentMethodList } from './api';

interface InvoicePaperProps {
  accountId: string;
}

interface InvoiceState {
  accountId: string;
  invoices: InvoiceList | null;
  methods: PaymentMethodList | null;
  portalUrl: string | null;
  loading: boolean;
  error: string | null;
}

export function InvoicePaper({ accountId }: InvoicePaperProps) {
  const [state, setState] = useState<InvoiceState>({
    accountId,
    invoices: null,
    methods: null,
    portalUrl: null,
    loading: true,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;

    Promise.all([
      listInvoices(accountId),
      listPaymentMethods(accountId),
      createPortalSession(accountId).catch(() => null),
    ])
      .then(([invoices, methods, portalUrl]) => {
        if (!cancelled) {
          setState({
            accountId,
            invoices,
            methods,
            portalUrl,
            loading: false,
            error: null,
          });
        }
      })
      .catch(() => {
        if (!cancelled) {
          setState({
            accountId,
            invoices: null,
            methods: null,
            portalUrl: null,
            loading: false,
            error: '請求情報を取得できませんでした。',
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountId]);

  if (state.accountId !== accountId || state.loading) {
    return <p style={{ margin: 0, fontSize: '0.8rem', color: 'var(--muted)' }}>読み込み中...</p>;
  }
  if (state.error) {
    return <p style={{ margin: 0, fontSize: '0.8rem', color: '#f87171' }}>{state.error}</p>;
  }

  const upcoming = Number(state.invoices?.upcomingAmount) || 0;
  const upcomingEnd = state.invoices?.upcomingPeriodEnd ?? '';
  const defaultMethod = state.methods?.paymentMethods.find((m) => m.isDefault) ?? state.methods?.paymentMethods[0];

  return (
    <div style={{ display: 'grid', gap: 18 }}>
      <Section title="今月の請求予定">
        {upcoming > 0 ? (
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
            <span style={{ fontSize: '1.4rem', fontWeight: 700 }}>
              ${upcoming.toFixed(2)}
            </span>
            {upcomingEnd && (
              <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>
                {upcomingEnd} に確定
              </span>
            )}
          </div>
        ) : (
          <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--muted)' }}>請求予定はありません。</p>
        )}
      </Section>

      <Section title="支払い方法">
        {defaultMethod ? (
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 10px', border: '1px solid var(--link-border)', borderRadius: 6, background: 'var(--surface-alt)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
              <div style={{ width: 32, height: 20, borderRadius: 3, background: '#fff', border: '1px solid var(--link-border)', display: 'flex', alignItems: 'center', justifyItems: 'center', overflow: 'hidden' }}>
                <PaymentBrandIcon brand={defaultMethod.brand} />
              </div>
              <div>
                <p style={{ margin: 0, fontSize: '0.9rem', fontWeight: 600 }}>
                  {defaultMethod.brand || 'Card'} •••• {defaultMethod.last4 || '----'}
                </p>
                <p style={{ margin: '2px 0 0', fontSize: '0.7rem', color: 'var(--muted)' }}>
                  有効期限 {String(defaultMethod.expMonth).padStart(2, '0')}/{String(defaultMethod.expYear).slice(-2)}
                </p>
              </div>
            </div>
            {state.portalUrl && (
              <a
                href={state.portalUrl}
                target="_blank"
                rel="noopener noreferrer"
                style={{ fontSize: '0.75rem', color: 'var(--accent)', textDecoration: 'none', fontWeight: 600 }}
              >
                変更 →
              </a>
            )}
          </div>
        ) : (
          <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--muted)' }}>
            支払い方法は登録されていません。{state.portalUrl && (
              <a href={state.portalUrl} target="_blank" rel="noopener noreferrer" style={{ color: 'var(--accent)', textDecoration: 'none', fontWeight: 600 }}>
                Stripe で追加 →
              </a>
            )}
          </p>
        )}
      </Section>

      <Section title="請求履歴">
        {state.invoices && state.invoices.invoices.length > 0 ? (
          <table style={{ width: '100%', fontSize: '0.8rem', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ background: 'var(--surface-alt)' }}>
                <th style={{ padding: '6px 8px', textAlign: 'left', fontWeight: 600 }}>請求日</th>
                <th style={{ padding: '6px 8px', textAlign: 'right', fontWeight: 600 }}>金額</th>
                <th style={{ padding: '6px 8px', textAlign: 'right', fontWeight: 600 }}>状態</th>
              </tr>
            </thead>
            <tbody>
              {state.invoices.invoices.map((row) => {
                const date = (row.paidAt || row.createdAt || '').slice(0, 10);
                const amount = Number(row.amount) || 0;
                const linkUrl = row.hostedInvoiceUrl || row.invoicePdfUrl;
                return (
                  <tr key={row.invoiceId} style={{ borderBottom: '1px solid var(--link-border)' }}>
                    <td style={{ padding: '6px 8px' }}>
                      {linkUrl ? (
                        <a href={linkUrl} target="_blank" rel="noopener noreferrer" style={{ color: 'inherit', textDecoration: 'underline' }}>
                          {date}
                        </a>
                      ) : (
                        date
                      )}
                    </td>
                    <td style={{ padding: '6px 8px', textAlign: 'right' }}>${amount.toFixed(2)}</td>
                    <td style={{ padding: '6px 8px', textAlign: 'right', color: row.status === 'paid' ? '#10b981' : 'var(--muted)' }}>
                      {row.status}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        ) : (
          <p style={{ margin: 0, fontSize: '0.85rem', color: 'var(--muted)' }}>請求履歴はまだありません。</p>
        )}
        {state.portalUrl && (
          <a
            href={state.portalUrl}
            target="_blank"
            rel="noopener noreferrer"
            style={{ display: 'inline-block', marginTop: 8, fontSize: '0.75rem', color: 'var(--accent)', textDecoration: 'none', fontWeight: 600 }}
          >
            Stripe で全請求書を確認 →
          </a>
        )}
      </Section>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p style={{ margin: '0 0 6px', fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--muted)', fontWeight: 700 }}>
        {title}
      </p>
      {children}
    </div>
  );
}

function PaymentBrandIcon({ brand }: { brand?: string }) {
  const b = (brand || '').toLowerCase();
  if (b === 'visa') {
    return (
      <svg viewBox="0 0 24 15" fill="none" xmlns="http://www.w3.org/2000/svg" style={{ width: '100%', height: 'auto', padding: '2px' }}>
        <path d="M11.5 0.5H15L13 14.5H9.5L11.5 0.5Z" fill="#1A1F71" />
        <path d="M21.5 0.5C20.5 0.5 19.5 1 19 2L16 14.5H19.5L20.5 11H24L24.5 14.5H28L25.5 0.5H21.5ZM21.5 8.5L22.5 4L23.5 8.5H21.5Z" fill="#1A1F71" />
        <path d="M0 0.5L3.5 14.5H7L3.5 0.5H0Z" fill="#1A1F71" />
        <path d="M6 0.5L4 10.5C4 12 5 13 6.5 13H10L10.5 9.5H8.5C7.5 9.5 7 9 7 8L8.5 0.5H6Z" fill="#1A1F71" />
      </svg>
    );
  }
  if (b === 'mastercard') {
    return (
      <svg viewBox="0 0 24 15" fill="none" xmlns="http://www.w3.org/2000/svg" style={{ width: '100%', height: 'auto', padding: '2px' }}>
        <circle cx="7" cy="7.5" r="7" fill="#EB001B" fillOpacity="0.8" />
        <circle cx="15" cy="7.5" r="7" fill="#F79E1B" fillOpacity="0.8" />
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 24 15" fill="none" xmlns="http://www.w3.org/2000/svg" style={{ width: '100%', height: 'auto', padding: '4px', opacity: 0.3 }}>
      <rect width="24" height="15" rx="2" fill="var(--muted)" />
    </svg>
  );
}

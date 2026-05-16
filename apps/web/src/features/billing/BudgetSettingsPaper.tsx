'use client';

import { useEffect, useState } from 'react';
import { getBillingAccount, updateBudget } from './api';

interface BudgetSettingsPaperProps {
  accountId: string;
}

interface BudgetState {
  accountId: string;
  budget: string;
  loading: boolean;
  error: string | null;
}

export function BudgetSettingsPaper({ accountId }: BudgetSettingsPaperProps) {
  const [state, setState] = useState<BudgetState>({
    accountId,
    budget: '',
    loading: true,
    error: null,
  });
  const [alert80, setAlert80] = useState(true);
  const [alert100, setAlert100] = useState(true);
  const [saving, setSaving] = useState(false);
  const [savedAt, setSavedAt] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;

    getBillingAccount(accountId)
      .then((account) => {
        if (!cancelled) {
          setState({
            accountId,
            budget: account.budgetLimit || '0',
            loading: false,
            error: null,
          });
        }
      })
      .catch(() => {
        if (!cancelled) {
          setState({
            accountId,
            budget: '0',
            loading: false,
            error: '予算情報を取得できませんでした。',
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [accountId]);

  const loading = state.accountId !== accountId || state.loading;

  async function handleSave() {
    setSaving(true);
    setState((current) => ({ ...current, error: null }));
    try {
      const normalized = state.budget.trim() === '' ? '0' : state.budget.trim();
      const result = await updateBudget(accountId, normalized);
      setState((current) => ({ ...current, budget: result }));
      setSavedAt(Date.now());
      setTimeout(() => setSavedAt(null), 2000);
    } catch {
      setState((current) => ({ ...current, error: '保存に失敗しました。' }));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={{ display: 'grid', gap: 14 }}>
      <p style={{ margin: 0, fontSize: '0.8rem', lineHeight: 1.6, color: 'var(--muted)' }}>
        月次予算を設定すると、上限に近づいた段階で通知が届き、上限到達時は処理中ジョブを途中成果物にまとめて終了します。
        0 を指定すると無制限になります。
      </p>

      <div>
        <label style={{ display: 'block', fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--muted)', fontWeight: 700, marginBottom: 4 }}>
          月次上限 (USD)
        </label>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span style={{ fontSize: '0.9rem' }}>$</span>
          <input
            type="number"
            min={0}
            step={1}
            value={state.budget}
            disabled={loading || saving}
            onChange={(e) => setState((current) => ({ ...current, budget: e.target.value }))}
            style={{
              flex: 1,
              padding: '6px 10px',
              border: '1px solid var(--link-border)',
              borderRadius: 4,
              background: 'var(--surface-alt)',
              fontSize: '0.9rem',
            }}
          />
        </div>
      </div>

      <div style={{ display: 'grid', gap: 8 }}>
        <label style={{ display: 'block', fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.08em', color: 'var(--muted)', fontWeight: 700 }}>
          アラート
        </label>
        <Checkbox
          checked={alert80}
          onChange={setAlert80}
          label="80% 到達でメール通知"
        />
        <Checkbox
          checked={alert100}
          onChange={setAlert100}
          label="100% 到達で通知 + ジョブの途中打ち切り"
        />
        <p style={{ margin: 0, fontSize: '0.65rem', color: 'var(--muted)', fontStyle: 'italic' }}>
          ※ アラート設定はサーバー側ポリシーで一律有効。表示のみのトグルです。
        </p>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <button
          type="button"
          onClick={handleSave}
          disabled={loading || saving}
          style={{
            padding: '6px 14px',
            borderRadius: 4,
            background: 'var(--accent)',
            color: '#fff',
            fontSize: '0.8rem',
            fontWeight: 600,
            border: 'none',
            cursor: loading || saving ? 'wait' : 'pointer',
            opacity: loading || saving ? 0.5 : 1,
          }}
        >
          {saving ? '保存中...' : '保存'}
        </button>
        {savedAt && <span style={{ fontSize: '0.75rem', color: 'var(--muted)' }}>保存しました</span>}
        {state.error && <span style={{ fontSize: '0.75rem', color: '#f87171' }}>{state.error}</span>}
      </div>
    </div>
  );
}

function Checkbox({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label: string }) {
  return (
    <label style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: '0.85rem', cursor: 'pointer' }}>
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span>{label}</span>
    </label>
  );
}

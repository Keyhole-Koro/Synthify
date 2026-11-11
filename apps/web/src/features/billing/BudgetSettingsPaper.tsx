import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ConnectError, Code } from '@connectrpc/connect';
import { updateBudget } from './api';
import { billingAccountQueryOptions, billingAccountQueryKey } from './queries';
import { log } from '@/lib/observability/log';

/**
 * Props for the budget settings configuration paper.
 */
interface BudgetSettingsPaperProps {
  /** Target account ID to configure budget for. */
  accountId: string;
}

/**
 * Paper component allowing users to set a monthly spending cap and configure budget threshold alerts.
 *
 * @param props - Account identifier whose budget settings are edited.
 * @returns Budget configuration paper with validation, saving, and server-error states.
 */
export function BudgetSettingsPaper({ accountId }: BudgetSettingsPaperProps) {
  const queryClient = useQueryClient();
  const { data: account, isLoading, error: queryError } = useQuery(billingAccountQueryOptions(accountId));
  const [budgetInput, setBudgetInput] = useState<string | null>(null);
  const [prevAccountId, setPrevAccountId] = useState(accountId);
  if (prevAccountId !== accountId) {
    setPrevAccountId(accountId);
    setBudgetInput(null);
  }

  const budget = budgetInput ?? (account?.budgetLimit || '0');
  const [saveError, setSaveError] = useState<string | null>(null);
  const [alert80, setAlert80] = useState(true);
  const [alert100, setAlert100] = useState(true);
  const [savedAt, setSavedAt] = useState<number | null>(null);

  const mutation = useMutation({
    mutationFn: async (newBudget: string) => {
      const normalized = newBudget.trim() === '' ? '0' : newBudget.trim();
      return updateBudget(accountId, normalized);
    },
    onSuccess: (result) => {
      setBudgetInput(result);
      setSaveError(null);
      setSavedAt(Date.now());
      setTimeout(() => setSavedAt(null), 2000);
      queryClient.invalidateQueries({ queryKey: billingAccountQueryKey(accountId) });
    },
    onError: (err) => {
      log.error('Failed to save budget', { source: 'budget_save', accountId }, err);
      if (err instanceof ConnectError) {
        if (err.code === Code.InvalidArgument) {
          setSaveError(err.rawMessage || '有効な予算金額を入力してください。');
          return;
        }
        if (err.code === Code.PermissionDenied) {
          setSaveError('予算を変更する権限がありません。');
          return;
        }
        if (err.code === Code.NotFound) {
          setSaveError('請求アカウントが見つかりませんでした。');
          return;
        }
        if (err.code === Code.Unavailable || err.code === Code.DeadlineExceeded) {
          setSaveError('通信環境を確認して、再度お試しください。');
          return;
        }
        if (err.rawMessage) {
          setSaveError(err.rawMessage);
          return;
        }
      }
      setSaveError('保存に失敗しました。');
    },
  });

  const saving = mutation.isPending;
  const loading = isLoading;
  const error = saveError || (queryError ? '予算情報を取得できませんでした。' : null);

  function handleSave() {
    const trimmed = budget.trim();
    if (trimmed !== '' && (Number.isNaN(Number(trimmed)) || Number(trimmed) < 0)) {
      setSaveError('有効な数値を入力してください。');
      return;
    }
    setSaveError(null);
    mutation.mutate(trimmed === '' ? '0' : trimmed);
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
            value={budget}
            disabled={loading || saving}
            onChange={(e) => setBudgetInput(e.target.value)}
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
        {error && <span style={{ fontSize: '0.75rem', color: '#f87171' }}>{error}</span>}
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

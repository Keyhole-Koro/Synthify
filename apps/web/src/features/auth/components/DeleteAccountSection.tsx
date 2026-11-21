import React, { useState } from 'react';
import { deleteAccount } from '@/features/auth/userApi';
import { InlineError } from '@/components/error/InlineError';
import { isAppError } from '@/lib/errors';

interface DeleteAccountSectionProps {
  /** Called once the account is gone; the caller signs out. */
  onDeleted: () => void;
}

// DeleteAccountSection deletes the signed-in user's account (退会) after an
// inline confirmation, like the workspace delete button.
export function DeleteAccountSection({ onDeleted }: DeleteAccountSectionProps) {
  const [open, setOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Keep clicks from reaching the paper, as the logout button does.
  function stop(e: React.PointerEvent) { e.stopPropagation(); }

  async function handleDelete() {
    setDeleting(true);
    setError(null);
    try {
      await deleteAccount();
      onDeleted();
    } catch (err) {
      setError(deleteAccountErrorMessage(err));
      setDeleting(false);
    }
  }

  if (!open) {
    return (
      <button
        type="button"
        onPointerDown={stop} onPointerUp={stop}
        onClick={() => setOpen(true)}
        className="self-start text-[11px] text-stone-400 underline-offset-2 hover:text-red-500 hover:underline"
      >
        アカウントを削除（退会）
      </button>
    );
  }

  return (
    <div role="group" aria-label="アカウント削除の確認" className="rounded-lg bg-red-50 p-3">
      <p className="text-[11px] leading-relaxed text-red-700">
        アカウントを削除します。自分のワークスペースと、その中の文書・ツリー・共有リンクはすべて消え、元に戻せません。共有しているメンバーも見られなくなります。ほかの人のワークスペースからは抜けます（上げた文書はそのワークスペースに残ります）。
      </p>
      {error && <InlineError message={error} className="mt-2" />}
      <div className="mt-2 flex justify-end gap-2">
        <button
          type="button"
          disabled={deleting}
          onPointerDown={stop} onPointerUp={stop}
          onClick={() => {
            setOpen(false);
            setError(null);
          }}
          className="h-7 rounded-md border border-stone-200 bg-white px-2.5 text-[12px] text-stone-500 disabled:opacity-60"
        >
          キャンセル
        </button>
        <button
          type="button"
          disabled={deleting}
          onPointerDown={stop} onPointerUp={stop}
          onClick={() => void handleDelete()}
          className="h-7 rounded-md border border-red-200 bg-white px-2.5 text-[12px] font-medium text-red-600 disabled:opacity-60"
        >
          {deleting ? '削除中…' : '退会する'}
        </button>
      </div>
    </div>
  );
}

function deleteAccountErrorMessage(err: unknown): string {
  if (isAppError(err) && err.code === 'FailedPrecondition') {
    if (err.message.includes('paid plan')) {
      return '有料プランの契約が続いています。先に「プラン・課金」から解約してください。';
    }
    return '処理中のドキュメントがあります。終わってからもう一度試してください。';
  }
  return 'アカウントの削除に失敗しました。時間をおいてもう一度試してください。';
}

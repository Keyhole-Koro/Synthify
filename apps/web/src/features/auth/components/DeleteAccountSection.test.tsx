import { ConnectError } from '@connectrpc/connect';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createAppError } from '@/lib/errors';
import { DeleteAccountSection } from './DeleteAccountSection';

// What a client call rejects with: the AppError that lib/connect.ts builds,
// wrapped by connect-es in a ConnectError with code Unknown.
function refused(message: string) {
  return ConnectError.from(createAppError({ kind: 'validation', code: 'FailedPrecondition', message }));
}

const api = vi.hoisted(() => ({ deleteAccount: vi.fn() }));

vi.mock('@/features/auth/userApi', () => ({ deleteAccount: api.deleteAccount }));

function openConfirmation() {
  const onDeleted = vi.fn();
  render(<DeleteAccountSection onDeleted={onDeleted} />);
  fireEvent.click(screen.getByRole('button', { name: 'アカウントを削除（退会）' }));
  return { onDeleted };
}

describe('DeleteAccountSection', () => {
  // A block body: a function returned from beforeEach would run as teardown.
  beforeEach(() => {
    api.deleteAccount.mockReset();
  });

  it('asks for confirmation before deleting', () => {
    openConfirmation();
    expect(screen.getByRole('group', { name: 'アカウント削除の確認' })).toBeTruthy();
    expect(screen.getByText(/元に戻せません/)).toBeTruthy();
    expect(api.deleteAccount).not.toHaveBeenCalled();
  });

  it('can be cancelled', () => {
    openConfirmation();
    fireEvent.click(screen.getByRole('button', { name: 'キャンセル' }));
    expect(screen.queryByRole('group', { name: 'アカウント削除の確認' })).toBeNull();
    expect(api.deleteAccount).not.toHaveBeenCalled();
  });

  it('deletes the account and hands over to sign-out', async () => {
    api.deleteAccount.mockResolvedValue(undefined);
    const { onDeleted } = openConfirmation();
    fireEvent.click(screen.getByRole('button', { name: '退会する' }));
    await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(1));
    expect(api.deleteAccount).toHaveBeenCalledTimes(1);
  });

  it('explains a refusal for a paid plan and stays signed in', async () => {
    api.deleteAccount.mockRejectedValue(refused('account has a paid plan that still renews'));
    const { onDeleted } = openConfirmation();
    fireEvent.click(screen.getByRole('button', { name: '退会する' }));
    expect(await screen.findByText(/先に「プラン・課金」から解約してください/)).toBeTruthy();
    expect(onDeleted).not.toHaveBeenCalled();
  });

  it('explains a refusal while a document is processing', async () => {
    api.deleteAccount.mockRejectedValue(refused('document is still being uploaded or processed'));
    openConfirmation();
    fireEvent.click(screen.getByRole('button', { name: '退会する' }));
    expect(await screen.findByText(/処理中のドキュメントがあります/)).toBeTruthy();
  });
});

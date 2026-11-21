import { describe, it, expect } from 'vitest';
import { formatAuthErrorMessage, toAuthFailure } from './authError';

describe('formatAuthErrorMessage', () => {
  it('maps known Firebase Auth codes to friendly messages', () => {
    expect(formatAuthErrorMessage({ code: 'auth/user-not-found' })).toBe('メールアドレスまたはパスワードが正しくありません。');
    expect(formatAuthErrorMessage({ code: 'auth/wrong-password' })).toBe('メールアドレスまたはパスワードが正しくありません。');
    expect(formatAuthErrorMessage({ code: 'auth/invalid-email' })).toBe('無効なメールアドレス形式です。');
    expect(formatAuthErrorMessage({ code: 'auth/network-request-failed' })).toBe('ネットワークエラーが発生しました。通信環境を確認してください。');
  });

  it('falls back to error message if present', () => {
    expect(formatAuthErrorMessage(new Error('Firebase network glitch'))).toBe('Firebase network glitch');
  });

  it('falls back to default message for unknown errors', () => {
    expect(formatAuthErrorMessage({})).toBe('認証エラーが発生しました。');
  });
});

describe('toAuthFailure', () => {
  it('creates structured AuthFailure object', () => {
    const failure = toAuthFailure({ code: 'auth/user-not-found' });
    expect(failure.code).toBe('auth/user-not-found');
    expect(failure.message).toBe('メールアドレスまたはパスワードが正しくありません。');
    expect(failure.retryable).toBe(false);
  });

  it('marks network errors as retryable', () => {
    const failure = toAuthFailure({ code: 'auth/network-request-failed' });
    expect(failure.retryable).toBe(true);
  });
});

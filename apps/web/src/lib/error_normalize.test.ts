import { describe, it, expect } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import { toAppError } from './error_normalize';
import { createAppError } from './errors';

describe('toAppError', () => {
  it('returns the same error if it is already an AppError', () => {
    const original = createAppError({ kind: 'auth', message: 'Existing' });
    const result = toAppError(original);
    expect(result).toBe(original);
  });

  describe('ConnectError handling', () => {
    it('maps Unauthenticated to auth kind', () => {
      const err = new ConnectError('Unauthorized', Code.Unauthenticated);
      const result = toAppError(err);
      expect(result.kind).toBe('auth');
      expect(result.retryable).toBe(false);
    });

    it('maps Unavailable to unavailable kind and makes it retryable', () => {
      const err = new ConnectError('Server down', Code.Unavailable);
      const result = toAppError(err);
      expect(result.kind).toBe('unavailable');
      expect(result.retryable).toBe(true);
    });

    it('maps InvalidArgument to validation kind', () => {
      const err = new ConnectError('Bad input', Code.InvalidArgument);
      const result = toAppError(err);
      expect(result.kind).toBe('validation');
    });
  });

  describe('Firebase Error handling', () => {
    it('maps auth/user-not-found to localized message', () => {
      const err = { code: 'auth/user-not-found', message: 'Original' };
      const result = toAppError(err);
      expect(result.kind).toBe('auth');
      expect(result.message).toBe('メールアドレスまたはパスワードが正しくありません。');
    });

    it('maps auth/network-request-failed to retryable network error', () => {
      const err = { code: 'auth/network-request-failed' };
      const result = toAppError(err);
      expect(result.kind).toBe('auth');
      expect(result.retryable).toBe(true);
    });

    it('handles permission-denied', () => {
      const err = { code: 'permission-denied' };
      const result = toAppError(err);
      expect(result.kind).toBe('auth');
      expect(result.message).toBe('アクセス権限がありません。');
    });

    it('handles FILE_TOO_LARGE', () => {
      const err = { code: 'FILE_TOO_LARGE' };
      const result = toAppError(err);
      expect(result.kind).toBe('validation');
    });
  });

  describe('Network Error handling', () => {
    it('handles Failed to fetch (TypeError)', () => {
      const err = new TypeError('Failed to fetch');
      const result = toAppError(err);
      expect(result.kind).toBe('network');
      expect(result.retryable).toBe(true);
      expect(result.message).toContain('サーバーに接続できません');
    });
  });

  describe('Fallback handling', () => {
    it('handles generic Error object', () => {
      const err = new Error('Something went wrong');
      const result = toAppError(err);
      expect(result.kind).toBe('unknown');
      expect(result.message).toBe('Something went wrong');
    });

    it('handles string errors', () => {
      const result = toAppError('Raw string error');
      expect(result.kind).toBe('unknown');
      expect(result.message).toBe('Raw string error');
    });
  });
});

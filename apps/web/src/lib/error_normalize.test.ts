import { describe, it, expect } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import { errorFromJson } from '@connectrpc/connect/protocol-connect';
import { toAppError } from './error_normalize';
import { createAppError, isIntakeClosedError } from './errors';

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

    it('localizes unsupported document type errors', () => {
      const err = new ConnectError('unsupported document type', Code.InvalidArgument);
      const result = toAppError(err);
      expect(result.message).toContain('実行ファイル形式はアップロードできません');
    });

    // The body the API sends while SYNTHIFY_INTAKE_CLOSED is on: the detail is
    // synthify.app.v1.ErrorDetail{reason: ERROR_REASON_INTAKE_CLOSED}.
    it('recognises the intake-closed refusal and keeps its message', () => {
      const err = errorFromJson(
        {
          code: 'failed_precondition',
          message: '新しい文書の受け付けは終了しました。',
          details: [{ type: 'synthify.app.v1.ErrorDetail', value: 'CAE' }],
        },
        undefined,
        new ConnectError('fallback', Code.Unknown),
      );
      const result = toAppError(err);
      expect(result.reason).toBe('intake_closed');
      expect(isIntakeClosedError(result)).toBe(true);
      expect(result.code).toBe('FailedPrecondition');
      expect(result.message).toBe('新しい文書の受け付けは終了しました。');
      expect(result.retryable).toBe(false);
    });

    // lib/connect.ts turns the response into an AppError, and connect-es hands
    // the caller that AppError wrapped in a ConnectError with code Unknown.
    it('keeps the intake-closed reason through the transport and connect-es', () => {
      const fromTransport = toAppError(errorFromJson(
        {
          code: 'failed_precondition',
          message: '有料プランの受け付けは終了しました。',
          details: [{ type: 'synthify.app.v1.ErrorDetail', value: 'CAE' }],
        },
        undefined,
        new ConnectError('fallback', Code.Unknown),
      ));
      const wrapped = ConnectError.from(fromTransport);
      expect(wrapped.code).toBe(Code.Unknown);

      const result = toAppError(wrapped);
      expect(isIntakeClosedError(result)).toBe(true);
      expect(result.message).toBe('有料プランの受け付けは終了しました。');
    });

    it('does not take other failed preconditions for closed intake', () => {
      const err = new ConnectError('document is still being uploaded or processed', Code.FailedPrecondition);
      const result = toAppError(err);
      expect(result.reason).toBeUndefined();
      expect(isIntakeClosedError(result)).toBe(false);
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

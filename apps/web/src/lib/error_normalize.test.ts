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

    it('does not mistake a ConnectError with an unrelated cause for a wrapped AppError', () => {
      const err = ConnectError.from(new Error('socket hang up'));
      const result = toAppError(err);
      expect(result.kind).toBe('unknown');
      expect(result.code).toBe('Unknown');
      expect(result.message).toBe('socket hang up');
    });

    it('does not take other failed preconditions for closed intake', () => {
      const err = new ConnectError('document is still being uploaded or processed', Code.FailedPrecondition);
      const result = toAppError(err);
      expect(result.reason).toBeUndefined();
      expect(isIntakeClosedError(result)).toBe(false);
    });
  });

  // lib/connect.ts turns every failed call into an AppError, and connect-es
  // hands the caller that AppError wrapped in a ConnectError with code Unknown
  // (runUnaryCall rejects with ConnectError.from(reason)). What callers
  // inspect must be the AppError, not the wrapper.
  describe('an RPC error as the caller receives it', () => {
    // What the transport builds from the API's error body.
    function fromTransport(code: string, message: string, details: { type: string; value: string }[] = []) {
      return toAppError(errorFromJson({ code, message, details }, undefined, new ConnectError('fallback', Code.Unknown)));
    }

    it('is the AppError the transport built', () => {
      const appError = fromTransport('failed_precondition', 'account has a paid plan that still renews');
      const wrapped = ConnectError.from(appError);
      expect(wrapped.code).toBe(Code.Unknown);
      expect(wrapped.message).toBe('[unknown] account has a paid plan that still renews');

      const result = toAppError(wrapped);
      expect(result).toBe(appError);
      expect(result.kind).toBe('validation');
      expect(result.code).toBe('FailedPrecondition');
      expect(result.message).toBe('account has a paid plan that still renews');
    });

    it('keeps not found, which the share link viewer looks for', () => {
      const result = toAppError(ConnectError.from(fromTransport('not_found', 'not found')));
      expect(result.kind).toBe('not_found');
      expect(result.code).toBe('NotFound');
    });

    it('stays retryable when the server is unavailable', () => {
      const result = toAppError(ConnectError.from(fromTransport('unavailable', 'upstream connect error')));
      expect(result.kind).toBe('unavailable');
      expect(result.retryable).toBe(true);
    });

    it('keeps the intake-closed reason', () => {
      const result = toAppError(ConnectError.from(fromTransport(
        'failed_precondition',
        '有料プランの受け付けは終了しました。',
        [{ type: 'synthify.app.v1.ErrorDetail', value: 'CAE' }],
      )));
      expect(isIntakeClosedError(result)).toBe(true);
      expect(result.message).toBe('有料プランの受け付けは終了しました。');
    });

    it('keeps a network failure retryable', () => {
      const network = toAppError(new TypeError('Failed to fetch'));
      const result = toAppError(ConnectError.from(network));
      expect(result.kind).toBe('network');
      expect(result.retryable).toBe(true);
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

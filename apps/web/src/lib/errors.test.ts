import { describe, it, expect } from 'vitest';
import { ConnectError, Code } from '@connectrpc/connect';
import {
  isExpectedConnectError,
  isRetryableError,
  isAuthError,
  formatConnectErrorMessage,
  EXPECTED_CONNECT_CODES,
  UserFacingError,
} from './errors';

describe('isExpectedConnectError', () => {
  it('returns true for expected ConnectError codes', () => {
    for (const code of EXPECTED_CONNECT_CODES) {
      const err = new ConnectError('Expected error', code);
      expect(isExpectedConnectError(err)).toBe(true);
    }
  });

  it('returns false for fatal/unimplemented ConnectError codes', () => {
    expect(isExpectedConnectError(new ConnectError('Internal', Code.Internal))).toBe(false);
    expect(isExpectedConnectError(new ConnectError('Unimplemented', Code.Unimplemented))).toBe(false);
    expect(isExpectedConnectError(new ConnectError('Unknown', Code.Unknown))).toBe(false);
    expect(isExpectedConnectError(new ConnectError('DataLoss', Code.DataLoss))).toBe(false);
  });

  it('returns false for non-ConnectError values', () => {
    expect(isExpectedConnectError(new Error('Normal error'))).toBe(false);
    expect(isExpectedConnectError('string error')).toBe(false);
    expect(isExpectedConnectError(null)).toBe(false);
    expect(isExpectedConnectError({ code: 3 })).toBe(false);
  });
});

describe('isRetryableError', () => {
  it('returns true for Unavailable, DeadlineExceeded, ResourceExhausted', () => {
    expect(isRetryableError(new ConnectError('Down', Code.Unavailable))).toBe(true);
    expect(isRetryableError(new ConnectError('Timeout', Code.DeadlineExceeded))).toBe(true);
    expect(isRetryableError(new ConnectError('Rate limit', Code.ResourceExhausted))).toBe(true);
  });

  it('returns false for non-retryable errors', () => {
    expect(isRetryableError(new ConnectError('Bad input', Code.InvalidArgument))).toBe(false);
    expect(isRetryableError(new ConnectError('Forbidden', Code.PermissionDenied))).toBe(false);
    expect(isRetryableError(new Error('Other'))).toBe(false);
  });
});

describe('isAuthError', () => {
  it('returns true for Unauthenticated and PermissionDenied', () => {
    expect(isAuthError(new ConnectError('Login required', Code.Unauthenticated))).toBe(true);
    expect(isAuthError(new ConnectError('Access denied', Code.PermissionDenied))).toBe(true);
  });

  it('returns false for other ConnectError codes', () => {
    expect(isAuthError(new ConnectError('Bad input', Code.InvalidArgument))).toBe(false);
    expect(isAuthError(new ConnectError('Not found', Code.NotFound))).toBe(false);
    expect(isAuthError(new ConnectError('Internal server error', Code.Internal))).toBe(false);
  });

  it('returns false for non-ConnectError values', () => {
    expect(isAuthError(new Error('Auth failed'))).toBe(false);
    expect(isAuthError('Unauthenticated')).toBe(false);
    expect(isAuthError(null)).toBe(false);
    expect(isAuthError(undefined)).toBe(false);
  });
});

describe('formatConnectErrorMessage', () => {
  it('prefers rawMessage if present', () => {
    const err = new ConnectError('カスタムメッセージ', Code.InvalidArgument);
    expect(formatConnectErrorMessage(err)).toBe('カスタムメッセージ');
  });

  it('provides Japanese defaults when rawMessage is empty', () => {
    const err = new ConnectError('', Code.PermissionDenied);
    expect(formatConnectErrorMessage(err)).toBe('この操作を行う権限がありません。');

    const notFound = new ConnectError('', Code.NotFound);
    expect(formatConnectErrorMessage(notFound)).toBe('対象のリソースが見つかりません。');
  });
});

describe('UserFacingError', () => {
  it('keeps the explicitly approved display message in message', () => {
    expect(new UserFacingError('ファイル形式が不正です。').message).toBe('ファイル形式が不正です。');
  });
});

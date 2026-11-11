import { ConnectError, Code } from '@connectrpc/connect';

/**
 * ConnectRPC error codes that are expected during normal UI usage and should be presented inline.
 * Unrecoverable system codes (Internal, Unimplemented, DataLoss, Unknown) are omitted so they
 * propagate up to the root error boundary (FatalErrorView).
 */
export const EXPECTED_CONNECT_CODES = new Set<Code>([
  Code.InvalidArgument,
  Code.AlreadyExists,
  Code.PermissionDenied,
  Code.Unauthenticated,
  Code.NotFound,
  Code.ResourceExhausted,
  Code.FailedPrecondition,
  Code.Unavailable,
  Code.DeadlineExceeded,
  Code.Canceled,
]);

/**
 * Determines whether an unknown error is an expected ConnectError suitable for UI inline handling.
 *
 * @param err - Error candidate.
 * @returns True if err is an expected ConnectError.
 */
export function isExpectedConnectError(err: unknown): err is ConnectError {
  return err instanceof ConnectError && EXPECTED_CONNECT_CODES.has(err.code);
}

/**
 * Determines whether an error is transient and retryable with exponential backoff.
 *
 * @param err - Error candidate.
 * @returns True if error is transient (Unavailable, DeadlineExceeded, ResourceExhausted).
 */
export function isRetryableError(err: unknown): boolean {
  if (err instanceof ConnectError) {
    return (
      err.code === Code.Unavailable ||
      err.code === Code.DeadlineExceeded ||
      err.code === Code.ResourceExhausted
    );
  }
  return false;
}

/**
 * Checks whether an error is a ConnectRPC unauthenticated or permission denied error.
 *
 * @param err - Error candidate.
 * @returns True if error is auth-related (Unauthenticated or PermissionDenied).
 */
export function isAuthError(err: unknown): err is ConnectError {
  if (err instanceof ConnectError) {
    return err.code === Code.Unauthenticated || err.code === Code.PermissionDenied;
  }
  return false;
}

/**
 * Returns a user-friendly error message corresponding to a ConnectError code.
 *
 * @param err - The ConnectError instance.
 * @param fallback - Fallback string if no specific message matches.
 * @returns Localized user error string.
 */
export function formatConnectErrorMessage(err: ConnectError, fallback = '処理に失敗しました。'): string {
  // Prefer the specific rawMessage sent by the server if available
  if (err.rawMessage && err.rawMessage.trim().length > 0) {
    return err.rawMessage;
  }

  switch (err.code) {
    case Code.InvalidArgument:
      return '入力内容が正しくありません。';
    case Code.AlreadyExists:
      return 'すでに存在しています。';
    case Code.PermissionDenied:
      return 'この操作を行う権限がありません。';
    case Code.Unauthenticated:
      return 'ログインが必要です。再ログインしてください。';
    case Code.NotFound:
      return '対象のリソースが見つかりません。';
    case Code.ResourceExhausted:
      return 'リクエストの上限に達しました。しばらく時間をおいて再度お試しください。';
    case Code.FailedPrecondition:
      return '前提条件が満たされていないため、操作を完了できませんでした。';
    case Code.Unavailable:
      return 'サービスが一時的に利用できません。通信環境を確認してください。';
    case Code.DeadlineExceeded:
      return '通信がタイムアウトしました。通信環境を確認してください。';
    case Code.Canceled:
      return '処理がキャンセルされました。';
    default:
      return fallback;
  }
}

/**
 * Error type intentionally thrown to display safe messages to the end user.
 */
export class UserFacingError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'UserFacingError';
  }
}

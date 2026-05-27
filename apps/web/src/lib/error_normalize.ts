import { ConnectError, Code } from '@connectrpc/connect';
import { type AppError, type AppErrorKind, createAppError, isAppError } from './errors';

/**
 * Normalizes various error types (Connect, Firebase, Fetch) into a unified AppError.
 */
export function toAppError(err: unknown): AppError {
  if (isAppError(err)) return err;

  // Handle ConnectError
  if (err instanceof ConnectError) {
    const kind = mapConnectCodeToKind(err.code);
    return createAppError({
      kind,
      message: err.rawMessage,
      retryable: isConnectCodeRetryable(err.code),
      code: Code[err.code],
      cause: err,
    });
  }

  // Handle Firebase Error (often has a 'code' property)
  if (typeof err === 'object' && err !== null && 'code' in err) {
    const code = (err as { code: string }).code;
    const message = (err as { message?: string }).message || 'Authentication error';
    
    if (code.startsWith('auth/')) {
      return createAppError({
        kind: 'auth',
        message: mapFirebaseErrorMessage(code, message),
        retryable: code === 'auth/network-request-failed',
        code,
        cause: err,
      });
    }

    if (code.startsWith('permission-denied')) {
      return createAppError({
        kind: 'auth',
        message: 'アクセス権限がありません。',
        retryable: false,
        code,
        cause: err,
      });
    }

    if (code === 'FILE_TOO_LARGE') {
      return createAppError({
        kind: 'validation',
        message: 'ファイルサイズが大きすぎます。',
        retryable: false,
        code,
        cause: err,
      });
    }

    if (code === 'UPLOAD_FAILED') {
      const status = (err as { status?: number }).status;
      let message = 'アップロードに失敗しました。';
      if (status === 403) message = 'アップロード権限がないか、有効期限が切れています。';
      return createAppError({
        kind: status && status >= 500 ? 'server' : 'unknown',
        message,
        retryable: !status || status >= 500,
        code,
        status,
        cause: err,
      });
    }
  }

  // Handle Fetch/Network errors
  if (err instanceof TypeError && err.message === 'Failed to fetch') {
    return createAppError({
      kind: 'network',
      message: 'サーバーに接続できません。ネットワーク設定を確認してください。',
      retryable: true,
      code: 'NETWORK_ERROR',
      cause: err,
    });
  }

  // Fallback for generic errors
  const message = err instanceof Error ? err.message : String(err);
  return createAppError({
    kind: 'unknown',
    message: message || '予期しないエラーが発生しました。',
    retryable: false,
    cause: err,
  });
}

function mapConnectCodeToKind(code: Code): AppErrorKind {
  switch (code) {
    case Code.Unauthenticated:
    case Code.PermissionDenied:
      return 'auth';
    case Code.NotFound:
      return 'not_found';
    case Code.InvalidArgument:
    case Code.AlreadyExists:
    case Code.FailedPrecondition:
    case Code.OutOfRange:
      return 'validation';
    case Code.Unavailable:
    case Code.Aborted:
      return 'unavailable';
    case Code.DeadlineExceeded:
      return 'network';
    case Code.Internal:
    case Code.Unimplemented:
    case Code.DataLoss:
      return 'server';
    default:
      return 'unknown';
  }
}

function isConnectCodeRetryable(code: Code): boolean {
  return (
    code === Code.Unavailable ||
    code === Code.Aborted ||
    code === Code.DeadlineExceeded ||
    code === Code.ResourceExhausted
  );
}

function mapFirebaseErrorMessage(code: string, originalMessage: string): string {
  switch (code) {
    case 'auth/user-not-found':
    case 'auth/wrong-password':
      return 'メールアドレスまたはパスワードが正しくありません。';
    case 'auth/invalid-email':
      return '無効なメールアドレス形式です。';
    case 'auth/user-disabled':
      return 'このアカウントは無効化されています。';
    case 'auth/too-many-requests':
      return '短時間に何度も失敗したため、一時的にブロックされています。しばらく時間をおいてから再度お試しください。';
    case 'auth/network-request-failed':
      return 'ネットワークエラーが発生しました。通信環境を確認してください。';
    case 'auth/popup-closed-by-user':
      return 'ログイン画面が閉じられました。';
    case 'auth/unauthorized-domain':
      return 'このドメインからのログインは許可されていません。';
    default:
      return originalMessage;
  }
}

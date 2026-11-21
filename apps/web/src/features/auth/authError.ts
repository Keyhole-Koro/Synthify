/**
 * Structured representation of an authentication failure with retryability classification.
 *
 * @property code Provider or application error code.
 * @property message Localized, user-facing explanation of the failure.
 * @property retryable Whether retrying the same operation may succeed without changing input.
 * @property cause Original caught value retained for diagnostics, when available.
 */
export interface AuthFailure {
  code: string;
  message: string;
  retryable: boolean;
  cause?: unknown;
}

/**
 * Translates Firebase Auth error codes into friendly user error messages.
 * Used exclusively on login/signin forms and not leaked into global error normalization.
 *
 * @param err - Unknown error candidate.
 * @returns Localized user-facing error message.
 */
export function formatAuthErrorMessage(err: unknown): string {
  if (typeof err === 'object' && err !== null && 'code' in err) {
    const code = String((err as { code: unknown }).code);
    switch (code) {
      case 'auth/user-not-found':
      case 'auth/wrong-password':
      case 'auth/invalid-credential':
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
    }
  }

  if (err instanceof Error && err.message) {
    return err.message;
  }

  return '認証エラーが発生しました。';
}

/**
 * Normalizes an unknown caught error into a structured AuthFailure.
 *
 * @param err - Caught error.
 * @returns An AuthFailure object.
 */
export function toAuthFailure(err: unknown): AuthFailure {
  const code =
    typeof err === 'object' && err !== null && 'code' in err
      ? String((err as { code: unknown }).code)
      : 'unknown';
  return {
    code,
    message: formatAuthErrorMessage(err),
    retryable: code === 'auth/network-request-failed',
    cause: err,
  };
}

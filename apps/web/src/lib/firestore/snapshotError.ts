import { type AppError } from '@/lib/errors';
import { toAppError } from '@/lib/error_normalize';

export type ClassifiedSnapshotError =
  | { kind: 'transient' }
  | { kind: 'fatal'; error: AppError };

// Firestore onSnapshot は permission-denied / unavailable / cancelled などで
// 発火するが、これらは SDK が auth 復帰やネットワーク復旧で勝手に再接続する。
// 呼び出し側に渡しても actionable でないため transient として握りつぶし、
// それ以外だけ AppError に正規化して上に返す。
export function classifyFirestoreSnapshotError(err: unknown): ClassifiedSnapshotError {
  const code = typeof err === 'object' && err !== null && 'code' in err
    ? String((err as { code: unknown }).code)
    : '';

  if (code === 'permission-denied' || code === 'unavailable' || code === 'cancelled') {
    return { kind: 'transient' };
  }
  return { kind: 'fatal', error: toAppError(err) };
}

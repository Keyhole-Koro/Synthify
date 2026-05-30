import { type AppError } from '@/lib/errors';
import { toAppError } from '@/lib/error_normalize';
import { noticeBrowserError, recordBrowserPageAction } from '@/lib/newrelic/browser';

export type ClassifiedSnapshotError =
  | { kind: 'transient' }
  | { kind: 'fatal'; error: AppError };

function firestoreErrorCode(err: unknown): string {
  return typeof err === 'object' && err !== null && 'code' in err
    ? String((err as { code: unknown }).code)
    : '';
}

// Firestore onSnapshot は permission-denied / unavailable / cancelled などで
// 発火する。これらは UI に出しても actionable でないため transient として
// 握りつぶし、それ以外だけ AppError に正規化して上に返す。
//
// 注意: permission-denied は SDK が自動再接続しない（リスナーが恒久終了する）。
// UI には出さないが、ルール/デプロイのバグを示すので観測はしたい。判定そのもの
// は純粋に保ち、観測は reportSnapshotError 側に分離している。
export function classifyFirestoreSnapshotError(err: unknown): ClassifiedSnapshotError {
  const code = firestoreErrorCode(err);

  if (code === 'permission-denied' || code === 'unavailable' || code === 'cancelled') {
    return { kind: 'transient' };
  }
  return { kind: 'fatal', error: toAppError(err) };
}

// reportSnapshotError は分類結果に応じて観測チャネルを振り分ける。
// - 共通: dev では console.warn（本番 console はユーザー端末に出るだけで我々は
//   見られないため dev 限定）
// - transient: PageAction で送る（握りつぶすが本番でも気づけるように。noticeError
//   ではないので JS エラー率/アラートは汚さない）
// - fatal: UI にも出るが、アラート対象として noticeError でも拾う
export function reportSnapshotError(
  err: unknown,
  classified: ClassifiedSnapshotError,
  context: { label?: string } = {},
) {
  const code = firestoreErrorCode(err);
  const message = err instanceof Error ? err.message : String(err);
  const label = context.label ?? null;

  if (process.env.NODE_ENV !== 'production') {
    console.warn(`[firestore] snapshot ${classified.kind} error`, { code, label, err });
  }

  if (classified.kind === 'transient') {
    recordBrowserPageAction('firestore_snapshot_swallowed', { code, label, message });
    return;
  }

  noticeBrowserError(err instanceof Error ? err : new Error(message), {
    source: 'firestore_snapshot',
    code,
    label,
  });
}

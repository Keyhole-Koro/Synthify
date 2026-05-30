import { type AppError } from '@/lib/errors';
import { toAppError } from '@/lib/error_normalize';
import { log } from '@/lib/observability/log';

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

// reportSnapshotError は分類結果に応じて観測レベルを振り分ける（log.ts 経由で
// NR Logs + console。console レベルは env.observability.consoleLevel で制御）。
// - transient: UI からは隠すが warn として残す（permission-denied 等のルール
//   不整合に気づけるように。error ではないので JS エラー率は汚さない）
// - fatal: UI にも出る handled error として error レベルで残す
export function reportSnapshotError(
  err: unknown,
  classified: ClassifiedSnapshotError,
  context: { label?: string } = {},
) {
  const attributes = {
    source: 'firestore_snapshot',
    code: firestoreErrorCode(err),
    label: context.label ?? null,
  };

  if (classified.kind === 'transient') {
    log.warn('firestore snapshot error swallowed', attributes);
    return;
  }
  log.error('firestore snapshot error', attributes, err);
}

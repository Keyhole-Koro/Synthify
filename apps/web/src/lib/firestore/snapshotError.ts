import type { FirestoreError } from 'firebase/firestore';
import { UserFacingError } from '@/lib/errors';
import { log } from '@/lib/observability/log';

/**
 * Firestore codes that onSnapshot reports for conditions the user cannot act on:
 * a dropped connection, a listener torn down during sign-out, or a rules
 * mismatch. They are logged but kept off the screen.
 */
const TRANSIENT_CODES = new Set<FirestoreError['code']>(['permission-denied', 'unavailable', 'cancelled']);

/**
 * Logs a snapshot listener error and returns what the UI should show.
 *
 * permission-denied ends the listener for good (the SDK does not reconnect), so
 * it is still logged at warn level: it points at a rules or deploy problem.
 *
 * @param err - Error passed to the onSnapshot error callback.
 * @param label - Which subscription failed, for the log.
 * @returns null for transient errors, otherwise an error with a Japanese message.
 */
export function handleSnapshotError(err: FirestoreError, label: string): UserFacingError | null {
  const attributes = { source: 'firestore_snapshot', code: err.code, label };
  if (TRANSIENT_CODES.has(err.code)) {
    log.warn('Firestore snapshot error hidden from the user', attributes);
    return null;
  }
  log.error('Firestore snapshot failed', attributes, err);
  return new UserFacingError('最新の状態を取得できませんでした。ページを再読み込みしてください。', { cause: err });
}

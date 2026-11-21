import { afterEach, describe, expect, it, vi } from 'vitest';
import type { FirestoreError } from 'firebase/firestore';
import { UserFacingError } from '@/lib/errors';
import { handleSnapshotError } from './snapshotError';

// The real logger validates env vars on import and forwards to New Relic.
const log = vi.hoisted(() => ({ warn: vi.fn(), error: vi.fn() }));
vi.mock('@/lib/observability/log', () => ({ log }));

// FirestoreError's constructor is private; the handler only reads code and message.
function firestoreError(code: FirestoreError['code'], message: string) {
  return Object.assign(new Error(message), { code, name: 'FirebaseError' }) as unknown as FirestoreError;
}

afterEach(() => {
  vi.clearAllMocks();
});

describe('handleSnapshotError', () => {
  it.each(['permission-denied', 'unavailable', 'cancelled'] as const)(
    'logs %s as a warning and keeps it off the screen',
    (code) => {
      const { warn, error } = log;

      expect(handleSnapshotError(firestoreError(code, 'Missing or insufficient permissions.'), 'job_status')).toBeNull();
      expect(warn).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({ code, label: 'job_status' }));
      expect(error).not.toHaveBeenCalled();
    },
  );

  it('logs other errors and returns a Japanese message instead of the raw Firestore text', () => {
    const { error } = log;
    const raw = firestoreError('internal', 'INTERNAL ASSERTION FAILED');

    const shown = handleSnapshotError(raw, 'workspace_jobs');

    expect(shown).toBeInstanceOf(UserFacingError);
    expect(shown?.message).toBe('最新の状態を取得できませんでした。ページを再読み込みしてください。');
    expect(shown?.cause).toBe(raw);
    expect(error).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({ code: 'internal' }), raw);
  });
});

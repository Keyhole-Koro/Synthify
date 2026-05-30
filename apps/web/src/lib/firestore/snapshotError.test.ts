import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('@/lib/observability/log', () => ({
  log: { debug: vi.fn(), info: vi.fn(), warn: vi.fn(), error: vi.fn() },
}));

import { classifyFirestoreSnapshotError, reportSnapshotError } from './snapshotError';
import { log } from '@/lib/observability/log';

describe('classifyFirestoreSnapshotError', () => {
  it.each(['permission-denied', 'unavailable', 'cancelled'])('treats %s as transient', (code) => {
    expect(classifyFirestoreSnapshotError({ code }).kind).toBe('transient');
  });

  it('treats other codes as fatal with a normalized AppError', () => {
    const result = classifyFirestoreSnapshotError({ code: 'not-found', message: 'nope' });
    expect(result.kind).toBe('fatal');
    if (result.kind === 'fatal') expect(result.error).toBeDefined();
  });

  it('treats errors without a code as fatal', () => {
    expect(classifyFirestoreSnapshotError('boom').kind).toBe('fatal');
  });
});

describe('reportSnapshotError', () => {
  beforeEach(() => vi.clearAllMocks());

  it('logs a transient error as warn (never error)', () => {
    const err = { code: 'permission-denied' };
    reportSnapshotError(err, classifyFirestoreSnapshotError(err), { label: 'workspace_jobs' });
    expect(log.warn).toHaveBeenCalledWith('firestore snapshot error swallowed', {
      source: 'firestore_snapshot',
      code: 'permission-denied',
      label: 'workspace_jobs',
    });
    expect(log.error).not.toHaveBeenCalled();
  });

  it('logs a fatal error as error with the original error and a null label by default', () => {
    const err = { code: 'internal', message: 'x' };
    reportSnapshotError(err, classifyFirestoreSnapshotError(err));
    expect(log.error).toHaveBeenCalledWith(
      'firestore snapshot error',
      { source: 'firestore_snapshot', code: 'internal', label: null },
      err,
    );
    expect(log.warn).not.toHaveBeenCalled();
  });
});

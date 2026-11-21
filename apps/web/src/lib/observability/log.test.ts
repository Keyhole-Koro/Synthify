import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { LogLevel } from '@/config/env';

const recordBrowserLog = vi.fn();

// log.ts captures console refs at module-eval time, so console must be spied
// before importing it. We re-import per test (resetModules) and mock the NR helper.
async function loadLog() {
  vi.resetModules();
  recordBrowserLog.mockClear();
  vi.doMock('@/lib/newrelic/browser', () => ({ recordBrowserLog }));
  return (await import('@/lib/observability/log')).log;
}

describe('log', () => {
  let spies: Record<LogLevel, ReturnType<typeof vi.spyOn>>;

  beforeEach(() => {
    spies = {
      debug: vi.spyOn(console, 'debug').mockImplementation(() => {}),
      info: vi.spyOn(console, 'info').mockImplementation(() => {}),
      warn: vi.spyOn(console, 'warn').mockImplementation(() => {}),
      error: vi.spyOn(console, 'error').mockImplementation(() => {}),
    };
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.resetModules();
  });

  it('delivers every level to New Relic but mirrors only warnings and errors', async () => {
    const log = await loadLog();
    log.debug('hello');
    log.info('quiet');
    log.warn('loud');
    log.error('failed');
    expect(recordBrowserLog).toHaveBeenCalledWith('debug', 'hello', {});
    expect(recordBrowserLog).toHaveBeenCalledWith('info', 'quiet', {});
    expect(recordBrowserLog).toHaveBeenCalledWith('warn', 'loud', {});
    expect(recordBrowserLog).toHaveBeenCalledWith('error', 'failed', {});
    expect(spies.debug).not.toHaveBeenCalled();
    expect(spies.info).not.toHaveBeenCalled();
    expect(spies.warn).toHaveBeenCalled();
    expect(spies.error).toHaveBeenCalled();
  });

  it('error(message, Error) attaches error attributes', async () => {
    const log = await loadLog();
    log.error('failed', new Error('boom'));
    expect(recordBrowserLog).toHaveBeenCalledWith(
      'error',
      'failed',
      expect.objectContaining({ errorName: 'Error', errorMessage: 'boom', errorStack: expect.any(String) }),
    );
  });

  it('error(message, attributes, Error) merges both', async () => {
    const log = await loadLog();
    log.error('failed', { source: 's' }, new Error('boom'));
    expect(recordBrowserLog).toHaveBeenCalledWith(
      'error',
      'failed',
      expect.objectContaining({ source: 's', errorName: 'Error', errorMessage: 'boom' }),
    );
  });

  it('error(message) with no error sends empty attributes', async () => {
    const log = await loadLog();
    log.error('failed');
    expect(recordBrowserLog).toHaveBeenCalledWith('error', 'failed', {});
  });

  it('error(message, non-Error) records errorValue', async () => {
    const log = await loadLog();
    log.error('failed', 'stringy');
    expect(recordBrowserLog).toHaveBeenCalledWith('error', 'failed', { errorValue: 'stringy' });
  });
});

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type { LogLevel } from '@/config/env';

const recordBrowserLog = vi.fn();

// log.ts captures console refs at module-eval time, so console must be spied
// before importing it. We re-import per test (resetModules) to vary the
// env-derived consoleLevel, mocking env + the NR helper.
async function loadLog(consoleLevel: LogLevel) {
  vi.resetModules();
  recordBrowserLog.mockClear();
  vi.doMock('@/config/env', () => ({ env: { observability: { consoleLevel } } }));
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

  it('delivers to New Relic even when the level is below the console threshold', async () => {
    const log = await loadLog('error');
    log.debug('hello');
    expect(recordBrowserLog).toHaveBeenCalledWith('debug', 'hello', {});
    expect(spies.debug).not.toHaveBeenCalled();
  });

  it('gates the console by level but always delivers to NR', async () => {
    const log = await loadLog('warn');
    log.info('quiet');
    log.warn('loud');
    expect(spies.info).not.toHaveBeenCalled();
    expect(spies.warn).toHaveBeenCalled();
    expect(recordBrowserLog).toHaveBeenCalledWith('info', 'quiet', {});
    expect(recordBrowserLog).toHaveBeenCalledWith('warn', 'loud', {});
  });

  it('error(message, Error) attaches error attributes', async () => {
    const log = await loadLog('error');
    log.error('failed', new Error('boom'));
    expect(recordBrowserLog).toHaveBeenCalledWith(
      'error',
      'failed',
      expect.objectContaining({ errorName: 'Error', errorMessage: 'boom', errorStack: expect.any(String) }),
    );
  });

  it('error(message, attributes, Error) merges both', async () => {
    const log = await loadLog('error');
    log.error('failed', { source: 's' }, new Error('boom'));
    expect(recordBrowserLog).toHaveBeenCalledWith(
      'error',
      'failed',
      expect.objectContaining({ source: 's', errorName: 'Error', errorMessage: 'boom' }),
    );
  });

  it('error(message) with no error sends empty attributes', async () => {
    const log = await loadLog('error');
    log.error('failed');
    expect(recordBrowserLog).toHaveBeenCalledWith('error', 'failed', {});
  });

  it('error(message, non-Error) records errorValue', async () => {
    const log = await loadLog('error');
    log.error('failed', 'stringy');
    expect(recordBrowserLog).toHaveBeenCalledWith('error', 'failed', { errorValue: 'stringy' });
  });
});

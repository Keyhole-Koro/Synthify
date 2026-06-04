import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useExponentialBackoff } from './useExponentialBackoff';

describe('useExponentialBackoff', () => {
  async function advanceTimers(ms: number) {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(ms);
    });
  }

  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('does nothing when there is no error', () => {
    const onRetry = vi.fn();
    const { result } = renderHook(() =>
      useExponentialBackoff({ onRetry, error: null })
    );

    expect(result.current.retryCount).toBe(0);
    expect(result.current.nextRetryInSeconds).toBe(null);
    expect(onRetry).not.toHaveBeenCalled();
  });

  it('triggers onRetry after initial delay when error occurs', async () => {
    const onRetry = vi.fn();
    const { result } = renderHook(() =>
      useExponentialBackoff({ onRetry, error: new Error('test'), initialDelayMs: 1000 })
    );

    expect(result.current.nextRetryInSeconds).toBe(1);

    await advanceTimers(1000);

    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(result.current.retryCount).toBe(1);
  });

  it('increases delay exponentially on subsequent errors', async () => {
    const onRetry = vi.fn();
    const { result, rerender } = renderHook(
      ({ err }) => useExponentialBackoff({ onRetry, error: err, initialDelayMs: 1000 }),
      { initialProps: { err: new Error('test') } }
    );

    // 1st retry: 1s
    await advanceTimers(1000);
    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(result.current.retryCount).toBe(1);

    // Rerender with same error to trigger next retry logic
    rerender({ err: new Error('test') });

    // 2nd retry: 2s
    expect(result.current.nextRetryInSeconds).toBe(2);
    await advanceTimers(2000);
    expect(onRetry).toHaveBeenCalledTimes(2);
    expect(result.current.retryCount).toBe(2);

    // 3rd retry: 4s
    rerender({ err: new Error('test') });
    expect(result.current.nextRetryInSeconds).toBe(4);
    await advanceTimers(4000);
    expect(onRetry).toHaveBeenCalledTimes(3);
    expect(result.current.retryCount).toBe(3);
  });

  it('caps the delay at maxDelayMs', async () => {
    const onRetry = vi.fn();
    const { result, rerender } = renderHook(
      ({ err }) =>
        useExponentialBackoff({
          onRetry,
          error: err,
          initialDelayMs: 1000,
          maxDelayMs: 3000,
        }),
      { initialProps: { err: new Error('test') } }
    );

    // 1st: 1s
    await advanceTimers(1000);
    rerender({ err: new Error('test') });

    // 2nd: 2s
    await advanceTimers(2000);
    rerender({ err: new Error('test') });

    // 3rd: 3s (capped from 4s)
    expect(result.current.nextRetryInSeconds).toBe(3);
    await advanceTimers(3000);
    expect(onRetry).toHaveBeenCalledTimes(3);
  });

  it('resets retry count when error is cleared', async () => {
    const onRetry = vi.fn();
    const { result, rerender } = renderHook(
      ({ err }) => useExponentialBackoff({ onRetry, error: err, initialDelayMs: 1000 }),
      { initialProps: { err: new Error('test') as Error | null } }
    );

    await advanceTimers(1000);
    expect(result.current.retryCount).toBe(1);

    rerender({ err: null });
    expect(result.current.retryCount).toBe(0);
    expect(result.current.nextRetryInSeconds).toBe(null);
  });
});

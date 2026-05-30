import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useExponentialBackoff } from './useExponentialBackoff';

describe('useExponentialBackoff', () => {
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

  it('triggers onRetry after initial delay when error occurs', () => {
    const onRetry = vi.fn();
    const { result } = renderHook(() =>
      useExponentialBackoff({ onRetry, error: new Error('test'), initialDelayMs: 1000 })
    );

    expect(result.current.nextRetryInSeconds).toBe(1);

    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(result.current.retryCount).toBe(1);
  });

  it('increases delay exponentially on subsequent errors', () => {
    const onRetry = vi.fn();
    const { result, rerender } = renderHook(
      ({ err }) => useExponentialBackoff({ onRetry, error: err, initialDelayMs: 1000 }),
      { initialProps: { err: new Error('test') } }
    );

    // 1st retry: 1s
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(result.current.retryCount).toBe(1);

    // Rerender with same error to trigger next retry logic
    rerender({ err: new Error('test') });

    // 2nd retry: 2s
    expect(result.current.nextRetryInSeconds).toBe(2);
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(onRetry).toHaveBeenCalledTimes(2);
    expect(result.current.retryCount).toBe(2);

    // 3rd retry: 4s
    rerender({ err: new Error('test') });
    expect(result.current.nextRetryInSeconds).toBe(4);
    act(() => {
      vi.advanceTimersByTime(4000);
    });
    expect(onRetry).toHaveBeenCalledTimes(3);
    expect(result.current.retryCount).toBe(3);
  });

  it('caps the delay at maxDelayMs', () => {
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
    act(() => vi.advanceTimersByTime(1000));
    rerender({ err: new Error('test') });

    // 2nd: 2s
    act(() => vi.advanceTimersByTime(2000));
    rerender({ err: new Error('test') });

    // 3rd: 3s (capped from 4s)
    expect(result.current.nextRetryInSeconds).toBe(3);
    act(() => vi.advanceTimersByTime(3000));
    expect(onRetry).toHaveBeenCalledTimes(3);
  });

  it('resets retry count when error is cleared', () => {
    const onRetry = vi.fn();
    const { result, rerender } = renderHook(
      ({ err }) => useExponentialBackoff({ onRetry, error: err, initialDelayMs: 1000 }),
      { initialProps: { err: new Error('test') as Error | null } }
    );

    act(() => vi.advanceTimersByTime(1000));
    expect(result.current.retryCount).toBe(1);

    rerender({ err: null });
    expect(result.current.retryCount).toBe(0);
    expect(result.current.nextRetryInSeconds).toBe(null);
  });
});

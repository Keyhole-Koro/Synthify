import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useElapsedTime } from './useElapsedTime';

describe('useElapsedTime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-06-02T00:00:10Z')); // 10s after start below
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  const startedAt = '2026-06-02T00:00:00Z';

  it('returns null until startedAt is known', () => {
    const { result } = renderHook(() => useElapsedTime(undefined, undefined, false));
    expect(result.current).toBeNull();
  });

  it('formats elapsed as M:SS from startedAt while running', () => {
    const { result } = renderHook(() => useElapsedTime(startedAt, undefined, false));
    expect(result.current).toBe('0:10');
  });

  it('ticks every second while running', () => {
    const { result } = renderHook(() => useElapsedTime(startedAt, undefined, false));
    expect(result.current).toBe('0:10');
    act(() => {
      vi.advanceTimersByTime(5000);
    });
    expect(result.current).toBe('0:15');
  });

  it('freezes at completedAt - startedAt when completed', () => {
    const completedAt = '2026-06-02T00:01:23Z'; // 1m23s after start
    const { result } = renderHook(() => useElapsedTime(startedAt, completedAt, true));
    expect(result.current).toBe('1:23');
    act(() => {
      vi.advanceTimersByTime(10000);
    });
    expect(result.current).toBe('1:23'); // does not advance
  });
});

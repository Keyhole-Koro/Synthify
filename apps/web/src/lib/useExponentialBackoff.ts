'use client';

import { useEffect, useState, useCallback, useRef } from 'react';

interface UseExponentialBackoffArgs {
  onRetry: () => void;
  error: any;
  initialDelayMs?: number;
  maxDelayMs?: number;
}

export function useExponentialBackoff({
  onRetry,
  error,
  initialDelayMs = 2000,
  maxDelayMs = 32000,
}: UseExponentialBackoffArgs) {
  const [retryCount, setRetryCount] = useState(0);
  const [nextRetryInMs, setNextRetryInMs] = useState<number | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const countdownRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const clearTimers = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
    if (countdownRef.current) {
      clearInterval(countdownRef.current);
      countdownRef.current = null;
    }
  }, []);

  useEffect(() => {
    if (!error) {
      setRetryCount(0);
      setNextRetryInMs(null);
      clearTimers();
      return;
    }

    const delay = Math.min(initialDelayMs * Math.pow(2, retryCount), maxDelayMs);
    setNextRetryInMs(delay);

    timerRef.current = setTimeout(() => {
      setRetryCount((c) => c + 1);
      onRetry();
    }, delay);

    // Countdown for UI feedback
    let remaining = delay;
    countdownRef.current = setInterval(() => {
      remaining -= 1000;
      setNextRetryInMs(Math.max(0, remaining));
    }, 1000);

    return clearTimers;
  }, [error, retryCount, onRetry, initialDelayMs, maxDelayMs, clearTimers]);

  return {
    retryCount,
    nextRetryInSeconds: nextRetryInMs !== null ? Math.ceil(nextRetryInMs / 1000) : null,
  };
}

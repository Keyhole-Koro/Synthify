'use client';

import { useEffect, useState, useCallback, useRef } from 'react';

interface UseExponentialBackoffArgs {
  onRetry: () => void;
  error: unknown;
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

  // Initialize the countdown whenever a backoff episode (re)starts — i.e. when
  // the error appears/changes or a retry advances retryCount — and reset it when
  // the error clears. Adjusting state during render (instead of in an effect)
  // avoids the cascading re-render that synchronous setState in useEffect causes.
  const [prevError, setPrevError] = useState(error);
  const [prevRetryCount, setPrevRetryCount] = useState(retryCount);
  if (error !== prevError || retryCount !== prevRetryCount) {
    setPrevError(error);
    if (error) {
      setPrevRetryCount(retryCount);
      setNextRetryInMs(Math.min(initialDelayMs * Math.pow(2, retryCount), maxDelayMs));
    } else {
      setRetryCount(0);
      setPrevRetryCount(0);
      setNextRetryInMs(null);
    }
  }

  useEffect(() => {
    if (!error) {
      clearTimers();
      return;
    }

    const delay = Math.min(initialDelayMs * Math.pow(2, retryCount), maxDelayMs);

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

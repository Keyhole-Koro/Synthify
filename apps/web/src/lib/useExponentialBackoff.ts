'use client';

import { useEffect, useState, useCallback, useRef } from 'react';

interface UseExponentialBackoffArgs {
  onRetry: () => void | Promise<void>;
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
  const retryingRef = useRef(false);

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

  const hasError = !!error;
  const [prevHasError, setPrevHasError] = useState(false);
  const [prevRetryCount, setPrevRetryCount] = useState(0);
  if (hasError !== prevHasError || retryCount !== prevRetryCount) {
    setPrevHasError(hasError);
    if (hasError) {
      setPrevRetryCount(retryCount);
      setNextRetryInMs(Math.min(initialDelayMs * Math.pow(2, retryCount), maxDelayMs));
    } else {
      setRetryCount(0);
      setPrevRetryCount(0);
      setNextRetryInMs(null);
    }
  }

  useEffect(() => {
    if (!hasError) {
      clearTimers();
      retryingRef.current = false;
      return;
    }

    let cancelled = false;

    const delay = Math.min(initialDelayMs * Math.pow(2, retryCount), maxDelayMs);

    timerRef.current = setTimeout(async () => {
      if (retryingRef.current) return;
      retryingRef.current = true;
      setNextRetryInMs(0);

      try {
        await onRetry();
      } finally {
        retryingRef.current = false;
        if (!cancelled) {
          setRetryCount((c) => c + 1);
        }
      }
    }, delay);

    let remaining = delay;
    countdownRef.current = setInterval(() => {
      remaining -= 1000;
      setNextRetryInMs(Math.max(0, remaining));
    }, 1000);

    return () => {
      cancelled = true;
      clearTimers();
    };
  }, [hasError, retryCount, onRetry, initialDelayMs, maxDelayMs, clearTimers]);

  return {
    retryCount,
    nextRetryInSeconds: nextRetryInMs !== null ? Math.ceil(nextRetryInMs / 1000) : null,
  };
}

import { useEffect, useState } from 'react';

/**
 * useElapsedTime returns the job's elapsed time as a "M:SS" string.
 *
 * While the job runs it ticks every second off `startedAt`. Once the job is
 * terminal (succeeded/failed) it freezes at `completedAt - startedAt`. Returns
 * null until `startedAt` is known (e.g. while still queued).
 *
 * @param startedAt - ISO-8601 start timestamp, or undefined while the job is queued.
 * @param completedAt - ISO-8601 terminal timestamp, or undefined while the job is running.
 * @param isTerminal - Whether the caller has observed a terminal job state.
 * @returns Elapsed duration formatted as `M:SS`, or null when no valid start timestamp exists.
 */
export function useElapsedTime(
  startedAt: string | undefined,
  completedAt: string | undefined,
  isTerminal: boolean,
): string | null {
  const startedMs = startedAt ? Date.parse(startedAt) : NaN;
  const completedMs = completedAt ? Date.parse(completedAt) : NaN;

  // Frozen final value once terminal (or whenever completedAt exists).
  const frozen = Number.isFinite(startedMs) && Number.isFinite(completedMs);

  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (frozen || isTerminal || !Number.isFinite(startedMs)) {
      return;
    }
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [frozen, isTerminal, startedMs]);

  if (!Number.isFinite(startedMs)) {
    return null;
  }
  const endMs = frozen ? completedMs : now;
  const seconds = Math.max(0, Math.floor((endMs - startedMs) / 1000));
  return formatElapsed(seconds);
}

/**
 * Formats a total duration in seconds as "M:SS".
 */
function formatElapsed(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

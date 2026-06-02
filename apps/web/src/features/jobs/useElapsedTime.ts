import { useEffect, useState } from 'react';

/**
 * useElapsedTime returns the job's elapsed time as a "M:SS" string.
 *
 * While the job runs it ticks every second off `startedAt`. Once the job is
 * terminal (succeeded/failed) it freezes at `completedAt - startedAt`. Returns
 * null until `startedAt` is known (e.g. while still queued).
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

function formatElapsed(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

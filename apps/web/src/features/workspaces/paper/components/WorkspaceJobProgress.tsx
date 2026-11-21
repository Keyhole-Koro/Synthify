
import { useElapsedTime } from '@/features/jobs/useElapsedTime';

/**
 * Props for the {@link WorkspaceJobProgress} component.
 */
interface WorkspaceJobProgressProps {
  /** Optional status or stage text reported by the worker. */
  message?: string | null;
  /** Numeric completion percentage (0-100), or undefined if indeterminate. */
  progress?: number;
  /** Whether the job terminated in an error state. */
  isFailed: boolean;
  /** ISO-8601 timestamp string when the job started. */
  startedAt?: string;
  /** ISO-8601 timestamp string when the job reached terminal state. */
  completedAt?: string;
  /** Whether the job has reached a final state (succeeded, failed, cancelled). */
  isTerminal?: boolean;
  /** Optional detail string describing the latest worker activity. */
  latestActivity?: string;
}

/**
 * In-place progress bar component for the actively running job in a workspace.
 * Renders stage messages, elapsed time counters, and animated progress bars.
 *
 * @param props - Component properties.
 * @returns The rendered progress indicator element.
 */
export function WorkspaceJobProgress({
  message,
  progress,
  isFailed,
  startedAt,
  completedAt,
  isTerminal = false,
  latestActivity,
}: WorkspaceJobProgressProps) {
  const hasProgress = typeof progress === 'number';
  const clampedProgress = hasProgress ? Math.max(0, Math.min(100, progress)) : 100;
  const label = message ?? (isFailed ? '解析に失敗しました' : '解析中');
  const elapsed = useElapsedTime(startedAt, completedAt, isTerminal || isFailed);

  return (
    <div className="mt-3">
      <div className="mb-1.5 flex items-baseline justify-between gap-3">
        <span
          className={[
            'truncate text-xs font-medium',
            isFailed ? 'text-red-600' : 'text-stone-700',
          ].join(' ')}
        >
          {label}
        </span>
        <span className="flex items-baseline gap-2 font-mono text-[10px] tabular-nums text-stone-400">
          {elapsed && <span>{elapsed}</span>}
          {hasProgress && !isFailed && <span>{clampedProgress}%</span>}
        </span>
      </div>
      <div className="h-1.5 overflow-hidden rounded-full bg-stone-100">
        <div
          className={[
            'h-full rounded-full transition-[width] duration-500 ease-out',
            isFailed
              ? 'bg-red-500'
              : hasProgress
                ? 'bg-indigo-500'
                : 'progress-stripe-indeterminate bg-indigo-400',
          ].join(' ')}
          style={{ width: `${clampedProgress}%` }}
        />
      </div>
      {latestActivity && !isFailed && (
        <p className="mt-1.5 truncate text-[10px] text-stone-400">{latestActivity}</p>
      )}
    </div>
  );
}

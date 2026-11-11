import { PanelError } from '@/components/error/PanelError';
import { isRetryableError } from '@/lib/errors';
import { useExponentialBackoff } from '@/lib/useExponentialBackoff';

/**
 * Props for the {@link WorkspaceError} display component.
 */
interface WorkspaceErrorProps {
  /** The error object caught during workspace loading or operations. */
  error: Error;
  /** Callback triggered to retry the failed workspace query or operation. */
  onRetry: () => void | Promise<void>;
}

/**
 * Renders an error message inside the workspace paper view with exponential backoff
 * countdown retry support for transient failures.
 *
 * @param props - Component properties.
 * @returns The rendered error panel with retry behavior.
 */
export function WorkspaceError({ error, onRetry }: WorkspaceErrorProps) {
  const isRetryable = isRetryableError(error);
  const { nextRetryInSeconds } = useExponentialBackoff({
    error: isRetryable ? error : null,
    onRetry,
  });

  const message = nextRetryInSeconds !== null
    ? `${error.message || 'しばらく時間をおいてから、再度お試しください。'} (${nextRetryInSeconds}秒後に再試行します...)`
    : error.message;

  return (
    <PanelError
      error={new Error(message)}
    />
  );
}

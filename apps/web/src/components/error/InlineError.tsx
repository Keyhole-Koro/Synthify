
/**
 * Props for rendering an inline error alert message.
 */
interface InlineErrorProps {
  /** Error message to display. If empty or null, renders nothing. */
  message: string;
  /** Optional additional CSS class names. */
  className?: string;
}

/**
 * Compact inline error message component with an alert icon, typically placed below form inputs.
 *
 * @param props - Message and optional CSS class names. Empty messages render no markup.
 * @returns Inline alert markup, or null when the message is empty.
 */
export function InlineError({ message, className = '' }: InlineErrorProps) {
  if (!message) return null;

  return (
    <div className={`flex items-center gap-1.5 text-xs font-medium text-red-600 ${className}`}>
      <svg
        className="h-3.5 w-3.5 flex-shrink-0"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
      <span>{message}</span>
    </div>
  );
}

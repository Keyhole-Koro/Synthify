import { isAuthError } from '@/lib/errors';

/**
 * Props for displaying a panel-level error alert box.
 */
interface PanelErrorProps {
  /** The error instance to inspect and display. */
  error: Error;
  /** Optional additional CSS class names. */
  className?: string;
}

/**
 * Boxed panel error component for display within sidebars, cards, or workspace panels.
 *
 * @param props - Error instance and optional CSS class names for the panel wrapper.
 * @returns Inline panel markup with an authentication-specific heading when applicable.
 */
export function PanelError({ error, className = '' }: PanelErrorProps) {
  return (
    <div className={`flex flex-col gap-3 pt-1 ${className}`}>
      <div className="rounded-lg border border-red-100 bg-red-50 px-3 py-2.5">
        <p className="text-xs font-semibold text-red-600">
          {isAuthError(error) ? '認証エラーが発生しました' : '読み込みに失敗しました'}
        </p>
        <p className="mt-0.5 text-[11px] leading-relaxed text-red-400">
          {error.message || 'しばらく時間をおいてから、再度お試しください。'}
        </p>
      </div>
    </div>
  );
}

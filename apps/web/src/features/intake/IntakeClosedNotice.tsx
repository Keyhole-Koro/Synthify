import React from 'react';
import { INTAKE_CLOSED_NOTICE } from './intakeStatus';

// IntakeClosedBanner runs across the top of the page while intake is closed,
// above the canvas, so it stays in view however the papers are opened.
export function IntakeClosedBanner() {
  return (
    <div
      role="status"
      className="flex shrink-0 items-center justify-center gap-2 border-b border-amber-200 bg-amber-50 px-4 py-2 text-[13px] font-medium leading-relaxed text-amber-900"
    >
      <svg className="h-4 w-4 shrink-0 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      <p className="m-0">{INTAKE_CLOSED_NOTICE}</p>
    </div>
  );
}

// IntakeClosedNote takes the place of an upload or upgrade control.
export function IntakeClosedNote({ message, className = '' }: { message: string; className?: string }) {
  return (
    <p
      role="status"
      className={`m-0 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-center text-xs font-medium text-amber-800 ${className}`}
    >
      {message}
    </p>
  );
}

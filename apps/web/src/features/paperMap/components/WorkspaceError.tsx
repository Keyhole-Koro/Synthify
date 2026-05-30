'use client';

import React from 'react';
import { PanelError } from '@/components/error/PanelError';
import { type AppError } from '@/lib/errors';
import { useExponentialBackoff } from '@/lib/useExponentialBackoff';

interface WorkspaceErrorProps {
  error: AppError;
  onRetry: () => void;
}

export function WorkspaceError({ error, onRetry }: WorkspaceErrorProps) {
  const { nextRetryInSeconds } = useExponentialBackoff({ error, onRetry });

  const message = nextRetryInSeconds !== null
    ? `${error.message || 'しばらく時間をおいてから、再度お試しください。'} (${nextRetryInSeconds}秒後に再試行します...)`
    : error.message;

  return (
    <PanelError
      error={{ ...error, message }}
    />
  );
}

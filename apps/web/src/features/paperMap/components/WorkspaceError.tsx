import React from 'react';
import { PanelError } from '@/components/error/PanelError';
import { type AppError } from '@/lib/errors';

interface WorkspaceErrorProps {
  error: AppError;
  onRetry: () => void;
  onLogout: () => void;
}

export function WorkspaceError({ error, onRetry, onLogout }: WorkspaceErrorProps) {
  return (
    <PanelError
      error={error}
      onRetry={onRetry}
      onLogout={onLogout}
    />
  );
}

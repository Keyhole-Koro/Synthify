'use client';

import { useEffect } from 'react';
import { FatalErrorView } from '@/components/error/FatalErrorView';

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // In a real app, you would log the error to a service like Sentry or New Relic
    console.error('Fatal Error Boundary:', error);
  }, [error]);

  return <FatalErrorView error={error} reset={reset} />;
}

'use client';

import { useEffect } from 'react';
import { FatalErrorView } from '@/components/error/FatalErrorView';
import { noticeBrowserError } from '@/lib/newrelic/browser';

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('Fatal Error Boundary:', error);
    noticeBrowserError(error, {
      source: 'next_error_boundary',
      digest: error.digest ?? '',
    });
  }, [error]);

  return <FatalErrorView error={error} reset={reset} />;
}

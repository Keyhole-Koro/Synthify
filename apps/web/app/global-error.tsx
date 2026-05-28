'use client';

import { useEffect } from 'react';
import { FatalErrorView } from '@/components/error/FatalErrorView';
import { noticeBrowserError } from '@/lib/newrelic/browser';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('Global Error Boundary:', error);
    noticeBrowserError(error, {
      source: 'next_global_error_boundary',
      digest: error.digest ?? '',
    });
  }, [error]);

  return (
    <html lang="ja">
      <body>
        <FatalErrorView error={error} reset={reset} />
      </body>
    </html>
  );
}

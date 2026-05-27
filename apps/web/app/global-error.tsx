'use client';

import { useEffect } from 'react';
import { FatalErrorView } from '@/components/error/FatalErrorView';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('Global Error Boundary:', error);
  }, [error]);

  return (
    <html lang="ja">
      <body>
        <FatalErrorView error={error} reset={reset} />
      </body>
    </html>
  );
}

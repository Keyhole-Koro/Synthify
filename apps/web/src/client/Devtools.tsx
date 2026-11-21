import React, { Suspense } from 'react';

const TanStackRouterDevtools = import.meta.env.DEV
  ? React.lazy(() =>
      import('@tanstack/router-devtools').then((res) => ({
        default: res.TanStackRouterDevtools,
      })),
    )
  : () => null;

const ReactQueryDevtools = import.meta.env.DEV
  ? React.lazy(() =>
      import('@tanstack/react-query-devtools').then((res) => ({
        default: res.ReactQueryDevtools,
      })),
    )
  : () => null;

/**
 * Lazy-loaded developer tools for TanStack Router and React Query, rendered exclusively in development mode.
 *
 * @returns Development-only devtools overlays, or null in production builds.
 */
export function Devtools() {
  if (!import.meta.env.DEV) return null;

  return (
    <Suspense fallback={null}>
      <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-right" />
      <TanStackRouterDevtools position="bottom-left" />
    </Suspense>
  );
}

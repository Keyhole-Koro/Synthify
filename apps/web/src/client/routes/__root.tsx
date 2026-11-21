import { useEffect } from 'react';
import { createRootRouteWithContext, Link, Outlet, type ErrorComponentProps } from '@tanstack/react-router';
import type { RouterContext } from '@/client/router';
import { NewRelicUserSync } from '@/client/NewRelicUserSync';
import { Devtools } from '@/client/Devtools';
import { FatalErrorView } from '@/components/error/FatalErrorView';
import { noticeBrowserError } from '@/lib/newrelic/browser';

/**
 * Top-level application shell rendering the active route outlet, devtools, and user telemetry sync.
 */
function RootLayout() {
  return (
    <>
      <NewRelicUserSync />
      <Outlet />
      <Devtools />
    </>
  );
}

/**
 * Root error boundary catching uncaught runtime exceptions and notifying New Relic Browser.
 */
function RootErrorComponent({ error, reset }: ErrorComponentProps) {
  useEffect(() => {
    console.error('Fatal Error Boundary:', error);
    const err = error instanceof Error ? error : new Error(String(error));
    noticeBrowserError(err, {
      source: 'tanstack_error_boundary',
    });
  }, [error]);

  const viewError = error instanceof Error ? error : new Error(String(error));

  return <FatalErrorView error={viewError} reset={reset} />;
}

/**
 * Catch-all 404 view for unmatched application routes.
 */
function RootNotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-slate-50 p-4 text-center">
      <h1 className="text-2xl font-bold text-slate-800">ページが見つかりません</h1>
      <p className="mt-2 text-sm text-slate-500">URL が正しいかご確認ください。</p>
      <Link
        to="/"
        className="mt-6 rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-blue-700"
      >
        ホームへ戻る
      </Link>
    </div>
  );
}

/**
 * Root route definition tying together application shell, error boundaries, and 404 views.
 */
export const Route = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  errorComponent: RootErrorComponent,
  notFoundComponent: RootNotFound,
});

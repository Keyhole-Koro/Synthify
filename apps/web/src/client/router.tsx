import { QueryClient } from '@tanstack/react-query';
import { createRouter } from '@tanstack/react-router';
import { routeTree } from '@/client/routeTree.gen';

/**
 * Shared TanStack Query client configured with disabled window focus refetching and standard stale times.
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
      staleTime: 30_000,
    },
  },
});

/**
 * Global TanStack Router context, accessible in route `beforeLoad` and `loader` functions.
 *
 * @property queryClient Shared TanStack Query cache used by route loaders and guards.
 */
export type RouterContext = {
  queryClient: QueryClient;
};

/**
 * Creates and configures the application TanStack router with query client context and scroll restoration.
 *
 * @returns Configured TanStack Router instance bound to the generated route tree and shared
 *   query client. The returned type is also registered with TanStack Router's module types.
 */
export function createAppRouter() {
  return createRouter({
    routeTree,
    context: { queryClient },
    defaultPreload: 'intent',
    scrollRestoration: true,
  });
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}

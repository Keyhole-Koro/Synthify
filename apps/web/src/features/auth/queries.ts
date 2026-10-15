import { queryOptions } from '@tanstack/react-query';
import { signInUser } from '@/features/auth/userApi';

/**
 * TanStack query options for provisioning the signed-in user (users, accounts and
 * account_users rows) once per user. The response carries the billing account ID.
 *
 * @param userId - The active user's ID or null.
 * @returns Configured TanStack query options object.
 */
export function provisionUserQueryOptions(userId?: string | null) {
  return queryOptions({
    queryKey: ['auth', 'provision', userId ?? ''] as const,
    queryFn: () => signInUser(),
    staleTime: Infinity,
    retry: 1,
    enabled: Boolean(userId),
  });
}

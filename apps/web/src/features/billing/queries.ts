import { queryOptions } from '@tanstack/react-query';
import {
  getBillingAccount,
  getUsage,
  listInvoices,
  listPaymentMethods,
  createPortalSession,
} from './api';

/**
 * Builds the stable React Query key for one billing account.
 * @param accountId - Account identifier used by the billing API.
 * @returns A tuple key whose account segment prevents cross-account cache reuse.
 */
export const billingAccountQueryKey = (accountId: string) => ['billing', 'account', accountId] as const;

/**
 * Builds the cache key for a billing usage report.
 * @param accountId - Account identifier used by the report.
 * @param periodStart - Inclusive period start sent to the API, or empty for the default.
 * @param periodEnd - Exclusive period end sent to the API, or empty for the default.
 * @returns A tuple containing the account and exact report period.
 */
export const billingUsageQueryKey = (accountId: string, periodStart = '', periodEnd = '') =>
  ['billing', 'usage', accountId, periodStart, periodEnd] as const;

/**
 * Builds the cache key for invoice history.
 * @param accountId - Account whose invoices are requested.
 * @param limit - Page size used by the request.
 * @returns A tuple containing the account and page size.
 */
export const billingInvoicesQueryKey = (accountId: string, limit = 12) =>
  ['billing', 'invoices', accountId, limit] as const;

/**
 * Builds the cache key for saved payment methods.
 * @param accountId - Account whose payment methods are requested.
 * @returns A stable account-scoped payment-method key.
 */
export const billingPaymentMethodsQueryKey = (accountId: string) =>
  ['billing', 'payment-methods', accountId] as const;

/**
 * Builds the cache key for a Stripe customer portal session.
 * @param accountId - Account for which the portal URL is created.
 * @returns A stable account-scoped portal key.
 */
export const billingPortalQueryKey = (accountId: string) =>
  ['billing', 'portal', accountId] as const;

/**
 * Constructs TanStack Query options for fetching billing account details.
 *
 * @param accountId - Target account identifier.
 * @returns Query options with a one-minute stale time and disabled execution for an empty ID.
 */
export function billingAccountQueryOptions(accountId: string) {
  return queryOptions({
    queryKey: billingAccountQueryKey(accountId),
    queryFn: () => getBillingAccount(accountId),
    staleTime: 60_000,
    enabled: Boolean(accountId),
  });
}

/**
 * Constructs TanStack Query options for fetching usage metrics.
 *
 * @param accountId - Target account identifier.
 * @param periodStart - Optional period start date.
 * @param periodEnd - Optional period end date.
 * @returns Query options with a 30-second stale time and the requested date range in its key.
 */
export function billingUsageQueryOptions(accountId: string, periodStart = '', periodEnd = '') {
  return queryOptions({
    queryKey: billingUsageQueryKey(accountId, periodStart, periodEnd),
    queryFn: () => getUsage(accountId, periodStart, periodEnd),
    staleTime: 30_000,
    enabled: Boolean(accountId),
  });
}

/**
 * Constructs TanStack Query options for retrieving invoice history.
 *
 * @param accountId - Target account identifier.
 * @param limit - Max invoices to retrieve.
 * @returns Query options whose key changes when the page size changes.
 */
export function billingInvoicesQueryOptions(accountId: string, limit = 12) {
  return queryOptions({
    queryKey: billingInvoicesQueryKey(accountId, limit),
    queryFn: () => listInvoices(accountId, limit),
    staleTime: 60_000,
    enabled: Boolean(accountId),
  });
}

/**
 * Constructs TanStack Query options for listing payment methods.
 *
 * @param accountId - Target account identifier.
 * @returns Account-scoped query options with a one-minute stale time.
 */
export function billingPaymentMethodsQueryOptions(accountId: string) {
  return queryOptions({
    queryKey: billingPaymentMethodsQueryKey(accountId),
    queryFn: () => listPaymentMethods(accountId),
    staleTime: 60_000,
    enabled: Boolean(accountId),
  });
}

/**
 * Constructs TanStack Query options for acquiring a Stripe customer portal session URL.
 *
 * @param accountId - Target account identifier.
 * @returns Query options that lazily create the portal session only for a non-empty account ID.
 */
export function billingPortalQueryOptions(accountId: string) {
  return queryOptions({
    queryKey: billingPortalQueryKey(accountId),
    queryFn: () => createPortalSession(accountId),
    staleTime: 60_000,
    enabled: Boolean(accountId),
  });
}

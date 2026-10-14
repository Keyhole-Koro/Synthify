import { createRPCClient } from '@/lib/connect';
import {
  BillingService,
  type GetBillingAccountResponse,
  type GetUsageResponse,
  type ListInvoicesResponse,
  type ListPaymentMethodsResponse,
} from '@/gen/proto/synthify/app/v1/billing_pb';

const client = createRPCClient(BillingService);

/**
 * Supported billing currencies ('jpy' or 'usd').
 */
export type BillingCurrency = 'jpy' | 'usd';
export type { GetBillingAccountResponse as BillingAccount };
export type { GetUsageResponse as UsageReport };
export type { ListInvoicesResponse as InvoiceList };
export type { ListPaymentMethodsResponse as PaymentMethodList };

/**
 * Fetches billing account details including current plan, quota, and budget status.
 *
 * @param accountId - The account identifier.
 * @returns Billing account response.
 */
export async function getBillingAccount(accountId: string): Promise<GetBillingAccountResponse> {
  return client.getBillingAccount({ accountId });
}

/**
 * Initiates a Stripe Checkout session for upgrading to the usage-based plan.
 *
 * @param accountId - Target account identifier.
 * @param currency - Desired currency ('jpy' or 'usd').
 * @returns The Stripe Checkout redirect URL.
 */
export async function createCheckoutSession(accountId: string, currency: BillingCurrency): Promise<string> {
  const res = await client.createCheckoutSession({ accountId, currency });
  return res.checkoutUrl;
}

/**
 * Creates a Stripe Customer Portal session URL for managing payment methods and subscriptions.
 *
 * @param accountId - Target account identifier.
 * @returns Stripe Customer Portal URL.
 */
export async function createPortalSession(accountId: string): Promise<string> {
  const res = await client.createPortalSession({ accountId });
  return res.portalUrl;
}

/**
 * Fetches aggregated LLM usage and cost reports for an account within an optional date range.
 *
 * @param accountId - Target account identifier.
 * @param periodStart - Optional ISO period start.
 * @param periodEnd - Optional ISO period end.
 * @returns Aggregated usage response.
 */
export async function getUsage(
  accountId: string,
  periodStart = '',
  periodEnd = '',
): Promise<GetUsageResponse> {
  return client.getUsage({ accountId, periodStart, periodEnd });
}

/**
 * Updates the monthly spending budget limit for an account.
 *
 * @param accountId - Target account identifier.
 * @param budgetLimit - New budget limit amount as a string (e.g. "50").
 * @returns The persisted budget limit.
 */
export async function updateBudget(accountId: string, budgetLimit: string): Promise<string> {
  const res = await client.updateBudget({ accountId, budgetLimit });
  return res.budgetLimit;
}

/**
 * Retrieves past billing invoices for an account.
 *
 * @param accountId - Target account identifier.
 * @param limit - Maximum number of invoices to return (defaults to 12).
 * @returns Invoice list response.
 */
export async function listInvoices(accountId: string, limit = 12): Promise<ListInvoicesResponse> {
  return client.listInvoices({ accountId, limit });
}

/**
 * Lists registered payment methods (credit cards) on file for an account.
 *
 * @param accountId - Target account identifier.
 * @returns Payment methods response.
 */
export async function listPaymentMethods(accountId: string): Promise<ListPaymentMethodsResponse> {
  return client.listPaymentMethods({ accountId });
}

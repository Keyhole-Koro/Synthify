import { createRPCClient } from '@/lib/connect';
import {
  BillingService,
  type GetBillingAccountResponse,
  type GetUsageResponse,
  type ListInvoicesResponse,
  type ListPaymentMethodsResponse,
} from '@synthify/proto-ts/gen/synthify/tree/v1/billing_pb';

const client = createRPCClient(BillingService);

export type BillingCurrency = 'jpy' | 'usd';
export type { GetBillingAccountResponse as BillingAccount };
export type { GetUsageResponse as UsageReport };
export type { ListInvoicesResponse as InvoiceList };
export type { ListPaymentMethodsResponse as PaymentMethodList };

export async function getBillingAccount(accountId: string): Promise<GetBillingAccountResponse> {
  return client.getBillingAccount({ accountId });
}

export async function createCheckoutSession(accountId: string, currency: BillingCurrency): Promise<string> {
  const res = await client.createCheckoutSession({ accountId, currency });
  return res.checkoutUrl;
}

export async function createPortalSession(accountId: string): Promise<string> {
  const res = await client.createPortalSession({ accountId });
  return res.portalUrl;
}

export async function getUsage(
  accountId: string,
  periodStart = '',
  periodEnd = '',
): Promise<GetUsageResponse> {
  return client.getUsage({ accountId, periodStart, periodEnd });
}

export async function updateBudget(accountId: string, budgetLimit: string): Promise<string> {
  const res = await client.updateBudget({ accountId, budgetLimit });
  return res.budgetLimit;
}

export async function listInvoices(accountId: string, limit = 12): Promise<ListInvoicesResponse> {
  return client.listInvoices({ accountId, limit });
}

export async function listPaymentMethods(accountId: string): Promise<ListPaymentMethodsResponse> {
  return client.listPaymentMethods({ accountId });
}

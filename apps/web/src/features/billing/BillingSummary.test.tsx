import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setIntakeClosed } from '@/features/intake/intakeStatus';
import { BillingSummary } from './BillingSummary';

const api = vi.hoisted(() => ({ getBillingAccount: vi.fn() }));

vi.mock('./api', () => ({ getBillingAccount: api.getBillingAccount }));

const account = {
  accountId: 'acc',
  plan: 'usage_based',
  storageQuotaBytes: BigInt(10_485_760),
  storageUsedBytes: BigInt(1_048_576),
  currentPeriodUsage: '25',
  budgetLimit: '100',
  budgetExceeded: false,
};

describe('BillingSummary', () => {
  beforeEach(() => {
    api.getBillingAccount.mockReset();
    api.getBillingAccount.mockResolvedValue(account);
  });

  // Unmount first: a mounted paper would react to the reset.
  afterEach(() => {
    cleanup();
    setIntakeClosed(false);
  });

  it('links to the upgrade while paid plans are open', async () => {
    render(<BillingSummary accountId="acc" />);
    expect(await screen.findByText('アップグレード')).toBeTruthy();
  });

  it('drops the upgrade once paid plans close but keeps the Stripe portal', async () => {
    setIntakeClosed(true);
    render(<BillingSummary accountId="acc" />);
    expect(await screen.findByText('管理')).toBeTruthy();
    expect(screen.getByText('Stripe ポータル')).toBeTruthy();
    expect(screen.queryByText('アップグレード')).toBeNull();
  });
});

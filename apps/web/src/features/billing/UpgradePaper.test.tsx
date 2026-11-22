import { ConnectError } from '@connectrpc/connect';
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setIntakeClosed } from '@/features/intake/intakeStatus';
import { createAppError } from '@/lib/errors';
import { UpgradePaper } from './UpgradePaper';

const api = vi.hoisted(() => ({ createCheckoutSession: vi.fn() }));

vi.mock('./api', () => ({ createCheckoutSession: api.createCheckoutSession }));

describe('UpgradePaper', () => {
  beforeEach(() => {
    api.createCheckoutSession.mockReset();
  });

  // Unmount first: a mounted paper would react to the reset.
  afterEach(() => {
    cleanup();
    setIntakeClosed(false);
  });

  it('offers Stripe checkout while paid plans are open', async () => {
    api.createCheckoutSession.mockResolvedValue('https://checkout.stripe.test/e2e');
    render(<UpgradePaper accountId="acc" />);
    expect(await screen.findByRole('link', { name: /Stripe で決済/ })).toBeTruthy();
  });

  it('says paid plans are closed and never asks for a checkout', () => {
    setIntakeClosed(true);
    render(<UpgradePaper accountId="acc" />);
    expect(screen.getByText('有料プランの受け付けは終了しました。')).toBeTruthy();
    expect(screen.queryByRole('link', { name: /Stripe で決済/ })).toBeNull();
    expect(screen.queryByRole('button', { name: 'jpy' })).toBeNull();
    expect(api.createCheckoutSession).not.toHaveBeenCalled();
  });

  it('shows what the API said when it refuses a page that is out of date', async () => {
    // What a client call rejects with: the transport's AppError, wrapped by
    // connect-es in a ConnectError.
    api.createCheckoutSession.mockRejectedValue(ConnectError.from(createAppError({
      kind: 'validation',
      message: '有料プランの受け付けは終了しました。',
      code: 'FailedPrecondition',
      reason: 'intake_closed',
    })));
    render(<UpgradePaper accountId="acc" />);
    expect(await screen.findByText('有料プランの受け付けは終了しました。')).toBeTruthy();
    expect(screen.queryByText('決済URLを取得できませんでした。')).toBeNull();
  });
});

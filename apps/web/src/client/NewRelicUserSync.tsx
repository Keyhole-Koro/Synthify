
import { useEffect } from 'react';
import { initNewRelicBrowser, setBrowserUserId } from '@/lib/newrelic/browser';
import { subscribeAuthUser } from '@/features/auth/session';

/**
 * Component that synchronizes authenticated user state to New Relic Browser and ensures early agent initialization.
 *
 * @returns Nothing visible; renders null after installing the auth subscription and telemetry hooks.
 */
export function NewRelicUserSync() {
  useEffect(() => {
    // Initialize eagerly (not just on the first auth callback) so the agent —
    // and the console wrapLogger backstop set up during init — is live as early
    // as possible, capturing errors that fire before auth resolves.
    void initNewRelicBrowser();
    return subscribeAuthUser((user) => {
      setBrowserUserId(user?.id ?? null);
    });
  }, []);

  return null;
}

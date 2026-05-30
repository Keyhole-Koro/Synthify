'use client';

import { useEffect } from 'react';
import { initNewRelicBrowser, setBrowserUserId } from '@/lib/newrelic/browser';
import { subscribeAuthUser } from '@/features/auth/session';

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
